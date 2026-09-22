package config

import (
	"fmt"
	"strconv"
	"time"
)

type DatabasePool struct {
	MaxConns         int32
	MinConns         int32
	ConnectTimeout   time.Duration
	ProbeTimeout     time.Duration
	MaxConnLifetime  time.Duration
	MaxConnIdleTime  time.Duration
	StatementTimeout time.Duration
}

func loadDatabasePool() (DatabasePool, error) {
	var c DatabasePool
	for _, s := range []struct {
		key, fallback string
		target        *int32
	}{
		{"DATABASE_MAX_CONNS", "10", &c.MaxConns}, {"DATABASE_MIN_CONNS", "0", &c.MinConns},
	} {
		n, err := strconv.ParseInt(value(s.key, s.fallback), 10, 32)
		if err != nil || n < 0 {
			return c, fmt.Errorf("%s must be a non-negative integer", s.key)
		}
		*s.target = int32(n)
	}
	if c.MaxConns < 1 || c.MinConns > c.MaxConns {
		return c, fmt.Errorf("database pool requires 0 <= min <= max and max > 0")
	}
	for _, s := range []struct {
		key      string
		fallback time.Duration
		target   *time.Duration
	}{
		{"DATABASE_CONNECT_TIMEOUT", 5 * time.Second, &c.ConnectTimeout},
		{"DATABASE_PROBE_TIMEOUT", time.Second, &c.ProbeTimeout},
		{"DATABASE_MAX_CONN_LIFETIME", 30 * time.Minute, &c.MaxConnLifetime},
		{"DATABASE_MAX_CONN_IDLE_TIME", 5 * time.Minute, &c.MaxConnIdleTime},
		{"DATABASE_STATEMENT_TIMEOUT", 5 * time.Second, &c.StatementTimeout},
	} {
		parsed, err := duration(s.key, s.fallback)
		if err != nil {
			return c, err
		}
		*s.target = parsed
	}
	if c.StatementTimeout < time.Millisecond {
		return c, fmt.Errorf("DATABASE_STATEMENT_TIMEOUT must be at least 1ms")
	}
	return c, nil
}
