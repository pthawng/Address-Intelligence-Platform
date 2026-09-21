package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testPolicy(t *testing.T) policy {
	t.Helper()
	data, err := os.ReadFile("../../dependency-policy.json")
	if err != nil {
		t.Fatal(err)
	}
	var p policy
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestImportBoundaries(t *testing.T) {
	p := testPolicy(t)
	const module = "address-intelligence-platform"
	for _, tc := range []struct {
		name, from, to string
		test, allowed  bool
	}{
		{"application to contract", "internal/resolution/application", module + "/internal/autocomplete/contract", false, true},
		{"application to foreign repository", "internal/resolution/application", module + "/internal/autocomplete/infrastructure/postgres", false, false},
		{"application to foreign service", "internal/resolution/application", module + "/internal/autocomplete/application", false, false},
		{"application to own repository", "internal/place/application", module + "/internal/place/infrastructure/postgres", false, false},
		{"infrastructure to foreign repository", "internal/resolution/infrastructure", module + "/internal/place/infrastructure", false, false},
		{"domain to own domain", "internal/place/domain/query", module + "/internal/place/domain", false, true},
		{"domain to foreign contract", "internal/place/domain", module + "/internal/search/contract", false, false},
		{"contract leaking domain", "internal/place/contract", module + "/internal/place/domain", false, false},
		{"contract chaining foreign contract", "internal/place/contract", module + "/internal/search/contract", false, false},
		{"application to own domain", "internal/place/application", module + "/internal/place/domain", false, true},
		{"transport to app", "internal/place/transport/http", module + "/internal/place/application", false, true},
		{"transport bypassing app", "internal/place/transport/http", module + "/internal/place/infrastructure/postgres", false, false},
		{"transport to database platform", "internal/place/transport/http", module + "/internal/platform/database", false, false},
		{"platform to business", "internal/platform/database", module + "/internal/place/domain", false, false},
		{"bootstrap wiring", "internal/bootstrap", module + "/internal/place/infrastructure/postgres", false, true},
		{"main bypassing bootstrap", "cmd/api", module + "/internal/place/application", false, false},
		{"main to bootstrap", "cmd/api", module + "/internal/bootstrap", false, true},
		{"unknown module", "internal/bootstrap", module + "/internal/address/application", false, false},
		{"root module import", "internal/bootstrap", module + "/internal/place", false, false},
		{"pgx approved adapter", "internal/place/infrastructure/postgres", "github.com/jackc/pgx/v5", false, true},
		{"pgx in core", "internal/place/application", "github.com/jackc/pgx/v5", false, false},
		{"pool owner", "internal/platform/database", "github.com/jackc/pgx/v5/pgxpool", false, true},
		{"pool outside owner", "internal/place/infrastructure/postgres", "github.com/jackc/pgx/v5/pgxpool", false, false},
		{"http framework", "internal/place/transport/http", "github.com/go-chi/chi/v5/middleware", false, true},
		{"unapproved framework", "internal/place/transport/http", "github.com/gin-gonic/gin", false, false},
		{"prefix spoofing", "internal/place/transport/http", "github.com/go-chi/chi/v50", false, false},
		{"redis deferred", "internal/place/infrastructure/redis", "github.com/redis/go-redis/v9", false, false},
		{"otel api", "internal/place/transport/http", "go.opentelemetry.io/otel/trace", false, true},
		{"otel sdk cannot inherit api permission", "internal/place/transport/http", "go.opentelemetry.io/otel/sdk/trace", false, false},
		{"otel sdk owner", "internal/platform/telemetry", "go.opentelemetry.io/otel/sdk/trace", false, true},
		{"unit tests keep boundaries", "internal/place/application", module + "/internal/search/infrastructure", true, false},
		{"integration wiring", "test/integration", module + "/internal/place/infrastructure", true, true},
		{"assertions in tests", "internal/place/domain", "github.com/stretchr/testify/require", true, true},
		{"assertions in production", "internal/place/domain", "github.com/stretchr/testify/require", false, false},
		{"domain network", "internal/place/domain", "net/http", false, false},
		{"application environment", "internal/place/application", "os", false, false},
		{"application logging", "internal/place/application", "log/slog", false, true},
		{"pure context", "internal/place/contract", "context", false, true},
		{"unit testing", "internal/place/domain", "testing", true, true},
		{"transport SQL escape", "internal/place/transport/http", "database/sql", false, false},
		{"adapter SQL", "internal/place/infrastructure/postgres", "database/sql", false, true},
		{"production testing", "internal/bootstrap", "testing", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			std := map[string]bool{"net/http": true, "os": true, "log/slog": true, "context": true, "testing": true, "database/sql": true}
			reason := p.checkImport(module, tc.from, tc.to, tc.test, std)
			if (reason == "") != tc.allowed {
				t.Fatalf("allowed=%v, want %v (%s)", reason == "", tc.allowed, reason)
			}
		})
	}
}

func TestScanIncludesTaggedGeneratedAndAliasedImports(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"internal/place/application/tagged.go":       "//go:build never\n\npackage application\nimport alias \"address-intelligence-platform/internal/search/infrastructure\"\n",
		"internal/place/application/generated.go":    "// Code generated by test. DO NOT EDIT.\npackage application\nimport _ \"github.com/redis/go-redis/v9\"\n",
		"internal/place/application/service_test.go": "package application_test\nimport . \"address-intelligence-platform/internal/search/application\"\n",
		"internal/place/doc.go":                      "package place\nfunc EscapeHatch() {}\n",
		"internal/mystery/domain/entity.go":          "package domain\n",
		"nested/go.mod":                              "module hidden\n",
	}
	for name, content := range files {
		filename := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	violations, err := scan(root, "address-intelligence-platform", testPolicy(t), map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != len(files) {
		t.Fatalf("got %d violations, want %d: %v", len(violations), len(files), violations)
	}
	for name := range files {
		if !strings.Contains(strings.Join(violations, "\n"), name+":") {
			t.Fatalf("missing violation for %s", name)
		}
	}
}

func TestMalformedSourceFailsClosed(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "bad.go"), []byte("not Go"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := scan(root, "example", testPolicy(t), nil); err == nil {
		t.Fatal("invalid Go source must fail")
	}
}
