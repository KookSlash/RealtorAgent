#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f infra/.env ]]; then
  echo "Missing infra/.env (required for bucket/queue names and region)." >&2
  exit 1
fi

set -a
source infra/.env
set +a

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

create_bucket() {
  local bucket="$1"

  if awslocal s3api head-bucket --bucket "${bucket}" >/dev/null 2>&1; then
    return 0
  fi

  if [[ "${AWS_REGION}" == "us-east-1" ]]; then
    awslocal s3api create-bucket --bucket "${bucket}" >/dev/null
  else
    awslocal s3api create-bucket \
      --bucket "${bucket}" \
      --create-bucket-configuration "LocationConstraint=${AWS_REGION}" >/dev/null
  fi
}

echo "Ensuring S3 buckets exist..."
create_bucket "${RAW_BUCKET}"
create_bucket "${VAULT_BUCKET}"

echo "Ensuring SQS queue exists..."
QUEUE_URL=""
if QUEUE_URL="$(awslocal sqs get-queue-url --queue-name "${RAW_EVENTS_QUEUE}" --query 'QueueUrl' --output text 2>/dev/null)"; then
  if [[ "${QUEUE_URL}" == "None" ]]; then
    QUEUE_URL=""
  fi
fi
if [[ -z "${QUEUE_URL}" ]]; then
  QUEUE_URL="$(awslocal sqs create-queue --queue-name "${RAW_EVENTS_QUEUE}" --query 'QueueUrl' --output text)"
fi

QUEUE_ARN="$(awslocal sqs get-queue-attributes \
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

awslocal s3api put-bucket-notification-configuration \
  --bucket "${RAW_BUCKET}" \
  --notification-configuration "${notif_json}"

echo "Init complete."
echo "RAW_BUCKET=${RAW_BUCKET}"
echo "VAULT_BUCKET=${VAULT_BUCKET}"
echo "RAW_EVENTS_QUEUE=${RAW_EVENTS_QUEUE}"
echo "RAW_EVENTS_QUEUE_URL=${QUEUE_URL}"
