-- DBA-only, explicit legacy adoption step. Back up first and stop writers.
-- Transfers only known application objects in this database, never extensions,
-- unrelated objects, the database itself, or objects in another database.
BEGIN;
DO $$
DECLARE item record;
BEGIN
    IF to_regclass('public.schema_migrations') IS NULL THEN
        RAISE EXCEPTION 'legacy schema_migrations is required';
    END IF;
    FOR item IN SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
        WHERE n.nspname='public' AND c.relkind IN ('r','p')
        AND c.relname = ANY (ARRAY[
            'schema_migrations','goose_db_version','data_sources','administrative_units',
            'administrative_unit_aliases','administrative_changes','administrative_change_members',
            'places','place_aliases','place_admin_relations','place_relations','geo_boundaries',
            'place_geometries','delivery_points','external_references','outbox_events'])
        AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.classid='pg_class'::regclass AND d.objid=c.oid AND d.deptype='e')
    LOOP
        -- PostgreSQL also transfers sequences owned by these table columns.
        EXECUTE format('ALTER TABLE public.%I OWNER TO address_migrator', item.relname);
    END LOOP;
    FOR item IN SELECT p.oid::regprocedure AS signature FROM pg_proc p
        JOIN pg_namespace n ON n.oid=p.pronamespace
        WHERE n.nspname='public' AND p.pronargs=0 AND p.prorettype='trigger'::regtype
        AND p.proname IN ('set_updated_at','prevent_administrative_unit_cycle','prevent_place_cycle')
        AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.classid='pg_proc'::regclass AND d.objid=p.oid AND d.deptype='e')
    LOOP
        EXECUTE format('ALTER FUNCTION %s OWNER TO address_migrator', item.signature);
    END LOOP;
END $$;
COMMIT;
