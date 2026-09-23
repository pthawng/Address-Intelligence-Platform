# Integration tests

Database tests require a dedicated disposable PostgreSQL 17/PostGIS 3.5 server.
They create/drop random databases and fixed cluster roles `address_runtime` and
`address_migrator`. Never use a shared/production/development data server or run
multiple suites concurrently against the same server.

```powershell
docker run -d --name aip-integration --publish 127.0.0.1:55432:5432 --env POSTGRES_PASSWORD=integration-test-only postgis/postgis:17-3.5
# Wait until: docker exec aip-integration pg_isready -U postgres
$env:TEST_DATABASE_URL='postgres://postgres:integration-test-only@localhost:55432/postgres?sslmode=disable'
go test -count=1 ./test/integration
docker rm -f -v aip-integration
```

Without TEST_DATABASE_URL, tests skip. CI supplies a disposable PostGIS service
and runs with the race detector. Coverage includes fresh/adopted schema,
drift rejection, concurrent migration, rollback, serializable hierarchy writes,
non-superuser migrator, ownership transfer, repeatable grant repair and runtime
denial of DDL/ledger writes. SQL invariant checks live in
`scripts/verify_database.sql`.
The source-import integration test also checks all six snapshots, canonical
administrative hierarchy, unresolved/invalid level-4 records, provenance,
outbox counts and idempotent reruns.
