package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"strings"
)

// checkContexts is a syntax guard, not a whole-program data-flow analysis.
// Tests may create roots and intentionally exercise invalid signatures.
func checkContexts(file *ast.File, set *token.FileSet, rel string) []string {
	if strings.HasSuffix(rel, "_test.go") || !(strings.HasPrefix(rel, "internal/") || strings.HasPrefix(rel, "cmd/")) {
		return nil
	}
	var violations []string
	report := func(pos token.Pos, reason string) {
		violations = append(violations, fmt.Sprintf("%s:%d: context policy: %s", rel, set.Position(pos).Line, reason))
	}
	alias := ""
	for _, spec := range file.Imports {
		name, _ := strconv.Unquote(spec.Path.Value)
		if name != "context" {
			continue
		}
		alias = "context"
		if spec.Name != nil {
			alias = spec.Name.Name
		}
		if alias == "." {
			report(spec.Pos(), "dot import of context is not allowed")
		}
	}
	if alias == "" {
		return violations
	}
	isContext := func(expr ast.Expr) bool {
		selector, ok := expr.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		pkg, ok := selector.X.(*ast.Ident)
		return ok && pkg.Name == alias && selector.Sel.Name == "Context"
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.FuncType:
			if n.Params == nil {
				break
			}
			for i, field := range n.Params.List {
				if !isContext(field.Type) {
					continue
				}
				if i != 0 || len(field.Names) > 1 {
					report(field.Pos(), "context.Context must be the first, single parameter")
				}
				if len(field.Names) == 1 && field.Names[0].Name != "ctx" {
					report(field.Pos(), "name the context parameter ctx")
				}
			}
		case *ast.StructType:
			for _, field := range n.Fields.List {
				if isContext(field.Type) {
					report(field.Pos(), "pass context to methods instead of storing it in a struct")
				}
			}
		case *ast.SelectorExpr:
			pkg, ok := n.X.(*ast.Ident)
			if !ok || pkg.Name != alias {
				break
			}
			switch n.Sel.Name {
			case "TODO":
				report(n.Pos(), "context.TODO is not allowed in production code")
			case "Background":
				if !strings.HasPrefix(rel, "cmd/") {
					report(n.Pos(), "create root contexts only at cmd entry points")
				}
			case "WithoutCancel":
				if rel != "internal/bootstrap/api.go" && rel != "internal/bootstrap/app.go" {
					report(n.Pos(), "detaching cancellation is reserved for bootstrap draining and bounded cleanup")
				}
			}
		}
		return true
	})
	return violations
}
