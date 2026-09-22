# Database and migration foundation

Goose v3 runs in `cmd/migrate`, separately from runtime, with migration credentials
and a PostgreSQL session lock. Runtime uses pgxpool and never executes migrations.

## Sources and versioning

- `migrations/000001_create_core_schema.sql` is the immutable legacy baseline.
- `internal/platform/database/legacy.sql` is its embedded copy, checked by a test.
- New Goose SQL migrations belong in `internal/platform/database/schema/`.
- `public.goose_db_version` is authoritative. The old `schema_migrations` ledger
  is retained for legacy adoption only.
- Never edit deployed migrations. Add a numbered migration and update
  `database.SchemaVersion` in the same release.
- Each new table/sequence needs explicit grants in its migration and an update
  to `scripts/database_access.sql`. Never grant runtime ledger writes. There are
  intentionally no blanket default grants on all future tables.
- Use transactions by default. Large backfills/nontransactional index operations
  require a separate deployment procedure and timeout budget.

## Fresh local database

Copy `docker/.env.example` to `docker/.env` only if absent; configure its three
passwords. Run from the repository root:

```powershell
docker compose --env-file docker/.env -f docker/compose.yaml -f docker/compose.dev.yaml up --build -d --wait
```

On a new volume, PostgreSQL init installs extensions and provisions roles.
Compose runs the one-shot migrator; API/indexer/worker wait for successful
completion. Migration failure prevents backend startup.

Outside Compose, run `docker/postgres/init/001_extensions.sql` as DBA, followed
by `scripts/database_roles.sql` with psql and `ON_ERROR_STOP=1` on the target
database. The role script reads `DATABASE_PASSWORD` and `MIGRATION_PASSWORD`
from process environment; existing role passwords are not changed.

Configure the runner using `MIGRATION_DATABASE_URL` or split
`MIGRATION_DATABASE_HOST`, `PORT`, `NAME`, `USER`, `PASSWORD`, `SSLMODE`
(all with the `MIGRATION_DATABASE_` prefix). Split fields accept raw passwords;
URL passwords need URL encoding. SSL defaults to `verify-full`; local Compose
uses `disable`. Runtime credentials are never a fallback. CLI does not load .env.

```powershell
go run ./cmd/migrate status
go run ./cmd/migrate up
```

Do not apply the legacy baseline manually to a fresh database.

## Existing legacy volume

Init scripts do not rerun on existing volumes. Stop all writers, take a backup,
and verify restore into a separate database before upgrading. Keep the existing
DBA credentials; changing environment passwords does not rotate stored passwords.
Recreate/start only PostgreSQL first so new role-password environment and script
mounts are available:

```powershell
docker compose --env-file docker/.env -f docker/compose.yaml stop api indexer worker
docker compose --env-file docker/.env -f docker/compose.yaml up -d --wait postgres
docker compose --env-file docker/.env -f docker/compose.yaml exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f /docker-entrypoint-initdb.d/001_extensions.sql -f /scripts/database_roles.sql -f /scripts/database_legacy_owner.sql'
docker compose --env-file docker/.env -f docker/compose.yaml run --build --rm migrate adopt-legacy
docker compose --env-file docker/.env -f docker/compose.yaml -f docker/compose.dev.yaml up --build -d --wait
```

The DBA ownership script transfers only named application tables, their owned
sequences, and trigger functions. It does not transfer the database, extensions,
unrelated objects or objects in other databases. Review its allowlist before using
a shared schema. Adoption verifies ledger and structure before recording the
baseline; existing rows are preserved. Drift/extra objects cause refusal and
require investigation rather than bypassing checks.

## Repair privileges / late role provisioning

Rerun `scripts/database_roles.sql` as DBA to create missing roles and reconcile
existing grants. To repair grants alone:

```powershell
docker compose --env-file docker/.env -f docker/compose.yaml exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f /scripts/database_access.sql'
```

This also works when migration v2 ran before the runtime role existed.
Repeated `migrate up` does not rerun old migrations or repair privileges.

## Deployment and recovery

Runtime requires the exact schema version compiled into the binary. Schema
upgrades require a maintenance window: stop writers, back up, provision, migrate
once, start matching binaries, check readiness, then resume traffic. Mixed-schema
rolling deployment is not supported by this policy.

Migrations are forward-only; no production down command is exposed. Transactional
failures roll back: inspect status, failed version and SQLSTATE, correct the cause,
then rerun. After a committed migration, prefer a corrective forward migration.
Backup recovery requires stopped writers, restore into a separate database,
verification, and switching the database and matching binary together. Define
backup retention/RPO/RTO in the deployment environment; local volumes are not backups.

Use `make db-verify` for rollback-only invariant checks. See
[integration test instructions](../test/integration/README.md) for a disposable
PostGIS test server; never run that suite against a shared/data-bearing server.
