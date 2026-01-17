#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/stack-env.sh"
source "${ROOT_DIR}/scripts/lib/compose.sh"

LOG_DIR="${ROOT_DIR}/tmp/run"
mkdir -p "${LOG_DIR}"
RUN_DEBUG_LOG="${LOG_DIR}/run.debug.log"
READAPI_LOG="${LOG_DIR}/readapi.log"
READAPI_PIDFILE="${LOG_DIR}/readapi.pid"

: > "${RUN_DEBUG_LOG}"
exec > >(tee -a "${RUN_DEBUG_LOG}") 2>&1

say() {
  echo "[run] $*"
}

die() {
  echo "ERROR: $*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "Missing required command: $1"
}

find_flutter_dir() {
  if [[ -n "${FLUTTER_DIR:-}" ]]; then
    if [[ -f "${FLUTTER_DIR}/pubspec.yaml" ]]; then
      echo "${FLUTTER_DIR}"
      return 0
    fi
    die "FLUTTER_DIR is set but pubspec.yaml was not found at ${FLUTTER_DIR}"
  fi

  if [[ -f "${ROOT_DIR}/apps/client/pubspec.yaml" ]]; then
    echo "${ROOT_DIR}/apps/client"
    return 0
  fi

  if [[ -f "${ROOT_DIR}/client/pubspec.yaml" ]]; then
    echo "${ROOT_DIR}/client"
    return 0
  fi

  local found
  found="$(find "${ROOT_DIR}/apps" "${ROOT_DIR}/client" -name pubspec.yaml -print 2>/dev/null | head -n 1 || true)"
  if [[ -n "${found}" ]]; then
    echo "$(dirname "${found}")"
    return 0
  fi

  return 1
}

listings_count() {
  local count
  count="$(docker exec -i "${POSTGRES_CONTAINER}" \
    psql -U realestate -d realestate -tAc "select count(*) from listings;")"
  count="$(echo "${count}" | tr -d '[:space:]')"
  case "${count}" in
    ''|*[!0-9]*) return 1 ;;
  esac
  echo "${count}"
}

readapi_health_ok() {
  curl -fsS --max-time 1 "${READAPI_BASE_URL}/healthz" >/dev/null 2>&1
}

wait_for_readapi() {
  local attempts=0
  while [[ "${attempts}" -lt 60 ]]; do
    attempts=$((attempts + 1))
    if readapi_health_ok; then
      return 0
    fi
    sleep 0.5
  done
  return 1
}

readapi_listener_pid() {
  lsof -nP -iTCP:${READAPI_PORT} -sTCP:LISTEN -t 2>/dev/null | head -n 1 || true
}

readapi_pidfile_matches() {
  local pid="$1"
  if [[ ! -f "${READAPI_PIDFILE}" ]]; then
    return 1
  fi
  local file_pid
  file_pid="$(cat "${READAPI_PIDFILE}" 2>/dev/null || true)"
  [[ -n "${file_pid}" && "${file_pid}" == "${pid}" ]]
}

readapi_cmd_matches() {
  local pid="$1"
  local cmd
  cmd="$(ps -p "${pid}" -o command= 2>/dev/null || true)"
  case "${cmd}" in
    *"cmd/readapi"*|*"services/readapi"*|*"/readapi"*|*" readapi "*|*" readapi") return 0 ;;
  esac
  return 1
}

stop_readapi_if_ours() {
  local pid="$1"
  if readapi_pidfile_matches "${pid}" || readapi_cmd_matches "${pid}"; then
    say "Stopping stale readapi (pid ${pid})"
    kill "${pid}" >/dev/null 2>&1 || true
    rm -f "${READAPI_PIDFILE}"
    sleep 0.5
    return 0
  fi
  return 1
}

cleanup() {
  local status=$?
  if [[ "${status}" -ne 0 ]]; then
    echo "[run] Failure detected; log tails:" >&2
    if [[ -f "${RUN_DEBUG_LOG}" ]]; then
      echo "[run] ${RUN_DEBUG_LOG} (tail 200):" >&2
      tail -n 200 "${RUN_DEBUG_LOG}" >&2 || true
    fi
    if [[ -f "${READAPI_LOG}" ]]; then
      echo "[run] ${READAPI_LOG} (tail 200):" >&2
      tail -n 200 "${READAPI_LOG}" >&2 || true
    fi
  fi

  if [[ -n "${READAPI_PID:-}" && "${READAPI_REUSED:-}" != "true" ]]; then
    kill "${READAPI_PID}" >/dev/null 2>&1 || true
    wait "${READAPI_PID}" >/dev/null 2>&1 || true
    rm -f "${READAPI_PIDFILE}"
  fi

  exit "${status}"
}
trap cleanup EXIT

require_cmd docker
require_cmd go
require_cmd flutter
require_cmd curl
require_cmd make
require_cmd lsof

if ! docker compose version >/dev/null 2>&1; then
  die "docker compose is required"
fi

[[ -f infra/.env ]] || die "Missing infra/.env"
[[ -f infra/docker-compose.yml ]] || die "Missing infra/docker-compose.yml"
[[ -f scripts/init-localstack.sh ]] || die "Missing scripts/init-localstack.sh"
[[ -f scripts/db-migrate.sh ]] || die "Missing scripts/db-migrate.sh"
[[ -f "${ROOT_DIR}/apps/client/pubspec.yaml" ]] || die "Missing Flutter app at apps/client"

STACK="run"
export STACK
stack_env

POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-calgary-realestate-infra-postgres-1}"
READAPI_PORT="${READAPI_PORT:-8090}"
READAPI_BASE_URL="http://127.0.0.1:${READAPI_PORT}"
READAPI_REUSED="false"

say "Logs directory: ${LOG_DIR}"
say "Stack: ${STACK} (project $(compose_project))"
say "Starting infra"
compose_infra up -d --remove-orphans

say "Initializing LocalStack"
./scripts/init-localstack.sh

say "Running DB migrations"
./scripts/db-migrate.sh

if [[ "${RUN_SCRAPE:-}" == "true" ]]; then
  say "RUN_SCRAPE=true; running make scrape"
  make scrape
else
  count=""
  if ! count="$(listings_count)"; then
    die "Failed to read listing count from ${POSTGRES_CONTAINER}"
  fi

  if [[ "${count}" -gt 0 ]]; then
    say "Using existing DB data (${count} listings)"
  else
    say "DB empty; running make scrape"
    make scrape
  fi
fi

count=""
if ! count="$(listings_count)"; then
  die "Failed to read listing count from ${POSTGRES_CONTAINER}"
fi
if [[ "${count}" -le 0 ]]; then
  die "No listings found in DB after scrape. Run make scrape to populate real data."
fi

if readapi_health_ok; then
  say "ReadAPI already running on :${READAPI_PORT}; reusing."
  READAPI_REUSED="true"
else
  listener_pid="$(readapi_listener_pid)"
  if [[ -n "${listener_pid}" ]]; then
    if ! stop_readapi_if_ours "${listener_pid}"; then
      die "Port ${READAPI_PORT} is in use by another process (pid ${listener_pid}). Set READAPI_PORT to a free port."
    fi
  fi

  : > "${READAPI_LOG}"
  say "Starting readapi on ${READAPI_BASE_URL}"
  PORT="${READAPI_PORT}" \
  DB_ENABLED=true \
  bash scripts/run-readapi.sh > "${READAPI_LOG}" 2>&1 &
  READAPI_PID=$!
  echo "${READAPI_PID}" > "${READAPI_PIDFILE}"
fi

say "Waiting for readapi readiness"
if ! wait_for_readapi; then
  die "ReadAPI is not ready. See ${READAPI_LOG}"
fi

say "ReadAPI is ready at ${READAPI_BASE_URL}"
say "Starting Flutter client..."
flutter_dir="$(find_flutter_dir || true)"
if [[ -z "${flutter_dir}" ]]; then
  die "Unable to locate Flutter app directory. Set FLUTTER_DIR or place pubspec.yaml under ./apps or ./client."
fi
cd "${flutter_dir}"
flutter run -d chrome --dart-define=READAPI_BASE_URL="${READAPI_BASE_URL}"
