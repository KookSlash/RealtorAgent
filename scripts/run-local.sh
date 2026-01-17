#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

LOG_DIR="${ROOT_DIR}/tmp/run-local"
mkdir -p "${LOG_DIR}"

say() {
  echo "[run-local] $*"
}

die() {
  echo "ERROR: $*" >&2
  echo "Logs: ${LOG_DIR}" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "Missing required command: $1"
}

require_cmd docker
require_cmd go
require_cmd flutter
require_cmd curl
require_cmd awslocal
if ! docker compose version >/dev/null 2>&1; then
  die "docker compose is required"
fi

if ! command -v lsof >/dev/null 2>&1; then
  die "lsof is required to check port availability"
fi

[[ -f infra/.env ]] || die "Missing infra/.env"
[[ -f infra/docker-compose.yml ]] || die "Missing infra/docker-compose.yml"
[[ -f infra/docker-compose.readapi.yml ]] || die "Missing infra/docker-compose.readapi.yml"
[[ -f scripts/init-localstack.sh ]] || die "Missing scripts/init-localstack.sh"
[[ -f scripts/db-migrate.sh ]] || die "Missing scripts/db-migrate.sh"

FIXTURE_PATH="scripts/fixtures/zolo_raw_minimal.jsonl"
[[ -f "${FIXTURE_PATH}" ]] || die "Missing fixture ${FIXTURE_PATH}"

set -a
source infra/.env
set +a

export AWS_REGION="${AWS_REGION:-us-west-2}"
export AWS_DEFAULT_REGION="${AWS_DEFAULT_REGION:-${AWS_REGION}}"

READAPI_PORT=8090
READAPI_BASE_URL="http://localhost:${READAPI_PORT}"
READAPI_LOG="${LOG_DIR}/readapi.log"
READAPI_PROBE_BODY="${LOG_DIR}/readapi_probe.json"

if lsof -nP -iTCP:${READAPI_PORT} -sTCP:LISTEN >/dev/null 2>&1; then
  die "port ${READAPI_PORT} is already in use; stop the process or choose another port"
fi

say "Logs directory: ${LOG_DIR}"

DATE_UTC="$(date -u +%F)"
RAW_KEY="raw/e2e/zolo/${DATE_UTC}/run-local.jsonl"
EXPECTED_LINES="$(wc -l < "${FIXTURE_PATH}" | tr -d ' ')"

say "Starting infra"
docker compose -f infra/docker-compose.yml up -d

say "Initializing LocalStack"
./scripts/init-localstack.sh

say "Running DB migrations"
./scripts/db-migrate.sh

say "Resetting DB state"
docker exec -i postgres psql -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -v ON_ERROR_STOP=1 -c \
  "TRUNCATE listings, price_history, processed_files, processing_attempts RESTART IDENTITY;"

say "Resetting SQS queue"
QUEUE_URL="$(awslocal sqs get-queue-url --queue-name "${RAW_EVENTS_QUEUE}" --query 'QueueUrl' --output text)"
if [[ -z "${QUEUE_URL}" || "${QUEUE_URL}" == "None" ]]; then
  die "Failed to resolve SQS queue URL for ${RAW_EVENTS_QUEUE}"
fi
if ! awslocal sqs purge-queue --queue-url "${QUEUE_URL}" >/dev/null 2>&1; then
  say "Purge unavailable; draining queue instead"
  while true; do
    RECEIPTS="$(awslocal sqs receive-message --queue-url "${QUEUE_URL}" --max-number-of-messages 10 --wait-time-seconds 1 --visibility-timeout 0 --query 'Messages[*].ReceiptHandle' --output text)"
    if [[ -z "${RECEIPTS}" || "${RECEIPTS}" == "None" ]]; then
      break
    fi
    for receipt in ${RECEIPTS}; do
      awslocal sqs delete-message --queue-url "${QUEUE_URL}" --receipt-handle "${receipt}" >/dev/null
    done
  done
fi

say "Resetting S3 prefixes"
awslocal s3 rm "s3://${RAW_BUCKET}/raw/e2e/" --recursive >/dev/null
awslocal s3 rm "s3://${VAULT_BUCKET}/vault/raw/raw/e2e/" --recursive >/dev/null
awslocal s3 rm "s3://${VAULT_BUCKET}/vault/normalized/raw/e2e/" --recursive >/dev/null
awslocal s3 rm "s3://${VAULT_BUCKET}/vault/errors/raw/e2e/" --recursive >/dev/null

say "Uploading fixture to raw bucket"
cat "${FIXTURE_PATH}" | docker compose -f infra/docker-compose.yml exec -T localstack sh -lc \
  "AWS_DEFAULT_REGION=${AWS_DEFAULT_REGION} AWS_REGION=${AWS_REGION} awslocal s3 cp - s3://${RAW_BUCKET}/${RAW_KEY}"

say "Running parser once"
(
  cd services/parser
  AWS_REGION="${AWS_REGION}" \
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
  go run ./cmd/parser
)

say "Starting readapi"
docker compose -f infra/docker-compose.yml -f infra/docker-compose.readapi.yml up -d --build readapi

say "Waiting for readapi readiness"
READAPI_STATUS="000"
for _ in {1..50}; do
  READAPI_STATUS="$(curl -s --max-time 1 -o "${READAPI_PROBE_BODY}" -w "%{http_code}" "${READAPI_BASE_URL}/v1/listings/count" 2>/dev/null || echo "000")"
  if [[ "${READAPI_STATUS}" == "200" ]]; then
    break
  fi
  sleep 0.2
done

if [[ "${READAPI_STATUS}" != "200" ]]; then
  docker compose -f infra/docker-compose.yml -f infra/docker-compose.readapi.yml logs --tail 200 readapi > "${READAPI_LOG}" 2>&1 || true
  BODY=""
  if [[ -f "${READAPI_PROBE_BODY}" ]]; then
    BODY="$(cat "${READAPI_PROBE_BODY}")"
  fi
  echo "ReadAPI readiness failed status=${READAPI_STATUS} body=${BODY}" >&2
  die "ReadAPI is not ready. See ${READAPI_LOG}"
fi

say "ReadAPI is ready at ${READAPI_BASE_URL}"

echo "Seeded ${EXPECTED_LINES} listings. Starting Flutter client..."
cd apps/client
flutter run -d chrome --dart-define=READAPI_BASE_URL=${READAPI_BASE_URL}
