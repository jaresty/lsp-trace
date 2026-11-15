package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"
	"unsafe"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
)

// Inert transport: no Starter, process, Manager.Start, READY, kernel, grants,
// reservations, native fixtures, source opens or captured historical authority.
type discoveryInertChild struct {
	in    io.ReadCloser
	reads int
}

func (c *discoveryInertChild) Stdin() io.WriteCloser { return discoveryDiscard{} }
func (c *discoveryInertChild) Stdout() io.ReadCloser { return discoveryInertReader{c} }
func (c *discoveryInertChild) Read(p []byte) (int, error) {
	n, e := c.in.Read(p)
	c.reads += n
	return n, e
}
func (c *discoveryInertChild) Close() managedprocess.ResourceObservation {
	return managedprocess.ResourceObservation{}
}
func (c *discoveryInertChild) Teardown(context.Context) managedprocess.TeardownObservation {
	return managedprocess.TeardownObservation{}
}

// readCloser wrapper avoids the Child.Close signature conflict.
type discoveryInertReader struct{ child *discoveryInertChild }

func (r discoveryInertReader) Read(p []byte) (int, error) { return r.child.Read(p) }
func (r discoveryInertReader) Close() error               { return nil }

type discoveryDiscard struct{}

func (discoveryDiscard) Write(p []byte) (int, error) { return len(p), nil }
func (discoveryDiscard) Close() error                { return nil }
func discoveryWire(body string) []byte {
	return []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body))
}
func discoveryWindow(body []byte) (*privateDiscoveryWindow, *privateDiscoveryChild, *discoveryInertChild) {
	l := PrivateDiscoveryLimits{SourceBytes: 16 * (MaxDocumentSupplyBytes + 1), RequestBytes: 64 << 20, Requests: 1024, ResponseBytes: 64 << 20, WireBytes: 128 << 20, RetainedBytes: 512 << 20, Work: 1 << 32, Messages: 4096, FrameBytes: 1 << 20, HeaderBytes: 8192}
	w := &privateDiscoveryWindow{ctx: context.Background(), limits: l, remaining: l}
	inert := &discoveryInertChild{in: io.NopCloser(bytes.NewReader(body))}
	c := &privateDiscoveryChild{wireChild: inert, window: w}
	w.child = c
	return w, c, inert
}
func TestPrivateDiscoveryWindowCumulativeFit(t *testing.T) {
	var wire bytes.Buffer
	for i := 0; i < 65; i++ {
		wire.Write(discoveryWire(`{"jsonrpc":"2.0","id":1,"result":null}`))
	}
	w, c, _ := discoveryWindow(wire.Bytes())
	for i := 0; i < 16; i++ {
		if n := w.sourceLimit(); n != MaxDocumentSupplyBytes {
			t.Fatalf("source quote %d: %d", i, n)
		}
	}
	slots, e := w.consumerSlots()
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 65; i++ {
		f, e := w.frameRecord(c)
		if e != nil {
			t.Fatalf("source16-small65 actual window frame=%d refusal=%+v err=%v", i, w.refusal, e)
		}
		if _, e = slots.store(f, false); e != nil {
			t.Fatal(e)
		}
		raw, e := w.selectedResult(f)
		if e != nil || string(raw) != "null" {
			t.Fatal(e)
		}
	}
	if w.limits.FrameBytes != 1<<20 || w.failed {
		t.Fatal("cap changed or failed")
	}
	t.Logf("source16-small65 actual production remaining retained=%d work=%d", w.remaining.RetainedBytes, w.remaining.Work)
}
func TestPrivateDiscoveryRecordAdmission(t *testing.T) {
	wire := discoveryWire(`{"jsonrpc":"2.0","id":1,"result":null}`)
	var readerCost uint64
	_, _, _, _, e := lspwire.ReadPrivateDiscoveryFrame(context.Background(), bytes.NewReader(wire), lspwire.Limits{MaxBodyBytes: 1 << 20, MaxHeaderBytes: 8192}, 1<<20, func(q lspwire.PrivateDiscoveryCharge) error { readerCost += q.Retained; return nil })
	if e != nil {
		t.Fatal(e)
	}
	w, c, inert := discoveryWindow(wire)
	k := uint64(unsafe.Sizeof(privateDiscoveryFrame{}))
	w.remaining.RetainedBytes = readerCost + k - 1
	f, e := w.frameRecord(c)
	if e == nil || f != nil || inert.reads != len(wire) || w.remaining.RetainedBytes != k-1 || w.refusal.Requested != k {
		t.Fatalf("record admission f=%p read=%d remaining=%d diagnostic=%+v err=%v", f, inert.reads, w.remaining.RetainedBytes, w.refusal, e)
	}
}

func TestPrivateDiscoveryConsumerBoundaries(t *testing.T) {
	w, c, inert := discoveryWindow(discoveryWire(`{"jsonrpc":"2.0","id":1,"result":[1]}`))
	w.remaining.Messages = 1
	slots, e := w.consumerSlots()
	if e != nil {
		t.Fatal(e)
	}
	f, e := w.frameRecord(c)
	if e != nil {
		t.Fatal(e)
	}
	before := w.remaining.RetainedBytes
	got, e := slots.store(f, false)
	if e != nil || len(got) != 1 || &got[0] != &slots.responses[0] || w.remaining.RetainedBytes != before {
		t.Fatal("shallow indexed store")
	}
	if _, e = slots.store(f, false); e == nil || len(slots.responses) != 1 {
		t.Fatal("full slots did not refuse")
	}
	if inert.reads == 0 {
		t.Fatal("not actual production frame")
	}
	w, c, _ = discoveryWindow(discoveryWire(`{"jsonrpc":"2.0","id":1,"result":[1]}`))
	f, e = w.frameRecord(c)
	if e != nil {
		t.Fatal(e)
	}
	w.remaining.RetainedBytes = uint64(len(f.message.Result)) - 1
	before = w.remaining.RetainedBytes
	if raw, e := w.selectedResult(f); e == nil || raw != nil || w.remaining.RetainedBytes != before {
		t.Fatal("copy refusal or atomic balance")
	}
	d := w.refusalError(PrivateDiscoveryCleanup, true)
	diag, ok := PrivateDiscoveryRefusalFromError(d)
	if !ok || !diag.CleanupUncertain || diag.Currency != PrivateDiscoveryRetainedBytes || diag.Remaining != before {
		t.Fatalf("chronology %+v", diag)
	}
	w, c, _ = discoveryWindow(discoveryWire(`{"jsonrpc":"2.0","id":1,"result":[1]}`))
	f, e = w.frameRecord(c)
	if e != nil {
		t.Fatal(e)
	}
	before = w.remaining.RetainedBytes
	raw, e := w.selectedResult(f)
	if e != nil || before-w.remaining.RetainedBytes != uint64(len(raw)) {
		t.Fatal("copy exact cost")
	}
	raw[0] = '!'
	if f.message.Result[0] != '[' {
		t.Fatal("copy aliases")
	}
	w, _, _ = discoveryWindow(nil)
	w.remaining.RetainedBytes = 0
	if slots, e := w.consumerSlots(); e == nil || slots != nil {
		t.Fatal("slot allocation before admission")
	}
	diag, ok = PrivateDiscoveryRefusalFromError(w.refusalError(PrivateDiscoveryCleanup, true))
	if !ok || diag.Remaining != 0 || !diag.AmountsAvailable {
		t.Fatalf("known zero %+v", diag)
	}
	t.Logf("sizeofMessage=%d two4096slots=%d K=%d S=%d I=%d", unsafe.Sizeof(lspwire.Message{}), 2*4096*unsafe.Sizeof(lspwire.Message{}), unsafe.Sizeof(privateDiscoveryFrame{}), unsafe.Sizeof(privateDiscoverySlots{}), unsafe.Sizeof((*privateDiscoveryFrame)(nil)))
}
func TestPrivateDiscoveryWindowReadBoundaries(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"result":null}`
	wire := discoveryWire(body)
	for _, budget := range []uint64{0, 2*8192 + 174, 2*8192 + 175} {
		w, c, inert := discoveryWindow(wire)
		w.remaining.RetainedBytes = budget
		f, e := w.frameRecord(c)
		if e == nil || f != nil {
			t.Fatal("expected admission refusal")
		}
		want := 0
		if budget == 2*8192+175 {
			want = len(wire) - len(body)
		}
		if inert.reads != want {
			t.Fatalf("budget=%d reads=%d want=%d", budget, inert.reads, want)
		}
	}
	w, c, _ := discoveryWindow(wire)
	w.remaining.ResponseBytes = 0
	if _, e := w.frameRecord(c); e == nil || w.refusal.Remaining != 0 || w.refusal.Currency != PrivateDiscoveryResponseBytes {
		t.Fatalf("zero response %+v", w.refusal)
	}
	w, c, _ = discoveryWindow(wire)
	other := &privateDiscoveryChild{wireChild: c.wireChild, window: w}
	if _, e := w.frameRecord(other); e == nil {
		t.Fatal("wrong child")
	}
	ctx, cancel := context.WithCancel(context.Background())
	w, c, inert := discoveryWindow(wire)
	w.ctx = ctx
	cancel()
	if _, e := w.frameRecord(c); e == nil || inert.reads != 0 {
		t.Fatal("cancel before read")
	}
	// Short reads spend requested bytes, including a failed read, with no refund.
	w, _, _ = discoveryWindow(nil)
	r := privateDiscoveryReader{io.NopCloser(bytes.NewReader([]byte("x"))), w}
	before := w.remaining.ResponseBytes
	var scratch [8]byte
	n, e := r.Read(scratch[:])
	if n != 1 || e != nil || before-w.remaining.ResponseBytes != 8 {
		t.Fatal("short read refunded")
	}
}
func TestPrivateDiscoveryIDs(t *testing.T) {
	for _, s := range []string{"0", "1", "18446744073709551615", "18446744073709551616", "999999999999999999999999x", "+1", "-1", "", "01", "1x", "null", "{}", "[]"} {
		got, ok := privateDiscoveryID([]byte(s))
		old, e := strconv.ParseUint(s, 10, 64)
		if got != old || ok != (e == nil) {
			t.Fatalf("ID %q got=%d/%v old=%d/%v", s, got, ok, old, e)
		}
	}
}
func TestPrivateDiscoveryShutdownInert(t *testing.T) {
	for iteration := 0; iteration < 2; iteration++ {
		wire := append(discoveryWire(`{"jsonrpc":"2.0","method":"notice","params":{}}`), discoveryWire(`{"jsonrpc":"2.0","id":1,"result":null}`)...)
		w, c, inert := discoveryWindow(wire)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		w.ctx = ctx
		m := &Manager{wire: lspwire.Limits{MaxBodyBytes: 1 << 20, MaxHeaderBytes: 8192}, limits: Limits{MaxObservations: 64}}
		observed := m.gracefulShutdown(c, lspwire.NewPending(2), 1)
		cancel()
		if !observed.acknowledged || !observed.exitNotified || observed.readJoin != nil || observed.facts.ReadFrames != 2 || inert.reads != len(wire) || w.failed {
			t.Fatalf("inert shutdown iteration=%d facts=%+v failed=%v", iteration, observed.facts, w.failed)
		}
	}
}

func TestPrivateDiscoveryStructure(t *testing.T) {
	_, here, _, _ := runtime.Caller(0)
	dir := filepath.Dir(here)
	specs := []struct {
		path string
		only string
	}{{filepath.Join(dir, "../internal/lspwire/private_discovery_codec.go"), ""}, {filepath.Join(dir, "../internal/lspwire/read_frame.go"), "ReadPrivateDiscoveryFrame"}, {filepath.Join(dir, "private_discovery_consumer.go"), ""}}
	for _, spec := range specs {
		fs := token.NewFileSet()
		f, e := parser.ParseFile(fs, spec.path, nil, 0)
		if e != nil {
			t.Fatal(e)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || spec.only != "" && fn.Name.Name != spec.only {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if _, ok := n.(*ast.MapType); ok {
					t.Errorf("map in replacement %s", fn.Name.Name)
				}
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if id, ok := call.Fun.(*ast.Ident); ok {
					switch id.Name {
					case "append", "NewReader", "ReadPrivateBoundedFrameObservedAtDecode", "ReadDirectionalReserved":
						t.Errorf("opaque/growing/fallback call %s in %s", id.Name, fn.Name.Name)
					}
				}
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					if pkg, ok := sel.X.(*ast.Ident); ok {
						if pkg.Name == "fmt" || pkg.Name == "reflect" || pkg.Name == "json" {
							t.Errorf("opaque call %s.%s in %s", pkg.Name, sel.Sel.Name, fn.Name.Name)
						}
					}
				}
				return true
			})
		}
	}
	f, e := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, "sessionruntime.go"), nil, 0)
	if e != nil {
		t.Fatal(e)
	}
	counts := map[string]int{}
	ast.Inspect(f, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "canonical" {
			counts["canonical"]++
		}
		if call, ok := n.(*ast.CallExpr); ok {
			if id, ok := call.Fun.(*ast.Ident); ok {
				switch id.Name {
				case "privateDiscoveryID", "privateDiscoveryIDEqual":
					counts[id.Name]++
				}
			}
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if ok {
			switch sel.Sel.Name {
			case "frameRecord", "store", "selectedResult":
				counts[sel.Sel.Name]++
			}
		}
		return true
	})
	if counts["frameRecord"] != 3 || counts["store"] != 3 || counts["selectedResult"] != 1 || counts["canonical"] != 2 || counts["privateDiscoveryID"] != 1 || counts["privateDiscoveryIDEqual"] != 2 {
		t.Fatalf("concrete integration calls: %+v", counts)
	}
	t.Log("replacement AST: no append/map/generic-json/reflection/fmt; runtime frameRecord=3 fixed-store=3 selected-copy=1")
}

func TestPrivateDiscoveryOwnedRecordAndNil(t *testing.T) {
	w, c, _ := discoveryWindow(discoveryWire(`{"jsonrpc":"2.0","id":1,"error":{"code":-1,"message":"x"}}`))
	f, e := w.frameRecord(c)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := w.selectedResult(f)
	if raw != nil || e != nil {
		t.Fatal("nil result not preserved")
	}
	f.owner = nil
	if _, e = w.selectedResult(f); !errors.Is(e, ErrPrivateDiscovery) {
		t.Fatal("forged owner")
	}
	var _ json.RawMessage = raw
}
