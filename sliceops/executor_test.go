package sliceops

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

type fakeRuntime struct {
	metadata sessionruntime.SessionMetadata
	failure  session.Failure
	calls    []string
	requests []sessionruntime.RoundTripRequest
	results  map[string]sessionruntime.RoundTripResult
	respond  func(sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult
}

func (f *fakeRuntime) Records() []sessionruntime.Record { return nil }
func (f *fakeRuntime) Metadata(string, uint64) (sessionruntime.SessionMetadata, session.Failure) {
	return f.metadata, f.failure
}
func (f *fakeRuntime) RoundTrip(_ context.Context, r sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult {
	var p struct {
		Item struct {
			Name string `json:"name"`
		} `json:"item"`
	}
	_ = json.Unmarshal(r.Params, &p)
	key := r.Method + ":" + p.Item.Name
	f.calls = append(f.calls, key)
	f.requests = append(f.requests, r)
	if f.respond != nil {
		return f.respond(r)
	}
	if result, ok := f.results[key]; ok {
		return result
	}
	return sessionruntime.RoundTripResult{Failure: session.RequestTimeout}
}

func validInput() json.RawMessage {
	return json.RawMessage(`{"session_id":"s","generation":1,"start_mode":"at","uri":"file:///w/a.go","line":0,"character":0,"down_depth":2,"up_depth":2,"max_nodes":20,"max_messages":64,"max_bytes":4194304,"timeout_ms":1000,"request_timeout_ms":100}`)
}

func item(name string, line int) string {
	return `{"name":"` + name + `","kind":12,"uri":"file:///w/a.go","range":{"start":{"line":` + string(rune('0'+line)) + `,"character":0},"end":{"line":` + string(rune('0'+line)) + `,"character":1}},"selectionRange":{"start":{"line":` + string(rune('0'+line)) + `,"character":0},"end":{"line":` + string(rune('0'+line)) + `,"character":1}}}`
}

func TestSliceAppliesConservativeDefaults(t *testing.T) {
	const assertion = "ASSERT_SLICE_CONSERVATIVE_DEFAULTS"
	t.Log("ASSERTION: " + assertion)
	root := item("root", 0)
	f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{PositionEncoding: "utf-16", CallHierarchySupport: true}, results: map[string]sessionruntime.RoundTripResult{
		"textDocument/prepareCallHierarchy:": {Result: json.RawMessage(`[` + root + `]`)},
		"callHierarchy/outgoingCalls:root":   {Result: json.RawMessage(`[]`)},
		"callHierarchy/incomingCalls:root":   {Result: json.RawMessage(`[]`)},
	}}
	result, failure := NewExecutor(f).Execute(context.Background(), operation.Request{Name: OperationSlice, Input: json.RawMessage(`{"session_id":"s","generation":1,"start_mode":"at","uri":"file:///w/a.go","line":0,"character":0}`)})
	if failure != nil {
		t.Fatalf("%s: %v", assertion, failure)
	}
	var got graph.Result
	if err := json.Unmarshal(result.Artifact, &got); err != nil || got.Invocation.Limits.MaxNodes <= 0 || got.Invocation.Limits.TimeoutMS <= 0 || got.Invocation.RequestTimeoutMS <= 0 || len(f.requests) == 0 || f.requests[0].MaxMessages <= 0 || f.requests[0].MaxBytes <= 0 {
		t.Fatalf("%s: limits=%+v request_timeout=%d requests=%+v err=%v", assertion, got.Invocation.Limits, got.Invocation.RequestTimeoutMS, f.requests, err)
	}
	t.Log("PASS " + assertion)
}

func TestSliceExactFrontierLeavesAndUpwardUnion(t *testing.T) {
	for _, assertion := range []string{"ASSERT_SLICE_EXACT_DEPTH_FRONTIER", "ASSERT_SLICE_FAILED_NULL_NOT_LEAF", "ASSERT_SLICE_UPWARD_SORTED_DEDUP_UNION", "ASSERT_SLICE_CAUSAL_CLOSURE_SEED_MEMBERSHIP"} {
		t.Log("ASSERTION: " + assertion)
	}
	root, mid, leaf, deep := item("root", 0), item("mid", 1), item("leaf", 2), item("deep", 3)
	f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{PositionEncoding: "utf-16", CallHierarchySupport: true}, results: map[string]sessionruntime.RoundTripResult{
		"textDocument/prepareCallHierarchy:": {Result: json.RawMessage(`[` + root + `]`)},
		"callHierarchy/outgoingCalls:root":   {Result: json.RawMessage(`[{"to":` + leaf + `,"fromRanges":[]},{"to":` + mid + `,"fromRanges":[]}]`)},
		"callHierarchy/outgoingCalls:leaf":   {Result: json.RawMessage(`[]`)},
		"callHierarchy/outgoingCalls:mid":    {Result: json.RawMessage(`[{"to":` + deep + `,"fromRanges":[]}]`)},
		"callHierarchy/incomingCalls:deep":   {Result: json.RawMessage(`[]`)},
		"callHierarchy/incomingCalls:leaf":   {Result: json.RawMessage(`[]`)},
	}}
	result, failure := NewExecutor(f).Execute(context.Background(), operation.Request{Name: OperationSlice, Input: validInput()})
	if failure != nil {
		t.Fatalf("ASSERT_SLICE_EXACT_DEPTH_FRONTIER: %v", failure)
	}
	var got struct {
		Slice *graph.SliceEvidence `json:"slice"`
	}
	if err := json.Unmarshal(result.Artifact, &got); err != nil {
		t.Fatal(err)
	}
	if got.Slice == nil || len(got.Slice.FrontierNodeIDs) != 1 || len(got.Slice.OutgoingTerminalNodeIDs) != 1 || len(got.Slice.UpwardStartNodeIDs) != 2 {
		t.Fatalf("ASSERT_SLICE_UPWARD_SORTED_DEDUP_UNION: slice=%+v", got.Slice)
	}
	if !strings.Contains(strings.Join(f.calls, ","), "callHierarchy/incomingCalls:deep") || !strings.Contains(strings.Join(f.calls, ","), "callHierarchy/incomingCalls:leaf") {
		t.Fatalf("ASSERT_SLICE_CAUSAL_CLOSURE_SEED_MEMBERSHIP: calls=%v", f.calls)
	}
}

func TestSliceReconcilesIncomingAliasToOutgoingPresentation(t *testing.T) {
	const assertion = "ASSERT_MANAGED_SLICE_UNIFIED_SYMBOL_IDENTITY"
	t.Log("ASSERTION: " + assertion)
	outgoingRoot := `{"name":"root","kind":12,"uri":"file:///w/a.go","range":{"start":{"line":0,"character":0},"end":{"line":2,"character":1}},"selectionRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":4}},"data":{"surface":"outgoing"}}`
	incomingRoot := `{"name":"root","kind":12,"uri":"file:///w/a.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":4}},"selectionRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":4}},"data":{"surface":"incoming"}}`
	leaf := item("leaf", 1)
	f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{PositionEncoding: "utf-16", CallHierarchySupport: true}, results: map[string]sessionruntime.RoundTripResult{
		"textDocument/prepareCallHierarchy:": {Result: json.RawMessage(`[` + outgoingRoot + `]`)},
		"callHierarchy/outgoingCalls:root":   {Result: json.RawMessage(`[{"to":` + leaf + `,"fromRanges":[]}]`)},
		"callHierarchy/outgoingCalls:leaf":   {Result: json.RawMessage(`[]`)},
		"callHierarchy/incomingCalls:leaf":   {Result: json.RawMessage(`[{"from":` + incomingRoot + `,"fromRanges":[]}]`)},
		"callHierarchy/incomingCalls:root":   {Result: json.RawMessage(`[]`)},
	}}
	result, failure := NewExecutor(f).Execute(context.Background(), operation.Request{Name: OperationSlice, Input: validInput()})
	if failure != nil {
		t.Fatalf("%s: %v", assertion, failure)
	}
	var got graph.Result
	if err := json.Unmarshal(result.Artifact, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Nodes) != 2 || len(got.Edges) != 1 {
		t.Fatalf("%s: nodes=%d edges=%d artifact=%s", assertion, len(got.Nodes), len(got.Edges), result.Artifact)
	}
	for _, n := range got.Nodes {
		if n.Name == "root" && (n.Range.End.Line != 2 || !strings.Contains(string(n.Data), "outgoing")) {
			t.Fatalf("%s: root=%+v", assertion, n)
		}
	}
}

func TestSliceWireBoundsReachEveryRoundTrip(t *testing.T) {
	const assertion = "ASSERT_SLICE_PER_WIRE_REQUEST_BOUNDS"
	t.Log("ASSERTION: " + assertion)
	root, leaf := item("root", 0), item("leaf", 1)
	f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{PositionEncoding: "utf-16", CallHierarchySupport: true}, results: map[string]sessionruntime.RoundTripResult{
		"textDocument/prepareCallHierarchy:": {Result: json.RawMessage(`[` + root + `]`)},
		"callHierarchy/outgoingCalls:root":   {Result: json.RawMessage(`[{"to":` + leaf + `,"fromRanges":[]}]`)},
		"callHierarchy/incomingCalls:leaf":   {Result: json.RawMessage(`[]`)},
	}}
	input := json.RawMessage(`{"session_id":"s","generation":1,"start_mode":"at","uri":"file:///w/a.go","line":0,"character":0,"down_depth":1,"up_depth":1,"max_nodes":20,"max_messages":7,"max_bytes":12345,"timeout_ms":1000,"request_timeout_ms":100}`)
	if _, failure := NewExecutor(f).Execute(context.Background(), operation.Request{Name: OperationSlice, Input: input}); failure != nil {
		t.Fatalf("%s: %v", assertion, failure)
	}
	if len(f.requests) != 3 {
		t.Fatalf("%s: requests=%+v", assertion, f.requests)
	}
	for _, request := range f.requests {
		if request.MaxMessages != 7 || request.MaxBytes != 12345 {
			t.Fatalf("%s: method=%s max_messages=%d max_bytes=%d", assertion, request.Method, request.MaxMessages, request.MaxBytes)
		}
	}
}

func TestSliceWireBoundsAffectOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name      string
		assertion string
		input     json.RawMessage
		matches   func(sessionruntime.RoundTripRequest) bool
	}{
		{"messages", "ASSERT_SLICE_MAX_MESSAGES_BOUNDED_OUTCOME", json.RawMessage(`{"session_id":"s","generation":1,"start_mode":"at","uri":"file:///w/a.go","line":0,"character":0,"down_depth":1,"up_depth":1,"max_nodes":20,"max_messages":1,"max_bytes":4194304,"timeout_ms":1000,"request_timeout_ms":100}`), func(r sessionruntime.RoundTripRequest) bool { return r.MaxMessages == 1 }},
		{"bytes", "ASSERT_SLICE_MAX_BYTES_BOUNDED_OUTCOME", json.RawMessage(`{"session_id":"s","generation":1,"start_mode":"at","uri":"file:///w/a.go","line":0,"character":0,"down_depth":1,"up_depth":1,"max_nodes":20,"max_messages":64,"max_bytes":1024,"timeout_ms":1000,"request_timeout_ms":100}`), func(r sessionruntime.RoundTripRequest) bool { return r.MaxBytes == 1024 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Log("ASSERTION: " + tc.assertion)
			f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{PositionEncoding: "utf-16", CallHierarchySupport: true}}
			f.respond = func(r sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult {
				if tc.matches(r) {
					return sessionruntime.RoundTripResult{Failure: session.ResourceExhausted}
				}
				return sessionruntime.RoundTripResult{Result: json.RawMessage(`[]`)}
			}
			result, failure := NewExecutor(f).Execute(context.Background(), operation.Request{Name: OperationSlice, Input: tc.input})
			if failure != nil || !strings.Contains(string(result.Artifact), string(session.ResourceExhausted)) {
				t.Fatalf("%s: failure=%v requests=%+v artifact=%s", tc.assertion, failure, f.requests, result.Artifact)
			}
		})
	}
}

func TestSliceRejectsBeforeEffects(t *testing.T) {
	const assertion = "ASSERT_SLICE_VALIDATE_BEFORE_EFFECTS"
	t.Log("ASSERTION: " + assertion)
	f := &fakeRuntime{}
	_, failure := NewExecutor(f).Execute(context.Background(), operation.Request{Name: OperationSlice, Input: json.RawMessage(`{"session_id":"","generation":0}`)})
	if failure == nil || failure.Code != operation.FailureInvalidInput || len(f.calls) != 0 {
		t.Fatalf("%s: failure=%v calls=%v", assertion, failure, f.calls)
	}
}

func TestSliceServerErrorIsPartialNotLeaf(t *testing.T) {
	const assertion = "ASSERT_SLICE_FAILED_NULL_NOT_LEAF"
	t.Log("ASSERTION: " + assertion)
	root := item("root", 0)
	f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{PositionEncoding: "utf-8", CallHierarchySupport: true}, results: map[string]sessionruntime.RoundTripResult{
		"textDocument/prepareCallHierarchy:": {Result: json.RawMessage(`[` + root + `]`)},
		"callHierarchy/outgoingCalls:root":   {ServerError: &lspwire.RPCError{Code: -32603, Message: "boom"}},
	}}
	result, failure := NewExecutor(f).Execute(context.Background(), operation.Request{Name: OperationSlice, Input: validInput()})
	if failure != nil || !strings.Contains(string(result.Artifact), "boom") || strings.Contains(string(result.Artifact), `"outgoing_terminal_node_ids":[`) {
		t.Fatalf("%s: failure=%v artifact=%s", assertion, failure, result.Artifact)
	}
}
