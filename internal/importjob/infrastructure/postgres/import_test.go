package postgres

import (
	"path/filepath"
	"testing"
)

func TestSnapshotsParseAndPreserveUnresolvedPlaces(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "..", "docs", "data")
	inputs := make([]input, 0, len(snapshots))
	admin, places := 0, 0
	for _, spec := range snapshots {
		in, err := readInput(dir, spec)
		if err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, in)
		for _, r := range in.rows {
			if r.level == 4 {
				places++
			} else {
				admin++
			}
		}
	}
	if err := validate(inputs); err != nil {
		t.Fatal(err)
	}
	if admin != 15290 || places != 171291 {
		t.Fatalf("admin=%d level4=%d", admin, places)
	}
}

func TestRejectUnsupportedSourceSQL(t *testing.T) {
	for _, line := range []string{
		"DELETE FROM administrative_units;",
		"INSERT INTO admin_units (unit_code, unit_name, unit_level, area_type) VALUES ('x', 'Xã X', 3, 2) ON CONFLICT (unit_code) DO NOTHING;",
	} {
		if _, err := parseLine(line, 1); err == nil {
			t.Fatalf("accepted %q", line)
		}
	}
}
