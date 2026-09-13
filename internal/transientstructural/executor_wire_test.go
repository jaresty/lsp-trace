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
	in     *io.PipeReader
	stdin  *io.PipeWriter
	out    *io.PipeWriter
	stdout *io.PipeReader
	uri    string
}

func newStructuralWire(uri string) *structuralWire {
	in, stdin := io.Pipe()
	stdout, out := io.Pipe()
	wire := &structuralWire{in: in, stdin: stdin, out: out, stdout: stdout, uri: uri}
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
	uri      string
	mu       sync.Mutex
	children []*structuralWire
}

func (s *structuralStarter) Start(context.Context, managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	child := newStructuralWire(s.uri)
	s.mu.Lock()
	s.children = append(s.children, child)
	s.mu.Unlock()
	return child, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
}

func structuralManager(t *testing.T) (*sessionruntime.Manager, sessionruntime.StartResult, string) {
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
	manager, err := sessionruntime.New(sessionruntime.Config{
		Limits:  sessionruntime.Limits{MaxSessions: 1, MaxRequests: 4, MaxChildren: 2, MaxCancels: 4, MaxTombstones: 8, MaxObservations: 64, MaxOperations: 8},
		Starter: &structuralStarter{uri: uri},
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
	t.Cleanup(func() {
		_ = manager.Stop(context.Background(), started.SessionID, "transient-test-cleanup")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = manager.Shutdown(ctx)
	})
	return manager, started, uri
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
	if result.State != StateComplete || result.Phase != PhaseDelivery || result.Qualification.SessionID != started.SessionID || result.Qualification.Generation != started.Generation || result.Qualification.PositionEncoding != "utf-16" {
		t.Fatalf("ASSERT_EXACT_QUALIFIED_DELIVERY: %+v", result)
	}
	if len(result.Analysis.Nodes) != 3 || len(result.Analysis.Occurrences) != 2 || result.Accounting.Requests.Attempted != 3 || result.Accounting.Requests.Succeeded != 3 || result.Accounting.Preparation != (PreparationAccounting{Attempted: 1, Returned: 1}) || result.Accounting.Frontier != (FrontierAccounting{Observed: 2, Expanded: 2}) {
		t.Fatalf("ASSERT_EXACT_WIRE_ACCOUNTING: result=%+v", result)
	}
	if !reconciles(result.Accounting) || hasNonDuplicateOmission(result.Accounting) {
		t.Fatalf("ASSERT_COMPLETE_RECONCILED: %+v", result.Accounting)
	}
	if !result.Claims.Transient || result.Claims.Retained || result.Claims.Replayable || result.Claims.Authoritative || result.GraphDigest == "" || result.TargetID == "" {
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
	if failure != nil || result.State != StateComplete {
		t.Fatalf("ASSERT_ZERO_DEPTH_COMPLETE: result=%+v failure=%+v", result, failure)
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
