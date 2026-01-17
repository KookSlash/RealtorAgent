# App Baseline (Step 0)

This document sets the baseline for the future data browsing app without changing the current pipeline.

## North Star
- Browse scraped real estate data in Postgres through a read-only Go API and a Flutter Web client.
- Keep the ingestion pipeline (scraper -> S3 -> SQS -> parser -> Postgres + vault S3) intact and idempotent.

## MVP Scope (later)
- Desktop browser only (Flutter Web).
- Listing browse view (list + filters/sort as needed for usability).
- Listing detail view with price history and raw metadata links.

## Constraints
- Docker-first local dev; use `infra/docker-compose.yml` and LocalStack to emulate cloud services.
- Read-only API: no writes to S3/SQS/Postgres beyond existing pipeline.
- Preserve idempotency invariants in parser and processed_files ledger.
- Monorepo Option C: app and API live alongside existing services, but remain separate.

## Non-goals
- No auth/roles.
- No analytics/telemetry.
- No production deployment or hosting decisions.
- No changes to scraper/parser logic or infra behavior beyond smoke wiring.

## Definition of Done
### Step 0 (this baseline)
- `docs/app/README.md` exists and reflects scope/constraints.
- `make app-smoke` runs end-to-end on a healthy repo.
- No new services, APIs, or pipeline changes introduced.

### MVP (later)
- Read-only Go API exposes list/detail/history from Postgres.
- Flutter Web UI consumes the API and supports list + detail + history flows.
- Local dev is reproducible via Docker, with basic tests and smoke checks.

## Trust-but-verify workflow
- Keep changes small and validate locally.
- Commands:
  - `make infra-up`
  - `make infra-init`
  - `make app-smoke`
- When touching scraper or parser code, run their tests directly:
  - `cd services/scraper && go test ./... -count=1`
  - `cd services/parser && go test ./... -count=1`

## Architectural intent (later)
- Add a read-only Go API service (no ingestion responsibilities).
- Add a Flutter Web client that talks to the API (no direct DB access).
- Keep pipeline services isolated from app concerns while staying in the monorepo.
