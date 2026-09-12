package traceops

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/acquisitionops"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

type fakeRuntime struct {
	symbols      []lsp.DocumentSymbol
	methods      []string
	preparations []sessionruntime.DocumentRequest
	metadata     sessionruntime.SessionMetadata
}

func (f *fakeRuntime) Metadata(string, uint64) (sessionruntime.SessionMetadata, session.Failure) {
	if f.metadata.PositionEncoding != "" || f.metadata.CallHierarchySupport || f.metadata.DocumentSymbolSupport {
		return f.metadata, ""
	}
	return sessionruntime.SessionMetadata{PositionEncoding: "utf-16", CallHierarchySupport: true, DocumentSymbolSupport: true}, ""
}
func (f *fakeRuntime) Records() []sessionruntime.Record {
	selector, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: "/w", Profile: "test", EnvironmentReference: "test"})
	if err != nil {
		panic(err)
	}
	return []sessionruntime.Record{{SessionID: "s", Generation: 1, State: session.Ready, Profile: runtimeprofile.Resolve(selector)}}
}
func (f *fakeRuntime) PrepareDocument(_ context.Context, req sessionruntime.DocumentRequest) sessionruntime.DocumentResult {
	f.preparations = append(f.preparations, req)
	result := sessionruntime.DocumentResult{URI: req.URI, LanguageID: "go", Version: 1}
	if req.CaptureSupply {
		result.Supply = &sessionruntime.DocumentSupply{URI: req.URI, SessionID: req.SessionID, Generation: req.Generation, DocumentVersion: 1, Classification: "LSP_SUPPLIED", Method: "textDocument/didOpen", Content: []byte("package fixture\n"), Params: json.RawMessage(`{"textDocument":{"uri":"file:///w/a.go","languageId":"go","version":1,"text":"package fixture\n"}}`)}
	}
	return result
}
func (f *fakeRuntime) RoundTrip(_ context.Context, r sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult {
	f.methods = append(f.methods, r.Method)
	switch r.Method {
	case "textDocument/documentSymbol":
		raw, _ := json.Marshal(f.symbols)
		return sessionruntime.RoundTripResult{Result: raw}
	case "textDocument/prepareCallHierarchy":
		return sessionruntime.RoundTripResult{Result: json.RawMessage(`[{"name":"Target","kind":12,"uri":"file:///w/a.go","range":{"start":{"line":1,"character":0},"end":{"line":1,"character":3}},"selectionRange":{"start":{"line":1,"character":2},"end":{"line":1,"character":3}}}]`)}
	case "callHierarchy/outgoingCalls", "callHierarchy/incomingCalls":
		return sessionruntime.RoundTripResult{Result: json.RawMessage(`[]`)}
	default:
		return sessionruntime.RoundTripResult{Result: json.RawMessage(`[]`)}
	}
}

type fakeAcquirer struct{ requests []operation.Request }

func (f *fakeAcquirer) Execute(_ context.Context, r operation.Request) (operation.Result, *operation.Failure) {
	f.requests = append(f.requests, r)
	return operation.Result{Artifact: []byte(`{"schema_version":"lsp-trace.graph-provenance.v5"}`)}, nil
}

type supplyAcquirer struct {
	runtime  *fakeRuntime
	position bool
	symbol   bool
}

func (a *supplyAcquirer) Execute(_ context.Context, _ operation.Request) (operation.Result, *operation.Failure) {
	a.position = true
	doc := a.runtime.PrepareDocument(context.Background(), sessionruntime.DocumentRequest{SessionID: "s", Generation: 1, URI: "file:///w/a.go", CaptureSupply: true})
	if doc.Supply == nil {
		return operation.Result{}, &operation.Failure{Code: "MISSING_SOURCE_SUPPLY"}
	}
	return operation.Result{Artifact: []byte(`{"schema_version":"lsp-trace.graph-provenance.v5","source_supply":"LSP_SUPPLIED"}`)}, nil
}
func (a *supplyAcquirer) ExecuteWithPreparedDocument(_ context.Context, _ operation.Request, doc sessionruntime.DocumentResult) (operation.Result, *operation.Failure) {
	a.symbol = true
	if doc.Supply == nil {
		return operation.Result{}, &operation.Failure{Code: "MISSING_SOURCE_SUPPLY"}
	}
	return operation.Result{Artifact: []byte(`{"schema_version":"lsp-trace.graph-provenance.v5","source_supply":"LSP_SUPPLIED"}`)}, nil
}

func symbol(name string, line, character uint32) lsp.DocumentSymbol {
	p := lsp.Position{Line: line, Character: character}
	return lsp.DocumentSymbol{Name: name, Kind: 12, Range: lsp.Range{Start: p, End: lsp.Position{Line: line, Character: character + 1}}, SelectionRange: lsp.Range{Start: p, End: lsp.Position{Line: line, Character: character + 1}}}
}

func TestExactSymbolAmbiguityFailsBeforeAcquisitionWithBoundedDeterministicCandidates(t *testing.T) {
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

func TestV5SourceSupplySurvivesPositionAndSymbolModes(t *testing.T) {
	for _, tc := range []struct {
		name, target string
		symbol       bool
	}{{"position", `"positions":[{"line":1,"character":2}]`, false}, {"symbol", `"symbol":"Target"`, true}} {
		t.Run(tc.name, func(t *testing.T) {
			r := &fakeRuntime{}
			if tc.symbol {
				r.symbols = []lsp.DocumentSymbol{symbol("Target", 1, 2)}
			}
			a := &supplyAcquirer{runtime: r}
			result, failure := (&Executor{runtime: r, acquisition: a}).Execute(context.Background(), operation.Request{Name: Operation, Input: []byte(`{"session_id":"s","generation":1,"uri":"file:///w/a.go",` + tc.target + `}`)})
			if failure != nil || !strings.Contains(string(result.Artifact), `"source_supply":"LSP_SUPPLIED"`) || len(r.preparations) != 1 || !r.preparations[0].CaptureSupply || !a.symbol || a.position {
				t.Fatalf("ASSERT_TRACE_%s_V5_FIRST_PREPARATION_SUPPLIES_SOURCE: failure=%v artifact=%s preparations=%+v position=%t symbol=%t", strings.ToUpper(tc.name), failure, result.Artifact, r.preparations, a.position, a.symbol)
			}
		})
	}
}

func TestTraceRejectsMalformedExactSymbolRanges(t *testing.T) {
	cases := []struct {
		name                  string
		rangeValue, selection lsp.Range
	}{
		{"range", lsp.Range{Start: lsp.Position{Line: 2}, End: lsp.Position{Line: 1}}, lsp.Range{}},
		{"selection", lsp.Range{Start: lsp.Position{}, End: lsp.Position{Line: 3}}, lsp.Range{Start: lsp.Position{Line: 2}, End: lsp.Position{Line: 1}}},
		{"outside", lsp.Range{Start: lsp.Position{Line: 1}, End: lsp.Position{Line: 2}}, lsp.Range{Start: lsp.Position{Line: 3}, End: lsp.Position{Line: 3, Character: 1}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &fakeRuntime{symbols: []lsp.DocumentSymbol{{Name: "Target", Kind: 12, Range: tc.rangeValue, SelectionRange: tc.selection}}}
			a := &fakeAcquirer{}
			_, failure := (&Executor{runtime: r, acquisition: a}).Execute(context.Background(), operation.Request{Name: Operation, Input: []byte(`{"session_id":"s","generation":1,"uri":"file:///w/a.go","symbol":"Target"}`)})
			if failure == nil || failure.Code != "DOCUMENT_SYMBOL_MALFORMED_RANGE" || len(a.requests) != 0 {
				t.Fatalf("ASSERT_TRACE_EXACT_SYMBOL_MALFORMED_RANGE_REJECTED: failure=%v requests=%d", failure, len(a.requests))
			}
		})
	}
}

func TestTraceMetadataRejectsBeforeDocumentOrLSPActivity(t *testing.T) {
	for _, tc := range []struct {
		name, input, want string
		metadata          sessionruntime.SessionMetadata
	}{
		{"call hierarchy", `"positions":[{"line":0,"character":0}]`, "UNSUPPORTED_CALL_HIERARCHY", sessionruntime.SessionMetadata{PositionEncoding: "utf-16"}},
		{"document symbol", `"symbol":"Target"`, "UNSUPPORTED_DOCUMENT_SYMBOL", sessionruntime.SessionMetadata{PositionEncoding: "utf-16", CallHierarchySupport: true}},
		{"encoding", `"positions":[{"line":0,"character":0}]`, "UNSUPPORTED_POSITION_ENCODING", sessionruntime.SessionMetadata{PositionEncoding: "guess", CallHierarchySupport: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &fakeRuntime{metadata: tc.metadata}
			a := &fakeAcquirer{}
			_, failure := (&Executor{runtime: r, acquisition: a}).Execute(context.Background(), operation.Request{Name: Operation, Input: []byte(`{"session_id":"s","generation":1,"uri":"file:///w/a.go",` + tc.input + `}`)})
			if failure == nil || failure.Code != tc.want || len(r.preparations) != 0 || len(r.methods) != 0 || len(a.requests) != 0 {
				t.Fatalf("ASSERT_TRACE_METADATA_REJECTS_BEFORE_ACTIVITY: failure=%v preparations=%d methods=%v acquisitions=%d", failure, len(r.preparations), r.methods, len(a.requests))
			}
		})
	}
}

func TestMultiPositionUsesOneNativeAcquisitionWithCanonicalExplicitCustody(t *testing.T) {
	r := &fakeRuntime{}
	a := &fakeAcquirer{}
	e := &Executor{runtime: r, acquisition: a}
	_, f := e.Execute(context.Background(), operation.Request{Name: Operation, Input: []byte(`{"session_id":"s","generation":1,"uri":"file:///w/a.go","positions":[{"line":1,"character":2},{"line":3,"character":4}]}`), RetainedSeedSpec: []byte("forged")})
	if f != nil || len(a.requests) != 1 || a.requests[0].Name != acquisitionops.SliceV3 || !strings.Contains(string(a.requests[0].RetainedSeedSpec), `"schema_version":"lsp-trace.seeds.v2"`) || strings.Contains(string(a.requests[0].RetainedSeedSpec), "discover") {
		t.Fatalf("ASSERT_TRACE_MULTI_POSITION_ONE_NATIVE_EXPLICIT_CUSTODY: failure=%v requests=%+v", f, a.requests)
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

func TestRealTraceToRealAcquisitionProducesAdmittedV5ForPositionAndSymbolRepeated(t *testing.T) {
	for _, tc := range []struct{ name, target string }{{"position", `"positions":[{"line":1,"character":2}]`}, {"symbol", `"symbol":"Target"`}} {
		t.Run(tc.name, func(t *testing.T) {
			r := &fakeRuntime{symbols: []lsp.DocumentSymbol{symbol("Target", 1, 2)}}
			e := NewExecutor(r)
			for run := 0; run < 2; run++ {
				result, failure := e.Execute(context.Background(), operation.Request{Name: Operation, Input: []byte(`{"session_id":"s","generation":1,"uri":"file:///w/a.go",` + tc.target + `}`)})
				if failure != nil {
					t.Fatalf("ASSERT_REAL_TRACE_ACQUISITION_V5_ADMITTED_%s_RUN_%d: %v", tc.name, run, failure)
				}
				if _, err := graphprovenance.ValidateFor(result.Artifact, graphprovenance.Family, "v5"); err != nil {
					t.Fatalf("ASSERT_REAL_TRACE_ACQUISITION_V5_SCHEMA_%s_RUN_%d: %v", tc.name, run, err)
				}
				var evidence graphprovenance.EvidenceV5
				if err := json.Unmarshal(result.Artifact, &evidence); err != nil || evidence.SeedSpec == nil || strings.Contains(string(evidence.SeedSpec.Bytes), "discover") {
					t.Fatalf("ASSERT_REAL_TRACE_EXPLICIT_SEED_CUSTODY_%s_RUN_%d: err=%v evidence=%+v", tc.name, run, err, evidence.SeedSpec)
				}
			}
		})
	}
}

func TestTraceExplicitZeroDepthIsPreserved(t *testing.T) {
	r := &fakeRuntime{}
	a := &fakeAcquirer{}
	_, failure := (&Executor{runtime: r, acquisition: a}).Execute(context.Background(), operation.Request{Name: Operation, Input: []byte(`{"session_id":"s","generation":1,"uri":"file:///w/a.go","positions":[{"line":0,"character":0}],"down_depth":0,"up_depth":0}`)})
	if failure != nil || len(a.requests) != 1 {
		t.Fatalf("ASSERT_TRACE_ZERO_DEPTH_PRESERVED: failure=%v requests=%d", failure, len(a.requests))
	}
	var in acquisitionops.Input
	if err := json.Unmarshal(a.requests[0].Input, &in); err != nil {
		t.Fatal(err)
	}
	req, err := in.SeedManifest.Request(acquisitionops.SliceV3)
	if err != nil || req.Root.DownDepth != 0 || req.Root.UpDepth != 0 {
		t.Fatalf("ASSERT_TRACE_ZERO_DEPTH_PRESERVED: request=%+v err=%v", req, err)
	}
}
