package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// The structural gate inspects the actual runner, not a mirrored route. The
// behavioral companion exercises that runner with refusal-only dependencies.
func TestQualifiedPreparationDispatchPrivate(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "census_runner.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var body *ast.BlockStmt
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "runCensusWithDependencies" {
			body = fn.Body
		}
	}
	if body == nil {
		t.Fatal("actual runner absent")
	}
	prepare, profile, starter := -1, -1, -1
	for i, s := range body.List {
		if branch, ok := s.(*ast.IfStmt); ok {
			ast.Inspect(branch.Cond, func(n ast.Node) bool {
				if v, ok := n.(*ast.SelectorExpr); ok && v.Sel.Name == "PrivateInventoryQualifiedPrepareConfigPath" {
					if len(branch.Body.List) == 1 {
						if ret, ok := branch.Body.List[0].(*ast.ReturnStmt); ok && len(ret.Results) == 1 {
							if call, ok := ret.Results[0].(*ast.CallExpr); ok {
								if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "runCensusApplicationQualifiedCLIPrivate" {
									prepare = i
								}
							}
						}
					}
				}
				return true
			})
		}
		ast.Inspect(s, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok {
				switch fn := c.Fun.(type) {
				case *ast.Ident:
					if fn.Name == "loadRequestedProfile" {
						profile = i
					}
				case *ast.SelectorExpr:
					if fn.Sel.Name == "newStarter" && starter < 0 {
						starter = i
					}
				}
			}
			return true
		})
	}
	if prepare < 0 || profile < 0 || starter < 0 || prepare >= profile || profile >= starter {
		t.Fatal("ASSERT qualified returning dispatch precedes profile and runtime: FAIL")
	}
	t.Log("ASSERT qualified returning dispatch precedes profile and runtime: PASS")
}
