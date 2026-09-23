//go:build darwin

package sessionruntime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
)

// This is a managed local-process fixture, not a provider-authenticated method
// receipt or evidence that a definition occurrence has been admitted.
func TestManagedProcessDefinitionCompletedWriteObservation(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "fake-lsp")
	build := exec.Command("go", "build", "-o", binary, "./cmd/fake-lsp")
	build.Dir = root
	build.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build fake-lsp offline: %v: %s", err, output)
	}
	supervisor, err := managedprocess.NewLocalDarwinSupervisor(managedprocess.Options{StderrLimit: 4096, GracePeriod: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := New(Config{
		Limits:  Limits{MaxSessions: 1, MaxRequests: 2, MaxChildren: 1, MaxCancels: 2, MaxTombstones: 2, MaxObservations: 16},
		Starter: ManagedStarter{Manager: supervisor},
	})
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "scheduled")
	trace := filepath.Join(t.TempDir(), "methods.log")
	started := manager.Start(context.Background(), StartRequest{
		Profile: profile(t),
		Process: managedprocess.Spec{Path: binary, Dir: root, Env: append(os.Environ(), "LSP_TRACE_FAKE_LSP_SCHEDULED="+marker, "LSP_TRACE_FAKE_LSP_TRACE="+trace)},
	})
	if started.Failure != "" || started.Start.Evidence != managedprocess.LocalDarwinSupervisionOnly {
		t.Fatalf("ASSERT_MANAGED_WRITE_PROCESS_START: %+v", started)
	}
	t.Cleanup(func() {
		stop := manager.Stop(context.Background(), started.SessionID, "managed-write-fixture")
		if stop.Failure != "" {
			t.Errorf("ASSERT_MANAGED_WRITE_PROCESS_STOP: %+v", stop)
			return
		}
		terminal := waitOperation(t, manager, stop.IntentID, OperationComplete)
		if terminal.Failure != "" {
			t.Errorf("ASSERT_MANAGED_WRITE_PROCESS_REAP: %+v", terminal)
		}
	})
	waitForFakeLSPScheduled(t, marker, localDarwinTestDeadline(t))
	pending := manager.BeginReadiness(context.Background(), started.SessionID, started.Generation, localDarwinTestDeadline(t))
	if pending.State != ReadinessPending {
		t.Fatalf("ASSERT_MANAGED_WRITE_PROCESS_READINESS_PENDING: %+v", pending)
	}
	ready, found := manager.WaitReadiness(context.Background(), pending.ID)
	if !found || ready.State != ReadinessReady || ready.Failure != "" {
		t.Fatalf("ASSERT_MANAGED_WRITE_PROCESS_READY: found=%v ready=%+v", found, ready)
	}

	params := json.RawMessage(`{"textDocument":{"uri":"file:///fixture/main.go"},"position":{"line":1,"character":2}}`)
	req := RoundTripRequest{SessionID: started.SessionID, Generation: started.Generation, Method: "textDocument/definition", Params: params, Deadline: time.Now().Add(5 * time.Second), MaxMessages: 1, MaxBytes: 4096, CaptureDefinitionResponseFrameMaxBytes: 4096}
	result := manager.RoundTrip(context.Background(), req)
	if result.Failure != "" || result.ServerError == nil || result.ServerError.Code != -32601 || result.Messages != 1 || result.Bytes <= 0 {
		t.Fatalf("ASSERT_MANAGED_WRITE_PROCESS_METHOD_NOT_FOUND: failure=%s error=%+v messages=%d bytes=%d", result.Failure, result.ServerError, result.Messages, result.Bytes)
	}
	methods, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, method := range strings.Split(strings.TrimSpace(string(methods)), "\n") {
		if method == req.Method {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("ASSERT_MANAGED_WRITE_PROCESS_METHOD_RECEIVED_ONCE: count=%d trace=%q", count, methods)
	}
	observed, ok := result.CompletedRequestWrite()
	if !ok || observed.SessionID != req.SessionID || observed.Generation != req.Generation || observed.Key != result.Key || observed.Method != req.Method {
		t.Fatalf("ASSERT_MANAGED_WRITE_PROCESS_OWNER_BINDING: present=%v observation=%+v key=%+v", ok, observed, result.Key)
	}
	body, err := json.Marshal(lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(strconv.FormatUint(result.Key.ID, 10)), Method: req.Method, Params: params})
	if err != nil {
		t.Fatal(err)
	}
	frame := append([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))), body...)
	if observed.FrameBytes != int64(len(frame)) || observed.FrameSHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256(frame)) {
		t.Fatalf("ASSERT_MANAGED_WRITE_PROCESS_EXACT_FRAME: bytes=%d digest=%s", observed.FrameBytes, observed.FrameSHA256)
	}
	read, ok := result.CompletedResponseRead()
	if !ok || read.SessionID != req.SessionID || read.Generation != req.Generation || read.Key != result.Key {
		t.Fatalf("ASSERT_MANAGED_READ_PROCESS_OWNER_BINDING: present=%v observation=%+v", ok, read)
	}
	responseBody, err := json.Marshal(lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(strconv.FormatUint(result.Key.ID, 10)), Error: result.ServerError})
	if err != nil {
		t.Fatal(err)
	}
	responseFrame := append([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(responseBody))), responseBody...)
	if read.FrameBytes != int64(len(responseFrame)) || read.FrameSHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256(responseFrame)) {
		t.Fatalf("ASSERT_MANAGED_READ_PROCESS_EXACT_FRAME: bytes=%d digest=%s", read.FrameBytes, read.FrameSHA256)
	}
	if raw, retained := result.CompletedDefinitionResponseFrame(); retained || raw != nil {
		t.Fatalf("ASSERT_ADR0011_MANAGER_PROCESS_ERROR_RAW_WITHHELD: retained=%v raw=%q", retained, raw)
	}
	t.Log("ASSERT_ADR0011_MANAGER_PROCESS_ERROR_RAW_WITHHELD: PASS")
}
