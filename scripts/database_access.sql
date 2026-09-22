-- Run as DBA or application object owner after migration (also safe to rerun).
-- Keep the table allowlist aligned with new migrations; never grant ledger writes.
BEGIN;
GRANT USAGE ON SCHEMA public TO address_runtime;
DO $$
DECLARE item record;
BEGIN
    FOR item IN SELECT c.oid, c.relname FROM pg_class c
        JOIN pg_namespace n ON n.oid=c.relnamespace
        WHERE n.nspname='public' AND c.relkind IN ('r','p')
        AND c.relname = ANY (ARRAY[
            'data_sources','administrative_units','administrative_unit_aliases',
            'administrative_changes','administrative_change_members','places',
            'place_aliases','place_admin_relations','place_relations','geo_boundaries',
            'place_geometries','delivery_points','external_references','outbox_events'])
    LOOP
        EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE public.%I TO address_runtime', item.relname);
        -- Only sequences owned by application columns, including identity columns.
        EXECUTE (
            SELECT coalesce(string_agg(format('GRANT USAGE, SELECT ON SEQUENCE %s TO address_runtime;', s.oid::regclass), ' '), 'SELECT 1')
            FROM pg_class s JOIN pg_depend d ON d.objid=s.oid AND d.classid='pg_class'::regclass
            WHERE s.relkind='S' AND d.refclassid='pg_class'::regclass
              AND d.refobjid=item.oid AND d.deptype IN ('a','i')
        );
    END LOOP;
    FOR item IN SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
        WHERE n.nspname='public' AND c.relname IN ('schema_migrations','goose_db_version') AND c.relkind='r'
    LOOP
        EXECUTE format('REVOKE ALL ON TABLE public.%I FROM address_runtime', item.relname);
        EXECUTE format('GRANT SELECT ON TABLE public.%I TO address_runtime', item.relname);
    END LOOP;
END $$;
COMMIT;
