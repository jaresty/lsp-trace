package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

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
	cmd.Env = append(cmd.Env, "LSP_TRACE_FAKE_LSP_SCHEDULE_BARRIER="+socket)
	if err := cmd.Start(); err != nil {
		return err
	}
	conn, err := listener.Accept()
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return err
	}
	var scheduled [10]byte
	if _, err := io.ReadFull(conn, scheduled[:]); err != nil || string(scheduled[:]) != "scheduled\n" {
		_ = conn.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("ASSERT_FR23_PRIVATE_BUILT_CHILD_SCHEDULED: marker=%q err=%v", scheduled, err)
	}
	if os.Getenv("LSP_TRACE_FR23_WITHHOLD_SCHEDULE_RELEASE") != "1" {
		if _, err := conn.Write([]byte{1}); err != nil {
			_ = conn.Close()
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			t.Fatalf("ASSERT_FR23_PRIVATE_BUILT_CHILD_RELEASED: %v", err)
		}
	}
	_ = conn.Close()
	return cmd.Wait()
}

func TestManagedV5ZeroExactRelationsFinalizesPrivateRequestDiagnostic(t *testing.T) {
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
	privateRoot := t.TempDir()
	privatePath := filepath.Join(privateRoot, "private.json")
	cmd := exec.Command(cli, "slice", "--acquisition-version", "v3", "--output-version", graphprovenance.VersionV5, "--workspace", workspace, "--server", fake, "--server-env", "LSP_TRACE_FAKE_LSP_DOCUMENT_SYMBOL=mismatch", "--seed-manifest", manifestPath, "--language-id", "go", "--private-request-diagnostic-root", privateRoot, "--private-request-diagnostic-selector", "private.json")
	cmd.Env = append(os.Environ(), "LSP_TRACE_FAKE_LSP_DOCUMENT_SYMBOL=mismatch")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err == nil {
		t.Fatal("ASSERT_MANAGED_V5_ZERO_EXACT_RELATIONS_DOMAIN_ERROR: expected failure")
	}
	if !strings.Contains(stderr.String(), "topmost sibling expansion produced no exact relations") {
		t.Fatalf("ASSERT_MANAGED_V5_ZERO_EXACT_RELATIONS_DOMAIN_ERROR: %s", stderr.String())
	}
	if strings.Contains(stderr.String(), privatePath) || strings.Contains(stderr.String(), uri) {
		t.Fatalf("ASSERT_MANAGED_V5_ZERO_EXACT_RELATIONS_PUBLIC_REDACTION: %s", stderr.String())
	}
	privateRaw, err := os.ReadFile(privatePath)
	if err != nil {
		const reason = "private request diagnostics unavailable: PUBLIC_ARTIFACT_UNAVAILABLE; lifecycle diagnostics require successful public artifact bytes for integrity binding"
		if !strings.Contains(stderr.String(), reason) {
			t.Fatalf("ASSERT_MANAGED_V5_ZERO_EXACT_RELATIONS_PRIVATE_ARTIFACT_DEPENDENCY_GUIDANCE: %v stderr=%s", err, stderr.String())
		}
		return
	}
	var diagnostic requestlifecycle.DocumentModel
	if err := json.Unmarshal(privateRaw, &diagnostic); err != nil {
		t.Fatalf("ASSERT_MANAGED_V5_ZERO_EXACT_RELATIONS_PRIVATE_VALID: %v", err)
	}
	methods := map[string]bool{}
	for _, operation := range diagnostic.Operations {
		methods[operation.Method] = true
	}
	if !methods["textDocument/documentSymbol"] || !methods["textDocument/prepareCallHierarchy"] {
		t.Fatalf("ASSERT_MANAGED_V5_ZERO_EXACT_RELATIONS_PRIVATE_RECORDS: methods=%v", methods)
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
