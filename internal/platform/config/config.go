// Package config loads and validates process configuration.
package config

import (
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment           string
	HTTPAddress           string
	DatabaseURL           string
	ElasticsearchURL      string
	OutboxPollInterval    time.Duration
	ShutdownTimeout       time.Duration
	LogLevel              slog.Level
	HTTPReadHeaderTimeout time.Duration
	HTTPReadTimeout       time.Duration
	HTTPWriteTimeout      time.Duration
	HTTPIdleTimeout       time.Duration
	OTLPEndpoint          string
}

func Load() (Config, error) {
	pollInterval, err := duration("OUTBOX_POLL_INTERVAL", time.Second)
	if err != nil {
		return Config{}, err
	}

	shutdownTimeout, err := duration("SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Environment:        value("APP_ENV", "development"),
		HTTPAddress:        value("HTTP_ADDRESS", ":8080"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		ElasticsearchURL:   value("ELASTICSEARCH_URL", "http://localhost:9200"),
		OutboxPollInterval: pollInterval,
		ShutdownTimeout:    shutdownTimeout,
		OTLPEndpoint:       os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
	}
	// Keep the existing runtime names authoritative when both names are set.
	if os.Getenv("HTTP_ADDRESS") == "" && os.Getenv("HTTP_PORT") != "" {
		cfg.HTTPAddress = ":" + os.Getenv("HTTP_PORT")
	}
	if os.Getenv("ELASTICSEARCH_URL") == "" {
		cfg.ElasticsearchURL = value("SEARCH_URL", "http://localhost:9200")
	}
	if cfg.Environment == "development" {
		cfg.LogLevel = slog.LevelDebug
	}
	if raw := os.Getenv("LOG_LEVEL"); raw != "" {
		switch strings.ToLower(raw) {
		case "debug":
			cfg.LogLevel = slog.LevelDebug
		case "info":
			cfg.LogLevel = slog.LevelInfo
		case "warn":
			cfg.LogLevel = slog.LevelWarn
		case "error":
			cfg.LogLevel = slog.LevelError
		default:
			return Config{}, fmt.Errorf("LOG_LEVEL must be debug, info, warn or error")
		}
	}
	for _, setting := range []struct {
		key      string
		fallback time.Duration
		target   *time.Duration
	}{
		{"HTTP_READ_HEADER_TIMEOUT", 5 * time.Second, &cfg.HTTPReadHeaderTimeout},
		{"HTTP_READ_TIMEOUT", 10 * time.Second, &cfg.HTTPReadTimeout},
		{"HTTP_WRITE_TIMEOUT", 15 * time.Second, &cfg.HTTPWriteTimeout},
		{"HTTP_IDLE_TIMEOUT", 60 * time.Second, &cfg.HTTPIdleTimeout},
	} {
		parsed, err := duration(setting.key, setting.fallback)
		if err != nil {
			return Config{}, err
		}
		*setting.target = parsed
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate checks syntax only; dependency connectivity belongs to readiness checks.
// Errors deliberately exclude raw values, which may contain credentials.
func (c Config) Validate() error {
	switch c.Environment {
	case "development", "test", "staging", "production":
	default:
		return fmt.Errorf("APP_ENV must be development, test, staging or production")
	}
	_, port, err := net.SplitHostPort(c.HTTPAddress)
	if err != nil || !validPort(port) {
		return fmt.Errorf("HTTP_ADDRESS (or HTTP_PORT) must specify host:port with port between 1 and 65535")
	}
	if c.DatabaseURL == "" {
		if c.Environment == "staging" || c.Environment == "production" {
			return fmt.Errorf("DATABASE_URL is required in staging and production")
		}
	} else if !validURL(c.DatabaseURL, "postgres", "postgresql") {
		return fmt.Errorf("DATABASE_URL must be a postgres or postgresql URL with a host and valid port")
	}
	if !validURL(c.ElasticsearchURL, "http", "https") {
		return fmt.Errorf("ELASTICSEARCH_URL (or SEARCH_URL) must be an http or https URL with a host and valid port")
	}
	if c.OTLPEndpoint != "" && !validURL(c.OTLPEndpoint, "http", "https") {
		return fmt.Errorf("OTEL_EXPORTER_OTLP_ENDPOINT must be an http or https URL with a host and valid port")
	}
	for _, setting := range []struct {
		key   string
		value time.Duration
	}{
		{"OUTBOX_POLL_INTERVAL", c.OutboxPollInterval},
		{"SHUTDOWN_TIMEOUT", c.ShutdownTimeout},
		{"HTTP_READ_HEADER_TIMEOUT", c.HTTPReadHeaderTimeout},
		{"HTTP_READ_TIMEOUT", c.HTTPReadTimeout},
		{"HTTP_WRITE_TIMEOUT", c.HTTPWriteTimeout},
		{"HTTP_IDLE_TIMEOUT", c.HTTPIdleTimeout},
	} {
		if setting.value <= 0 {
			return fmt.Errorf("%s must be positive", setting.key)
		}
	}
	return nil
}

func validPort(raw string) bool {
	if raw == "" || strings.Trim(raw, "0123456789") != "" {
		return false
	}
	port, err := strconv.Atoi(raw)
	return err == nil && port > 0 && port <= 65535
}

func validURL(raw string, schemes ...string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.Fragment != "" {
		return false
	}
	if strings.HasSuffix(u.Host, ":") || (u.Port() != "" && !validPort(u.Port())) {
		return false
	}
	for _, scheme := range schemes {
		if u.Scheme == scheme {
			return true
		}
	}
	return false
}

func value(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func duration(key string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration such as 1s or 500ms", key)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be positive", key)
	}
	return parsed, nil
}
