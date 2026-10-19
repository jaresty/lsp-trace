package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/requestlifecycle"
)

func buildFR23Binary(t *testing.T, name, pkg string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	cmd := exec.Command("go", "build", "-o", path, pkg)
	cmd.Dir = filepath.Join("..", "..")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v: %s", pkg, err, out)
	}
	return path
}

func runFR23WithScheduledFake(t *testing.T, cmd *exec.Cmd) error {
	return runFR23WithScheduledFakeDeadline(t, cmd, 30*time.Second)
}

func runFR23WithScheduledFakeDeadline(t *testing.T, cmd *exec.Cmd, timeout time.Duration) error {
	t.Helper()
	socketFile, err := os.CreateTemp("", "fr23-scheduled-*.sock")
	if err != nil {
		t.Fatalf("allocate fake-lsp scheduling barrier: %v", err)
	}
	socket := socketFile.Name()
	if err := socketFile.Close(); err != nil {
		t.Fatalf("close fake-lsp scheduling barrier placeholder: %v", err)
	}
	if err := os.Remove(socket); err != nil {
		t.Fatalf("prepare fake-lsp scheduling barrier: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(socket) })
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen for fake-lsp scheduling barrier: %v", err)
	}
	defer listener.Close()

	var capturedStderr bytes.Buffer
	if cmd.Stderr == nil {
		cmd.Stderr = &capturedStderr
	} else {
		cmd.Stderr = io.MultiWriter(cmd.Stderr, &capturedStderr)
	}
	cmd.Env = append(cmd.Env, "LSP_TRACE_FAKE_LSP_SCHEDULE_BARRIER="+socket)
	if err := cmd.Start(); err != nil {
		return err
	}

	type acceptResult struct {
		conn net.Conn
		err  error
	}
	accepted := make(chan acceptResult, 1)
	go func() {
		conn, err := listener.Accept()
		accepted <- acceptResult{conn: conn, err: err}
	}()
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	deadline := time.Now().Add(timeout)

	closeListenerAndJoinAccept := func() {
		_ = listener.Close()
		result := <-accepted
		if result.conn != nil {
			_ = result.conn.Close()
		}
	}
	killAndReap := func() error {
		select {
		case waitErr := <-waited:
			return waitErr
		default:
		}
		_ = cmd.Process.Kill()
		return <-waited
	}
	timeoutError := func() error {
		closeListenerAndJoinAccept()
		waitErr := killAndReap()
		return fmt.Errorf("timed out after %s waiting for fake-lsp scheduling barrier; child reaped: %v; stderr=%s", timeout, waitErr, capturedStderr.String())
	}

	var conn net.Conn
	select {
	case result := <-accepted:
		if result.err != nil {
			waitErr := killAndReap()
			return fmt.Errorf("accept fake-lsp scheduling barrier: %w; child=%v; stderr=%s", result.err, waitErr, capturedStderr.String())
		}
		conn = result.conn
	case waitErr := <-waited:
		closeListenerAndJoinAccept()
		return fmt.Errorf("child exited before fake-lsp scheduling barrier: %v; stderr=%s", waitErr, capturedStderr.String())
	case <-timer.C:
		return timeoutError()
	}
	defer conn.Close()
	if err := conn.SetDeadline(deadline); err != nil {
		_ = killAndReap()
		return fmt.Errorf("deadline fake-lsp scheduling barrier: %w", err)
	}

	var scheduled [10]byte
	if _, err := io.ReadFull(conn, scheduled[:]); err != nil || string(scheduled[:]) != "scheduled\n" {
		waitErr := killAndReap()
		if timeoutErr, ok := err.(net.Error); ok && timeoutErr.Timeout() {
			return fmt.Errorf("timed out after %s reading fake-lsp scheduling marker; child reaped: %v; stderr=%s", timeout, waitErr, capturedStderr.String())
		}
		return fmt.Errorf("ASSERT_FR23_PRIVATE_BUILT_CHILD_SCHEDULED: marker=%q err=%v child=%v stderr=%s", scheduled, err, waitErr, capturedStderr.String())
	}
	if os.Getenv("LSP_TRACE_FR23_WITHHOLD_SCHEDULE_RELEASE") != "1" {
		if _, err := conn.Write([]byte{1}); err != nil {
			waitErr := killAndReap()
			return fmt.Errorf("ASSERT_FR23_PRIVATE_BUILT_CHILD_RELEASED: %v child=%v stderr=%s", err, waitErr, capturedStderr.String())
		}
	}
	_ = conn.Close()
	select {
	case waitErr := <-waited:
		return waitErr
	case <-timer.C:
		waitErr := killAndReap()
		return fmt.Errorf("timed out after %s waiting for child after fake-lsp scheduling release; child reaped: %v; stderr=%s", timeout, waitErr, capturedStderr.String())
	}
}

func TestFR23ScheduledFakeHelperProcess(t *testing.T) {
	mode := os.Getenv("LSP_TRACE_FR23_HELPER_PROCESS")
	if mode == "" {
		return
	}
	switch mode {
	case "exit-without-dial":
		_, _ = os.Stderr.WriteString("helper exited before dialing\n")
		os.Exit(23)
	case "stay-alive-without-dial":
		time.Sleep(time.Hour)
	case "dial-and-release":
		conn, err := net.Dial("unix", os.Getenv("LSP_TRACE_FAKE_LSP_SCHEDULE_BARRIER"))
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		if _, err := conn.Write([]byte("scheduled\n")); err != nil {
			t.Fatal(err)
		}
		var release [1]byte
		if _, err := io.ReadFull(conn, release[:]); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("unknown helper process mode %q", mode)
	}
}

func fr23HelperCommand(t *testing.T, mode string) (*exec.Cmd, *bytes.Buffer) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestFR23ScheduledFakeHelperProcess$")
	cmd.Env = append(os.Environ(), "LSP_TRACE_FR23_HELPER_PROCESS="+mode)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	return cmd, &stderr
}

func TestRunFR23WithScheduledFakeChildExitsWithoutDial(t *testing.T) {
	cmd, stderr := fr23HelperCommand(t, "exit-without-dial")
	started := time.Now()
	err := runFR23WithScheduledFake(t, cmd)
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("ASSERT_FR23_SCHEDULE_BARRIER_CHILD_EXIT_PROMPT: elapsed=%s", elapsed)
	}
	if err == nil || !strings.Contains(err.Error(), "child exited before fake-lsp scheduling barrier") {
		t.Fatalf("ASSERT_FR23_SCHEDULE_BARRIER_CHILD_EXIT_DIAGNOSTIC: err=%v", err)
	}
	if !strings.Contains(err.Error(), "helper exited before dialing") || !strings.Contains(stderr.String(), "helper exited before dialing") {
		t.Fatalf("ASSERT_FR23_SCHEDULE_BARRIER_CHILD_EXIT_STDERR: err=%v stderr=%q", err, stderr.String())
	}
	if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
		t.Fatalf("ASSERT_FR23_SCHEDULE_BARRIER_CHILD_EXIT_REAPED: state=%v", cmd.ProcessState)
	}
}

func TestRunFR23WithScheduledFakeDeadlineWithoutDial(t *testing.T) {
	cmd, _ := fr23HelperCommand(t, "stay-alive-without-dial")
	started := time.Now()
	err := runFR23WithScheduledFakeDeadline(t, cmd, 150*time.Millisecond)
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("ASSERT_FR23_SCHEDULE_BARRIER_DEADLINE_PROMPT: elapsed=%s", elapsed)
	}
	if err == nil || !strings.Contains(err.Error(), "timed out") || !strings.Contains(err.Error(), "waiting for fake-lsp scheduling barrier") {
		t.Fatalf("ASSERT_FR23_SCHEDULE_BARRIER_DEADLINE_DIAGNOSTIC: err=%v", err)
	}
	if cmd.ProcessState == nil {
		t.Fatalf("ASSERT_FR23_SCHEDULE_BARRIER_DEADLINE_REAPED: state=%v", cmd.ProcessState)
	}
}

func TestRunFR23WithScheduledFakeSuccessfulDial(t *testing.T) {
	cmd, stderr := fr23HelperCommand(t, "dial-and-release")
	if err := runFR23WithScheduledFakeDeadline(t, cmd, 2*time.Second); err != nil {
		t.Fatalf("ASSERT_FR23_SCHEDULE_BARRIER_SUCCESS: err=%v stderr=%q", err, stderr.String())
	}
	if cmd.ProcessState == nil || !cmd.ProcessState.Success() {
		t.Fatalf("ASSERT_FR23_SCHEDULE_BARRIER_SUCCESS_REAPED: state=%v", cmd.ProcessState)
	}
}

func TestManagedV5ZeroExactRelationsIsExplicitSuccessfulOutcome(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("managed process CLI uses Darwin supervisor")
	}
	cli := buildFR23Binary(t, "lsp-trace", "./cmd/lsp-trace")
	fake := buildFR23Binary(t, "fake-lsp", "./cmd/fake-lsp")
	workspace := t.TempDir()
	sourcePath := filepath.Join(workspace, "main.go")
	if err := os.WriteFile(sourcePath, []byte("leaf\n"), 0600); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: sourcePath}).String()
	target := map[string]any{"id": "root", "locator": map[string]any{"uri": uri, "line": 0, "character": 0}, "down_depth": 0, "up_depth": 0}
	manifest := map[string]any{
		"schema_version": "lsp-trace.seed-manifest.v2", "coordinate_convention": "zero-based-session",
		"root": target, "required_targets": []any{}, "expansion": map[string]any{"topmost_siblings": true},
		"limits": map[string]any{"max_nodes": 4, "max_requests": 8, "timeout_ms": 5000, "request_timeout_ms": 1000},
	}
	manifestRaw, _ := json.Marshal(manifest)
	manifestPath := filepath.Join(t.TempDir(), "seeds.json")
	if err := os.WriteFile(manifestPath, manifestRaw, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(cli, "slice", "--acquisition-version", "v3", "--output-version", graphprovenance.VersionV5, "--workspace", workspace, "--server", fake, "--server-env", "LSP_TRACE_FAKE_LSP_DOCUMENT_SYMBOL=mismatch", "--seed-manifest", manifestPath, "--language-id", "go")
	cmd.Env = append(os.Environ(), "LSP_TRACE_FAKE_LSP_DOCUMENT_SYMBOL=mismatch")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := runFR23WithScheduledFake(t, cmd); err != nil {
		t.Fatalf("ASSERT_MANAGED_V5_ZERO_EXACT_RELATIONS_SUCCEEDS: %v stderr=%s", err, stderr.String())
	}
	if strings.Contains(stderr.String(), "topmost sibling expansion produced no exact relations") {
		t.Fatalf("ASSERT_MANAGED_V5_ZERO_EXACT_RELATIONS_NOT_ERROR: %s", stderr.String())
	}
	if _, err := graphprovenance.ValidateFor(stdout.Bytes(), graphprovenance.Family, "v5"); err != nil {
		t.Fatalf("ASSERT_MANAGED_V5_ZERO_EXACT_RELATIONS_VALID_V5: %v", err)
	}
	var envelope graphprovenance.EvidenceV5
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	native, err := base64.StdEncoding.DecodeString(envelope.GraphV5)
	if err != nil {
		t.Fatal(err)
	}
	var result graph.Result
	if err := json.Unmarshal(native, &result); err != nil {
		t.Fatal(err)
	}
	if result.Invocation.Expansion.TopmostSiblingOutcome != graph.TopmostSiblingNoExactRelations {
		t.Fatalf("ASSERT_MANAGED_V5_ZERO_EXACT_RELATIONS_EXPLICIT_OUTCOME: %q", result.Invocation.Expansion.TopmostSiblingOutcome)
	}
}

func TestFR23BuiltCLIPrivateRequestFailureDiagnostic(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("managed process CLI uses Darwin supervisor")
	}
	cli := buildFR23Binary(t, "lsp-trace", "./cmd/lsp-trace")
	fake := buildFR23Binary(t, "fake-lsp", "./cmd/fake-lsp")
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("leaf\n"), 0600); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.Join(workspace, "main.go")}).String()
	target := map[string]any{"id": "root", "locator": map[string]any{"uri": uri, "line": 0, "character": 0}, "down_depth": 0, "up_depth": 0}
	manifest := map[string]any{"schema_version": "lsp-trace.seed-manifest.v2", "coordinate_convention": "zero-based-session", "root": target, "required_targets": []any{}, "limits": map[string]any{"max_nodes": 1, "max_requests": 2, "timeout_ms": 1000, "request_timeout_ms": 500}}
	manifestRaw, _ := json.Marshal(manifest)
	manifestPath := filepath.Join(t.TempDir(), "seeds.json")
	if err := os.WriteFile(manifestPath, manifestRaw, 0600); err != nil {
		t.Fatal(err)
	}
	privateRoot := t.TempDir()
	if err := os.Chmod(privateRoot, 0700); err != nil {
		t.Fatal(err)
	}
	privatePath := filepath.Join(privateRoot, "private.json")
	args := []string{"slice", "--acquisition-version", "v3", "--workspace", workspace, "--server", fake, "--seed-manifest", manifestPath, "--language-id", "go", "--private-request-diagnostic-root", privateRoot, "--private-request-diagnostic-selector", "private.json"}
	cmd := exec.Command(cli, args...)
	cmd.Env = append(os.Environ(), "LSP_TRACE_FAKE_LSP_HANG_PREPARE=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := runFR23WithScheduledFake(t, cmd); err != nil {
		t.Fatalf("ASSERT_FR23_PRIVATE_BUILT_CLI_RUN: %v stderr=%s", err, stderr.String())
	}
	if _, err := graphprovenance.ValidateFor(stdout.Bytes(), graphprovenance.Family, "v3"); err != nil {
		t.Fatalf("ASSERT_FR23_PRIVATE_PUBLIC_V3_UNCHANGED: %v", err)
	}
	if strings.Contains(strings.ToLower(stderr.String()), "timeout") || strings.Contains(strings.ToLower(stderr.String()), "private.json") {
		t.Fatalf("ASSERT_FR23_PRIVATE_PUBLIC_STDERR_GENERIC: %s", stderr.String())
	}
	privateRaw, err := os.ReadFile(privatePath)
	if err != nil {
		t.Fatalf("ASSERT_FR23_PRIVATE_BUILT_ARTIFACT_PRESENT: %v stderr=%s", err, stderr.String())
	}
	d, err := requestlifecycle.Verify(privateRaw, stdout.Bytes())
	if err != nil {
		t.Fatalf("ASSERT_FR23_PRIVATE_BUILT_ARTIFACT_VALID: %v", err)
	}
	prepareTimeout := false
	for _, event := range d.Events {
		if event.Kind == "TERMINAL_TIMEOUT" {
			for _, operation := range d.Operations {
				if operation.OperationID == event.OperationID && operation.Method == "textDocument/prepareCallHierarchy" {
					prepareTimeout = true
				}
			}
		}
	}
	if !prepareTimeout || bytes.Contains(privateRaw, []byte(uri)) {
		t.Fatalf("ASSERT_FR23_PRIVATE_LIFECYCLE_TIMEOUT_AND_PRIVACY: timeout=%v", prepareTimeout)
	}

	absent := filepath.Join(t.TempDir(), "absent.json")
	without := exec.Command(cli, args[:len(args)-4]...)
	without.Env = append(os.Environ(), "LSP_TRACE_FAKE_LSP_HANG_PREPARE=1")
	var withoutOutput bytes.Buffer
	without.Stdout = &withoutOutput
	without.Stderr = &withoutOutput
	if err := runFR23WithScheduledFake(t, without); err != nil {
		t.Fatalf("ASSERT_FR23_PRIVATE_NO_OPT_IN_RUN: %v %s", err, withoutOutput.Bytes())
	}
	if _, err := os.Stat(absent); !os.IsNotExist(err) {
		t.Fatalf("ASSERT_FR23_PRIVATE_NO_OPT_IN_NO_ARTIFACT: %v", err)
	}
}
