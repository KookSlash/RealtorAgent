#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f infra/.env ]]; then
  echo "Missing infra/.env (required for bucket/queue names)." >&2
  exit 1
fi

set -a
source infra/.env
set +a

required_vars=(RAW_BUCKET RAW_EVENTS_QUEUE)
for var in "${required_vars[@]}"; do
  if [[ -z "${!var:-}" ]]; then
    echo "Missing required env var: ${var}" >&2
    exit 1
  fi
done

date_str="$(date -u +%Y-%m-%d)"
run_id="$(date -u +%Y%m%d%H%M%S)-${RANDOM}"
object_key="raw/realtorca/${date_str}/run-${run_id}.jsonl"

tmpfile="$(mktemp)"
trap 'rm -f "${tmpfile}"' EXIT
printf '{"run_id":"%s","created_at":"%s"}\n' "${run_id}" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "${tmpfile}"

echo "Uploading ${object_key} to s3://${RAW_BUCKET}..."
awslocal s3 cp "${tmpfile}" "s3://${RAW_BUCKET}/${object_key}" >/dev/null

QUEUE_URL="$(awslocal sqs get-queue-url --queue-name "${RAW_EVENTS_QUEUE}" --query 'QueueUrl' --output text)"

echo "Polling ${RAW_EVENTS_QUEUE} for ObjectCreated events..."
found=0
for _ in {1..10}; do
  lines="$(awslocal sqs receive-message \
    --queue-url "${QUEUE_URL}" \
    --max-number-of-messages 10 \
    --wait-time-seconds 2 \
    --query 'Messages[*].[Body,ReceiptHandle]' \
    --output text)"

  if [[ -z "${lines}" || "${lines}" == "None" ]]; then
    continue
  fi

  while IFS=$'\t' read -r msg receipt; do
    if [[ -z "${msg}" ]]; then
      continue
    fi

    if [[ "${msg}" == *"s3:TestEvent"* ]]; then
      echo "Ignoring s3:TestEvent message"
      if [[ -n "${receipt}" ]]; then
        awslocal sqs delete-message --queue-url "${QUEUE_URL}" --receipt-handle "${receipt}" >/dev/null
      fi
      continue
    fi

    decoded="${msg//\\\"/\"}"
    decoded="${decoded//\\\\/\\}"

    if [[ "${decoded}" == *"ObjectCreated"* ]]; then
      bucket="$(echo "${decoded}" | sed -n 's/.*"bucket"[^{]*{[^}]*"name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
      key="$(echo "${decoded}" | sed -n 's/.*"object"[^{]*{[^}]*"key"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
      if [[ -n "${bucket}" && -n "${key}" ]]; then
        echo "ObjectCreated: bucket=${bucket} key=${key}"
      else
        echo "ObjectCreated event found but bucket/key parsing failed"
      fi
      found=1
      break 2
    fi
  done <<< "${lines}"
done

if [[ "${found}" -ne 1 ]]; then
  echo "No ObjectCreated event found."
  exit 1
fi
