BEGIN;

CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS btree_gist;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS unaccent;

CREATE TABLE schema_migrations (
    version       BIGINT PRIMARY KEY,
    description   TEXT NOT NULL,
    applied_at    TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE data_sources (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source_code   VARCHAR(50) NOT NULL,
    source_name   VARCHAR(255) NOT NULL,
    source_type   VARCHAR(30) NOT NULL,
    reference     TEXT,
    version       VARCHAR(100),
    published_at  TIMESTAMPTZ,
    imported_at   TIMESTAMPTZ,
    metadata      JSONB NOT NULL DEFAULT '{}'::JSONB,

    CONSTRAINT uq_data_sources_source_code UNIQUE (source_code),
    CONSTRAINT ck_data_sources_source_code_not_blank CHECK (btrim(source_code) <> ''),
    CONSTRAINT ck_data_sources_source_name_not_blank CHECK (btrim(source_name) <> ''),
    CONSTRAINT ck_data_sources_source_type CHECK (
        source_type IN ('OPEN_DATA', 'GOVERNMENT', 'INTERNAL', 'PARTNER')
    ),
    CONSTRAINT ck_data_sources_metadata_object CHECK (jsonb_typeof(metadata) = 'object')
);

CREATE TABLE administrative_units (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    unit_code        VARCHAR(50) NOT NULL,
    unit_name        VARCHAR(255) NOT NULL,
    normalized_name  VARCHAR(255) NOT NULL,
    unit_type        VARCHAR(30) NOT NULL,
    admin_level      SMALLINT NOT NULL,
    parent_id        BIGINT REFERENCES administrative_units(id),
    status           VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    valid_from       DATE,
    valid_to         DATE,
    source_id        BIGINT REFERENCES data_sources(id),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),

    CONSTRAINT ck_administrative_units_code_not_blank CHECK (btrim(unit_code) <> ''),
    CONSTRAINT ck_administrative_units_name_not_blank CHECK (btrim(unit_name) <> ''),
    CONSTRAINT ck_administrative_units_normalized_name_not_blank CHECK (btrim(normalized_name) <> ''),
    CONSTRAINT ck_administrative_units_admin_level CHECK (admin_level > 0),
    CONSTRAINT ck_administrative_units_not_self_parent CHECK (parent_id IS NULL OR parent_id <> id),
    CONSTRAINT ck_administrative_units_status CHECK (
        status IN ('ACTIVE', 'INACTIVE', 'MERGED', 'SUPERSEDED')
    ),
    CONSTRAINT ck_administrative_units_valid_period CHECK (
        valid_to IS NULL OR valid_from IS NULL OR valid_to > valid_from
    ),
    CONSTRAINT ex_administrative_units_no_overlap EXCLUDE USING GIST (
        unit_code WITH =,
        daterange(valid_from, valid_to, '[)') WITH &&
    )
);

CREATE TABLE administrative_unit_aliases (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    admin_unit_id     BIGINT NOT NULL REFERENCES administrative_units(id),
    alias             VARCHAR(255) NOT NULL,
    normalized_alias  VARCHAR(255) NOT NULL,
    alias_type        VARCHAR(30) NOT NULL,
    priority          SMALLINT NOT NULL DEFAULT 0,
    valid_from        DATE,
    valid_to          DATE,
    source_id         BIGINT REFERENCES data_sources(id),
    verified          BOOLEAN NOT NULL DEFAULT FALSE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),

    CONSTRAINT ck_admin_alias_alias_not_blank CHECK (btrim(alias) <> ''),
    CONSTRAINT ck_admin_alias_normalized_not_blank CHECK (btrim(normalized_alias) <> ''),
    CONSTRAINT ck_admin_alias_type CHECK (
        alias_type IN (
            'OFFICIAL', 'OLD_NAME', 'SHORT_NAME', 'ABBREVIATION',
            'COMMON_NAME', 'ALTERNATIVE_SPELLING', 'SEARCH_SYNONYM'
        )
    ),
    CONSTRAINT ck_admin_alias_priority CHECK (priority >= 0),
    CONSTRAINT ck_admin_alias_valid_period CHECK (
        valid_to IS NULL OR valid_from IS NULL OR valid_to > valid_from
    ),
    CONSTRAINT ex_admin_alias_no_overlap EXCLUDE USING GIST (
        admin_unit_id WITH =,
        normalized_alias WITH =,
        alias_type WITH =,
        daterange(valid_from, valid_to, '[)') WITH &&
    )
);

CREATE TABLE administrative_changes (
    id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    change_type         VARCHAR(30) NOT NULL,
    effective_date      DATE NOT NULL,
    resolution_no       VARCHAR(100),
    description         TEXT,
    source_id           BIGINT REFERENCES data_sources(id),
    created_by_user_id  BIGINT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),

    CONSTRAINT ck_administrative_changes_type CHECK (
        change_type IN (
            'RENAME', 'MERGE', 'SPLIT', 'BOUNDARY_ADJUSTMENT',
            'LEVEL_CHANGE', 'CODE_CHANGE', 'CREATE', 'DISSOLVE', 'OTHER'
        )
    )
);

CREATE TABLE administrative_change_members (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    change_id      BIGINT NOT NULL REFERENCES administrative_changes(id),
    admin_unit_id  BIGINT NOT NULL REFERENCES administrative_units(id),
    role           VARCHAR(10) NOT NULL,
    note           TEXT,

    CONSTRAINT uq_administrative_change_member UNIQUE (change_id, admin_unit_id, role),
    CONSTRAINT ck_administrative_change_member_role CHECK (role IN ('SOURCE', 'TARGET'))
);

CREATE TABLE places (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    place_code       VARCHAR(100),
    place_name       VARCHAR(255) NOT NULL,
    normalized_name  VARCHAR(255) NOT NULL,
    place_type       VARCHAR(40) NOT NULL,
    parent_place_id  BIGINT REFERENCES places(id),
    location         geometry(Point, 4326),
    status           VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    valid_from       DATE,
    valid_to         DATE,
    source_id        BIGINT REFERENCES data_sources(id),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),

    CONSTRAINT ck_places_code_not_blank CHECK (place_code IS NULL OR btrim(place_code) <> ''),
    CONSTRAINT ck_places_name_not_blank CHECK (btrim(place_name) <> ''),
    CONSTRAINT ck_places_normalized_name_not_blank CHECK (btrim(normalized_name) <> ''),
    CONSTRAINT ck_places_type CHECK (
        place_type IN (
            'STREET', 'ALLEY', 'HAMLET', 'VILLAGE', 'RESIDENTIAL_AREA',
            'BUILDING', 'APARTMENT_COMPLEX', 'INDUSTRIAL_ZONE', 'MARKET',
            'SCHOOL', 'HOSPITAL', 'LANDMARK', 'POI'
        )
    ),
    CONSTRAINT ck_places_not_self_parent CHECK (parent_place_id IS NULL OR parent_place_id <> id),
    CONSTRAINT ck_places_status CHECK (
        status IN ('ACTIVE', 'INACTIVE', 'MERGED', 'SUPERSEDED')
    ),
    CONSTRAINT ck_places_valid_period CHECK (
        valid_to IS NULL OR valid_from IS NULL OR valid_to > valid_from
    )
);

CREATE UNIQUE INDEX uq_places_source_code
    ON places(source_id, place_code)
    WHERE place_code IS NOT NULL;

CREATE TABLE place_aliases (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    place_id          BIGINT NOT NULL REFERENCES places(id),
    alias             VARCHAR(255) NOT NULL,
    normalized_alias  VARCHAR(255) NOT NULL,
    alias_type        VARCHAR(30) NOT NULL,
    priority          SMALLINT NOT NULL DEFAULT 0,
    source_id         BIGINT REFERENCES data_sources(id),
    verified          BOOLEAN NOT NULL DEFAULT FALSE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),

    CONSTRAINT uq_place_alias UNIQUE (place_id, normalized_alias, alias_type),
    CONSTRAINT ck_place_alias_alias_not_blank CHECK (btrim(alias) <> ''),
    CONSTRAINT ck_place_alias_normalized_not_blank CHECK (btrim(normalized_alias) <> ''),
    CONSTRAINT ck_place_alias_type CHECK (
        alias_type IN (
            'OFFICIAL', 'OLD_NAME', 'SHORT_NAME', 'ABBREVIATION',
            'COMMON_NAME', 'ALTERNATIVE_SPELLING', 'SEARCH_SYNONYM'
        )
    ),
    CONSTRAINT ck_place_alias_priority CHECK (priority >= 0)
);

CREATE TABLE place_admin_relations (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    place_id       BIGINT NOT NULL REFERENCES places(id),
    admin_unit_id  BIGINT NOT NULL REFERENCES administrative_units(id),
    relation_type  VARCHAR(30) NOT NULL,
    is_primary     BOOLEAN NOT NULL DEFAULT FALSE,
    valid_from     DATE,
    valid_to       DATE,
    source_id      BIGINT REFERENCES data_sources(id),

    CONSTRAINT ck_place_admin_relation_type CHECK (
        relation_type IN ('WITHIN', 'INTERSECTS', 'SERVES', 'NEAR')
    ),
    CONSTRAINT ck_place_admin_relation_valid_period CHECK (
        valid_to IS NULL OR valid_from IS NULL OR valid_to > valid_from
    ),
    CONSTRAINT ex_place_admin_relation_no_overlap EXCLUDE USING GIST (
        place_id WITH =,
        admin_unit_id WITH =,
        relation_type WITH =,
        daterange(valid_from, valid_to, '[)') WITH &&
    )
);

CREATE UNIQUE INDEX uq_place_admin_primary_current
    ON place_admin_relations(place_id)
    WHERE is_primary AND valid_to IS NULL;

CREATE TABLE place_relations (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source_place_id  BIGINT NOT NULL REFERENCES places(id),
    target_place_id  BIGINT NOT NULL REFERENCES places(id),
    relation_type    VARCHAR(30) NOT NULL,
    source_id        BIGINT REFERENCES data_sources(id),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),

    CONSTRAINT uq_place_relation UNIQUE (source_place_id, target_place_id, relation_type),
    CONSTRAINT ck_place_relation_not_self CHECK (source_place_id <> target_place_id),
    CONSTRAINT ck_place_relation_type CHECK (
        relation_type IN ('ENTRANCE_OF', 'PART_OF', 'CONNECTED_TO', 'NEAR')
    )
);

CREATE TABLE geo_boundaries (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    admin_unit_id  BIGINT NOT NULL REFERENCES administrative_units(id),
    boundary       geometry(MultiPolygon, 4326) NOT NULL,
    centroid       geometry(Point, 4326),
    valid_from     DATE,
    valid_to       DATE,
    source_id      BIGINT REFERENCES data_sources(id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),

    CONSTRAINT ck_geo_boundaries_valid CHECK (ST_IsValid(boundary)),
    CONSTRAINT ck_geo_boundaries_valid_period CHECK (
        valid_to IS NULL OR valid_from IS NULL OR valid_to > valid_from
    ),
    CONSTRAINT ex_geo_boundaries_no_overlap EXCLUDE USING GIST (
        admin_unit_id WITH =,
        daterange(valid_from, valid_to, '[)') WITH &&
    )
);

CREATE TABLE place_geometries (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    place_id       BIGINT NOT NULL REFERENCES places(id),
    geometry_type  VARCHAR(20) NOT NULL,
    geometry       geometry(Geometry, 4326) NOT NULL,
    is_primary     BOOLEAN NOT NULL DEFAULT FALSE,
    valid_from     DATE,
    valid_to       DATE,
    source_id      BIGINT REFERENCES data_sources(id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),

    CONSTRAINT ck_place_geometries_type CHECK (
        geometry_type IN ('POINT', 'LINESTRING', 'POLYGON', 'MULTIPOLYGON')
    ),
    CONSTRAINT ck_place_geometries_type_matches CHECK (
        upper(GeometryType(geometry)) = geometry_type
    ),
    CONSTRAINT ck_place_geometries_valid CHECK (ST_IsValid(geometry)),
    CONSTRAINT ck_place_geometries_valid_period CHECK (
        valid_to IS NULL OR valid_from IS NULL OR valid_to > valid_from
    )
);

CREATE UNIQUE INDEX uq_place_geometries_primary_current
    ON place_geometries(place_id)
    WHERE is_primary AND valid_to IS NULL;

CREATE TABLE delivery_points (
    id                     BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    place_id               BIGINT REFERENCES places(id),
    point_type             VARCHAR(30) NOT NULL,
    location               geometry(Point, 4326) NOT NULL,
    confidence             NUMERIC(5, 4),
    verification_status    VARCHAR(20) NOT NULL DEFAULT 'UNVERIFIED',
    source_id              BIGINT REFERENCES data_sources(id),
    successful_deliveries  BIGINT NOT NULL DEFAULT 0,
    last_verified_at       TIMESTAMPTZ,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),

    CONSTRAINT ck_delivery_points_type CHECK (
        point_type IN (
            'ENTRANCE', 'DELIVERY', 'PICKUP', 'LOADING_GATE',
            'WAREHOUSE_GATE', 'ACCESS_POINT'
        )
    ),
    CONSTRAINT ck_delivery_points_confidence CHECK (
        confidence IS NULL OR confidence BETWEEN 0 AND 1
    ),
    CONSTRAINT ck_delivery_points_verification CHECK (
        verification_status IN ('UNVERIFIED', 'VERIFIED', 'REJECTED')
    ),
    CONSTRAINT ck_delivery_points_successful_deliveries CHECK (successful_deliveries >= 0)
);

CREATE TABLE external_references (
    id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source_id          BIGINT NOT NULL REFERENCES data_sources(id),
    admin_unit_id      BIGINT REFERENCES administrative_units(id),
    place_id           BIGINT REFERENCES places(id),
    geo_boundary_id    BIGINT REFERENCES geo_boundaries(id),
    delivery_point_id  BIGINT REFERENCES delivery_points(id),
    external_type      VARCHAR(30) NOT NULL,
    external_id        VARCHAR(100) NOT NULL,
    external_version   VARCHAR(100),
    last_synced_at     TIMESTAMPTZ,
    metadata           JSONB NOT NULL DEFAULT '{}'::JSONB,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),

    CONSTRAINT uq_external_reference_source_object UNIQUE (source_id, external_type, external_id),
    CONSTRAINT ck_external_reference_exactly_one_entity CHECK (
        num_nonnulls(admin_unit_id, place_id, geo_boundary_id, delivery_point_id) = 1
    ),
    CONSTRAINT ck_external_reference_type_not_blank CHECK (btrim(external_type) <> ''),
    CONSTRAINT ck_external_reference_id_not_blank CHECK (btrim(external_id) <> ''),
    CONSTRAINT ck_external_reference_metadata_object CHECK (jsonb_typeof(metadata) = 'object')
);

CREATE TABLE outbox_events (
    id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_type          VARCHAR(100) NOT NULL,
    aggregate_type      VARCHAR(50) NOT NULL,
    aggregate_id        BIGINT NOT NULL,
    aggregate_revision  BIGINT NOT NULL,
    payload             JSONB NOT NULL,
    occurred_at         TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    available_at        TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    processed_at        TIMESTAMPTZ,
    attempt_count       INTEGER NOT NULL DEFAULT 0,
    locked_at           TIMESTAMPTZ,
    locked_by           VARCHAR(100),
    last_error          TEXT,

    CONSTRAINT uq_outbox_event_revision UNIQUE (
        event_type, aggregate_type, aggregate_id, aggregate_revision
    ),
    CONSTRAINT ck_outbox_event_type_not_blank CHECK (btrim(event_type) <> ''),
    CONSTRAINT ck_outbox_aggregate_type_not_blank CHECK (btrim(aggregate_type) <> ''),
    CONSTRAINT ck_outbox_aggregate_revision CHECK (aggregate_revision > 0),
    CONSTRAINT ck_outbox_payload_object CHECK (jsonb_typeof(payload) = 'object'),
    CONSTRAINT ck_outbox_attempt_count CHECK (attempt_count >= 0),
    CONSTRAINT ck_outbox_lock_pair CHECK (
        (locked_at IS NULL AND locked_by IS NULL) OR
        (locked_at IS NOT NULL AND locked_by IS NOT NULL)
    )
);

CREATE FUNCTION set_updated_at() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = clock_timestamp();
    RETURN NEW;
END;
$$;

CREATE FUNCTION prevent_administrative_unit_cycle() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
DECLARE
    cycle_found BOOLEAN;
BEGIN
    IF NEW.parent_id IS NULL THEN
        RETURN NEW;
    END IF;

    WITH RECURSIVE ancestors AS (
        SELECT id, parent_id
        FROM administrative_units
        WHERE id = NEW.parent_id
        UNION ALL
        SELECT unit.id, unit.parent_id
        FROM administrative_units unit
        JOIN ancestors parent ON unit.id = parent.parent_id
    )
    SELECT EXISTS (SELECT 1 FROM ancestors WHERE id = NEW.id)
    INTO cycle_found;

    IF cycle_found THEN
        RAISE EXCEPTION 'administrative unit hierarchy cycle detected for id %', NEW.id
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE FUNCTION prevent_place_cycle() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
DECLARE
    cycle_found BOOLEAN;
BEGIN
    IF NEW.parent_place_id IS NULL THEN
        RETURN NEW;
    END IF;

    WITH RECURSIVE ancestors AS (
        SELECT id, parent_place_id
        FROM places
        WHERE id = NEW.parent_place_id
        UNION ALL
        SELECT place.id, place.parent_place_id
        FROM places place
        JOIN ancestors parent ON place.id = parent.parent_place_id
    )
    SELECT EXISTS (SELECT 1 FROM ancestors WHERE id = NEW.id)
    INTO cycle_found;

    IF cycle_found THEN
        RAISE EXCEPTION 'place hierarchy cycle detected for id %', NEW.id
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_administrative_units_updated_at
BEFORE UPDATE ON administrative_units
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_places_updated_at
BEFORE UPDATE ON places
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_geo_boundaries_updated_at
BEFORE UPDATE ON geo_boundaries
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_place_geometries_updated_at
BEFORE UPDATE ON place_geometries
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_delivery_points_updated_at
BEFORE UPDATE ON delivery_points
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_external_references_updated_at
BEFORE UPDATE ON external_references
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_administrative_units_prevent_cycle
BEFORE INSERT OR UPDATE OF parent_id ON administrative_units
FOR EACH ROW EXECUTE FUNCTION prevent_administrative_unit_cycle();

CREATE TRIGGER trg_places_prevent_cycle
BEFORE INSERT OR UPDATE OF parent_place_id ON places
FOR EACH ROW EXECUTE FUNCTION prevent_place_cycle();

CREATE INDEX idx_admin_units_parent ON administrative_units(parent_id);
CREATE INDEX idx_admin_units_name ON administrative_units(normalized_name);
CREATE INDEX idx_admin_units_name_trgm ON administrative_units USING GIN(normalized_name gin_trgm_ops);
CREATE INDEX idx_admin_alias_normalized ON administrative_unit_aliases(normalized_alias);
CREATE INDEX idx_admin_alias_normalized_trgm ON administrative_unit_aliases USING GIN(normalized_alias gin_trgm_ops);
CREATE INDEX idx_admin_change_members_unit ON administrative_change_members(admin_unit_id);

CREATE INDEX idx_places_parent ON places(parent_place_id);
CREATE INDEX idx_places_type_status ON places(place_type, status);
CREATE INDEX idx_places_name_trgm ON places USING GIN(normalized_name gin_trgm_ops);
CREATE INDEX idx_places_location_gist ON places USING GIST(location);
CREATE INDEX idx_place_alias_normalized ON place_aliases(normalized_alias);
CREATE INDEX idx_place_alias_normalized_trgm ON place_aliases USING GIN(normalized_alias gin_trgm_ops);
CREATE INDEX idx_place_admin_place ON place_admin_relations(place_id);
CREATE INDEX idx_place_admin_unit ON place_admin_relations(admin_unit_id);
CREATE INDEX idx_place_relations_target ON place_relations(target_place_id);

CREATE INDEX idx_geo_boundaries_boundary_gist ON geo_boundaries USING GIST(boundary);
CREATE INDEX idx_place_geometries_geometry_gist ON place_geometries USING GIST(geometry);
CREATE INDEX idx_delivery_points_location_gist ON delivery_points USING GIST(location);

CREATE INDEX idx_external_references_admin_unit ON external_references(admin_unit_id)
    WHERE admin_unit_id IS NOT NULL;
CREATE INDEX idx_external_references_place ON external_references(place_id)
    WHERE place_id IS NOT NULL;
CREATE INDEX idx_external_references_geo_boundary ON external_references(geo_boundary_id)
    WHERE geo_boundary_id IS NOT NULL;
CREATE INDEX idx_external_references_delivery_point ON external_references(delivery_point_id)
    WHERE delivery_point_id IS NOT NULL;

CREATE INDEX idx_outbox_events_pending
    ON outbox_events(available_at, id)
    WHERE processed_at IS NULL;
CREATE INDEX idx_outbox_events_stale_locks
    ON outbox_events(locked_at)
    WHERE processed_at IS NULL AND locked_at IS NOT NULL;

INSERT INTO schema_migrations(version, description)
VALUES (1, 'create place-centric canonical schema and transactional outbox');

COMMIT;
