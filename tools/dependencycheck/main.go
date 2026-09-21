// Command dependencycheck enforces module boundaries and approved direct imports.
package main

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type externalRule struct {
	Prefix    string   `json:"prefix"`
	Importers []string `json:"importers"`
	TestOnly  bool     `json:"test_only"`
}

type policy struct {
	Modules             []string       `json:"modules"`
	Platforms           []string       `json:"platforms"`
	PureStandardLibrary []string       `json:"pure_standard_library"`
	External            []externalRule `json:"external"`
}

type location struct{ kind, module, layer, component string }

func within(value, prefix string) bool {
	return value == prefix || strings.HasPrefix(value, prefix+"/")
}
func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

// A trailing /** matches the package itself and any descendant; * matches one segment.
func matches(pattern, value string) bool {
	if strings.HasSuffix(pattern, "/**") {
		base := strings.TrimSuffix(pattern, "/**")
		parts := strings.Split(value, "/")
		for n := len(parts); n > 0; n-- {
			if ok, _ := path.Match(base, strings.Join(parts[:n], "/")); ok {
				return true
			}
		}
		return false
	}
	ok, _ := path.Match(pattern, value)
	return ok
}

func (p policy) classify(pkg string) location {
	parts := strings.Split(pkg, "/")
	if len(parts) >= 2 && parts[0] == "cmd" {
		return location{kind: "cmd"}
	}
	if len(parts) >= 2 && parts[0] == "tools" {
		return location{kind: "tools"}
	}
	if len(parts) >= 2 && parts[0] == "test" && contains([]string{"integration", "e2e", "fixtures"}, parts[1]) {
		return location{kind: "test"}
	}
	if len(parts) < 2 || parts[0] != "internal" {
		return location{}
	}
	if parts[1] == "bootstrap" {
		return location{kind: "bootstrap"}
	}
	if parts[1] == "platform" {
		if len(parts) >= 3 && contains(p.Platforms, parts[2]) {
			return location{kind: "platform", component: parts[2]}
		}
		return location{}
	}
	if !contains(p.Modules, parts[1]) {
		return location{}
	}
	if len(parts) == 2 {
		return location{kind: "module-root", module: parts[1]}
	}
	if !contains([]string{"domain", "application", "contract", "infrastructure", "transport"}, parts[2]) {
		return location{}
	}
	return location{kind: "module", module: parts[1], layer: parts[2]}
}

func (p policy) checkImport(module, importer, imported string, test bool, std map[string]bool) string {
	source := p.classify(importer)
	if std[imported] {
		if (within(imported, "testing") || imported == "net/http/httptest") && !test {
			return "standard-library test dependency imported by production code"
		}
		if within(imported, "database/sql") && !(matches("internal/platform/database/**", importer) || matches("internal/*/infrastructure/postgres/**", importer) || (source.kind == "test" && test)) {
			return "SQL access is restricted to PostgreSQL adapters"
		}
		if source.kind == "module" && contains([]string{"domain", "application", "contract"}, source.layer) {
			if test && imported == "testing" {
				return ""
			}
			if source.layer == "application" && imported == "log/slog" {
				return ""
			}
			for _, allowed := range p.PureStandardLibrary {
				if within(imported, allowed) {
					return ""
				}
			}
			return "standard-library I/O/framework dependency is not allowed in this layer"
		}
		return ""
	}
	if within(imported, module) {
		targetPath := strings.TrimPrefix(imported, module+"/")
		target := p.classify(targetPath)
		if target.kind == "" || target.kind == "module-root" || target.kind == "cmd" || target.kind == "tools" {
			return "target is not an importable architecture package"
		}
		if test && targetPath == importer {
			return ""
		} // external package tests
		switch source.kind {
		case "cmd":
			if target.kind == "bootstrap" {
				return ""
			}
		case "tools":
			return "tooling must not depend on application code"
		case "test":
			if test && target.kind != "test" {
				return ""
			}
		case "bootstrap":
			if target.kind != "test" {
				return ""
			}
		case "platform":
			if target.kind == "platform" {
				return ""
			}
		case "module":
			if target.kind == "platform" {
				if source.layer == "infrastructure" {
					return ""
				}
				if source.layer == "transport" && contains([]string{"logging", "telemetry"}, target.component) {
					return ""
				}
			}
			if target.kind == "module" {
				if source.module != target.module {
					if target.layer == "contract" && contains([]string{"application", "infrastructure"}, source.layer) {
						return ""
					}
					return "cross-module imports must use a contract from application/infrastructure"
				}
				allowed := map[string][]string{
					"contract":       {"contract"},
					"domain":         {"domain"},
					"application":    {"application", "domain", "contract"},
					"infrastructure": {"infrastructure", "application", "domain", "contract"},
					"transport":      {"transport", "application", "domain", "contract"},
				}
				if contains(allowed[source.layer], target.layer) {
					return ""
				}
			}
		}
		return "dependency direction violates architecture policy"
	}
	// The most specific prefix wins, so an SDK cannot inherit the API rule.
	var chosen *externalRule
	for i := range p.External {
		rule := &p.External[i]
		if within(imported, rule.Prefix) && (chosen == nil || len(rule.Prefix) > len(chosen.Prefix)) {
			chosen = rule
		}
	}
	if chosen == nil {
		return "external package is not approved in dependency-policy.json"
	}
	if chosen.TestOnly && !test {
		return "test dependency imported by production code"
	}
	// Assertions are allowed in unit tests; they do not relax internal boundaries.
	if source.kind == "module" && contains([]string{"domain", "contract", "application"}, source.layer) && !chosen.TestOnly {
		return "core layers must not depend on external libraries"
	}
	for _, allowed := range chosen.Importers {
		if matches(allowed, importer) {
			return ""
		}
	}
	return "external package is not approved for this importer"
}

func scan(root, module string, p policy, std map[string]bool) ([]string, error) {
	var violations []string
	err := filepath.WalkDir(root, func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if filename != root && (strings.HasPrefix(entry.Name(), ".") || contains([]string{"vendor", "node_modules", "testdata"}, entry.Name())) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, filename)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.Name() == "go.mod" && rel != "go.mod" {
			violations = append(violations, rel+": nested Go modules require a policy change")
		}
		if entry.Name() == "go.work" {
			violations = append(violations, rel+": Go workspaces require a policy change")
		}
		if !strings.HasSuffix(filename, ".go") {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			violations = append(violations, rel+": Go source symlinks are not supported")
			return nil
		}
		pkg := path.Dir(rel)
		loc := p.classify(pkg)
		if loc.kind == "" {
			violations = append(violations, rel+": unregistered module, layer or package location")
		}
		set := token.NewFileSet()
		file, err := parser.ParseFile(set, filename, nil, 0)
		if err != nil {
			return fmt.Errorf("parse %s: %w", rel, err)
		}
		if loc.kind == "module-root" && (entry.Name() != "doc.go" || len(file.Decls) != 0) {
			violations = append(violations, rel+": module root is documentation-only; use an explicit layer")
		}
		test := strings.HasSuffix(filename, "_test.go")
		if loc.kind == "test" && !test && len(file.Decls) != 0 {
			violations = append(violations, rel+": integration/e2e helpers must use _test.go")
		}
		for _, spec := range file.Imports {
			imported, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			if reason := p.checkImport(module, pkg, imported, test, std); reason != "" {
				violations = append(violations, fmt.Sprintf("%s:%d: %s -> %s: %s", rel, set.Position(spec.Pos()).Line, pkg, imported, reason))
			}
		}
		return nil
	})
	sort.Strings(violations)
	return violations, err
}

func run() error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(root, "dependency-policy.json"))
	if err != nil {
		return fmt.Errorf("run from repository root: %w", err)
	}
	var p policy
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&p); err != nil {
		return err
	}
	modData, err := exec.Command("go", "mod", "edit", "-json").Output()
	if err != nil {
		return err
	}
	var mod struct {
		Module  struct{ Path string }
		Replace []json.RawMessage
	}
	if err := json.Unmarshal(modData, &mod); err != nil {
		return err
	}
	if len(mod.Replace) > 0 {
		return fmt.Errorf("go.mod replace directives require an explicit policy change")
	}
	stdData, err := exec.Command("go", "list", "std").Output()
	if err != nil {
		return err
	}
	std := map[string]bool{}
	for _, pkg := range strings.Fields(string(stdData)) {
		std[pkg] = true
	}
	violations, err := scan(root, mod.Module.Path, p, std)
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		return fmt.Errorf("dependency policy violations:\n%s", strings.Join(violations, "\n"))
	}
	fmt.Println("Dependency policy passed (including tests, generated files and build-tagged source).")
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
