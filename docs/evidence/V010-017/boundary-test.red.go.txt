package auth

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// This architecture check proves only the code boundary. Application tests and
// the existing real HTTP/PG/Redis suite separately prove product behavior.
func TestHTTPFunctionsDoNotExecuteAuthenticationStorage(t *testing.T) {
	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".go") || strings.HasSuffix(file.Name(), "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file.Name(), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			httpInput := false
			ast.Inspect(fn.Type, func(n ast.Node) bool {
				if sel, ok := n.(*ast.SelectorExpr); ok {
					if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "http" && (sel.Sel.Name == "Request" || sel.Sel.Name == "ResponseWriter") {
						httpInput = true
					}
				}
				return true
			})
			if !httpInput {
				continue
			}
			checked++
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch f := call.Fun.(type) {
				case *ast.Ident:
					if f.Name == "hashPassword" || f.Name == "verifyPassword" {
						t.Errorf("%s: HTTP function %s performs credential work", file.Name(), fn.Name.Name)
					}
				case *ast.SelectorExpr:
					if f.Sel.Name == "BeginTx" || f.Sel.Name == "Commit" || f.Sel.Name == "Rollback" {
						t.Errorf("%s: HTTP function %s owns a database transaction", file.Name(), fn.Name.Name)
					}
					if pkg, ok := f.X.(*ast.Ident); ok && (pkg.Name == "authsql" || pkg.Name == "persistence" || pkg.Name == "invitation") {
						t.Errorf("%s: HTTP function %s bypasses the application boundary", file.Name(), fn.Name.Name)
					}
				}
				return true
			})
		}
	}
	if checked == 0 {
		t.Fatal("architecture check did not inspect any HTTP functions")
	}
}
