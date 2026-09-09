package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"lsp-trace/internal/schema"
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
	edge := struct {
		ID                  string `json:"id"`
		From                string `json:"from"`
		To                  string `json:"to"`
		Relation            string `json:"relation"`
		ProvenanceAuthority string `json:"provenance_authority"`
		ProvenanceCustody   string `json:"provenance_custody"`
		ProvenanceID        string `json:"provenance_id"`
		SupportGroup        string `json:"support_group"`
	}{"e1", "a", "b", "CALLS", "VALIDATED_AUTHORITY", "PROVIDER_VERIFIED", "p1", "g1"}
	graph, _ := json.Marshal(graphWire{"lsp-trace.normative-retained-graph.v1", "r1", []string{"a", "b"}, []any{edge}})
	root := t.TempDir()
	graphPath := filepath.Join(root, "graph.json")
	if err := os.WriteFile(graphPath, graph, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(graph)
	digest := "sha256:" + hex.EncodeToString(sum[:])
	selector := map[string]any{"selector": "graph.json", "artifact_schema_id": "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.normative-retained-graph.v1.schema.json", "artifact_digest": digest, "artifact_byte_length": len(graph)}
	for _, tc := range []struct{ command, tool, operation, family string }{
		{"bounded-retained-analysis-v2", "lsp_trace_v2_bounded_retained_analysis", "ANALYSIS", schema.FamilyBoundedAnalysisV2},
		{"bounded-retained-metrics-v2", "lsp_trace_v2_bounded_retained_metrics", "METRICS", schema.FamilyBoundedMetricsV2},
		{"bounded-retained-ranking-v2", "lsp_trace_v2_bounded_retained_ranking", "RANKING", schema.FamilyBoundedRankingV2},
	} {
		for _, max := range []string{"1", "10000"} {
			cliInline := runCLIProcess(t, cliBinary, tc.command, "--operation", tc.operation, "--filter", "CALLS", "--max-work", max, graphPath)
			args := map[string]any{"input": string(graph), "operation": tc.operation, "filter": []string{"CALLS"}, "max_work": json.Number(max)}
			mcpInline := inlineArtifactBytes(t, decodeProcessCall(t, runMCPProcess(t, mcpBinary, nil, []map[string]any{callRequest(1, tc.tool, args)})[0]).env)
			args["input"] = json.RawMessage(graph)
			mcpRaw := inlineArtifactBytes(t, decodeProcessCall(t, runMCPProcess(t, mcpBinary, nil, []map[string]any{callRequest(2, tc.tool, args)})[0]).env)
			if !bytes.Equal(cliInline, mcpInline) || !bytes.Equal(mcpInline, mcpRaw) {
				t.Fatalf("ASSERT_PUBLIC_ANALYTICS_V2_INLINE_PROCESS_BYTE_PARITY %s/%s", tc.operation, max)
			}
			if _, err := schema.ValidateFor(cliInline, tc.family, "v2"); err != nil {
				t.Fatalf("ASSERT_PUBLIC_ANALYTICS_V2_CERTIFIED_SCHEMA %s/%s: %v", tc.operation, max, err)
			}
			validated := decodeProcessCall(t, runMCPProcess(t, mcpBinary, nil, []map[string]any{callRequest(20, "lsp_trace_v1_validate", map[string]any{
				"input": string(cliInline), "schema": map[string]any{"family": tc.family, "version": "v2"},
			})})[0]).env
			if validated["outcome"] != "COMPLETE" || validated["operation_status"] != "SUCCEEDED" || !bytes.Equal(inlineArtifactBytes(t, validated), cliInline) {
				t.Fatalf("ASSERT_PUBLIC_ANALYTICS_V2_MCP_VALIDATE_EXACT_BYTES %s/%s: %v", tc.operation, max, validated)
			}
			wrongFamily := schema.FamilyBoundedAnalysisV2
			if tc.family == wrongFamily {
				wrongFamily = schema.FamilyBoundedMetricsV2
			}
			mixed := decodeProcessCall(t, runMCPProcess(t, mcpBinary, nil, []map[string]any{callRequest(21, "lsp_trace_v1_validate", map[string]any{
				"input": string(cliInline), "schema": map[string]any{"family": wrongFamily, "version": "v2"},
			})})[0]).env
			if mixed["outcome"] != "DOMAIN_ERROR" || mixed["operation_status"] != "FAILED" {
				t.Fatalf("ASSERT_PUBLIC_ANALYTICS_V2_SCHEMA_IDENTITY_MIXUP_REJECTED %s/%s: %v", tc.operation, max, mixed)
			}
			gotSchema := runCLIProcess(t, cliBinary, "schema", "get", "--family", tc.family, "--version", "v2")
			wantSchema, err := schema.BytesFor(tc.family, "v2")
			if err != nil || !bytes.Equal(gotSchema, wantSchema) {
				t.Fatalf("ASSERT_PUBLIC_ANALYTICS_V2_SCHEMA_GET %s/%s: %v", tc.operation, max, err)
			}
			published := filepath.Join(t.TempDir(), "artifact.json")
			runCLIProcess(t, cliBinary, tc.command, "--operation", tc.operation, "--filter", "CALLS", "--max-work", max, "--output", published, graphPath)
			selectorBytes, err := os.ReadFile(published)
			var generation struct {
				Generation string `json:"generation"`
			}
			if err == nil {
				err = json.Unmarshal(selectorBytes, &generation)
			}
			var publishedBytes []byte
			if err == nil {
				publishedBytes, err = os.ReadFile(filepath.Join(filepath.Dir(published), generation.Generation, "artifact.json"))
			}
			if err != nil || !bytes.Equal(publishedBytes, cliInline) {
				t.Fatalf("ASSERT_PUBLIC_ANALYTICS_V2_PUBLISHED_BYTES %s/%s: %v", tc.operation, max, err)
			}
			runCLIProcess(t, cliBinary, "verify", "--family", tc.family, "--version", "v2", published)
			cliSelected := runCLIProcess(t, cliBinary, tc.command, "--operation", tc.operation, "--filter", "CALLS", "--max-work", max, "--publication-root", root, "--publication-selector", "graph.json", "--input-schema-id", selector["artifact_schema_id"].(string), "--input-digest", digest, "--input-byte-length", strconv.Itoa(len(graph)))
			selectedArgs := map[string]any{"publication_selector": selector, "operation": tc.operation, "filter": []string{"CALLS"}, "max_work": json.Number(max)}
			mcpSelected := inlineArtifactBytes(t, decodeProcessCall(t, runMCPProcess(t, mcpBinary, []string{"--publication-root", root}, []map[string]any{callRequest(3, tc.tool, selectedArgs)})[0]).env)
			if string(cliSelected) != string(mcpSelected) {
				t.Fatalf("ASSERT_PUBLIC_ANALYTICS_V2_SELECTOR_PROCESS_BYTE_PARITY %s/%s", tc.operation, max)
			}
		}
	}
}
