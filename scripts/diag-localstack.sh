#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

COMPOSE="docker compose -f infra/docker-compose.yml"
export AWS_PAGER=""

OUTPUT_DIR="$ROOT_DIR/tmp"
OUTPUT_PATH="$OUTPUT_DIR/localstack-diagnostics.json"

mkdir -p "$OUTPUT_DIR"

timestamp_utc="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
queue_suffix="$(date -u +"%Y%m%dT%H%M%SZ")"
diag_queue="diag-queue-${queue_suffix}"

CMD_OUT=""
CMD_ERR=""
CMD_RC=0

run_cmd() {
  local out_file err_file
  out_file="$(mktemp)"
  err_file="$(mktemp)"
  set +e
  "$@" >"$out_file" 2>"$err_file"
  CMD_RC=$?
  set -e
  CMD_OUT="$(cat "$out_file")"
  CMD_ERR="$(cat "$err_file")"
  rm -f "$out_file" "$err_file"
}

json_escape() {
  local s="$1"
  s=${s//\\/\\\\}
  s=${s//\"/\\\"}
  s=${s//$'\t'/\\t}
  s=${s//$'\r'/\\r}
  s=${s//$'\n'/\\n}
  printf '%s' "$s"
}

extract_env_value() {
  local key="$1"
  printf '%s\n' "$2" | awk -F= -v k="$key" '$1==k { $1=""; sub(/^=/,""); print $0 }' | tail -n 1
}

run_cmd bash -lc "$COMPOSE ps"
compose_ps="$CMD_OUT"

run_cmd docker exec localstack env
localstack_env="$CMD_OUT"

env_services="$(extract_env_value "SERVICES" "$localstack_env")"
env_localstack_services="$(extract_env_value "LOCALSTACK_SERVICES" "$localstack_env")"
env_debug="$(extract_env_value "DEBUG" "$localstack_env")"
env_aws_default_region="$(extract_env_value "AWS_DEFAULT_REGION" "$localstack_env")"
env_edge_port="$(extract_env_value "EDGE_PORT" "$localstack_env")"
env_hostname_external="$(extract_env_value "HOSTNAME_EXTERNAL" "$localstack_env")"

run_cmd docker exec localstack sh -lc "curl -sf http://localhost:4566/_localstack/health"
health_raw="$CMD_OUT"
health_rc=$CMD_RC
if [ -z "$health_raw" ]; then
  health_raw="$CMD_ERR"
fi

run_cmd docker logs --tail 200 localstack
logs_tail="$CMD_OUT"

run_cmd docker exec localstack sh -lc "command -v awslocal"
awslocal_path="$CMD_OUT"
if [ -z "$awslocal_path" ]; then
  awslocal_path="$CMD_ERR"
fi

run_cmd docker exec localstack sh -lc "aws --version"
aws_version="$CMD_OUT"
if [ -z "$aws_version" ]; then
  aws_version="$CMD_ERR"
fi

run_cmd docker exec localstack sh -lc "awslocal --version"
awslocal_version="$CMD_OUT"
if [ -z "$awslocal_version" ]; then
  awslocal_version="$CMD_ERR"
fi

run_cmd awslocal sqs list-queues --output json
list_queues_stdout="$CMD_OUT"
list_queues_stderr="$CMD_ERR"
list_queues_rc=$CMD_RC

run_cmd awslocal sqs create-queue --queue-name "$diag_queue" --output json
create_queue_stdout="$CMD_OUT"
create_queue_stderr="$CMD_ERR"
create_queue_rc=$CMD_RC
created_queue_url="$(printf '%s\n' "$create_queue_stdout" | awk -F'"' '/QueueUrl/ {print $4; exit}')"

run_cmd awslocal sqs list-queues --output json
list_queues_after_create_stdout="$CMD_OUT"
list_queues_after_create_rc=$CMD_RC

delete_queue_rc=1
if [ -n "$created_queue_url" ]; then
  run_cmd awslocal sqs delete-queue --queue-url "$created_queue_url"
  delete_queue_rc=$CMD_RC
fi

run_cmd awslocal s3api list-buckets --output json
buckets_raw="$CMD_OUT"
if [ -z "$buckets_raw" ]; then
  buckets_raw="$CMD_ERR"
fi

run_cmd awslocal s3api head-bucket --bucket calgary-raw-bucket
raw_bucket_exists=false
if [ "$CMD_RC" -eq 0 ]; then
  raw_bucket_exists=true
fi

run_cmd awslocal s3api get-bucket-notification-configuration --bucket calgary-raw-bucket --output json
raw_bucket_notification="$CMD_OUT"
if [ -z "$raw_bucket_notification" ]; then
  raw_bucket_notification="$CMD_ERR"
fi

queue_arns_lines="$(printf '%s\n' "$raw_bucket_notification" | awk -F'"' '/QueueArn/ {print $4}')"

queue_arns_json="[]"
if [ -n "$queue_arns_lines" ]; then
  queue_arns_json="["
  first=1
  while IFS= read -r arn; do
    if [ -n "$arn" ]; then
      if [ $first -eq 0 ]; then
        queue_arns_json+=","
      fi
      first=0
      queue_arns_json+="\"$(json_escape "$arn")\""
    fi
  done <<< "$queue_arns_lines"
  queue_arns_json+="]"
fi

queue_urls_resolvable=false
queue_list_for_resolution="$list_queues_after_create_stdout"
if [ -z "$(printf '%s' "$queue_list_for_resolution" | tr -d '[:space:]')" ]; then
  queue_list_for_resolution="$list_queues_stdout"
fi

if [ -n "$queue_arns_lines" ] && [ -n "$queue_list_for_resolution" ]; then
  queue_urls_resolvable=true
  while IFS= read -r arn; do
    name="${arn##*:}"
    if ! printf '%s' "$queue_list_for_resolution" | grep -q "/$name" >/dev/null 2>&1; then
      queue_urls_resolvable=false
      break
    fi
  done <<< "$queue_arns_lines"
fi

fail=0
if [ "$health_rc" -ne 0 ]; then
  fail=1
fi
if ! printf '%s' "$health_raw" | tr -d '[:space:]' | grep -q '"sqs":"running"' >/dev/null 2>&1; then
  fail=1
fi
if [ "$list_queues_rc" -ne 0 ]; then
  fail=1
fi
if [ "$create_queue_rc" -ne 0 ]; then
  fail=1
fi
if [ "$list_queues_after_create_rc" -ne 0 ]; then
  fail=1
fi
if [ "$delete_queue_rc" -ne 0 ]; then
  fail=1
fi

diagnostics_json="$(
  cat <<EOF
{
  "timestamp_utc": "$(json_escape "$timestamp_utc")",
  "docker": {
    "compose_ps": "$(json_escape "$compose_ps")"
  },
  "localstack": {
    "env_selected": {
      "SERVICES": "$(json_escape "$env_services")",
      "LOCALSTACK_SERVICES": "$(json_escape "$env_localstack_services")",
      "DEBUG": "$(json_escape "$env_debug")",
      "AWS_DEFAULT_REGION": "$(json_escape "$env_aws_default_region")",
      "EDGE_PORT": "$(json_escape "$env_edge_port")",
      "HOSTNAME_EXTERNAL": "$(json_escape "$env_hostname_external")"
    },
    "health": "$(json_escape "$health_raw")",
    "logs_tail": "$(json_escape "$logs_tail")"
  },
  "awscli": {
    "awslocal_path": "$(json_escape "$awslocal_path")",
    "aws_version": "$(json_escape "$aws_version")",
    "awslocal_version": "$(json_escape "$awslocal_version")"
  },
  "s3": {
    "buckets": "$(json_escape "$buckets_raw")",
    "raw_bucket_notification": "$(json_escape "$raw_bucket_notification")",
    "raw_bucket_exists": $raw_bucket_exists
  },
  "sqs": {
    "list_queues_stdout": "$(json_escape "$list_queues_stdout")",
    "list_queues_stderr": "$(json_escape "$list_queues_stderr")",
    "list_queues_rc": $list_queues_rc
  },
  "sqs_functionality_probe": {
    "created_queue_url": "$(json_escape "$created_queue_url")",
    "create_queue_stdout": "$(json_escape "$create_queue_stdout")",
    "create_queue_stderr": "$(json_escape "$create_queue_stderr")",
    "create_queue_rc": $create_queue_rc,
    "list_queues_after_create_stdout": "$(json_escape "$list_queues_after_create_stdout")",
    "delete_queue_rc": $delete_queue_rc
  },
  "s3_to_sqs_wiring_check": {
    "expected": {
      "raw_bucket": "calgary-raw-bucket"
    },
    "observed": {
      "queue_arns_in_notification": $queue_arns_json,
      "queue_urls_resolvable": $queue_urls_resolvable
    }
  }
}
EOF
)"

printf '%s\n' "$diagnostics_json" | tee "$OUTPUT_PATH"

exit "$fail"
