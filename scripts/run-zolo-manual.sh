#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/stack-env.sh"
STACK="${STACK:-run}"
export STACK
source "${ROOT_DIR}/scripts/lib/compose.sh"

say() {
  echo "[run-zolo] $*"
}

die() {
  echo "ERROR: $*" >&2
  exit 1
}

dc() {
  compose_infra "$@"
}

quote_arg() {
  printf "'%s'" "$(printf "%s" "$1" | sed "s/'/'\\\\''/g")"
}

awslocal_cmd() {
  local cmd="AWS_PAGER=\"\" AWS_DEFAULT_REGION=${AWS_REGION} AWS_REGION=${AWS_REGION} awslocal"
  for arg in "$@"; do
    cmd+=" $(quote_arg "$arg")"
  done
  printf "%s" "${cmd}"
}

awslocal_in_container() {
  local cmd
  cmd="$(awslocal_cmd "$@")"
  compose_infra exec -T localstack sh -lc "${cmd}"
}

psql_in_container() {
  local cmd="$*"
  compose_infra exec -T postgres sh -lc "${cmd}"
}

[[ -f "${ROOT_DIR}/infra/docker-compose.yml" ]] || die "Missing infra/docker-compose.yml"
[[ -f "${ROOT_DIR}/scripts/init-localstack.sh" ]] || die "Missing scripts/init-localstack.sh"
[[ -f "${ROOT_DIR}/services/scraper/go.mod" ]] || die "Missing services/scraper/go.mod"
[[ -f "${ROOT_DIR}/services/parser/go.mod" ]] || die "Missing services/parser/go.mod"
[[ -f "${ROOT_DIR}/infra/.env" ]] || die "Missing infra/.env"

set -a
source "${ROOT_DIR}/infra/.env"
set +a
stack_env

say "Starting infra"
# Remove orphans (like readapi) so infra stays clean and warnings are avoided.
dc up -d --remove-orphans

say "Initializing LocalStack"
AWS_PAGER="" ./scripts/init-localstack.sh

say "Checking Postgres"
psql_in_container "PGPASSWORD=${POSTGRES_PASSWORD} psql -U ${POSTGRES_USER} -d ${POSTGRES_DB} -c \"select 1\" >/dev/null"

mkdir -p "${ROOT_DIR}/tmp"

say "Running scraper"
scraper_log_base="$(mktemp "${ROOT_DIR}/tmp/run-zolo-scraper-XXXXXX")"
scraper_log="${scraper_log_base}.log"
mv "${scraper_log_base}" "${scraper_log}"
(
  cd services/scraper
  AWS_REGION="${AWS_REGION}" \
  LOCALSTACK_ENDPOINT="http://localhost:${LOCALSTACK_PORT}" \
  RAW_BUCKET="${RAW_BUCKET}" \
  SCRAPER_STRATEGY=zolo_ca \
  ZOLO_ENTRYPOINT_URL="https://www.zolo.ca/index.php?sarea=Calgary&filter=1" \
  MAX_PAGES="${MAX_PAGES:-0}" \
  DRY_RUN=false \
  go run ./cmd/scraper
) | tee "${scraper_log}"

output_key="$(awk -F= '/^output_key=/{print $2; exit}' "${scraper_log}" | tr -d '[:space:]')"
[[ -n "${output_key}" ]] || die "Failed to extract output_key from scraper output"

say "Checking raw S3 object"
awslocal_in_container s3api head-object --bucket "${RAW_BUCKET}" --key "${output_key}" >/dev/null

say "Running parser"
parser_log_base="$(mktemp "${ROOT_DIR}/tmp/run-zolo-parser-XXXXXX")"
parser_log="${parser_log_base}.log"
mv "${parser_log_base}" "${parser_log}"
(
  cd services/parser
  AWS_REGION="${AWS_REGION}" \
  RAW_BUCKET="${RAW_BUCKET}" \
  VAULT_BUCKET="${VAULT_BUCKET}" \
  RAW_EVENTS_QUEUE="${RAW_EVENTS_QUEUE}" \
  LOCALSTACK_ENDPOINT="http://localhost:${LOCALSTACK_PORT}" \
  POSTGRES_HOST=localhost \
  POSTGRES_PORT="${POSTGRES_PORT}" \
  POSTGRES_DB="${POSTGRES_DB}" \
  POSTGRES_USER="${POSTGRES_USER}" \
  POSTGRES_PASSWORD="${POSTGRES_PASSWORD}" \
  ONCE=true \
  DRY_RUN=false \
  go run ./cmd/parser
) | tee "${parser_log}"

safe_key="${output_key//\'/\'\'}"
processed_row="$(psql_in_container "PGPASSWORD=realestate psql -U realestate -d realestate -t -A -c \"SELECT status, COALESCE(vault_raw_key,''), COALESCE(vault_normalized_key,''), COALESCE(vault_errors_key,'') FROM processed_files WHERE s3_bucket='calgary-raw-bucket' AND s3_key='${safe_key}'\"")"
processed_row="${processed_row//$'\n'/}"
[[ -n "${processed_row}" ]] || die "processed_files row not found for ${output_key}"

IFS='|' read -r status vault_raw_key vault_norm_key vault_err_key <<<"${processed_row}"
status="${status//[[:space:]]/}"

[[ "${status}" == "PROCESSED" ]] || die "processed_files status is not PROCESSED: ${status}"
[[ -n "${vault_raw_key}" ]] || die "vault_raw_key is empty"
[[ -n "${vault_norm_key}" ]] || die "vault_normalized_key is empty"
[[ -n "${vault_err_key}" ]] || die "vault_errors_key is empty"

say "Checking vault objects"
awslocal_in_container s3api head-object --bucket calgary-vault-bucket --key "${vault_raw_key}" >/dev/null
awslocal_in_container s3api head-object --bucket calgary-vault-bucket --key "${vault_norm_key}" >/dev/null
awslocal_in_container s3api head-object --bucket calgary-vault-bucket --key "${vault_err_key}" >/dev/null

echo "raw_s3_key=${output_key}"
echo "vault_raw_key=${vault_raw_key}"
echo "vault_normalized_key=${vault_norm_key}"
echo "vault_errors_key=${vault_err_key}"
