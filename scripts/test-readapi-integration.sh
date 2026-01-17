#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/stack-env.sh"
STACK="${STACK:-test}"
export STACK
stack_env
source "${ROOT_DIR}/scripts/lib/compose.sh"

say() {
  echo "[readapi-integration] $*"
}

if [[ "${STACK}" != "test" ]]; then
  echo "ERROR: refusing to run readapi integration tests on stack=${STACK}. Set STACK=test." >&2
  exit 1
fi
if [[ "${LOCALSTACK_PORT}" == "4566" || "${POSTGRES_PORT}" == "5432" ]]; then
  echo "ERROR: refusing to run readapi integration tests against RUN ports (LOCALSTACK_PORT=${LOCALSTACK_PORT}, POSTGRES_PORT=${POSTGRES_PORT})." >&2
  exit 1
fi

run_make_target() {
  local target="$1"
  if make -n "${target}" >/dev/null 2>&1; then
    make "${target}"
    return 0
  fi
  return 1
}

say "Starting infra"
if ! run_make_target infra-up; then
  # Remove orphans (like readapi) so infra stays clean and warnings are avoided.
  compose_infra up -d --remove-orphans
fi

say "Running DB migrations"
./scripts/db-migrate.sh

set -a
source infra/.env
set +a

export POSTGRES_HOST="localhost"
export POSTGRES_SSLMODE="disable"
export DB_ENABLED="true"

say "Running readapi integration tests"
(cd services/readapi && go test -tags=integration ./...)
