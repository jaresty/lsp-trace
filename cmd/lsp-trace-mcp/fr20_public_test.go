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
)

func TestFR20PublicProcess(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("managed process CLI uses Darwin supervisor")
	}
	cli := buildBinary(t, "lsp-trace", "./cmd/lsp-trace")
	mcp := buildMCPBinary(t)
	fake := buildBinary(t, "fake-lsp", "./cmd/fake-lsp")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("leaf\n\ncaller\n"), 0600); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.Join(root, "main.go")}).String()
	target := func(id string) map[string]any {
		return map[string]any{"id": id, "locator": map[string]any{"uri": uri, "line": 0, "character": 0}, "down_depth": 0, "up_depth": 0}
	}
	manifest := map[string]any{"schema_version": "lsp-trace.seed-manifest.v2", "coordinate_convention": "zero-based-session", "root": target("root"), "required_targets": []any{target("alias"), target("second")}, "limits": map[string]any{"max_nodes": 3, "max_requests": 8}}
	raw, _ := json.Marshal(manifest)
	path := filepath.Join(t.TempDir(), "seeds.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	config := map[string]any{"version": 1, "processes": []any{map[string]any{"alias": "fixture", "language_id": "go", "profile": map[string]any{"trust_domain": "public-acquisition", "workspace": root, "profile": "cli", "environment_reference": "cli"}, "execution": map[string]any{"path": fake, "directory": root, "environment": os.Environ()}}}}
	for _, mode := range []string{"slice", "incoming"} {
		t.Run(mode, func(t *testing.T) {
			cmd := exec.Command(cli, mode, "--acquisition-version", "v2", "--workspace", root, "--server", fake, "--seed-manifest", path, "--language-id", "go")
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			if err != nil {
				t.Errorf("ASSERT_FR20_CLI_PRODUCES_V2: %v %s", err, stderr.String())
			}
			results := runMCPProcess(t, mcp, []string{"--bootstrap-config", writeBootstrapJSON(t, config)}, []map[string]any{callRequest(1, "lsp_trace_v2_"+mode, map[string]any{"session_id": "fixture", "generation": 1, "seed_manifest": manifest})})
			if results[0]["error"] != nil {
				t.Fatalf("ASSERT_FR20_MCP_PRODUCES_V2: %v", results[0])
			}
			call := decodeProcessCall(t, results[0])
			if call.env["operation_status"] != "SUCCEEDED" {
				t.Fatalf("ASSERT_FR20_MCP_PRODUCES_V2: %v", call.env)
			}
			artifact := inlineArtifactBytes(t, call.env)
			if _, err := graphprovenance.ValidateFor(artifact, "graph-provenance", "v2"); err != nil {
				t.Fatalf("ASSERT_FR20_ADMISSION: %v", err)
			}
			var e graphprovenance.EvidenceV2
			if err := json.Unmarshal(artifact, &e); err != nil {
				t.Fatal(err)
			}
			if e.WorkspaceURI != (&url.URL{Scheme: "file", Path: root}).String() || e.Acquisition.Request.Context.Generation != 1 || e.Acquisition.Request.Context.SessionID == "" {
				t.Fatal("ASSERT_FR20_HOST_WORKSPACE_GENERATION")
			}
			if len(e.Acquisition.Targets) != 3 || e.Acquisition.Targets[1].Requested.ID != "alias" || e.Acquisition.Targets[2].Requested.ID != "second" {
				t.Fatal("ASSERT_FR20_ORDERED_ACCOUNTING")
			}
			if e.Acquisition.Targets[1].Connection.Status != "FOUND" || e.Acquisition.Targets[1].Connection.Work != 1 {
				t.Fatal("ASSERT_FR20_ZERO_HOP")
			}
			if e.Acquisition.Usage.Requests > 8 || e.Acquisition.Usage.Nodes > 3 {
				t.Fatal("ASSERT_FR20_SHARED_BUDGET")
			}
			if err == nil && !bytes.Equal(stdout.Bytes(), artifact) {
				t.Fatal("ASSERT_FR20_EXACT_PROCESS_PARITY")
			}
			selected := filepath.Join(t.TempDir(), "selected.json")
			publishArgs := append(append([]string{}, cmd.Args[1:]...), "--output", selected)
			runCLIProcess(t, cli, publishArgs...)
			if output, err := exec.Command(cli, "verify", "--family", "graph-provenance", "--version", "v2", selected).CombinedOutput(); err != nil {
				t.Errorf("ASSERT_FR20_EXPLICIT_VERIFY: %v %s", err, output)
			}
			offline := runMCPProcess(t, mcp, nil, []map[string]any{callRequest(1, "lsp_trace_v2_verify", map[string]any{"input": selected, "schema": map[string]any{"family": "graph-provenance", "version": "v2"}})})
			if offline[0]["error"] != nil {
				t.Fatalf("ASSERT_FR20_MCP_VERIFY: %v", offline[0])
			}
			verified := decodeProcessCall(t, offline[0])
			if verified.env["operation_status"] != "SUCCEEDED" {
				t.Fatalf("ASSERT_FR20_MCP_VERIFY: %v", verified.env)
			}
		})
	}
}
