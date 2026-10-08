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
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

type targetWirePlan struct {
	documentSymbolNotifications int
	documentSymbolResult        json.RawMessage
	holdDocumentSymbol          bool
}

type targetWireStarter struct {
	uri      string
	mu       sync.Mutex
	plans    []targetWirePlan
	children []*targetWire
}

func (s *targetWireStarter) Start(context.Context, managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	s.mu.Lock()
	index := len(s.children)
	plan := targetWirePlan{documentSymbolResult: targetDocumentSymbols(s.uri)}
	if index < len(s.plans) {
		plan = s.plans[index]
	}
	child := newTargetWire(plan)
	s.children = append(s.children, child)
	s.mu.Unlock()
	go child.serve(s.uri)
	return child, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
}

func (s *targetWireStarter) child(index int) *targetWire {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 || index >= len(s.children) {
		return nil
	}
	return s.children[index]
}

type targetWire struct {
	serverIn          *io.PipeReader
	hostStdin         *io.PipeWriter
	serverOut         *io.PipeWriter
	hostStdout        *io.PipeReader
	plan              targetWirePlan
	mu                sync.Mutex
	methods           []string
	notices           int
	responseAttempted bool
	responseWritten   bool
	closed            int
	teardowns         int
}

func newTargetWire(plan targetWirePlan) *targetWire {
	serverIn, hostStdin := io.Pipe()
	hostStdout, serverOut := io.Pipe()
	return &targetWire{serverIn: serverIn, hostStdin: hostStdin, serverOut: serverOut, hostStdout: hostStdout, plan: plan}
}

func (w *targetWire) serve(uri string) {
	reader := lspwire.NewReader(w.serverIn, lspwire.DefaultLimits())
	writer := lspwire.NewWriter(w.serverOut, lspwire.DefaultLimits())
	for {
		message, err := reader.Read()
		if err != nil {
			return
		}
		w.mu.Lock()
		w.methods = append(w.methods, message.Method)
		w.mu.Unlock()
		switch message.Method {
		case "initialize":
			if writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: message.ID, Result: json.RawMessage(`{"capabilities":{"callHierarchyProvider":true,"documentSymbolProvider":true,"positionEncoding":"utf-16"}}`)}) != nil {
				return
			}
		case "initialized", "textDocument/didOpen", "textDocument/didChange":
			continue
		case "textDocument/documentSymbol":
			if w.plan.holdDocumentSymbol {
				continue
			}
			for i := 0; i < w.plan.documentSymbolNotifications; i++ {
				notice := lspwire.Message{JSONRPC: lspwire.Version, Method: "$/progress", Params: json.RawMessage(fmt.Sprintf(`{"token":"fixture-%d","value":{"kind":"report","message":"bounded notice %d"}}`, i, i))}
				if writer.Write(notice) != nil {
					return
				}
				w.mu.Lock()
				w.notices++
				w.mu.Unlock()
			}
			w.mu.Lock()
			w.responseAttempted = true
			w.mu.Unlock()
			err := writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: message.ID, Result: append(json.RawMessage(nil), w.plan.documentSymbolResult...)})
			if err == nil {
				w.mu.Lock()
				w.responseWritten = true
				w.mu.Unlock()
			}
			if err != nil {
				return
			}
		case "shutdown":
			if writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: message.ID, Result: json.RawMessage(`null`)}) != nil {
				return
			}
		case "exit":
			return
		}
	}
}

func (w *targetWire) Stdin() io.WriteCloser { return w.hostStdin }
func (w *targetWire) Stdout() io.ReadCloser { return w.hostStdout }

func (w *targetWire) Close() managedprocess.ResourceObservation {
	w.mu.Lock()
	w.closed++
	w.mu.Unlock()
	_ = w.hostStdout.Close()
	_ = w.hostStdin.Close()
	_ = w.serverOut.Close()
	_ = w.serverIn.Close()
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}

func (w *targetWire) Teardown(context.Context) managedprocess.TeardownObservation {
	w.mu.Lock()
	w.teardowns++
	w.mu.Unlock()
	_ = w.hostStdin.Close()
	_ = w.serverIn.Close()
	_ = w.serverOut.Close()
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}

func (w *targetWire) snapshot() (methods []string, notices, closeCount, teardownCount int, responseAttempted, responseWritten bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.methods...), w.notices, w.closed, w.teardowns, w.responseAttempted, w.responseWritten
}

func waitTargetWireSnapshot(t *testing.T, wire *targetWire, ready func(notices, closes, teardowns int, responseAttempted, responseWritten bool) bool) (notices, closes, teardowns int, responseAttempted, responseWritten bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		_, notices, closes, teardowns, responseAttempted, responseWritten = wire.snapshot()
		if ready(notices, closes, teardowns, responseAttempted, responseWritten) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	return
}

func targetDocumentSymbols(uri string) json.RawMessage {
	result, _ := json.Marshal([]any{map[string]any{
		"name": "F", "kind": 12, "range": targetRange(1, 0, 12), "selectionRange": targetRange(1, 5, 6),
	}})
	return result
}

func targetRange(line, start, end int) map[string]any {
	return map[string]any{"start": map[string]int{"line": line, "character": start}, "end": map[string]int{"line": line, "character": end}}
}

func oversizedTargetDocumentSymbols(uri string) json.RawMessage {
	items := make([]any, 0, 40)
	for i := 0; i < 40; i++ {
		items = append(items, map[string]any{
			"name": fmt.Sprintf("F_%02d_with_valid_but_bounded_payload", i), "kind": 12,
			"range": targetRange(1, 0, 12), "selectionRange": targetRange(1, 5, 6),
		})
	}
	result, _ := json.Marshal(items)
	return result
}

func newTargetResourceFixture(t *testing.T, plans ...targetWirePlan) (*sessionruntime.Manager, sessionruntime.StartResult, string, *targetWireStarter) {
	t.Helper()
	workspace := t.TempDir()
	path := filepath.Join(workspace, "code.go")
	if err := os.WriteFile(path, []byte("package p\nfunc F() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
	profile, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "transient-test", Workspace: workspace, Profile: "fake", EnvironmentReference: "test"})
	if err != nil {
		t.Fatal(err)
	}
	starter := &targetWireStarter{uri: uri, plans: append([]targetWirePlan(nil), plans...)}
	manager, err := sessionruntime.New(sessionruntime.Config{
		Limits:  sessionruntime.Limits{MaxSessions: 3, MaxRequests: 4, MaxChildren: 2, MaxCancels: 4, MaxTombstones: 8, MaxObservations: 64, MaxOperations: 8},
		Starter: starter,
	})
	if err != nil {
		t.Fatal(err)
	}
	started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(profile), BootstrapAlias: "stable-alias", LanguageID: "go"})
	if started.Failure != "" {
		t.Fatalf("fixture startup failed: %+v", started)
	}
	pending := manager.BeginReadiness(context.Background(), started.SessionID, started.Generation, time.Now().Add(time.Second))
	ready, ok := manager.WaitReadiness(context.Background(), pending.ID)
	if !ok || ready.State != sessionruntime.ReadinessReady || ready.Failure != "" {
		t.Fatalf("fixture readiness failed: %+v", ready)
	}
	cleanupManager := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		for _, record := range manager.Records() {
			stopped := manager.Stop(ctx, record.SessionID, "target-resource-test-cleanup-"+record.SessionID)
			if stopped.IntentID != "" {
				waitTargetOperation(t, manager, stopped.IntentID)
			}
		}
		_ = manager.Shutdown(ctx)
	}
	t.Cleanup(cleanupManager)
	return manager, started, uri, starter
}

func startTargetAuxSession(t *testing.T, manager *sessionruntime.Manager, index int) sessionruntime.StartResult {
	t.Helper()
	workspace := t.TempDir()
	profile, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "transient-test", Workspace: workspace, Profile: "fake", EnvironmentReference: "test"})
	if err != nil {
		t.Fatal(err)
	}
	started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(profile), BootstrapAlias: fmt.Sprintf("aux-alias-%d", index), LanguageID: "go"})
	if started.Failure != "" {
		t.Fatalf("auxiliary session startup failed: %+v", started)
	}
	pending := manager.BeginReadiness(context.Background(), started.SessionID, started.Generation, time.Now().Add(time.Second))
	ready, ok := manager.WaitReadiness(context.Background(), pending.ID)
	if !ok || ready.State != sessionruntime.ReadinessReady || ready.Failure != "" {
		t.Fatalf("auxiliary session readiness failed: %+v", ready)
	}
	return started
}

func waitTargetOperation(t *testing.T, manager *sessionruntime.Manager, operationID string) sessionruntime.OperationSnapshot {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if operation, ok := manager.Operation(operationID); ok && operation.State != sessionruntime.OperationPending {
			return operation
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("lifecycle operation did not finish: %s", operationID)
	return sessionruntime.OperationSnapshot{}
}

func targetResourceRequest(generation uint64, uri string, maxMessages int, maxBytes int64) Request {
	line, character := uint32(1), uint32(7)
	return Request{
		SessionID: "stable-alias", Generation: generation, LanguageID: "go",
		Target: Target{URI: uri, Line: &line, Character: &character}, SourceOnlyTarget: true,
		UpDepth: 0, DownDepth: 0, MaxNodes: 5, TimeoutMS: 1000, RequestTimeoutMS: 250,
		MaxMessages: maxMessages, MaxBytes: maxBytes, Analysis: AnalysisRequest{Kind: AnalysisNeighborhood}, CaptureSupply: true,
	}
}

func assertPoisonedResourceWireBeforeClassification(t *testing.T, ctx context.Context, manager *sessionruntime.Manager, started sessionruntime.StartResult, child *targetWire) {
	t.Helper()
	if err := ctx.Err(); err != nil {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_LIVE_CONTEXT: context=%v", err)
	}
	found := false
	for _, observation := range manager.Observations() {
		if observation.SessionID == started.SessionID && observation.Generation == started.Generation && observation.Kind == "response" && observation.Failure == session.ResourceExhausted && observation.State == session.Poisoned {
			found = true
		}
	}
	if !found {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_TYPED_POISONED_WIRE_RESULT: observations=%+v", manager.Observations())
	}
	records := manager.Records()
	if len(records) != 1 || records[0].Generation != started.Generation || records[0].State != session.Poisoned {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_EXACT_GENERATION_POISONED: records=%+v", records)
	}
	if _, failure := manager.Metadata(started.SessionID, started.Generation); failure != session.LifecycleConflict {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_RETIRED_METADATA: failure=%s", failure)
	}
	_, _, closes, teardowns, _, _ := child.snapshot()
	if closes != 1 || teardowns != 1 {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_TRANSPORT_RETIREMENT: close=%d teardown=%d", closes, teardowns)
	}
}

func assertTargetFailure(t *testing.T, failure *DomainFailure, action TargetAction) {
	t.Helper()
	if failure == nil || failure.Phase != PhasePreflight || failure.State != StateResourceLimit {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_DOMAIN_CLASSIFICATION: failure=%+v", failure)
	}
	if failure.TargetDiagnostic == nil || failure.TargetDiagnostic.Action != action {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_TARGET_ACTION: diagnostic=%+v", failure.TargetDiagnostic)
	}
}

func TestTargetResourceFailureMessageBudget(t *testing.T) {
	manager, started, uri, starter := newTargetResourceFixture(t, targetWirePlan{
		documentSymbolNotifications: 1,
		documentSymbolResult:        targetDocumentSymbols(""),
	})
	child := starter.child(0)
	ctx := context.Background()
	_, failure := Execute(ctx, manager, targetResourceRequest(started.Generation, uri, 1, 8192))
	assertPoisonedResourceWireBeforeClassification(t, ctx, manager, started, child)
	assertTargetFailure(t, failure, TargetActionFailDocument)
	notices, _, _, responseAttempted, responseWritten := waitTargetWireSnapshot(t, child, func(notices, _, _ int, responseAttempted, _ bool) bool {
		return notices == 1 && responseAttempted
	})
	if notices != 1 || !responseAttempted || responseWritten {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_MESSAGE_WIRE_SHAPE: notices=%d responseAttempted=%t responseWritten=%t", notices, responseAttempted, responseWritten)
	}
}

func TestTargetResourceFailureByteBudget(t *testing.T) {
	manager, started, uri, starter := newTargetResourceFixture(t, targetWirePlan{documentSymbolResult: oversizedTargetDocumentSymbols("")})
	child := starter.child(0)
	ctx := context.Background()
	_, failure := Execute(ctx, manager, targetResourceRequest(started.Generation, uri, 8, 1))
	assertPoisonedResourceWireBeforeClassification(t, ctx, manager, started, child)
	assertTargetFailure(t, failure, TargetActionFailDocument)
	_, notices, _, _, responseAttempted, responseWritten := child.snapshot()
	if notices != 0 || !responseAttempted || !responseWritten {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_BYTE_WIRE_SHAPE: notices=%d responseAttempted=%t responseWritten=%t", notices, responseAttempted, responseWritten)
	}
}

func TestTargetResourceFailureRecoveryLifecycle(t *testing.T) {
	manager, started, uri, starter := newTargetResourceFixture(t,
		targetWirePlan{documentSymbolNotifications: 2, documentSymbolResult: targetDocumentSymbols("")},
		targetWirePlan{documentSymbolResult: targetDocumentSymbols("")},
	)
	oldChild := starter.child(0)
	ctx := context.Background()
	request := targetResourceRequest(started.Generation, uri, 1, 8192)
	_, failure := Execute(ctx, manager, request)
	assertPoisonedResourceWireBeforeClassification(t, ctx, manager, started, oldChild)
	assertTargetFailure(t, failure, TargetActionFailDocument)

	beforeMethods, _, _, _, _, _ := oldChild.snapshot()
	_, retryFailure := Execute(ctx, manager, request)
	afterMethods, _, _, _, _, _ := oldChild.snapshot()
	if len(afterMethods) != len(beforeMethods) {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_SAME_GENERATION_NO_WIRE: before=%v after=%v", beforeMethods, afterMethods)
	}
	if retryFailure == nil {
		t.Fatal("ASSERT_TARGET_RESOURCE_FAILURE_SAME_GENERATION_RETRY_REMAINS_UNUSABLE: got nil failure")
	}

	restarted := manager.Restart(ctx, started.SessionID, "target-resource-test-restart")
	if restarted.Failure != "" || restarted.IntentID == "" {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_RESTART_ACCEPTED: result=%+v", restarted)
	}
	operation := waitTargetOperation(t, manager, restarted.IntentID)
	if operation.State != sessionruntime.OperationComplete || operation.Failure != "" {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_RESTART_COMPLETE: operation=%+v", operation)
	}
	records := manager.Records()
	if len(records) != 1 || records[0].Generation <= started.Generation || records[0].State != session.Ready {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_NEW_READY_GENERATION: records=%+v", records)
	}
	newGeneration := records[0].Generation
	if _, stale := manager.Metadata(started.SessionID, started.Generation); stale != session.StaleGeneration {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_OLD_GENERATION_UNUSABLE: failure=%s", stale)
	}
	beforeNewChild := starter.child(1)
	if beforeNewChild == nil {
		t.Fatal("ASSERT_TARGET_RESOURCE_FAILURE_REPLACEMENT_CHILD_EXISTS: missing child")
	}
	oldMethods, _, _, _, _, _ := oldChild.snapshot()
	_, oldGenerationFailure := Execute(ctx, manager, request)
	if oldGenerationFailure == nil {
		t.Fatal("ASSERT_TARGET_RESOURCE_FAILURE_OLD_GENERATION_EXECUTE_REJECTED: got nil failure")
	}
	stillOldMethods, _, _, _, _, _ := oldChild.snapshot()
	if !reflect.DeepEqual(stillOldMethods, oldMethods) {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_OLD_GENERATION_NO_WIRE: before=%v after=%v", oldMethods, stillOldMethods)
	}

	recovered := targetResourceRequest(newGeneration, uri, 1, 8192)
	_, recoveryFailure := Execute(ctx, manager, recovered)
	if recoveryFailure != nil {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_HEALTHY_REPLACEMENT_RECOVERY: failure=%+v", recoveryFailure)
	}
	methods, _, _, _, responseAttempted, responseWritten := beforeNewChild.snapshot()
	if !containsTargetMethod(methods, "textDocument/didOpen") || !containsTargetMethod(methods, "textDocument/documentSymbol") || !responseAttempted || !responseWritten {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_FRESH_DOCUMENT_AND_SYMBOL_REQUEST: methods=%v attempted=%t written=%t", methods, responseAttempted, responseWritten)
	}
	if got := targetResolutionState(ctx, "OTHER", Accounting{}, true, manager, started.SessionID, newGeneration); got != StateResourceLimit {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_RESOURCE_EVIDENCE_AFTER_METADATA_SUCCESS: got=%s", got)
	}
}

func containsTargetMethod(methods []string, want string) bool {
	for _, method := range methods {
		if method == want {
			return true
		}
	}
	return false
}

func TestTargetResourceFailurePrecedenceControls(t *testing.T) {
	manager, started, _, _ := newTargetResourceFixture(t)
	if got := targetResolutionState(context.Background(), "OTHER", Accounting{}, true, manager, started.SessionID, started.Generation+1); got != StateGenerationChanged {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_STALE_GENERATION_PRECEDENCE: got=%s", got)
	}
	if got := targetResolutionState(context.Background(), "OTHER", Accounting{}, true, manager, "missing-session", started.Generation); got != StateInvalidServerResponse {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_MISSING_GENERATION_PRECEDENCE: got=%s", got)
	}
	if got := targetResolutionState(context.Background(), "OTHER", Accounting{}, true, manager, started.SessionID, started.Generation); got != StateResourceLimit {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_METADATA_SUCCESS: got=%s", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := targetResolutionState(ctx, "OTHER", Accounting{}, true, manager, started.SessionID, started.Generation); got != StateCancelled {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_CANCEL_PRECEDENCE: got=%s", got)
	}
	deadline, cancelDeadline := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancelDeadline()
	if got := targetResolutionState(deadline, "OTHER", Accounting{}, true, manager, started.SessionID, started.Generation); got != StateTimeout {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_DEADLINE_PRECEDENCE: got=%s", got)
	}
	if got := targetResolutionState(context.Background(), "OTHER", Accounting{}, false, manager, started.SessionID, started.Generation); got != StateInvalidServerResponse {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_ABSENT_EVIDENCE_CONTROL: got=%s", got)
	}

	cases := []struct {
		code string
		want TerminalState
	}{
		{"POSITION_SYMBOL_ABSENT", StateTargetNotFound},
		{"DOCUMENT_SYMBOL_ABSENT", StateTargetNotFound},
		{"SOURCE_TARGET_ABSENT", StateTargetNotFound},
		{"ENUMERATION_TRUNCATED", StateTargetNotFound},
		{"DOCUMENT_SYMBOL_UNPREPARABLE", StateTargetNotFound},
		{"POSITION_PREPARE_MISMATCH", StateAmbiguousTarget},
		{"POSITION_PREPARE_AMBIGUOUS", StateAmbiguousTarget},
		{"POSITION_SYMBOL_AMBIGUOUS", StateAmbiguousTarget},
		{"DOCUMENT_SYMBOL_AMBIGUOUS", StateAmbiguousTarget},
		{"SOURCE_TARGET_AMBIGUOUS", StateAmbiguousTarget},
		{"DOCUMENT_SYMBOL_PREPARE_MISMATCH", StateAmbiguousTarget},
		{"DOCUMENT_SYMBOL_UNSUPPORTED", StateUnsupported},
		{"CANCELLED", StateCancelled},
		{"REQUEST_TIMEOUT", StateTimeout},
		{"DOCUMENT_SYMBOL_FAILED", StateInvalidServerResponse},
		{"DOCUMENT_SYMBOL_MALFORMED_RANGE", StateInvalidServerResponse},
		{"DOCUMENT_SYMBOL_PREPARE_FAILED", StateInvalidServerResponse},
	}
	for _, tc := range cases {
		if got := targetResolutionState(context.Background(), tc.code, Accounting{}, false, manager, started.SessionID, started.Generation); got != tc.want {
			t.Errorf("ASSERT_TARGET_RESOLUTION_CODE_EXHAUSTIVE_%s: got=%s want=%s", tc.code, got, tc.want)
		}
	}
	if got := targetResolutionState(ctx, "DOCUMENT_SYMBOL_ABSENT", Accounting{}, false, manager, started.SessionID, started.Generation); got != StateCancelled {
		t.Fatalf("ASSERT_TARGET_RESOLUTION_CANCEL_PRECEDES_CLASSIFICATION: got=%s", got)
	}
	if got := targetResolutionState(context.Background(), "DOCUMENT_SYMBOL_ABSENT", Accounting{}, true, manager, started.SessionID, started.Generation); got != StateResourceLimit {
		t.Fatalf("ASSERT_TARGET_RESOLUTION_RESOURCE_PRECEDES_CLASSIFICATION: got=%s", got)
	}
}

func TestTargetResourceFailureGenericMalformedResponseDoesNotPoison(t *testing.T) {
	manager, started, uri, starter := newTargetResourceFixture(t, targetWirePlan{documentSymbolResult: json.RawMessage(`{"not":"a document symbol list"}`)})
	ctx := context.Background()
	_, failure := Execute(ctx, manager, targetResourceRequest(started.Generation, uri, 8, 8192))
	if failure == nil || failure.State == StateResourceLimit {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_GENERIC_MALFORMED_NOT_RESOURCE: failure=%+v", failure)
	}
	if _, metadataFailure := manager.Metadata(started.SessionID, started.Generation); metadataFailure != "" {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_GENERIC_MALFORMED_NONPOISONING: metadata=%s", metadataFailure)
	}
	child := starter.child(0)
	_, closes, teardowns, _, responseWritten := waitTargetWireSnapshot(t, child, func(_ int, closes, teardowns int, _ bool, responseWritten bool) bool {
		return responseWritten || closes != 0 || teardowns != 0
	})
	if closes != 0 || teardowns != 0 || !responseWritten {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_GENERIC_MALFORMED_WIRE_CONTROL: close=%d teardown=%d written=%t", closes, teardowns, responseWritten)
	}
}

func TestTargetResourceFailureAdmissionLimitDoesNotPoison(t *testing.T) {
	manager, started, uri, starter := newTargetResourceFixture(t,
		targetWirePlan{documentSymbolResult: targetDocumentSymbols("")},
		targetWirePlan{holdDocumentSymbol: true},
		targetWirePlan{holdDocumentSymbol: true},
	)
	ctx := context.Background()
	document := manager.PrepareDocument(ctx, sessionruntime.DocumentRequest{SessionID: started.SessionID, Generation: started.Generation, URI: uri, LanguageID: "go", CaptureSupply: true})
	if document.Failure != "" || document.Supply == nil {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_ADMISSION_SETUP_DOCUMENT_CACHED: document=%+v", document)
	}
	auxA := startTargetAuxSession(t, manager, 1)
	auxB := startTargetAuxSession(t, manager, 2)

	holdA, cancelA := context.WithCancel(ctx)
	holdB, cancelB := context.WithCancel(ctx)
	defer cancelA()
	defer cancelB()
	doneA := make(chan sessionruntime.RoundTripResult, 1)
	doneB := make(chan sessionruntime.RoundTripResult, 1)
	params, _ := json.Marshal(map[string]any{"textDocument": map[string]string{"uri": uri}})
	go func() {
		doneA <- manager.RoundTrip(holdA, sessionruntime.RoundTripRequest{SessionID: auxA.SessionID, Generation: auxA.Generation, Method: "textDocument/documentSymbol", Params: params, Deadline: time.Now().Add(5 * time.Second), MaxMessages: 8, MaxBytes: 8192})
	}()
	go func() {
		doneB <- manager.RoundTrip(holdB, sessionruntime.RoundTripRequest{SessionID: auxB.SessionID, Generation: auxB.Generation, Method: "textDocument/documentSymbol", Params: params, Deadline: time.Now().Add(5 * time.Second), MaxMessages: 8, MaxBytes: 8192})
	}()
	deadline := time.Now().Add(time.Second)
	for manager.Census().Workers < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if manager.Census().Workers != 2 {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_ADMISSION_SETUP_WORKERS_SATURATED: census=%+v", manager.Census())
	}

	child := starter.child(0)
	_, failure := Execute(ctx, manager, targetResourceRequest(started.Generation, uri, 8, 8192))
	if failure == nil || failure.State != StateResourceLimit {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_NONPOISONING_ADMISSION_CLASSIFICATION: failure=%+v", failure)
	}
	if _, metadataFailure := manager.Metadata(started.SessionID, started.Generation); metadataFailure != "" {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_ADMISSION_DOES_NOT_POISON_TARGET: metadata=%s", metadataFailure)
	}
	methods, _, closes, teardowns, _, _ := child.snapshot()
	if closes != 0 || teardowns != 0 || containsTargetMethod(methods, "textDocument/documentSymbol") {
		t.Fatalf("ASSERT_TARGET_RESOURCE_FAILURE_ADMISSION_NO_TARGET_WIRE_OR_RETIREMENT: methods=%v close=%d teardown=%d", methods, closes, teardowns)
	}
	cancelA()
	cancelB()
	<-doneA
	<-doneB
}
