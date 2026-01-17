#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/stack-env.sh"
STACK="${STACK:-run}"
export STACK
stack_env
source "${ROOT_DIR}/scripts/lib/compose.sh"

echo "== Docker containers =="
compose_infra ps

echo
echo "== LocalStack health =="
curl -sf http://localhost:4566/_localstack/health || true
echo
