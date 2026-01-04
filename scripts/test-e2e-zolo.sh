#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

if [[ ! -f infra/.env ]]; then
  echo "Missing infra/.env (required for LocalStack/Postgres settings)." >&2
  exit 1
fi

set -a
source infra/.env
set +a

required_vars=(LOCALSTACK_PORT AWS_REGION RAW_BUCKET VAULT_BUCKET RAW_EVENTS_QUEUE POSTGRES_USER POSTGRES_DB POSTGRES_PASSWORD POSTGRES_PORT)
for var in "${required_vars[@]}"; do
  if [[ -z "${!var:-}" ]]; then
    echo "Missing required env var: ${var}" >&2
    exit 1
  fi
done

export AWS_PAGER=""
export AWS_DEFAULT_REGION="${AWS_REGION}"
export LOCALSTACK_ENDPOINT="http://localhost:${LOCALSTACK_PORT}"

COMPOSE="docker compose -f infra/docker-compose.yml"

pg_exec() {
  local sql="$1"
  ${COMPOSE} exec -T -e PGPASSWORD="${POSTGRES_PASSWORD}" postgres \
    psql -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -t -A -c "${sql}"
}

aws_ls() {
  awslocal "$@"
}

echo "[e2e] Resetting infra..."
${COMPOSE} down -v
${COMPOSE} up -d

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
  docker logs --tail 200 localstack >&2 || true
  exit 1
}

wait_postgres() {
  for _ in {1..30}; do
    if ${COMPOSE} exec -T postgres pg_isready -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "Postgres is not ready." >&2
  docker logs --tail 200 postgres >&2 || true
  exit 1
}

if ! ${COMPOSE} wait --timeout 120 localstack postgres >/dev/null 2>&1; then
  wait_localstack
  wait_postgres
fi

./scripts/init-localstack.sh
./scripts/db-migrate.sh

queue_url="$(aws_ls sqs get-queue-url --queue-name "${RAW_EVENTS_QUEUE}" --query 'QueueUrl' --output text)"
if [[ -z "${queue_url}" || "${queue_url}" == "None" ]]; then
  echo "RAW_EVENTS_QUEUE missing after init" >&2
  exit 1
fi

for i in {1..10}; do
  receipts="$(aws_ls sqs receive-message --queue-url "${queue_url}" --max-number-of-messages 10 --wait-time-seconds 1 --query 'Messages[*].ReceiptHandle' --output text)"
  if [[ -z "${receipts}" || "${receipts}" == "None" ]]; then
    break
  fi
  for receipt in ${receipts}; do
    aws_ls sqs delete-message --queue-url "${queue_url}" --receipt-handle "${receipt}" >/dev/null
  done
done

pg_exec "TRUNCATE listings, price_history, processed_files, processing_attempts RESTART IDENTITY CASCADE;" >/dev/null

for table in listings price_history processed_files processing_attempts; do
  count="$(pg_exec "SELECT COUNT(*) FROM ${table};")"
  count="${count//[[:space:]]/}"
  if [[ "${count}" != "0" ]]; then
    echo "Expected ${table} to be empty after truncate, got ${count}" >&2
    exit 1
  fi
done

fixture_path="services/parser/tests/testdata/zolo_identity.jsonl"
if [[ ! -f "${fixture_path}" ]]; then
  echo "Fixture not found: ${fixture_path}" >&2
  exit 1
fi

fixture_lines="$(wc -l < "${fixture_path}" | tr -d '[:space:]')"
if [[ -z "${fixture_lines}" || "${fixture_lines}" -le 0 ]]; then
  echo "Fixture line count invalid: ${fixture_lines}" >&2
  exit 1
fi

date_str="$(date -u +%Y-%m-%d)"
run_id="e2e-$(date -u +%Y%m%dT%H%M%SZ)-$$"
raw_key="raw/zolo/${date_str}/run-${run_id}.jsonl"

echo "[e2e] Uploading fixture to s3://${RAW_BUCKET}/${raw_key}"
aws_ls s3 cp "${fixture_path}" "s3://${RAW_BUCKET}/${raw_key}" --content-type application/x-ndjson >/dev/null

content_length="$(aws_ls s3api head-object --bucket "${RAW_BUCKET}" --key "${raw_key}" --query 'ContentLength' --output text)"
content_type="$(aws_ls s3api head-object --bucket "${RAW_BUCKET}" --key "${raw_key}" --query 'ContentType' --output text)"
raw_etag="$(aws_ls s3api head-object --bucket "${RAW_BUCKET}" --key "${raw_key}" --query 'ETag' --output text | tr -d '"')"

if [[ -z "${content_length}" || "${content_length}" == "None" || "${content_length}" -le 0 ]]; then
  echo "Raw object content length invalid: ${content_length}" >&2
  exit 1
fi
if [[ "${content_type}" != "application/x-ndjson" ]]; then
  echo "Raw object content type mismatch: ${content_type}" >&2
  exit 1
fi

encoded_key="${raw_key//\//%2F}"
found_msg=""
receipt_handle=""

for _ in {1..10}; do
  msg="$(aws_ls sqs receive-message --queue-url "${queue_url}" --max-number-of-messages 1 --wait-time-seconds 1 --visibility-timeout 10 --query 'Messages[0].[Body,ReceiptHandle]' --output text)"
  if [[ -n "${msg}" && "${msg}" != "None" ]]; then
    IFS=$'\t' read -r body receipt <<<"${msg}"
    if [[ -n "${body}" ]] && (printf '%s' "${body}" | grep -q "${raw_key}" || printf '%s' "${body}" | grep -q "${encoded_key}"); then
      found_msg="${body}"
      receipt_handle="${receipt}"
      break
    fi
    if [[ -n "${receipt}" && "${receipt}" != "None" ]]; then
      aws_ls sqs change-message-visibility --queue-url "${queue_url}" --receipt-handle "${receipt}" --visibility-timeout 0 >/dev/null
    fi
  fi
  sleep 1
done

if [[ -z "${found_msg}" ]]; then
  echo "Did not find SQS message for ${raw_key}" >&2
  exit 1
fi
if [[ -n "${receipt_handle}" && "${receipt_handle}" != "None" ]]; then
  aws_ls sqs change-message-visibility --queue-url "${queue_url}" --receipt-handle "${receipt_handle}" --visibility-timeout 0 >/dev/null
fi

validator_go="${ROOT_DIR}/tmp/jsonl-validate-${run_id}.go"
cat >"${validator_go}" <<'GO'
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	file := flag.String("file", "", "")
	require := flag.String("require", "", "")
	printKeys := flag.Bool("print-keys", false, "")
	flag.Parse()
	if *file == "" {
		fmt.Fprintln(os.Stderr, "missing --file")
		os.Exit(1)
	}

	required := map[string]struct{}{}
	if strings.TrimSpace(*require) != "" {
		for _, key := range strings.Split(*require, ",") {
			k := strings.TrimSpace(key)
			if k != "" {
				required[k] = struct{}{}
			}
		}
	}

	f, err := os.Open(*file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	count := 0
	keys := []string{}
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			fmt.Fprintln(os.Stderr, "empty line encountered")
			os.Exit(1)
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			fmt.Fprintf(os.Stderr, "invalid json at line %d: %v\n", count+1, err)
			os.Exit(1)
		}
		for key := range required {
			val, ok := payload[key]
			if !ok || val == nil {
				fmt.Fprintf(os.Stderr, "missing field %s at line %d\n", key, count+1)
				os.Exit(1)
			}
			str := strings.TrimSpace(fmt.Sprintf("%v", val))
			if str == "" {
				fmt.Fprintf(os.Stderr, "empty field %s at line %d\n", key, count+1)
				os.Exit(1)
			}
		}
		if *printKeys {
			val, ok := payload["property_key"]
			if !ok || val == nil {
				fmt.Fprintf(os.Stderr, "missing property_key at line %d\n", count+1)
				os.Exit(1)
			}
			key := strings.TrimSpace(fmt.Sprintf("%v", val))
			if key == "" {
				fmt.Fprintf(os.Stderr, "empty property_key at line %d\n", count+1)
				os.Exit(1)
			}
			keys = append(keys, key)
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "scan failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("count=%d\n", count)
	if *printKeys {
		fmt.Printf("keys=%s\n", strings.Join(keys, ","))
	}
}
GO

raw_meta="$(go run "${validator_go}" --file "${fixture_path}")"
raw_count="$(printf '%s' "${raw_meta}" | awk -F= '/^count=/{print $2}')"
if [[ -z "${raw_count}" || "${raw_count}" -le 0 ]]; then
  echo "Raw fixture count invalid: ${raw_count}" >&2
  exit 1
fi

parser_log="${ROOT_DIR}/tmp/parser-e2e-${run_id}.log"
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

processed_row="$(pg_exec "SELECT status, COALESCE(vault_raw_key,''), COALESCE(vault_normalized_key,''), COALESCE(vault_errors_key,''), COALESCE(error_message,''), COALESCE(etag,''), COALESCE(size_bytes,0) FROM processed_files WHERE s3_bucket='${RAW_BUCKET}' AND s3_key='${raw_key}'")"
processed_row="${processed_row//$'\n'/}"
if [[ -z "${processed_row}" ]]; then
  echo "processed_files row not found for ${raw_key}" >&2
  exit 1
fi

IFS='|' read -r status vault_raw_key vault_norm_key vault_err_key error_message db_etag db_size <<<"${processed_row}"
status="${status//[[:space:]]/}"
error_message="${error_message//[[:space:]]/}"

if [[ "${status}" != "PROCESSED" ]]; then
  echo "processed_files status is not PROCESSED: ${status}" >&2
  exit 1
fi
if [[ -z "${vault_raw_key}" || -z "${vault_norm_key}" || -z "${vault_err_key}" ]]; then
  echo "processed_files vault keys missing" >&2
  exit 1
fi
if [[ -n "${error_message}" ]]; then
  echo "processed_files error_message set: ${error_message}" >&2
  exit 1
fi

vault_raw_len="$(aws_ls s3api head-object --bucket "${VAULT_BUCKET}" --key "${vault_raw_key}" --query 'ContentLength' --output text)"
if [[ -z "${vault_raw_len}" || "${vault_raw_len}" -le 0 ]]; then
  echo "vault raw object missing or empty: ${vault_raw_key}" >&2
  exit 1
fi
if [[ "${vault_raw_len}" != "${content_length}" ]]; then
  echo "vault raw size mismatch: raw=${content_length} vault=${vault_raw_len}" >&2
  exit 1
fi

vault_norm_file="${ROOT_DIR}/tmp/vault-norm-${run_id}.jsonl"
vault_err_file="${ROOT_DIR}/tmp/vault-err-${run_id}.jsonl"
aws_ls s3 cp "s3://${VAULT_BUCKET}/${vault_norm_key}" "${vault_norm_file}" >/dev/null
aws_ls s3 cp "s3://${VAULT_BUCKET}/${vault_err_key}" "${vault_err_file}" >/dev/null

norm_meta="$(go run "${validator_go}" --file "${vault_norm_file}" --require "property_key,address,scraped_at" --print-keys)"
normalized_count="$(printf '%s' "${norm_meta}" | awk -F= '/^count=/{print $2}')"
keys_csv="$(printf '%s' "${norm_meta}" | awk -F= '/^keys=/{print $2}')"

if [[ -z "${normalized_count}" || "${normalized_count}" -le 0 ]]; then
  echo "Normalized JSONL count invalid: ${normalized_count}" >&2
  exit 1
fi
if [[ "${normalized_count}" != "${raw_count}" ]]; then
  echo "Normalized count (${normalized_count}) does not match raw count (${raw_count})" >&2
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

IFS=',' read -r -a keys <<<"${keys_csv}"
if [[ ${#keys[@]} -eq 0 ]]; then
  echo "No property_keys found in normalized output" >&2
  exit 1
fi

in_list="'${keys[0]}'"
for ((i=1; i<${#keys[@]}; i++)); do
  in_list+=",'${keys[i]}'"
done

listings_count="$(pg_exec "SELECT COUNT(*) FROM listings WHERE property_key IN (${in_list});")"
listings_count="${listings_count//[[:space:]]/}"
if [[ "${listings_count}" -ne "${normalized_count}" ]]; then
  echo "Listings count (${listings_count}) does not match normalized count (${normalized_count})" >&2
  exit 1
fi

history_count="$(pg_exec "SELECT COUNT(*) FROM price_history WHERE property_key IN (${in_list});")"
history_count="${history_count//[[:space:]]/}"
if [[ "${history_count}" -ne "${normalized_count}" ]]; then
  echo "price_history count (${history_count}) does not match normalized count (${normalized_count})" >&2
  exit 1
fi

processed_count="$(pg_exec "SELECT COUNT(*) FROM processed_files WHERE s3_bucket='${RAW_BUCKET}' AND s3_key='${raw_key}' AND status='PROCESSED'")"
processed_count="${processed_count//[[:space:]]/}"
if [[ "${processed_count}" != "1" ]]; then
  echo "processed_files count expected 1, got ${processed_count}" >&2
  exit 1
fi

event_body="{\"Records\":[{\"s3\":{\"bucket\":{\"name\":\"${RAW_BUCKET}\"},\"object\":{\"key\":\"${raw_key}\"}}}]}"
aws_ls sqs send-message --queue-url "${queue_url}" --message-body "${event_body}" >/dev/null

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
) >>"${parser_log}" 2>&1
parser_rc=$?
set -e
if [[ ${parser_rc} -ne 0 ]]; then
  echo "Parser failed on idempotency run with exit code ${parser_rc}. Log:" >&2
  tail -n 200 "${parser_log}" >&2
  exit 1
fi

listings_after="$(pg_exec "SELECT COUNT(*) FROM listings WHERE property_key IN (${in_list});")"
listings_after="${listings_after//[[:space:]]/}"
history_after="$(pg_exec "SELECT COUNT(*) FROM price_history WHERE property_key IN (${in_list});")"
history_after="${history_after//[[:space:]]/}"
processed_after="$(pg_exec "SELECT COUNT(*) FROM processed_files WHERE s3_bucket='${RAW_BUCKET}' AND s3_key='${raw_key}' AND status='PROCESSED'")"
processed_after="${processed_after//[[:space:]]/}"

if [[ "${listings_after}" != "${listings_count}" || "${history_after}" != "${history_count}" || "${processed_after}" != "1" ]]; then
  echo "Idempotency violated: listings=${listings_after} history=${history_after} processed=${processed_after}" >&2
  exit 1
fi

echo "[e2e] PASS key=${raw_key} etag=${raw_etag} raw=${raw_count} normalized=${normalized_count} errors=${err_count}"
