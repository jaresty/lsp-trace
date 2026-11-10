package inventorysuccessor

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"testing"
	"time"
)

func TestPrepareDescribeV1Refusals(t *testing.T) {
	for _, fault := range []string{"nil-context", "zero", "expired"} {
		t.Run(fault, func(t *testing.T) {
			q := PrepareDescribeRequestV1{}
			ctx := context.Background()
			if fault == "nil-context" {
				ctx = nil
			}
			if fault == "expired" {
				q.Persist.Deadline = time.Now().Add(-time.Second)
			}
			p, e := PrepareDescribeV1(ctx, q)
			if e == nil || len(p.Packet) != 0 || p.Continuation != (PreparedDescribeAnchorsV1{}) {
				t.Fatal("ASSERT_PREPARE_PREFLIGHT_ZERO")
			}
		})
	}
}
func TestPrepareDescribeV1PaidReadReject(t *testing.T) {
	f := preparedDescribeFixtureV1(t, t.TempDir())
	p, _ := preparedDescribeFixtureCommitV1(t, f)
	before, e := f.owner.k.reopen(context.Background(), f.owner.job, f.owner.limits)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := f.owner.preparedDescribePacketV1(context.Background(), p, hashLabelV2("unit2-rejected-encoding"), time.Now().Add(10*time.Second), 1)
	after, reopenErr := f.owner.k.reopen(context.Background(), f.owner.job, f.owner.limits)
	if e == nil || raw != nil || reopenErr != nil || after.Windows != before.Windows+1 || after.Charged == before.Charged || after.Charged.LogicalBytes <= before.Charged.LogicalBytes {
		t.Fatal("ASSERT_PREPARE_REJECTED_READ_PAID", e, reopenErr)
	}
	if f.owner.k.root.ValidatePrivate() != nil {
		t.Fatal("borrowed root closed")
	}
	t.Log("ASSERT_PREPARE_REJECTED_READ_PAID: PASS")
}

// Bounded structural assertion: the new composition's direct effects remain
// exactly these native operations. This is not a transitive CALLS inference;
// the separately HEAD-locked helpers retain their existing contracts.
func TestPrepareDescribeV1Effects(t *testing.T) {
	raw, e := os.ReadFile("private_prepare_describe_v1.go")
	if e != nil {
		t.Fatal(e)
	}
	f, e := parser.ParseFile(token.NewFileSet(), "composition.go", raw, 0)
	if e != nil {
		t.Fatal(e)
	}
	imports := map[string]bool{`"context"`: true, `"time"`: true, `"lsp-trace/internal/programcadmission"`: true}
	for _, i := range f.Imports {
		if !imports[i.Path.Value] {
			t.Fatal("ASSERT_PREPARE_NATIVE_EFFECTS", i.Path.Value)
		}
	}
	calls := map[string]bool{}
	for _, n := range []string{"fail", "cancel", "len", "hashV2", "preparedDescribeRequestV1", "preparedDescribeTextV1", "ExecuteMixedInputV2", "ExecuteRPreparationV1", "CommitPreparedDescribeV1", "Now", "Err", "Equal", "Lock", "Unlock", "Path", "WithDeadline", "ReserveMixedInputV2", "PreparationHeadV1", "Workspace", "String", "ReserveRPreparationV1", "ReservePreparedDescribeV1", "preparedDescribePacketV1", "Before", "Anchors"} {
		calls[n] = true
	}
	fields := map[string]bool{}
	for _, n := range []string{"Owner", "Capture", "CaptureAttempt", "PreparationAttempt", "PacketAttempt", "RunID", "SeedNamespace", "Seed", "Persist", "PacketLimit"} {
		fields[n] = true
	}
	ast.Inspect(f, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok || ts.Name.Name != "PrepareDescribeRequestV1" {
			return true
		}
		s, ok := ts.Type.(*ast.StructType)
		if !ok {
			t.Error("ASSERT_PREPARE_NATIVE_EFFECTS shape")
			return false
		}
		for _, f := range s.Fields.List {
			for _, n := range f.Names {
				if !fields[n.Name] {
					t.Error("ASSERT_PREPARE_NATIVE_EFFECTS control", n.Name)
				}
			}
		}
		return false
	})
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "PrepareDescribeV1" {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := ""
			switch v := call.Fun.(type) {
			case *ast.Ident:
				name = v.Name
			case *ast.SelectorExpr:
				name = v.Sel.Name
			}
			if !calls[name] {
				t.Error("ASSERT_PREPARE_NATIVE_EFFECTS", name)
			}
			return true
		})
	}
}
func TestPrepareDescribeV1Quote(t *testing.T) {
	c, e := preparedDescribeChargeV1(preparedDescribeRecordMaxV1, time.Now(), time.Now().Add(time.Second))
	if e != nil {
		t.Fatal(e)
	}
	for i, n := range c.vector() {
		switch i {
		case 5, 6, 10, 11, 12:
			if n != 0 {
				t.Fatal("provider credit")
			}
		default:
			if n == 0 {
				t.Fatal("unfunded coordinate", i)
			}
		}
	}
	if c.LogicalBytes < 20*(64<<20) {
		t.Fatal("rejecting snapshot unquoted")
	}
}
