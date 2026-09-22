package programccompose

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"
)

type branchSentinel struct{}

func (*branchSentinel) Error() string { return "sentinel" }

type branchWrapper struct{ error }

func TestComposeBranchInventoryAndErrorContract(t *testing.T) {
	input, _ := os.ReadFile("compose.go")
	f, err := parser.ParseFile(token.NewFileSet(), "compose.go", input, 0)
	if err != nil {
		t.Fatal(err)
	}
	codes := map[string]bool{}
	for _, code := range []string{"BranchInputCount", "BranchInputBytes", "BranchInputIdentity", "BranchInputMetadata", "BranchInputIdentityConflict", "BranchInputProvenance", "BranchInputEnvelopeDecode", "BranchInputGraphBase64", "BranchInputGraphDecode", "BranchSourceRecords", "BranchCompatibility", "BranchSummaryDecode", "BranchNodeConflict", "BranchOccurrenceLimit", "BranchEdgeConflict", "BranchResourceLimit"} {
		codes[code] = strings.Contains(string(input), "tagged("+code)
	}
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "Compose" {
			continue
		}
		ast.Inspect(fn, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 2 {
				return true
			}
			if sel, ok := call.Fun.(*ast.Ident); ok && sel.Name == "tagged" {
				if lit, ok := call.Args[0].(*ast.SelectorExpr); ok {
					codes[lit.Sel.Name] = true
				}
			}
			return true
		})
	}
	if len(codes) != 16 {
		t.Fatalf("ASSERT_STRUCTURAL_16_BRANCHES got=%d", len(codes))
	}
	for code, present := range codes {
		if !present {
			t.Fatalf("ASSERT_STRUCTURAL_BRANCH_MISSING %s", code)
		}
	}
}

func TestComposeBranchContract(t *testing.T) {
	cause := &branchSentinel{}
	wrapped := tagged(BranchInputCount, fmt.Errorf("input count: %w", cause))
	if ErrorBranch(nil) != BranchUnknown || ErrorBranch(errors.New("plain")) != BranchUnknown || ErrorBranch(tagged(BranchCode("NOPE"), cause)) != BranchUnknown {
		t.Fatal("ASSERT_UNKNOWN_NORMALIZATION")
	}
	if ErrorBranch(wrapped) != BranchInputCount || wrapped.Error() != "input count: sentinel" || !errors.Is(wrapped, cause) {
		t.Fatal("ASSERT_ERROR_TEXT_IS_CHAIN")
	}
	var got *branchSentinel
	if !errors.As(wrapped, &got) || got != cause {
		t.Fatal("ASSERT_ERROR_AS_CHAIN")
	}
	if reflect.ValueOf(wrapped).Pointer() == reflect.ValueOf(cause).Pointer() {
		t.Fatal("ASSERT_TRANSPARENT_WRAPPER_EXPECTED")
	}
}

func TestComposeBranchSuccessFingerprintUnchanged(t *testing.T) {
	a, b := capture(t, "branch-a", nil, nil, true, false), capture(t, "branch-b", nil, nil, true, false)
	one, err := Compose([]Input{a, b})
	if err != nil {
		t.Fatal(err)
	}
	two, err := Compose([]Input{b, a})
	if err != nil {
		t.Fatal(err)
	}
	if string(one.Bytes) != string(two.Bytes) || one.Artifact.OutputSHA256 != two.Artifact.OutputSHA256 {
		t.Fatal("ASSERT_SUCCESS_FINGERPRINT_PERMUTATION")
	}
}
