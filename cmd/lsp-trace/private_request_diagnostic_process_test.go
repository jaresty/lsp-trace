package main

import (
	"bytes"
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"lsp-trace/internal/graphprovenance"
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
	privatePath := filepath.Join(privateRoot, "private.json")
	args := []string{"slice", "--acquisition-version", "v3", "--workspace", workspace, "--server", fake, "--seed-manifest", manifestPath, "--language-id", "go", "--private-request-diagnostic-root", privateRoot, "--private-request-diagnostic-selector", "private.json"}
	cmd := exec.Command(cli, args...)
	cmd.Env = append(os.Environ(), "LSP_TRACE_FAKE_LSP_HANG_PREPARE=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
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
	d, err := decodePrivateRequestDiagnostic(bytes.TrimSpace(privateRaw))
	if err != nil {
		t.Fatalf("ASSERT_FR23_PRIVATE_BUILT_ARTIFACT_VALID: %v", err)
	}
	if d.SelectedOperation.Method != "textDocument/prepareCallHierarchy" || d.SelectedOperation.Classification != "TIMEOUT_OBSERVED" || d.SelectedOperation.Write.Count != 1 {
		t.Fatalf("ASSERT_FR23_PRIVATE_EXACT_ONE_ATTEMPT: %+v", d.SelectedOperation)
	}

	absent := filepath.Join(t.TempDir(), "absent.json")
	without := exec.Command(cli, args[:len(args)-2]...)
	without.Env = append(os.Environ(), "LSP_TRACE_FAKE_LSP_HANG_PREPARE=1")
	if out, err := without.CombinedOutput(); err != nil {
		t.Fatalf("ASSERT_FR23_PRIVATE_NO_OPT_IN_RUN: %v %s", err, out)
	}
	if _, err := os.Stat(absent); !os.IsNotExist(err) {
		t.Fatalf("ASSERT_FR23_PRIVATE_NO_OPT_IN_NO_ARTIFACT: %v", err)
	}
}
