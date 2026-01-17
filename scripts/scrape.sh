#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/stack-env.sh"
source "${ROOT_DIR}/scripts/lib/compose.sh"

LOG_DIR="${ROOT_DIR}/tmp/scrape"
mkdir -p "${LOG_DIR}"
SCRAPER_LOG="${LOG_DIR}/scraper.log"
PARSER_LOG="${LOG_DIR}/parser.log"

say() {
  echo "[scrape] $*"
}

die() {
  echo "ERROR: $*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "Missing required command: $1"
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
}

awslocal_exec() {
  local cmd="$1"
  compose_infra exec -T localstack sh -lc "AWS_PAGER=\"\" AWS_DEFAULT_REGION=${AWS_REGION} AWS_REGION=${AWS_REGION} ${cmd}"
}

require_cmd docker
require_cmd go
require_cmd curl

if ! docker compose version >/dev/null 2>&1; then
  die "docker compose is required"
fi

[[ -f infra/.env ]] || die "Missing infra/.env"
[[ -f infra/docker-compose.yml ]] || die "Missing infra/docker-compose.yml"
[[ -f scripts/init-localstack.sh ]] || die "Missing scripts/init-localstack.sh"
[[ -f scripts/db-migrate.sh ]] || die "Missing scripts/db-migrate.sh"

set -a
source infra/.env
set +a
stack_env

export AWS_REGION="${AWS_REGION:-us-west-2}"
export AWS_DEFAULT_REGION="${AWS_DEFAULT_REGION:-${AWS_REGION}}"
export AWS_PAGER=""

SCRAPER_STRATEGY="${SCRAPER_STRATEGY:-zolo_ca}"
if [[ "${SCRAPER_STRATEGY}" == "zolo_ca" && -z "${ZOLO_ENTRYPOINT_URL:-}" ]]; then
  ZOLO_ENTRYPOINT_URL="https://www.zolo.ca/index.php?sarea=Calgary&filter=1"
fi

say "Stack: ${STACK} (project $(compose_project))"
say "Starting infra"
compose_infra up -d --remove-orphans

say "Initializing LocalStack"
./scripts/init-localstack.sh

say "Running DB migrations"
./scripts/db-migrate.sh

: > "${SCRAPER_LOG}"
say "Running scraper (strategy=${SCRAPER_STRATEGY})"
(
  cd services/scraper
  AWS_REGION="${AWS_REGION}" \
  LOCALSTACK_ENDPOINT="http://localhost:${LOCALSTACK_PORT}" \
  RAW_BUCKET="${RAW_BUCKET}" \
  SCRAPER_STRATEGY="${SCRAPER_STRATEGY}" \
  ZOLO_ENTRYPOINT_URL="${ZOLO_ENTRYPOINT_URL:-}" \
  MAX_PAGES="${MAX_PAGES:-0}" \
  DRY_RUN=false \
  go run ./cmd/scraper
) | tee "${SCRAPER_LOG}"

: > "${PARSER_LOG}"
say "Running parser until SQS queue drains"
run_parser_until_queue_drains

listings_count="$(compose_infra exec -T postgres psql -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -t -A -c "SELECT COUNT(*) FROM listings" | tr -d '[:space:]')"
history_count="$(compose_infra exec -T postgres psql -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -t -A -c "SELECT COUNT(*) FROM price_history" | tr -d '[:space:]')"

say "Listings count: ${listings_count}"
say "Price history count: ${history_count}"
