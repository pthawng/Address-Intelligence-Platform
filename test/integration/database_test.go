package integration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"address-intelligence-platform/internal/platform/config"
	"address-intelligence-platform/internal/platform/database"
	"github.com/jackc/pgx/v5"
)

// Every test owns a random database on an explicitly configured test server.
func isolatedDB(t *testing.T) (context.Context, string, *pgx.Conn) {
	t.Helper()
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("set TEST_DATABASE_URL to an isolated PostGIS server")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	admin, err := pgx.Connect(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	var random [8]byte
	_, _ = rand.Read(random[:])
	name := "aip_test_" + hex.EncodeToString(random[:])
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		admin.Close(ctx)
		t.Fatal(err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	conn, err := pgx.Connect(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		conn.Close(cleanup)
		if _, err := admin.Exec(cleanup, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
			t.Error(err)
		}
		admin.Close(cleanup)
	})
	return ctx, u.String(), conn
}

func poolLimits() config.DatabasePool {
	return config.DatabasePool{MaxConns: 2, ConnectTimeout: 3 * time.Second, ProbeTimeout: time.Second, MaxConnLifetime: time.Minute, MaxConnIdleTime: time.Minute, StatementTimeout: 2 * time.Second}
}

func TestDatabaseFreshMigrationAndRuntime(t *testing.T) {
	ctx, raw, conn := isolatedDB(t)
	p, err := database.NewMigrator(ctx, raw, false)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, err = p.Up(ctx); err != nil {
		t.Fatal(err)
	}
	if results, err := p.Up(ctx); err != nil || len(results) != 0 {
		t.Fatalf("rerun: %v %v", results, err)
	}
	pool, err := database.Open(ctx, raw, "test", poolLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	queryCtx, cancel := context.WithTimeout(ctx, 40*time.Millisecond)
	err = pool.QueryRow(queryCtx, "SELECT pg_sleep(10)").Scan(new(any))
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("query not cancelled: %v", err)
	}
	if err = database.Ready(ctx, pool); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("rollback requested")
	err = database.InTx(ctx, pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO data_sources(source_code,source_name,source_type) VALUES('rollback','test','INTERNAL')`); err != nil {
			return err
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	var count int
	if err = conn.QueryRow(ctx, `SELECT count(*) FROM data_sources WHERE source_code='rollback'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("transaction not rolled back", err, count)
	}
}

func TestLegacyAdoptionAndDrift(t *testing.T) {
	for _, drift := range []bool{false, true} {
		t.Run(map[bool]string{false: "matching", true: "drift"}[drift], func(t *testing.T) {
			ctx, raw, conn := isolatedDB(t)
			baseline, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000001_create_core_schema.sql"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err = conn.Exec(ctx, string(baseline)); err != nil {
				t.Fatal(err)
			}
			if _, err = conn.Exec(ctx, `INSERT INTO data_sources(source_code,source_name,source_type) VALUES('preserve','existing','INTERNAL')`); err != nil {
				t.Fatal(err)
			}
			p, err := database.NewMigrator(ctx, raw, false)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = p.Up(ctx); err == nil {
				t.Fatal("up silently adopted legacy schema")
			}
			p.Close()
			if drift {
				if _, err = conn.Exec(ctx, `ALTER TABLE places DROP CONSTRAINT ck_places_name_not_blank`); err != nil {
					t.Fatal(err)
				}
			}
			p, err = database.NewMigrator(ctx, raw, true)
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			_, err = p.Up(ctx)
			if (err != nil) != drift {
				t.Fatalf("drift=%v: %v", drift, err)
			}
			var name string
			if err = conn.QueryRow(ctx, `SELECT source_name FROM data_sources WHERE source_code='preserve'`).Scan(&name); err != nil || name != "existing" {
				t.Fatal("legacy data changed", err)
			}
		})
	}
}

func TestConcurrentMigrators(t *testing.T) {
	ctx, raw, _ := isolatedDB(t)
	var wg sync.WaitGroup
	failures := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, err := database.NewMigrator(ctx, raw, false)
			if err == nil {
				_, err = p.Up(ctx)
				p.Close()
			}
			failures <- err
		}()
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestFailedMigrationRollsBack(t *testing.T) {
	ctx, raw, conn := isolatedDB(t)
	p, err := database.NewMigrator(ctx, raw, false)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, err = p.UpTo(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, `INSERT INTO places(place_name,normalized_name,place_type) VALUES('a','a','STREET'),('b','b','STREET'); ALTER TABLE places DISABLE TRIGGER trg_places_prevent_cycle; UPDATE places SET parent_place_id=3-id; ALTER TABLE places ENABLE TRIGGER trg_places_prevent_cycle`); err != nil {
		t.Fatal(err)
	}
	if _, err = p.Up(ctx); err == nil {
		t.Fatal("migration accepted corrupt hierarchy")
	}
	version, err := p.GetDBVersion(ctx)
	if err != nil || version != 1 {
		t.Fatal("failed migration changed version", version, err)
	}
	if _, err = conn.Exec(ctx, `UPDATE places SET parent_place_id=NULL`); err != nil {
		t.Fatal(err)
	}
	if _, err = p.Up(ctx); err != nil {
		t.Fatal("migration failed after repair", err)
	}
}

func TestRuntimeRoleCannotMigrate(t *testing.T) {
	ctx, raw, conn := isolatedDB(t)
	if _, err := conn.Exec(ctx, `CREATE ROLE address_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD 'test-runtime-only'`); err != nil {
		t.Fatal("requires isolated test server without address_runtime role", err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := conn.Exec(cleanup, `DROP OWNED BY address_runtime; DROP ROLE address_runtime`); err != nil {
			t.Error(err)
		}
	})
	if _, err := conn.Exec(ctx, `REVOKE CREATE ON SCHEMA public FROM PUBLIC`); err != nil {
		t.Fatal(err)
	}
	p, err := database.NewMigrator(ctx, raw, false)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, err = p.Up(ctx); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.UserPassword("address_runtime", "test-runtime-only")
	pool, err := database.Open(ctx, u.String(), "test", poolLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = pool.Exec(ctx, `INSERT INTO data_sources(source_code,source_name,source_type) VALUES('runtime','runtime','INTERNAL')`); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{`CREATE TABLE public.forbidden(id int)`, `DELETE FROM goose_db_version`, `ALTER TABLE places ADD COLUMN forbidden int`} {
		if _, err = pool.Exec(ctx, query); err == nil {
			t.Fatal("runtime role performed DDL or changed ledger", query)
		}
	}
}

func TestHierarchyConcurrentWrites(t *testing.T) {
	ctx, raw, conn := isolatedDB(t)
	p, err := database.NewMigrator(ctx, raw, false)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, err = p.Up(ctx); err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(ctx, `INSERT INTO places(place_name,normalized_name,place_type) VALUES('a','a','STREET'),('b','b','STREET')`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, `UPDATE places SET parent_place_id=2 WHERE id=1`); err == nil {
		t.Fatal("unsafe isolation accepted")
	}
	other, err := pgx.Connect(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close(ctx)
	a, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Rollback(ctx)
	b, err := other.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Rollback(ctx)
	if _, err = a.Exec(ctx, `UPDATE places SET parent_place_id=2 WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	if _, err = b.Exec(ctx, `UPDATE places SET parent_place_id=1 WHERE id=2`); err != nil {
		t.Fatal(err)
	}
	errA, errB := a.Commit(ctx), b.Commit(ctx)
	if errA == nil && errB == nil {
		t.Fatal("both cycle-producing transactions committed")
	}
	for _, err := range []error{errA, errB} {
		if err != nil {
			var state interface{ SQLState() string }
			if !errors.As(err, &state) || state.SQLState() != "40001" {
				t.Fatalf("unexpected error: %v", err)
			}
		}
	}
}
