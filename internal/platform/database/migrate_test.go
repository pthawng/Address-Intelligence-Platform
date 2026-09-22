package database

import (
	"os"
	"testing"
)

func TestEmbeddedBaselineMatchesImmutableMigration(t *testing.T) {
	original, err := os.ReadFile("../../../migrations/000001_create_core_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	if string(original) != legacySQL {
		t.Fatal("embedded baseline diverged from immutable legacy migration")
	}
}
