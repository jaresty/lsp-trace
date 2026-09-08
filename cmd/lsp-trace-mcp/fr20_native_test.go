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

// Disposable native fixture only; not qualification of any real product.
func TestFR20InstalledGopls(t *testing.T) {
	server := os.Getenv("LSP_TRACE_FR20_GOPLS")
	if server == "" {
		t.Skip("set LSP_TRACE_FR20_GOPLS to an installed gopls; never installed by this test")
	}
	if runtime.GOOS != "darwin" {
		t.Skip("managed native CLI uses Darwin supervisor")
	}
	if !filepath.IsAbs(server) {
		t.Fatal("absolute installed server path required")
	}
	version, err := exec.Command(server, "version").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("installed fixture server: %s", version)
	for k, v := range map[string]string{"GOPROXY": "off", "GOSUMDB": "off", "GOTOOLCHAIN": "local", "GOTELEMETRY": "off"} {
		t.Setenv(k, v)
	}
	cli := buildBinary(t, "lsp-trace", "./cmd/lsp-trace")
	mcp := buildMCPBinary(t)
	for _, mode := range []string{"slice", "incoming"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			for name, text := range map[string]string{"go.mod": "module fr20fixture\n\ngo 1.23\n", "a.go": "package fixture\nfunc A(){ B() }\n", "b.go": "package fixture\nfunc B(){ C() }\n", "c.go": "package fixture\nfunc C(){}\n", "d.go": "package fixture\nfunc D(){}\n"} {
				if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0600); err != nil {
					t.Fatal(err)
				}
			}
			target := func(id, file, symbol string) map[string]any {
				return map[string]any{"id": id, "locator": map[string]any{"uri": (&url.URL{Scheme: "file", Path: filepath.Join(root, file)}).String(), "symbol": symbol, "language_id": "go"}, "down_depth": 3, "up_depth": 3}
			}
			positional := func(id, file string) map[string]any {
				return map[string]any{"id": id, "locator": map[string]any{"uri": (&url.URL{Scheme: "file", Path: filepath.Join(root, file)}).String(), "line": 1, "character": 5, "language_id": "go"}, "down_depth": 3, "up_depth": 3}
			}
			a, c, aliasFile := target("root", "a.go", "A"), target("end", "c.go", "C"), "a.go"
			if mode == "incoming" {
				a, c, aliasFile = target("root", "c.go", "C"), target("end", "a.go", "A"), "c.go"
			}
			manifest := map[string]any{"schema_version": "lsp-trace.seed-manifest.v2", "coordinate_convention": "zero-based-session", "root": a, "required_targets": []any{c, target("disconnected", "d.go", "D"), positional("positional-alias", aliasFile)}, "limits": map[string]any{"timeout_ms": 60000, "request_timeout_ms": 10000}}
			raw, _ := json.Marshal(manifest)
			p := filepath.Join(t.TempDir(), "manifest.json")
			if err := os.WriteFile(p, raw, 0600); err != nil {
				t.Fatal(err)
			}
			args := []string{mode, "--acquisition-version", "v2", "--workspace", root, "--server", server, "--seed-manifest", p, "--language-id", "go"}
			cliRaw := runCLIProcess(t, cli, args...)
			config := map[string]any{"version": 1, "processes": []any{map[string]any{"alias": "fixture", "language_id": "go", "profile": map[string]any{"trust_domain": "public-acquisition", "workspace": root, "profile": "cli", "environment_reference": "cli"}, "execution": map[string]any{"path": server, "directory": root, "environment": os.Environ()}}}}
			responses := runMCPProcess(t, mcp, []string{"--bootstrap-config", writeBootstrapJSON(t, config)}, []map[string]any{callRequest(1, "lsp_trace_v2_"+mode, map[string]any{"session_id": "fixture", "generation": 1, "seed_manifest": manifest})})
			call := decodeProcessCall(t, responses[0])
			if call.env["operation_status"] != "SUCCEEDED" {
				t.Fatalf("ASSERT_NATIVE_MCP: %v", call.env)
			}
			mcpRaw := inlineArtifactBytes(t, call.env)
			if !bytes.Equal(cliRaw, mcpRaw) {
				t.Fatal("ASSERT_NATIVE_EXACT_PARITY")
			}
			var e graphprovenance.EvidenceV2
			if err := json.Unmarshal(mcpRaw, &e); err != nil {
				t.Fatal(err)
			}
			if len(e.Acquisition.Targets) != 4 || e.Acquisition.Targets[1].Connection.Status != "FOUND" || e.Acquisition.Targets[2].Connection.Status != "NOT_FOUND_IN_RETAINED_GRAPH" || e.Acquisition.Targets[3].Resolution.Status != "RESOLVED" || len(e.Acquisition.Targets[1].Connection.Path.GroupIDs) != 2 {
				t.Fatalf("ASSERT_NATIVE_CONNECTED_DISCONNECTED: %+v", e.Acquisition.Targets)
			}
			if len(e.Captures) != 4 {
				t.Fatalf("ASSERT_NATIVE_MULTIFILE_CAPTURE: %d", len(e.Captures))
			}
			selector := filepath.Join(t.TempDir(), "selected.json")
			runCLIProcess(t, cli, append(args, "--output", selector)...)
			if err := os.RemoveAll(root); err != nil {
				t.Fatal(err)
			}
			if err := graphprovenance.ValidateV2(e); err != nil {
				t.Fatal("ASSERT_NATIVE_SOURCE_GONE", err)
			}
			runCLIProcess(t, cli, "verify", "--family", "graph-provenance", "--version", "v2", selector)
			offline := runMCPProcess(t, mcp, nil, []map[string]any{callRequest(1, "lsp_trace_v1_validate", map[string]any{"input": string(mcpRaw), "schema": map[string]any{"family": "graph-provenance", "version": "v2"}}), callRequest(2, "lsp_trace_v2_verify", map[string]any{"input": selector, "schema": map[string]any{"family": "graph-provenance", "version": "v2"}})})
			for _, r := range offline {
				if call := decodeProcessCall(t, r); call.env["operation_status"] != "SUCCEEDED" {
					t.Fatalf("ASSERT_NATIVE_OFFLINE_PUBLIC: %v", call.env)
				}
			}
		})
	}
}
