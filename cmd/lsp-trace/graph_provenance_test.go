package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGraphProvenanceCLIOptIn(t *testing.T) {
	_, err := parseSlice([]string{"--workspace", t.TempDir(), "--server", "gopls", "--at", "seed.go:2:6", "--graph-provenance"})
	if err != nil {
		t.Fatalf("ASSERT_GRAPH_PROVENANCE_CLI_ROUTE: %v", err)
	}
}

func TestGraphProvenanceCLIFileSymbolUsesManagedMCPResolver(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"slice", "--workspace", workspace, "--server", os.Args[0], "--server-arg", "-test.run=^TestFakeLanguageServerProcess$", "--server-env", "LSP_TRACE_FAKE_SERVER=1", "--server-env", "LSP_TRACE_FAKE_SCENARIO=slice-symbol", "--graph-provenance", "--from-file", "main.go", "--symbol", "start", "--down-depth", "1", "--up-depth", "0", "--request-timeout", "1s", "--timeout", "5s"}
	stdout, stderr, code := captureRun(t, args)
	if code != 0 || !strings.Contains(stdout, `"schema_version":"lsp-trace.graph-provenance.v1"`) {
		t.Fatalf("ASSERT_MANAGED_CLI_MCP_FILE_SYMBOL_PARITY: code=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
}
