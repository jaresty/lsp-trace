package requestlifecycle

import (
	"context"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

type projectionChild struct {
	input  *io.PipeReader
	stdin  *io.PipeWriter
	output *io.PipeWriter
	stdout *io.PipeReader
}

func newProjectionChild() *projectionChild {
	input, stdin := io.Pipe()
	stdout, output := io.Pipe()
	c := &projectionChild{input: input, stdin: stdin, output: output, stdout: stdout}
	go c.serve()
	return c
}
func (c *projectionChild) serve() {
	r := lspwire.NewReader(c.input, lspwire.DefaultLimits())
	w := lspwire.NewWriter(c.output, lspwire.DefaultLimits())
	for {
		msg, err := r.Read()
		if err != nil {
			return
		}
		switch msg.Method {
		case "initialize":
			_ = w.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: msg.ID, Result: json.RawMessage(`{"capabilities":{"positionEncoding":"utf-8","callHierarchyProvider":true,"documentSymbolProvider":true}}`)})
		case "textDocument/documentSymbol":
			_ = w.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: msg.ID, Result: json.RawMessage(`[]`)})
		}
	}
}
func (c *projectionChild) Stdin() io.WriteCloser { return c.stdin }
func (c *projectionChild) Stdout() io.ReadCloser { return c.stdout }
func (c *projectionChild) Teardown(context.Context) managedprocess.TeardownObservation {
	_ = c.stdin.Close()
	_ = c.input.Close()
	_ = c.output.Close()
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}
func (c *projectionChild) Close() managedprocess.ResourceObservation {
	_ = c.stdout.Close()
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}

type projectionStarter struct{ child sessionruntime.Child }

func (s projectionStarter) Start(context.Context, managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	return s.child, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
}

func projectionRuntime(t *testing.T) (*sessionruntime.Manager, sessionruntime.StartResult, sessionruntime.ReadinessSnapshot, sessionruntime.DocumentResult, sessionruntime.RoundTripResult) {
	t.Helper()
	workspace := t.TempDir()
	path := filepath.Join(workspace, "a.go")
	if err := os.WriteFile(path, []byte("package a\n"), 0600); err != nil {
		t.Fatal(err)
	}
	validated, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: workspace, Profile: "fake", EnvironmentReference: "test"})
	if err != nil {
		t.Fatal(err)
	}
	child := newProjectionChild()
	store := manageddiagnostic.NewStore(manageddiagnostic.Bounds{MaxRecords: 64, MaxBytes: 65536})
	m, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 4, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 8, MaxObservations: 64, MaxOperations: 8}, Starter: projectionStarter{child}, Diagnostics: store})
	if err != nil {
		t.Fatal(err)
	}
	started := m.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(validated)})
	pending := m.BeginReadiness(context.Background(), started.SessionID, started.Generation, time.Now().Add(time.Second))
	ready, ok := m.WaitReadiness(context.Background(), pending.ID)
	if !ok || ready.State != sessionruntime.ReadinessReady {
		t.Fatalf("readiness: %+v", ready)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
	document := m.PrepareDocument(context.Background(), sessionruntime.DocumentRequest{SessionID: started.SessionID, Generation: started.Generation, URI: uri, LanguageID: "go", CaptureSupply: true})
	if document.Failure != "" {
		t.Fatal(document.Failure)
	}
	params, _ := json.Marshal(map[string]any{"textDocument": map[string]string{"uri": uri}})
	symbols := m.RoundTrip(context.Background(), sessionruntime.RoundTripRequest{SessionID: started.SessionID, Generation: started.Generation, Method: "textDocument/documentSymbol", Params: params, Deadline: time.Now().Add(time.Second), MaxMessages: 2, MaxBytes: 4096})
	if symbols.Failure != "" {
		t.Fatal(symbols.Failure)
	}
	return m, started, ready, document, symbols
}

func TestRuntimeProjectionAcceptsOnlyCertifiedExactBoundedSnapshots(t *testing.T) {
	m, started, ready, document, symbols := projectionRuntime(t)
	handles := []sessionruntime.DiagnosticOperationHandle{ready.DiagnosticOperation, document.DiagnosticOperation, symbols.DiagnosticOperation}
	set, ok := m.DiagnosticSnapshotSetFor(started.AttemptID, started.DiagnosticGeneration, handles, MaxRecords)
	if !ok {
		t.Fatal("ASSERT_STAGE2_CERTIFIED_SET")
	}
	raw, err := projectRuntime(set)
	if err != nil {
		t.Fatal(err)
	}
	model, err := Verify(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	if model.Initialize.Status != "MATCHED" || !model.Initialize.DocumentSymbolCapability || len(model.Documents) != 1 || model.Documents[0].DocumentSymbolOperationID == "" {
		t.Fatalf("ASSERT_STAGE2_OPERATION_PROJECTION: %+v", model)
	}
	ops := set.Operations()
	ops[0].Events.Events[0].Code = 0xffff
	if again := set.Operations(); again[0].Events.Events[0].Code == 0xffff {
		t.Fatal("ASSERT_STAGE2_IMMUTABLE_SET")
	}
	if _, err := projectRuntime(sessionruntime.DiagnosticSnapshotSet{}); err == nil {
		t.Fatal("ASSERT_STAGE2_FORGED_SET_REJECTED")
	}
	if _, ok := m.DiagnosticSnapshotSetFor(started.AttemptID, started.DiagnosticGeneration, append(handles, handles[0]), MaxRecords); ok {
		t.Fatal("ASSERT_STAGE2_DUPLICATE_EVENT_SOURCE_REJECTED")
	}
	if _, ok := m.DiagnosticSnapshotSetFor(started.AttemptID, sessionruntime.DiagnosticGenerationHandle{}, handles, MaxRecords); ok {
		t.Fatal("ASSERT_STAGE2_STALE_GENERATION_REJECTED")
	}
	if !strings.Contains(string(raw), `"length":0`) {
		t.Fatal("ASSERT_STAGE2_PUBLIC_BINDING_PLACEHOLDER")
	}
}
