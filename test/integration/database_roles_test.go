package integration

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"address-intelligence-platform/internal/platform/database"
	"github.com/jackc/pgx/v5"
)

func runSQLFile(t *testing.T, ctx context.Context, conn *pgx.Conn, parts ...string) {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(append([]string{"..", ".."}, parts...)...))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, string(contents)); err != nil {
		t.Fatal(err)
	}
}

func roleURL(t *testing.T, raw, role string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.UserPassword(role, "role-test-only")
	return u.String()
}

// Fixed role names require a disposable, dedicated test server (no parallel runs).
func TestProvisionedMigrationRoles(t *testing.T) {
	ctx, _, admin := isolatedDB(t)
	if _, err := admin.Exec(ctx, `CREATE ROLE address_migrator LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD 'role-test-only'; CREATE ROLE address_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD 'role-test-only'`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := admin.Exec(cleanup, `DROP ROLE address_runtime; DROP ROLE address_migrator`); err != nil {
			t.Error(err)
		}
	})
	for _, legacy := range []bool{false, true} {
		name := "fresh"
		if legacy {
			name = "legacy"
		}
		t.Run(name, func(t *testing.T) {
			ctx, raw, conn := isolatedDB(t)
			runSQLFile(t, ctx, conn, "docker", "postgres", "init", "001_extensions.sql")
			runSQLFile(t, ctx, conn, "scripts", "database_role_grants.sql")
			if legacy {
				runSQLFile(t, ctx, conn, "migrations", "000001_create_core_schema.sql")
				if _, err := conn.Exec(ctx, `INSERT INTO data_sources(source_code,source_name,source_type) VALUES('preserved','existing','INTERNAL')`); err != nil {
					t.Fatal(err)
				}
				p, err := database.NewMigrator(ctx, roleURL(t, raw, "address_migrator"), true)
				if err != nil {
					t.Fatal(err)
				}
				_, err = p.Up(ctx)
				p.Close()
				if err == nil {
					t.Fatal("adoption without ownership unexpectedly succeeded")
				}
				runSQLFile(t, ctx, conn, "scripts", "database_legacy_owner.sql")
				runSQLFile(t, ctx, conn, "scripts", "database_legacy_owner.sql")
			}
			p, err := database.NewMigrator(ctx, roleURL(t, raw, "address_migrator"), legacy)
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			if _, err = p.Up(ctx); err != nil {
				t.Fatal(err)
			}
			if result, err := p.Up(ctx); err != nil || len(result) != 0 {
				t.Fatalf("rerun: %v", err)
			}
			if legacy {
				var count int
				if err = conn.QueryRow(ctx, `SELECT count(*) FROM data_sources WHERE source_code='preserved'`).Scan(&count); err != nil || count != 1 {
					t.Fatal("legacy data lost", err)
				}
			}
			// Simulate missing grants after migration; reconciliation must repair them.
			if _, err = conn.Exec(ctx, `REVOKE ALL ON ALL TABLES IN SCHEMA public FROM address_runtime; REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM address_runtime`); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				runSQLFile(t, ctx, conn, "scripts", "database_access.sql")
			}
			pool, err := database.Open(ctx, roleURL(t, raw, "address_runtime"), name, poolLimits())
			if err != nil {
				t.Fatal(err)
			}
			defer pool.Close()
			if _, err = pool.Exec(ctx, `INSERT INTO data_sources(source_code,source_name,source_type) VALUES('runtime','runtime','INTERNAL')`); err != nil {
				t.Fatal(err)
			}
			for _, query := range []string{`CREATE TABLE public.forbidden(id int)`, `DELETE FROM goose_db_version`, `DELETE FROM schema_migrations`, `ALTER TABLE places ADD COLUMN forbidden int`} {
				if _, err = pool.Exec(ctx, query); err == nil {
					t.Fatalf("runtime allowed: %s", query)
				}
			}
		})
	}
	t.Run("runtime_role_created_after_migration", func(t *testing.T) {
		ctx, raw, conn := isolatedDB(t)
		if _, err := conn.Exec(ctx, `DROP ROLE address_runtime`); err != nil {
			t.Fatal(err)
		}
		// Restore the cluster role even if migration fails, so parent cleanup works.
		defer func() {
			cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if _, err := conn.Exec(cleanup, `DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='address_runtime') THEN CREATE ROLE address_runtime LOGIN PASSWORD 'role-test-only'; END IF; END $$`); err != nil {
				t.Error(err)
			}
		}()
		p, err := database.NewMigrator(ctx, raw, false)
		if err != nil {
			t.Fatal(err)
		}
		defer p.Close()
		if _, err = p.Up(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err = conn.Exec(ctx, `CREATE ROLE address_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD 'role-test-only'`); err != nil {
			t.Fatal(err)
		}
		if _, err = p.Up(ctx); err != nil {
			t.Fatal(err)
		}
		if pool, err := database.Open(ctx, roleURL(t, raw, "address_runtime"), "late-role", poolLimits()); err == nil {
			pool.Close()
			t.Fatal("runtime should require privilege reconciliation")
		}
		runSQLFile(t, ctx, conn, "scripts", "database_role_grants.sql")
		runSQLFile(t, ctx, conn, "scripts", "database_access.sql")
		pool, err := database.Open(ctx, roleURL(t, raw, "address_runtime"), "late-role", poolLimits())
		if err != nil {
			t.Fatal(err)
		}
		pool.Close()
	})
}
