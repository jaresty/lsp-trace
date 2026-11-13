package inventorytracediag

import (
	"bytes"
	"go/ast"
	"os"
	"testing"
)

var phase14Guards = []string{"err != nil", "!ok", "err != nil", "len(out.Predecessors)+1 > 8", "uint64(len(p.Bytes)) > (1<<20)-total", "!ok || total > 1<<20 || bound > (1<<20)-total || 65536 > (1<<20)-total-bound"}

func phase14Accepted(src []byte) bool {
	g, c, e := phase14Decisions(src)
	if e != nil || len(g) != 6 || len(c) != 6 {
		return false
	}
	for i := range c {
		if c[i] != phase14Codes[i] || g[i] != phase14Guards[i] {
			return false
		}
	}
	return true
}
func TestPhase14BracketDiscriminators(t *testing.T) {
	disabled(t)
	src, e := os.ReadFile("../inventorysuccessor/private_managed_input_operation_v2.go")
	if e != nil {
		t.Fatal(e)
	}
	for _, code := range []string{"ProjectionAttempt", "BuildInputAttempt"} {
		t.Run(code, func(t *testing.T) {
			needle := []byte("inventorytracediag.Event(context.Background(), inventorytracediag." + code + ")")
			if bytes.Count(src, needle) != 1 {
				t.Fatal("PHASE14_BRACKET_SETUP")
			}
			changed := bytes.Replace(src, needle, []byte("_ = inventorytracediag."+code), 1)
			if phase14Accepted(changed) {
				t.Fatal("PHASE14_SELECTOR_BRACKET_ACCEPTED")
			}
		})
	}
}
func TestPhase14AssociationPerturbations(t *testing.T) {
	disabled(t)
	src, e := os.ReadFile("../inventorysuccessor/private_managed_input_operation_v2.go")
	if e != nil {
		t.Fatal(e)
	}
	if !phase14Accepted(src) {
		t.Fatal("PHASE14_CURRENT")
	}
	for i, code := range phase14Codes {
		t.Run(code, func(t *testing.T) {
			needle := []byte("inventorytracediag.Event(context.Background(), inventorytracediag." + code + ")")
			if bytes.Count(src, needle) != 1 {
				t.Fatal("PHASE14_ASSOCIATION_SETUP")
			}
			for name, replacement := range map[string][]byte{"removed": nil, "swapped": []byte("inventorytracediag.Event(context.Background(), inventorytracediag." + phase14Codes[(i+1)%6] + ")"), "detached": []byte("inventorytracediag.Event(context.Background(), inventorytracediag." + code + "); nativeWork23()")} {
				if phase14Accepted(bytes.Replace(src, needle, replacement, 1)) {
					t.Errorf("PHASE14_%s_ACCEPTED %s", name, code)
				}
			}
		})
	}
}

// The owner returns the clone and admitted bound only after preflight. Require
// the actual three-statement dominance seam, not a retained-length proxy.
func assemblyOwnerSeam(f *ast.File) bool {
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "privateGroupingAssemble" {
			continue
		}
		clones, budgets, found := 0, 0, 0
		ast.Inspect(fn, func(n ast.Node) bool {
			if s, ok := n.(*ast.SelectorExpr); ok && selector(s, "projection", "CompositeSourceBinding") {
				clones++
			}
			if c, ok := n.(*ast.CallExpr); ok && selector(c.Fun, "projection", "PrivateCompositeSourceBindingBudget") {
				budgets++
			}
			b, ok := n.(*ast.IfStmt)
			if !ok || b.Init == nil || render(b.Init) != "retained := projection.RetainedCallsCanonicalBytes()" || render(b.Cond) != "len(retained) > 0" || b.Else != nil {
				return true
			}
			if len(b.Body.List) < 3 || render(b.Body.List[0]) != "sourceBinding, bindingBound, ok := projection.PrivateCompositeSourceBindingBudget()" {
				return true
			}
			refusal, ok := b.Body.List[1].(*ast.IfStmt)
			if !ok || refusal.Init != nil || refusal.Else != nil || render(refusal.Cond) != "!ok" || len(refusal.Body.List) == 0 {
				return true
			}
			if _, ok := refusal.Body.List[len(refusal.Body.List)-1].(*ast.ReturnStmt); !ok {
				return true
			}
			for _, st := range refusal.Body.List[:len(refusal.Body.List)-1] {
				if !diagnosticSyntax(st) {
					return true
				}
			}
			if render(b.Body.List[2]) != "binding, err := privateGroupingJSON(sourceBinding, bindingBound)" {
				return true
			}
			found++
			return true
		})
		return clones == 0 && budgets == 1 && found == 1
	}
	return false
}
func TestAssemblyOwnerSeamDiscriminators(t *testing.T) {
	disabled(t)
	prefix := `package p;func privateGroupingAssemble(){ if retained := projection.RetainedCallsCanonicalBytes(); len(retained)>0 { `
	good := `sourceBinding, bindingBound, ok := projection.PrivateCompositeSourceBindingBudget(); if !ok { inventorytracediag.Event(context.Background(),inventorytracediag.CompositeBindingBoundRefused);return out, err }; binding,err := privateGroupingJSON(sourceBinding,bindingBound); _ = binding; _ = err`
	cases := []struct {
		name, body string
		want       bool
	}{
		{"good", good, true},
		{"oldheuristic", `if len(retained)>1000000{return out,err};binding,err:=privateGroupingJSON(projection.CompositeSourceBinding(),1000000);_=binding;_=err`, false},
		{"ignoredok", `sourceBinding,bindingBound,ok:=projection.PrivateCompositeSourceBindingBudget();_=ok;binding,err:=privateGroupingJSON(sourceBinding,bindingBound);_=binding;_=err`, false},
		{"earlyserialize", `binding,err:=privateGroupingJSON(sourceBinding,bindingBound);sourceBinding,bindingBound,ok:=projection.PrivateCompositeSourceBindingBudget();if !ok{return out,err};_=binding;_=err`, false},
		{"preclone", `_=projection.CompositeSourceBinding();` + good, false},
		{"refusalclone", `sourceBinding,bindingBound,ok:=projection.PrivateCompositeSourceBindingBudget();if !ok{_=projection.CompositeSourceBinding();return out,err};binding,err:=privateGroupingJSON(sourceBinding,bindingBound);_=binding;_=err`, false},
		{"wrongbound", `sourceBinding,bindingBound,ok:=projection.PrivateCompositeSourceBindingBudget();if !ok{return out,err};binding,err:=privateGroupingJSON(sourceBinding,1<<20);_=binding;_=err`, false},
	}
	for _, s := range cases {
		t.Run(s.name, func(t *testing.T) {
			f := parseCurrent(t, []byte(prefix+s.body+` } }`))
			if got := assemblyOwnerSeam(f); got != s.want {
				t.Errorf("ASSEMBLY_OWNER_SEAM %s got=%v want=%v", s.name, got, s.want)
			}
		})
	}
}
func TestAssemblyOwnerSeamCurrent(t *testing.T) {
	disabled(t)
	src, e := os.ReadFile("../inventorysuccessor/private_managed_input_operation_v2.go")
	if e != nil {
		t.Fatal(e)
	}
	if !assemblyOwnerSeam(parseCurrent(t, src)) {
		t.Fatal("ASSEMBLY_OWNER_SEAM current")
	}
}
