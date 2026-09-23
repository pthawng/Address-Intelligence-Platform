package integration

import (
	"path/filepath"
	"testing"

	sourceimport "address-intelligence-platform/internal/importjob/infrastructure/postgres"
	"address-intelligence-platform/internal/platform/database"
)

func TestLegacySourceImport(t *testing.T) {
	ctx, raw, conn := isolatedDB(t)
	p, err := database.NewMigrator(ctx, raw, false)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, err := p.Up(ctx); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join("..", "..", "docs", "data")
	count, unchanged, err := sourceimport.Import(ctx, raw, dir)
	if err != nil {
		t.Fatal(err)
	}
	if count != 15290 || unchanged {
		t.Fatalf("count=%d unchanged=%v", count, unchanged)
	}
	count, unchanged, err = sourceimport.Import(ctx, raw, dir)
	if err != nil || count != 0 || !unchanged {
		t.Fatalf("rerun: count=%d unchanged=%v error=%v", count, unchanged, err)
	}
	for _, check := range []struct {
		query    string
		expected int
	}{
		{`SELECT count(*) FROM source_records`, 186581},
		{`SELECT count(*) FROM source_records WHERE resolution_status='LINKED'`, 15290},
		{`SELECT count(*) FROM source_records WHERE resolution_status='UNRESOLVED'`, 171284},
		{`SELECT count(*) FROM source_records WHERE resolution_status='INVALID'`, 7},
		{`SELECT count(*) FROM administrative_units`, 15290},
		{`SELECT count(*) FROM administrative_units WHERE parent_id IS NULL`, 97},
		{`SELECT count(*) FROM external_references WHERE external_type='ADMIN_UNIT'`, 15290},
		{`SELECT count(*) FROM outbox_events`, 15290},
		{`SELECT count(*) FROM places`, 0},
	} {
		var got int
		if err := conn.QueryRow(ctx, check.query).Scan(&got); err != nil || got != check.expected {
			t.Fatalf("%s: got %d, want %d: %v", check.query, got, check.expected, err)
		}
	}
}
