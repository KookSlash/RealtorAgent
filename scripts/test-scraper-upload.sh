#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if ! command -v awslocal >/dev/null 2>&1; then
  echo "awslocal is required (pip install awscli-local)."
  exit 1
fi

if [ ! -f "$ROOT_DIR/infra/.env" ]; then
  echo "infra/.env not found. Copy infra/.env.example to infra/.env and set values."
  exit 1
fi

set -a
. "$ROOT_DIR/infra/.env"
set +a

make -C "$ROOT_DIR" infra-up
make -C "$ROOT_DIR" infra-init

export LOCALSTACK_ENDPOINT="http://localhost:4566"
export AWS_REGION="${AWS_REGION:-us-west-2}"
export RAW_BUCKET="${RAW_BUCKET:?RAW_BUCKET not set in infra/.env}"
export DRY_RUN="false"
export SCRAPE_DATE="$(date -u +%Y-%m-%d)"
export CGO_ENABLED=0

scraper_output="$(
  cd "$ROOT_DIR/services/scraper"
  go run ./cmd/scraper
)"

echo "$scraper_output"

output_key="$(printf "%s\n" "$scraper_output" | awk -F= '/^output_key=/{print $2}')"
if [ -z "$output_key" ]; then
  echo "Could not read output_key from scraper output."
  exit 1
fi

awslocal s3api head-object --bucket "$RAW_BUCKET" --key "$output_key" >/dev/null

temp_file="$(mktemp)"
trap 'rm -f "$temp_file"' EXIT

awslocal s3 cp "s3://$RAW_BUCKET/$output_key" "$temp_file" >/dev/null

python3 - "$temp_file" <<'PY'
import json
import sys

path = sys.argv[1]
with open(path, "r", encoding="utf-8") as handle:
    lines = [line.rstrip("\n") for line in handle if line.strip()]

if len(lines) != 2:
    raise SystemExit(f"expected 2 lines, got {len(lines)}")

required = ["address", "postal_code", "property_type", "price", "url", "scraped_at"]
for line in lines:
    obj = json.loads(line)
    for field in required:
        if field not in obj:
            raise SystemExit(f"missing required field {field}")
        if isinstance(obj[field], str) and not obj[field].strip():
            raise SystemExit(f"empty required field {field}")
    if "source_payload" not in obj:
        raise SystemExit("missing source_payload")
    json.dumps(obj["source_payload"])

print("verified_jsonl_lines=2")
PY
