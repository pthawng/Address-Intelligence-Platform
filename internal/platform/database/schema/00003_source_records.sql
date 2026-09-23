-- +goose Up
SET LOCAL search_path = public;

-- Source records retain the input identity even when canonical resolution is
-- impossible. A line is unique within an immutable, checksum-versioned source.
CREATE TABLE source_records (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source_id BIGINT NOT NULL REFERENCES data_sources(id),
    source_line INTEGER NOT NULL,
    external_id VARCHAR(100) NOT NULL,
    raw_name TEXT NOT NULL,
    raw_level SMALLINT NOT NULL,
    raw_parent_id VARCHAR(100),
    area_type SMALLINT NOT NULL,
    unit_type VARCHAR(30),
    canonical_admin_unit_id BIGINT REFERENCES administrative_units(id),
    canonical_place_id BIGINT REFERENCES places(id),
    resolution_status VARCHAR(20) NOT NULL DEFAULT 'STAGED',
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT uq_source_records_line UNIQUE (source_id, source_line),
    CONSTRAINT ck_source_records_line CHECK (source_line > 0),
    CONSTRAINT ck_source_records_required_input CHECK (
        resolution_status = 'INVALID' OR (btrim(external_id) <> '' AND btrim(raw_name) <> '')
    ),
    CONSTRAINT ck_source_records_level CHECK (raw_level BETWEEN 1 AND 4),
    CONSTRAINT ck_source_records_area CHECK (area_type IN (1, 2)),
    CONSTRAINT ck_source_records_unit_type CHECK ((raw_level < 4) = (unit_type IS NOT NULL)),
    CONSTRAINT ck_source_records_resolution CHECK (
        (resolution_status = 'STAGED' AND num_nonnulls(canonical_admin_unit_id, canonical_place_id) = 0) OR
        (resolution_status = 'LINKED' AND num_nonnulls(canonical_admin_unit_id, canonical_place_id) = 1) OR
        (resolution_status IN ('UNRESOLVED', 'INVALID') AND num_nonnulls(canonical_admin_unit_id, canonical_place_id) = 0)
    )
);

CREATE INDEX idx_source_records_external ON source_records(source_id, external_id);
CREATE INDEX idx_source_records_unresolved ON source_records(source_id, raw_level)
    WHERE resolution_status = 'UNRESOLVED';

-- The migration role owns the table. The runtime role can ingest and resolve
-- source records but has no migration-ledger write privilege.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'address_runtime') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON source_records TO address_runtime;
        GRANT USAGE, SELECT ON SEQUENCE source_records_id_seq TO address_runtime;
    END IF;
END $$;
-- +goose StatementEnd
