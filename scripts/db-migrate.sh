#!/usr/bin/env bash
set -euo pipefail

# Load infra env (postgres creds/port)
set -a
source infra/.env
set +a

MIGRATIONS_DIR="db/migrations"

echo "Waiting for Postgres to be ready..."
until docker exec postgres pg_isready -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" >/dev/null 2>&1; do
  sleep 1
done

echo "Ensuring schema_migrations table exists..."
docker exec -i postgres psql -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" <<'SQL'
CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
SQL

echo "Applying migrations from ${MIGRATIONS_DIR}..."
shopt -s nullglob
for f in "${MIGRATIONS_DIR}"/*.sql; do
  v="$(basename "$f")"
  already="$(docker exec -i postgres psql -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -tAc "SELECT 1 FROM schema_migrations WHERE version='${v}'")"
  if [[ "$already" == "1" ]]; then
    echo "  = skip ${v}"
    continue
  fi

  echo "  + apply ${v}"
  docker exec -i postgres psql -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" < "$f"
  docker exec -i postgres psql -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -c "INSERT INTO schema_migrations(version) VALUES ('${v}')"
done

echo "Done."
