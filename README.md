# Calgary Real Estate Monitor

A local-first, AWS-compatible pipeline to **observe the Calgary real-estate market over time** by ingesting daily REALTOR.ca snapshots, deduplicating physical properties, and tracking meaningful changes.

## What this does (functional)

This project helps you answer questions like:
- “How are prices evolving in a given neighborhood / property type?”
- “What changed since yesterday?” (price, beds/baths/sqft, etc.)
- “How long do properties stay on the market?” (derived client-side if needed)

It does this by:
1) **Scraping daily** listing snapshots (JSONL files) from REALTOR.ca (scraper to be added).
2) Writing raw snapshot files to S3 (LocalStack locally, AWS later).
3) Triggering parsing via **S3 → SQS events**.
4) Parsing + normalizing into Postgres:
   - `listings`: latest state per physical property
   - `price_history`: **event-only history** (only when something changed)
5) Archiving artifacts to a vault bucket for audit/restore:
   - raw copy
   - normalized output
   - errors/rejects

### Non-negotiable invariants

- **Idempotent ingestion**  
  Reprocessing the same S3 object must not create duplicates. We identify processed objects by:
  - `(s3_bucket, s3_key, etag)` in `processed_files`.

- **Event-only history**  
  We write to `price_history` only when a **snapshot_hash changes** (URL is excluded to avoid noise).  
  A daily time series can be reconstructed client-side by forward-filling.

- **Raw bucket immutability by convention**  
  We do not move/delete raw objects after processing. Auditability comes from `processed_files` + the vault.

- **Vault mapping (deterministic)**
  For a raw object key `<original-key>`, vault artifacts are:
  - `vault/raw/<original-key>`
  - `vault/normalized/<original-key>.normalized.jsonl`
  - `vault/errors/<original-key>.errors.jsonl`

## Architecture

```mermaid
flowchart LR
  subgraph Source
    A[REALTOR.ca\n(daily scrape)] -->|JSONL| B[Scraper\n(once/day)]
  end

  subgraph Storage
    C[S3 RAW Bucket\n(raw/...)] -->|archive copy| V[S3 VAULT Bucket\n(vault/raw|normalized|errors)]
  end

  subgraph Events
    C -->|ObjectCreated| Q[SQS Queue\nRAW_EVENTS_QUEUE]
  end

  subgraph Processing
    Q --> P[Parser (Go)\nSQS consumer]
    P -->|HEAD/GET raw| C
    P -->|UPSERT listings\nINSERT price_history (event-only)\nLEDGER processed_files| D[(Postgres)]
    P -->|PUT normalized/errors\nCOPY raw| V
  end

  note1{{Note:\nS3 may emit s3:TestEvent\nParser must ignore}} --- Q


## Quickstart
1) Follow prerequisites: docs/prerequisites.md
2) Start infra:
make infra-up
make infra-status

3) Initialize LocalStack:
   - ./scripts/init-localstack.sh
4) Optional smoke test:
   - ./scripts/smoke-s3-sqs.sh

Note: S3 can emit an initial s3:TestEvent to SQS; consumers must ignore it.

## Repo structure
- infra/ (docker-compose, local aws emulation)
- services/ (scraper, parser, api later)
- docs/

## Parser tests
Run the integration tests against LocalStack + Postgres:
- make infra-up
- make infra-init
- make db-migrate
- ./scripts/test-parser.sh

Notes:
- Tests create and consume messages in the shared RAW_EVENTS_QUEUE.
- Consumers must ignore initial s3:TestEvent messages.

## Unit tests
Run fast, hermetic unit tests locally:
- go test ./...
- go test -race ./...
- go test ./... -run TestNormalizeLineValid
- go test ./... -fuzz=FuzzNormalizeAddress -fuzztime=10s
- go test ./... -coverprofile=coverage.out && go tool cover -func=coverage.out
