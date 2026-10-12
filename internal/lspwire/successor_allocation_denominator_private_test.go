package lspwire

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSuccessorAllocationProductionDenominatorAndCapacityFormulas(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("source location unavailable")
	}
	file := filepath.Join(filepath.Dir(here), "successor_ingress.go")
	f, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	calls := map[SuccessorAllocationRole][]string{}
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 2 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "ReserveSuccessorAllocation" {
			return true
		}
		roleID, ok := call.Args[0].(*ast.Ident)
		if !ok {
			t.Fatalf("reservation role is not a typed identifier: %T", call.Args[0])
		}
		role := map[string]SuccessorAllocationRole{
			"SuccessorAllocationPrefetch":       SuccessorAllocationPrefetch,
			"SuccessorAllocationHeader":         SuccessorAllocationHeader,
			"SuccessorAllocationBody":           SuccessorAllocationBody,
			"SuccessorAllocationConsumeScratch": SuccessorAllocationConsumeScratch,
			"SuccessorAllocationResultToken":    SuccessorAllocationResultToken,
			"SuccessorAllocationOriginalFrame":  SuccessorAllocationOriginalFrame,
			"SuccessorAllocationDecodedMessage": SuccessorAllocationDecodedMessage,
		}[roleID.Name]
		if role == 0 {
			t.Fatalf("unknown production allocation role %q", roleID.Name)
		}
		calls[role] = append(calls[role], normalizedCapacityFormula(t, call.Args[1]))
		return true
	})

	want := map[SuccessorAllocationRole][]string{
		SuccessorAllocationPrefetch:       {"4096"},
		SuccessorAllocationConsumeScratch: {"uint64(n)"},
		SuccessorAllocationHeader:         {"uint64(headerCapacity)"},
		SuccessorAllocationBody:           {"uint64(length)"},
		SuccessorAllocationOriginalFrame:  {"uint64(frameCapacity)"},
		SuccessorAllocationResultToken:    {"uint64(end-start)"},
		SuccessorAllocationDecodedMessage: {"capacity"},
	}
	if len(calls) != len(want) {
		t.Fatalf("ASSERT_SUCCESSOR_ALLOCATION_DENOMINATOR_ROLES got=%v want=%v", calls, want)
	}
	for role, formulas := range want {
		got := calls[role]
		if len(got) != 1 || got[0] != formulas[0] {
			t.Fatalf("ASSERT_SUCCESSOR_ALLOCATION_DENOMINATOR role=%d got=%v want=%v", role, got, formulas)
		}
	}
}

func normalizedCapacityFormula(t *testing.T, e ast.Expr) string {
	t.Helper()
	switch x := e.(type) {
	case *ast.BasicLit:
		return x.Value
	case *ast.Ident:
		return x.Name
	case *ast.CallExpr:
		id, ok := x.Fun.(*ast.Ident)
		if !ok || len(x.Args) != 1 {
			t.Fatalf("unsupported capacity call %T", x.Fun)
		}
		return id.Name + "(" + normalizedCapacityFormula(t, x.Args[0]) + ")"
	case *ast.BinaryExpr:
		return normalizedCapacityFormula(t, x.X) + x.Op.String() + normalizedCapacityFormula(t, x.Y)
	default:
		t.Fatalf("unsupported capacity formula AST %T", e)
		return ""
	}
}
