package structuralcontextsymbolops

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"lsp-trace/internal/lsp"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/session"
	"lsp-trace/internal/transientstructural"
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

func TestV2ExactWorkspaceSymbolDelegatesOriginalSymbolWithoutRangeStart(t *testing.T) {
	const assertion = "ASSERT_STRUCTURAL_CONTEXT_SYMBOL_V2_ONE_EXACT_LOOKUP_DELEGATES_V2_ORIGINAL_SYMBOL"
	f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{WorkspaceSymbolSupport: true}, result: json.RawMessage(`[{"name":"Target","kind":12,"location":{"uri":"file:///workspace/a.go","range":{"start":{"line":7,"character":3},"end":{"line":7,"character":9}}}},{"name":"target","kind":12,"location":{"uri":"file:///workspace/b.go","range":{"start":{"line":1,"character":0},"end":{"line":1,"character":1}}}}]`)}
	d := &delegate{}
	_, failure := NewV2Executor(f, d).Execute(context.Background(), operation.Request{Name: operation.Name("structural_context_symbol_v2"), Input: minimalInput("Target")})
	if failure != nil || len(f.requests) != 1 || f.requests[0].Method != "workspace/symbol" || len(d.calls) != 1 {
		t.Fatalf("%s: failure=%v requests=%v calls=%v", assertion, failure, f.requests, d.calls)
	}
	delegated := string(d.calls[0].Input)
	if d.calls[0].Name != "structural_context_v2" || !strings.Contains(delegated, `"uri":"file:///workspace/a.go"`) || !strings.Contains(delegated, `"symbol":"Target"`) || strings.Contains(delegated, `"line"`) || strings.Contains(delegated, `"character"`) {
		t.Fatalf("%s: delegated=%s", assertion, delegated)
	}
}

func TestUnifiedV2SymbolBranchUsesCanonicalOperationName(t *testing.T) {
	const assertion = "ASSERT_UNIFIED_CONTEXT_SYMBOL_BRANCH_DELEGATES_CANONICAL_V2"
	f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{WorkspaceSymbolSupport: true}, result: json.RawMessage(`[{"name":"Target","kind":12,"location":{"uri":"file:///workspace/a.go","range":{"start":{"line":7,"character":3},"end":{"line":7,"character":9}}}}]`)}
	d := &delegate{}
	projection := `{"mode":"TARGET","body":"OMIT","include_relation_occurrences":false,"include_ancillary":false,"limits":{"max_objects":1,"max_ranges":1,"max_source_bytes":0,"max_work":1,"max_response_bytes":4096},"privacy_policy_id":"public"}`
	input := json.RawMessage(`{"session_id":"s","generation":1,"symbol":"Target","projection":` + projection + `,"analysis":{"kind":"NEIGHBORHOOD"}}`)
	_, failure := NewUnifiedV2Executor(f, d).Execute(context.Background(), operation.Request{Name: operation.Name("structural_context_v2"), Input: input})
	if failure != nil || len(f.requests) != 1 || len(d.calls) != 1 || d.calls[0].Name != operation.Name("structural_context_v2") {
		t.Fatalf("%s: failure=%v requests=%v calls=%v", assertion, failure, f.requests, d.calls)
	}
	var delegated struct {
		Projection json.RawMessage `json:"projection"`
	}
	if json.Unmarshal(d.calls[0].Input, &delegated) != nil || string(delegated.Projection) != projection {
		t.Fatalf("%s_PROJECTION_PRESERVED: %s", assertion, d.calls[0].Input)
	}
}

func TestAmbiguousTargetCandidatesAreBoundedSortedDeduplicatedAndConfined(t *testing.T) {
	r := func(uri string, line uint32) lsp.WorkspaceSymbol {
		return lsp.WorkspaceSymbol{Name: "Target", Kind: 12, Location: lsp.Location{URI: uri, Range: &lsp.Range{Start: lsp.Position{Line: line}, End: lsp.Position{Line: line, Character: 1}}}}
	}
	matches := []lsp.WorkspaceSymbol{r("file:///workspace/z.go", 2), r("file:///workspace/a.go", 1), r("file:///workspace/a.go", 1), r("file:///other/x.go", 0)}
	diagnostic := ambiguousTargetDiagnostic("/workspace", matches, matches, 10000)
	if len(diagnostic.Candidates) != 2 || diagnostic.Candidates[0].URI != "file:///workspace/a.go" || diagnostic.Candidates[1].URI != "file:///workspace/z.go" {
		t.Fatalf("ASSERT_AMBIGUOUS_CANDIDATE_ORDER_AND_BOUND: %+v", diagnostic.Candidates)
	}
	accounting := diagnostic.CandidateAccounting
	if accounting == nil || accounting.Observed == nil || *accounting.Observed != 4 || accounting.Accepted != 3 || accounting.Returned != 2 || accounting.Excluded != 1 || accounting.Deduplicated != 1 || accounting.Truncated != 0 {
		t.Fatalf("ASSERT_AMBIGUOUS_CANDIDATE_ACCOUNTING: %+v", accounting)
	}
	many := make([]lsp.WorkspaceSymbol, 0, defaultMaxNodes+1)
	for i := 0; i <= defaultMaxNodes; i++ {
		many = append(many, r(fmt.Sprintf("file:///workspace/%03d.go", i), uint32(i)))
	}
	diagnostic = ambiguousTargetDiagnostic("/workspace", many, many, 10000)
	if len(diagnostic.Candidates) != defaultMaxNodes || diagnostic.CandidateAccounting.Truncated != 1 {
		t.Fatalf("ASSERT_AMBIGUOUS_CANDIDATE_PUBLIC_CAP: candidates=%d accounting=%+v", len(diagnostic.Candidates), diagnostic.CandidateAccounting)
	}
}

func TestUnifiedV2MalformedExactMatchDiagnosticIsActionableProjectionIndependentAndSafe(t *testing.T) {
	fixture, err := os.ReadFile("testdata/workspace-symbol-one-match-malformed-range.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, projection := range []string{"", `,"projection":{"mode":"TARGET","body":"OMIT","include_relation_occurrences":false,"include_ancillary":false,"limits":{"max_objects":1,"max_ranges":1,"max_source_bytes":0,"max_work":1,"max_response_bytes":4096},"privacy_policy_id":"public"}`} {
		f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{WorkspaceSymbolSupport: true}, result: fixture}
		d := &delegate{}
		in := json.RawMessage(`{"session_id":"s","generation":1,"symbol":"Runner","down_depth":0,"up_depth":0,"max_nodes":5` + projection + `,"analysis":{"kind":"NEIGHBORHOOD"}}`)
		_, failure := NewUnifiedV2Executor(f, d).Execute(context.Background(), operation.Request{Name: operation.Name("structural_context_v2"), Input: in})
		if failure == nil || failure.Code != "INVALID_SERVER_RESPONSE" || len(d.calls) != 0 {
			t.Fatalf("ASSERT_MALFORMED_EXACT_ONE_FAIL_CLOSED: failure=%+v calls=%d", failure, len(d.calls))
		}
		raw, _ := json.Marshal(failure.Err)
		var domain map[string]any
		if json.Unmarshal(raw, &domain) != nil {
			t.Fatalf("ASSERT_MALFORMED_EXACT_ONE_TYPED_DIAGNOSTIC: %s", raw)
		}
		diagnostic, _ := domain["target_diagnostic"].(map[string]any)
		for key, want := range map[string]any{"exact_matches": float64(1), "action": "FAIL_MALFORMED", "provider_method": "workspace/symbol", "item_index": float64(0), "normalization_stage": "POST_DECODE_NORMALIZATION", "failed_invariant": "RANGE_ORDER", "projection_entered": false} {
			if diagnostic[key] != want {
				t.Fatalf("ASSERT_MALFORMED_EXACT_ONE_ACTIONABLE_%s: diagnostic=%v", strings.ToUpper(key), diagnostic)
			}
		}
		recovery, _ := diagnostic["recovery"].(map[string]any)
		if recovery["kind"] != "REGEX_LOCATOR_TEMPLATE" || recovery["uri"] != "file:///workspace/runner.go" || recovery["required_pattern"] != true || recovery["match_index"] != float64(0) {
			t.Fatalf("ASSERT_MALFORMED_EXACT_ONE_REGEX_RECOVERY: %v", recovery)
		}
		for _, leaked := range []string{"Runner", `\"name\"`, `\"kind\"`, "raw_error", "source_body", "/Users/"} {
			if strings.Contains(string(raw), leaked) {
				t.Fatalf("ASSERT_MALFORMED_EXACT_ONE_NO_LEAK_%q: %s", leaked, raw)
			}
		}
	}
}

func TestUnifiedV2TruncatedSymbolEnumerationOffersExplicitRecovery(t *testing.T) {
	symbols := make([]map[string]any, 100)
	for i := range symbols {
		symbols[i] = map[string]any{"name": fmt.Sprintf("Other%d", i), "kind": 12, "location": map[string]any{"uri": fmt.Sprintf("file:///workspace/%03d.go", i), "range": map[string]any{"start": map[string]any{"line": 0, "character": 0}, "end": map[string]any{"line": 0, "character": 1}}}}
	}
	raw, _ := json.Marshal(symbols)
	f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{WorkspaceSymbolSupport: true}, result: raw}
	d := &delegate{}
	_, failure := NewUnifiedV2Executor(f, d).Execute(context.Background(), operation.Request{Name: operation.Name("structural_context_v2"), Input: minimalInput("Missing")})
	if failure == nil || failure.Code != "ENUMERATION_TRUNCATED" || len(d.calls) != 0 {
		t.Fatalf("ASSERT_TRUNCATED_SYMBOL_ENUMERATION_TYPED_RECOVERY: failure=%+v calls=%d", failure, len(d.calls))
	}
	body, _ := json.Marshal(failure.Err)
	text := string(body)
	for _, required := range []string{`"action":"ENUMERATION_TRUNCATED"`, `"total_symbols":100`, `"omitted_symbols":100`, `"locator_scope":"LSP_SYMBOLS"`, `"completeness":"UNKNOWN"`, `"kind":"POSITION_LOCATOR_TEMPLATE"`, `"kind":"REGEX_LOCATOR_TEMPLATE"`} {
		if !strings.Contains(text, required) {
			t.Fatalf("ASSERT_TRUNCATED_SYMBOL_ENUMERATION_TYPED_RECOVERY missing %s: %s", required, text)
		}
	}
}

func TestUnifiedV2AbsentSymbolExplainsRegexLocatorWithoutGuessingURI(t *testing.T) {
	f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{WorkspaceSymbolSupport: true}, result: json.RawMessage(`[]`)}
	_, failure := NewUnifiedV2Executor(f, &delegate{}).Execute(context.Background(), operation.Request{Name: operation.Name("structural_context_v2"), Input: minimalInput("nearest_outward_consumer")})
	if failure == nil || failure.Code != "TARGET_NOT_FOUND" {
		t.Fatalf("ASSERT_EXHAUSTIVE_ZERO_MATCH_REMAINS_TARGET_NOT_FOUND: %+v", failure)
	}
	if domain, ok := failure.Err.(*transientstructural.DomainFailure); !ok || domain.TargetDiagnostic == nil || domain.TargetDiagnostic.Action != transientstructural.TargetActionFailAbsent || domain.TargetDiagnostic.OmittedSymbols != 0 {
		t.Fatalf("ASSERT_EXHAUSTIVE_ZERO_MATCH_REMAINS_TARGET_NOT_FOUND: %+v", failure.Err)
	}
	raw, _ := json.Marshal(failure.Err)
	text := string(raw)
	for _, required := range []string{"no exact LSP symbol match was reported", "does not establish source absence", "provide regex_locator.uri and regex_locator.pattern", "LSP_SYMBOLS", "regex_locator", "uri", "pattern", "match_index", "max_document_bytes", "max_pattern_bytes", "max_work", "max_matches"} {
		if !strings.Contains(text, required) {
			t.Fatalf("ASSERT_TARGET_NOT_FOUND_GUIDANCE_%s: %s", strings.ToUpper(required), text)
		}
	}
	if strings.Contains(text, "file://") || strings.Contains(text, "nearest_outward_consumer") {
		t.Fatalf("ASSERT_TARGET_NOT_FOUND_NO_GUESSED_URI_OR_SYMBOL: %s", text)
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
			raw, err := json.Marshal(failure)
			if err != nil {
				t.Fatal(err)
			}
			for _, forbidden := range []string{"Target", "target", "file:", "/workspace", "line", "character", "selector", "provider", "environment"} {
				if strings.Contains(string(raw), forbidden) {
					t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_SYMBOL_FAILURE_DIAGNOSTICS_SANITIZED_%s: leaked %q in %s", tc.code, forbidden, raw)
				}
			}
		})
	}
}
