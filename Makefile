COMPOSE = docker compose --env-file docker/.env -f docker/compose.yaml
DEV_COMPOSE = $(COMPOSE) -f docker/compose.dev.yaml

.PHONY: test vet fmt deps-check deps-verify vuln run-api run-indexer run-worker docker-config docker-up docker-down docker-logs db-bootstrap db-verify

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w cmd internal tools test

deps-check:
	go run ./tools/dependencycheck

deps-verify:
	go mod tidy -diff
	go mod verify

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...

run-api:
	go run ./cmd/api

run-indexer:
	go run ./cmd/indexer

run-worker:
	go run ./cmd/worker

docker-config:
	$(DEV_COMPOSE) config --quiet

docker-up:
	$(DEV_COMPOSE) up --build -d --wait

docker-down:
	$(DEV_COMPOSE) down

docker-logs:
	$(DEV_COMPOSE) logs --follow

db-bootstrap:
	go run ./cmd/migrate up

db-verify:
	$(COMPOSE) exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -U "$$POSTGRES_USER" -d "$$POSTGRES_DB" -f /scripts/verify_database.sql'

.PHONY: db-status db-adopt-legacy test-db
db-status:
	go run ./cmd/migrate status
db-adopt-legacy:
	go run ./cmd/migrate adopt-legacy
test-db:
	go test -count=1 ./test/integration

.PHONY: docker-runtime-up docker-worker-up
docker-runtime-up:
	$(COMPOSE) up --build -d --wait

docker-worker-up:
	$(DEV_COMPOSE) --profile worker up --build -d --wait
