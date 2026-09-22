#!/bin/sh
set -eu
: "${DATABASE_PASSWORD:?set a runtime password}"
: "${MIGRATION_PASSWORD:?set a migration password}"
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" -f /scripts/database_roles.sql
