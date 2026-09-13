package transientstructural

import (
	"context"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

type structuralWire struct {
	in        *io.PipeReader
	stdin     *io.PipeWriter
	out       *io.PipeWriter
	stdout    *io.PipeReader
	uri       string
	mu        sync.Mutex
	methods   []string
	responses map[string]json.RawMessage
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
	if result.Accounting.Requests != (RequestAccounting{Attempted: 5, Succeeded: 5}) || result.Accounting.Preparation != (PreparationAccounting{Attempted: 2, Returned: 2}) {
		t.Fatalf("ASSERT_ALL_SYMBOL_PREPARES_ACCOUNTED: %+v", result.Accounting)
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
