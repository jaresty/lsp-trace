package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestPublicAnalyticsV2ProcessExactParity(t *testing.T) {
	mcpBinary := buildMCPBinary(t)
	cliBinary := buildBinary(t, "lsp-trace", "./cmd/lsp-trace")
	type graphWire struct {
		SchemaVersion string   `json:"schema_version"`
		BuildRevision string   `json:"build_revision"`
		Nodes         []string `json:"nodes"`
		Edges         []any    `json:"edges"`
	}
	graph, _ := json.Marshal(graphWire{"lsp-trace.normative-retained-graph.v1", "r1", []string{"a", "b"}, []any{}})
	root := t.TempDir()
	graphPath := filepath.Join(root, "graph.json")
	if err := os.WriteFile(graphPath, graph, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(graph)
	digest := "sha256:" + hex.EncodeToString(sum[:])
	selector := map[string]any{"selector": "graph.json", "artifact_schema_id": "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.normative-retained-graph.v1.schema.json", "artifact_digest": digest, "artifact_byte_length": len(graph)}
	for _, tc := range []struct{ command, tool, operation string }{
		{"bounded-retained-analysis-v2", "lsp_trace_v2_bounded_retained_analysis", "ANALYSIS"},
		{"bounded-retained-metrics-v2", "lsp_trace_v2_bounded_retained_metrics", "METRICS"},
		{"bounded-retained-ranking-v2", "lsp_trace_v2_bounded_retained_ranking", "RANKING"},
	} {
		for _, max := range []string{"1", "10000"} {
			cliInline := runCLIProcess(t, cliBinary, tc.command, "--operation", tc.operation, "--filter", "CALLS", "--max-work", max, graphPath)
			args := map[string]any{"input": string(graph), "operation": tc.operation, "filter": []string{"CALLS"}, "max_work": json.Number(max)}
			mcpInline := inlineArtifactBytes(t, decodeProcessCall(t, runMCPProcess(t, mcpBinary, nil, []map[string]any{callRequest(1, tc.tool, args)})[0]).env)
			args["input"] = json.RawMessage(graph)
			mcpRaw := inlineArtifactBytes(t, decodeProcessCall(t, runMCPProcess(t, mcpBinary, nil, []map[string]any{callRequest(2, tc.tool, args)})[0]).env)
			if string(cliInline) != string(mcpInline) || string(mcpInline) != string(mcpRaw) {
				t.Fatalf("ASSERT_PUBLIC_ANALYTICS_V2_INLINE_PROCESS_BYTE_PARITY %s/%s", tc.operation, max)
			}
			cliSelected := runCLIProcess(t, cliBinary, tc.command, "--operation", tc.operation, "--filter", "CALLS", "--max-work", max, "--publication-root", root, "--publication-selector", "graph.json", "--input-schema-id", selector["artifact_schema_id"].(string), "--input-digest", digest, "--input-byte-length", strconv.Itoa(len(graph)))
			selectedArgs := map[string]any{"publication_selector": selector, "operation": tc.operation, "filter": []string{"CALLS"}, "max_work": json.Number(max)}
			mcpSelected := inlineArtifactBytes(t, decodeProcessCall(t, runMCPProcess(t, mcpBinary, []string{"--publication-root", root}, []map[string]any{callRequest(3, tc.tool, selectedArgs)})[0]).env)
			if string(cliSelected) != string(mcpSelected) {
				t.Fatalf("ASSERT_PUBLIC_ANALYTICS_V2_SELECTOR_PROCESS_BYTE_PARITY %s/%s", tc.operation, max)
			}
		}
	}
}
