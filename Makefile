SHELL := /bin/bash
.ONESHELL:
.SHELLFLAGS := -euo pipefail -c

INFRA_COMPOSE := infra/docker-compose.yml
INFRA_ENV     := infra/.env

.PHONY: help infra-up infra-down infra-restart infra-logs infra-status infra-init infra-smoke reset

help:
	@echo "Targets:"
	@echo "  make infra-up       # start LocalStack + Postgres"
	@echo "  make infra-down     # stop infra"
	@echo "  make infra-restart  # restart infra"
	@echo "  make infra-logs     # tail infra logs"
	@echo "  make infra-status   # show infra status + LocalStack health"
	@echo "  make infra-init     # create buckets/queue + S3->SQS notification"
	@echo "  make infra-smoke    # upload test object and read SQS events"
	@echo "  make reset          # WARNING: wipe volumes, recreate infra and resources"

infra-up:
	docker compose -f $(INFRA_COMPOSE) --env-file $(INFRA_ENV) up -d
	@echo "Infra started."

infra-down:
	docker compose -f $(INFRA_COMPOSE) --env-file $(INFRA_ENV) down
	@echo "Infra stopped."

infra-restart: infra-down infra-up

infra-logs:
	docker compose -f $(INFRA_COMPOSE) --env-file $(INFRA_ENV) logs -f --tail=200

infra-status:
	./scripts/infra-status.sh

infra-init:
	./scripts/init-localstack.sh
	@echo "LocalStack resources initialized."

infra-smoke:
	./scripts/smoke-s3-sqs.sh

reset:
	@echo "This will delete docker volumes for LocalStack and Postgres."
	@echo "Press Ctrl+C to abort, or wait 3 seconds to continue..."
	sleep 3
	docker compose -f $(INFRA_COMPOSE) --env-file $(INFRA_ENV) down -v
	docker compose -f $(INFRA_COMPOSE) --env-file $(INFRA_ENV) up -d
	./scripts/init-localstack.sh
	./scripts/smoke-s3-sqs.sh

.PHONY: db-migrate db-psql

db-migrate:
	./scripts/db-migrate.sh

db-psql:
	@set -a; source infra/.env; set +a; \
	docker exec -it postgres psql -U "$$POSTGRES_USER" -d "$$POSTGRES_DB"
