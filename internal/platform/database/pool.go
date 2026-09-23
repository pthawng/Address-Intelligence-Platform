package database

import (
	"context"
	"errors"
	"strconv"
	"time"

	"address-intelligence-platform/internal/platform/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const SchemaVersion int64 = 3

type Pool struct{ *pgxpool.Pool }

// Open verifies connectivity and schema before publishing the pool. Errors never
// include a connection string or a driver's potentially sensitive diagnostics.
func Open(ctx context.Context, url, service string, limits config.DatabasePool) (*Pool, error) {
	c, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, errors.New("invalid database connection configuration")
	}
	if limits.MaxConns < 1 || limits.MinConns < 0 || limits.MinConns > limits.MaxConns || limits.ConnectTimeout <= 0 || limits.ProbeTimeout <= 0 || limits.MaxConnLifetime <= 0 || limits.MaxConnIdleTime <= 0 || limits.StatementTimeout < time.Millisecond {
		return nil, errors.New("invalid database pool limits")
	}
	c.MaxConns, c.MinConns = limits.MaxConns, limits.MinConns
	c.MaxConnLifetime, c.MaxConnIdleTime = limits.MaxConnLifetime, limits.MaxConnIdleTime
	c.MaxConnLifetimeJitter = limits.MaxConnLifetime / 10
	c.ConnConfig.ConnectTimeout = limits.ConnectTimeout
	c.ConnConfig.RuntimeParams["application_name"] = "address-intelligence-" + service
	c.ConnConfig.RuntimeParams["search_path"] = "public"
	c.ConnConfig.RuntimeParams["timezone"] = "UTC"
	c.ConnConfig.RuntimeParams["statement_timeout"] = strconv.FormatInt(limits.StatementTimeout.Milliseconds(), 10)
	c.ConnConfig.RuntimeParams["lock_timeout"] = "3000"
	c.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] = "30000"
	pool, err := pgxpool.NewWithConfig(ctx, c)
	if err != nil {
		return nil, errors.New("database pool initialization failed")
	}
	probeCtx, cancel := context.WithTimeout(ctx, limits.ConnectTimeout)
	defer cancel()
	wrapped := &Pool{pool}
	if err := Ready(probeCtx, wrapped); err != nil {
		pool.Close()
		return nil, err
	}
	return wrapped, nil
}

// Ready checks reachability, migrated schema and runtime table privileges.
func Ready(ctx context.Context, pool *Pool) error {
	if pool == nil {
		return errors.New("database is not configured")
	}
	var version int64
	if err := pool.QueryRow(ctx, `SELECT version_id FROM public.goose_db_version WHERE is_applied ORDER BY id DESC LIMIT 1`).Scan(&version); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("database schema is unavailable; run migrations first")
	}
	if version != SchemaVersion {
		return errors.New("database schema version does not match this binary")
	}
	var allowed bool
	if err := pool.QueryRow(ctx, `SELECT bool_and(has_table_privilege(current_user,t,p)) FROM unnest(ARRAY['public.places','public.outbox_events']) t CROSS JOIN unnest(ARRAY['SELECT','INSERT','UPDATE','DELETE']) p`).Scan(&allowed); err != nil || !allowed {
		return errors.New("database runtime privileges are missing")
	}
	return nil
}

// InTx commits on success, rolls back on error or panic. The caller owns retry
// policy and must keep non-transactional side effects out of retryable callbacks.
func InTx(ctx context.Context, pool *Pool, options pgx.TxOptions, operation func(pgx.Tx) error) error {
	return pgx.BeginTxFunc(ctx, pool, options, operation)
}
