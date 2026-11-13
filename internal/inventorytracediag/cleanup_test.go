package inventorytracediag

import (
	"bytes"
	"go/ast"
	"os"
	"testing"
)

// Accepted repair02 is Finish -> failure attribution -> abort on the SAME
// window. This checks current source placement, not native completion/leak proof.
func cleanupOrder23(f *ast.File) bool {
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "ExecuteMixedInputV2" {
			continue
		}
		found, deadlines, windows, aborts := 0, 0, 0, 0
		var finish, complete, confirmed tokenPosition23
		ast.Inspect(fn, func(n ast.Node) bool {
			if a, ok := n.(*ast.AssignStmt); ok {
				switch render(a) {
				case "execution, cancel := context.WithDeadline(ctx, q.Start.Deadline)":
					deadlines++
				case "window, err := q.Manager.PrivateProviderWindowContext(execution, s.envelope, q.Start.Deadline)":
					windows++
				case "completion, err := q.Manager.FinishPrivateProviderWindow(window)":
					finish = tokenPosition23(a.Pos())
				case "out.sourceConfirmed = true":
					confirmed = tokenPosition23(a.Pos())
				}
			}
			if c, ok := n.(*ast.CallExpr); ok && selector(c.Fun, "operation", "completeFixed") && len(c.Args) == 1 && render(c.Args[0]) == "execution" {
				complete = tokenPosition23(c.Pos())
			}
			if c, ok := n.(*ast.CallExpr); ok && selector(c.Fun, "operation", "abortFixed") {
				aborts++
			}
			ds, ok := n.(*ast.DeferStmt)
			if !ok {
				return true
			}
			lit, ok := ds.Call.Fun.(*ast.FuncLit)
			if !ok {
				return true
			}
			native := []string{}
			for _, st := range lit.Body.List {
				if diagnosticSyntax(st) {
					continue
				}
				native = append(native, render(st))
			}
			want := []string{"_, finishErr := q.Manager.FinishPrivateProviderWindow(window)", "failure = privatePrepareDeferredFinishV1(failure, finishErr)", "operation.abortFixed()"}
			if len(native) == len(want) {
				same := true
				for i := range want {
					same = same && native[i] == want[i]
				}
				if same {
					found++
				}
			}
			return true
		})
		// A second context deadline/window construction is not a renewal seam.
		callsDeadline, callsWindow := 0, 0
		ast.Inspect(fn, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok {
				if selector(c.Fun, "context", "WithDeadline") || selector(c.Fun, "context", "WithTimeout") {
					callsDeadline++
				}
				if render(c.Fun) == "q.Manager.PrivateProviderWindowContext" {
					callsWindow++
				}
			}
			return true
		})
		return found == 1 && deadlines == 1 && windows == 1 && callsDeadline == 1 && callsWindow == 1 && aborts == 1 && finish > 0 && finish < complete && complete < confirmed
	}
	return false
}

type tokenPosition23 int

func TestTraceCleanupOrderCurrent(t *testing.T) {
	disabled(t)
	src, e := os.ReadFile("../inventorysuccessor/private_mixed_input_operation_v2.go")
	if e != nil {
		t.Fatal(e)
	}
	if !cleanupOrder23(parseCurrent(t, src)) {
		t.Fatal("CURRENT_CLEANUP_ORDER")
	}
	for name, pair := range map[string][2]string{
		"abortfirst":         {"failure = privatePrepareDeferredFinishV1(failure, finishErr)", "operation.abortFixed(); failure = privatePrepareDeferredFinishV1(failure, finishErr)"},
		"attributionremoved": {"failure = privatePrepareDeferredFinishV1(failure, finishErr)", "_ = finishErr"},
		"differentwindow":    {"_, finishErr := q.Manager.FinishPrivateProviderWindow(window)", "_, finishErr := q.Manager.FinishPrivateProviderWindow(execution)"},
		"renewal":            {"_, finishErr := q.Manager.FinishPrivateProviderWindow(window)", "window, cancel := context.WithDeadline(ctx, q.Start.Deadline); _ = cancel; _, finishErr := q.Manager.FinishPrivateProviderWindow(window)"},
		"newcontext":         {"_, finishErr := q.Manager.FinishPrivateProviderWindow(window)", "window = context.Background(); _, finishErr := q.Manager.FinishPrivateProviderWindow(window)"},
		"earlyavailability":  {"out.sourceConfirmed = true", "out.sourceConfirmed = false"},
	} {
		t.Run(name, func(t *testing.T) {
			if bytes.Count(src, []byte(pair[0])) != 1 {
				t.Fatal("CURRENT_CLEANUP_SETUP")
			}
			changed := bytes.Replace(src, []byte(pair[0]), []byte(pair[1]), 1)
			if cleanupOrder23(parseCurrent(t, changed)) {
				t.Errorf("CURRENT_CLEANUP_ACCEPTED %s", name)
			}
		})
	}
}
