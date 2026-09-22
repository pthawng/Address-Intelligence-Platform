-- Run as DBA; shared by role provisioning and integration tests.
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT USAGE, CREATE ON SCHEMA public TO address_migrator;
DO $$
BEGIN
    EXECUTE format('GRANT CONNECT, CREATE ON DATABASE %I TO address_migrator', current_database());
    EXECUTE format('GRANT CONNECT ON DATABASE %I TO address_runtime', current_database());
END $$;
