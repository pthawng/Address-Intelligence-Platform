package config

import (
	"net/url"
	"testing"
)

func TestMigrationCredentialsAreSeparate(t *testing.T) {
	for _, suffix := range []string{"URL", "HOST", "PORT", "NAME", "USER", "PASSWORD", "SSLMODE"} {
		t.Setenv("MIGRATION_DATABASE_"+suffix, "")
	}
	t.Setenv("DATABASE_URL", "postgres://runtime:secret@localhost/app")
	if got, err := MigrationURL(); err != nil || got != "" {
		t.Fatalf("runtime credentials used: %v", err)
	}
	t.Setenv("MIGRATION_DATABASE_HOST", "postgres")
	t.Setenv("MIGRATION_DATABASE_NAME", "app")
	t.Setenv("MIGRATION_DATABASE_USER", "address_migrator")
	t.Setenv("MIGRATION_DATABASE_PASSWORD", "raw$p@ss/#%")
	got, err := MigrationURL()
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	password, _ := u.User.Password()
	if password != "raw$p@ss/#%" || u.User.Username() != "address_migrator" || u.Query().Get("sslmode") != "verify-full" {
		t.Fatal("incorrect migration URL encoding/defaults")
	}
	t.Setenv("MIGRATION_DATABASE_URL", "postgres://explicit/db")
	if got, err = MigrationURL(); err != nil || got != "postgres://explicit/db" {
		t.Fatal("explicit URL did not take precedence")
	}
}
