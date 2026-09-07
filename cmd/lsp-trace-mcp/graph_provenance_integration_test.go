package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/verification"
)

// Explicit installed-server qualification, separate from hermetic fake-wire tests.
func TestGraphProvenanceRealGoplsCLIAndMCP(t *testing.T) {
	server := os.Getenv("LSP_TRACE_GRAPH_PROVENANCE_GOPLS")
	if server == "" {
		t.Skip("set LSP_TRACE_GRAPH_PROVENANCE_GOPLS to an installed absolute gopls path")
	}
	if runtime.GOOS != "darwin" {
		t.Skip("existing managed CLI supervisor is Darwin-only")
	}
	if !filepath.IsAbs(server) {
		t.Fatal("gopls path must be absolute")
	}
	version, err := exec.Command(server, "version").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("real server: %s", version)
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOSUMDB", "off")
	t.Setenv("GOTOOLCHAIN", "local")
	t.Setenv("GOTELEMETRY", "off")
	cli := buildBinary(t, "lsp-trace", "./cmd/lsp-trace")
	mcp := buildMCPBinary(t)
	root := t.TempDir()
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module provenancefixture\n\ngo 1.23\n")
	for i := 0; i < 6; i++ {
		body := ""
		if i < 5 {
			body = fmt.Sprintf("F%d(); F%d()", i+1, i+1)
		}
		write(fmt.Sprintf("f%d.go", i), fmt.Sprintf("package fixture\nfunc F%d() { %s }\n", i, body))
	}
	build := exec.Command("go", "test", "./...")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("fixture build: %v %s", err, output)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Join(root, "f0.go"))}).String()
	sliceArgs := []string{"slice", "--workspace", root, "--server", server, "--at", "f0.go:2:6", "--graph-provenance", "--down-depth", "5", "--up-depth", "1", "--max-nodes", "100", "--timeout", "60s", "--request-timeout", "10s", "--language-id", "go"}
	cliRaw := runCLIProcess(t, cli, sliceArgs...)
	selectedPath := filepath.Join(t.TempDir(), "selected.json")
	outputArgs := append(append([]string{}, sliceArgs...), "--output", selectedPath)
	if output := runCLIProcess(t, cli, outputArgs...); len(output) != 0 {
		t.Fatal("output mode wrote stdout artifact")
	}
	selectorRaw, err := os.ReadFile(selectedPath)
	if err != nil {
		t.Fatal(err)
	}
	selector, err := verification.DecodeSelector(selectorRaw)
	if err != nil {
		t.Fatal(err)
	}
	generationPath := filepath.Join(filepath.Dir(selectedPath), selector.Generation)
	selectedRaw, err := os.ReadFile(filepath.Join(generationPath, "artifact.json"))
	if err != nil {
		t.Fatal(err)
	}
	selectedReceipt, err := os.ReadFile(filepath.Join(generationPath, "receipt.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := verification.VerifyReceipt(selectedRaw, selectedReceipt); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(selectedRaw, cliRaw) {
		t.Fatal("ASSERT_OUTPUT_EXACT_BYTES")
	}
	if _, err := graphprovenance.ValidateFor(selectedRaw, graphprovenance.Family, "v1"); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(cli, outputArgs...).CombinedOutput(); err == nil {
		t.Fatalf("ASSERT_OUTPUT_NO_REPLACE: %s", output)
	}
	afterSelector, err := os.ReadFile(selectedPath)
	if err != nil || !bytes.Equal(afterSelector, selectorRaw) {
		t.Fatal("ASSERT_OUTPUT_SELECTOR_UNCHANGED")
	}
	entries, err := os.ReadDir(filepath.Dir(selectedPath))
	if err != nil || len(entries) != 2 {
		t.Fatal("ASSERT_FAILED_OUTPUT_CLEANUP")
	}
	var cliEvidence graphprovenance.Evidence
	if err := json.Unmarshal(cliRaw, &cliEvidence); err != nil {
		t.Fatal(err)
	}
	args := map[string]any{"session_id": "fixture", "generation": 1, "start_mode": "at", "uri": uri, "line": 1, "character": 5, "language_id": "go", "down_depth": 5, "up_depth": 1, "max_nodes": 100, "timeout_ms": 60000, "request_timeout_ms": 10000, "graph_provenance": true}
	config := map[string]any{"version": 1, "processes": []any{map[string]any{"alias": "fixture", "language_id": "go", "profile": map[string]any{"trust_domain": "graph-provenance", "workspace": root, "profile": "cli", "environment_reference": "cli"}, "execution": map[string]any{"path": server, "directory": root, "environment": os.Environ()}}}}
	plain := cloneMap(args)
	delete(plain, "graph_provenance")
	results := runMCPProcess(t, mcp, []string{"--bootstrap-config", writeBootstrapJSON(t, config)}, []map[string]any{callRequest(1, "lsp_trace_v1_slice", args), callRequest(2, "lsp_trace_v1_slice", args), callRequest(3, "lsp_trace_v1_slice", plain)})
	first := decodeProcessCall(t, results[0])
	if first.env["operation_status"] != "SUCCEEDED" {
		t.Fatalf("ASSERT_REAL_MCP_PROVENANCE: %v", first.env)
	}
	mcpRaw := inlineArtifactBytes(t, first.env)
	if !bytes.Equal(cliRaw, mcpRaw) {
		t.Fatalf("ASSERT_REAL_CLI_MCP_EXACT_PARITY: cli=%s\nmcp=%s", cliRaw, mcpRaw)
	}
	var e graphprovenance.Evidence
	_ = json.Unmarshal(mcpRaw, &e)
	var g graph.Result
	if err := json.Unmarshal(e.GraphBytes, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Nodes) != 6 || len(g.Edges) != 5 || len(e.Captures) != 6 || e.Supply == nil {
		t.Fatalf("ASSERT_REAL_FIVE_NONSEED: nodes=%d edges=%d captures=%d", len(g.Nodes), len(g.Edges), len(e.Captures))
	}
	distinct := map[string]bool{}
	for _, n := range g.Nodes {
		if n.URI != uri {
			distinct[n.URI] = true
		}
	}
	if len(distinct) != 5 {
		t.Fatal("ASSERT_FIVE_DISTINCT_NONSEED_FILES")
	}
	for _, r := range e.Captures {
		if r.Status != "READABLE" || r.AnalyzedVersion != graphprovenance.Unverified {
			t.Fatalf("ASSERT_REAL_UNVERIFIED_CAPTURE: %+v", r)
		}
	}
	var cached graphprovenance.Evidence
	_ = json.Unmarshal(inlineArtifactBytes(t, decodeProcessCall(t, results[1]).env), &cached)
	if cached.Supply != nil || cached.SupplyStatus != "NO_NOTIFICATION_OBSERVATION" {
		t.Fatal("ASSERT_REAL_CACHED_NOT_NEW_SUPPLY")
	}
	legacy := inlineArtifactBytes(t, decodeProcessCall(t, results[2]).env)
	if !bytes.Equal(e.GraphBytes, legacy) || !bytes.Equal(cached.GraphBytes, legacy) {
		t.Fatal("ASSERT_MANAGED_OMITTED_EXACT_BYTES")
	}
	// Independent fixed six-layer oracle: not derived from the census predicate.
	bindings := map[string]bool{}
	for _, b := range e.Bindings {
		bindings[b.Pointer] = true
	}
	for i := 0; i < 6; i++ {
		pointer := fmt.Sprintf("/slice/layers/%d/node_ids/0", i)
		if !bindings[pointer] {
			t.Fatalf("ASSERT_REAL_LAYER_POINTER: %s", pointer)
		}
	}
	// Remove the entire source fixture before independent CLI and MCP validation.
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	testRetainedCallsRealOffline(t, cli, mcp, mcpRaw)
	if output := runCLIProcess(t, cli, "verify", "--family", "graph-provenance", "--version", "v1", selectedPath); !strings.Contains(string(output), "verified integrity and custody") {
		t.Fatal("ASSERT_OFFLINE_SELECTED_VERIFY")
	}
	if _, err := exec.Command(cli, "verify", selectedPath).CombinedOutput(); err == nil {
		t.Fatal("ASSERT_HISTORICAL_VERIFY_REMAINS_GRAPH_ONLY")
	}
	retained := filepath.Join(t.TempDir(), "evidence.json")
	if err := os.WriteFile(retained, mcpRaw, 0600); err != nil {
		t.Fatal(err)
	}
	if output := runCLIProcess(t, cli, "validate", "--family", "graph-provenance", "--version", "v1", retained); !strings.Contains(string(output), "valid lsp-trace.graph-provenance.v1") {
		t.Fatal("ASSERT_OFFLINE_CLI")
	}
	publication := t.TempDir()
	offline := runMCPProcess(t, mcp, []string{"--publication-root", publication}, []map[string]any{
		callRequest(1, "lsp_trace_v1_validate", map[string]any{"input": string(mcpRaw), "schema": map[string]any{"family": "graph-provenance", "version": "v1"}}),
		callRequest(2, "lsp_trace_v1_schema_get", map[string]any{"schema": map[string]any{"family": "graph-provenance", "version": "v1"}}),
		callRequest(3, "lsp_trace_v1_validate", map[string]any{"input": string(mcpRaw), "schema": map[string]any{"family": "graph-provenance", "version": "v1"}, "output_selector": "provenance.json"}),
	})
	for i, r := range offline {
		call := decodeProcessCall(t, r)
		if call.env["operation_status"] != "SUCCEEDED" {
			t.Fatalf("ASSERT_OFFLINE_MCP_%d: %v", i, call.env)
		}
	}
	if !bytes.Equal(inlineArtifactBytes(t, decodeProcessCall(t, offline[0]).env), mcpRaw) {
		t.Fatal("ASSERT_OFFLINE_EXACT_BYTES")
	}
	schemaRaw := runCLIProcess(t, cli, "schema", "get", "--family", "graph-provenance", "--version", "v1")
	if !bytes.Equal(schemaRaw, inlineArtifactBytes(t, decodeProcessCall(t, offline[1]).env)) {
		t.Fatal("ASSERT_SCHEMA_GET_PARITY")
	}
	published, err := os.ReadFile(filepath.Join(publication, "provenance.json"))
	if err != nil || !bytes.Equal(published, mcpRaw) {
		t.Fatalf("ASSERT_PUBLICATION_EXACT_BYTES: %v", err)
	}
	if retain := os.Getenv("LSP_TRACE_GRAPH_PROVENANCE_RETAIN"); retain != "" {
		if !filepath.IsAbs(retain) {
			t.Fatal("retention path must be absolute")
		}
		if err := os.MkdirAll(retain, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(retain, selector.Generation), 0700); err != nil {
			t.Fatal(err)
		}
		for name, data := range map[string][]byte{"selected.json": selectorRaw, filepath.Join(selector.Generation, "artifact.json"): selectedRaw, filepath.Join(selector.Generation, "receipt.json"): selectedReceipt, "cli.json": cliRaw, "mcp.json": mcpRaw, "legacy-graph.json": legacy, "schema.json": schemaRaw, "gopls-version.txt": version} {
			if err := os.WriteFile(filepath.Join(retain, name), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Log("PASS real gopls: six files, five non-seed files, exact CLI/MCP wrapper and omitted graph parity, cached supply, offline CLI/MCP validation after fixture deletion, schema_get and immutable publication")
}
