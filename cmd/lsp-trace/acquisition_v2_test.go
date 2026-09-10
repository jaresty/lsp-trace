package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/seedbinding"
)

func TestDeprecatedAcquisitionVersionsWarnOnlyOnStderr(t *testing.T) {
	for _, version := range []string{"v2", "v3"} {
		var stdout, stderr strings.Builder
		code := runAcquisitionVersion("slice", version, nil, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("ASSERT_DEPRECATED_ACQUISITION_REJECTS_INVALID_INPUT[%s]: code=%d", version, code)
		}
		if stdout.Len() != 0 {
			t.Fatalf("ASSERT_DEPRECATION_NOTICE_NOT_JSON_STDOUT[%s]: %q", version, stdout.String())
		}
		want := "DEPRECATED: Graph Provenance " + strings.ToUpper(version) + " production is deprecated; migrate new production to source-qualified Graph Provenance V5."
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("ASSERT_DEPRECATED_ACQUISITION_V5_MIGRATION[%s]: stderr=%q", version, stderr.String())
		}
	}
}

func TestSourceQualifiedV5SelectionIsNotDeprecated(t *testing.T) {
	var stdout, stderr strings.Builder
	code := runAcquisitionVersion("slice", "v3", []string{"--output-version", "lsp-trace.graph-provenance.v5"}, &stdout, &stderr)
	if code != 1 || stdout.Len() != 0 {
		t.Fatalf("ASSERT_V5_INVALID_INPUT_REMAINS_PATH_FREE: code=%d stdout=%q", code, stdout.String())
	}
	if strings.Contains(stderr.String(), "DEPRECATED") {
		t.Fatalf("ASSERT_SOURCE_QUALIFIED_V5_NOT_DEPRECATED: stderr=%q", stderr.String())
	}
}

func TestAcquisitionVersionPreservesArgumentValues(t *testing.T) {
	for _, args := range [][]string{{"--server-arg", "--acquisition-version=v2"}, {"--output", "--acquisition-version"}, {"--server-arg", "--acquisition-version", "--at", "x:1:1"}} {
		version, rest, err := acquisitionVersion(args)
		if err != nil || version != "" || !reflect.DeepEqual(args, rest) {
			t.Fatalf("ASSERT_VERSION_FLAG_NOT_ARGUMENT_VALUE: %v -> %q %v %v", args, version, rest, err)
		}
	}
}

func buildStartupSinkCLI(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "lsp-trace")
	cmd := exec.Command("go", "build", "-o", binary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v: %s", err, out)
	}
	return binary
}

func startupSinkCLIArgs(workspace, manifestPath string) []string {
	return []string{"slice", "--acquisition-version=v2", "--workspace", workspace, "--server", os.Args[0], "--server-arg", "-test.run=^TestRuntimeHelperServer$", "--server-env", "LSP_TRACE_RUNTIME_HELPER=1", "--server-env", "LSP_TRACE_RUNTIME_HELPER_EXIT_INITIALIZE=1", "--seed-manifest", manifestPath}
}

func TestPrivateStartupSinkBuiltCLIInitializationFailureAndOptIn(t *testing.T) {
	const assertion = "ASSERT_FR23_PRIVATE_STARTUP_BUILT_CLI_INITIALIZATION_FAILED_PATH_FREE"
	if runtime.GOOS != "darwin" {
		t.Skip("managed process CLI uses Darwin supervisor")
	}
	binary := buildStartupSinkCLI(t)
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
	args := append(startupSinkCLIArgs(workspace, manifestPath), "--private-startup-diagnostic-root", root, "--private-startup-diagnostic-selector", "attempt.json")
	cmd := exec.Command(binary, args...)
	stdout, stderr := strings.Builder{}, strings.Builder{}
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	artifact, readErr := os.ReadFile(filepath.Join(root, "attempt.json"))
	if readErr != nil {
		t.Fatalf("%s: artifact: %v stderr=%q", assertion, readErr, stderr.String())
	}
	if err == nil || stdout.Len() != 0 || !strings.Contains(stderr.String(), "DEPRECATED: Graph Provenance V2 production is deprecated") || !strings.Contains(stderr.String(), "managed readiness: INITIALIZATION_FAILED") || strings.Contains(stderr.String(), root) || strings.Contains(stderr.String(), workspace) || strings.Contains(stderr.String(), manifestPath) {
		t.Fatalf("%s: err=%v stdout=%q stderr=%q", assertion, err, stdout.String(), stderr.String())
	}
	sum := sha256.Sum256(artifact)
	if err := manageddiagnostic.VerifyStartupDiagnostics(artifact, "sha256:"+hex.EncodeToString(sum[:]), len(artifact)); err != nil {
		t.Fatalf("%s: verify: %v", assertion, err)
	}
}

func TestCLIExplicitLocalV3BindingReachesMechanicalValidationBeforeProvider(t *testing.T) {
	const assertion = "ASSERT_CLI_LOCAL_V3_BINDING_REACHES_MECHANICAL_VALIDATOR_PREPROVIDER"
	if runtime.GOOS != "darwin" {
		t.Skip("managed process CLI uses Darwin supervisor")
	}
	workspace := t.TempDir()
	manifest := map[string]any{"schema_version": "lsp-trace.seed-manifest.v2", "coordinate_convention": "zero-based-session", "root": map[string]any{"id": "root", "locator": map[string]any{"uri": "file:///missing.go", "line": 0, "character": 0}, "down_depth": 0, "up_depth": 0}, "required_targets": []any{}, "limits": map[string]any{"max_nodes": 1, "max_requests": 1}}
	manifestRaw, _ := json.Marshal(manifest)
	manifestPath := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(manifestPath, manifestRaw, 0600); err != nil {
		t.Fatal(err)
	}
	bindingRoot := t.TempDir()
	bindingRaw, _ := json.Marshal(seedbinding.Manifest{SchemaVersion: seedbinding.VersionV3, CustodyMode: seedbinding.CallerAssertedLocal})
	if err := os.WriteFile(filepath.Join(bindingRoot, "binding.json"), bindingRaw, 0600); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(workspace, "provider-started")
	var stdout, stderr strings.Builder
	code := runAcquisitionVersion("slice", "v3", []string{"--workspace", workspace, "--server", "/bin/sh", "--server-arg", "-c", "--server-arg", "touch " + marker, "--seed-manifest", manifestPath, "--private-seed-binding-root", bindingRoot, "--private-seed-binding-selector", "binding.json"}, &stdout, &stderr)
	t.Logf("ASSERTION: %s; code=%d stderr=%q", assertion, code, stderr.String())
	if code != 1 || !strings.Contains(stderr.String(), seedbinding.LocatorInvalid) {
		t.Fatalf("%s: expected mechanical taxonomy; code=%d stdout=%q stderr=%q", assertion, code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("%s: provider started before mechanical MATCH: %v", assertion, err)
	}
}

func TestPrivateStartupSinkBuiltCLIAbsentOptInWritesNothingAndBothFlagsRequired(t *testing.T) {
	const assertion = "ASSERT_FR23_PRIVATE_STARTUP_BUILT_CLI_NO_DEFAULT_OUTPUT"
	if runtime.GOOS != "darwin" {
		t.Skip("managed process CLI uses Darwin supervisor")
	}
	binary := buildStartupSinkCLI(t)
	workspace, root := t.TempDir(), t.TempDir()
	manifestPath := filepath.Join(t.TempDir(), "manifest.json")
	manifest := map[string]any{"schema_version": "lsp-trace.seed-manifest.v2", "coordinate_convention": "zero-based-session", "root": map[string]any{"id": "root", "locator": map[string]any{"uri": "file:///private/secret.go", "line": 0, "character": 0}, "down_depth": 0, "up_depth": 0}, "required_targets": []any{}, "limits": map[string]any{"max_nodes": 1, "max_requests": 1}}
	raw, _ := json.Marshal(manifest)
	if err := os.WriteFile(manifestPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, startupSinkCLIArgs(workspace, manifestPath)...)
	_, _ = cmd.CombinedOutput()
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("%s: entries=%v err=%v", assertion, entries, err)
	}
	for _, partial := range [][]string{{"--private-startup-diagnostic-root", root}, {"--private-startup-diagnostic-selector", "attempt.json"}} {
		args := append(startupSinkCLIArgs(workspace, manifestPath), partial...)
		if err := exec.Command(binary, args...).Run(); err == nil {
			t.Fatalf("%s: partial opt-in accepted: %v", assertion, partial)
		}
	}
	entries, err = os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("%s: partial opt-in wrote entries=%v err=%v", assertion, entries, err)
	}
	version, rest, err := acquisitionVersion([]string{"--server", "x"})
	if err != nil || version != "" || !reflect.DeepEqual(rest, []string{"--server", "x"}) {
		t.Fatalf("%s: parser changed", assertion)
	}
}
