-- +goose Up
SET LOCAL search_path = public;
-- SSI prevents write-skew cycles that a recursive trigger alone cannot detect.
LOCK TABLE administrative_units, places IN SHARE ROW EXCLUSIVE MODE;

-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        WITH RECURSIVE tree AS (
            SELECT id, parent_id FROM administrative_units
            UNION ALL
            SELECT p.id,p.parent_id FROM administrative_units p JOIN tree t ON p.id=t.parent_id
        ) CYCLE id SET is_cycle USING path
        SELECT 1 FROM tree WHERE is_cycle
    ) OR EXISTS (
        WITH RECURSIVE tree AS (
            SELECT id,parent_place_id FROM places
            UNION ALL
            SELECT p.id,p.parent_place_id FROM places p JOIN tree t ON p.id=t.parent_place_id
        ) CYCLE id SET is_cycle USING path
        SELECT 1 FROM tree WHERE is_cycle
    ) THEN
        RAISE EXCEPTION 'existing hierarchy cycle must be repaired before migration';
    END IF;
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
DO $$
DECLARE item record;
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname='address_runtime') THEN
        GRANT USAGE ON SCHEMA public TO address_runtime;
        FOR item IN SELECT tablename FROM pg_tables WHERE schemaname='public'
            AND tablename NOT IN ('goose_db_version','schema_migrations','spatial_ref_sys')
        LOOP
            EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE public.%I TO address_runtime',item.tablename);
        END LOOP;
        GRANT SELECT ON public.goose_db_version, public.schema_migrations TO address_runtime;
        GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO address_runtime;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION prevent_administrative_unit_cycle() RETURNS TRIGGER
LANGUAGE plpgsql SET search_path = pg_catalog, public AS $$
DECLARE cycle_found BOOLEAN;
BEGIN
    IF NEW.parent_id IS NULL THEN RETURN NEW; END IF;
    IF current_setting('transaction_isolation') <> 'serializable' THEN
        RAISE EXCEPTION 'hierarchy writes require a serializable transaction' USING ERRCODE='25001';
    END IF;
    WITH RECURSIVE ancestors AS (
        SELECT id,parent_id FROM administrative_units WHERE id=NEW.parent_id
        UNION
        SELECT u.id,u.parent_id FROM administrative_units u JOIN ancestors p ON u.id=p.parent_id
    ) SELECT EXISTS(SELECT 1 FROM ancestors WHERE id=NEW.id) INTO cycle_found;
    IF cycle_found THEN RAISE EXCEPTION 'administrative hierarchy cycle' USING ERRCODE='23514'; END IF;
    RETURN NEW;
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION prevent_place_cycle() RETURNS TRIGGER
LANGUAGE plpgsql SET search_path = pg_catalog, public AS $$
DECLARE cycle_found BOOLEAN;
BEGIN
    IF NEW.parent_place_id IS NULL THEN RETURN NEW; END IF;
    IF current_setting('transaction_isolation') <> 'serializable' THEN
        RAISE EXCEPTION 'hierarchy writes require a serializable transaction' USING ERRCODE='25001';
    END IF;
    WITH RECURSIVE ancestors AS (
        SELECT id,parent_place_id FROM places WHERE id=NEW.parent_place_id
        UNION
        SELECT p.id,p.parent_place_id FROM places p JOIN ancestors a ON p.id=a.parent_place_id
    ) SELECT EXISTS(SELECT 1 FROM ancestors WHERE id=NEW.id) INTO cycle_found;
    IF cycle_found THEN RAISE EXCEPTION 'place hierarchy cycle' USING ERRCODE='23514'; END IF;
    RETURN NEW;
END $$;
-- +goose StatementEnd
