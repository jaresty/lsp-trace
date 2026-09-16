package structuralcontextsymbolops

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/internal/operation"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

type fakeRuntime struct {
	result   json.RawMessage
	requests []sessionruntime.RoundTripRequest
	metadata sessionruntime.SessionMetadata
}

func (f *fakeRuntime) Metadata(string, uint64) (sessionruntime.SessionMetadata, session.Failure) {
	return f.metadata, ""
}
func (f *fakeRuntime) Records() []sessionruntime.Record {
	return []sessionruntime.Record{{SessionID: "s", Generation: 1, State: session.Ready, Routing: sessionruntime.RoutingMetadata{WorkspaceRoot: "/workspace"}}}
}
func (f *fakeRuntime) RoundTrip(_ context.Context, r sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult {
	f.requests = append(f.requests, r)
	return sessionruntime.RoundTripResult{Result: f.result}
}

type delegate struct{ calls []operation.Request }

func (d *delegate) Execute(_ context.Context, r operation.Request) (operation.Result, *operation.Failure) {
	d.calls = append(d.calls, r)
	return operation.Result{Artifact: []byte(`{"state":"EMPTY"}`)}, nil
}
func input(symbol string) json.RawMessage {
	return json.RawMessage(`{"session_id":"s","generation":1,"symbol":"` + symbol + `","down_depth":2,"up_depth":2,"max_nodes":100,"timeout_ms":5000,"request_timeout_ms":1000,"analysis":{"kind":"NEIGHBORHOOD"}}`)
}
func minimalInput(symbol string) json.RawMessage {
	return json.RawMessage(`{"session_id":"s","generation":1,"symbol":"` + symbol + `","analysis":{"kind":"NEIGHBORHOOD"}}`)
}
func zeroDepthInput(symbol string) json.RawMessage {
	return json.RawMessage(`{"session_id":"s","generation":1,"symbol":"` + symbol + `","down_depth":0,"up_depth":0,"analysis":{"kind":"NEIGHBORHOOD"}}`)
}

func TestExactWorkspaceSymbolDelegatesOneConcreteLocator(t *testing.T) {
	const assertion = "ASSERT_STRUCTURAL_CONTEXT_SYMBOL_ONE_EXACT_LOOKUP_DELEGATES_SYMBOL_IN_DOCUMENT"
	f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{WorkspaceSymbolSupport: true}, result: json.RawMessage(`[{"name":"Target","kind":12,"location":{"uri":"file:///workspace/a.go","range":{"start":{"line":7,"character":3},"end":{"line":7,"character":9}}}},{"name":"target","kind":12,"location":{"uri":"file:///workspace/b.go","range":{"start":{"line":1,"character":0},"end":{"line":1,"character":1}}}}]`)}
	d := &delegate{}
	_, failure := NewExecutor(f, d).Execute(context.Background(), operation.Request{Name: Operation, Input: input("Target")})
	if failure != nil || len(f.requests) != 1 || f.requests[0].Method != "workspace/symbol" || len(d.calls) != 1 {
		t.Fatalf("%s: failure=%v requests=%v calls=%v", assertion, failure, f.requests, d.calls)
	}
	delegated := string(d.calls[0].Input)
	if d.calls[0].Name != "structural_context" || !strings.Contains(delegated, `"uri":"file:///workspace/a.go"`) || !strings.Contains(delegated, `"symbol":"Target"`) || strings.Contains(delegated, `"line"`) || strings.Contains(delegated, `"character"`) {
		t.Fatalf("%s: delegated=%s", assertion, delegated)
	}
}

func TestOmittedMechanicalBoundsDelegateCanonicalDefaults(t *testing.T) {
	const assertion = "ASSERT_STRUCTURAL_CONTEXT_SYMBOL_MECHANICAL_BOUNDS_DEFAULT"
	f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{WorkspaceSymbolSupport: true}, result: json.RawMessage(`[{"name":"Target","kind":12,"location":{"uri":"file:///workspace/a.go","range":{"start":{"line":7,"character":3},"end":{"line":7,"character":9}}}}]`)}
	d := &delegate{}
	_, failure := NewExecutor(f, d).Execute(context.Background(), operation.Request{Name: Operation, Input: minimalInput("Target")})
	if failure != nil || len(d.calls) != 1 {
		t.Fatalf("%s: failure=%v calls=%v", assertion, failure, d.calls)
	}
	got := string(d.calls[0].Input)
	for _, field := range []string{`"down_depth":2`, `"up_depth":2`, `"max_nodes":100`, `"timeout_ms":5000`, `"request_timeout_ms":1000`, `"max_messages":64`, `"max_bytes":4194304`} {
		if !strings.Contains(got, field) {
			t.Fatalf("%s: missing %s in %s", assertion, field, got)
		}
	}

	d = &delegate{}
	_, failure = NewExecutor(f, d).Execute(context.Background(), operation.Request{Name: Operation, Input: zeroDepthInput("Target")})
	if failure != nil || len(d.calls) != 1 || !strings.Contains(string(d.calls[0].Input), `"down_depth":0`) || !strings.Contains(string(d.calls[0].Input), `"up_depth":0`) {
		t.Fatalf("%s_EXPLICIT_ZERO_PRESERVED: failure=%v calls=%v", assertion, failure, d.calls)
	}
}

func TestWorkspaceSymbolFailuresAreExplicitAndDoNotDelegate(t *testing.T) {
	cases := []struct{ name, result, code string }{{"absent", `[{"name":"target","kind":12,"location":{"uri":"file:///workspace/a.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}}]`, "WORKSPACE_SYMBOL_ABSENT"}, {"ambiguous", `[{"name":"Target","kind":12,"location":{"uri":"file:///workspace/a.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}},{"name":"Target","kind":12,"location":{"uri":"file:///workspace/b.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}}]`, "WORKSPACE_SYMBOL_AMBIGUOUS"}, {"unresolved", `[{"name":"Target","kind":12,"location":{"uri":"file:///workspace/a.go"}}]`, "WORKSPACE_SYMBOL_MALFORMED"}, {"root", `[{"name":"Target","kind":12,"location":{"uri":"file:///workspace","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}}]`, "WORKSPACE_SYMBOL_OUTSIDE_WORKSPACE"}, {"outside", `[{"name":"Target","kind":12,"location":{"uri":"file:///other/a.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}}]`, "WORKSPACE_SYMBOL_OUTSIDE_WORKSPACE"}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{WorkspaceSymbolSupport: true}, result: json.RawMessage(tc.result)}
			d := &delegate{}
			_, failure := NewExecutor(f, d).Execute(context.Background(), operation.Request{Name: Operation, Input: input("Target")})
			if failure == nil || failure.Code != tc.code || len(f.requests) != 1 || len(d.calls) != 0 {
				t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_SYMBOL_EXPLICIT_FAIL_CLOSED_%s: failure=%v requests=%d calls=%d", tc.code, failure, len(f.requests), len(d.calls))
			}
		})
	}
}
