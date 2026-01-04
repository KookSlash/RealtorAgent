#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

output_file="$ROOT_DIR/tmp/localstack-diagnostics.json"

./scripts/diag-localstack.sh >/tmp/diag-localstack.out

if [ ! -f "$output_file" ]; then
  echo "diagnostics file not found: $output_file" >&2
  exit 1
fi

grep -q '"raw_bucket_exists": true' "$output_file"
grep -q '"list_queues_rc": 0' "$output_file"
grep -q '"create_queue_rc": 0' "$output_file"
grep -q '"delete_queue_rc": 0' "$output_file"
grep -q '"raw_bucket_notification":' "$output_file"

echo "diag-localstack_test: ok"
