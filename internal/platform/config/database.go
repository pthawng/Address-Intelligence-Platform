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
	return connectionURL("DATABASE_")
}

// MigrationURL uses only migration credentials, never runtime credentials.
func MigrationURL() (string, error) {
	return connectionURL("MIGRATION_DATABASE_")
}

func connectionURL(prefix string) (string, error) {
	if raw := os.Getenv(prefix + "URL"); raw != "" {
		return raw, nil
	}
	keys := []string{"HOST", "PORT", "NAME", "USER", "PASSWORD", "SSLMODE"}
	present := false
	for _, key := range keys {
		present = present || os.Getenv(prefix+key) != ""
	}
	if !present {
		return "", nil
	}
	for _, suffix := range []string{"HOST", "NAME", "USER"} {
		key := prefix + suffix
		if strings.TrimSpace(os.Getenv(key)) == "" {
			return "", fmt.Errorf("%s is required when using split database settings", key)
		}
	}
	host := os.Getenv(prefix + "HOST")
	if strings.ContainsAny(host, "/?#@[] \\ \t\r\n") || (strings.Contains(host, ":") && net.ParseIP(host) == nil) {
		return "", fmt.Errorf("%sHOST must be a hostname or IP without a port", prefix)
	}
	port := value(prefix+"PORT", "5432")
	if !validPort(port) {
		return "", fmt.Errorf("%sPORT must be between 1 and 65535", prefix)
	}
	sslmode := value(prefix+"SSLMODE", "verify-full")
	switch sslmode {
	case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
	default:
		return "", fmt.Errorf("%sSSLMODE must be disable, allow, prefer, require, verify-ca or verify-full", prefix)
	}
	db := &url.URL{
		Scheme:   "postgres",
		Host:     net.JoinHostPort(host, port),
		User:     url.UserPassword(os.Getenv(prefix+"USER"), os.Getenv(prefix+"PASSWORD")),
		Path:     "/" + os.Getenv(prefix+"NAME"),
		RawQuery: url.Values{"sslmode": {sslmode}}.Encode(),
	}
	return db.String(), nil
}
