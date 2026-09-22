package database

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pressly/goose/v3"
)

func TestSafeMigrationError(t *testing.T) {
	raw := &goose.PartialError{
		Failed: &goose.MigrationResult{Source: &goose.Source{Version: 2}},
		Err:    &pgconn.PgError{Code: "42501", Message: "secret SQL and credentials", Detail: "private row"},
	}
	got := SafeMigrationError(raw).Error()
	if !strings.Contains(got, "version 2") || !strings.Contains(got, "42501") || strings.Contains(got, "secret") || strings.Contains(got, "private") {
		t.Fatal(got)
	}
	if !errors.Is(SafeMigrationError(fmt.Errorf("unsafe wrapper: %w", errLegacyDrift)), errLegacyDrift) {
		t.Fatal("lost safe adoption diagnosis")
	}
}
