package incomingops

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/provider"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

type fakeRuntime struct {
	metadata         sessionruntime.SessionMetadata
	failure          session.Failure
	calls            []string
	requests         []sessionruntime.RoundTripRequest
	results          map[string][]json.RawMessage
	observed         map[string][]sessionruntime.RoundTripResult
	records          []sessionruntime.Record
	relationCalls    [][]string
	relationArtifact json.RawMessage
	relationErr      error
}

func (f *fakeRuntime) Records() []sessionruntime.Record {
	return append([]sessionruntime.Record(nil), f.records...)
}
func (f *fakeRuntime) Metadata(string, uint64) (sessionruntime.SessionMetadata, session.Failure) {
	return f.metadata, f.failure
}
func (f *fakeRuntime) CollectRelations(_ context.Context, selected []string, _ json.RawMessage) (provider.Result, error) {
	f.relationCalls = append(f.relationCalls, append([]string(nil), selected...))
	if f.relationErr != nil {
		return provider.Result{}, f.relationErr
	}
	var legacy struct {
		ProviderID string           `json:"provider_id"`
		Terminal   string           `json:"terminal"`
		Complete   bool             `json:"complete"`
		Truncated  bool             `json:"truncated"`
		Bounds     json.RawMessage  `json:"bounds"`
		Relations  []graph.Relation `json:"relations"`
	}
	if err := json.Unmarshal(f.relationArtifact, &legacy); err != nil {
		return provider.Result{}, err
	}
	return provider.Result{ProviderID: legacy.ProviderID, Terminal: legacy.Terminal, Complete: legacy.Complete, Truncated: legacy.Truncated, GraphV4: graph.NormalizedRelations{SchemaVersion: graph.NormalizedRelationsSchemaVersion, ArtifactKind: graph.NormalizedRelationsArtifactKind, Relations: legacy.Relations, Provenance: &graph.NormalizedRelationsProvenance{ProviderID: legacy.ProviderID, Bounds: legacy.Bounds}}}, nil
}
func (f *fakeRuntime) RoundTrip(_ context.Context, r sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult {
	f.calls = append(f.calls, r.Method)
	f.requests = append(f.requests, r)
	if queue := f.observed[r.Method]; len(queue) > 0 {
		f.observed[r.Method] = queue[1:]
		return queue[0]
	}
	queue := f.results[r.Method]
	if len(queue) == 0 {
		return sessionruntime.RoundTripResult{Failure: session.RequestTimeout}
	}
	f.results[r.Method] = queue[1:]
	return sessionruntime.RoundTripResult{Result: queue[0]}
}

func validInput() json.RawMessage {
	return json.RawMessage(`{"session_id":"s","generation":1,"uri":"file:///w/a.go","line":0,"character":0,"max_depth":4,"max_nodes":20,"timeout_ms":1000,"request_timeout_ms":100}`)
}

func TestIncomingUnsupportedCallHierarchySendsNoHierarchyRequests(t *testing.T) {
	const assertion = "ASSERT_UNSUPPORTED_CALL_HIERARCHY_SENDS_NO_HIERARCHY_REQUESTS"
	f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{PositionEncoding: "utf-16", CallHierarchySupport: false}}
	_, failure := NewExecutor(f).Execute(context.Background(), operation.Request{Name: OperationIncoming, Input: validInput()})
	if failure == nil || failure.Code != string(graph.UnsupportedCallHierarchy) || len(f.calls) != 0 {
		t.Fatalf("%s: failure=%+v calls=%v", assertion, failure, f.calls)
	}
}

func TestIncomingAppliesConservativeDefaults(t *testing.T) {
	const assertion = "ASSERT_INCOMING_CONSERVATIVE_DEFAULTS"
	t.Log("ASSERTION: " + assertion)
	item := `{"name":"leaf","kind":12,"uri":"file:///w/a.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":4}},"selectionRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":4}}}`
	f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{PositionEncoding: "utf-16", CallHierarchySupport: true}, results: map[string][]json.RawMessage{
		"textDocument/prepareCallHierarchy": {json.RawMessage(`[` + item + `]`)},
		"callHierarchy/incomingCalls":       {json.RawMessage(`[]`)},
	}}
	result, failure := NewExecutor(f).Execute(context.Background(), operation.Request{Name: OperationIncoming, Input: json.RawMessage(`{"session_id":"s","generation":1,"uri":"file:///w/a.go","line":0,"character":0}`)})
	if failure != nil {
		t.Fatalf("%s: %v", assertion, failure)
	}
	var got graph.Result
	if err := json.Unmarshal(result.Artifact, &got); err != nil || got.Invocation.Limits.MaxDepth <= 0 || got.Invocation.Limits.MaxNodes <= 0 || got.Invocation.Limits.TimeoutMS <= 0 || got.Invocation.RequestTimeoutMS <= 0 {
		t.Fatalf("%s: limits=%+v request_timeout=%d err=%v", assertion, got.Invocation.Limits, got.Invocation.RequestTimeoutMS, err)
	}
	t.Log("PASS " + assertion)
}

func TestIncomingUsesRoundTripAndExistingDeterministicTraversal(t *testing.T) {
	const assertion = "ASSERT_INCOMING_ROUNDTRIP_TRAVERSE_DETERMINISTIC"
	item := `{"name":"leaf","kind":12,"uri":"file:///w/a.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":4}},"selectionRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":4}},"data":{"opaque":1}}`
	caller := `{"name":"caller","kind":12,"uri":"file:///w/a.go","range":{"start":{"line":2,"character":0},"end":{"line":2,"character":6}},"selectionRange":{"start":{"line":2,"character":0},"end":{"line":2,"character":6}}}`
	f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{PositionEncoding: "utf-16", CallHierarchySupport: true}, results: map[string][]json.RawMessage{
		"textDocument/prepareCallHierarchy": {json.RawMessage(`[` + item + `]`)},
		"callHierarchy/incomingCalls":       {json.RawMessage(`[{"from":` + caller + `,"fromRanges":[{"start":{"line":2,"character":1},"end":{"line":2,"character":2}}]}]`), json.RawMessage(`[]`)},
	}}
	result, failure := NewExecutor(f).Execute(context.Background(), operation.Request{Name: OperationIncoming, Input: validInput()})
	if failure != nil {
		t.Fatalf("%s: %v", assertion, failure)
	}
	var got graph.Result
	if err := json.Unmarshal(result.Artifact, &got); err != nil {
		t.Fatal(err)
	}
	if strings.Join(f.calls, ",") != "textDocument/prepareCallHierarchy,callHierarchy/incomingCalls,callHierarchy/incomingCalls" || got.Summary.NodeCount != 2 || got.Summary.EdgeCount != 1 || !strings.Contains(string(result.Artifact), `"traversal_complete":true`) {
		t.Fatalf("%s: calls=%v summary=%+v artifact=%s", assertion, f.calls, got.Summary, result.Artifact)
	}
}

func TestIncomingRetainsFixedPerWireRequestBounds(t *testing.T) {
	const assertion = "ASSERT_INCOMING_FIXED_PER_WIRE_REQUEST_BOUNDS"
	t.Log("ASSERTION: " + assertion)
	item := `{"name":"leaf","kind":12,"uri":"file:///w/a.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":4}},"selectionRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":4}}}`
	f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{PositionEncoding: "utf-16", CallHierarchySupport: true}, results: map[string][]json.RawMessage{
		"textDocument/prepareCallHierarchy": {json.RawMessage(`[` + item + `]`)},
		"callHierarchy/incomingCalls":       {json.RawMessage(`[]`)},
	}}
	if _, failure := NewExecutor(f).Execute(context.Background(), operation.Request{Name: OperationIncoming, Input: validInput()}); failure != nil {
		t.Fatalf("%s: %v", assertion, failure)
	}
	for _, request := range f.requests {
		if request.MaxMessages != 64 || request.MaxBytes != 4<<20 {
			t.Fatalf("%s: method=%s max_messages=%d max_bytes=%d", assertion, request.Method, request.MaxMessages, request.MaxBytes)
		}
	}
}

func TestIncomingRejectsBeforeEffectsAndLabelsFailures(t *testing.T) {
	for _, tc := range []struct {
		name     string
		input    json.RawMessage
		metadata sessionruntime.SessionMetadata
		failure  session.Failure
		code     string
	}{
		{"invalid", json.RawMessage(`{"session_id":"","generation":1}`), sessionruntime.SessionMetadata{}, "", operation.FailureInvalidInput},
		{"stale", validInput(), sessionruntime.SessionMetadata{}, session.StaleGeneration, string(session.StaleGeneration)},
		{"unsupported", validInput(), sessionruntime.SessionMetadata{PositionEncoding: "utf-16"}, "", string(graph.UnsupportedCallHierarchy)},
		{"encoding", validInput(), sessionruntime.SessionMetadata{PositionEncoding: "guess", CallHierarchySupport: true}, "", "UNSUPPORTED_POSITION_ENCODING"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeRuntime{metadata: tc.metadata, failure: tc.failure}
			_, got := NewExecutor(f).Execute(context.Background(), operation.Request{Name: OperationIncoming, Input: tc.input})
			if got == nil || got.Code != tc.code || len(f.calls) != 0 {
				t.Fatalf("ASSERT_INCOMING_VALIDATE_BEFORE_EFFECTS_%s: failure=%v calls=%v", tc.name, got, f.calls)
			}
		})
	}
}

func TestIncomingEmptyPrepareIsComplete(t *testing.T) {
	for _, prepare := range []json.RawMessage{json.RawMessage(`[]`), json.RawMessage(`null`)} {
		f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{PositionEncoding: "utf-16", CallHierarchySupport: true}, results: map[string][]json.RawMessage{"textDocument/prepareCallHierarchy": {prepare}}}
		result, failure := NewExecutor(f).Execute(context.Background(), operation.Request{Name: OperationIncoming, Input: validInput()})
		if failure != nil || !strings.Contains(string(result.Artifact), `"traversal_complete":true`) || len(f.calls) != 1 {
			t.Fatalf("ASSERT_INCOMING_EMPTY_PREPARE: failure=%v calls=%v artifact=%s", failure, f.calls, result.Artifact)
		}
	}
}

func TestIncomingRuntimeFailuresAreHonest(t *testing.T) {
	for _, tc := range []struct {
		name     string
		observed sessionruntime.RoundTripResult
		want     string
	}{
		{"cancel", sessionruntime.RoundTripResult{Failure: session.RequestCancelled}, "CANCELLED"},
		{"unsupported-method", sessionruntime.RoundTripResult{ServerError: &lspwire.RPCError{Code: -32601, Message: "method not found"}}, "method not found"},
		{"server-error", sessionruntime.RoundTripResult{ServerError: &lspwire.RPCError{Code: -32603, Message: "boom"}}, "boom"},
		{"eof-crash", sessionruntime.RoundTripResult{Failure: session.SessionCrashed}, string(session.SessionCrashed)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{PositionEncoding: "utf-16", CallHierarchySupport: true}, observed: map[string][]sessionruntime.RoundTripResult{"textDocument/prepareCallHierarchy": {tc.observed}}}
			result, failure := NewExecutor(f).Execute(context.Background(), operation.Request{Name: OperationIncoming, Input: validInput()})
			if failure != nil || !strings.Contains(string(result.Artifact), tc.want) || !strings.Contains(string(result.Artifact), `"traversal_complete":false`) {
				t.Fatalf("ASSERT_INCOMING_RUNTIME_FAILURE_%s: failure=%v artifact=%s", tc.name, failure, result.Artifact)
			}
		})
	}
}

func TestIncomingCycleAndBounds(t *testing.T) {
	item := `{"name":"a","kind":12,"uri":"file:///w/a.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}},"selectionRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}`
	caller := `{"name":"b","kind":12,"uri":"file:///w/a.go","range":{"start":{"line":1,"character":0},"end":{"line":1,"character":1}},"selectionRange":{"start":{"line":1,"character":0},"end":{"line":1,"character":1}}}`
	for _, tc := range []struct {
		name  string
		input json.RawMessage
		calls []json.RawMessage
		want  string
	}{
		{"cycle", validInput(), []json.RawMessage{json.RawMessage(`[{"from":` + caller + `,"fromRanges":[]}]`), json.RawMessage(`[{"from":` + item + `,"fromRanges":[]}]`)}, `"node_count":2`},
		{"node-bound", json.RawMessage(`{"session_id":"s","generation":1,"uri":"file:///w/a.go","line":0,"character":0,"max_depth":4,"max_nodes":1,"timeout_ms":1000,"request_timeout_ms":100}`), []json.RawMessage{json.RawMessage(`[{"from":` + caller + `,"fromRanges":[]}]`)}, "MAX_NODES"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{PositionEncoding: "utf-16", CallHierarchySupport: true}, results: map[string][]json.RawMessage{"textDocument/prepareCallHierarchy": {json.RawMessage(`[` + item + `]`)}, "callHierarchy/incomingCalls": tc.calls}}
			result, failure := NewExecutor(f).Execute(context.Background(), operation.Request{Name: OperationIncoming, Input: tc.input})
			if failure != nil || !strings.Contains(string(result.Artifact), tc.want) {
				t.Fatalf("ASSERT_INCOMING_%s: failure=%v artifact=%s", tc.name, failure, result.Artifact)
			}
		})
	}
}

func TestIncomingMalformedAndTimeoutArePartialHonest(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare json.RawMessage
		want    string
	}{
		{"malformed", json.RawMessage(`{"wrong":true}`), "malformed"},
		{"timeout", nil, "REQUEST_TIMEOUT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{PositionEncoding: "utf-8", CallHierarchySupport: true}, results: map[string][]json.RawMessage{}}
			if tc.prepare != nil {
				f.results["textDocument/prepareCallHierarchy"] = []json.RawMessage{tc.prepare}
			}
			result, failure := NewExecutor(f).Execute(context.Background(), operation.Request{Name: OperationIncoming, Input: validInput()})
			if failure != nil {
				t.Fatal(failure)
			}
			var got graph.Result
			_ = json.Unmarshal(result.Artifact, &got)
			raw := string(result.Artifact)
			if got.Summary.Complete || !strings.Contains(raw, tc.want) {
				t.Fatalf("ASSERT_INCOMING_PARTIAL_LABEL_%s: %s", tc.name, raw)
			}
		})
	}
}

func TestIncomingSymbolFailuresAreExplicit(t *testing.T) {
	const symbol = `{"name":"Target","kind":12,"range":{"start":{"line":1,"character":0},"end":{"line":1,"character":2}},"selectionRange":{"start":{"line":1,"character":0},"end":{"line":1,"character":2}}}`
	base := func() *fakeRuntime {
		return &fakeRuntime{metadata: sessionruntime.SessionMetadata{PositionEncoding: "utf-16", CallHierarchySupport: true}, results: map[string][]json.RawMessage{}}
	}
	cases := []struct {
		name, assertion, want string
		configure             func(*fakeRuntime)
		input                 string
	}{
		{"unsupported", "ASSERT_DOCUMENT_SYMBOL_UNSUPPORTED", "DOCUMENT_SYMBOL_UNSUPPORTED", func(f *fakeRuntime) {
			f.observed = map[string][]sessionruntime.RoundTripResult{"textDocument/documentSymbol": {sessionruntime.RoundTripResult{ServerError: &lspwire.RPCError{Code: -32601, Message: "method not found"}}}}
		}, `{"session_id":"s","generation":1,"uri":"file:///w/a.go","symbol":"Target"}`},
		{"failed", "ASSERT_DOCUMENT_SYMBOL_FAILED", "DOCUMENT_SYMBOL_FAILED", func(f *fakeRuntime) {
			f.results["textDocument/documentSymbol"] = []json.RawMessage{json.RawMessage(`{"not":"an array"}`)}
		}, `{"session_id":"s","generation":1,"uri":"file:///w/a.go","symbol":"Target"}`},
		{"absent", "ASSERT_DOCUMENT_SYMBOL_ABSENT", "DOCUMENT_SYMBOL_ABSENT", func(f *fakeRuntime) {
			f.results["textDocument/documentSymbol"] = []json.RawMessage{json.RawMessage(`[]`)}
		}, `{"session_id":"s","generation":1,"uri":"file:///w/a.go","symbol":"Target"}`},
		{"ambiguous", "ASSERT_DOCUMENT_SYMBOL_AMBIGUOUS", "DOCUMENT_SYMBOL_AMBIGUOUS", func(f *fakeRuntime) {
			f.results["textDocument/documentSymbol"] = []json.RawMessage{json.RawMessage(`[` + symbol + `,{"name":"Container","kind":5,"range":{"start":{"line":0,"character":0},"end":{"line":2,"character":0}},"selectionRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":9}},"children":[` + symbol + `]}]`)}
		}, `{"session_id":"s","generation":1,"uri":"file:///w/a.go","symbol":"Target"}`},
		{"mixed", "ASSERT_TARGET_MODE_EXCLUSIVE", operation.FailureInvalidInput, func(*fakeRuntime) {}, `{"session_id":"s","generation":1,"uri":"file:///w/a.go","symbol":"Target","line":1,"character":0}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := base()
			tc.configure(f)
			_, failure := NewExecutor(f).Execute(context.Background(), operation.Request{Name: OperationIncoming, Input: json.RawMessage(tc.input)})
			if failure == nil || failure.Code != tc.want {
				t.Fatalf("%s: failure=%v", tc.assertion, failure)
			}
			t.Log("PASS " + tc.assertion)
		})
	}
}

func TestIncomingRealGoplsSymbolSpecimenReconcilesPreparePosition(t *testing.T) {
	const assertion = "ASSERT_REAL_GOPLS_NEW_PREPARE_POSITION_355_5"
	t.Log("ASSERTION: " + assertion)
	raw, err := os.ReadFile("testdata/gopls-v0.23.0-sessionruntime-new-document-symbol.json")
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatalf("ASSERT_REAL_GOPLS_DOCUMENT_SYMBOL_SPECIMEN_VALID: %v", err)
	}
	item := `{"name":"New","kind":12,"uri":"file:///w/sessionruntime.go","range":{"start":{"line":355,"character":5},"end":{"line":355,"character":8}},"selectionRange":{"start":{"line":355,"character":5},"end":{"line":355,"character":8}}}`
	miss := sessionruntime.RoundTripResult{ServerError: &lspwire.RPCError{Code: 0, Message: "identifier not found"}}
	f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{PositionEncoding: "utf-16", CallHierarchySupport: true},
		results:  map[string][]json.RawMessage{"textDocument/documentSymbol": {response.Result}, "textDocument/prepareCallHierarchy": {json.RawMessage(`[` + item + `]`), json.RawMessage(`[` + item + `]`)}, "callHierarchy/incomingCalls": {json.RawMessage(`[]`)}},
		observed: map[string][]sessionruntime.RoundTripResult{"textDocument/prepareCallHierarchy": {miss, miss, miss, miss, miss}},
	}
	result, failure := NewExecutor(f).Execute(context.Background(), operation.Request{Name: OperationIncoming, Input: json.RawMessage(`{"session_id":"s","generation":1,"uri":"file:///w/sessionruntime.go","symbol":"New"}`)})
	if failure != nil {
		t.Fatalf("%s: failure=%v", assertion, failure)
	}
	var got graph.Result
	if err := json.Unmarshal(result.Artifact, &got); err != nil {
		t.Fatal(err)
	}
	if got.Invocation.Target.Line != 355 || got.Invocation.Target.Column != 5 {
		t.Fatalf("%s: target=%+v requests=%v", assertion, got.Invocation.Target, f.requests)
	}
	for i, request := range f.requests[1:7] {
		var params struct {
			Position struct{ Line, Character uint32 } `json:"position"`
		}
		if err := json.Unmarshal(request.Params, &params); err != nil || params.Position.Line != 355 || params.Position.Character != uint32(i) {
			t.Fatalf("%s: probe[%d]=%s err=%v", assertion, i, request.Params, err)
		}
	}
	t.Log("PASS " + assertion)
}

func TestIncomingSymbolProbeBoundsAndFailures(t *testing.T) {
	const symbol = `[{"name":"Target","kind":12,"location":{"uri":"file:///w/a.go","range":{"start":{"line":7,"character":0},"end":{"line":9,"character":1}}}}]`
	t.Run("bounded", func(t *testing.T) {
		const assertion = "ASSERT_SYMBOL_PREPARE_PROBE_BOUNDED_65"
		f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{PositionEncoding: "utf-16", CallHierarchySupport: true}, results: map[string][]json.RawMessage{"textDocument/documentSymbol": {json.RawMessage(symbol)}, "textDocument/prepareCallHierarchy": make([]json.RawMessage, 65)}}
		for i := range f.results["textDocument/prepareCallHierarchy"] {
			f.results["textDocument/prepareCallHierarchy"][i] = json.RawMessage(`[]`)
		}
		_, failure := NewExecutor(f).Execute(context.Background(), operation.Request{Name: OperationIncoming, Input: json.RawMessage(`{"session_id":"s","generation":1,"uri":"file:///w/a.go","symbol":"Target"}`)})
		if failure == nil || failure.Code != "DOCUMENT_SYMBOL_UNPREPARABLE" || len(f.requests) != 66 {
			t.Fatalf("%s: failure=%v requests=%d", assertion, failure, len(f.requests))
		}
	})
	t.Run("malformed range", func(t *testing.T) {
		const assertion = "ASSERT_SYMBOL_MALFORMED_RANGE_NO_PROBE"
		malformed := `[{"name":"Target","kind":12,"location":{"uri":"file:///w/a.go","range":{"start":{"line":9,"character":0},"end":{"line":7,"character":1}}}}]`
		f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{PositionEncoding: "utf-16", CallHierarchySupport: true}, results: map[string][]json.RawMessage{"textDocument/documentSymbol": {json.RawMessage(malformed)}}}
		_, failure := NewExecutor(f).Execute(context.Background(), operation.Request{Name: OperationIncoming, Input: json.RawMessage(`{"session_id":"s","generation":1,"uri":"file:///w/a.go","symbol":"Target"}`)})
		if failure == nil || failure.Code != "DOCUMENT_SYMBOL_MALFORMED_RANGE" || len(f.requests) != 1 {
			t.Fatalf("%s: failure=%v requests=%v", assertion, failure, f.requests)
		}
	})
	for _, tc := range []struct {
		name, assertion, want string
		observed              sessionruntime.RoundTripResult
	}{
		{"cancel", "ASSERT_SYMBOL_PROBE_CANCELLATION_STOPS", "CANCELLED", sessionruntime.RoundTripResult{Failure: session.RequestCancelled}},
		{"timeout", "ASSERT_SYMBOL_PROBE_TIMEOUT_STOPS", "REQUEST_TIMEOUT", sessionruntime.RoundTripResult{Failure: session.RequestTimeout}},
		{"server", "ASSERT_SYMBOL_PROBE_NON_POSITION_SERVER_ERROR_STOPS", "DOCUMENT_SYMBOL_PREPARE_FAILED", sessionruntime.RoundTripResult{ServerError: &lspwire.RPCError{Code: -32603, Message: "boom"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{PositionEncoding: "utf-16", CallHierarchySupport: true}, results: map[string][]json.RawMessage{"textDocument/documentSymbol": {json.RawMessage(symbol)}}, observed: map[string][]sessionruntime.RoundTripResult{"textDocument/prepareCallHierarchy": {tc.observed}}}
			_, failure := NewExecutor(f).Execute(context.Background(), operation.Request{Name: OperationIncoming, Input: json.RawMessage(`{"session_id":"s","generation":1,"uri":"file:///w/a.go","symbol":"Target"}`)})
			if failure == nil || failure.Code != tc.want || len(f.requests) != 2 {
				t.Fatalf("%s: failure=%v requests=%v", tc.assertion, failure, f.requests)
			}
		})
	}
}

func TestIncomingSelectorContracts(t *testing.T) {
	documentSymbol := `{"name":"Target","kind":12,"range":{"start":{"line":7,"character":1},"end":{"line":9,"character":1}},"selectionRange":{"start":{"line":7,"character":3},"end":{"line":7,"character":9}}}`
	item := `{"name":"Target","kind":12,"uri":"file:///w/a.go","range":{"start":{"line":7,"character":1},"end":{"line":9,"character":1}},"selectionRange":{"start":{"line":7,"character":3},"end":{"line":7,"character":9}}}`
	t.Run("omitted generation", func(t *testing.T) {
		const assertion = "ASSERT_INCOMING_OMITTED_GENERATION_UNIQUE_READY"
		f := &fakeRuntime{records: []sessionruntime.Record{{SessionID: "s", Generation: 2, State: session.Ready}}, metadata: sessionruntime.SessionMetadata{PositionEncoding: "utf-16", CallHierarchySupport: true}, results: map[string][]json.RawMessage{"textDocument/prepareCallHierarchy": {json.RawMessage(`[` + item + `]`)}, "callHierarchy/incomingCalls": {json.RawMessage(`[]`)}}}
		_, failure := NewExecutor(f).Execute(context.Background(), operation.Request{Name: OperationIncoming, Input: json.RawMessage(`{"session_id":"s","uri":"file:///w/a.go","line":0,"character":0}`)})
		if failure != nil {
			t.Fatalf("%s: READY failure=%v", assertion, failure)
		}
		f.records[0].State = session.Stopped
		_, failure = NewExecutor(f).Execute(context.Background(), operation.Request{Name: OperationIncoming, Input: json.RawMessage(`{"session_id":"s","uri":"file:///w/a.go","line":0,"character":0}`)})
		if failure == nil || failure.Code != "SESSION_NOT_READY" {
			t.Fatalf("%s: non-READY failure=%v", assertion, failure)
		}
	})
	t.Run("unique symbol", func(t *testing.T) {
		const assertion = "ASSERT_INCOMING_SYMBOL_UNIQUE_SELECTION_START"
		f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{PositionEncoding: "utf-16", CallHierarchySupport: true}, results: map[string][]json.RawMessage{"textDocument/documentSymbol": {json.RawMessage(`[` + documentSymbol + `]`)}, "textDocument/prepareCallHierarchy": {json.RawMessage(`[` + item + `]`)}, "callHierarchy/incomingCalls": {json.RawMessage(`[]`)}}}
		_, failure := NewExecutor(f).Execute(context.Background(), operation.Request{Name: OperationIncoming, Input: json.RawMessage(`{"session_id":"s","generation":1,"uri":"file:///w/a.go","symbol":"Target"}`)})
		if failure != nil || len(f.requests) < 2 || f.requests[0].Method != "textDocument/documentSymbol" || !strings.Contains(string(f.requests[1].Params), `"line":7,"character":3`) {
			t.Fatalf("%s: failure=%v requests=%v", assertion, failure, f.requests)
		}
	})
	for _, tc := range []struct {
		name, assertion, symbols, wantPosition string
	}{
		{
			name:         "gopls hierarchical document symbols",
			assertion:    "ASSERT_GOPLS_DOCUMENT_SYMBOL_HIERARCHY_SELECTION_START",
			symbols:      `[{"name":"Manager","detail":"struct{...}","kind":23,"tags":[],"deprecated":false,"range":{"start":{"line":2,"character":0},"end":{"line":12,"character":1}},"selectionRange":{"start":{"line":2,"character":5},"end":{"line":2,"character":12}},"children":[{"name":"Target","detail":"func()","kind":12,"tags":[],"deprecated":false,"range":{"start":{"line":7,"character":1},"end":{"line":9,"character":1}},"selectionRange":{"start":{"line":7,"character":3},"end":{"line":7,"character":9}}}]}]`,
			wantPosition: `"line":7,"character":3`,
		},
		{
			name:         "flat symbol information",
			assertion:    "ASSERT_SYMBOL_INFORMATION_LOCATION_START",
			symbols:      `[{"name":"Target","kind":12,"tags":[],"deprecated":false,"location":{"uri":"file:///w/a.go","range":{"start":{"line":7,"character":4},"end":{"line":9,"character":1}}},"containerName":"Manager"}]`,
			wantPosition: `"line":7,"character":4`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var specimen any
			if err := json.Unmarshal([]byte(tc.symbols), &specimen); err != nil {
				t.Fatalf("%s_SPECIMEN_VALID: %v", tc.assertion, err)
			}
			f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{PositionEncoding: "utf-16", CallHierarchySupport: true}, results: map[string][]json.RawMessage{"textDocument/documentSymbol": {json.RawMessage(tc.symbols)}, "textDocument/prepareCallHierarchy": {json.RawMessage(`[` + item + `]`)}, "callHierarchy/incomingCalls": {json.RawMessage(`[]`)}}}
			_, failure := NewExecutor(f).Execute(context.Background(), operation.Request{Name: OperationIncoming, Input: json.RawMessage(`{"session_id":"s","generation":1,"uri":"file:///w/a.go","symbol":"Target"}`)})
			if failure != nil || len(f.requests) < 2 || !strings.Contains(string(f.requests[1].Params), tc.wantPosition) {
				t.Fatalf("%s: failure=%v requests=%v", tc.assertion, failure, f.requests)
			}
		})
	}
}

func TestIncomingExplicitRelationComposition(t *testing.T) {
	const assertionSelection = "ASSERT_INCOMING_EXPLICIT_SELECTION_EXACT"
	const assertionKind = "ASSERT_INCOMING_PROVIDER_KIND_PRESERVED"
	const assertionReceipt = "ASSERT_INCOMING_PROVIDER_RECEIPT_FAILS_CLOSED"
	provider := json.RawMessage(`{"provider_id":"ember@1","terminal":"UNAVAILABLE","complete":false,"truncated":false,"bounds":{"max_relations":7,"max_source_bytes":99,"max_operations":3,"timeout_ms":50},"relations":[{"relation_id":"r1","kind":"PASSES_CALLBACK"}]}`)
	f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{PositionEncoding: "utf-16", CallHierarchySupport: true}, relationArtifact: provider}
	input := json.RawMessage(`{"session_id":"s","generation":1,"uri":"file:///w/a.gjs","line":0,"character":0,"relations":["PASSES_CALLBACK"],"max_depth":4,"max_nodes":20,"timeout_ms":1000,"request_timeout_ms":100}`)
	result, failure := NewExecutor(f).Execute(context.Background(), operation.Request{Name: OperationIncoming, Input: input})
	raw := string(result.Artifact)
	if failure != nil || len(f.calls) != 0 || len(f.relationCalls) != 1 || strings.Join(f.relationCalls[0], ",") != "PASSES_CALLBACK" {
		t.Fatalf("%s: failure=%v lsp_calls=%v relation_calls=%v", assertionSelection, failure, f.calls, f.relationCalls)
	}
	t.Log("PASS " + assertionSelection)
	if !strings.Contains(raw, `"kind":"PASSES_CALLBACK"`) || strings.Contains(raw, `"kind":"CALLS"`) {
		t.Fatalf("%s: artifact=%s", assertionKind, raw)
	}
	t.Log("PASS " + assertionKind)
	if !strings.Contains(raw, `"schema_version":"lsp-trace.graph.v4"`) || !strings.Contains(raw, `"provider_id":"ember@1"`) || !strings.Contains(raw, `"max_relations":7`) {
		t.Fatalf("%s: artifact=%s", assertionReceipt, raw)
	}
	t.Log("PASS " + assertionReceipt)
}

func TestIncomingProviderSuccessPreservesCallsGap(t *testing.T) {
	const assertion = "ASSERT_PROVIDER_SUCCESS_NEVER_OVERWRITES_CALLS_GAP"
	item := json.RawMessage(`[{"name":"root","kind":12,"uri":"file:///w/a.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}},"selectionRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}]`)
	provider := json.RawMessage(`{"provider_id":"source@1","terminal":"COMPLETE_WITHIN_BOUNDS","complete":true,"truncated":false,"bounds":{"max_relations":7},"relations":[{"relation_id":"r1","kind":"PASSES_CALLBACK"}]}`)
	f := &fakeRuntime{
		metadata:         sessionruntime.SessionMetadata{PositionEncoding: "utf-16", CallHierarchySupport: true},
		relationArtifact: provider,
		results:          map[string][]json.RawMessage{"textDocument/prepareCallHierarchy": {item}},
		observed:         map[string][]sessionruntime.RoundTripResult{"callHierarchy/incomingCalls": {{ServerError: &lspwire.RPCError{Code: -32603, Message: "calls gap"}}}},
	}
	input := json.RawMessage(`{"session_id":"s","generation":1,"uri":"file:///w/a.go","line":0,"character":0,"relations":["CALLS","PASSES_CALLBACK"],"max_depth":4,"max_nodes":20,"timeout_ms":1000,"request_timeout_ms":100}`)
	result, failure := NewExecutor(f).Execute(context.Background(), operation.Request{Name: OperationIncoming, Input: input})
	raw := string(result.Artifact)
	if failure != nil || !strings.Contains(raw, `"calls":`) || !strings.Contains(raw, "calls gap") || !strings.Contains(raw, `"provider_id":"source@1"`) || !strings.Contains(raw, `"complete":false`) {
		t.Fatalf("%s: failure=%v artifact=%s", assertion, failure, raw)
	}
	t.Log("PASS " + assertion)
}

var _ = lspwire.RequestKey{}
