#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/stack-env.sh"
source "${ROOT_DIR}/scripts/lib/compose.sh"

LOG_DIR="${ROOT_DIR}/tmp/run"
mkdir -p "${LOG_DIR}"
RUN_DEBUG_LOG="${LOG_DIR}/run.debug.log"
PARSER_LOG="${LOG_DIR}/parser.log"
READAPI_LOG="${LOG_DIR}/readapi.log"
READAPI_BIN="${LOG_DIR}/readapi"
READAPI_COUNT_BODY="${LOG_DIR}/readapi_count.json"
READAPI_LIST_BODY="${LOG_DIR}/readapi_listings.json"

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

awslocal_exec() {
  local cmd="$1"
  compose_infra exec -T localstack sh -lc "AWS_PAGER=\"\" AWS_DEFAULT_REGION=${AWS_REGION} AWS_REGION=${AWS_REGION} ${cmd}"
}

listings_count() {
  local count
  count="$(compose_infra exec -T postgres psql -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -t -A -c "select count(*) from listings;")"
  count="$(echo "${count}" | tr -d '[:space:]')"
  case "${count}" in
    ''|*[!0-9]*) return 1 ;;
  esac
  echo "${count}"
}

json_has_count() {
  local file="$1"
  "${PYTHON_BIN}" - <<'PY' < "${file}" >/dev/null 2>&1
import json
import sys
from numbers import Integral

data = json.load(sys.stdin)
if "count" not in data:
    raise SystemExit(1)
if not isinstance(data["count"], Integral):
    raise SystemExit(1)
PY
}

json_has_items() {
  local file="$1"
  "${PYTHON_BIN}" - <<'PY' < "${file}" >/dev/null 2>&1
import json
import sys

data = json.load(sys.stdin)
items = data.get("items")
if not isinstance(items, list):
    raise SystemExit(1)
PY
}

latest_zolo_run_key() {
  local key
  key="$(awslocal_exec "awslocal s3api list-objects-v2 --bucket \"${RAW_BUCKET}\" --prefix \"raw/zolo/\" --query 'reverse(sort_by(Contents || \`[]\`, &LastModified))[0].Key' --output text")"
  if [[ -z "${key}" || "${key}" == "None" ]]; then
    return 1
  fi
  if [[ "${key}" != raw/zolo/*/run-*.jsonl ]]; then
    return 1
  fi
  echo "${key}"
  return 0
}

queue_empty() {
  local queue_url="$1"
  local visible invisible
  visible="$(awslocal_exec "awslocal sqs get-queue-attributes --queue-url \"${queue_url}\" --attribute-names ApproximateNumberOfMessages --query 'Attributes.ApproximateNumberOfMessages' --output text")"
  invisible="$(awslocal_exec "awslocal sqs get-queue-attributes --queue-url \"${queue_url}\" --attribute-names ApproximateNumberOfMessagesNotVisible --query 'Attributes.ApproximateNumberOfMessagesNotVisible' --output text")"
  [[ -z "${visible}" || "${visible}" == "0" || "${visible}" == "None" ]] && \
    [[ -z "${invisible}" || "${invisible}" == "0" || "${invisible}" == "None" ]]
}

run_parser_once() {
  (
    cd services/parser
    AWS_REGION="${AWS_REGION}" \
    AWS_DEFAULT_REGION="${AWS_DEFAULT_REGION}" \
    RAW_BUCKET="${RAW_BUCKET}" \
    VAULT_BUCKET="${VAULT_BUCKET}" \
    RAW_EVENTS_QUEUE="${RAW_EVENTS_QUEUE}" \
    LOCALSTACK_ENDPOINT="http://localhost:${LOCALSTACK_PORT}" \
    POSTGRES_HOST="localhost" \
    POSTGRES_PORT="${POSTGRES_PORT}" \
    POSTGRES_DB="${POSTGRES_DB}" \
    POSTGRES_USER="${POSTGRES_USER}" \
    POSTGRES_PASSWORD="${POSTGRES_PASSWORD}" \
    ONCE=true \
    DRY_RUN=false \
    LOG_LEVEL=info \
    go run ./cmd/parser
  )
}

run_parser_until_queue_drains() {
  local queue_url
  queue_url="$(awslocal_exec "awslocal sqs get-queue-url --queue-name \"${RAW_EVENTS_QUEUE}\" --query 'QueueUrl' --output text")"
  if [[ -z "${queue_url}" || "${queue_url}" == "None" ]]; then
    die "Failed to resolve SQS queue URL for ${RAW_EVENTS_QUEUE}"
  fi

  local max_loops=20
  local i=1
  while [[ "${i}" -le "${max_loops}" ]]; do
    say "Parser pass ${i}/${max_loops}"
    run_parser_once 2>&1 | tee -a "${PARSER_LOG}"
    if queue_empty "${queue_url}"; then
      return 0
    fi
    sleep 0.5
    i=$((i + 1))
  done

  return 0
}

wait_for_readapi() {
  local attempts=0
  while [[ "${attempts}" -lt 60 ]]; do
    attempts=$((attempts + 1))

    local status
    status="$(curl -s --max-time 1 -o "${READAPI_COUNT_BODY}" -w "%{http_code}" \
      "${READAPI_BASE_URL}/v1/listings/count" 2>/dev/null || echo "000")"
    if [[ "${status}" == "200" ]] && json_has_count "${READAPI_COUNT_BODY}"; then
      local list_status
      list_status="$(curl -s --max-time 1 -o "${READAPI_LIST_BODY}" -w "%{http_code}" \
        "${READAPI_BASE_URL}/v1/listings?limit=10&offset=0" 2>/dev/null || echo "000")"
      if [[ "${list_status}" == "200" ]] && json_has_items "${READAPI_LIST_BODY}"; then
        return 0
      fi
    fi

    sleep 0.5
  done

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
    if [[ -f "${PARSER_LOG}" ]]; then
      echo "[run] ${PARSER_LOG} (tail 200):" >&2
      tail -n 200 "${PARSER_LOG}" >&2 || true
    fi
    if [[ -f "${READAPI_LOG}" ]]; then
      echo "[run] ${READAPI_LOG} (tail 200):" >&2
      tail -n 200 "${READAPI_LOG}" >&2 || true
    fi
  fi

  if [[ -n "${READAPI_PID:-}" ]]; then
    kill "${READAPI_PID}" >/dev/null 2>&1 || true
    wait "${READAPI_PID}" >/dev/null 2>&1 || true
  fi

  exit "${status}"
}
trap cleanup EXIT

require_cmd docker
require_cmd go
require_cmd flutter
require_cmd curl

if ! docker compose version >/dev/null 2>&1; then
  die "docker compose is required"
fi

PYTHON_BIN=""
if command -v python3 >/dev/null 2>&1; then
  PYTHON_BIN="python3"
elif command -v python >/dev/null 2>&1; then
  PYTHON_BIN="python"
else
  die "python3 or python is required to parse JSON responses"
fi

[[ -f infra/.env ]] || die "Missing infra/.env"
[[ -f infra/docker-compose.yml ]] || die "Missing infra/docker-compose.yml"
[[ -f scripts/init-localstack.sh ]] || die "Missing scripts/init-localstack.sh"
[[ -f scripts/db-migrate.sh ]] || die "Missing scripts/db-migrate.sh"
[[ -f services/readapi/go.mod ]] || die "Missing services/readapi/go.mod (readapi module)"

set -a
source infra/.env
set +a
stack_env

for var in LOCALSTACK_PORT RAW_BUCKET VAULT_BUCKET RAW_EVENTS_QUEUE POSTGRES_DB POSTGRES_USER POSTGRES_PASSWORD POSTGRES_PORT; do
  value="$(printenv "${var}" || true)"
  if [[ -z "${value}" ]]; then
    die "Missing required env var: ${var}"
  fi
done

export AWS_REGION="us-west-2"
export AWS_DEFAULT_REGION="${AWS_REGION}"
export AWS_PAGER=""

READAPI_PORT=8090
READAPI_BASE_URL="http://localhost:${READAPI_PORT}"

say "Logs directory: ${LOG_DIR}"
say "Stack: ${STACK} (project $(compose_project))"
say "Starting infra"
# Remove orphans (like readapi) so infra stays clean and warnings are avoided.
compose_infra up -d --remove-orphans

say "Initializing LocalStack"
./scripts/init-localstack.sh

say "Running DB migrations"
./scripts/db-migrate.sh

count=""
if ! count="$(listings_count)"; then
  die "Failed to read listing count from Postgres"
fi

if [[ "${count}" -gt 0 ]]; then
  say "Using existing DB data (${count} listings)"
else
  raw_key="$(latest_zolo_run_key || true)"
  if [[ -z "${raw_key}" ]]; then
    die "No listings in DB and no raw/zolo/.../run-*.jsonl objects found in s3://${RAW_BUCKET} for stack=${STACK}. If your data exists in another stack, run with STACK=... or run: make run-zolo"
  fi

  say "DB empty; found raw data at ${raw_key}"
  : > "${PARSER_LOG}"
  say "Running parser until SQS queue drains"
  run_parser_until_queue_drains

  if ! count="$(listings_count)"; then
    die "Failed to read listing count after parsing"
  fi
  if [[ "${count}" -le 0 ]]; then
    die "No listings found in DB after parsing. Ensure SQS has raw events or run a real scrape: make run-zolo"
  fi
fi

say "Building readapi"
go -C services/readapi build -o "${READAPI_BIN}" ./cmd/readapi

: > "${READAPI_LOG}"
say "Starting readapi on ${READAPI_BASE_URL}"
DB_ENABLED=true \
PORT="${READAPI_PORT}" \
POSTGRES_HOST="localhost" \
POSTGRES_PORT="${POSTGRES_PORT}" \
POSTGRES_DB="${POSTGRES_DB}" \
POSTGRES_USER="${POSTGRES_USER}" \
POSTGRES_PASSWORD="${POSTGRES_PASSWORD}" \
"${READAPI_BIN}" > "${READAPI_LOG}" 2>&1 &
READAPI_PID=$!

say "Waiting for readapi readiness"
if ! wait_for_readapi; then
  die "ReadAPI is not ready. See ${READAPI_LOG}"
fi

say "ReadAPI is ready at ${READAPI_BASE_URL}"
say "Starting Flutter client..."
cd apps/client
flutter run -d chrome --dart-define=READAPI_BASE_URL="${READAPI_BASE_URL}"
