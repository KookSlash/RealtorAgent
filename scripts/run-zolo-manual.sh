#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

say() {
  echo "[run-zolo] $*"
}

die() {
  echo "ERROR: $*" >&2
  exit 1
}

dc() {
  docker compose -f "${ROOT_DIR}/infra/docker-compose.yml" "$@"
}

quote_arg() {
  printf "'%s'" "$(printf "%s" "$1" | sed "s/'/'\\\\''/g")"
}

awslocal_cmd() {
  local cmd="AWS_PAGER=\"\" AWS_DEFAULT_REGION=us-west-2 awslocal"
  for arg in "$@"; do
    cmd+=" $(quote_arg "$arg")"
  done
  printf "%s" "${cmd}"
}

awslocal_in_container() {
  local cmd
  cmd="$(awslocal_cmd "$@")"
  dc exec -T localstack sh -lc "${cmd}"
}

psql_in_container() {
  local cmd="$*"
  dc exec -T postgres sh -lc "${cmd}"
}

[[ -f "${ROOT_DIR}/infra/docker-compose.yml" ]] || die "Missing infra/docker-compose.yml"
[[ -f "${ROOT_DIR}/scripts/init-localstack.sh" ]] || die "Missing scripts/init-localstack.sh"
[[ -f "${ROOT_DIR}/services/scraper/go.mod" ]] || die "Missing services/scraper/go.mod"
[[ -f "${ROOT_DIR}/services/parser/go.mod" ]] || die "Missing services/parser/go.mod"

say "Starting infra"
dc up -d

say "Initializing LocalStack"
awslocal_shim_dir="$(mktemp -d)"
trap 'rm -rf "${awslocal_shim_dir}"' EXIT
cat > "${awslocal_shim_dir}/awslocal" <<'EOF_AWS'
#!/usr/bin/env bash
set -euo pipefail
quote_arg() {
  printf "'%s'" "$(printf "%s" "$1" | sed "s/'/'\\\\''/g")"
}

awslocal_cmd() {
  local cmd="AWS_PAGER=\"\" AWS_DEFAULT_REGION=us-west-2 awslocal"
  for arg in "$@"; do
    cmd+=" $(quote_arg "$arg")"
  done
  printf "%s" "${cmd}"
}

cmd="$(awslocal_cmd "$@")"
docker compose -f "__ROOT_DIR__/infra/docker-compose.yml" exec -T localstack sh -lc "${cmd}"
EOF_AWS
sed -i '' "s|__ROOT_DIR__|${ROOT_DIR}|g" "${awslocal_shim_dir}/awslocal"
chmod +x "${awslocal_shim_dir}/awslocal"
PATH="${awslocal_shim_dir}:${PATH}" AWS_PAGER="" AWS_DEFAULT_REGION="us-west-2" ./scripts/init-localstack.sh

say "Checking Postgres"
psql_in_container "PGPASSWORD=realestate psql -U realestate -d realestate -c \"select 1\" >/dev/null"

mkdir -p "${ROOT_DIR}/tmp"

say "Running scraper"
scraper_log_base="$(mktemp "${ROOT_DIR}/tmp/run-zolo-scraper-XXXXXX")"
scraper_log="${scraper_log_base}.log"
mv "${scraper_log_base}" "${scraper_log}"
(
  cd services/scraper
  AWS_REGION=us-west-2 \
  LOCALSTACK_ENDPOINT=http://localhost:4566 \
  RAW_BUCKET=calgary-raw-bucket \
  SCRAPER_STRATEGY=zolo_ca \
  ZOLO_ENTRYPOINT_URL="https://www.zolo.ca/index.php?sarea=Calgary&filter=1" \
  MAX_PAGES="${MAX_PAGES:-0}" \
  DRY_RUN=false \
  go run ./cmd/scraper
) | tee "${scraper_log}"

output_key="$(awk -F= '/^output_key=/{print $2; exit}' "${scraper_log}" | tr -d '[:space:]')"
[[ -n "${output_key}" ]] || die "Failed to extract output_key from scraper output"

say "Checking raw S3 object"
awslocal_in_container s3api head-object --bucket calgary-raw-bucket --key "${output_key}" >/dev/null

say "Running parser"
parser_log_base="$(mktemp "${ROOT_DIR}/tmp/run-zolo-parser-XXXXXX")"
parser_log="${parser_log_base}.log"
mv "${parser_log_base}" "${parser_log}"
(
  cd services/parser
  AWS_REGION=us-west-2 \
  RAW_BUCKET=calgary-raw-bucket \
  VAULT_BUCKET=calgary-vault-bucket \
  RAW_EVENTS_QUEUE=calgary-raw-events-queue \
  LOCALSTACK_ENDPOINT=http://localhost:4566 \
  POSTGRES_HOST=localhost \
  POSTGRES_PORT=5432 \
  POSTGRES_DB=realestate \
  POSTGRES_USER=realestate \
  POSTGRES_PASSWORD=realestate \
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
