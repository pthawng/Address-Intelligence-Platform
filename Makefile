.PHONY: test vet fmt run-api run-indexer run-worker docker-config docker-up docker-down docker-logs db-bootstrap db-verify

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w cmd internal

run-api:
	go run ./cmd/api

run-indexer:
	go run ./cmd/indexer

run-worker:
	go run ./cmd/worker

docker-config:
	docker compose --env-file .env.docker config --quiet

docker-up:
	docker compose --env-file .env.docker up --build -d

docker-down:
	docker compose --env-file .env.docker down

docker-logs:
	docker compose --env-file .env.docker logs --follow

db-bootstrap:
	docker exec address-intelligence-platform-postgres-1 psql -v ON_ERROR_STOP=1 -U address_app -d address_intelligence -f /migrations/000001_create_core_schema.sql

db-verify:
	docker exec address-intelligence-platform-postgres-1 psql -v ON_ERROR_STOP=1 -U address_app -d address_intelligence -f /scripts/verify_database.sql
