#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/stack-env.sh"
STACK="${STACK:-test}"
export STACK
source "${ROOT_DIR}/scripts/lib/compose.sh"

if [[ ! -f infra/.env ]]; then
  echo "Missing infra/.env (required for LocalStack/Postgres settings)." >&2
  exit 1
fi

set -a
source infra/.env
set +a
export AWS_PAGER=""
stack_env

if [[ "${STACK}" != "test" ]]; then
  echo "ERROR: refusing to run test-parser on stack=${STACK}. Set STACK=test." >&2
  exit 1
fi
if [[ "${LOCALSTACK_PORT}" == "4566" || "${POSTGRES_PORT}" == "5432" ]]; then
  echo "ERROR: refusing to run test-parser against RUN ports (LOCALSTACK_PORT=${LOCALSTACK_PORT}, POSTGRES_PORT=${POSTGRES_PORT})." >&2
  exit 1
fi

health_url="http://localhost:${LOCALSTACK_PORT}/_localstack/health"
if ! curl -sf "${health_url}" >/dev/null; then
  echo "LocalStack is not reachable at ${health_url}. Run: make infra-up" >&2
  exit 1
fi

if ! compose_infra exec -T postgres pg_isready -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" >/dev/null 2>&1; then
  echo "Postgres is not ready. Run: make infra-up" >&2
  exit 1
fi

if ! awslocal s3api head-bucket --bucket "${RAW_BUCKET}" >/dev/null 2>&1; then
  echo "RAW_BUCKET not found. Run: make infra-init" >&2
  exit 1
fi

if ! awslocal s3api head-bucket --bucket "${VAULT_BUCKET}" >/dev/null 2>&1; then
  echo "VAULT_BUCKET not found. Run: make infra-init" >&2
  exit 1
fi

if ! awslocal sqs get-queue-url --queue-name "${RAW_EVENTS_QUEUE}" >/dev/null 2>&1; then
  echo "RAW_EVENTS_QUEUE not found. Run: make infra-init" >&2
  exit 1
fi

AWS_PAGER="" make db-migrate

# Create a dedicated test object
DATE_STR="$(date -u +%Y-%m-%d)"
TEST_KEY="raw/realtorca/${DATE_STR}/run-test.jsonl"
TMP_FILE="$(mktemp)"
trap 'rm -f "${TMP_FILE}"' EXIT

cat > "${TMP_FILE}" <<'JSONL'
{"listing_id":"TEST-1","address":"1 Test St","postal_code":"T2P 0X0","property_type":"House","price":100000,"beds":1,"baths":1,"sqft":500,"lat":51.0,"lon":-114.0,"url":"http://example.com/test","scraped_at":"2025-01-01T00:00:00Z"}
JSONL

awslocal s3 cp "${TMP_FILE}" "s3://${RAW_BUCKET}/${TEST_KEY}" >/dev/null

# Ensure an SQS message is present (receive once)
QUEUE_URL="$(awslocal sqs get-queue-url --queue-name "${RAW_EVENTS_QUEUE}" --query 'QueueUrl' --output text)"
RECEIVED="$(awslocal sqs receive-message --queue-url "${QUEUE_URL}" --max-number-of-messages 1 --wait-time-seconds 2 --visibility-timeout 0 --query 'Messages[0].[Body,ReceiptHandle]' --output text)"
if [[ -z "${RECEIVED}" || "${RECEIVED}" == "None" ]]; then
  echo "No SQS message received after uploading ${TEST_KEY}." >&2
  exit 1
fi

IFS=$'\t' read -r BODY RECEIPT <<< "${RECEIVED}"
if [[ -n "${RECEIPT}" && "${RECEIPT}" != "None" ]]; then
  awslocal sqs delete-message --queue-url "${QUEUE_URL}" --receipt-handle "${RECEIPT}" >/dev/null
fi

GOPATH_DIR="$(go env GOPATH)"

docker run --rm \
  --network calgary_net \
  -v "${PWD}:/repo" \
  -v "${GOPATH_DIR}/pkg/mod:/go/pkg/mod" \
  -v "${GOPATH_DIR}/pkg/sumdb:/go/pkg/sumdb" \
  -w /repo/services/parser \
  -e AWS_REGION="${AWS_REGION}" \
  -e LOCALSTACK_ENDPOINT="http://localstack:4566" \
  -e RAW_BUCKET="${RAW_BUCKET}" \
  -e VAULT_BUCKET="${VAULT_BUCKET}" \
  -e RAW_EVENTS_QUEUE="${RAW_EVENTS_QUEUE}" \
  -e POSTGRES_HOST=postgres \
  -e POSTGRES_PORT="${POSTGRES_PORT}" \
  -e POSTGRES_DB="${POSTGRES_DB}" \
  -e POSTGRES_USER="${POSTGRES_USER}" \
  -e POSTGRES_PASSWORD="${POSTGRES_PASSWORD}" \
  -e MAX_RETRIES=3 \
  -e DRY_RUN=false \
  -e LOG_LEVEL=info \
  golang:1.22 go test ./...
