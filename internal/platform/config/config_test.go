package config

import (
	"log/slog"
	"net/url"
	"strings"
	"testing"
	"time"
)

func cleanEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"APP_ENV", "HTTP_ADDRESS", "HTTP_PORT", "DATABASE_URL", "ELASTICSEARCH_URL", "SEARCH_URL",
		"OUTBOX_POLL_INTERVAL", "SHUTDOWN_TIMEOUT", "LOG_LEVEL", "OTEL_EXPORTER_OTLP_ENDPOINT",
		"HTTP_READ_HEADER_TIMEOUT", "HTTP_READ_TIMEOUT", "HTTP_WRITE_TIMEOUT", "HTTP_IDLE_TIMEOUT", "HTTP_MAX_HEADER_BYTES", "HTTP_REQUEST_TIMEOUT", "HTTP_CORS_ORIGINS", "HTTP_RATE_PER_SECOND", "HTTP_RATE_BURST",
		"DATABASE_HOST", "DATABASE_PORT", "DATABASE_NAME", "DATABASE_USER", "DATABASE_PASSWORD", "DATABASE_SSLMODE",
	} {
		t.Setenv(key, "")
	}
}

func TestSplitDatabaseCredentials(t *testing.T) {
	for _, host := range []string{"localhost", "::1"} {
		t.Run(host, func(t *testing.T) {
			cleanEnvironment(t)
			password := "p@ss:/?#%$'\\ with spaces"
			user := "user@domain:name"
			name := "database ?#%"
			t.Setenv("DATABASE_HOST", host)
			t.Setenv("DATABASE_USER", user)
			t.Setenv("DATABASE_PASSWORD", password)
			t.Setenv("DATABASE_NAME", name)
			cfg, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			u, err := url.Parse(cfg.DatabaseURL)
			if err != nil {
				t.Fatal("generated URL cannot be parsed")
			}
			gotPassword, _ := u.User.Password()
			if gotPassword != password || u.User.Username() != user || u.Path != "/"+name || u.Hostname() != host || u.Port() != "5432" || u.Query().Get("sslmode") != "verify-full" {
				t.Fatal("database settings did not round-trip correctly")
			}
		})
	}
}

func TestSplitDatabaseValidation(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"DATABASE_HOST", ""}, {"DATABASE_HOST", "localhost:5432"}, {"DATABASE_HOST", "db/path"},
		{"DATABASE_NAME", ""}, {"DATABASE_USER", " "}, {"DATABASE_PORT", "0"},
		{"DATABASE_PORT", "65536"}, {"DATABASE_SSLMODE", "private-token"},
	} {
		t.Run(tc.key+"/"+tc.value, func(t *testing.T) {
			cleanEnvironment(t)
			t.Setenv("DATABASE_HOST", "localhost")
			t.Setenv("DATABASE_NAME", "app")
			t.Setenv("DATABASE_USER", "user")
			t.Setenv("DATABASE_PASSWORD", "private-token")
			t.Setenv(tc.key, tc.value)
			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("expected error naming %s", tc.key)
			}
			if strings.Contains(err.Error(), "private-token") {
				t.Fatal("error leaks configuration value")
			}
		})
	}
}

func TestDatabaseURLPrecedence(t *testing.T) {
	cleanEnvironment(t)
	const raw = "postgres://user:password@localhost/app?sslmode=disable"
	t.Setenv("DATABASE_URL", raw)
	t.Setenv("DATABASE_HOST", "invalid/host")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabaseURL != raw {
		t.Fatal("explicit URL must take precedence")
	}
}

func TestRuntimeScope(t *testing.T) {
	for _, runtime := range []Runtime{API, Indexer, Worker} {
		t.Run(string(runtime), func(t *testing.T) {
			cleanEnvironment(t)
			if runtime != API {
				t.Setenv("HTTP_ADDRESS", "invalid")
				t.Setenv("HTTP_READ_TIMEOUT", "invalid")
			}
			if runtime != Indexer {
				t.Setenv("OUTBOX_POLL_INTERVAL", "invalid")
			}
			if runtime == Worker {
				t.Setenv("ELASTICSEARCH_URL", "invalid")
			}
			if _, err := LoadFor(runtime); err != nil {
				t.Fatalf("unused settings blocked runtime: %v", err)
			}
			key := "HTTP_READ_TIMEOUT"
			if runtime == Indexer {
				key = "OUTBOX_POLL_INTERVAL"
			}
			if runtime == Worker {
				key = "SHUTDOWN_TIMEOUT"
			}
			t.Setenv(key, "invalid")
			if _, err := LoadFor(runtime); err == nil {
				t.Fatal("used setting must be validated")
			}
		})
	}
	if _, err := LoadFor("unknown"); err == nil {
		t.Fatal("unknown runtime accepted")
	}
}

func TestDeployedSearchRequirement(t *testing.T) {
	for _, environment := range []string{"staging", "production"} {
		for _, runtime := range []Runtime{API, Indexer, Worker} {
			t.Run(environment+"/"+string(runtime), func(t *testing.T) {
				cleanEnvironment(t)
				t.Setenv("APP_ENV", environment)
				t.Setenv("DATABASE_URL", "postgres://localhost/app")
				cfg, err := LoadFor(runtime)
				if runtime == Worker {
					if err != nil {
						t.Fatal(err)
					}
				} else {
					if err == nil || !strings.Contains(err.Error(), "ELASTICSEARCH_URL") {
						t.Fatalf("missing search URL must fail: %v", err)
					}
					t.Setenv("SEARCH_URL", "https://search.example")
					cfg, err = LoadFor(runtime)
					if err != nil {
						t.Fatal(err)
					}
				}
				if cfg.LogLevel != slog.LevelInfo {
					t.Fatal("deployed environments must default to info")
				}
			})
		}
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
		"HTTP_READ_HEADER_TIMEOUT": "2s", "HTTP_READ_TIMEOUT": "3s", "HTTP_WRITE_TIMEOUT": "4s", "HTTP_IDLE_TIMEOUT": "5s", "HTTP_REQUEST_TIMEOUT": "2s",
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

func TestHTTPHeaderLimit(t *testing.T) {
	cleanEnvironment(t)
	cfg, err := LoadFor(API)
	if err != nil || cfg.HTTPMaxHeaderBytes != 32768 {
		t.Fatalf("default header limit: %v %v", cfg.HTTPMaxHeaderBytes, err)
	}
	t.Setenv("HTTP_MAX_HEADER_BYTES", "65536")
	cfg, err = LoadFor(API)
	if err != nil || cfg.HTTPMaxHeaderBytes != 65536 {
		t.Fatal("override failed", err)
	}
	cfg.HTTPMaxHeaderBytes = 0
	if cfg.Validate() == nil {
		t.Fatal("manual zero accepted")
	}
	for _, value := range []string{"0", "-1", "bad", "999999999999999999999999"} {
		t.Setenv("HTTP_MAX_HEADER_BYTES", value)
		if _, err := LoadFor(API); err == nil {
			t.Fatal("invalid limit accepted")
		}
		for _, runtime := range []Runtime{Indexer, Worker} {
			if _, err := LoadFor(runtime); err != nil {
				t.Fatal("unused HTTP setting blocked runtime", err)
			}
		}
	}
}

func TestMiddlewareSettings(t *testing.T) {
	cleanEnvironment(t)
	cfg, err := LoadFor(API)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CORSOrigins != "http://localhost:3000" || cfg.HTTPRequestTimeout != 8*time.Second || cfg.HTTPRateBurst != 200 {
		t.Fatal("invalid defaults")
	}
	for _, tc := range []struct{ key, value string }{{"HTTP_REQUEST_TIMEOUT", "0s"}, {"HTTP_REQUEST_TIMEOUT", "15s"}, {"HTTP_CORS_ORIGINS", "*"}, {"HTTP_CORS_ORIGINS", "https://host/path"}, {"HTTP_RATE_PER_SECOND", "0"}, {"HTTP_RATE_BURST", "bad"}} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			cleanEnvironment(t)
			t.Setenv(tc.key, tc.value)
			if _, err := LoadFor(API); err == nil {
				t.Fatal("invalid middleware config accepted")
			}
			if _, err := LoadFor(Worker); err != nil {
				t.Fatal("worker parsed HTTP settings", err)
			}
		})
	}
}
