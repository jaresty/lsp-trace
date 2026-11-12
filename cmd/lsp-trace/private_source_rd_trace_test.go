//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"runtime/trace"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestPrivateRDDriverTraceWriterCap(t *testing.T) {
	var dst bytes.Buffer
	w := &privatePrepareTraceWriter{dst: &dst, remaining: 4}
	if n, e := w.Write([]byte("abcde")); n != 4 || e == nil || dst.String() != "abcd" || !w.overflow || w.remaining != 0 {
		t.Fatal("trace cap not enforced during write")
	}
	if n, e := w.Write([]byte("x")); n != 0 || e == nil || dst.String() != "abcd" {
		t.Fatal("trace error not latched")
	}
	dst.Reset()
	short := &privateRDDriverWriter{n: 1}
	w = &privatePrepareTraceWriter{dst: short, remaining: 4}
	if _, e := w.Write([]byte("abcd")); e == nil || w.remaining != 0 {
		t.Fatal("short prefix not consumed")
	}
	if _, e := w.Write([]byte("x")); e == nil || short.calls != 1 {
		t.Fatal("trace retry")
	}
	t.Log("ASSERT_RD_TRACE_CAP: PASS")
}
func privateRDDriverTraceFD(t *testing.T) (*os.File, int) {
	t.Helper()
	f, e := os.CreateTemp(t.TempDir(), "trace")
	if e != nil {
		t.Fatal(e)
	}
	if e = f.Chmod(0600); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = f.Close() })
	fd, e := syscall.Dup(int(f.Fd()))
	if e != nil {
		t.Fatal(e)
	}
	return f, fd
}
func TestPrivateRDDriverTraceFD(t *testing.T) {
	for _, raw := range []string{"0", "1", "2", "03", "-3", "", "3x"} {
		if privatePrepareTraceFile(raw, io.Discard, io.Discard) != nil {
			t.Fatal("noncanonical FD accepted")
		}
	}
	f, fd := privateRDDriverTraceFD(t)
	if privatePrepareTraceFile(strconv.Itoa(fd), f, io.Discard) != nil {
		t.Fatal("aliased output accepted")
	}
	_, fd = privateRDDriverTraceFD(t)
	admitted := privatePrepareTraceFile(strconv.Itoa(fd), io.Discard, io.Discard)
	if admitted == nil {
		t.Fatal("valid trace descriptor refused")
	}
	_ = admitted.Close()
	f, fd = privateRDDriverTraceFD(t)
	if _, e := f.Write([]byte("x")); e != nil {
		t.Fatal(e)
	}
	if privatePrepareTraceFile(strconv.Itoa(fd), io.Discard, io.Discard) != nil {
		t.Fatal("nonempty trace accepted")
	}
	t.Log("ASSERT_RD_TRACE_FD: PASS")
}
func TestPrivateRDDriverTraceStartCollision(t *testing.T) {
	// Synthetic runtime tracer only: no CLI, manager, owner, provider or capture.
	if e := trace.Start(io.Discard); e != nil {
		t.Fatal(e)
	}
	defer trace.Stop()
	_, fd := privateRDDriverTraceFD(t)
	f := privatePrepareTraceFile(strconv.Itoa(fd), io.Discard, io.Discard)
	if f == nil {
		t.Fatal("trace FD")
	}
	if _, ok := privateRDStartTrace(f); ok {
		t.Fatal("collision admitted")
	}
	if !trace.IsEnabled() {
		t.Fatal("Start refusal stopped somebody else's tracer")
	}
	if _, e := f.Stat(); e == nil {
		t.Fatal("refused owned file left open")
	}
	t.Log("ASSERT_RD_TRACE_START_COLLISION: PASS")
}
func TestPrivateRDDriverTraceSyntheticStop(t *testing.T) {
	_, fd := privateRDDriverTraceFD(t)
	f := privatePrepareTraceFile(strconv.Itoa(fd), io.Discard, io.Discard)
	if f == nil {
		t.Fatal("trace FD")
	}
	tracing, ok := privateRDStartTrace(f)
	if !ok {
		t.Fatal("synthetic Start")
	}
	trace.Log(context.Background(), "private.prepare.cli", "TRACE_START")
	if tracing.stop() != privateRDTraceComplete {
		t.Fatal("synthetic trace incomplete")
	}
	if tracing.stop() != privateRDTraceInvalid {
		t.Fatal("second Stop admitted")
	}
	if _, e := f.Stat(); e == nil {
		t.Fatal("owned descriptor not closed")
	}
	t.Log("ASSERT_RD_TRACE_STOP: PASS")
}
func TestPrivateRDDriverTraceCloseFailure(t *testing.T) {
	_, fd := privateRDDriverTraceFD(t)
	f := privatePrepareTraceFile(strconv.Itoa(fd), io.Discard, io.Discard)
	if f == nil {
		t.Fatal("trace FD")
	}
	tracing, ok := privateRDStartTrace(f)
	if !ok {
		t.Fatal("synthetic Start")
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if tracing.stop() != privateRDTraceInvalid {
		t.Fatal("closed trace reported complete")
	}
	t.Log("ASSERT_RD_TRACE_CLOSE_FAILURE: PASS (synthetic only)")
}

func TestPrivateRDDriverCanceledReportZero(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w := &privateRDDriverWriter{n: -1}
	var b privateRDBudget
	out := privateRDOutcome{native: privateRDNativeAvailable, qualified: true, cleanup: true, deadline: time.Now().Add(time.Second)}
	if privateRDFinishReport(ctx, w, &b, out, privateRDTraceComplete) == 0 || w.calls != 0 {
		t.Fatal("canceled report")
	}
	out.deadline = time.Now().Add(-time.Second)
	if privateRDFinishReport(context.Background(), w, &b, out, privateRDTraceComplete) == 0 || w.calls != 0 {
		t.Fatal("expired report")
	}
	if out.native != privateRDNativeAvailable {
		t.Fatal("native outcome rewritten")
	}
	t.Log("ASSERT_RD_REPORT_ZERO_AFTER_CANCEL_DEADLINE: PASS")
}

// Source-contract guard, NOT native execution. Only the explicitly permitted
// exact implementation files are parsed. It pins the emission sites and fixed
// finite loop; semantic custody still relies on the inspected SDK, not this AST.
func TestPrivateRDDriverTraceSourceBound(t *testing.T) {
	root := filepath.Join("..", "..")
	readFuncs := func(rel string) map[string]*ast.FuncDecl {
		t.Helper()
		f, e := parser.ParseFile(token.NewFileSet(), filepath.Join(root, rel), nil, 0)
		if e != nil {
			t.Fatal(e)
		}
		out := map[string]*ast.FuncDecl{}
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok {
				out[fn.Name.Name] = fn
			}
		}
		return out
	}
	calls := func(fn *ast.FuncDecl, name string) int {
		t.Helper()
		if fn == nil {
			t.Fatal("missing pinned function")
		}
		n := 0
		ast.Inspect(fn.Body, func(nod ast.Node) bool {
			if c, ok := nod.(*ast.CallExpr); ok {
				switch f := c.Fun.(type) {
				case *ast.Ident:
					if f.Name == name {
						n++
					}
				case *ast.SelectorExpr:
					if f.Sel.Name == name {
						n++
					}
				}
			}
			return true
		})
		return n
	}
	emit := readFuncs("sessionruntime/private_prepare_trace.go")
	if calls(emit["privatePrepareTraceLeaseCancel"], "Log") != 2 || calls(emit["privatePrepareTraceTransaction"], "Log") != 4 || calls(emit["privatePrepareTracePresence"], "Log") != 4 || calls(emit["privatePrepareTraceMessages"], "Log") != 1 {
		t.Fatal("emitter layout changed")
	}
	// Presence contains four syntactic Log sites, but exactly two executed branches.
	runtime := readFuncs("sessionruntime/sessionruntime.go")
	if calls(runtime["finishRoundTrip"], "privatePrepareTraceLeaseCancel") != 1 || calls(runtime["finishRoundTrip"], "invalidateSessionLeaseLocked") != 1 {
		t.Fatal("lease emission multiplicity changed")
	}
	cleared := false
	ast.Inspect(runtime["invalidateSessionLeaseLocked"].Body, func(n ast.Node) bool {
		a, ok := n.(*ast.AssignStmt)
		if ok && len(a.Lhs) == 2 && len(a.Rhs) == 2 {
			if s, ok := a.Lhs[1].(*ast.SelectorExpr); ok && s.Sel.Name == "leaseCancel" {
				if id, ok := a.Rhs[1].(*ast.Ident); ok && id.Name == "nil" {
					cleared = true
				}
			}
		}
		return true
	})
	if !cleared {
		t.Fatal("single lease callback not cleared")
	}
	owned := readFuncs("sessionruntime/private_grouping_owned_handoff.go")["capturePrivateGroupingOwnedObserved"]
	if calls(owned, "privatePrepareTraceTransaction") != 1 || calls(owned, "privatePrepareTraceMessages") != 1 || calls(owned, "transact") != 2 {
		t.Fatal("capture transaction sites changed")
	}
	loopBound := false
	ast.Inspect(owned.Body, func(n ast.Node) bool {
		if r, ok := n.(*ast.RangeStmt); ok {
			if lit, ok := r.X.(*ast.CompositeLit); ok && len(lit.Elts) == 2 {
				if _, ok := lit.Type.(*ast.ArrayType); ok {
					loopBound = true
				}
			}
		}
		return true
	})
	if !loopBound {
		t.Fatal("fixed two-method range missing")
	}
	if privateRDTraceAnnotations != 27 || privateRDTraceAnnotations > 32 || privateRDTraceClassifierUnits != 493 || privateRDTraceBoundaryUnits != 19 || privateRDTraceLogicalUnits != 512 {
		t.Fatal("trace bound changed")
	}
	t.Log("ASSERT_RD_TRACE_BOUND: PASS annotations=27 logical_units=512 (493 runtime + 19 wrapper; logical not CPU/heap)")
}
func TestPrivateRDDriverNoBodyOperations(t *testing.T) {
	funcs := func() map[string]*ast.FuncDecl {
		f, e := parser.ParseFile(token.NewFileSet(), "private_source_rd_prepare.go", nil, 0)
		if e != nil {
			t.Fatal(e)
		}
		m := map[string]*ast.FuncDecl{}
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok {
				m[fn.Name.Name] = fn
			}
		}
		return m
	}()
	fn := funcs["privateRDTargetData"]
	if fn == nil {
		t.Fatal("target helper")
	}
	allowed := map[*ast.SelectorExpr]bool{}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			if id, ok := c.Fun.(*ast.Ident); ok && id.Name == "len" && len(c.Args) == 1 {
				if s, ok := c.Args[0].(*ast.SelectorExpr); ok && s.Sel.Name == "Body" {
					allowed[s] = true
				}
			}
		}
		return true
	})
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if s, ok := n.(*ast.SelectorExpr); ok && s.Sel.Name == "Body" && !allowed[s] {
			t.Fatal("Body observed beyond len")
		}
		return true
	})
	if len(allowed) != 2 {
		t.Fatal("body length guard changed")
	}
	t.Log("ASSERT_RD_BODY_AST: PASS")
}
