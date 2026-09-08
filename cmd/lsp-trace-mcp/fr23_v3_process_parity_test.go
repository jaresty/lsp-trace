package main

import (
	"bytes"
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/mcpcontract"
)

func TestFR23V3RealProcessCLIAndMCPByteParityGenerationOne(t *testing.T) {
	const assertion = "ASSERT_FR23_V3_REAL_PROCESS_CLI_MCP_BYTE_PARITY_GENERATION_ONE"
	if runtime.GOOS != "darwin" {
		t.Skip("managed process CLI uses Darwin supervisor")
	}
	cli := buildBinary(t, "lsp-trace", "./cmd/lsp-trace")
	mcp := buildMCPBinary(t)
	fake := buildBinary(t, "fake-lsp", "./cmd/fake-lsp")
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("leaf\n\ncaller\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.Join(workspace, "main.go")}).String()
	target := map[string]any{"id": "root", "locator": map[string]any{"uri": uri, "line": 0, "character": 0}, "down_depth": 0, "up_depth": 0}
	manifest := map[string]any{
		"schema_version": "lsp-trace.seed-manifest.v2", "coordinate_convention": "zero-based-session",
		"root": target, "required_targets": []any{}, "limits": map[string]any{"max_nodes": 1, "max_requests": 4},
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(t.TempDir(), "seeds.json")
	if err := os.WriteFile(manifestPath, manifestBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	config := map[string]any{"version": 1, "processes": []any{map[string]any{
		"alias": "fixture", "language_id": "go",
		"profile":   map[string]any{"trust_domain": "public-acquisition", "workspace": workspace, "profile": "cli", "environment_reference": "cli"},
		"execution": map[string]any{"path": fake, "directory": workspace, "environment": os.Environ()},
	}}}

	cliCommand := exec.Command(cli, "slice", "--acquisition-version", "v3", "--workspace", workspace, "--server", fake, "--seed-manifest", manifestPath, "--language-id", "go")
	var cliStdout, cliStderr bytes.Buffer
	cliCommand.Stdout, cliCommand.Stderr = &cliStdout, &cliStderr
	if err := cliCommand.Run(); err != nil {
		t.Fatalf("%s: CLI: %v stderr=%s", assertion, err, cliStderr.String())
	}
	responses := runMCPProcess(t, mcp, []string{"--bootstrap-config", writeBootstrapJSON(t, config)}, []map[string]any{
		callRequest(1, "lsp_trace_v3_slice", map[string]any{"session_id": "fixture", "generation": 1, "seed_manifest": manifest}),
	})
	call := decodeProcessCall(t, responses[0])
	if call.env["operation_status"] != "SUCCEEDED" || call.env["artifact_schema_id"] != mcpcontract.GraphProvenanceV3ArtifactID {
		t.Fatalf("%s: MCP envelope=%v", assertion, call.env)
	}
	mcpArtifact := inlineArtifactBytes(t, call.env)
	if !bytes.Equal(cliStdout.Bytes(), mcpArtifact) {
		at := 0
		for ; at < cliStdout.Len() && at < len(mcpArtifact) && cliStdout.Bytes()[at] == mcpArtifact[at]; at++ {
		}
		start, end := at-80, at+160
		if start < 0 {
			start = 0
		}
		if end > cliStdout.Len() {
			end = cliStdout.Len()
		}
		if end > len(mcpArtifact) {
			end = len(mcpArtifact)
		}
		t.Fatalf("%s: CLI bytes=%d MCP bytes=%d first_diff=%d CLI=%q MCP=%q", assertion, cliStdout.Len(), len(mcpArtifact), at, cliStdout.Bytes()[start:end], mcpArtifact[start:end])
	}
	if _, err := graphprovenance.ValidateFor(mcpArtifact, graphprovenance.Family, "v3"); err != nil {
		t.Fatalf("%s: admission: %v", assertion, err)
	}
	if bytes.Contains(bytes.ToLower(mcpArtifact), []byte("secret")) {
		t.Fatalf("%s: secret marker present", assertion)
	}
	t.Logf("PASS %s: cli_bytes=%d mcp_bytes=%d generation=1", assertion, cliStdout.Len(), len(mcpArtifact))
}
