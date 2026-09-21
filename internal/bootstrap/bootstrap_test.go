package bootstrap

import (
	"context"
	"log/slog"
	"strings"
	"testing"
)

func runtimeEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"APP_ENV", "LOG_LEVEL", "DATABASE_URL", "DATABASE_HOST", "DATABASE_PORT", "DATABASE_NAME",
		"DATABASE_USER", "DATABASE_PASSWORD", "DATABASE_SSLMODE", "ELASTICSEARCH_URL", "SEARCH_URL",
		"OTEL_EXPORTER_OTLP_ENDPOINT", "SHUTDOWN_TIMEOUT", "OUTBOX_POLL_INTERVAL",
		"HTTP_ADDRESS", "HTTP_PORT", "HTTP_READ_HEADER_TIMEOUT", "HTTP_READ_TIMEOUT", "HTTP_WRITE_TIMEOUT", "HTTP_IDLE_TIMEOUT",
	} {
		t.Setenv(key, "")
	}
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })
}

// Exercise the actual composition roots, including environment loading and logging.
func TestBackgroundRuntimeConfiguration(t *testing.T) {
	for name, newRuntime := range map[string]func(...Option) (*App, error){"worker": NewWorker, "indexer": NewIndexer} {
		t.Run(name, func(t *testing.T) {
			runtimeEnvironment(t)
			t.Setenv("HTTP_ADDRESS", "invalid")
			t.Setenv("HTTP_READ_TIMEOUT", "invalid")
			t.Setenv("LOG_LEVEL", "error")
			if name == "worker" {
				t.Setenv("ELASTICSEARCH_URL", "invalid")
				t.Setenv("OUTBOX_POLL_INTERVAL", "invalid")
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			previous := slog.Default()
			app, err := newRuntime()
			if err != nil {
				t.Fatal(err)
			}
			if err := app.Run(ctx); err != nil {
				t.Fatal(err)
			}
			if app.logger.Enabled(ctx, slog.LevelInfo) || !app.logger.Enabled(ctx, slog.LevelError) {
				t.Fatal("configured log level not applied")
			}
			if slog.Default() != previous {
				t.Fatal("bootstrap mutated global logger")
			}
		})
	}
}

func TestRuntimesFailBeforeStartupWithoutDatabase(t *testing.T) {
	for name, run := range map[string]func(context.Context) error{"api": RunAPI, "indexer": RunIndexer, "worker": RunWorker} {
		t.Run(name, func(t *testing.T) {
			runtimeEnvironment(t)
			t.Setenv("APP_ENV", "production")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cancel()
			err := run(ctx)
			if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
				t.Fatalf("expected startup configuration failure, got %v", err)
			}
		})
	}
}
