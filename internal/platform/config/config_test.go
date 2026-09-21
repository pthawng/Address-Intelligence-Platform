package config

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func cleanEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"APP_ENV", "HTTP_ADDRESS", "HTTP_PORT", "DATABASE_URL", "ELASTICSEARCH_URL", "SEARCH_URL",
		"OUTBOX_POLL_INTERVAL", "SHUTDOWN_TIMEOUT", "LOG_LEVEL", "OTEL_EXPORTER_OTLP_ENDPOINT",
		"HTTP_READ_HEADER_TIMEOUT", "HTTP_READ_TIMEOUT", "HTTP_WRITE_TIMEOUT", "HTTP_IDLE_TIMEOUT",
	} {
		t.Setenv(key, "")
	}
}

func TestLoadDefaults(t *testing.T) {
	cleanEnvironment(t)
	t.Setenv("APP_ENV", "")
	t.Setenv("HTTP_ADDRESS", "")
	t.Setenv("OUTBOX_POLL_INTERVAL", "")
	t.Setenv("SHUTDOWN_TIMEOUT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Environment != "development" {
		t.Fatalf("Environment = %q, want development", cfg.Environment)
	}
	if cfg.OutboxPollInterval != time.Second {
		t.Fatalf("OutboxPollInterval = %v, want %v", cfg.OutboxPollInterval, time.Second)
	}
	if cfg.HTTPAddress != ":8080" || cfg.ElasticsearchURL != "http://localhost:9200" || cfg.DatabaseURL != "" || cfg.LogLevel != slog.LevelDebug {
		t.Fatal("unexpected default configuration")
	}
	if cfg.HTTPReadHeaderTimeout != 5*time.Second || cfg.HTTPReadTimeout != 10*time.Second || cfg.HTTPWriteTimeout != 15*time.Second || cfg.HTTPIdleTimeout != time.Minute || cfg.ShutdownTimeout != 10*time.Second {
		t.Fatal("unexpected default timeouts")
	}
}

func TestLoadRejectsInvalidDuration(t *testing.T) {
	cleanEnvironment(t)
	t.Setenv("OUTBOX_POLL_INTERVAL", "not-a-duration")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want an invalid duration error")
	}
}

func TestLoadOverrides(t *testing.T) {
	cleanEnvironment(t)
	for key, value := range map[string]string{
		"APP_ENV": "production", "HTTP_ADDRESS": "[::1]:9090", "LOG_LEVEL": "WARN",
		"DATABASE_URL":      "postgresql://user:password@localhost:5432/app",
		"ELASTICSEARCH_URL": "https://search.example:9243", "OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4318",
		"OUTBOX_POLL_INTERVAL": "250ms", "SHUTDOWN_TIMEOUT": "20s",
		"HTTP_READ_HEADER_TIMEOUT": "2s", "HTTP_READ_TIMEOUT": "3s", "HTTP_WRITE_TIMEOUT": "4s", "HTTP_IDLE_TIMEOUT": "5s",
	} {
		t.Setenv(key, value)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Environment != "production" || cfg.HTTPAddress != "[::1]:9090" || cfg.LogLevel != slog.LevelWarn || cfg.DatabaseURL != "postgresql://user:password@localhost:5432/app" || cfg.ElasticsearchURL != "https://search.example:9243" || cfg.OTLPEndpoint != "http://localhost:4318" {
		t.Fatal("environment overrides were not applied")
	}
	if cfg.OutboxPollInterval != 250*time.Millisecond || cfg.ShutdownTimeout != 20*time.Second || cfg.HTTPReadHeaderTimeout != 2*time.Second || cfg.HTTPReadTimeout != 3*time.Second || cfg.HTTPWriteTimeout != 4*time.Second || cfg.HTTPIdleTimeout != 5*time.Second {
		t.Fatal("timeout overrides were not applied")
	}
}

func TestLoadAliases(t *testing.T) {
	cleanEnvironment(t)
	t.Setenv("HTTP_PORT", "9090")
	t.Setenv("SEARCH_URL", "https://search.example")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddress != ":9090" || cfg.ElasticsearchURL != "https://search.example" {
		t.Fatal("aliases not applied")
	}
	t.Setenv("HTTP_ADDRESS", ":8081")
	t.Setenv("ELASTICSEARCH_URL", "http://localhost:9201")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddress != ":8081" || cfg.ElasticsearchURL != "http://localhost:9201" {
		t.Fatal("canonical names must take precedence")
	}
}

func TestLoadValidation(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"APP_ENV", "prod"}, {"LOG_LEVEL", "verbose"},
		{"HTTP_ADDRESS", "localhost"}, {"HTTP_ADDRESS", ":0"}, {"HTTP_ADDRESS", ":65536"}, {"HTTP_ADDRESS", ":http"},
		{"HTTP_PORT", "-1"}, {"DATABASE_URL", "mysql://localhost/app"},
		{"DATABASE_URL", "postgres://user:private-token@localhost:bad/app"},
		{"ELASTICSEARCH_URL", "localhost:9200"}, {"ELASTICSEARCH_URL", "http://localhost:65536"},
		{"SEARCH_URL", "http://"}, {"OTEL_EXPORTER_OTLP_ENDPOINT", "grpc://localhost:4317"},
		{"OUTBOX_POLL_INTERVAL", "0s"}, {"SHUTDOWN_TIMEOUT", "-1s"},
		{"HTTP_READ_HEADER_TIMEOUT", "bad"}, {"HTTP_READ_TIMEOUT", "0"},
		{"HTTP_WRITE_TIMEOUT", "-1s"}, {"HTTP_IDLE_TIMEOUT", "private-token"},
	} {
		t.Run(tc.key+"/"+tc.value, func(t *testing.T) {
			cleanEnvironment(t)
			t.Setenv(tc.key, tc.value)
			cfg, err := Load()
			if err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("expected error naming %s, got %v", tc.key, err)
			}
			if strings.Contains(err.Error(), "private-token") {
				t.Fatal("error leaks raw configuration")
			}
			if cfg != (Config{}) {
				t.Fatal("invalid configuration must not be returned")
			}
		})
	}
}

func TestDatabaseRequirement(t *testing.T) {
	for _, environment := range []string{"development", "test", "staging", "production"} {
		t.Run(environment, func(t *testing.T) {
			cleanEnvironment(t)
			t.Setenv("APP_ENV", environment)
			cfg, err := Load()
			required := environment == "staging" || environment == "production"
			if required {
				if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
					t.Fatalf("expected DATABASE_URL requirement, got %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if environment == "test" && cfg.LogLevel != slog.LevelInfo {
				t.Fatal("test must default to info logging")
			}
		})
	}
}
