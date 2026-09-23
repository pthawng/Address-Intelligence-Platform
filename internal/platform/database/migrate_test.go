package database

import (
	"os"
	"strings"
	"testing"
)

func TestNormalizeDumpedCheckArray(t *testing.T) {
	left := "constraint:places:ck_places_status:CHECK ((status)::text = ANY (ARRAY[('ACTIVE'::character varying)::text, ('INACTIVE'::character varying)::text])):true"
	right := "constraint:places:ck_places_status:CHECK ((status)::text = ANY ((ARRAY['ACTIVE'::character varying, 'INACTIVE'::character varying])::text[])):true"
	if normalizeDumpedCheckArray(left) != normalizeDumpedCheckArray(right) {
		t.Fatal("equivalent restored CHECK arrays differ")
	}
	if normalizeDumpedCheckArray(left) == normalizeDumpedCheckArray(strings.Replace(right, "INACTIVE", "MERGED", 1)) {
		t.Fatal("changed constraint values were accepted")
	}
}

func TestEmbeddedBaselineMatchesImmutableMigration(t *testing.T) {
	original, err := os.ReadFile("../../../migrations/000001_create_core_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	if string(original) != legacySQL {
		t.Fatal("embedded baseline diverged from immutable legacy migration")
	}
}
