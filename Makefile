.PHONY: infra-up infra-init db-migrate infra-status test-parser diag-localstack diag-localstack-test

infra-up:
	docker compose -f infra/docker-compose.yml up -d

infra-init:
	./scripts/init-localstack.sh

db-migrate:
	./scripts/db-migrate.sh

infra-status:
	./scripts/infra-status.sh

test-parser:
	./scripts/test-parser.sh

diag-localstack:
	./scripts/diag-localstack.sh

diag-localstack-test:
	./scripts/diag-localstack_test.sh
