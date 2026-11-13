package inventorytracediag

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

func render(n ast.Node) string {
	var b bytes.Buffer
	_ = format.Node(&b, token.NewFileSet(), n)
	return b.String()
}
func selector(n ast.Expr, owner, name string) bool {
	s, ok := n.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := s.X.(*ast.Ident)
	return ok && id.Name == owner && s.Sel.Name == name
}

// Only logging/context-free syntax may disappear under diagnostic erasure.
// This prevents an enabled wrapper from hiding a native side effect from the
// token-erasure guard. It does not prove runtime attribution or trace overhead.
func diagnosticSyntax(n ast.Node) bool {
	// Closed statement grammar: mentioning the package is not authorization.
	switch x := n.(type) {
	case *ast.IfStmt:
		c, ok := x.Cond.(*ast.CallExpr)
		if !ok || !selector(c.Fun, "inventorytracediag", "Enabled") || len(c.Args) != 0 || x.Init != nil || x.Else != nil || len(x.Body.List) == 0 {
			return false
		}
		for _, child := range x.Body.List {
			if !diagnosticSyntax(child) {
				return false
			}
		}
	case *ast.ExprStmt:
		c, ok := x.X.(*ast.CallExpr)
		if !ok || !directDiagnosticCall(c) {
			return false
		}
	case *ast.DeferStmt:
		if !directDiagnosticCall(x.Call) {
			return false
		}
	case *ast.AssignStmt:
		// Exact CapabilityMasks assignment is checked below.
	default:
		return false
	}
	good := true
	ast.Inspect(n, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.ForStmt, *ast.RangeStmt, *ast.GoStmt, *ast.ReturnStmt, *ast.SendStmt, *ast.IncDecStmt, *ast.DeclStmt:
			good = false
		case *ast.AssignStmt:
			if x.Tok != token.DEFINE || len(x.Lhs) != 3 || len(x.Rhs) != 1 || render(x.Lhs[0]) != "requested" || render(x.Lhs[1]) != "observed" || render(x.Lhs[2]) != "_" {
				good = false
				break
			}
			c, ok := x.Rhs[0].(*ast.CallExpr)
			if !ok || !selector(c.Fun, "inventorytracediag", "CapabilityMasks") {
				good = false
			}
		case *ast.CallExpr:
			if id, ok := x.Fun.(*ast.Ident); ok {
				if id.Name != "len" {
					good = false
				}
				break
			}
			s, ok := x.Fun.(*ast.SelectorExpr)
			if !ok {
				good = false
				break
			}
			if s.Sel.Name == "End" {
				c, ok := s.X.(*ast.CallExpr)
				if !ok || !selector(c.Fun, "inventorytracediag", "Begin") || len(x.Args) != 0 {
					good = false
				}
				break
			}
			if selector(x.Fun, "context", "Background") {
				if len(x.Args) != 0 {
					good = false
				}
				break
			}
			id, ok := s.X.(*ast.Ident)
			if !ok || id.Name != "inventorytracediag" {
				good = false
				break
			}
			switch s.Sel.Name {
			case "Enabled":
				if len(x.Args) != 0 {
					good = false
				}
			case "Event", "Begin":
				if len(x.Args) != 2 {
					good = false
					break
				}
				code, ok := x.Args[1].(*ast.SelectorExpr)
				if !ok {
					good = false
					break
				}
				owner, ok := code.X.(*ast.Ident)
				if !ok || owner.Name != "inventorytracediag" {
					good = false
				}
				switch render(x.Args[0]) {
				case "ctx", "base", "parent", "context.Background()":
				default:
					good = false
				}
			case "CapabilityMasks":
				if render(x) != "inventorytracediag.CapabilityMasks(w.spec.plan.Calls, w.spec.plan.References, w.spec.plan.Definitions, metadata.DocumentSymbolSupport, metadata.CallHierarchySupport, metadata.ReferencesSupport, metadata.DefinitionSupport)" {
					good = false
				}
			case "Capabilities":
				if render(x) != "inventorytracediag.Capabilities(parent, requested, observed)" {
					good = false
				}
			case "MethodState":
				if render(x) != "inventorytracediag.MethodState(context.Background(), kind == programcadmission.TypedResolvesToDefinition, len(parsed.Items) == 0)" {
					good = false
				}
			default:
				good = false
			}
		}
		return true
	})
	return good
}
func TestTraceDiagnosticSyntax(t *testing.T) {
	disabled(t)
	for _, s := range []struct {
		body string
		want bool
	}{
		{`if inventorytracediag.Enabled(){ manager.Start() }`, false},
		{`if inventorytracediag.Enabled() || mutate(){ inventorytracediag.Event(ctx,inventorytracediag.Execute) }`, false},
		{`if inventorytracediag.Enabled(){ inventorytracediag.Event(ctx,inventorytracediag.Execute) } else { native() }`, false},
		{`if x := mutate(); inventorytracediag.Enabled(){ inventorytracediag.Event(ctx,inventorytracediag.Execute) }`, false},
		{`if other { inventorytracediag.Event(ctx,inventorytracediag.Execute) }`, false},
		{`if inventorytracediag.Enabled(){ return }`, false},
		{`if inventorytracediag.Enabled(){ inventorytracediag.Event(mutate(),inventorytracediag.Execute) }`, false},
		{`inventorytracediag.Begin(ctx,inventorytracediag.Execute).End()`, true},
		{`if failure != nil && finishErr != nil { inventorytracediag.Event(ctx,inventorytracediag.Execute) }`, false},
		{`if inventorytracediag.Enabled(){ inventorytracediag.Event(ctx,inventorytracediag.Execute) }`, true},
		{`if inventorytracediag.Enabled(){ originalContext = context.Background() }`, false},
		{`inventorytracediag.Event(ctx, arbitraryCode)`, false},
	} {
		f, e := parser.ParseFile(token.NewFileSet(), "candidate", "package p;func f(){"+s.body+"}", 0)
		if e != nil {
			t.Fatal(e)
		}
		stmt := f.Decls[0].(*ast.FuncDecl).Body.List[0]
		if diagnosticSyntax(stmt) != s.want {
			t.Errorf("ASSERT_TRACE_DIAGNOSTIC_SYNTAX discriminator body=%s want=%v", s.body, s.want)
		}
	}
	for _, path := range currentPaths[:4] {
		src, e := os.ReadFile(filepath.Join("../..", path))
		if e != nil {
			t.Fatal(e)
		}
		f, e := parser.ParseFile(token.NewFileSet(), path, src, 0)
		if e != nil {
			t.Fatal(e)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if s, ok := n.(ast.Stmt); ok && diagStatement(s) {
				if !diagnosticSyntax(s) {
					t.Fatalf("ASSERT_TRACE_DIAGNOSTIC_SYNTAX %s", path)
				}
				return false
			}
			return true
		})
	}
	t.Log("ASSERT_TRACE_DIAGNOSTIC_SYNTAX PASS (structural; adversarial AST states distinguished)")
}
