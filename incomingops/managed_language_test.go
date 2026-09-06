package incomingops

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/source"
	"lsp-trace/sessionruntime"
)

type managedLanguageChild struct {
	input  *io.PipeReader
	stdin  *io.PipeWriter
	output *io.PipeWriter
	stdout *io.PipeReader
	frames chan lspwire.Message
}

func newManagedLanguageChild() *managedLanguageChild {
	input, stdin := io.Pipe()
	stdout, output := io.Pipe()
	child := &managedLanguageChild{input: input, stdin: stdin, output: output, stdout: stdout, frames: make(chan lspwire.Message, 8)}
	go child.serve()
	return child
}

func (c *managedLanguageChild) serve() {
	reader := lspwire.NewReader(c.input, lspwire.DefaultLimits())
	writer := lspwire.NewWriter(c.output, lspwire.DefaultLimits())
	for {
		message, err := reader.Read()
		if err != nil {
			return
		}
		c.frames <- message
		switch message.Method {
		case "initialize":
			_ = writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: message.ID, Result: json.RawMessage(`{"capabilities":{"callHierarchyProvider":true}}`)})
		case "textDocument/prepareCallHierarchy":
			_ = writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: message.ID, Result: json.RawMessage(`[]`)})
		case "shutdown":
			_ = writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: message.ID, Result: json.RawMessage(`null`)})
		}
	}
}

func (c *managedLanguageChild) Stdin() io.WriteCloser { return c.stdin }
func (c *managedLanguageChild) Stdout() io.ReadCloser { return c.stdout }
func (c *managedLanguageChild) Teardown(context.Context) managedprocess.TeardownObservation {
	_ = c.stdin.Close()
	_ = c.output.Close()
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}
func (c *managedLanguageChild) Close() managedprocess.ResourceObservation {
	_ = c.stdout.Close()
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}

type managedLanguageStarter struct{ child *managedLanguageChild }

func (s managedLanguageStarter) Start(context.Context, managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	return s.child, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
}

func managedLanguageProfile(t *testing.T, workspace string) runtimeprofile.Profile {
	t.Helper()
	validated, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: workspace, Profile: "typescript", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	return runtimeprofile.Resolve(validated)
}

func readyManagedLanguageManager(t *testing.T, workspace, configuredLanguage string) (*sessionruntime.Manager, sessionruntime.StartResult, *managedLanguageChild) {
	t.Helper()
	child := newManagedLanguageChild()
	manager, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 1, MaxTombstones: 2, MaxObservations: 32, MaxOperations: 2}, Starter: managedLanguageStarter{child: child}})
	if err != nil {
		t.Fatal(err)
	}
	started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: managedLanguageProfile(t, workspace), LanguageID: configuredLanguage})
	ready := manager.BeginReadiness(context.Background(), started.SessionID, started.Generation, time.Now().Add(time.Second))
	if terminal, ok := manager.WaitReadiness(context.Background(), ready.ID); !ok || terminal.Failure != "" {
		t.Fatalf("readiness=%+v found=%t", terminal, ok)
	}
	for i := 0; i < 2; i++ {
		<-child.frames
	}
	return manager, started, child
}

func TestManagedPlainJavaScriptOpensWithEffectiveLanguageBeforePreparation(t *testing.T) {
	const assertion = "ASSERT_FR11_MANAGED_JS_EFFECTIVE_LANGUAGE_DIDOPEN"
	workspace := t.TempDir()
	path := filepath.Join(workspace, "calls.js")
	if err := os.WriteFile(path, []byte("function calls() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	uri, err := source.FileURI(path)
	if err != nil {
		t.Fatal(err)
	}
	manager, started, child := readyManagedLanguageManager(t, workspace, "")

	_, failure := NewExecutor(manager).Execute(context.Background(), operation.Request{Name: OperationIncoming, Input: json.RawMessage(`{"session_id":"` + started.SessionID + `","generation":1,"uri":"` + uri + `","line":0,"character":0}`)})
	if failure != nil {
		t.Fatalf("%s: execute failure=%+v", assertion, failure)
	}
	frame := <-child.frames
	if frame.Method != "textDocument/didOpen" || !json.Valid(frame.Params) || !containsJSON(frame.Params, `"uri":"`+uri+`"`, `"languageId":"javascript"`) {
		t.Fatalf("%s: first_document_frame=%s params=%s", assertion, frame.Method, frame.Params)
	}
}

func TestManagedLanguageResolutionFailClosedAndPrecedence(t *testing.T) {
	workspace := t.TempDir()
	unknown := filepath.Join(workspace, "calls.unknown")
	if err := os.WriteFile(unknown, []byte("function calls() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	uri, err := source.FileURI(unknown)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("unknown sends nothing", func(t *testing.T) {
		manager, started, child := readyManagedLanguageManager(t, workspace, "")
		_, failure := NewExecutor(manager).Execute(context.Background(), operation.Request{Name: OperationIncoming, Input: json.RawMessage(`{"session_id":"` + started.SessionID + `","generation":1,"uri":"` + uri + `","line":0,"character":0}`)})
		if failure == nil || failure.Code != string(sessionruntime.LanguageIDUnavailable) {
			t.Fatalf("ASSERT_FR11_UNKNOWN_FAIL_CLOSED: failure=%+v", failure)
		}
		select {
		case frame := <-child.frames:
			t.Fatalf("ASSERT_FR11_UNKNOWN_NO_LSP_REQUEST: frame=%+v", frame)
		default:
		}
	})
	t.Run("explicit wins", func(t *testing.T) {
		manager, started, child := readyManagedLanguageManager(t, workspace, "typescript")
		_, failure := NewExecutor(manager).Execute(context.Background(), operation.Request{Name: OperationIncoming, Input: json.RawMessage(`{"session_id":"` + started.SessionID + `","generation":1,"uri":"` + uri + `","language_id":"javascript","line":0,"character":0}`)})
		if failure != nil {
			t.Fatal(failure)
		}
		frame := <-child.frames
		if !containsJSON(frame.Params, `"languageId":"javascript"`) {
			t.Fatalf("ASSERT_FR11_EXPLICIT_LANGUAGE_PRECEDENCE: %s", frame.Params)
		}
	})
	t.Run("configured wins", func(t *testing.T) {
		manager, started, child := readyManagedLanguageManager(t, workspace, "typescript")
		_, failure := NewExecutor(manager).Execute(context.Background(), operation.Request{Name: OperationIncoming, Input: json.RawMessage(`{"session_id":"` + started.SessionID + `","generation":1,"uri":"` + uri + `","line":0,"character":0}`)})
		if failure != nil {
			t.Fatal(failure)
		}
		frame := <-child.frames
		if !containsJSON(frame.Params, `"languageId":"typescript"`) {
			t.Fatalf("ASSERT_FR11_CONFIGURED_LANGUAGE_PRECEDENCE: %s", frame.Params)
		}
	})
}

func TestManagedDocumentReuseChangeAndConflict(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(workspace, "calls.js")
	if err := os.WriteFile(path, []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	uri, _ := source.FileURI(path)
	manager, started, child := readyManagedLanguageManager(t, workspace, "")
	request := sessionruntime.DocumentRequest{SessionID: started.SessionID, Generation: 1, URI: uri}
	first := manager.PrepareDocument(context.Background(), request)
	if first.Failure != "" || first.Version != 1 {
		t.Fatalf("ASSERT_FR11_FIRST_LEASE: %+v", first)
	}
	<-child.frames
	reused := manager.PrepareDocument(context.Background(), request)
	if reused.Failure != "" || reused.Version != 1 {
		t.Fatalf("ASSERT_FR11_REUSED_LEASE: %+v", reused)
	}
	select {
	case frame := <-child.frames:
		t.Fatalf("ASSERT_FR11_REUSE_NO_SYNC: %+v", frame)
	default:
	}
	if err := os.WriteFile(path, []byte("two\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	changed := manager.PrepareDocument(context.Background(), request)
	frame := <-child.frames
	if changed.Version != 2 || frame.Method != "textDocument/didChange" {
		t.Fatalf("ASSERT_FR11_REOPEN_DIDCHANGE: result=%+v frame=%+v", changed, frame)
	}
	request.LanguageID = "typescript"
	if conflict := manager.PrepareDocument(context.Background(), request); conflict.Failure == "" {
		t.Fatalf("ASSERT_FR11_LANGUAGE_LEASE_CONFLICT: %+v", conflict)
	}
}

func containsJSON(raw []byte, fragments ...string) bool {
	text := string(raw)
	for _, fragment := range fragments {
		found := false
		for i := 0; i+len(fragment) <= len(text); i++ {
			if text[i:i+len(fragment)] == fragment {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
