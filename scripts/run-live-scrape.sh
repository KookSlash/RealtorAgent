#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

: "${AWS_REGION:=us-west-2}"
: "${LOCALSTACK_ENDPOINT:=http://localhost:4566}"
: "${RAW_BUCKET:=calgary-raw-bucket}"
: "${DRY_RUN:=false}"
: "${FETCH_MODE:=browser}"
: "${BROWSER_HEADLESS:=true}"
: "${BROWSER_TIMEOUT_MS:=30000}"
: "${BROWSER_WAIT_MS:=5000}"

if [[ -z "${SEARCH_ENTRYPOINT_URL:-}" ]]; then
  echo "ERROR: SEARCH_ENTRYPOINT_URL is required."
  echo "Example:"
  echo "  export SEARCH_ENTRYPOINT_URL=\"https://www.realtor.ca/ab/calgary/real-estate\""
  exit 1
fi

echo "== Live scrape =="
pushd "${ROOT_DIR}/services/scraper" >/dev/null
AWS_REGION="${AWS_REGION}" \
LOCALSTACK_ENDPOINT="${LOCALSTACK_ENDPOINT}" \
RAW_BUCKET="${RAW_BUCKET}" \
DRY_RUN="${DRY_RUN}" \
FETCH_MODE="${FETCH_MODE}" \
BROWSER_HEADLESS="${BROWSER_HEADLESS}" \
BROWSER_TIMEOUT_MS="${BROWSER_TIMEOUT_MS}" \
BROWSER_WAIT_MS="${BROWSER_WAIT_MS}" \
SEARCH_ENTRYPOINT_URL="${SEARCH_ENTRYPOINT_URL}" \
SCRAPER_SAVE_HTML_DIR="${SCRAPER_SAVE_HTML_DIR:-}" \
go run ./cmd/scraper
popd >/dev/null
