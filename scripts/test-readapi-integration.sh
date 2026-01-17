#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

say() {
  echo "[readapi-integration] $*"
}

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
  docker compose -f infra/docker-compose.yml up -d
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
