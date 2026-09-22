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
	runtime               Runtime
	Environment           string
	HTTPAddress           string
	DatabaseURL           string
	DatabasePool          DatabasePool
	ElasticsearchURL      string
	OutboxPollInterval    time.Duration
	ShutdownTimeout       time.Duration
	LogLevel              slog.Level
	HTTPReadHeaderTimeout time.Duration
	HTTPReadTimeout       time.Duration
	HTTPWriteTimeout      time.Duration
	HTTPIdleTimeout       time.Duration
	HTTPMaxHeaderBytes    int
	OTLPEndpoint          string
	HTTPRequestTimeout    time.Duration
	CORSOrigins           string
	HTTPRatePerSecond     int
	HTTPRateBurst         int
}

type Runtime string

const (
	API     Runtime = "api"
	Indexer Runtime = "indexer"
	Worker  Runtime = "worker"
)

func (r Runtime) usesHTTP() bool   { return r == "" || r == API }
func (r Runtime) usesSearch() bool { return r != Worker }
func (r Runtime) usesOutbox() bool { return r == "" || r == Indexer }

// Load validates all configuration groups. Runtime entry points use LoadFor.
func Load() (Config, error) {
	return LoadFor("")
}

// LoadFor ignores settings that are not used by the selected runtime.
// The empty runtime preserves Load's validation of every group.
func LoadFor(runtime Runtime) (Config, error) {
	switch runtime {
	case "", API, Indexer, Worker:
	default:
		return Config{}, fmt.Errorf("runtime must be api, indexer or worker")
	}

	shutdownTimeout, err := duration("SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		runtime:         runtime,
		Environment:     value("APP_ENV", "development"),
		ShutdownTimeout: shutdownTimeout,
		OTLPEndpoint:    os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
	}
	// Keep the existing runtime names authoritative when both names are set.
	if runtime.usesHTTP() {
		originDefault := ""
		if cfg.Environment == "development" || cfg.Environment == "test" {
			originDefault = "http://localhost:3000"
		}
		cfg.CORSOrigins = value("HTTP_CORS_ORIGINS", originDefault)
		cfg.HTTPRatePerSecond, err = strconv.Atoi(value("HTTP_RATE_PER_SECOND", "100"))
		if err != nil {
			return Config{}, fmt.Errorf("HTTP_RATE_PER_SECOND must be a positive integer")
		}
		cfg.HTTPRateBurst, err = strconv.Atoi(value("HTTP_RATE_BURST", "200"))
		if err != nil {
			return Config{}, fmt.Errorf("HTTP_RATE_BURST must be a positive integer")
		}
		cfg.HTTPMaxHeaderBytes, err = strconv.Atoi(value("HTTP_MAX_HEADER_BYTES", "32768"))
		if err != nil || cfg.HTTPMaxHeaderBytes <= 0 {
			return Config{}, fmt.Errorf("HTTP_MAX_HEADER_BYTES must be a positive integer")
		}
		cfg.HTTPAddress = value("HTTP_ADDRESS", ":"+value("HTTP_PORT", "8080"))
	}
	cfg.DatabaseURL, err = databaseURL()
	if err != nil {
		return Config{}, err
	}
	cfg.DatabasePool, err = loadDatabasePool()
	if err != nil {
		return Config{}, err
	}
	if runtime.usesSearch() {
		fallback := ""
		if cfg.Environment == "development" || cfg.Environment == "test" {
			fallback = "http://localhost:9200"
		}
		cfg.ElasticsearchURL = value("ELASTICSEARCH_URL", value("SEARCH_URL", fallback))
	}
	if runtime.usesOutbox() {
		cfg.OutboxPollInterval, err = duration("OUTBOX_POLL_INTERVAL", time.Second)
		if err != nil {
			return Config{}, err
		}
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
		{"HTTP_REQUEST_TIMEOUT", 8 * time.Second, &cfg.HTTPRequestTimeout},
		{"HTTP_READ_HEADER_TIMEOUT", 5 * time.Second, &cfg.HTTPReadHeaderTimeout},
		{"HTTP_READ_TIMEOUT", 10 * time.Second, &cfg.HTTPReadTimeout},
		{"HTTP_WRITE_TIMEOUT", 15 * time.Second, &cfg.HTTPWriteTimeout},
		{"HTTP_IDLE_TIMEOUT", 60 * time.Second, &cfg.HTTPIdleTimeout},
	} {
		if !runtime.usesHTTP() {
			continue
		}
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
	if c.runtime.usesHTTP() {
		if c.HTTPMaxHeaderBytes <= 0 {
			return fmt.Errorf("HTTP_MAX_HEADER_BYTES must be positive")
		}
		_, port, err := net.SplitHostPort(c.HTTPAddress)
		if err != nil || !validPort(port) {
			return fmt.Errorf("HTTP_ADDRESS (or HTTP_PORT) must specify host:port with port between 1 and 65535")
		}
	}
	if c.DatabaseURL == "" {
		if c.Environment == "staging" || c.Environment == "production" {
			return fmt.Errorf("DATABASE_URL or DATABASE_HOST/NAME/USER is required in staging and production")
		}
	} else if !validURL(c.DatabaseURL, "postgres", "postgresql") {
		return fmt.Errorf("DATABASE_URL must be a postgres or postgresql URL with a host and valid port")
	}
	if c.runtime.usesSearch() && c.ElasticsearchURL == "" {
		return fmt.Errorf("ELASTICSEARCH_URL (or SEARCH_URL) is required in staging and production")
	}
	if c.runtime.usesSearch() && !validURL(c.ElasticsearchURL, "http", "https") {
		return fmt.Errorf("ELASTICSEARCH_URL (or SEARCH_URL) must be an http or https URL with a host and valid port")
	}
	if c.OTLPEndpoint != "" && !validURL(c.OTLPEndpoint, "http", "https") {
		return fmt.Errorf("OTEL_EXPORTER_OTLP_ENDPOINT must be an http or https URL with a host and valid port")
	}
	for _, setting := range []struct {
		key   string
		value time.Duration
		used  bool
	}{
		{"OUTBOX_POLL_INTERVAL", c.OutboxPollInterval, c.runtime.usesOutbox()},
		{"SHUTDOWN_TIMEOUT", c.ShutdownTimeout, true},
		{"HTTP_READ_HEADER_TIMEOUT", c.HTTPReadHeaderTimeout, c.runtime.usesHTTP()},
		{"HTTP_READ_TIMEOUT", c.HTTPReadTimeout, c.runtime.usesHTTP()},
		{"HTTP_WRITE_TIMEOUT", c.HTTPWriteTimeout, c.runtime.usesHTTP()},
		{"HTTP_IDLE_TIMEOUT", c.HTTPIdleTimeout, c.runtime.usesHTTP()},
		{"HTTP_REQUEST_TIMEOUT", c.HTTPRequestTimeout, c.runtime.usesHTTP()},
	} {
		if setting.used && setting.value <= 0 {
			return fmt.Errorf("%s must be positive", setting.key)
		}
	}
	if c.runtime.usesHTTP() {
		if c.HTTPRatePerSecond <= 0 {
			return fmt.Errorf("HTTP_RATE_PER_SECOND must be positive")
		}
		if c.HTTPRateBurst <= 0 {
			return fmt.Errorf("HTTP_RATE_BURST must be positive")
		}
		if c.HTTPRequestTimeout >= c.HTTPWriteTimeout {
			return fmt.Errorf("HTTP_REQUEST_TIMEOUT must be shorter than HTTP_WRITE_TIMEOUT")
		}
		for _, origin := range strings.Split(c.CORSOrigins, ",") {
			origin = strings.TrimSpace(origin)
			if origin == "" {
				continue
			}
			u, err := url.Parse(origin)
			if err != nil || !validURL(origin, "http", "https") || u.User != nil || u.Path != "" || u.RawQuery != "" || strings.Contains(origin, "*") {
				return fmt.Errorf("HTTP_CORS_ORIGINS must contain explicit http(s) origins")
			}
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
