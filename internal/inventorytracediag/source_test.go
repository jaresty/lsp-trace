package inventorytracediag

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
)

// Current coverage is deliberately independent of the archived historical pins.
var currentPaths = []string{
	"internal/inventorysuccessor/private_mixed_input_operation_v2.go",
	"internal/inventorysuccessor/private_managed_input_operation_v2.go",
	"sessionruntime/private_provider_window.go",
	"internal/referencegrouping/input.go",
	"internal/inventorytracediag/trace.go",
}

func diagExpr(n ast.Node) bool {
	yes := false
	ast.Inspect(n, func(n ast.Node) bool {
		if s, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := s.X.(*ast.Ident); ok && id.Name == "inventorytracediag" {
				yes = true
			}
		}
		return true
	})
	return yes
}
func directDiagnosticCall(c *ast.CallExpr) bool {
	s, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if id, ok := s.X.(*ast.Ident); ok && id.Name == "inventorytracediag" {
		return true
	}
	inner, ok := s.X.(*ast.CallExpr)
	return s.Sel.Name == "End" && ok && selector(inner.Fun, "inventorytracediag", "Begin")
}

// Candidate detection is NOT authorization to erase. Native return-owning
// branches retain their conditions; only their individual logging calls erase.
func diagStatement(s ast.Stmt) bool {
	switch s := s.(type) {
	case *ast.ExprStmt:
		c, ok := s.X.(*ast.CallExpr)
		return ok && directDiagnosticCall(c)
	case *ast.DeferStmt:
		return directDiagnosticCall(s.Call)
	case *ast.IfStmt:
		return diagExpr(s.Cond)
	}
	return false
}

// Package references are resolved through actual imports, including aliases.
// Reject dot/blank imports and package-value escape, not just direct calls.
func importReferences(f *ast.File, path string) (map[*ast.SelectorExpr]bool, error) {
	aliases := map[string]bool{}
	allowed := map[*ast.Ident]bool{}
	for _, im := range f.Imports {
		p, e := strconv.Unquote(im.Path.Value)
		if e != nil {
			return nil, e
		}
		if p != path {
			continue
		}
		name := filepath.Base(p)
		if im.Name != nil {
			name = im.Name.Name
			allowed[im.Name] = true
		}
		if name == "." || name == "_" {
			return nil, fmt.Errorf("unsupported import %s", name)
		}
		aliases[name] = true
	}
	refs := map[*ast.SelectorExpr]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		if s, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := s.X.(*ast.Ident); ok && aliases[id.Name] && id.Obj == nil {
				refs[s] = true
				allowed[id] = true
			}
		}
		return true
	})
	var err error
	ast.Inspect(f, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && aliases[id.Name] && !allowed[id] {
			err = fmt.Errorf("package alias escape or shadow %s", id.Name)
		}
		return true
	})
	return refs, err
}

func eraseDiagnosticSource(path string, src []byte) ([]byte, error) {
	set := token.NewFileSet()
	f, e := parser.ParseFile(set, path, src, 0)
	if e != nil {
		return nil, e
	}
	refs, e := importReferences(f, "lsp-trace/internal/inventorytracediag")
	if e != nil {
		return nil, e
	}
	// Normalize resolved aliases only for the closed syntax predicate; positions
	// still address the original bytes. Unrelated names cannot authorize erasure.
	for s := range refs {
		s.X.(*ast.Ident).Name = "inventorytracediag"
	}
	type span struct{ lo, hi int }
	var spans []span
	remove := func(n ast.Node) {
		lo, hi := set.Position(n.Pos()).Offset, set.Position(n.End()).Offset
		spans = append(spans, span{lo, hi})
	}
	var refusal error
	ast.Inspect(f, func(n ast.Node) bool {
		if im, ok := n.(*ast.ImportSpec); ok && im.Path.Value == `"lsp-trace/internal/inventorytracediag"` {
			remove(im)
			return false
		}
		if s, ok := n.(ast.Stmt); ok && diagStatement(s) {
			resolved := false
			ast.Inspect(s, func(n ast.Node) bool {
				if sel, ok := n.(*ast.SelectorExpr); ok && refs[sel] {
					resolved = true
				}
				return true
			})
			if !resolved || !diagnosticSyntax(s) {
				refusal = fmt.Errorf("unsupported diagnostic syntax: %s", render(s))
				return false
			}
			remove(s)
			return false
		}
		return true
	})
	if refusal != nil {
		return nil, refusal
	}
	// Do not remove an import while leaving a captured/unsupported diagnostic
	// selector behind. Every resolved reference must belong to an approved span.
	for sel := range refs {
		covered := false
		for _, p := range spans {
			if set.Position(sel.Pos()).Offset >= p.lo && set.Position(sel.End()).Offset <= p.hi {
				covered = true
			}
		}
		if !covered {
			return nil, fmt.Errorf("diagnostic reference outside approved statement: %s", render(sel))
		}
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].lo > spans[j].lo })
	out := append([]byte(nil), src...)
	// Replace exactly AST-owned bytes with whitespace (not neighboring native
	// statements on the same line). Preserve newlines and avoid token fusion.
	for _, p := range spans {
		for i := p.lo; i < p.hi; i++ {
			if out[i] != '\n' && out[i] != '\r' {
				out[i] = ' '
			}
		}
	}
	return out, nil
}
func erased(t *testing.T, path string, src []byte) []byte {
	t.Helper()
	out, e := eraseDiagnosticSource(path, src)
	if e != nil {
		t.Fatal(e)
	}
	return out
}

// Closed, import-aware ownership contract; function-value escape is rejected.
// No runtime tracing is activated by this AST reader.
func traceNoOwnership(f *ast.File) bool {
	refs, err := importReferences(f, "runtime/trace")
	if err != nil {
		return false
	}
	calls := map[ast.Expr]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			calls[c.Fun] = true
		}
		return true
	})
	for s := range refs {
		switch s.Sel.Name {
		case "Region": // Existing helper field type, not task/trace ownership.
		case "IsEnabled", "Log", "StartRegion":
			if !calls[s] {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func TestTraceNoOwnership(t *testing.T) {
	disabled(t)
	for _, path := range currentPaths {
		src, e := os.ReadFile(filepath.Join("../..", path))
		if e != nil {
			t.Fatal(e)
		}
		if !traceNoOwnership(parseCurrent(t, src)) {
			t.Fatalf("CURRENT_NO_OWNERSHIP %s", path)
		}
	}
}
func TestTraceDiagnosticErasureSyntax(t *testing.T) {
	disabled(t)
	for _, path := range currentPaths[:4] {
		src, e := os.ReadFile(filepath.Join("../..", path))
		if e != nil {
			t.Fatal(e)
		}
		if _, e := eraseDiagnosticSource(path, src); e != nil {
			t.Fatalf("CURRENT_ERASURE_SYNTAX %s: %v", path, e)
		}
	}
}
