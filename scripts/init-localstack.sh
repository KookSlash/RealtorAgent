#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/stack-env.sh"
source "${ROOT_DIR}/scripts/lib/compose.sh"

if [[ ! -f infra/.env ]]; then
  echo "Missing infra/.env (required for bucket/queue names and region)." >&2
  exit 1
fi

set -a
source infra/.env
set +a
stack_env

required_vars=(LOCALSTACK_PORT AWS_REGION RAW_BUCKET VAULT_BUCKET RAW_EVENTS_QUEUE)
for var in "${required_vars[@]}"; do
  if [[ -z "${!var:-}" ]]; then
    echo "Missing required env var: ${var}" >&2
    exit 1
  fi
done

health_url="http://localhost:${LOCALSTACK_PORT}/_localstack/health"

echo "Waiting for LocalStack to be ready..."
until health="$(curl -sf "${health_url}")" && \
  echo "${health}" | grep -Eq '"s3": "(running|available)"' && \
  echo "${health}" | grep -Eq '"sqs": "(running|available)"'; do
  sleep 1
done

awslocal_exec() {
  local cmd="AWS_PAGER=\"\" AWS_DEFAULT_REGION=${AWS_REGION} AWS_REGION=${AWS_REGION} awslocal"
  for arg in "$@"; do
    cmd+=" $(quote_arg "${arg}")"
  done
  compose_infra exec -T localstack sh -lc "${cmd}"
}

quote_arg() {
  printf "'%s'" "$(printf "%s" "$1" | sed "s/'/'\\\\''/g")"
}

create_bucket() {
  local bucket="$1"

  if awslocal_exec s3api head-bucket --bucket "${bucket}" >/dev/null 2>&1; then
    return 0
  fi

  if [[ "${AWS_REGION}" == "us-east-1" ]]; then
    awslocal_exec s3api create-bucket --bucket "${bucket}" >/dev/null
  else
    awslocal_exec s3api create-bucket \
      --bucket "${bucket}" \
      --create-bucket-configuration "LocationConstraint=${AWS_REGION}" >/dev/null
  fi
}

echo "Ensuring S3 buckets exist..."
create_bucket "${RAW_BUCKET}"
create_bucket "${VAULT_BUCKET}"

echo "Ensuring SQS queue exists..."
QUEUE_URL=""
if QUEUE_URL="$(awslocal_exec sqs get-queue-url --queue-name "${RAW_EVENTS_QUEUE}" --query 'QueueUrl' --output text 2>/dev/null)"; then
  if [[ "${QUEUE_URL}" == "None" ]]; then
    QUEUE_URL=""
  fi
fi
if [[ -z "${QUEUE_URL}" ]]; then
  QUEUE_URL="$(awslocal_exec sqs create-queue --queue-name "${RAW_EVENTS_QUEUE}" --query 'QueueUrl' --output text)"
fi

QUEUE_ARN="$(awslocal_exec sqs get-queue-attributes \
  --queue-url "${QUEUE_URL}" \
  --attribute-names QueueArn \
  --query 'Attributes.QueueArn' --output text)"

echo "Configuring S3 -> SQS notifications on raw bucket..."
notif_json="$(cat <<JSON
{
  "QueueConfigurations": [
    {
      "QueueArn": "${QUEUE_ARN}",
      "Events": ["s3:ObjectCreated:*"],
      "Filter": {
        "Key": {
          "FilterRules": [
            {"Name": "prefix", "Value": "raw/"}
          ]
        }
      }
    }
  ]
}
JSON
)"

awslocal_exec s3api put-bucket-notification-configuration \
  --bucket "${RAW_BUCKET}" \
  --notification-configuration "${notif_json}"

echo "Init complete."
echo "RAW_BUCKET=${RAW_BUCKET}"
echo "VAULT_BUCKET=${VAULT_BUCKET}"
echo "RAW_EVENTS_QUEUE=${RAW_EVENTS_QUEUE}"
echo "RAW_EVENTS_QUEUE_URL=${QUEUE_URL}"
