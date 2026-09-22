\set ON_ERROR_STOP on
-- Run as DBA on the intended database; passwords come from environment, not argv.
-- Existing passwords are deliberately not changed. Provision before migrations.
\getenv runtime_password DATABASE_PASSWORD
\getenv migration_password MIGRATION_PASSWORD
BEGIN;
SELECT format('CREATE ROLE address_migrator LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD %L', :'migration_password')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='address_migrator') \gexec
SELECT format('CREATE ROLE address_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD %L', :'runtime_password')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='address_runtime') \gexec
\ir database_role_grants.sql
COMMIT;
-- Reconcile existing objects too; fresh schema objects are granted by migration.
\ir database_access.sql
