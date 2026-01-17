#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
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
stack_env

required_vars=(LOCALSTACK_PORT AWS_REGION RAW_BUCKET VAULT_BUCKET RAW_EVENTS_QUEUE POSTGRES_USER POSTGRES_DB POSTGRES_PASSWORD POSTGRES_PORT)
for var in "${required_vars[@]}"; do
  if [[ -z "${!var:-}" ]]; then
    echo "Missing required env var: ${var}" >&2
    exit 1
  fi
done

if [[ "${STACK}" != "test" ]]; then
  echo "ERROR: refusing to run test-e2e-zolo on stack=${STACK}. Set STACK=test." >&2
  exit 1
fi
if [[ "${LOCALSTACK_PORT}" == "4566" || "${POSTGRES_PORT}" == "5432" ]]; then
  echo "ERROR: refusing to run test-e2e-zolo against RUN ports (LOCALSTACK_PORT=${LOCALSTACK_PORT}, POSTGRES_PORT=${POSTGRES_PORT})." >&2
  exit 1
fi

export AWS_PAGER=""
export AWS_DEFAULT_REGION="${AWS_REGION}"
export LOCALSTACK_ENDPOINT="http://localhost:${LOCALSTACK_PORT}"

pg_exec() {
  local sql="$1"
  compose_infra exec -T -e PGPASSWORD="${POSTGRES_PASSWORD}" postgres \
    psql -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -t -A -c "${sql}"
}

aws_ls() {
  awslocal "$@"
}

wait_localstack() {
  local health_url="${LOCALSTACK_ENDPOINT}/_localstack/health"
  for _ in {1..60}; do
    health="$(curl -sf "${health_url}" 2>/dev/null || true)"
    if [[ -n "${health}" ]] && \
      printf '%s' "${health}" | grep -Eq '"s3"[[:space:]]*:[[:space:]]*"(running|available)"' && \
      printf '%s' "${health}" | grep -Eq '"sqs"[[:space:]]*:[[:space:]]*"(running|available)"'; then
      return 0
    fi
    sleep 1
  done
  echo "LocalStack is not reachable or unhealthy at ${health_url}." >&2
  compose_infra logs --tail 200 localstack >&2 || true
  exit 1
}

wait_postgres() {
  for _ in {1..30}; do
    if compose_infra exec -T postgres pg_isready -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "Postgres is not ready." >&2
  compose_infra logs --tail 200 postgres >&2 || true
  exit 1
}

fixture_pid=""
fixture_log=""

cleanup() {
  if [[ -n "${fixture_pid}" ]] && kill -0 "${fixture_pid}" >/dev/null 2>&1; then
    kill "${fixture_pid}" >/dev/null 2>&1 || true
    wait "${fixture_pid}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

mkdir -p "${ROOT_DIR}/tmp"

echo "[e2e] Starting infra"
make infra-up

if ! compose_infra wait --timeout 120 localstack postgres >/dev/null 2>&1; then
  wait_localstack
  wait_postgres
fi

echo "[e2e] Initializing LocalStack + migrations"
./scripts/init-localstack.sh
./scripts/db-migrate.sh

queue_url="$(aws_ls sqs get-queue-url --queue-name "${RAW_EVENTS_QUEUE}" --query 'QueueUrl' --output text)"
if [[ -z "${queue_url}" || "${queue_url}" == "None" ]]; then
  echo "RAW_EVENTS_QUEUE missing after init" >&2
  exit 1
fi

echo "[e2e] Draining SQS queue"
for _ in {1..20}; do
  receipts="$(aws_ls sqs receive-message --queue-url "${queue_url}" --max-number-of-messages 10 --wait-time-seconds 1 --query 'Messages[*].ReceiptHandle' --output text)"
  if [[ -z "${receipts}" || "${receipts}" == "None" ]]; then
    break
  fi
  for receipt in ${receipts}; do
    aws_ls sqs delete-message --queue-url "${queue_url}" --receipt-handle "${receipt}" >/dev/null
  done
done

fixture_log="${ROOT_DIR}/tmp/fixture-zolo.log"
(
  cd services/scraper
  go run ./testdata/e2e_zolo/fixture_server.go
) >"${fixture_log}" 2>&1 &
fixture_pid=$!

fixture_base_url=""
for _ in {1..50}; do
  if [[ -f "${fixture_log}" ]]; then
    line="$(grep -m1 '^FIXTURE_BASE_URL=' "${fixture_log}" || true)"
    if [[ -n "${line}" ]]; then
      fixture_base_url="${line#FIXTURE_BASE_URL=}"
      break
    fi
  fi
  if ! kill -0 "${fixture_pid}" >/dev/null 2>&1; then
    echo "Fixture server exited early." >&2
    tail -n 200 "${fixture_log}" >&2 || true
    exit 1
  fi
  sleep 0.1
done

if [[ -z "${fixture_base_url}" ]]; then
  echo "Fixture server did not report a base URL." >&2
  tail -n 200 "${fixture_log}" >&2 || true
  exit 1
fi

echo "[e2e] Fixture base URL: ${fixture_base_url}"

ZOLO_BASE_URL="${fixture_base_url}"
ZOLO_ENTRYPOINT_URL="${fixture_base_url}/calgary-real-estate"

if ! curl -sf "${ZOLO_ENTRYPOINT_URL}" >/dev/null; then
  echo "Fixture entrypoint not reachable: ${ZOLO_ENTRYPOINT_URL}" >&2
  exit 1
fi

run_id="e2e-$(date -u +%Y%m%dT%H%M%SZ)-$$"
scrape_date="2000-01-01"

scraper_log="${ROOT_DIR}/tmp/scraper-e2e-${run_id}.log"

echo "[e2e] Running scraper"
set +e
(
  cd services/scraper
  AWS_REGION="${AWS_REGION}" \
  LOCALSTACK_ENDPOINT="${LOCALSTACK_ENDPOINT}" \
  RAW_BUCKET="${RAW_BUCKET}" \
  SCRAPER_STRATEGY="zolo_ca" \
  FETCH_MODE="http" \
  DRY_RUN="false" \
  MAX_PAGES="5" \
  RATE_LIMIT_MS="0" \
  ZOLO_BASE_URL="${ZOLO_BASE_URL}" \
  ZOLO_ENTRYPOINT_URL="${ZOLO_ENTRYPOINT_URL}" \
  RUN_ID="${run_id}" \
  SCRAPE_DATE="${scrape_date}" \
  go run ./cmd/scraper
) >"${scraper_log}" 2>&1
scraper_rc=$?
set -e
if [[ ${scraper_rc} -ne 0 ]]; then
  echo "Scraper failed with exit code ${scraper_rc}. Log:" >&2
  tail -n 200 "${scraper_log}" >&2
  exit 1
fi

records_extracted="$(awk -F= '/^records_extracted=/{print $2}' "${scraper_log}" | tail -n 1 | tr -d '[:space:]')"
output_key="$(awk -F= '/^output_key=/{print $2}' "${scraper_log}" | tail -n 1 | tr -d '[:space:]')"

if [[ -z "${records_extracted}" || "${records_extracted}" != "3" ]]; then
  echo "Expected records_extracted=3, got ${records_extracted}" >&2
  tail -n 200 "${scraper_log}" >&2
  exit 1
fi
if [[ -z "${output_key}" ]]; then
  echo "Missing output_key from scraper output" >&2
  tail -n 200 "${scraper_log}" >&2
  exit 1
fi

echo "[e2e] Scraper records_extracted=${records_extracted} output_key=${output_key}"

encoded_key="${output_key//\//%2F}"
found_msg=""
receipt_handle=""

for _ in {1..20}; do
  msg="$(aws_ls sqs receive-message --queue-url "${queue_url}" --max-number-of-messages 1 --wait-time-seconds 1 --visibility-timeout 5 --query 'Messages[0].[Body,ReceiptHandle]' --output text)"
  if [[ -z "${msg}" || "${msg}" == "None" ]]; then
    sleep 1
    continue
  fi
  IFS=$'\t' read -r body receipt <<<"${msg}"
  if [[ -n "${body}" ]] && (printf '%s' "${body}" | grep -q "${output_key}" || printf '%s' "${body}" | grep -q "${encoded_key}"); then
    found_msg="${body}"
    receipt_handle="${receipt}"
    break
  fi
  if [[ -n "${receipt}" && "${receipt}" != "None" ]]; then
    aws_ls sqs change-message-visibility --queue-url "${queue_url}" --receipt-handle "${receipt}" --visibility-timeout 0 >/dev/null
  fi
  sleep 1
done

if [[ -z "${found_msg}" ]]; then
  echo "Did not find SQS message for ${output_key}" >&2
  exit 1
fi
if [[ -n "${receipt_handle}" && "${receipt_handle}" != "None" ]]; then
  aws_ls sqs change-message-visibility --queue-url "${queue_url}" --receipt-handle "${receipt_handle}" --visibility-timeout 0 >/dev/null
fi

fixture_addresses=(
  "111 Test Ave NW, Calgary, AB T2P 1A1"
  "222 Sunrise Rd SW, Calgary, AB"
  "333 Riverwalk Ct SE, Calgary, AB T2E 2B2"
)
addr_list=""
for addr in "${fixture_addresses[@]}"; do
  escaped="${addr//\'/\'\'}"
  if [[ -z "${addr_list}" ]]; then
    addr_list="'${escaped}'"
  else
    addr_list+=", '${escaped}'"
  fi
done

echo "[e2e] Clearing prior fixture listings (if any)"
pg_exec "DELETE FROM listings WHERE address IN (${addr_list});" >/dev/null

listings_before="$(pg_exec "SELECT COUNT(*) FROM listings;")"
price_before="$(pg_exec "SELECT COUNT(*) FROM price_history;")"
listings_before="${listings_before//[[:space:]]/}"
price_before="${price_before//[[:space:]]/}"

parser_log="${ROOT_DIR}/tmp/parser-e2e-${run_id}.log"

echo "[e2e] Running parser"
set +e
(
  cd services/parser
  AWS_REGION="${AWS_REGION}" \
  RAW_BUCKET="${RAW_BUCKET}" \
  VAULT_BUCKET="${VAULT_BUCKET}" \
  RAW_EVENTS_QUEUE="${RAW_EVENTS_QUEUE}" \
  LOCALSTACK_ENDPOINT="${LOCALSTACK_ENDPOINT}" \
  POSTGRES_HOST="localhost" \
  POSTGRES_PORT="${POSTGRES_PORT}" \
  POSTGRES_DB="${POSTGRES_DB}" \
  POSTGRES_USER="${POSTGRES_USER}" \
  POSTGRES_PASSWORD="${POSTGRES_PASSWORD}" \
  ONCE="true" \
  DRY_RUN="false" \
  LOG_LEVEL="info" \
  go run ./cmd/parser
) >"${parser_log}" 2>&1
parser_rc=$?
set -e
if [[ ${parser_rc} -ne 0 ]]; then
  echo "Parser failed with exit code ${parser_rc}. Log:" >&2
  tail -n 200 "${parser_log}" >&2
  exit 1
fi

processed_row="$(pg_exec "SELECT status, COALESCE(vault_raw_key,''), COALESCE(vault_normalized_key,''), COALESCE(vault_errors_key,'') FROM processed_files WHERE s3_bucket='${RAW_BUCKET}' AND s3_key='${output_key}'")"
processed_row="${processed_row//$'\n'/}"
if [[ -z "${processed_row}" ]]; then
  echo "processed_files row not found for ${output_key}" >&2
  exit 1
fi

IFS='|' read -r status vault_raw_key vault_norm_key vault_err_key <<<"${processed_row}"
status="${status//[[:space:]]/}"

expected_vault_raw="vault/raw/${output_key}"
expected_vault_norm="vault/normalized/${output_key}.normalized.jsonl"
expected_vault_err="vault/errors/${output_key}.errors.jsonl"

if [[ "${status}" != "PROCESSED" ]]; then
  echo "processed_files status is not PROCESSED: ${status}" >&2
  exit 1
fi
if [[ -z "${vault_raw_key}" || -z "${vault_norm_key}" || -z "${vault_err_key}" ]]; then
  echo "processed_files vault keys missing" >&2
  exit 1
fi
if [[ "${vault_raw_key}" != "${expected_vault_raw}" || "${vault_norm_key}" != "${expected_vault_norm}" || "${vault_err_key}" != "${expected_vault_err}" ]]; then
  echo "processed_files vault keys do not match expected conventions" >&2
  echo "expected_raw=${expected_vault_raw}" >&2
  echo "expected_norm=${expected_vault_norm}" >&2
  echo "expected_err=${expected_vault_err}" >&2
  echo "db_raw=${vault_raw_key}" >&2
  echo "db_norm=${vault_norm_key}" >&2
  echo "db_err=${vault_err_key}" >&2
  exit 1
fi

aws_ls s3api head-object --bucket "${VAULT_BUCKET}" --key "${expected_vault_raw}" >/dev/null
aws_ls s3api head-object --bucket "${VAULT_BUCKET}" --key "${expected_vault_norm}" >/dev/null
aws_ls s3api head-object --bucket "${VAULT_BUCKET}" --key "${expected_vault_err}" >/dev/null

vault_norm_file="${ROOT_DIR}/tmp/vault-norm-${run_id}.jsonl"
vault_err_file="${ROOT_DIR}/tmp/vault-err-${run_id}.jsonl"
aws_ls s3 cp "s3://${VAULT_BUCKET}/${expected_vault_norm}" "${vault_norm_file}" >/dev/null
aws_ls s3 cp "s3://${VAULT_BUCKET}/${expected_vault_err}" "${vault_err_file}" >/dev/null

normalized_count="$(wc -l < "${vault_norm_file}" | tr -d '[:space:]')"
if [[ "${normalized_count}" != "3" ]]; then
  echo "Expected 3 normalized lines, got ${normalized_count}" >&2
  exit 1
fi

err_count="$(wc -l < "${vault_err_file}" | tr -d '[:space:]')"
if [[ -z "${err_count}" ]]; then
  err_count=0
fi
if [[ "${err_count}" != "0" ]]; then
  echo "Expected 0 error lines, got ${err_count}" >&2
  exit 1
fi

listings_after="$(pg_exec "SELECT COUNT(*) FROM listings;")"
price_after="$(pg_exec "SELECT COUNT(*) FROM price_history;")"
listings_after="${listings_after//[[:space:]]/}"
price_after="${price_after//[[:space:]]/}"

listings_delta=$((listings_after - listings_before))
price_delta=$((price_after - price_before))

if [[ "${listings_delta}" -ne 3 ]]; then
  echo "Expected listings delta 3, got ${listings_delta}" >&2
  exit 1
fi
if [[ "${price_delta}" -ne 3 ]]; then
  echo "Expected price_history delta 3, got ${price_delta}" >&2
  exit 1
fi

echo "[e2e] Vault normalized_lines=${normalized_count} errors=${err_count}"
echo "[e2e] DB delta listings=${listings_delta} price_history=${price_delta}"
echo "[e2e] PASS output_key=${output_key}"
