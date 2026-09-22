package main

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestContextPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, path, source string
		want               int
	}{
		{"first", "internal/search/application/search.go", `func Search(ctx context.Context, q string) {}`, 0},
		{"pure", "internal/search/domain/address.go", `func Normalize(q string) string { return q }`, 0},
		{"last", "internal/search/application/search.go", `func Search(q string, ctx context.Context) {}`, 1},
		{"name", "internal/search/application/search.go", `func Search(c context.Context) {}`, 1},
		{"interface", "internal/search/application/ports.go", `type Repo interface { Get(int, context.Context) error }`, 1},
		{"callback", "internal/search/application/ports.go", `type F func(string, context.Context) error`, 1},
		{"struct", "internal/search/application/search.go", `type Service struct { ctx context.Context }`, 1},
		{"embedded", "internal/search/application/search.go", `type Service struct { context.Context }`, 1},
		{"repository root", "internal/search/infrastructure/postgres/repo.go", `var root = context.Background`, 1},
		{"todo", "cmd/api/main.go", `var root = context.TODO()`, 1},
		{"detached", "internal/search/application/search.go", `func Search(ctx context.Context) { _ = context.WithoutCancel(ctx) }`, 1},
		{"entry root", "cmd/api/main.go", `var root = context.Background()`, 0},
		{"cleanup", "internal/bootstrap/app.go", `func stop(ctx context.Context) { _ = context.WithoutCancel(ctx) }`, 0},
		{"test root", "internal/search/application/search_test.go", `var root = context.Background()`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			set := token.NewFileSet()
			file, err := parser.ParseFile(set, tc.path, "package example\nimport \"context\"\n"+tc.source, 0)
			if err != nil {
				t.Fatal(err)
			}
			if got := checkContexts(file, set, tc.path); len(got) != tc.want {
				t.Fatalf("got %v, want %d violations", got, tc.want)
			}
		})
	}
}

func TestContextImportAlias(t *testing.T) {
	for _, src := range []string{
		`package example; import c "context"; func Get(id int, ctx c.Context) { _ = c.Background() }`,
		`package example; import . "context"; func Get() { _ = Background() }`,
	} {
		set := token.NewFileSet()
		file, err := parser.ParseFile(set, "repo.go", src, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(checkContexts(file, set, "internal/search/infrastructure/postgres/repo.go")) == 0 {
			t.Fatal("alias bypassed context checks")
		}
	}
}
