package database

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

//go:embed schema/*.sql
var schema embed.FS

//go:embed legacy.sql
var legacySQL string

// NewMigrator owns a dedicated SQL pool, separate from runtime pgxpool.
// Adoption is explicit and never skips structural drift checks.
func NewMigrator(ctx context.Context, url string, adoptLegacy bool) (*goose.Provider, error) {
	c, err := pgx.ParseConfig(url)
	if err != nil {
		return nil, errors.New("invalid migration connection configuration")
	}
	c.RuntimeParams["search_path"] = "public"
	c.ConnectTimeout = 5 * time.Second
	c.RuntimeParams["lock_timeout"] = "5000"
	c.RuntimeParams["statement_timeout"] = "120000"
	c.RuntimeParams["application_name"] = "address-intelligence-migrate"
	db := stdlib.OpenDB(*c)
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, errors.New("migration database connection failed")
	}
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		db.Close()
		return nil, err
	}
	sqlFiles, err := fs.Sub(schema, "schema")
	if err != nil {
		db.Close()
		return nil, err
	}
	baseline := goose.NewGoMigration(1, &goose.GoFunc{RunTx: func(ctx context.Context, tx *sql.Tx) error {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT to_regclass('public.schema_migrations') IS NOT NULL`).Scan(&exists); err != nil {
			return err
		}
		if exists {
			if !adoptLegacy {
				return errLegacyDetected
			}
			return verifyLegacy(ctx, tx)
		}
		if adoptLegacy {
			return errLegacyRequired
		}
		_, err := tx.ExecContext(ctx, baselineSQL())
		return err
	}}, nil)
	provider, err := goose.NewProvider(goose.DialectPostgres, db, sqlFiles, goose.WithSessionLocker(locker), goose.WithGoMigrations(baseline), goose.WithDisableGlobalRegistry(true))
	if err != nil {
		db.Close()
		return nil, err
	}
	return provider, nil
}

func baselineSQL() string {
	text := strings.TrimSpace(legacySQL)
	text = strings.TrimPrefix(text, "BEGIN;")
	return strings.TrimSuffix(strings.TrimSpace(text), "COMMIT;")
}

// Compare structure against a rollback-only baseline schema; never rewrite data.
func verifyLegacy(ctx context.Context, tx *sql.Tx) error {
	var valid bool
	if err := tx.QueryRowContext(ctx, `SELECT count(*)=1 AND min(version)=1 FROM public.schema_migrations`).Scan(&valid); err != nil || !valid {
		return errLegacyLedger
	}
	actual, err := snapshot(ctx, tx, "public")
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `SAVEPOINT baseline_probe; CREATE SCHEMA aip_baseline_probe; SET LOCAL search_path = aip_baseline_probe, public`); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, baselineSQL()); err != nil {
		return fmt.Errorf("create baseline comparison: %w", err)
	}
	expected, err := snapshot(ctx, tx, "aip_baseline_probe")
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT baseline_probe; RELEASE SAVEPOINT baseline_probe; SET LOCAL search_path=public`); err != nil {
		return err
	}
	if actual != expected {
		return errLegacyDrift
	}
	return nil
}

func snapshot(ctx context.Context, tx *sql.Tx, namespace string) (string, error) {
	if _, err := tx.ExecContext(ctx, `SET LOCAL search_path = pg_catalog`); err != nil {
		return "", err
	}
	rows, err := tx.QueryContext(ctx, `
WITH owned AS (
 SELECT c.oid,c.relname,c.relkind,c.relrowsecurity,c.relforcerowsecurity FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname=$1 AND c.relkind IN ('r','p','v','m') AND c.relname <> 'goose_db_version'
 AND NOT EXISTS(SELECT 1 FROM pg_depend d WHERE d.classid='pg_class'::regclass AND d.objid=c.oid AND d.deptype='e')
), definitions AS (
 SELECT 'table:'||relname||':'||relkind::text||':'||relrowsecurity||':'||relforcerowsecurity AS definition FROM owned
 UNION ALL SELECT 'column:'||c.relname||':'||a.attname||':'||a.attnum||':'||format_type(a.atttypid,a.atttypmod)||':'||a.attnotnull||':'||a.attidentity::text||':'||coalesce(pg_get_expr(d.adbin,d.adrelid),'') FROM owned c JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped LEFT JOIN pg_attrdef d ON d.adrelid=c.oid AND d.adnum=a.attnum
 UNION ALL SELECT 'constraint:'||c.relname||':'||k.conname||':'||pg_get_constraintdef(k.oid)||':'||k.convalidated FROM owned c JOIN pg_constraint k ON k.conrelid=c.oid
 UNION ALL SELECT 'index:'||c.relname||':'||pg_get_indexdef(i.indexrelid)||':'||i.indisvalid FROM owned c JOIN pg_index i ON i.indrelid=c.oid
 UNION ALL SELECT 'trigger:'||c.relname||':'||pg_get_triggerdef(t.oid)||':'||t.tgenabled::text FROM owned c JOIN pg_trigger t ON t.tgrelid=c.oid AND NOT t.tgisinternal
 UNION ALL SELECT 'function:'||pg_get_functiondef(p.oid) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname=$1 AND p.prokind='f' AND NOT EXISTS(SELECT 1 FROM pg_depend d WHERE d.classid='pg_proc'::regclass AND d.objid=p.oid AND d.deptype='e')
) SELECT definition FROM definitions ORDER BY definition`, namespace)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return "", err
		}
		line = strings.ReplaceAll(line, namespace+".", "app_schema.")
		line = strings.ReplaceAll(line, "public.", "app_schema.")
		lines = append(lines, line)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n"), rows.Err()
}
