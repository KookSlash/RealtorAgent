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
