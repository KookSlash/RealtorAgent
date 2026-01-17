#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
STACK="${STACK:-test}"
export STACK
source "${ROOT_DIR}/scripts/lib/stack-env.sh"
stack_env

# Load infra env (buckets/queue/region/db) if present
if [[ -f "${ROOT_DIR}/infra/.env" ]]; then
  set -a
  source "${ROOT_DIR}/infra/.env"
  set +a
fi
stack_env

: "${AWS_REGION:=us-west-2}"
: "${LOCALSTACK_ENDPOINT:=http://localhost:4566}"
: "${RAW_BUCKET:=calgary-raw-bucket}"
: "${VAULT_BUCKET:=calgary-vault-bucket}"
: "${RAW_EVENTS_QUEUE:=calgary-raw-events-queue}"
: "${MAX_PAGES:=20}"
: "${RATE_LIMIT_MS:=500}"
: "${DRY_RUN:=false}"
: "${MAX_RETRIES:=5}"

if [[ -z "${SEARCH_ENTRYPOINT_URL:-}" ]]; then
  echo "ERROR: SEARCH_ENTRYPOINT_URL is required."
  echo "Example:"
  echo "  export SEARCH_ENTRYPOINT_URL=\"https://www.realtor.ca/ab/calgary/real-estate\""
  exit 1
fi

echo "== 1) Start infra =="
make -C "${ROOT_DIR}" infra-up
make -C "${ROOT_DIR}" infra-init
make -C "${ROOT_DIR}" db-migrate

echo "== 2) Run scraper (writes JSONL + uploads to RAW bucket) =="
pushd "${ROOT_DIR}/services/scraper" >/dev/null
POSTGRES_HOST="${POSTGRES_HOST}" \
POSTGRES_PORT="${POSTGRES_PORT}" \
POSTGRES_USER="${POSTGRES_USER}" \
POSTGRES_PASSWORD="${POSTGRES_PASSWORD}" \
POSTGRES_DB="${POSTGRES_DB}" \
AWS_REGION="${AWS_REGION}" \
LOCALSTACK_ENDPOINT="${LOCALSTACK_ENDPOINT}" \
RAW_BUCKET="${RAW_BUCKET}" \
MAX_PAGES="${MAX_PAGES}" \
RATE_LIMIT_MS="${RATE_LIMIT_MS}" \
DRY_RUN="${DRY_RUN}" \
SEARCH_ENTRYPOINT_URL="${SEARCH_ENTRYPOINT_URL}" \
go run ./cmd/scraper
popd >/dev/null

echo "== 3) Peek SQS (should contain ObjectCreated events; may include TestEvent) =="
QUEUE_URL="$(awslocal sqs get-queue-url --queue-name "${RAW_EVENTS_QUEUE}" --query QueueUrl --output text)"
awslocal sqs receive-message --queue-url "${QUEUE_URL}" --max-number-of-messages 5 >/tmp/e2e_sqs_peek.json || true
echo "SQS peek saved to /tmp/e2e_sqs_peek.json"
echo "(Parser will ignore TestEvent and process ObjectCreated records.)"

# DB defaults for parser
: "${POSTGRES_HOST:=localhost}"
: "${POSTGRES_PORT:=5432}"
: "${POSTGRES_USER:=postgres}"
: "${POSTGRES_PASSWORD:=postgres}"
: "${POSTGRES_DB:=realtor}"

echo "== 4) Run parser ONCE (consume queue, write DB, archive to vault) =="
# We rely on the parser supporting ONCE=true (or equivalent). If your parser uses a different flag, adjust here.
pushd "${ROOT_DIR}/services/parser" >/dev/null
POSTGRES_HOST="${POSTGRES_HOST}" \\
POSTGRES_PORT="${POSTGRES_PORT}" \\
POSTGRES_USER="${POSTGRES_USER}" \\
POSTGRES_PASSWORD="${POSTGRES_PASSWORD}" \\
POSTGRES_DB="${POSTGRES_DB}" \\
POSTGRES_HOST="${POSTGRES_HOST}" \
POSTGRES_PORT="${POSTGRES_PORT}" \
POSTGRES_USER="${POSTGRES_USER}" \
POSTGRES_PASSWORD="${POSTGRES_PASSWORD}" \
POSTGRES_DB="${POSTGRES_DB}" \
AWS_REGION="${AWS_REGION}" \
LOCALSTACK_ENDPOINT="${LOCALSTACK_ENDPOINT}" \
RAW_BUCKET="${RAW_BUCKET}" \
VAULT_BUCKET="${VAULT_BUCKET}" \
RAW_EVENTS_QUEUE="${RAW_EVENTS_QUEUE}" \
MAX_RETRIES="${MAX_RETRIES}" \
ONCE="true" \
go run ./cmd/parser
popd >/dev/null

echo "== 5) Basic DB checks =="
# Use envs from infra/.env if available; otherwise fall back to common defaults.
: "${POSTGRES_USER:=postgres}"
: "${POSTGRES_PASSWORD:=postgres}"
: "${POSTGRES_DB:=realtor}"
: "${POSTGRES_HOST:=localhost}"
: "${POSTGRES_PORT:=5432}"

echo "Listings count:"
PGPASSWORD="${POSTGRES_PASSWORD}" psql -h "${POSTGRES_HOST}" -p "${POSTGRES_PORT}" -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -tAc "select count(*) from listings;" || true

echo "Price history count:"
PGPASSWORD="${POSTGRES_PASSWORD}" psql -h "${POSTGRES_HOST}" -p "${POSTGRES_PORT}" -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -tAc "select count(*) from price_history;" || true

echo "Processed files (latest 5):"
PGPASSWORD="${POSTGRES_PASSWORD}" psql -h "${POSTGRES_HOST}" -p "${POSTGRES_PORT}" -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -c "select s3_bucket,s3_key,etag,status,processed_at from processed_files order by processed_at desc limit 5;" || true

echo "== 6) Vault sanity check (list last few objects) =="
awslocal s3 ls "s3://${VAULT_BUCKET}/vault/" --recursive | tail -n 20 || true

echo "== DONE =="
echo "If counts are > 0 and vault contains artifacts, the E2E pipeline works."
