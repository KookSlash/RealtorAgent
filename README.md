# Calgary Real Estate Monitor

## What this does (functional)
Daily snapshots of REALTOR.ca listings, deduplicates properties, tracks changes over time, and enables analytics/search.

## Quickstart
1) Follow prerequisites: docs/prerequisites.md
2) Start infra:
   - docker compose -f infra/docker-compose.yml up -d
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
