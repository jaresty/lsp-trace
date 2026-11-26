package inventorytracediag

import (
	"bytes"
	"go/ast"
	"os"
	"testing"
)

// The moved event owner still receives the exact original operation/lease data.
// This checks forwarding syntax, not runtime custody or reviewed F04 source pins.
func TestTraceManagedOwnerForwarding(t *testing.T) {
	source, err := os.ReadFile("../inventorysuccessor/private_managed_input_operation_v2.go")
	if err != nil {
		t.Fatal(err)
	}
	binding, err := os.ReadFile("../inventorysuccessor/private_retained_mixed_input_v1.go")
	if err != nil {
		t.Fatal(err)
	}
	check := func(source, binding []byte) bool {
		var wrapper, bind *ast.FuncDecl
		for _, d := range parseCurrent(t, source).Decls {
			if f, ok := d.(*ast.FuncDecl); ok && f.Name.Name == "issuePrivateManagedGroupingInput" && f.Recv == nil {
				wrapper = f
			}
		}
		for _, d := range parseCurrent(t, binding).Decls {
			if f, ok := d.(*ast.FuncDecl); ok && f.Name.Name == "bindGroupingProducerSourceV1" && f.Recv == nil {
				bind = f
			}
		}
		if wrapper == nil || wrapper.Body == nil || len(wrapper.Body.List) != 1 || render(wrapper.Body.List[0]) != "return issuePrivateManagedGroupingOwnedV1(ctx, m, h, q, nil)" {
			return false
		}
		if bind == nil || bind.Body == nil || len(bind.Body.List) == 0 {
			return false
		}
		first, ok := bind.Body.List[0].(*ast.IfStmt)
		return ok && first.Init == nil && first.Else == nil && render(first.Cond) == "r == nil" && len(first.Body.List) == 1 && render(first.Body.List[0]) == "return p.bindOwnedFixed(h, source)"
	}
	if !check(source, binding) {
		t.Fatal("CURRENT_OWNER_FORWARDING")
	}
	for _, replacement := range []string{"return issuePrivateManagedGroupingOwnedV1(ctx, m, h, q, detached)", "return issuePrivateManagedGroupingOwnedV1(ctx, m, other, q, nil)", "return otherOwner(ctx, m, h, q, nil)"} {
		changed := bytes.Replace(source, []byte("return issuePrivateManagedGroupingOwnedV1(ctx, m, h, q, nil)"), []byte(replacement), 1)
		if bytes.Equal(source, changed) || check(changed, binding) {
			t.Fatal("CURRENT_OWNER_MUTATION_ACCEPTED", replacement)
		}
	}
	for _, mutation := range [][2]string{{"if r == nil {", "if r != nil {"}, {"return p.bindOwnedFixed(h, source)", "return p.bindOwnedFixed(h, other)"}, {"return p.bindOwnedFixed(h, source)", "return other.bindOwnedFixed(h, source)"}} {
		changed := bytes.Replace(binding, []byte(mutation[0]), []byte(mutation[1]), 1)
		if bytes.Equal(binding, changed) || check(source, changed) {
			t.Fatal("CURRENT_BINDING_MUTATION_ACCEPTED", mutation)
		}
	}
}
