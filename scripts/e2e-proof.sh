#!/usr/bin/env bash
# Debugging:
# - Set KEEP_TMP=1 to preserve temp files and logs after the run.
# - ReadAPI logs are written under TMP_DIR (printed after TMP_DIR is created).
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

say() {
  echo "[e2e-proof] $*"
}

die() {
  echo "ERROR: $*" >&2
  exit 1
}

E2E_SUCCESS=false
READAPI_LOG_TAILED=false

note_tmp_dir() {
  if [[ -n "${TMP_DIR:-}" && -d "${TMP_DIR}" ]]; then
    echo "[e2e-proof] TMP_DIR preserved at ${TMP_DIR}" >&2
  fi
}

dump_readapi_log() {
  if [[ -n "${READAPI_LOG:-}" && -f "${READAPI_LOG}" && "${READAPI_LOG_TAILED}" != "true" ]]; then
    echo "[e2e-proof] ReadAPI log tail:" >&2
    tail -n 200 "${READAPI_LOG}" >&2 || true
    READAPI_LOG_TAILED=true
  fi
}

ensure_readapi_port_available() {
  if ! command -v lsof >/dev/null 2>&1; then
    die "lsof is required to check port ${READAPI_PORT}"
  fi

  local lsof_out
  lsof_out="$(lsof -nP -iTCP:${READAPI_PORT} -sTCP:LISTEN 2>/dev/null || true)"
  if [[ -z "${lsof_out}" ]]; then
    return 0
  fi

  local cmd pid
  local -a readapi_pids=()
  local -a other_entries=()
  while read -r cmd pid _; do
    [[ -z "${cmd}" || "${cmd}" == "COMMAND" ]] && continue
    if [[ "${cmd}" == "readapi" ]]; then
      readapi_pids+=("${pid}")
    else
      other_entries+=("${cmd}:${pid}")
    fi
  done <<< "${lsof_out}"

  if (( ${#other_entries[@]} > 0 )); then
    die "port ${READAPI_PORT} is already in use by ${other_entries[*]}; set READAPI_PORT to a free port (e.g., READAPI_PORT=8091 make e2e-proof)"
  fi

  if (( ${#readapi_pids[@]} > 0 )); then
    say "Port ${READAPI_PORT} is in use by readapi (pid(s): ${readapi_pids[*]}), terminating"
    for pid in "${readapi_pids[@]}"; do
      kill "${pid}" >/dev/null 2>&1 || true
    done
    for _ in {1..10}; do
      if ! lsof -nP -iTCP:${READAPI_PORT} -sTCP:LISTEN >/dev/null 2>&1; then
        return 0
      fi
      sleep 0.2
    done
    die "port ${READAPI_PORT} is still in use after terminating readapi; set READAPI_PORT to a free port (e.g., READAPI_PORT=8091 make e2e-proof)"
  fi
}

cleanup() {
  if [[ "${E2E_SUCCESS}" != "true" ]]; then
    note_tmp_dir
    dump_readapi_log
  fi
  if [[ -n "${READAPI_PID:-}" ]]; then
    kill "${READAPI_PID}" >/dev/null 2>&1 || true
    wait "${READAPI_PID}" >/dev/null 2>&1 || true
  fi
  if [[ -n "${TMP_DIR:-}" && -d "${TMP_DIR}" ]]; then
    if [[ "${E2E_SUCCESS}" == "true" && "${KEEP_TMP:-}" != "1" ]]; then
      rm -rf "${TMP_DIR}"
    fi
  fi
}
trap cleanup EXIT

PYTHON_BIN=""
if command -v python3 >/dev/null 2>&1; then
  PYTHON_BIN="python3"
elif command -v python >/dev/null 2>&1; then
  PYTHON_BIN="python"
else
  die "python3 or python is required to parse JSON responses"
fi

[[ -f infra/.env ]] || die "Missing infra/.env"
[[ -f infra/docker-compose.yml ]] || die "Missing infra/docker-compose.yml"
[[ -f scripts/init-localstack.sh ]] || die "Missing scripts/init-localstack.sh"
[[ -f scripts/db-migrate.sh ]] || die "Missing scripts/db-migrate.sh"

FIXTURE_PATH="scripts/fixtures/zolo_raw_minimal.jsonl"
[[ -f "${FIXTURE_PATH}" ]] || die "Missing fixture ${FIXTURE_PATH}"

set -a
source infra/.env
set +a

export AWS_REGION="${AWS_REGION:-us-west-2}"
export AWS_DEFAULT_REGION="${AWS_DEFAULT_REGION:-${AWS_REGION}}"

DATE_UTC="$(date -u +%F)"
RAW_KEY="raw/e2e/zolo/${DATE_UTC}/run-e2e.jsonl"
EXPECTED_LINES="$(wc -l < "${FIXTURE_PATH}" | tr -d ' ')"
TMP_DIR="$(mktemp -d)"
READAPI_LOG="${TMP_DIR}/readapi.log"
READAPI_BIN="${TMP_DIR}/readapi"
READAPI_PORT="${READAPI_PORT:-8090}"
READAPI_BASE_URL="http://localhost:${READAPI_PORT}"
say "TMP_DIR=${TMP_DIR}"
say "READAPI_LOG=${READAPI_LOG}"
say "READAPI_BIN=${READAPI_BIN}"
say "READAPI_PORT=${READAPI_PORT}"

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

say "Validating processed_files row"
processed_count="$(docker exec -i postgres psql -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -t -A -c \
  "SELECT COUNT(*) FROM processed_files WHERE s3_bucket='${RAW_BUCKET}' AND s3_key='${RAW_KEY}'")"
processed_count="$(echo "${processed_count}" | tr -d '[:space:]')"
if [[ "${processed_count}" != "1" ]]; then
  die "expected 1 processed_files row for ${RAW_KEY}, got ${processed_count}"
fi

processed_row="$(docker exec -i postgres psql -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -t -A -c \
  "SELECT status, COALESCE(vault_raw_key,''), COALESCE(vault_normalized_key,''), COALESCE(vault_errors_key,'') FROM processed_files WHERE s3_bucket='${RAW_BUCKET}' AND s3_key='${RAW_KEY}'")"
processed_row="$(echo "${processed_row}" | tr -d '\r' | tr -d '\n')"
IFS='|' read -r status vault_raw_key vault_norm_key vault_err_key <<< "${processed_row}"
status="$(echo "${status}" | tr -d '[:space:]')"

if [[ "${status}" != "PROCESSED" ]]; then
  die "processed_files status is not PROCESSED: ${status}"
fi
[[ -n "${vault_raw_key}" ]] || die "vault_raw_key is empty"
[[ -n "${vault_norm_key}" ]] || die "vault_normalized_key is empty"
[[ -n "${vault_err_key}" ]] || die "vault_errors_key is empty"

say "Checking vault objects exist"
awslocal s3api head-object --bucket "${VAULT_BUCKET}" --key "${vault_raw_key}" >/dev/null
awslocal s3api head-object --bucket "${VAULT_BUCKET}" --key "${vault_norm_key}" >/dev/null
awslocal s3api head-object --bucket "${VAULT_BUCKET}" --key "${vault_err_key}" >/dev/null

listings_count="$(docker exec -i postgres psql -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -t -A -c "SELECT COUNT(*) FROM listings")"
listings_count="$(echo "${listings_count}" | tr -d '[:space:]')"
if [[ "${listings_count}" != "${EXPECTED_LINES}" ]]; then
  die "expected listings count ${EXPECTED_LINES}, got ${listings_count}"
fi

price_history_count="$(docker exec -i postgres psql -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -t -A -c "SELECT COUNT(*) FROM price_history")"
price_history_count="$(echo "${price_history_count}" | tr -d '[:space:]')"
if [[ "${price_history_count}" -lt "${EXPECTED_LINES}" ]]; then
  die "expected price_history count >= ${EXPECTED_LINES}, got ${price_history_count}"
fi
if [[ "${price_history_count}" != "${EXPECTED_LINES}" ]]; then
  say "price_history count ${price_history_count} (>= ${EXPECTED_LINES})"
fi

norm_file="${TMP_DIR}/normalized.jsonl"
err_file="${TMP_DIR}/errors.jsonl"

say "Validating vault artifact contents"
awslocal s3 cp "s3://${VAULT_BUCKET}/${vault_norm_key}" "${norm_file}" >/dev/null
awslocal s3 cp "s3://${VAULT_BUCKET}/${vault_err_key}" "${err_file}" >/dev/null

norm_lines="$(wc -l < "${norm_file}" | tr -d ' ')"
err_lines="$(wc -l < "${err_file}" | tr -d ' ')"

if [[ "${norm_lines}" != "${EXPECTED_LINES}" ]]; then
  die "expected normalized lines ${EXPECTED_LINES}, got ${norm_lines}"
fi
if [[ "${err_lines}" != "0" ]]; then
  die "expected errors lines 0, got ${err_lines}"
fi

say "Starting readapi"
ensure_readapi_port_available
say "Building readapi binary"
(
  cd services/readapi
  go build -o "${READAPI_BIN}" ./cmd/readapi
)
: > "${READAPI_LOG}"
PORT="${READAPI_PORT}" \
DB_ENABLED=true \
POSTGRES_HOST="localhost" \
POSTGRES_PORT="${POSTGRES_PORT}" \
POSTGRES_DB="${POSTGRES_DB}" \
POSTGRES_USER="${POSTGRES_USER}" \
POSTGRES_PASSWORD="${POSTGRES_PASSWORD}" \
POSTGRES_SSLMODE="disable" \
"${READAPI_BIN}" > "${READAPI_LOG}" 2>&1 &
READAPI_PID=$!
if ! kill -0 "${READAPI_PID}" >/dev/null 2>&1; then
  echo "[e2e-proof] ReadAPI process exited immediately after start" >&2
  dump_readapi_log
  die "readapi failed to start"
fi

say "Waiting for readapi readiness"
READAPI_PROBE_URL="${READAPI_BASE_URL}/v1/listings/count"
READAPI_PROBE_BODY="${TMP_DIR}/readapi-probe.json"
READAPI_STATUS="000"
probe_readapi() {
  curl -s --max-time 1 -o "${READAPI_PROBE_BODY}" -w "%{http_code}" "${READAPI_PROBE_URL}" 2>/dev/null || echo "000"
}
for _ in {1..50}; do
  READAPI_STATUS="$(probe_readapi)"
  if [[ "${READAPI_STATUS}" == "200" ]]; then
    break
  fi
  sleep 0.2
done
if ! kill -0 "${READAPI_PID}" >/dev/null 2>&1; then
  READAPI_BODY=""
  if [[ -f "${READAPI_PROBE_BODY}" ]]; then
    READAPI_BODY="$(cat "${READAPI_PROBE_BODY}")"
  fi
  echo "[e2e-proof] ReadAPI process exited before readiness url=${READAPI_PROBE_URL} status=${READAPI_STATUS} body=${READAPI_BODY}" >&2
  dump_readapi_log
  die "readapi failed to start"
fi
if [[ "${READAPI_STATUS}" != "200" ]]; then
  READAPI_BODY=""
  if [[ -f "${READAPI_PROBE_BODY}" ]]; then
    READAPI_BODY="$(cat "${READAPI_PROBE_BODY}")"
  fi
  echo "[e2e-proof] ReadAPI readiness failed url=${READAPI_PROBE_URL} status=${READAPI_STATUS} body=${READAPI_BODY}" >&2
  dump_readapi_log
  die "readapi failed to become ready"
fi

count_body_file="${TMP_DIR}/readapi-count.json"
count_status="$(curl -sS -o "${count_body_file}" -w "%{http_code}" "${READAPI_PROBE_URL}" || echo "000")"
count_raw="$(cat "${count_body_file}")"
if [[ "${count_status}" != "200" ]]; then
  echo "[e2e-proof] ReadAPI count failed url=${READAPI_PROBE_URL} status=${count_status} body=${count_raw}" >&2
  dump_readapi_log
  die "readapi count request failed"
fi
if ! count_value="$("${PYTHON_BIN}" -c 'import json,sys; data=json.load(sys.stdin); value=data.get("count"); num=(int,); num=(int, long) if "long" in dir(__builtins__) else (int,); sys.exit(1) if not isinstance(value,num) else None; print(value)' < "${count_body_file}")"; then
  die "readapi count response invalid: ${count_raw}"
fi
if [[ "${count_value}" -lt "1" ]]; then
  die "readapi returned count < 1: ${count_value} raw=${count_raw}"
fi
if [[ "${count_value}" != "${EXPECTED_LINES}" ]]; then
  die "readapi count mismatch expected=${EXPECTED_LINES} actual=${count_value} raw=${count_raw}"
fi

browse_url="${READAPI_BASE_URL}/v1/listings?limit=10&offset=0"
browse_body_file="${TMP_DIR}/readapi-browse.json"
browse_status="$(curl -sS -o "${browse_body_file}" -w "%{http_code}" "${browse_url}" || echo "000")"
browse_raw="$(cat "${browse_body_file}")"
if [[ "${browse_status}" != "200" ]]; then
  echo "[e2e-proof] ReadAPI browse failed url=${browse_url} status=${browse_status} body=${browse_raw}" >&2
  diag_count_file="${TMP_DIR}/readapi-count-diag.json"
  diag_count_status="$(curl -sS -o "${diag_count_file}" -w "%{http_code}" "${READAPI_PROBE_URL}" || echo "000")"
  diag_count_body=""
  if [[ -f "${diag_count_file}" ]]; then
    diag_count_body="$(cat "${diag_count_file}")"
  fi
  echo "[e2e-proof] ReadAPI count diag url=${READAPI_PROBE_URL} status=${diag_count_status} body=${diag_count_body}" >&2
  dump_readapi_log
  die "readapi browse request failed"
fi
if ! browse_summary="$("${PYTHON_BIN}" -c 'import json,sys; data=json.load(sys.stdin); items=data.get("items"); returned=data.get("returned"); num=(int,); num=(int, long) if "long" in dir(__builtins__) else (int,); sys.exit(1) if not isinstance(items, list) or not isinstance(returned, num) else None; print("{}|{}".format(len(items), returned))' < "${browse_body_file}")"; then
  die "readapi browse response invalid: ${browse_raw}"
fi
IFS='|' read -r browse_items browse_returned <<< "${browse_summary}"
if [[ "${browse_items}" != "${EXPECTED_LINES}" ]]; then
  die "readapi browse items mismatch expected=${EXPECTED_LINES} actual=${browse_items} raw=${browse_raw}"
fi
if [[ "${browse_returned}" != "${EXPECTED_LINES}" ]]; then
  die "readapi browse returned mismatch expected=${EXPECTED_LINES} actual=${browse_returned} raw=${browse_raw}"
fi

E2E_SUCCESS=true
say "E2E proof complete (listings=${listings_count}, price_history=${price_history_count}, readapi_count=${count_value}, readapi_browse=${browse_items})"
