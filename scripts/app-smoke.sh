#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

say() {
  echo "[app-smoke] $*"
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

say "Initializing LocalStack"
if ! run_make_target infra-init; then
  ./scripts/init-localstack.sh
fi

say "Running parser smoke"
bash scripts/test-parser.sh

say "Running scraper tests"
(cd services/scraper && go test ./... -count=1)

say "All app-smoke checks passed (parser smoke ran via scripts/test-parser.sh)"
