\set ON_ERROR_STOP on

BEGIN ISOLATION LEVEL SERIALIZABLE;

DO $$
DECLARE
    source_key BIGINT;
    parent_key BIGINT;
    child_key  BIGINT;
    place_key  BIGINT;
BEGIN
    INSERT INTO data_sources(source_code, source_name, source_type)
    VALUES ('VERIFY_SOURCE', 'Database verification source', 'INTERNAL')
    RETURNING id INTO source_key;

    INSERT INTO administrative_units(
        unit_code, unit_name, normalized_name, unit_type, admin_level,
        valid_from, valid_to, source_id
    ) VALUES (
        'VERIFY-01', 'Verification Province', 'verification province',
        'PROVINCE', 1, DATE '2020-01-01', DATE '2030-01-01', source_key
    ) RETURNING id INTO parent_key;

    BEGIN
        INSERT INTO administrative_units(
            unit_code, unit_name, normalized_name, unit_type, admin_level,
            valid_from, valid_to, source_id
        ) VALUES (
            'VERIFY-01', 'Overlapping Province', 'overlapping province',
            'PROVINCE', 1, DATE '2025-01-01', NULL, source_key
        );
        RAISE EXCEPTION 'temporal overlap was not rejected';
    EXCEPTION
        WHEN exclusion_violation THEN NULL;
    END;

    INSERT INTO administrative_units(
        unit_code, unit_name, normalized_name, unit_type, admin_level,
        parent_id, valid_from, source_id
    ) VALUES (
        'VERIFY-02', 'Verification Ward', 'verification ward',
        'WARD', 2, parent_key, DATE '2020-01-01', source_key
    ) RETURNING id INTO child_key;

    BEGIN
        UPDATE administrative_units SET parent_id = child_key WHERE id = parent_key;
        RAISE EXCEPTION 'administrative hierarchy cycle was not rejected';
    EXCEPTION
        WHEN check_violation THEN NULL;
    END;

    INSERT INTO places(
        place_code, place_name, normalized_name, place_type,
        location, source_id
    ) VALUES (
        'VERIFY-PLACE', 'Verification Street', 'verification street', 'STREET',
        ST_SetSRID(ST_MakePoint(106.7, 10.8), 4326), source_key
    ) RETURNING id INTO place_key;

    BEGIN
        INSERT INTO external_references(
            source_id, external_type, external_id
        ) VALUES (
            source_key, 'WAY', 'invalid-no-target'
        );
        RAISE EXCEPTION 'targetless external reference was not rejected';
    EXCEPTION
        WHEN check_violation THEN NULL;
    END;

    INSERT INTO external_references(
        source_id, place_id, external_type, external_id
    ) VALUES (
        source_key, place_key, 'WAY', 'valid-place-target'
    );

    BEGIN
        INSERT INTO delivery_points(
            place_id, point_type, location, confidence, source_id
        ) VALUES (
            place_key, 'ENTRANCE',
            ST_SetSRID(ST_MakePoint(106.7, 10.8), 4326), 1.1, source_key
        );
        RAISE EXCEPTION 'invalid confidence was not rejected';
    EXCEPTION
        WHEN check_violation THEN NULL;
    END;

    INSERT INTO outbox_events(
        event_type, aggregate_type, aggregate_id, aggregate_revision, payload
    ) VALUES (
        'SEARCH_ENTITY_CHANGED', 'PLACE', place_key, 1, '{"reason":"VERIFY"}'::JSONB
    );

    BEGIN
        INSERT INTO outbox_events(
            event_type, aggregate_type, aggregate_id, aggregate_revision, payload
        ) VALUES (
            'SEARCH_ENTITY_CHANGED', 'PLACE', place_key, 1, '{"reason":"DUPLICATE"}'::JSONB
        );
        RAISE EXCEPTION 'duplicate outbox revision was not rejected';
    EXCEPTION
        WHEN unique_violation THEN NULL;
    END;
END;
$$;

ROLLBACK;

-- Goose is authoritative; schema_migrations retains only the legacy baseline.
SELECT version_id AS goose_schema_version
FROM public.goose_db_version
WHERE is_applied
ORDER BY id DESC
LIMIT 1;

SELECT count(*) AS core_table_count
FROM information_schema.tables
WHERE table_schema = 'public'
  AND table_name IN (
      'data_sources',
      'administrative_units',
      'administrative_unit_aliases',
      'administrative_changes',
      'administrative_change_members',
      'places',
      'place_aliases',
      'place_admin_relations',
      'place_relations',
      'geo_boundaries',
      'place_geometries',
      'delivery_points',
      'external_references',
      'outbox_events'
  );
