package incomingops

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
	results  map[string][]json.RawMessage
	observed map[string][]sessionruntime.RoundTripResult
}

func (f *fakeRuntime) Metadata(string, uint64) (sessionruntime.SessionMetadata, session.Failure) {
	return f.metadata, f.failure
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

var _ = lspwire.RequestKey{}
