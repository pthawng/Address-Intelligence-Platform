package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
)

// An explicit URL takes precedence. Split settings accept raw credentials;
// net/url performs escaping exactly once, including reserved characters.
func databaseURL() (string, error) {
	if raw := os.Getenv("DATABASE_URL"); raw != "" {
		return raw, nil
	}
	keys := []string{"DATABASE_HOST", "DATABASE_PORT", "DATABASE_NAME", "DATABASE_USER", "DATABASE_PASSWORD", "DATABASE_SSLMODE"}
	present := false
	for _, key := range keys {
		present = present || os.Getenv(key) != ""
	}
	if !present {
		return "", nil
	}
	for _, key := range []string{"DATABASE_HOST", "DATABASE_NAME", "DATABASE_USER"} {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			return "", fmt.Errorf("%s is required when using split database settings", key)
		}
	}
	host := os.Getenv("DATABASE_HOST")
	if strings.ContainsAny(host, "/?#@[] \\ \t\r\n") || (strings.Contains(host, ":") && net.ParseIP(host) == nil) {
		return "", fmt.Errorf("DATABASE_HOST must be a hostname or IP without a port")
	}
	port := value("DATABASE_PORT", "5432")
	if !validPort(port) {
		return "", fmt.Errorf("DATABASE_PORT must be between 1 and 65535")
	}
	sslmode := value("DATABASE_SSLMODE", "verify-full")
	switch sslmode {
	case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
	default:
		return "", fmt.Errorf("DATABASE_SSLMODE must be disable, allow, prefer, require, verify-ca or verify-full")
	}
	db := &url.URL{
		Scheme:   "postgres",
		Host:     net.JoinHostPort(host, port),
		User:     url.UserPassword(os.Getenv("DATABASE_USER"), os.Getenv("DATABASE_PASSWORD")),
		Path:     "/" + os.Getenv("DATABASE_NAME"),
		RawQuery: url.Values{"sslmode": {sslmode}}.Encode(),
	}
	return db.String(), nil
}
