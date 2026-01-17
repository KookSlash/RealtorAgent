#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-calgary-realestate-infra-postgres-1}"
POSTGRES_USER="${POSTGRES_USER:-realestate}"
IT_DB="${POSTGRES_IT_DB:-realestate_it}"
MIGRATIONS_DIR="${ROOT_DIR}/db/migrations"

die() {
  echo "ERROR: $*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "Missing required command: $1"
}

require_cmd docker

if ! docker exec -i "${POSTGRES_CONTAINER}" true >/dev/null 2>&1; then
  die "Postgres container ${POSTGRES_CONTAINER} is not running"
fi

echo "[ensure-it-db] Ensuring ${IT_DB} exists"
exists="$(docker exec -i "${POSTGRES_CONTAINER}" \
  psql -U "${POSTGRES_USER}" -d postgres -tAc \
  "SELECT 1 FROM pg_database WHERE datname='${IT_DB}'")"
exists="$(echo "${exists}" | tr -d '[:space:]')"

if [[ "${exists}" != "1" ]]; then
  docker exec -i "${POSTGRES_CONTAINER}" \
    psql -U "${POSTGRES_USER}" -d postgres -c "CREATE DATABASE ${IT_DB};"
fi

echo "[ensure-it-db] Ensuring schema_migrations table exists"
docker exec -i "${POSTGRES_CONTAINER}" psql -U "${POSTGRES_USER}" -d "${IT_DB}" <<'SQL'
CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
SQL

echo "[ensure-it-db] Applying migrations from ${MIGRATIONS_DIR}"
shopt -s nullglob
for f in "${MIGRATIONS_DIR}"/*.sql; do
  v="$(basename "$f")"
  already="$(docker exec -i "${POSTGRES_CONTAINER}" \
    psql -U "${POSTGRES_USER}" -d "${IT_DB}" -tAc \
    "SELECT 1 FROM schema_migrations WHERE version='${v}'")"
  already="$(echo "${already}" | tr -d '[:space:]')"
  if [[ "${already}" == "1" ]]; then
    echo "  = skip ${v}"
    continue
  fi

  echo "  + apply ${v}"
  docker exec -i "${POSTGRES_CONTAINER}" psql -U "${POSTGRES_USER}" -d "${IT_DB}" < "$f"
  docker exec -i "${POSTGRES_CONTAINER}" \
    psql -U "${POSTGRES_USER}" -d "${IT_DB}" -c "INSERT INTO schema_migrations(version) VALUES ('${v}')"
done

echo "[ensure-it-db] Done"
