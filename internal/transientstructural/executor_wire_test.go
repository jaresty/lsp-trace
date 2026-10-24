package transientstructural

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

type structuralWire struct {
	in               *io.PipeReader
	stdin            *io.PipeWriter
	out              *io.PipeWriter
	stdout           *io.PipeReader
	uri              string
	mu               sync.Mutex
	methods          []string
	preparePositions []string
	responses        map[string]json.RawMessage
}

func newStructuralWire(uri string, responses map[string]json.RawMessage) *structuralWire {
	in, stdin := io.Pipe()
	stdout, out := io.Pipe()
	wire := &structuralWire{in: in, stdin: stdin, out: out, stdout: stdout, uri: uri, responses: responses}
	go wire.serve()
	return wire
}

func (w *structuralWire) serve() {
	reader := lspwire.NewReader(w.in, lspwire.DefaultLimits())
	writer := lspwire.NewWriter(w.out, lspwire.DefaultLimits())
	for {
		message, err := reader.Read()
		if err != nil {
			return
		}
		w.mu.Lock()
		w.methods = append(w.methods, message.Method)
		override, overridden := w.responses[message.Method]
		w.mu.Unlock()
		result := json.RawMessage(`[]`)
		switch message.Method {
		case "initialize":
			result = json.RawMessage(`{"capabilities":{"callHierarchyProvider":true,"documentSymbolProvider":true,"positionEncoding":"utf-16"}}`)
		case "initialized", "textDocument/didOpen", "textDocument/didChange":
			continue
		case "textDocument/documentSymbol":
			outer := map[string]any{"name": "Outer", "kind": 5, "range": w.positionRange(0, 0, 12), "selectionRange": w.positionRange(0, 5, 10), "children": []any{w.documentSymbol("F", 1)}}
			result, _ = json.Marshal([]any{outer})
		case "textDocument/prepareCallHierarchy":
			var params struct {
				Position struct {
					Line      uint32 `json:"line"`
					Character uint32 `json:"character"`
				} `json:"position"`
			}
			_ = json.Unmarshal(message.Params, &params)
			w.mu.Lock()
			w.preparePositions = append(w.preparePositions, fmt.Sprintf("%d:%d", params.Position.Line, params.Position.Character))
			w.mu.Unlock()
			result, _ = json.Marshal([]any{w.item("F", 1)})
		case "callHierarchy/outgoingCalls":
			var params struct {
				Item struct {
					Name string `json:"name"`
				} `json:"item"`
			}
			_ = json.Unmarshal(message.Params, &params)
			if params.Item.Name == "F" {
				result, _ = json.Marshal([]any{map[string]any{"to": w.item("G", 2), "fromRanges": []any{w.positionRange(1, 7, 8)}}})
			}
		case "callHierarchy/incomingCalls":
			var params struct {
				Item struct {
					Name string `json:"name"`
				} `json:"item"`
			}
			_ = json.Unmarshal(message.Params, &params)
			if params.Item.Name == "F" {
				result, _ = json.Marshal([]any{map[string]any{"from": w.item("H", 3), "fromRanges": []any{w.positionRange(3, 7, 8)}}})
			}
		case "shutdown":
			result = json.RawMessage(`null`)
		case "exit":
			return
		}
		if overridden {
			result = append(json.RawMessage(nil), override...)
		}
		if len(message.ID) != 0 {
			if err := writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: message.ID, Result: result}); err != nil {
				return
			}
		}
	}
}

func (w *structuralWire) item(name string, line int) map[string]any {
	return map[string]any{
		"name": name, "kind": 12, "uri": w.uri,
		"range":          w.positionRange(line, 0, 12),
		"selectionRange": w.positionRange(line, 5, 6),
	}
}

func (w *structuralWire) documentSymbol(name string, line int) map[string]any {
	return map[string]any{"name": name, "kind": 12, "range": w.positionRange(line, 0, 12), "selectionRange": w.positionRange(line, 5, 6)}
}

func (w *structuralWire) positionRange(line, start, end int) map[string]any {
	return map[string]any{"start": map[string]int{"line": line, "character": start}, "end": map[string]int{"line": line, "character": end}}
}

func (w *structuralWire) Stdin() io.WriteCloser { return w.stdin }
func (w *structuralWire) Stdout() io.ReadCloser { return w.stdout }
func (w *structuralWire) resetMethods() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.methods = nil
}
func (w *structuralWire) observedMethods() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.methods...)
}
func (w *structuralWire) observedPreparePositions() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.preparePositions...)
}
func (w *structuralWire) Teardown(context.Context) managedprocess.TeardownObservation {
	_ = w.stdin.Close()
	_ = w.in.Close()
	_ = w.out.Close()
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}
func (w *structuralWire) Close() managedprocess.ResourceObservation {
	_ = w.stdout.Close()
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}

type structuralStarter struct {
	uri       string
	responses map[string]json.RawMessage
	mu        sync.Mutex
	children  []*structuralWire
}

func (s *structuralStarter) Start(context.Context, managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	child := newStructuralWire(s.uri, s.responses)
	s.mu.Lock()
	s.children = append(s.children, child)
	s.mu.Unlock()
	return child, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
}

func structuralManager(t *testing.T) (*sessionruntime.Manager, sessionruntime.StartResult, string) {
	t.Helper()
	manager, started, uri, _ := structuralManagerWithResponses(t, nil)
	return manager, started, uri
}

func structuralManagerWithResponses(t *testing.T, build func(string) map[string]json.RawMessage) (*sessionruntime.Manager, sessionruntime.StartResult, string, *structuralStarter) {
	t.Helper()
	workspace := t.TempDir()
	path := filepath.Join(workspace, "code.go")
	if err := os.WriteFile(path, []byte("package p\nfunc F() {}\nfunc G() {}\nfunc H() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
	profile, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "transient-test", Workspace: workspace, Profile: "fake", EnvironmentReference: "test"})
	if err != nil {
		t.Fatal(err)
	}
	var responses map[string]json.RawMessage
	if build != nil {
		responses = build(uri)
	}
	starter := &structuralStarter{uri: uri, responses: responses}
	manager, err := sessionruntime.New(sessionruntime.Config{
		Limits:  sessionruntime.Limits{MaxSessions: 1, MaxRequests: 4, MaxChildren: 2, MaxCancels: 4, MaxTombstones: 8, MaxObservations: 64, MaxOperations: 8},
		Starter: starter,
	})
	if err != nil {
		t.Fatal(err)
	}
	started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(profile), BootstrapAlias: "stable-alias", LanguageID: "go"})
	pending := manager.BeginReadiness(context.Background(), started.SessionID, started.Generation, time.Now().Add(time.Second))
	ready, ok := manager.WaitReadiness(context.Background(), pending.ID)
	if !ok || ready.State != sessionruntime.ReadinessReady {
		t.Fatalf("readiness failed: %+v", ready)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if methods := starter.children[0].observedMethods(); len(methods) != 0 && methods[len(methods)-1] == "initialized" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	starter.children[0].resetMethods()
	t.Cleanup(func() {
		_ = manager.Stop(context.Background(), started.SessionID, "transient-test-cleanup")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = manager.Shutdown(ctx)
	})
	return manager, started, uri, starter
}

func TestSourceOnlyTargetUsesDocumentSymbolsWithoutCallHierarchy(t *testing.T) {
	manager, started, uri, starter := structuralManagerWithResponses(t, func(string) map[string]json.RawMessage {
		return map[string]json.RawMessage{
			"initialize": json.RawMessage(`{"capabilities":{"documentSymbolProvider":true,"definitionProvider":true,"referencesProvider":true,"positionEncoding":"utf-16"}}`),
		}
	})
	line, character := uint32(1), uint32(7)
	result, failure := Execute(context.Background(), manager, Request{
		SessionID: "stable-alias", Generation: started.Generation, LanguageID: "cue",
		Target: Target{URI: uri, Line: &line, Character: &character}, SourceOnlyTarget: true,
		UpDepth: 0, DownDepth: 0, MaxNodes: 5, TimeoutMS: 1000, RequestTimeoutMS: 250, MaxMessages: 8, MaxBytes: 8192,
		Analysis: AnalysisRequest{Kind: AnalysisNeighborhood}, CaptureSupply: true,
	})
	if failure != nil || result.State != StateComplete || len(result.Analysis.Nodes) != 1 || len(result.Analysis.Occurrences) != 0 || !strings.Contains(result.ClaimCeiling, "establishes no callable identity") {
		t.Fatalf("ASSERT_SOURCE_ONLY_TARGET_DOCUMENT_SYMBOL_SUCCESS: result=%+v failure=%+v", result, failure)
	}
	methods := starter.children[0].observedMethods()
	if !slices.Contains(methods, "textDocument/documentSymbol") {
		t.Fatalf("ASSERT_SOURCE_ONLY_TARGET_DOCUMENT_SYMBOL_REQUESTED: methods=%v", methods)
	}
	for _, forbidden := range []string{"workspace/symbol", "textDocument/prepareCallHierarchy", "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls", "textDocument/definition", "textDocument/references"} {
		if slices.Contains(methods, forbidden) {
			t.Fatalf("ASSERT_SOURCE_ONLY_TARGET_NO_RELATION_OR_WORKSPACE_REQUESTS: forbidden=%s methods=%v", forbidden, methods)
		}
	}
}

func TestUnsupportedCallHierarchyDiagnosticReturnsSourceOnlyRecoveryByLocatorMode(t *testing.T) {
	for _, tc := range []struct {
		name       string
		target     func(string) Target
		locatorKey string
		want       map[string]any
	}{
		{name: "position", target: func(uri string) Target {
			line, character := uint32(1), uint32(7)
			return Target{URI: uri, Line: &line, Character: &character}
		}, want: map[string]any{"line": 1, "character": 7}},
		{name: "symbol-uri", target: func(uri string) Target { return Target{URI: uri, Symbol: "ExactSymbol"} }, want: map[string]any{"symbol": "ExactSymbol"}},
		{name: "regex", target: func(uri string) Target {
			return Target{URI: uri, Regex: &RegexLocator{Pattern: `func (Exact)`, MatchIndex: 2, CaptureGroup: 1, ExpectedDigest: "sha256:" + strings.Repeat("a", 64), MaxDocumentBytes: 4096, MaxMatches: 10, MaxPatternBytes: 128, MaxWork: 8192}}
		}, locatorKey: "regex_locator", want: map[string]any{"pattern": `func (Exact)`, "match_index": 2, "capture_group": 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manager, started, uri, _ := structuralManagerWithResponses(t, func(string) map[string]json.RawMessage {
				return map[string]json.RawMessage{"initialize": json.RawMessage(`{"capabilities":{"documentSymbolProvider":true,"definitionProvider":true,"referencesProvider":true,"positionEncoding":"utf-16"}}`)}
			})
			_, failure := Execute(context.Background(), manager, Request{SessionID: "stable-alias", Generation: started.Generation, Target: tc.target(uri), UpDepth: 1, DownDepth: 1, MaxNodes: 5, TimeoutMS: 1000, RequestTimeoutMS: 250, MaxMessages: 8, MaxBytes: 8192, Analysis: AnalysisRequest{Kind: AnalysisNeighborhood}})
			if failure == nil || failure.State != StateUnsupported || failure.TargetDiagnostic == nil {
				t.Fatalf("ASSERT_CALL_HIERARCHY_UNSUPPORTED_TYPED: %+v", failure)
			}
			diagnostic := failure.TargetDiagnostic
			for _, want := range []string{"failed_capability=textDocument/prepareCallHierarchy", "textDocument/documentSymbol", "textDocument/definition", "textDocument/references", "workspace_symbol_dependency=false", "definition and references are non-CALLS"} {
				if !strings.Contains(diagnostic.Guidance, want) {
					t.Fatalf("ASSERT_CALL_HIERARCHY_CAPABILITY_DIAGNOSTIC_%q: %+v", want, diagnostic)
				}
			}
			recovery := diagnostic.Recovery
			if recovery == nil || recovery.Kind != "SOURCE_ONLY_REQUEST_TEMPLATE" || recovery.Complete == nil || *recovery.Complete || !reflect.DeepEqual(recovery.OmittedFields, []string{"projection.privacy_policy_id"}) {
				t.Fatalf("ASSERT_CALL_HIERARCHY_RECOVERY_CLOSED_TEMPLATE: %+v", recovery)
			}
			fragment := recovery.RequestFragment
			if fragment["up_depth"] != 0 || fragment["down_depth"] != 0 {
				t.Fatalf("ASSERT_CALL_HIERARCHY_RECOVERY_ZERO_DEPTH: %#v", fragment)
			}
			projection := fragment["projection"].(map[string]any)
			if projection["mode"] != "TARGET" || projection["include_relation_occurrences"] != false || projection["body"] != "INCLUDE" {
				t.Fatalf("ASSERT_CALL_HIERARCHY_RECOVERY_TARGET_PROJECTION: %#v", projection)
			}
			locator := fragment
			if tc.locatorKey != "" {
				locator = fragment[tc.locatorKey].(map[string]any)
			}
			if locator["uri"] != uri {
				t.Fatalf("ASSERT_CALL_HIERARCHY_RECOVERY_URI_%s: %#v", tc.name, locator)
			}
			for field, want := range tc.want {
				if locator[field] != want {
					t.Fatalf("ASSERT_CALL_HIERARCHY_RECOVERY_LOCATOR_%s_%s: got=%#v want=%#v", tc.name, field, locator[field], want)
				}
			}
		})
	}
}

func TestSourceOnlyTargetLocatorVariants(t *testing.T) {
	for _, tc := range []struct {
		name   string
		target func(string) Target
	}{
		{name: "symbol-uri", target: func(uri string) Target { return Target{URI: uri, Symbol: "F"} }},
		{name: "regex-position", target: func(uri string) Target {
			return Target{URI: uri, Regex: &RegexLocator{Pattern: "func F", MatchIndex: 0, MaxDocumentBytes: 4096, MaxMatches: 10, MaxPatternBytes: 128, MaxWork: 4096}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manager, started, uri, starter := structuralManagerWithResponses(t, func(string) map[string]json.RawMessage {
				return map[string]json.RawMessage{"initialize": json.RawMessage(`{"capabilities":{"documentSymbolProvider":true,"definitionProvider":true,"referencesProvider":true,"positionEncoding":"utf-16"}}`)}
			})
			result, failure := Execute(context.Background(), manager, Request{SessionID: "stable-alias", Generation: started.Generation, LanguageID: "cue", Target: tc.target(uri), SourceOnlyTarget: true, UpDepth: 0, DownDepth: 0, MaxNodes: 5, TimeoutMS: 1000, RequestTimeoutMS: 250, MaxMessages: 8, MaxBytes: 8192, Analysis: AnalysisRequest{Kind: AnalysisNeighborhood}, CaptureSupply: true})
			if failure != nil || result.State != StateComplete || len(result.Analysis.Nodes) != 1 || len(result.Analysis.Occurrences) != 0 {
				t.Fatalf("ASSERT_SOURCE_ONLY_TARGET_LOCATOR_%s: result=%+v failure=%+v", tc.name, result, failure)
			}
			methods := starter.children[0].observedMethods()
			for _, forbidden := range []string{"workspace/symbol", "textDocument/prepareCallHierarchy", "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls", "textDocument/definition", "textDocument/references"} {
				if slices.Contains(methods, forbidden) {
					t.Fatalf("ASSERT_SOURCE_ONLY_TARGET_LOCATOR_NO_RELATIONS_%s: forbidden=%s methods=%v", tc.name, forbidden, methods)
				}
			}
		})
	}
}

func TestMalformedGoplsSymbolInformationDiagnosticIsActionableFailClosedAndSafe(t *testing.T) {
	fixture, err := os.ReadFile("../../incomingops/testdata/document-symbol-gopls-symbol-information-runner.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, projection := range []bool{false, true} {
		t.Run(fmt.Sprintf("projection_%t", projection), func(t *testing.T) {
			manager, started, uri, starter := structuralManagerWithResponses(t, func(uri string) map[string]json.RawMessage {
				raw := strings.ReplaceAll(string(fixture), "file:///w/a.go", uri)
				return map[string]json.RawMessage{"textDocument/documentSymbol": json.RawMessage(raw)}
			})
			result, failure := Execute(context.Background(), manager, Request{
				SessionID: "stable-alias", Generation: started.Generation, LanguageID: "go", Target: Target{URI: uri, Symbol: "Runner"},
				UpDepth: 0, DownDepth: 0, MaxNodes: 5, TimeoutMS: 1000, RequestTimeoutMS: 250, MaxMessages: 8, MaxBytes: 8192,
				Analysis: AnalysisRequest{Kind: AnalysisNeighborhood},
			})
			if failure == nil || result.State != "" || failure.State != StateInvalidServerResponse || failure.Phase != PhasePreflight {
				t.Fatalf("ASSERT_GOPLS_SYMBOL_INFORMATION_FAIL_CLOSED: result=%+v failure=%+v", result, failure)
			}
			diagnostic := failure.TargetDiagnostic
			if diagnostic == nil || diagnostic.ExactMatches != 1 || diagnostic.TotalSymbols != 2 || diagnostic.Action != TargetActionFailMalformed || diagnostic.ProviderMethod != "textDocument/documentSymbol" || diagnostic.ItemIndex == nil || *diagnostic.ItemIndex != 0 || diagnostic.NormalizationStage != "POST_DECODE_NORMALIZATION" || diagnostic.FailedField != "kind" || diagnostic.FailedInvariant != "CALLABLE_SYMBOL_KIND" || diagnostic.ProjectionEntered == nil || *diagnostic.ProjectionEntered {
				t.Fatalf("ASSERT_GOPLS_SYMBOL_INFORMATION_ACTIONABLE_DIAGNOSTIC: %+v", diagnostic)
			}
			if diagnostic.Recovery == nil || diagnostic.Recovery.Kind != "POSITION_LOCATOR" || diagnostic.Recovery.URI != uri || diagnostic.Recovery.Line == nil || *diagnostic.Recovery.Line != 42 || diagnostic.Recovery.Character == nil || *diagnostic.Recovery.Character != 3 {
				t.Fatalf("ASSERT_GOPLS_SYMBOL_INFORMATION_POSITION_RECOVERY: %+v", diagnostic.Recovery)
			}
			methods := starter.children[0].observedMethods()
			for _, forbidden := range []string{"textDocument/prepareCallHierarchy", "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls"} {
				if slices.Contains(methods, forbidden) {
					t.Fatalf("ASSERT_GOPLS_SYMBOL_INFORMATION_NO_PREPARE_OR_TRAVERSAL: forbidden=%s methods=%v", forbidden, methods)
				}
			}
			raw, _ := json.Marshal(failure)
			for _, leaked := range []string{"Runner", `\"name\"`, `\"location\"`, "provider_error", "source_body", "/Users/"} {
				if strings.Contains(string(raw), leaked) {
					t.Fatalf("ASSERT_GOPLS_SYMBOL_INFORMATION_NO_LEAK_%q: %s", leaked, raw)
				}
			}
		})
	}
}

func TestMalformedGoplsSymbolInformationWithoutValidLocatorExplainsUnavailableSafely(t *testing.T) {
	fixture, err := os.ReadFile("../../incomingops/testdata/document-symbol-gopls-symbol-information-runner-no-uri.json")
	if err != nil {
		t.Fatal(err)
	}
	manager, started, uri, starter := structuralManagerWithResponses(t, func(string) map[string]json.RawMessage {
		return map[string]json.RawMessage{"textDocument/documentSymbol": fixture}
	})
	_, failure := Execute(context.Background(), manager, Request{
		SessionID: "stable-alias", Generation: started.Generation, LanguageID: "go", Target: Target{URI: uri, Symbol: "Runner"},
		UpDepth: 0, DownDepth: 0, MaxNodes: 5, TimeoutMS: 1000, RequestTimeoutMS: 250, MaxMessages: 8, MaxBytes: 8192,
		Analysis: AnalysisRequest{Kind: AnalysisNeighborhood},
	})
	if failure == nil || failure.State != StateInvalidServerResponse || failure.TargetDiagnostic == nil {
		t.Fatalf("ASSERT_GOPLS_SYMBOL_INFORMATION_NO_LOCATOR_FAIL_CLOSED: %+v", failure)
	}
	recovery := failure.TargetDiagnostic.Recovery
	if recovery == nil || recovery.Kind != "UNAVAILABLE" || recovery.UnavailableReason != "NO_INDEPENDENTLY_VALID_LOCATOR" || recovery.URI != "" || recovery.Line != nil || recovery.Character != nil {
		t.Fatalf("ASSERT_GOPLS_SYMBOL_INFORMATION_RECOVERY_UNAVAILABLE_REASON: %+v", recovery)
	}
	methods := starter.children[0].observedMethods()
	for _, forbidden := range []string{"textDocument/prepareCallHierarchy", "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls"} {
		if slices.Contains(methods, forbidden) {
			t.Fatalf("ASSERT_GOPLS_SYMBOL_INFORMATION_NO_LOCATOR_NO_TRAVERSAL: forbidden=%s methods=%v", forbidden, methods)
		}
	}
	raw, _ := json.Marshal(failure)
	for _, leaked := range []string{"Runner", `\"name\"`, `\"location\"`, "provider_error", "source_body", "/Users/"} {
		if strings.Contains(string(raw), leaked) {
			t.Fatalf("ASSERT_GOPLS_SYMBOL_INFORMATION_NO_LOCATOR_NO_LEAK_%q: %s", leaked, raw)
		}
	}
}

func TestExecuteAcceptsGoplsIdentifierRangeCallHierarchyVariant(t *testing.T) {
	manager, started, uri, starter := structuralManagerWithResponses(t, func(uri string) map[string]json.RawMessage {
		rng := func(line, start, end int) map[string]any {
			return map[string]any{"start": map[string]int{"line": line, "character": start}, "end": map[string]int{"line": line, "character": end}}
		}
		item := func(name string, line, start, end int) map[string]any {
			nameRange := rng(line, start, end)
			return map[string]any{"name": name, "kind": 12, "detail": "fixture", "uri": uri, "range": nameRange, "selectionRange": nameRange}
		}
		prepared, _ := json.Marshal([]any{item("F", 1, 5, 6)})
		outgoing, _ := json.Marshal([]any{map[string]any{"to": item("G", 2, 5, 6), "fromRanges": []any{rng(1, 9, 10)}}})
		incoming, _ := json.Marshal([]any{map[string]any{"from": item("H", 3, 5, 6), "fromRanges": []any{rng(4, 7, 8)}}})
		return map[string]json.RawMessage{
			"textDocument/prepareCallHierarchy": prepared,
			"callHierarchy/outgoingCalls":       outgoing,
			"callHierarchy/incomingCalls":       incoming,
		}
	})
	result, failure := Execute(context.Background(), manager, baseWireRequest(started, uri))
	if failure != nil || result.State != StateComplete || len(result.Analysis.Occurrences) != 2 {
		t.Fatalf("ASSERT_GOPLS_IDENTIFIER_RANGE_VARIANT_COMPLETE: result=%+v failure=%+v", result, failure)
	}
	wantMethods := []string{"textDocument/didOpen", "textDocument/prepareCallHierarchy", "callHierarchy/outgoingCalls", "callHierarchy/incomingCalls"}
	if got := starter.children[0].observedMethods(); !reflect.DeepEqual(got, wantMethods) {
		t.Fatalf("ASSERT_GOPLS_IDENTIFIER_RANGE_VARIANT_TRANSCRIPT: got=%v want=%v", got, wantMethods)
	}
}

func TestExecuteMalformedOutgoingItemCarriesAttributableSafeDiagnostic(t *testing.T) {
	manager, started, uri, _ := structuralManagerWithResponses(t, func(string) map[string]json.RawMessage {
		return map[string]json.RawMessage{"callHierarchy/outgoingCalls": json.RawMessage(`[{"to":{"name":"G","kind":12,"uri":"not a uri","range":{"start":{"line":2,"character":0},"end":{"line":2,"character":1}},"selectionRange":{"start":{"line":2,"character":0},"end":{"line":2,"character":1}}},"fromRanges":[{"start":{"line":1,"character":7},"end":{"line":1,"character":8}}]}]`)}
	})
	_, failure := Execute(context.Background(), manager, baseWireRequest(started, uri))
	if failure == nil || failure.TraversalDiagnostic == nil {
		t.Fatalf("ASSERT_MALFORMED_OUTGOING_ITEM_TYPED_FAILURE: %+v", failure)
	}
	raw, _ := json.Marshal(failure.TraversalDiagnostic)
	var diagnostic map[string]any
	_ = json.Unmarshal(raw, &diagnostic)
	for field, want := range map[string]any{"provider_method": "callHierarchy/outgoingCalls", "item_index": float64(0), "failed_field": "item.uri", "failed_invariant": "CONCRETE_DOCUMENT_URI", "provider_variant": "CALL_HIERARCHY_OUTGOING_CALL", "projection_entered": false, "guidance": "USE_TARGET_MODE_OR_FIX_PROVIDER_RESPONSE"} {
		if !reflect.DeepEqual(diagnostic[field], want) {
			t.Fatalf("ASSERT_MALFORMED_OUTGOING_ITEM_ACTIONABLE_%s: got=%v want=%v diagnostic=%s", strings.ToUpper(field), diagnostic[field], want, raw)
		}
	}
	for _, leaked := range []string{"not a uri", uri, "provider_error", "raw_response", "source_body"} {
		if strings.Contains(string(raw), leaked) {
			t.Fatalf("ASSERT_MALFORMED_OUTGOING_ITEM_PRIVACY_%q: %s", leaked, raw)
		}
	}
}

func TestExecuteConcreteManagerTransientNeighborhood(t *testing.T) {
	manager, started, uri := structuralManager(t)
	line, character := uint32(1), uint32(5)
	result, failure := Execute(context.Background(), manager, Request{
		SessionID: "stable-alias", Generation: started.Generation, LanguageID: "go", Target: Target{URI: uri, Line: &line, Character: &character},
		UpDepth: 1, DownDepth: 1, MaxNodes: 8, TimeoutMS: 1000, RequestTimeoutMS: 250, MaxMessages: 8, MaxBytes: 8192,
		Analysis: AnalysisRequest{Kind: AnalysisNeighborhood},
	})
	if failure != nil {
		t.Fatalf("ASSERT_CONCRETE_MANAGER_COMPLETE: %+v", failure)
	}
	if result.State != StateComplete || result.Phase != PhaseDeliveryCheck || result.Qualification.SessionID != started.SessionID || result.Qualification.Generation != started.Generation || result.Qualification.PositionEncoding != "utf-16" {
		t.Fatalf("ASSERT_EXACT_QUALIFIED_DELIVERY: %+v", result)
	}
	if len(result.Analysis.Nodes) != 3 || len(result.Analysis.Occurrences) != 2 || result.Accounting.Requests.Attempted != 3 || result.Accounting.Requests.Succeeded != 3 || result.Accounting.Preparation != (PreparationAccounting{Attempted: 1, Returned: 1}) || result.Accounting.Frontier != (FrontierAccounting{Observed: 2, Expanded: 2}) {
		t.Fatalf("ASSERT_EXACT_WIRE_ACCOUNTING: result=%+v", result)
	}
	if !reconciles(result.Accounting) || hasNonDuplicateOmission(result.Accounting) {
		t.Fatalf("ASSERT_COMPLETE_RECONCILED: %+v", result.Accounting)
	}
	if result.SchemaVersion != resultSchemaVersion || result.EvidenceClass != evidenceClassTransientLive || result.Authority != 0 || result.SourceGraphComplete != sourceGraphCompleteUnknown || result.Retained || result.Replayable || result.PublicationEligible || result.HydrationEligible || result.ClaimCeiling != claimCeiling || result.GraphDigest == "" || result.TargetID == "" {
		t.Fatalf("ASSERT_TRANSIENT_CLAIM_BOUNDARY: %+v", result)
	}
}

func TestExecuteMalformedTraversalResponsesCarryClosedDiagnostics(t *testing.T) {
	cases := []struct {
		name      string
		method    string
		stage     TraversalStage
		direction Direction
	}{
		{name: "prepare", method: "textDocument/prepareCallHierarchy", stage: TraversalStagePrepare},
		{name: "outgoing", method: "callHierarchy/outgoingCalls", stage: TraversalStageOutgoing, direction: DirectionOutgoing},
		{name: "incoming", method: "callHierarchy/incomingCalls", stage: TraversalStageIncoming, direction: DirectionIncoming},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manager, started, uri, _ := structuralManagerWithResponses(t, func(string) map[string]json.RawMessage {
				return map[string]json.RawMessage{tc.method: json.RawMessage(`{}`)}
			})
			result, failure := Execute(context.Background(), manager, baseWireRequest(started, uri))
			if failure == nil || failure.State != StateInvalidServerResponse || !reflect.DeepEqual(result, Result{}) {
				t.Fatalf("ASSERT_MALFORMED_%s_TYPED_FAILURE: result=%+v failure=%+v", tc.stage, result, failure)
			}
			if tc.stage == TraversalStagePrepare {
				if failure.Phase != PhasePreflight || failure.TraversalDiagnostic != nil || failure.TargetDiagnostic == nil || failure.TargetDiagnostic.Action != TargetActionFailDocument {
					t.Fatalf("ASSERT_MALFORMED_PREPARE_TARGET_DIAGNOSTIC: %+v", failure)
				}
			} else {
				if failure.Phase != PhaseTraversal || failure.TraversalDiagnostic == nil || failure.TraversalDiagnostic.Stage != tc.stage || failure.TraversalDiagnostic.Method != tc.method || failure.TraversalDiagnostic.Direction != tc.direction || failure.TraversalDiagnostic.Depth != nil {
					t.Fatalf("ASSERT_MALFORMED_%s_EXACT_DIAGNOSTIC: got=%+v", tc.stage, failure.TraversalDiagnostic)
				}
				diagnosticRaw, _ := json.Marshal(failure.TraversalDiagnostic)
				var diagnostic map[string]any
				_ = json.Unmarshal(diagnosticRaw, &diagnostic)
				for field, want := range map[string]any{"provider_method": tc.method, "failed_field": "response", "failed_invariant": "ARRAY_RESULT", "provider_variant": "NON_ARRAY", "projection_entered": false, "guidance": "RETRY_PROVIDER_OR_REPORT_MALFORMED_RESPONSE"} {
					if !reflect.DeepEqual(diagnostic[field], want) {
						t.Fatalf("ASSERT_MALFORMED_%s_ACTIONABLE_%s: got=%v want=%v diagnostic=%s", tc.stage, strings.ToUpper(field), diagnostic[field], want, diagnosticRaw)
					}
				}
				if _, exists := diagnostic["item_index"]; exists {
					t.Fatalf("ASSERT_MALFORMED_%s_UNATTRIBUTABLE_ITEM_INDEX_OMITTED: %s", tc.stage, diagnosticRaw)
				}
			}
			raw, err := json.Marshal(failure)
			if err != nil {
				t.Fatal(err)
			}
			for _, forbidden := range []string{"private", uri, "source_body", "selector", "environment", "provider_error", "raw_response"} {
				if forbidden != "" && string(raw) != "" && containsJSONText(raw, forbidden) {
					t.Fatalf("ASSERT_MALFORMED_%s_PRIVACY: leaked %q in %s", tc.stage, forbidden, raw)
				}
			}
		})
	}
}

func containsJSONText(raw []byte, text string) bool {
	return strings.Contains(string(raw), text)
}

func TestExecuteRequestScopedDocumentSupply(t *testing.T) {
	for _, capture := range []bool{false, true} {
		manager, started, uri := structuralManager(t)
		req := baseWireRequest(started, uri)
		req.CaptureSupply = capture
		result, failure := Execute(context.Background(), manager, req)
		if failure != nil {
			t.Fatalf("ASSERT_TRANSIENT_SUPPLY_EXECUTION_%t: %+v", capture, failure)
		}
		if !capture {
			if result.SourceSupply != nil {
				t.Fatalf("ASSERT_TRANSIENT_SUPPLY_OMITTED: %+v", result.SourceSupply)
			}
			continue
		}
		if result.SourceSupply == nil || result.SourceSupply.SessionID != started.SessionID || result.SourceSupply.Generation != started.Generation || result.SourceSupply.URI != uri || result.SourceSupply.Classification != "LSP_SUPPLIED" || len(result.SourceSupply.Content) == 0 {
			t.Fatalf("ASSERT_TRANSIENT_SUPPLY_EXACT_REQUEST: %+v", result.SourceSupply)
		}
	}
}

func TestExecuteZeroDepthPerformsNoCallTraversal(t *testing.T) {
	manager, started, uri := structuralManager(t)
	line, character := uint32(1), uint32(5)
	result, failure := Execute(context.Background(), manager, Request{
		SessionID: started.SessionID, Generation: started.Generation, LanguageID: "go", Target: Target{URI: uri, Line: &line, Character: &character},
		UpDepth: 0, DownDepth: 0, MaxNodes: 1, TimeoutMS: 1000, RequestTimeoutMS: 250, MaxMessages: 8, MaxBytes: 8192,
		Analysis: AnalysisRequest{Kind: AnalysisNeighborhood},
	})
	if failure != nil || result.State != StateEmpty || result.Phase != PhaseDeliveryCheck {
		t.Fatalf("ASSERT_ZERO_CALLS_EMPTY_AFTER_DELIVERY: result=%+v failure=%+v", result, failure)
	}
	if result.Accounting.Requests != (RequestAccounting{Attempted: 1, Succeeded: 1}) || result.Accounting.Frontier != (FrontierAccounting{}) || len(result.Analysis.Nodes) != 1 || len(result.Analysis.Occurrences) != 0 {
		t.Fatalf("ASSERT_ZERO_DEPTH_NO_CALL_REQUESTS: %+v", result)
	}
}

func TestExecuteExhaustiveHierarchicalSymbolResolution(t *testing.T) {
	manager, started, uri := structuralManager(t)
	result, failure := Execute(context.Background(), manager, Request{
		SessionID: started.SessionID, Generation: started.Generation, LanguageID: "go", Target: Target{URI: uri, Symbol: "F"},
		UpDepth: 1, DownDepth: 1, MaxNodes: 8, TimeoutMS: 1000, RequestTimeoutMS: 250, MaxMessages: 8, MaxBytes: 8192,
		Analysis: AnalysisRequest{Kind: AnalysisImpact, Direction: DirectionIncoming, MaxDepth: 1},
	})
	if failure != nil || result.State != StateComplete {
		t.Fatalf("ASSERT_HIERARCHICAL_SYMBOL_COMPLETE: result=%+v failure=%+v", result, failure)
	}
	if result.Accounting.Requests != (RequestAccounting{Attempted: 4, Succeeded: 4}) || result.Accounting.Preparation != (PreparationAccounting{Attempted: 1, Returned: 1}) {
		t.Fatalf("ASSERT_SYMBOL_PREPARED_ITEM_REUSED_ONCE: %+v", result.Accounting)
	}
	if len(result.Analysis.Nodes) != 1 || len(result.Analysis.Occurrences) != 1 || result.Analysis.Direction != DirectionIncoming {
		t.Fatalf("ASSERT_SYMBOL_IMPACT_EXACT: %+v", result.Analysis)
	}
}

func TestExecuteFailsClosedForStaleGenerationAndUnsupportedAnalysis(t *testing.T) {
	manager, started, uri := structuralManager(t)
	line, character := uint32(1), uint32(5)
	base := Request{SessionID: started.SessionID, Generation: started.Generation, LanguageID: "go", Target: Target{URI: uri, Line: &line, Character: &character},
		UpDepth: 1, DownDepth: 1, MaxNodes: 8, TimeoutMS: 1000, RequestTimeoutMS: 250, MaxMessages: 8, MaxBytes: 8192, Analysis: AnalysisRequest{Kind: AnalysisNeighborhood}}
	stale := base
	stale.Generation++
	if result, failure := Execute(context.Background(), manager, stale); failure == nil || failure.Phase != PhasePreflight || failure.State != StateGenerationChanged || !reflect.DeepEqual(result, Result{}) {
		t.Fatalf("ASSERT_STALE_ZERO_RESULT: result=%+v failure=%+v", result, failure)
	}
	unsupported := base
	unsupported.Analysis.Kind = "PAGERANK"
	if result, failure := Execute(context.Background(), manager, unsupported); failure == nil || failure.Phase != PhasePreflight || failure.State != StateInvalidServerResponse || !reflect.DeepEqual(result, Result{}) {
		t.Fatalf("ASSERT_ANALYSIS_SCOPE_ZERO_RESULT: result=%+v failure=%+v", result, failure)
	}
}

func baseWireRequest(started sessionruntime.StartResult, uri string) Request {
	line, character := uint32(1), uint32(5)
	return Request{SessionID: started.SessionID, Generation: started.Generation, LanguageID: "go", Target: Target{URI: uri, Line: &line, Character: &character},
		UpDepth: 1, DownDepth: 1, MaxNodes: 8, TimeoutMS: 1000, RequestTimeoutMS: 250, MaxMessages: 8, MaxBytes: 8192, Analysis: AnalysisRequest{Kind: AnalysisNeighborhood}}
}

func TestExactExternalTargetStopsBeforeLocatorAndSourceDisclosure(t *testing.T) {
	manager, started, _, starter := structuralManagerWithResponses(t, nil)
	external := (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Join(t.TempDir(), "sdk.go"))}).String()
	request := baseWireRequest(started, external)
	request.Target.Line, request.Target.Character = nil, nil
	request.Target.Regex = &RegexLocator{Pattern: "Validate", MaxDocumentBytes: 1024, MaxMatches: 1, MaxPatternBytes: 64, MaxWork: 1024}
	_, failure := Execute(context.Background(), manager, request)
	if failure == nil || failure.State != StateSourceUnavailable || failure.Reason != FailureReasonDocumentOutsideWorkspace || failure.TargetDiagnostic != nil || failure.TraversalDiagnostic != nil {
		t.Fatalf("ASSERT_EXTERNAL_TYPED_SOURCE_UNAVAILABLE: %+v", failure)
	}
	if methods := starter.children[0].observedMethods(); len(methods) != 0 {
		t.Fatalf("ASSERT_EXTERNAL_NO_DOCUMENT_SYMBOL_OR_CALL_HIERARCHY: %v", methods)
	}
}

func TestExecuteCapabilityAndEncodingPreflightFailures(t *testing.T) {
	cases := []struct {
		name       string
		initialize string
	}{
		{"capability", `{"capabilities":{"callHierarchyProvider":false,"documentSymbolProvider":true,"positionEncoding":"utf-16"}}`},
		{"encoding", `{"capabilities":{"callHierarchyProvider":true,"documentSymbolProvider":true,"positionEncoding":"utf-7"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manager, started, uri, starter := structuralManagerWithResponses(t, func(string) map[string]json.RawMessage {
				return map[string]json.RawMessage{"initialize": json.RawMessage(tc.initialize)}
			})
			result, failure := Execute(context.Background(), manager, baseWireRequest(started, uri))
			if failure == nil || failure.Phase != PhasePreflight || failure.State != StateUnsupported || !reflect.DeepEqual(result, Result{}) {
				t.Fatalf("ASSERT_CAPABILITY_ENCODING_PREFLIGHT_ZERO_RESULT: result=%+v failure=%+v", result, failure)
			}
			if methods := starter.children[0].observedMethods(); len(methods) != 0 {
				t.Fatalf("ASSERT_PREFLIGHT_BEFORE_TRAVERSAL: %v", methods)
			}
		})
	}
}

func TestExecuteTargetZeroAndMultipleMapExactly(t *testing.T) {
	positionRange := func(line int) map[string]any {
		return map[string]any{"start": map[string]int{"line": line, "character": 0}, "end": map[string]int{"line": line, "character": 1}}
	}
	for _, tc := range []struct {
		name  string
		count int
		want  TerminalState
	}{{"zero", 0, StateTargetNotFound}, {"multiple", 2, StateAmbiguousTarget}} {
		t.Run(tc.name, func(t *testing.T) {
			manager, started, uri, starter := structuralManagerWithResponses(t, func(string) map[string]json.RawMessage {
				var symbols []any
				for i := 0; i < tc.count; i++ {
					symbols = append(symbols, map[string]any{"name": "F", "kind": 12, "range": positionRange(i + 1), "selectionRange": positionRange(i + 1)})
				}
				raw, _ := json.Marshal(symbols)
				return map[string]json.RawMessage{"textDocument/documentSymbol": raw}
			})
			request := baseWireRequest(started, uri)
			request.Target.Symbol, request.Target.Line, request.Target.Character = "F", nil, nil
			result, failure := Execute(context.Background(), manager, request)
			if failure == nil || failure.Phase != PhasePreflight || failure.State != tc.want || !reflect.DeepEqual(result, Result{}) {
				t.Fatalf("ASSERT_TARGET_CARDINALITY_EXACT_ZERO_RESULT: result=%+v failure=%+v", result, failure)
			}
			if methods := starter.children[0].observedMethods(); !reflect.DeepEqual(methods, []string{"textDocument/didOpen", "textDocument/documentSymbol"}) {
				t.Fatalf("ASSERT_TARGET_FAILURE_WIRE_SEQUENCE: %v", methods)
			}
		})
	}
}

func TestExecuteRegexLocatorManagedWire(t *testing.T) {
	t.Run("late capture after direct position", func(t *testing.T) {
		manager, started, uri, starter := structuralManagerWithResponses(t, nil)
		if result, failure := Execute(context.Background(), manager, baseWireRequest(started, uri)); failure != nil || result.State != StateComplete {
			t.Fatalf("ASSERT_REGEX_LATE_CAPTURE_DIRECT_PRECONDITION: result=%+v failure=%+v", result, failure)
		}
		starter.children[0].resetMethods()
		request := baseWireRequest(started, uri)
		request.Target.Line, request.Target.Character = nil, nil
		request.Target.Regex = &RegexLocator{Pattern: `func (F)`, CaptureGroup: 1, MaxDocumentBytes: 1024, MaxMatches: 10, MaxPatternBytes: 100, MaxWork: 2048}
		result, failure := Execute(context.Background(), manager, request)
		if failure != nil || result.State != StateComplete {
			t.Fatalf("ASSERT_REGEX_LATE_CAPTURE_EQUIVALENT_TO_DIRECT_POSITION: result=%+v failure=%+v", result, failure)
		}
		if got := starter.children[0].observedPreparePositions(); !reflect.DeepEqual(got, []string{"1:5", "1:5"}) {
			t.Fatalf("ASSERT_REGEX_LATE_CAPTURE_UTF16_POSITION: %v", got)
		}
		t.Log("ASSERT_REGEX_LATE_CAPTURE_EQUIVALENT_TO_DIRECT_POSITION: PASS")
		t.Log("ASSERT_REGEX_LATE_CAPTURE_UTF16_POSITION: PASS")
	})
	t.Run("success", func(t *testing.T) {
		manager, started, uri, starter := structuralManagerWithResponses(t, nil)
		request := baseWireRequest(started, uri)
		request.Target.Line, request.Target.Character = nil, nil
		request.Target.Regex = &RegexLocator{Pattern: `func (F)`, CaptureGroup: 1, MaxDocumentBytes: 1024, MaxMatches: 10, MaxPatternBytes: 100, MaxWork: 2048}
		result, failure := Execute(context.Background(), manager, request)
		if failure != nil || result.State != StateComplete {
			t.Fatalf("ASSERT_REGEX_MANAGED_COMPLETE: result=%+v failure=%+v", result, failure)
		}
		wantMethods := []string{"textDocument/didOpen", "textDocument/prepareCallHierarchy", "callHierarchy/outgoingCalls", "callHierarchy/incomingCalls"}
		if got := starter.children[0].observedMethods(); !reflect.DeepEqual(got, wantMethods) {
			t.Fatalf("ASSERT_REGEX_EXACTLY_ONCE_PREPARE: got=%v want=%v", got, wantMethods)
		}
		if got := starter.children[0].observedPreparePositions(); !reflect.DeepEqual(got, []string{"1:5"}) {
			t.Fatalf("ASSERT_REGEX_SELECTED_EXACT_POSITION: %v", got)
		}
	})
	t.Run("resource rejection precedes prepare and projection", func(t *testing.T) {
		manager, started, uri, starter := structuralManagerWithResponses(t, nil)
		request := baseWireRequest(started, uri)
		request.Target.Line, request.Target.Character = nil, nil
		request.Target.Regex = &RegexLocator{Pattern: `func (F)`, CaptureGroup: 1, MaxDocumentBytes: 1024, MaxMatches: 10, MaxPatternBytes: 100, MaxWork: 1}
		result, failure := Execute(context.Background(), manager, request)
		if failure == nil || failure.Phase != PhasePreflight || failure.State != StateResourceLimit || failure.ResourceDiagnostic == nil || failure.ResourceDiagnostic.Field != ResourceFieldRegexMaxWork || !reflect.DeepEqual(result, Result{}) {
			t.Fatalf("ASSERT_REGEX_RESOURCE_FAILS_PREFLIGHT: result=%+v failure=%+v", result, failure)
		}
		for _, method := range starter.children[0].observedMethods() {
			if method != "textDocument/didOpen" {
				t.Fatalf("ASSERT_REGEX_RESOURCE_FAILS_BEFORE_PREPARE_TRAVERSAL_PROJECTION: %v", starter.children[0].observedMethods())
			}
		}
	})
	t.Run("absent", func(t *testing.T) {
		manager, started, uri, starter := structuralManagerWithResponses(t, nil)
		request := baseWireRequest(started, uri)
		request.Target.Line, request.Target.Character = nil, nil
		request.Target.Regex = &RegexLocator{Pattern: `func (Missing)`, CaptureGroup: 1, MaxDocumentBytes: 1024, MaxMatches: 10, MaxPatternBytes: 100, MaxWork: 2048}
		result, failure := Execute(context.Background(), manager, request)
		if failure == nil || failure.Phase != PhasePreflight || failure.State != StateTargetNotFound || failure.Reason != FailureReasonNoRegexMatch || !reflect.DeepEqual(result, Result{}) {
			t.Fatalf("ASSERT_REGEX_ABSENT_ZERO_RESULT: result=%+v failure=%+v", result, failure)
		}
		for _, method := range starter.children[0].observedMethods() {
			if method != "textDocument/didOpen" {
				t.Fatalf("ASSERT_REGEX_FAILURE_ZERO_TRAVERSAL: %v", starter.children[0].observedMethods())
			}
		}
	})
	t.Run("truncated document-symbol recovery", func(t *testing.T) {
		manager, started, uri, starter := structuralManagerWithResponses(t, func(string) map[string]json.RawMessage {
			symbols := make([]map[string]any, 0, 9)
			for i := 0; i < 9; i++ {
				rng := map[string]any{"start": map[string]int{"line": 8 + i, "character": 0}, "end": map[string]int{"line": 8 + i, "character": 4}}
				symbols = append(symbols, map[string]any{"name": fmt.Sprintf("Outside%d", i), "kind": 12, "range": rng, "selectionRange": rng})
			}
			raw, _ := json.Marshal(symbols)
			return map[string]json.RawMessage{"textDocument/prepareCallHierarchy": json.RawMessage(`[]`), "textDocument/documentSymbol": raw}
		})
		request := baseWireRequest(started, uri)
		request.Target.Line, request.Target.Character = nil, nil
		request.Target.Regex = &RegexLocator{Pattern: `func (F)`, CaptureGroup: 1, MaxDocumentBytes: 1024, MaxMatches: 10, MaxPatternBytes: 100, MaxWork: 2048}
		result, failure := Execute(context.Background(), manager, request)
		if failure == nil || failure.State != StateTargetNotFound || failure.TargetDiagnostic == nil || failure.TargetDiagnostic.Action != TargetActionEnumerationTruncated || failure.TargetDiagnostic.Completeness != "UNKNOWN" || len(failure.TargetDiagnostic.Recoveries) != 2 || !reflect.DeepEqual(result, Result{}) {
			t.Fatalf("ASSERT_REGEX_RECOVERY_TRUNCATED_UNKNOWN_NOT_ABSENT: result=%+v failure=%+v", result, failure)
		}
		wantLimits := map[string]int{"max_document_bytes": 60 * 1024, "max_pattern_bytes": 4 * 1024, "max_work": 64 * 1024, "max_matches": 100}
		if got := failure.TargetDiagnostic.Recoveries[1]; got.Kind != "REGEX_LOCATOR_TEMPLATE" || !reflect.DeepEqual(got.Limits, wantLimits) {
			t.Fatalf("ASSERT_REGEX_RECOVERY_PRACTICAL_INITIAL_LIMITS: got=%+v want=%v", got, wantLimits)
		}
		methods := starter.children[0].observedMethods()
		if slices.Contains(methods, "workspace/symbol") || !reflect.DeepEqual(methods, []string{"textDocument/didOpen", "textDocument/prepareCallHierarchy", "textDocument/documentSymbol"}) {
			t.Fatalf("ASSERT_REGEX_RECOVERY_NAMED_DOCUMENT_NO_WORKSPACE_ENUMERATION: %v", methods)
		}
	})
}

func TestExecuteExactWireSequence(t *testing.T) {
	manager, started, uri, starter := structuralManagerWithResponses(t, nil)
	result, failure := Execute(context.Background(), manager, baseWireRequest(started, uri))
	if failure != nil || result.State != StateComplete {
		t.Fatalf("ASSERT_WIRE_SEQUENCE_FIXTURE_COMPLETE: result=%+v failure=%+v", result, failure)
	}
	want := []string{"textDocument/didOpen", "textDocument/prepareCallHierarchy", "callHierarchy/outgoingCalls", "callHierarchy/incomingCalls"}
	if methods := starter.children[0].observedMethods(); !reflect.DeepEqual(methods, want) {
		t.Fatalf("ASSERT_EXACT_ALLOWED_WIRE_METHOD_SEQUENCE: got=%v want=%v", methods, want)
	}
}

func TestExecuteManagedWireSelfCallDedupAndMultipleSites(t *testing.T) {
	manager, started, uri, _ := structuralManagerWithResponses(t, func(uri string) map[string]json.RawMessage {
		positionRange := func(line, start, end int) map[string]any {
			return map[string]any{"start": map[string]int{"line": line, "character": start}, "end": map[string]int{"line": line, "character": end}}
		}
		item := map[string]any{"name": "F", "kind": 12, "uri": uri, "range": positionRange(1, 0, 12), "selectionRange": positionRange(1, 5, 6)}
		sites := []any{positionRange(1, 7, 8), positionRange(1, 9, 10)}
		outgoing, _ := json.Marshal([]any{map[string]any{"to": item, "fromRanges": sites}})
		incoming, _ := json.Marshal([]any{map[string]any{"from": item, "fromRanges": sites}})
		return map[string]json.RawMessage{"callHierarchy/outgoingCalls": outgoing, "callHierarchy/incomingCalls": incoming}
	})
	result, failure := Execute(context.Background(), manager, baseWireRequest(started, uri))
	if failure != nil || len(result.Analysis.Occurrences) != 2 {
		t.Fatalf("ASSERT_MANAGED_WIRE_SELF_CALL_MULTI_SITE_DEDUP: result=%+v failure=%+v", result, failure)
	}
	if result.Accounting.Occurrences != (AdmissionAccounting{Observed: 4, Admitted: 2, Omitted: 2}) || !hasOmission(result.Accounting, OmissionDuplicate) {
		t.Fatalf("ASSERT_MANAGED_WIRE_DEDUP_OMISSION_ACCOUNTING: %+v", result.Accounting)
	}
}

func TestExecuteStopRestartAndGenerationChangeAtEveryPhase(t *testing.T) {
	phases := []Phase{PhasePreflight, PhaseTraversal, PhaseAdmission, PhaseAnalysis, PhaseDeliveryCheck}
	for _, phase := range phases {
		for _, lifecycle := range []string{"stop", "restart"} {
			t.Run(string(phase)+"/"+lifecycle, func(t *testing.T) {
				manager, started, uri := structuralManager(t)
				called := false
				hooks := executionHooks{afterPhase: func(observed Phase) {
					if called || observed != phase {
						return
					}
					called = true
					if lifecycle == "stop" {
						accepted := manager.Stop(context.Background(), started.SessionID, "phase-hook-stop")
						if accepted.Failure != "" {
							t.Fatalf("ASSERT_STOP_HOOK_ACCEPTED: %+v", accepted)
						}
						return
					}
					accepted := manager.Restart(context.Background(), started.SessionID, "phase-hook-restart")
					if accepted.Failure != "" || accepted.IntentID == "" {
						t.Fatalf("ASSERT_RESTART_HOOK_ACCEPTED: %+v", accepted)
					}
					deadline := time.Now().Add(time.Second)
					for time.Now().Before(deadline) {
						operation, ok := manager.Operation(accepted.IntentID)
						if ok && operation.State == sessionruntime.OperationComplete {
							return
						}
						if ok && operation.State == sessionruntime.OperationFailed {
							t.Fatalf("ASSERT_RESTART_HOOK_COMPLETES: %+v", operation)
						}
						time.Sleep(time.Millisecond)
					}
					t.Fatal("ASSERT_RESTART_HOOK_COMPLETES: timeout")
				}}
				result, failure := execute(context.Background(), manager, baseWireRequest(started, uri), hooks)
				want := StateCancelled
				if lifecycle == "restart" {
					want = StateGenerationChanged
				}
				if !called || failure == nil || failure.Phase != phase || failure.State != want || !reflect.DeepEqual(result, Result{}) {
					t.Fatalf("ASSERT_LIFECYCLE_EACH_PHASE_ZERO_RESULT: called=%v result=%+v failure=%+v want=%s/%s", called, result, failure, phase, want)
				}
			})
		}
	}
}
