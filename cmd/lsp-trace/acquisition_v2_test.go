package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"lsp-trace/internal/manageddiagnostic"
)

func TestAcquisitionVersionPreservesArgumentValues(t *testing.T) {
	for _, args := range [][]string{{"--server-arg", "--acquisition-version=v2"}, {"--output", "--acquisition-version"}, {"--server-arg", "--acquisition-version", "--at", "x:1:1"}} {
		version, rest, err := acquisitionVersion(args)
		if err != nil || version != "" || !reflect.DeepEqual(args, rest) {
			t.Fatalf("ASSERT_VERSION_FLAG_NOT_ARGUMENT_VALUE: %v -> %q %v %v", args, version, rest, err)
		}
	}
}

func TestPrivateStartupSinkCLIExistsOnInitializationFailureAndStderrIsGeneric(t *testing.T) {
	const assertion = "ASSERT_FR23_PRIVATE_STARTUP_CLI_INITIALIZATION_FAILED_PATH_FREE"
	if runtime.GOOS != "darwin" {
		t.Skip("managed process CLI uses Darwin supervisor")
	}
	workspace := t.TempDir()
	manifest := map[string]any{"schema_version": "lsp-trace.seed-manifest.v2", "coordinate_convention": "zero-based-session", "root": map[string]any{"id": "root", "locator": map[string]any{"uri": "file:///private/secret.go", "line": 0, "character": 0}, "down_depth": 0, "up_depth": 0}, "required_targets": []any{}, "limits": map[string]any{"max_nodes": 1, "max_requests": 1}}
	raw, _ := json.Marshal(manifest)
	manifestPath := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(manifestPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runAcquisitionVersion("slice", "v2", []string{"--workspace", workspace, "--server", os.Args[0], "--server-arg", "-test.run=^TestRuntimeHelperServer$", "--server-env", "LSP_TRACE_RUNTIME_HELPER=1", "--server-env", "LSP_TRACE_RUNTIME_HELPER_EXIT_INITIALIZE=1", "--seed-manifest", manifestPath, "--private-startup-diagnostic-root", root, "--private-startup-diagnostic-selector", "attempt.json"}, &stdout, &stderr)
	artifact, err := os.ReadFile(filepath.Join(root, "attempt.json"))
	if err != nil {
		t.Fatalf("%s: artifact: %v stderr=%q", assertion, err, stderr.String())
	}
	if code == 0 || stdout.Len() != 0 || strings.TrimSpace(stderr.String()) != "managed readiness: INITIALIZATION_FAILED" || strings.Contains(stderr.String(), root) || strings.Contains(stderr.String(), workspace) || strings.Contains(stderr.String(), "runtime helper diagnostic") {
		t.Fatalf("%s: code=%d stdout=%q stderr=%q", assertion, code, stdout.String(), stderr.String())
	}
	if err := manageddiagnostic.ValidateStartupDiagnostics(artifact); err != nil {
		t.Fatalf("%s: %v", assertion, err)
	}

	badRoot := t.TempDir()
	if err := os.Chmod(badRoot, 0755); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	code = runAcquisitionVersion("slice", "v2", []string{"--workspace", workspace, "--server", os.Args[0], "--server-arg", "-test.run=^TestRuntimeHelperServer$", "--server-env", "LSP_TRACE_RUNTIME_HELPER=1", "--server-env", "LSP_TRACE_RUNTIME_HELPER_EXIT_INITIALIZE=1", "--seed-manifest", manifestPath, "--private-startup-diagnostic-root", badRoot, "--private-startup-diagnostic-selector", "attempt.json"}, &stdout, &stderr)
	if code == 0 || stdout.Len() != 0 || strings.TrimSpace(stderr.String()) != "managed readiness: INITIALIZATION_FAILED" {
		t.Fatalf("ASSERT_FR23_STARTUP_SINK_FAILURE_PRESERVES_PRIMARY: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(badRoot, "attempt.json")); !os.IsNotExist(err) {
		t.Fatalf("ASSERT_FR23_STARTUP_SINK_FAILURE_PRESERVES_PRIMARY: artifact err=%v", err)
	}
}

func TestPrivateStartupSinkAbsentOptInWritesNothing(t *testing.T) {
	const assertion = "ASSERT_FR23_PRIVATE_STARTUP_CLI_NO_DEFAULT_OUTPUT"
	version, rest, err := acquisitionVersion([]string{"--server", "x"})
	if err != nil || version != "" || !reflect.DeepEqual(rest, []string{"--server", "x"}) {
		t.Fatalf("%s: parser changed", assertion)
	}
}
