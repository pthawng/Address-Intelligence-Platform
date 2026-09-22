package config

import "testing"

func TestDatabasePoolLimits(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"DATABASE_MAX_CONNS", "0"}, {"DATABASE_MIN_CONNS", "-1"}, {"DATABASE_MAX_CONNS", "2147483648"},
		{"DATABASE_CONNECT_TIMEOUT", "0s"}, {"DATABASE_PROBE_TIMEOUT", "bad"}, {"DATABASE_STATEMENT_TIMEOUT", "1ns"},
	} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)
			if _, err := loadDatabasePool(); err == nil {
				t.Fatal("invalid limit accepted")
			}
		})
	}
	t.Setenv("DATABASE_MAX_CONNS", "2")
	t.Setenv("DATABASE_MIN_CONNS", "3")
	if _, err := loadDatabasePool(); err == nil {
		t.Fatal("min greater than max accepted")
	}
}
