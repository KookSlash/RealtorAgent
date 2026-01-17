.PHONY: infra-up infra-init db-migrate infra-status test-parser test-e2e test-e2e-zolo run-zolo diag-localstack diag-localstack-test app-smoke readapi-test e2e-proof run-local

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

test-e2e:
	./scripts/test-e2e-zolo.sh

test-e2e-zolo:
	./scripts/test-e2e-zolo.sh

run-zolo:
	bash scripts/run-zolo-manual.sh

app-smoke:
	bash scripts/app-smoke.sh

readapi-test:
	cd services/readapi && go test ./...

e2e-proof:
	bash scripts/e2e-proof.sh

run-local:
	bash scripts/run-local.sh

diag-localstack:
	./scripts/diag-localstack.sh

diag-localstack-test:
	./scripts/diag-localstack_test.sh
