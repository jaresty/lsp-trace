package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/retainedcalls"
	"lsp-trace/internal/verification"
)

func TestRetainedCallsV2OfflineCLIAndMCP(t *testing.T) {
	input, err := os.ReadFile(filepath.Join("..", "..", "internal", "hydratedevidence", "testdata", "focused-fr20.v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	cli := buildBinary(t, "lsp-trace", "./cmd/lsp-trace")
	mcp := buildMCPBinary(t)
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "fr20.json")
	if err = os.WriteFile(inputPath, input, 0600); err != nil {
		t.Fatal(err)
	}
	want := runCLIProcess(t, cli, "export-retained-calls", "--version", "v2", inputPath)
	var exported retainedcalls.EvidenceV2
	if err = json.Unmarshal(want, &exported); err != nil {
		t.Fatal(err)
	}
	if exported.Tables.Acquisition.Targets == nil || exported.Tables.Connections == nil || exported.Tables.Groups == nil || exported.Tables.Occurrences == nil {
		t.Fatal("ASSERT_MCP_RETAINED_V2_EXPLICIT_ARRAYS")
	}
	publication := t.TempDir()
	calls := runMCPProcess(t, mcp, []string{"--publication-root", publication}, []map[string]any{
		callRequest(1, "lsp_trace_v2_export_retained_calls", map[string]any{"input": string(input)}),
		callRequest(2, "lsp_trace_v2_export_retained_calls", map[string]any{"input": string(input), "output_selector": "retained-v2.json"}),
		callRequest(3, "lsp_trace_v2_export_retained_calls", map[string]any{"input": string(input), "output_selector": "compact-v2.json", "detail": "compact"}),
		callRequest(4, "lsp_trace_v1_validate", map[string]any{"input": string(want), "schema": map[string]any{"family": "retained-calls", "version": "v2"}}),
		callRequest(5, "lsp_trace_v2_export_retained_calls", map[string]any{"input": `{}`}),
		callRequest(6, "lsp_trace_v1_export_retained_calls", map[string]any{"input": string(input)}),
	})
	for _, i := range []int{0, 1, 2, 3} {
		if call := decodeProcessCall(t, calls[i]); call.env["operation_status"] != "SUCCEEDED" {
			t.Fatalf("ASSERT_MCP_RETAINED_V2_SUCCESS_%d: %v", i, call.env)
		}
	}
	inline := inlineArtifactBytes(t, decodeProcessCall(t, calls[0]).env)
	if !bytes.Equal(inline, want) {
		t.Fatal("ASSERT_MCP_CLI_RETAINED_V2_EXACT_CANONICAL_PARITY")
	}
	published, err := os.ReadFile(filepath.Join(publication, "retained-v2.json"))
	if err != nil || !bytes.Equal(published, want) {
		t.Fatalf("ASSERT_MCP_RETAINED_V2_IMMUTABLE_PUBLICATION: %v", err)
	}
	compactCall := decodeProcessCall(t, calls[2]).env
	if compactCall["summary"] == nil || compactCall["content"] != nil || compactCall["publication_receipt"] == nil {
		t.Fatal("ASSERT_MCP_RETAINED_V2_COMPACT_PUBLICATION", compactCall)
	}
	publicationCall := decodeProcessCall(t, calls[1]).env
	receipt, _ := publicationCall["publication_receipt"].(map[string]any)
	if publicationCall["artifact_schema_id"] != "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.retained-calls.v2.schema.json" || receipt["artifact_schema_id"] != publicationCall["artifact_schema_id"] {
		t.Fatal("ASSERT_MCP_RETAINED_V2_PUBLICATION_FAMILY", publicationCall)
	}
	if call := decodeProcessCall(t, calls[4]); call.env["operation_status"] != "FAILED" || call.env["code"] != "INPUT_INVALID" {
		t.Fatal("ASSERT_MCP_RETAINED_V2_CORRUPT_REJECT", call.env)
	}
	if call := decodeProcessCall(t, calls[5]); call.env["operation_status"] != "FAILED" || call.env["code"] != "INPUT_INVALID" {
		t.Fatal("ASSERT_MCP_RETAINED_V1_REJECTS_V2", call.env)
	}
	if got, err := retainedcalls.ExportV2(input); err != nil || !bytes.Equal(got, want) {
		t.Fatal("ASSERT_MCP_RETAINED_V2_SHARED_EXPORT", err)
	}
}

// Hermetic transport qualification is separate from installed-gopls evidence.
func TestRetainedCallsOfflineCLIAndMCP(t *testing.T) {
	root := t.TempDir()
	r := graph.Result{SchemaVersion: graph.SchemaVersionV3, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
	for i := 0; i < 6; i++ {
		file := filepath.Join(root, fmt.Sprintf("f%d.go", i))
		if err := os.WriteFile(file, []byte("package p\n"), 0600); err != nil {
			t.Fatal(err)
		}
		uri := (&url.URL{Scheme: "file", Path: file}).String()
		n := graph.NewNode(graph.Item{Name: fmt.Sprintf("F%d", i), Kind: 12, URI: uri, Range: graph.Range{End: graph.Position{Line: 10}}, SelectionRange: graph.Range{End: graph.Position{Character: 1}}})
		r.Nodes = append(r.Nodes, n)
		if i > 0 {
			r.Edges = graph.MergeEdge(r.Edges, graph.Edge{CallerNodeID: r.Nodes[0].ID, CalleeNodeID: n.ID, CallSites: []graph.Range{{Start: graph.Position{Line: 1}, End: graph.Position{Line: 1, Character: 1}}, {Start: graph.Position{Line: 2}, End: graph.Position{Line: 2, Character: 1}}}})
		}
	}
	seed := r.Nodes[0].URI
	ids := []string{}
	rels := []string{}
	for _, n := range r.Nodes {
		ids = append(ids, n.ID)
	}
	for _, e := range r.Edges {
		rels = append(rels, e.RelationID)
	}
	r.Targets = ids[:1]
	r.Invocation.Target = graph.Target{URI: seed}
	r.Invocation.Seeds = []graph.InvocationSeed{{Label: "start", At: seed + ":0:0", ResolvedURI: seed}}
	r.Seeds = []graph.SeedResult{{Label: "start", Requested: r.Invocation.Target, PreparedTargetIDs: ids[:1], ReachedNodeIDs: ids, ReachedRelationIDs: rels, ReachedEdges: r.Edges}}
	r.Slice = &graph.SliceEvidence{StartMode: "at", SourceURI: seed, DownDepth: 1, UpDepth: 1, StartingNodeIDs: ids[:1], Layers: []graph.SliceLayer{{Depth: 0, NodeIDs: ids[:1]}, {Depth: 1, NodeIDs: ids[1:]}}, FrontierNodeIDs: ids[1:], UpwardStartNodeIDs: ids[1:], OutgoingRelationIDs: rels}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	input, err := graphprovenance.Capture(context.Background(), raw, root, seed, "synthetic", 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	testRetainedCallsRealOffline(t, buildBinary(t, "lsp-trace", "./cmd/lsp-trace"), buildMCPBinary(t), input)
}

// Also called by the real gopls fixture only after deleting all six source files.
func testRetainedCallsRealOffline(t *testing.T, cli, mcp string, input []byte) {
	t.Helper()
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.json")
	if err := os.WriteFile(inputPath, input, 0600); err != nil {
		t.Fatal(err)
	}
	raw := runCLIProcess(t, cli, "export-retained-calls", inputPath)
	var e retainedcalls.Evidence
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	if len(e.Tables.Groups) != 5 || len(e.Tables.Occurrences) != 10 || e.Tables.SupportTotal != 5 {
		t.Fatal("ASSERT_REAL_TWO_CALLSITES_PER_GROUP_NO_SUPPORT_MULTIPLICATION")
	}
	for _, g := range e.Tables.Groups {
		if len(g.OccurrenceIDs) != 2 || g.Receipt.SupportContribution != 1 {
			t.Fatal("ASSERT_REAL_DISTINCT_RETAINED_RANGES")
		}
	}
	e.InputBytes = nil
	if p, err := retainedcalls.Reconstruct(e.Tables); err != nil || len(p.Edges) != 5 || p.Receipt.SupportTotal != 5 {
		t.Fatalf("ASSERT_REAL_TABLES_ONLY: %v", err)
	}
	testBoundedRealOffline(t, cli, mcp, raw)
	testMetricsRealOffline(t, cli, mcp, raw)
	exportedPath := filepath.Join(dir, "exported.json")
	if err := os.WriteFile(exportedPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if output := runCLIProcess(t, cli, "validate", "--family", "retained-calls", "--version", "v1", exportedPath); !strings.Contains(string(output), "valid lsp-trace.retained-calls.v1") {
		t.Fatal("ASSERT_REAL_CLI_RETAINED_VALIDATE")
	}
	selectorPath := filepath.Join(dir, "selected.json")
	runCLIProcess(t, cli, "export-retained-calls", "--output", selectorPath, inputPath)
	selected, _ := os.ReadFile(selectorPath)
	selector, err := verification.DecodeSelector(selected)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := os.ReadFile(filepath.Join(dir, selector.Generation, "artifact.json"))
	if err != nil || !bytes.Equal(artifact, raw) {
		t.Fatal("ASSERT_REAL_RETAINED_PUBLICATION_EXACT_BYTES")
	}
	runCLIProcess(t, cli, "verify", "--family", "retained-calls", "--version", "v1", selectorPath)
	if _, err := exec.Command(cli, "verify", selectorPath).CombinedOutput(); err == nil {
		t.Fatal("ASSERT_DEFAULT_VERIFY_REMAINS_GRAPH_ONLY")
	}
	if _, err := exec.Command(cli, "export-retained-calls", "--output", selectorPath, inputPath).CombinedOutput(); err == nil {
		t.Fatal("ASSERT_RETAINED_NO_REPLACE")
	}
	after, _ := os.ReadFile(selectorPath)
	if !bytes.Equal(after, selected) {
		t.Fatal("ASSERT_SELECTOR_NOT_REPLACED")
	}
	publication := t.TempDir()
	offline := runMCPProcess(t, mcp, []string{"--publication-root", publication}, []map[string]any{
		callRequest(1, "lsp_trace_v1_export_retained_calls", map[string]any{"input": string(input)}),
		callRequest(2, "lsp_trace_v1_validate", map[string]any{"input": string(raw), "schema": map[string]any{"family": "retained-calls", "version": "v1"}}),
		callRequest(3, "lsp_trace_v1_schema_get", map[string]any{"schema": map[string]any{"family": "retained-calls", "version": "v1"}}),
		callRequest(4, "lsp_trace_v1_export_retained_calls", map[string]any{"input": string(input), "output_selector": "retained.json"}),
		callRequest(5, "lsp_trace_v1_export_retained_calls", map[string]any{"input": string(input), "output_selector": "retained.json"}),
		callRequest(6, "lsp_trace_v1_export_retained_calls", map[string]any{"input": string(input), "output_selector": "compact.json", "detail": "compact"}),
		callRequest(7, "lsp_trace_v1_export_retained_calls", map[string]any{"input": "{}"}),
	})
	for i, r := range offline[:4] {
		if call := decodeProcessCall(t, r); call.env["operation_status"] != "SUCCEEDED" {
			t.Fatalf("ASSERT_REAL_OFFLINE_RETAINED_MCP_%d: %v", i, call.env)
		}
	}
	if call := decodeProcessCall(t, offline[4]); call.env["operation_status"] != "FAILED" || call.env["outcome"] != "PUBLICATION_ERROR" {
		t.Fatal("ASSERT_MCP_RETAINED_NO_REPLACE", call.env)
	}
	if call := decodeProcessCall(t, offline[5]); call.env["operation_status"] != "SUCCEEDED" || call.env["summary"] == nil || call.env["content"] != nil {
		t.Fatal("ASSERT_MCP_RETAINED_COMPACT", call.env)
	}
	if call := decodeProcessCall(t, offline[6]); call.env["operation_status"] != "FAILED" || call.env["code"] != "INPUT_INVALID" {
		t.Fatal("ASSERT_MCP_RETAINED_INPUT_REJECT", call.env)
	}
	if !bytes.Equal(inlineArtifactBytes(t, decodeProcessCall(t, offline[0]).env), raw) {
		t.Fatal("ASSERT_REAL_CLI_MCP_RETAINED_PARITY")
	}
	if !bytes.Equal(inlineArtifactBytes(t, decodeProcessCall(t, offline[1]).env), raw) {
		t.Fatal("ASSERT_MCP_RETAINED_VALIDATE_BYTES")
	}
	schemaRaw := runCLIProcess(t, cli, "schema", "get", "--family", "retained-calls", "--version", "v1")
	if !bytes.Equal(inlineArtifactBytes(t, decodeProcessCall(t, offline[2]).env), schemaRaw) {
		t.Fatal("ASSERT_RETAINED_SCHEMA_GET_PARITY")
	}
	published, err := os.ReadFile(filepath.Join(publication, "retained.json"))
	if err != nil || !bytes.Equal(published, raw) {
		t.Fatalf("ASSERT_MCP_RETAINED_PUBLICATION: %v", err)
	}
}
