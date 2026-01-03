# Prerequisites (macOS)

## Required
- Docker Desktop (Apple Silicon)
  - Disable Rosetta emulation (ARM64 only)
  - Set resource limits (CPU/RAM)
- Homebrew
- AWS CLI v2
- pipx
- awslocal (awscli-local)

## Verification commands
- docker version
- docker compose version
- brew --version
- aws --version
- awslocal --version

## LocalStack sanity check
After infra is running, initialize LocalStack and optionally run a smoke test:
- ./scripts/init-localstack.sh
- ./scripts/smoke-s3-sqs.sh

Note: LocalStack may send an initial s3:TestEvent to SQS; consumers must ignore it.
