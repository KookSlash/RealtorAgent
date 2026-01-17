# ReadAPI

Read-only API for browsing listings, inspecting price history, and reading change feeds.

## Run locally

```bash
make infra-up
make infra-init
make db-migrate
make readapi
```

## Endpoints

Health:

```bash
curl http://localhost:8090/healthz
```

Browse listings:

```bash
curl "http://localhost:8090/v1/listings?page=1&page_size=20&sort=price_desc&min_beds=2&postal_code=T2P"
```

Value score sort (higher is better value within its comps group):

```bash
curl "http://localhost:8090/v1/listings?sort=value_desc&page=1&page_size=20"
```

Listing detail:

```bash
curl "http://localhost:8090/v1/listings/<property_key>"
```

Price history:

```bash
curl "http://localhost:8090/v1/listings/<property_key>/price-history"
```

Changes feed:

```bash
curl "http://localhost:8090/v1/changes?since=2026-01-01T00:00:00Z&type=price_change"
```

## Value score fields

`ppsf_percentile` and `value_score` are computed per comps group (property_type + beds bucket) after filters are applied.

- `ppsf_percentile`: percentile of price-per-sqft within the group (0=best value, 100=worst).
- `value_score`: inverse of percentile (100=best value).
- `comps_count`: count of comparable listings with a non-null ppsf in the group.

## Tests

```bash
DB_ENABLED=true
mkdir -p tmp/go-cache tmp/go-tmp
GOCACHE=$PWD/tmp/go-cache GOTMPDIR=$PWD/tmp/go-tmp go -C services/readapi test ./...
DB_ENABLED=true GOCACHE=$PWD/tmp/go-cache GOTMPDIR=$PWD/tmp/go-tmp go -C services/readapi test -tags=integration ./... -count=1
```

Integration tests default missing POSTGRES_* env vars to 127.0.0.1:5432 with realestate/realestate credentials when DB_ENABLED=true.
Integration tests target the `realestate_it` database by default (created/migrated by `make test-readapi-integration`).

Make target:

```bash
make test-readapi-integration
```
