//go:build darwin || linux

package sourceoperator

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/describerequest"
	inv "lsp-trace/internal/inventorysuccessor"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	"lsp-trace/internal/publication"
)

func TestSourceOperatorOriginalRecordJoin(t *testing.T) {
	// DTO-only supplement: genuine paid predicate/admission controls live in
	// TestSuppliedOpPreparationControls, and actual mailbox export in
	// TestSourceOperatorOriginalQueryTarget. This function cannot mint custody.
	original := describerequest.Record{RecordID: "original-record", Envelope: describerequest.Envelope{AdmissionID: "original"}, Lineage: describerequest.Lineage{PacketID: "original", CensusID: "original"}}
	other := original
	other.RecordID = "other-record"
	other.Envelope.AdmissionID = "other"
	other.Lineage.PacketID = "other"
	other.Lineage.CensusID = "other"
	for _, records := range [][]describerequest.Record{{other, original}, {original, other}} {
		r, e := originalQueryRecordForID("original", records)
		if e != nil || r != original {
			t.Fatal("ASSERT_ORIGINAL_RECORD_ORDER_INDEPENDENT", e)
		}
	}
	for name, records := range map[string][]describerequest.Record{"missing": {other}, "duplicate": {original, original}, "empty": nil} {
		if r, e := originalQueryRecordForID("original", records); e == nil || r.RecordID != "" {
			t.Fatal("ASSERT_ORIGINAL_RECORD_JOIN_REFUSED", name)
		}
	}
	for _, change := range []func(*describerequest.Record){func(r *describerequest.Record) { r.Lineage.PacketID = "other" }, func(r *describerequest.Record) { r.Lineage.CensusID = "other" }, func(r *describerequest.Record) { r.RecordID = "" }, func(r *describerequest.Record) { r.Envelope.AdmissionID = "other" }} {
		r := original
		change(&r)
		if got, e := originalQueryRecordForID("original", []describerequest.Record{r}); e == nil || got.RecordID != "" {
			t.Fatal("ASSERT_ORIGINAL_RECORD_JOIN_REFUSED")
		}
	}
	if _, e := originalQueryRecord(inv.PreparedSourceRV1{}, &sc.PrivateAllModeSourcesResult{Requests: []describerequest.Record{original}}); e == nil {
		t.Fatal("ASSERT_ORIGINAL_ZERO_CUSTODY_REFUSED")
	}
	raw, e := os.ReadFile("run.go")
	if e != nil {
		t.Fatal(e)
	}
	s := string(raw)
	selection, prepare := strings.Index(s, "record, e := originalQueryRecord(prepared, source)"), strings.Index(s, "sc.PrepareSourceHost")
	if selection < 0 || prepare <= selection || strings.Contains(s, "record := source.Requests[0]") {
		t.Fatal("ASSERT_ORIGINAL_ACTUAL_SELECTION_BEFORE_HOST")
	}
	t.Log("ASSERT_ORIGINAL_RECORD_ORDER_INDEPENDENT: PASS; ASSERT_ORIGINAL_RECORD_JOIN_REFUSED: PASS; ASSERT_ORIGINAL_ZERO_CUSTODY_REFUSED: PASS; ASSERT_ORIGINAL_ACTUAL_SELECTION_BEFORE_HOST: PASS (DTO/finite source supplement)")
}

func TestSourceOperatorZeroCustody(t *testing.T) {
	if out, e := Run(context.Background(), Request{}); e == nil || out.NativeID != "" {
		t.Fatal("ASSERT_OPERATOR_ZERO_REFUSED")
	}
	if _, e := (*publicationOwner)(nil).sealNative(sc.DeliveredSourceResponse{}, nil); e == nil {
		t.Fatal("ASSERT_NATIVE_DTO_CANNOT_MINT")
	}
}
func TestSourceOperatorHostPhaseAggregate(t *testing.T) {
	// Independent native-only and unconnected HOST checks fit, yet their
	// simultaneous external retention makes the connected HOST phase fail.
	if _, _, e := nativeAggregateUpper(1204, 200, 300, 400, 500, 300100); e != nil {
		t.Fatal("ASSERT_HOST_NATIVE_ONLY_CONTROL", e)
	}
	for i := 0; i < 3; i++ {
		v := [3]uint64{1, 1, 1}
		v[i] = 300000
		p := &publicationOwner{limit: 300100, metadata: 103}
		if e := p.admitHostMetadata(101, sc.SourceHostPhaseBounds{Source: v[0], Lower: v[1], Binder: v[2]}); e == nil {
			t.Fatal("ASSERT_HOST_NATIVE_FITS_CONNECTED_REFUSES", i)
		}
	}
	t.Log("ASSERT_HOST_NATIVE_ONLY_CONTROL: PASS; ASSERT_HOST_NATIVE_FITS_CONNECTED_REFUSES: PASS")
	for i, name := range []string{"source", "lower", "binder"} {
		t.Run(name, func(t *testing.T) {
			values := [3]uint64{1, 1, 1}
			values[i] = 1000
			phases := func() sc.SourceHostPhaseBounds {
				return sc.SourceHostPhaseBounds{Source: values[0], Lower: values[1], Binder: values[2]}
			}
			p := &publicationOwner{limit: 1204, metadata: 103}
			if e := p.admitHostMetadata(101, phases()); e != nil {
				t.Fatal("ASSERT_HOST_PHASE_EQUALITY", e)
			}
			values[i]++
			if e := p.admitHostMetadata(101, phases()); e == nil {
				t.Fatal("ASSERT_HOST_PHASE_ONE_OVER")
			}
			values[i]--
			if e := p.admitHostMetadata(102, phases()); e == nil {
				t.Fatal("ASSERT_HOST_PREPARED_ONE_OVER")
			}
			p.metadata++
			if e := p.admitHostMetadata(101, phases()); e == nil {
				t.Fatal("ASSERT_HOST_METADATA_ONE_OVER")
			}
			p.metadata--
			p.limit = ^uint64(0)
			values[i] = ^uint64(0)
			if e := p.admitHostMetadata(101, phases()); e == nil {
				t.Fatal("ASSERT_HOST_PHASE_OVERFLOW")
			}
			values[i] = ^uint64(0) - 101
			if e := p.admitHostMetadata(101, phases()); e == nil {
				t.Fatal("ASSERT_HOST_METADATA_OVERFLOW")
			}
			values[i] = 0
			if e := p.admitHostMetadata(101, phases()); e == nil {
				t.Fatal("ASSERT_HOST_MISSING_PHASE")
			}
			t.Log("ASSERT_HOST_PHASE_EQUALITY: PASS; ASSERT_HOST_PHASE_ONE_OVER: PASS; ASSERT_HOST_PREPARED_ONE_OVER: PASS; ASSERT_HOST_METADATA_ONE_OVER: PASS; ASSERT_HOST_PHASE_OVERFLOW: PASS; ASSERT_HOST_METADATA_OVERFLOW: PASS; ASSERT_HOST_MISSING_PHASE: PASS")
		})
	}
	for _, p := range []*publicationOwner{nil, {}, {limit: 1204}, {metadata: 103}} {
		if e := p.admitHostMetadata(101, sc.SourceHostPhaseBounds{Source: 1, Lower: 1, Binder: 1}); e == nil {
			t.Fatal("ASSERT_HOST_MISSING_OWNER")
		}
	}
	if e := (&publicationOwner{limit: 1204, metadata: 103}).admitHostMetadata(0, sc.SourceHostPhaseBounds{Source: 1, Lower: 1, Binder: 1}); e == nil {
		t.Fatal("ASSERT_HOST_MISSING_PREPARATION")
	}
}

func TestSourceOperatorHostPhaseBinding(t *testing.T) {
	raw, e := os.ReadFile("run.go")
	if e != nil {
		t.Fatal(e)
	}
	s := string(raw)
	prepare, complete := strings.Index(s, "sc.PrepareSourceHost"), strings.Index(s, "sc.CompleteSourceHost")
	bounds1, bounds2 := strings.Index(s, "source.HostPhaseBounds"), strings.LastIndex(s, "source.HostPhaseBounds")
	guard1, guard2 := strings.Index(s, "roots.admitHostMetadata(preparedUpper, phases)"), strings.LastIndex(s, "roots.admitHostMetadata(preparedUpper, phases)")
	if bounds1 < 0 || guard1 <= bounds1 || prepare <= guard1 || bounds2 <= prepare || guard2 <= bounds2 || complete <= guard2 {
		t.Fatal("ASSERT_HOST_PHASE_BOTH_EARLY_GUARDS")
	}
	// Finite implementation supplement: the genuine opaque fixture separately
	// exercises this method; this checks its exact cached terms and forbids work.
	raw, e = os.ReadFile("../sourcecontinuation/host_handoff.go")
	if e != nil {
		t.Fatal(e)
	}
	s = string(raw)
	start, end := strings.Index(s, "func (out *PrivateAllModeSourcesResult) HostPhaseBounds("), strings.Index(s, "func (r DeliveredSourceResponse) Supplied()")
	if start < 0 || end <= start {
		t.Fatal("ASSERT_HOST_PHASE_CACHED_BINDING")
	}
	s = s[start:end]
	for _, term := range []string{"sourceHostPreflight(ctx, c, q, limits)", "sourceResponseAdd(extra, store.retained.Load())", "s.handoff.RetainedPayloadUpper()", "sourceAcquisitionEnvelopePeak(handoff, s.regeneration.source, s.regeneration.source, extra, raw, uint64(parent))", "sourceAcquisitionEnvelopePeak(handoff, s.regeneration.lower, s.regeneration.lower, extra, raw, uint64(parent))", "sourceResponseAdd(handoff, s.regeneration.response, extra)", "raw := uint64(c.Mailbox.Limits.MaxResponseBytes)", "sourceResponseAggregateBound(retained, record, metadata, raw,", "Binder: b.peak"} {
		if !strings.Contains(s, term) {
			t.Fatal("ASSERT_HOST_PHASE_CACHED_BINDING", term)
		}
	}
	for _, work := range []string{".Bytes(", ".Get(", ".Put(", ".Semantic(", "Marshal(", "Sum256(", "sourceResponseJoin(", "Resume("} {
		if strings.Contains(s, work) {
			t.Fatal("ASSERT_HOST_PHASE_METADATA_ONLY", work)
		}
	}
	t.Log("ASSERT_HOST_PHASE_BOTH_EARLY_GUARDS: PASS; ASSERT_HOST_PHASE_CACHED_BINDING: PASS; ASSERT_HOST_PHASE_METADATA_ONLY: PASS (finite source supplement)")
}

func TestSourceOperatorNativeAggregate(t *testing.T) {
	w, peak, e := nativeAggregateUpper(1000, 200, 300, 400, 500, ^uint64(0))
	if e != nil || w != 29080 || peak != 282000 {
		t.Fatal("ASSERT_NATIVE_PEAK_FORMULA", w, peak, e)
	}
	if _, _, e = nativeAggregateUpper(1000, 200, 300, 400, 500, peak); e != nil {
		t.Fatal("ASSERT_NATIVE_EXACT_FIT", e)
	}
	if _, _, e = nativeAggregateUpper(1001, 200, 300, 400, 500, peak); e == nil {
		t.Fatal("ASSERT_NATIVE_RETAINED_ONE_OVER")
	}
	if _, _, e = nativeAggregateUpper(1000, 201, 300, 400, 500, peak); e == nil {
		t.Fatal("ASSERT_NATIVE_PEAK_ONE_OVER")
	}
	if _, _, e = nativeAggregateUpper(^uint64(0), 200, 300, 400, 500, ^uint64(0)); e == nil {
		t.Fatal("ASSERT_NATIVE_CHECKED_OVERFLOW")
	}
	if _, _, e = nativeAggregateUpper(1000, 200, 300, ^uint64(0), 500, ^uint64(0)); e == nil {
		t.Fatal("ASSERT_NATIVE_CHECKED_OVERFLOW")
	}
	t.Log("ASSERT_NATIVE_PEAK_FORMULA: PASS; ASSERT_NATIVE_EXACT_FIT: PASS; ASSERT_NATIVE_RETAINED_ONE_OVER: PASS; ASSERT_NATIVE_PEAK_ONE_OVER: PASS; ASSERT_NATIVE_CHECKED_OVERFLOW: PASS")
}

// Fixed scalars make the 8-MiB boundary independent of checkout/path metadata.
func TestSourceOperatorNativeAggregateEightMiBBoundary(t *testing.T) {
	const limit uint64 = 8 << 20
	const supplied, delivery, semantic, gaps uint64 = 200, 300, 400, 500
	// Independent expansion, not a peak returned by the production helper.
	const overhead = 10*(supplied+delivery) + 168*semantic + 8*gaps + 204800
	const upstream = limit - overhead
	if nativeMaxBytes != limit {
		t.Fatal("ASSERT_NATIVE_EIGHT_MIB_UNCHANGED", nativeMaxBytes)
	}
	wire, peak, err := nativeAggregateUpper(upstream, supplied, delivery, semantic, gaps, limit)
	if err != nil || peak != limit || wire != 20480+supplied+delivery+19*semantic+gaps {
		t.Fatal("ASSERT_NATIVE_EIGHT_MIB_EXACT_FIT", wire, peak, err)
	}
	if _, _, err = nativeAggregateUpper(upstream+1, supplied, delivery, semantic, gaps, limit); !errors.Is(err, ErrNativeAggregate) {
		t.Fatal("ASSERT_NATIVE_EIGHT_MIB_ONE_OVER", err)
	}
	t.Log("ASSERT_NATIVE_EIGHT_MIB_UNCHANGED: PASS; ASSERT_NATIVE_EIGHT_MIB_EXACT_FIT: PASS; ASSERT_NATIVE_EIGHT_MIB_ONE_OVER: PASS")
}

func TestSourceOperatorNativeAggregateSemanticFixtureReduction(t *testing.T) {
	const upstream, delivery, gaps uint64 = 3100000, 300, 500
	const adapter uint64 = 8192
	var peaks [2]uint64
	for i, ceiling := range []uint64{4096, 2048} {
		// Independent response-envelope model: S=6*min(raw,semanticLimit)+2048.
		// Non-semantic supplied terms are fixed; only its one S term changes.
		semantic := 6*min(adapter, ceiling) + 2048
		supplied := 10000 + semantic
		wantWire := 20480 + supplied + delivery + 19*semantic + gaps
		wantPeak := upstream + 10*(supplied+delivery) + 168*semantic + 8*gaps + 204800
		wire, peak, err := nativeAggregateUpper(upstream, supplied, delivery, semantic, gaps, ^uint64(0))
		if err != nil || wire != wantWire || peak != wantPeak {
			t.Fatal("ASSERT_NATIVE_FIXTURE_INDEPENDENT_FORMULA", ceiling, wire, peak, wantWire, wantPeak, err)
		}
		peaks[i] = peak
	}
	// 168 S copies plus 10 supplied copies: 178 * 12288 = 2187264.
	if peaks[0]-peaks[1] != 2187264 {
		t.Fatal("ASSERT_NATIVE_FIXTURE_CEILING_DELTA", peaks)
	}
	t.Log("ASSERT_NATIVE_FIXTURE_INDEPENDENT_FORMULA: PASS; ASSERT_NATIVE_FIXTURE_CEILING_DELTA: PASS")
}

// Err cancels only after both actual owned publication roots have closed. This
// is a test-local context seam, not a replaceable production close/clock hook.
type terminalCloseContext struct {
	context.Context
	cancel context.CancelFunc
	roots  [2]*publication.ContentRetentionRoot
}

func (c terminalCloseContext) Err() error {
	if c.roots[0].ValidatePrivate() != nil && c.roots[1].ValidatePrivate() != nil {
		c.cancel()
	}
	return c.Context.Err()
}

func TestSourceOperatorTerminalAfterClose(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := &publicationOwner{}
	home, e := os.UserHomeDir()
	if e != nil {
		t.Fatal(e)
	}
	dir, e := os.MkdirTemp(home, ".source-terminal-")
	if e != nil {
		t.Fatal(e)
	}
	defer os.RemoveAll(dir)
	for i := range p.outputs {
		path := filepath.Join(dir, fmt.Sprint(i))
		if e = os.Mkdir(path, 0700); e != nil {
			t.Fatal(e)
		}
		p.outputs[i], e = publication.OpenContentRetentionRoot(parent, path)
		if e != nil {
			t.Fatal(e)
		}
	}
	ctx := terminalCloseContext{parent, cancel, p.outputs}
	if ctx.Err() != nil {
		t.Fatal("ASSERT_TERMINAL_PRE_CLOSE_LIVE")
	}
	out := Outcome{Disposition: "SOURCE_PROVISIONAL_DELIVERED_PAIR", Published: [2]bool{true, true}, Readback: [2]bool{true, false}}
	closeErr := p.close()
	if e := finalizeTerminal(ctx, time.Now().Add(time.Second), &out, closeErr); !errors.Is(e, context.Canceled) || out.Disposition != "FAILED_RETAINED" || out.Published != [2]bool{true, true} || out.Readback != [2]bool{true, false} {
		t.Fatal("ASSERT_TERMINAL_POST_CLOSE_CANCEL", out, e)
	}
	out.Disposition = "SOURCE_PROVISIONAL_DELIVERED_PAIR"
	if e := finalizeTerminal(context.Background(), time.Now().Add(-time.Second), &out, nil); !errors.Is(e, context.DeadlineExceeded) || out.Disposition != "FAILED_RETAINED" {
		t.Fatal("ASSERT_TERMINAL_ORIGINAL_DEADLINE", out, e)
	}
	sentinel := errors.New("owned close error")
	if e := finalizeTerminal(context.Background(), time.Now().Add(time.Second), &out, sentinel); !errors.Is(e, sentinel) {
		t.Fatal("ASSERT_TERMINAL_CLOSE_ERROR", e)
	}
	out.Disposition = "SOURCE_PROVISIONAL_DELIVERED_PAIR"
	if e := finalizeTerminal(context.Background(), time.Now().Add(time.Second), &out, nil); e != nil || out.Disposition != "SOURCE_PROVISIONAL_DELIVERED_PAIR" {
		t.Fatal("ASSERT_TERMINAL_LIVE_CONTROL", out, e)
	}
	t.Log("ASSERT_TERMINAL_POST_CLOSE_CANCEL: PASS; ASSERT_TERMINAL_ORIGINAL_DEADLINE: PASS; ASSERT_TERMINAL_CLOSE_ERROR: PASS; ASSERT_TERMINAL_LIVE_CONTROL: PASS")
}

// Finite AST supplements behavioral seam tests: pin the real owner's call order,
// not just a zero DTO. No compiler-closure scan or production test hook is used.
func TestSourceOperatorActualFinalizationBinding(t *testing.T) {
	fset := token.NewFileSet()
	f, e := parser.ParseFile(fset, "run.go", nil, 0)
	if e != nil {
		t.Fatal(e)
	}
	var body *ast.BlockStmt
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "Run" {
			body = fn.Body
		}
	}
	if body == nil {
		t.Fatal("ASSERT_TERMINAL_ACTUAL_BINDING")
	}
	var closePos, finalPos, eligiblePos token.Pos
	ast.Inspect(body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "finalizeTerminal" {
				finalPos = call.Pos()
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok {
					if id.Name == "roots" && sel.Sel.Name == "close" {
						closePos = call.Pos()
					}
					if id.Name == "eligible" && sel.Sel.Name == "Close" {
						eligiblePos = call.Pos()
					}
				}
			}
		}
		return true
	})
	if closePos == 0 || finalPos <= closePos || eligiblePos <= finalPos {
		t.Fatal("ASSERT_TERMINAL_ACTUAL_BINDING")
	}
	// First registered owner defer executes last; eligible's defer is later.
	raw, e := os.ReadFile("run.go")
	if e != nil || !strings.Contains(string(raw), "defer func() { err = errors.Join(err, eligible.Close()) }()") {
		t.Fatal("ASSERT_TERMINAL_ACTUAL_BINDING", e)
	}
	t.Log("ASSERT_TERMINAL_ACTUAL_BINDING: PASS (finite AST supplement; no atomic-clock proof)")
}

func TestSourceOperatorNativeBeforeCopiesBinding(t *testing.T) {
	fs := token.NewFileSet()
	f, e := parser.ParseFile(fs, "native.go", nil, 0)
	if e != nil {
		t.Fatal(e)
	}
	positions := map[string]token.Pos{}
	ast.Inspect(f, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			name := ""
			switch fn := c.Fun.(type) {
			case *ast.Ident:
				name = fn.Name
			case *ast.SelectorExpr:
				name = fn.Sel.Name
			}
			if positions[name] == 0 {
				positions[name] = c.Pos()
			}
		}
		return true
	})
	b, g := positions["NativePayloadBounds"], positions["nativeAggregateUpper"]
	if b == 0 || g <= b {
		t.Fatal("ASSERT_NATIVE_ACTUAL_BOUND_BEFORE_COPIES")
	}
	for _, name := range []string{"Bytes", "Semantic", "Marshal", "Sum256"} {
		if positions[name] <= g {
			t.Fatal("ASSERT_NATIVE_ACTUAL_BOUND_BEFORE_COPIES", name)
		}
	}
	raw, e := os.ReadFile("run.go")
	if e != nil {
		t.Fatal(e)
	}
	s := string(raw)
	first, host := strings.Index(s, "roots.admitNativeMetadata"), strings.Index(s, "sc.PrepareSourceHost")
	last, seal := strings.LastIndex(s, "roots.admitNativeMetadata"), strings.Index(s, "roots.sealNative")
	if first < 0 || first >= host || last <= host || last >= seal {
		t.Fatal("ASSERT_NATIVE_PRE_HOST_AND_POST_RESPONSE_BINDING")
	}
	t.Log("ASSERT_NATIVE_ACTUAL_BOUND_BEFORE_COPIES: PASS; ASSERT_NATIVE_PRE_HOST_AND_POST_RESPONSE_BINDING: PASS (finite AST supplement)")
}

func TestSourceOperatorPublication(t *testing.T) {
	home, e := os.UserHomeDir()
	if e != nil {
		t.Fatal(e)
	}
	dir, e := os.MkdirTemp(home, ".source-pub-")
	if e != nil {
		t.Fatal(e)
	}
	defer os.RemoveAll(dir)
	for _, s := range []string{"source", "account", "mailbox", "a", "b"} {
		if e = os.Mkdir(filepath.Join(dir, s), 0700); e != nil {
			t.Fatal(e)
		}
	}
	ctx := context.Background()
	p := &publicationOwner{limit: nativeMaxBytes, nativeUpper: nativeMaxBytes}
	p.source, e = publication.OpenSourceProtectionRootPrivateV2(ctx, filepath.Join(dir, "source"))
	if e != nil {
		t.Fatal(e)
	}
	for _, s := range []string{"account", "mailbox"} {
		r, e := publication.OpenObservedRoot(ctx, filepath.Join(dir, s))
		if e != nil {
			t.Fatal(e)
		}
		p.protected = append(p.protected, r)
	}
	for i, s := range []string{"a", "b"} {
		p.outputs[i], e = publication.OpenContentRetentionRoot(ctx, filepath.Join(dir, s))
		if e != nil {
			t.Fatal(e)
		}
	}
	defer p.close()
	if e = p.topology(ctx); e != nil {
		t.Fatal(e)
	}
	original := p.outputs[1]
	p.outputs[1] = p.outputs[0]
	if e = p.topology(ctx); e == nil {
		t.Fatal("ASSERT_NATIVE_ALIAS_REFUSED")
	}
	p.outputs[1] = original
	raw := []byte(`{"schema":"SOURCE_NATIVE_V1","test_only":"publication byte-custody control, not delivery"}`)
	n := nativeOutcome{id: fmt.Sprintf("sha256:%x", sha256.Sum256(raw)), raw: raw}
	// Wrong readback object in the second root: first publication remains retained.
	if e = os.WriteFile(filepath.Join(dir, "b", n.id), []byte("wrong"), 0600); e != nil {
		t.Fatal(e)
	}
	var out Outcome
	if e = p.publish(ctx, n, &out); e == nil || !out.Published[0] || !out.Readback[0] || out.Readback[1] {
		t.Fatal("ASSERT_NATIVE_PARTIAL_RETAINED", out, e)
	}
	if _, e = os.Stat(filepath.Join(dir, "a", n.id)); e != nil {
		t.Fatal("ASSERT_NATIVE_PARTIAL_RETAINED", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if e = p.publish(cancelled, n, &Outcome{}); e == nil {
		t.Fatal("ASSERT_NATIVE_CANCEL_REFUSED")
	}
	if e = p.outputs[1].Close(); e != nil {
		t.Fatal(e)
	}
	if e = p.publish(ctx, n, &Outcome{}); e == nil {
		t.Fatal("ASSERT_NATIVE_CLOSE_REFUSED")
	}
	t.Log("ASSERT_NATIVE_ALIAS_REFUSED: PASS; ASSERT_NATIVE_PARTIAL_RETAINED: PASS; ASSERT_NATIVE_CANCEL_REFUSED: PASS; ASSERT_NATIVE_CLOSE_REFUSED: PASS")
}
