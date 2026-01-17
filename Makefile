.PHONY: infra-up infra-init db-migrate infra-status test-parser test-e2e test-e2e-zolo run-zolo diag-localstack diag-localstack-test app-smoke readapi readapi-test test-readapi-integration e2e-proof run-local run run-dev scrape nuke-test nuke-dev

STACK ?= run
ifeq ($(STACK),run)
COMPOSE = docker compose -f infra/docker-compose.yml
else
COMPOSE = docker compose -p realtoragent-$(STACK) -f infra/docker-compose.yml
endif
export STACK

infra-up:
	$(COMPOSE) up -d --remove-orphans

infra-init:
	./scripts/init-localstack.sh

db-migrate:
	./scripts/db-migrate.sh

infra-status:
	./scripts/infra-status.sh

test-parser:
	STACK=test ./scripts/test-parser.sh

test-e2e:
	STACK=test ./scripts/test-e2e-zolo.sh

test-e2e-zolo:
	STACK=test ./scripts/test-e2e-zolo.sh

run-zolo:
	bash scripts/run-zolo-manual.sh

app-smoke:
	STACK=test bash scripts/app-smoke.sh

readapi-test:
	cd services/readapi && go test ./...

test-readapi-integration:
	mkdir -p tmp/go-cache tmp/go-tmp
	bash scripts/ensure-it-db.sh
	DB_ENABLED=true POSTGRES_HOST=127.0.0.1 POSTGRES_PORT=5432 POSTGRES_DB=realestate_it POSTGRES_USER=realestate POSTGRES_PASSWORD=realestate GOCACHE=$$PWD/tmp/go-cache GOTMPDIR=$$PWD/tmp/go-tmp go -C services/readapi test -tags=integration ./... -count=1

readapi:
	bash scripts/run-readapi.sh

e2e-proof:
	STACK=test bash scripts/e2e-proof.sh

run-local:
	STACK=test bash scripts/run-local.sh

run:
	bash scripts/run.sh

run-dev:
	STACK=dev bash scripts/run.sh

scrape:
	bash scripts/scrape.sh

diag-localstack:
	./scripts/diag-localstack.sh

diag-localstack-test:
	./scripts/diag-localstack_test.sh

nuke-test: STACK = test
nuke-test:
	@echo "WARNING: this will delete realtoragent-test containers and volumes."
	$(COMPOSE) down -v

nuke-dev: STACK = dev
nuke-dev:
	@echo "WARNING: this will delete realtoragent-dev containers and volumes (real data)."
	$(COMPOSE) down -v
