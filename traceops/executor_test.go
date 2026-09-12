package traceops

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/acquisitionops"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

type fakeRuntime struct {
	symbols []lsp.DocumentSymbol
	methods []string
}

func (f *fakeRuntime) Metadata(string, uint64) (sessionruntime.SessionMetadata, session.Failure) {
	return sessionruntime.SessionMetadata{PositionEncoding: "utf-16", CallHierarchySupport: true, DocumentSymbolSupport: true}, ""
}
func (f *fakeRuntime) Records() []sessionruntime.Record { return nil }
func (f *fakeRuntime) RoundTrip(_ context.Context, r sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult {
	f.methods = append(f.methods, r.Method)
	raw, _ := json.Marshal(f.symbols)
	return sessionruntime.RoundTripResult{Result: raw}
}

type fakeAcquirer struct{ requests []operation.Request }

func (f *fakeAcquirer) Execute(_ context.Context, r operation.Request) (operation.Result, *operation.Failure) {
	f.requests = append(f.requests, r)
	return operation.Result{Artifact: []byte(`{"schema_version":"lsp-trace.graph-provenance.v5"}`)}, nil
}
func symbol(name string, line, character uint32) lsp.DocumentSymbol {
	p := lsp.Position{Line: line, Character: character}
	return lsp.DocumentSymbol{Name: name, Kind: 12, Range: lsp.Range{Start: p, End: lsp.Position{Line: line, Character: character + 1}}, SelectionRange: lsp.Range{Start: p, End: lsp.Position{Line: line, Character: character + 1}}}
}

func TestExactSymbolAmbiguityFailsBeforePrepareWithBoundedDeterministicCandidates(t *testing.T) {
	r := &fakeRuntime{}
	for i := uint32(10); i > 0; i-- {
		r.symbols = append(r.symbols, symbol("Same", i, i))
	}
	a := &fakeAcquirer{}
	e := &Executor{runtime: r, acquisition: a}
	_, f := e.Execute(context.Background(), operation.Request{Name: Operation, Input: []byte(`{"session_id":"s","generation":1,"uri":"file:///w/a.go","symbol":"Same"}`)})
	if f == nil || f.Code != "DOCUMENT_SYMBOL_AMBIGUOUS" || len(a.requests) != 0 || len(r.methods) != 1 || r.methods[0] != "textDocument/documentSymbol" || len(f.Diagnostics) != 1 || !strings.Contains(f.Diagnostics[0], "total=10 omitted=2") || !strings.Contains(f.Diagnostics[0], "line=1,character=1") || !strings.Contains(f.Diagnostics[0], "use positions") {
		t.Fatalf("ASSERT_TRACE_EXACT_AMBIGUITY_PREPARE_FREE_BOUNDED: failure=%+v methods=%v requests=%d", f, r.methods, len(a.requests))
	}
}

func TestMultiPositionUsesOneNativeAcquisitionWithDefaultsAndNoSeedCustody(t *testing.T) {
	r := &fakeRuntime{}
	a := &fakeAcquirer{}
	e := &Executor{runtime: r, acquisition: a}
	_, f := e.Execute(context.Background(), operation.Request{Name: Operation, Input: []byte(`{"session_id":"s","generation":1,"uri":"file:///w/a.go","positions":[{"line":1,"character":2},{"line":3,"character":4}]}`), RetainedSeedSpec: []byte("forged")})
	if f != nil || len(a.requests) != 1 || a.requests[0].Name != acquisitionops.SliceV3 || len(a.requests[0].RetainedSeedSpec) != 0 {
		t.Fatalf("ASSERT_TRACE_MULTI_POSITION_ONE_NATIVE_ZERO_CUSTODY: failure=%v requests=%+v", f, a.requests)
	}
	var in acquisitionops.Input
	if err := json.Unmarshal(a.requests[0].Input, &in); err != nil {
		t.Fatal(err)
	}
	if in.OutputVersion != "lsp-trace.graph-provenance.v5" || len(in.SeedManifest.RequiredTargets) != 1 || in.SeedManifest.Root.DownDepth != nil || in.SeedManifest.Root.UpDepth != nil || in.SeedManifest.Expansion.TopmostSiblings {
		t.Fatalf("ASSERT_TRACE_DEFAULT_DEPTH2_ZERO_SIBLINGS: %+v", in)
	}
	req, err := in.SeedManifest.Request(acquisitionops.SliceV3)
	if err != nil || req.Root.DownDepth != 2 || req.Root.UpDepth != 2 || req.TopmostSiblings {
		t.Fatalf("ASSERT_TRACE_NATIVE_DEFAULTS: req=%+v err=%v", req, err)
	}
}
