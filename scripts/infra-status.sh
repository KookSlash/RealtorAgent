#!/usr/bin/env bash
set -euo pipefail

echo "== Docker containers =="
docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | sed -n '1p;/localstack/p;/postgres/p'

echo
echo "== LocalStack health =="
curl -sf http://localhost:4566/_localstack/health || true
echo
