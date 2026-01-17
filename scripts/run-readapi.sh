#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/stack-env.sh"

[[ -f infra/.env ]] || { echo "Missing infra/.env" >&2; exit 1; }

set -a
source infra/.env
set +a
stack_env

export DB_ENABLED="true"
export PORT="${PORT:-8090}"
export POSTGRES_HOST="localhost"
export POSTGRES_PORT="${POSTGRES_PORT}"
export POSTGRES_DB="${POSTGRES_DB}"
export POSTGRES_USER="${POSTGRES_USER}"
export POSTGRES_PASSWORD="${POSTGRES_PASSWORD}"

go -C services/readapi run ./cmd/readapi
