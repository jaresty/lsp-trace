package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestProgramCI02FrozenFixtureParity binds Gate I I-02 to repository-owned
// retained inputs. Algorithm-oracle and resource-limit obligations remain in
// their owning package tests and are selected by scripts/check-program-c-i-02.sh.
func TestProgramCI02FrozenFixtureParity(t *testing.T) {
	cli := buildBinary(t, "lsp-trace", "./cmd/lsp-trace")
	mcp := buildMCPBinary(t)

	t.Run("retained-calls-v1", func(t *testing.T) {
		raw, err := os.ReadFile(filepath.Join("..", "..", "internal", "retainedcalls", "testdata", "frozen-v1-export.json"))
		if err != nil {
			t.Fatal(err)
		}
		testBoundedRealOffline(t, cli, mcp, raw)
		testMetricsRealOffline(t, cli, mcp, raw)
		testRankingRealOffline(t, cli, mcp, raw)
	})

	t.Run("normative-retained-graph-v1", func(t *testing.T) {
		raw, err := os.ReadFile(filepath.Join("testdata", "d01-program-b-normative-graph.json"))
		if err != nil {
			t.Fatal(err)
		}
		graphPath := filepath.Join(t.TempDir(), "graph.json")
		if err = os.WriteFile(graphPath, raw, 0600); err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			command, tool, operation string
		}{
			{"bounded-retained-analysis-v2", "lsp_trace_v2_bounded_retained_analysis", "ANALYSIS"},
			{"bounded-retained-metrics-v2", "lsp_trace_v2_bounded_retained_metrics", "METRICS"},
			{"bounded-retained-ranking-v2", "lsp_trace_v2_bounded_retained_ranking", "RANKING"},
		} {
			first := runCLIProcess(t, cli, tc.command, "--operation", tc.operation, "--filter", "CALLS", "--max-work", "1000000", graphPath)
			second := runCLIProcess(t, cli, tc.command, "--operation", tc.operation, "--filter", "CALLS", "--max-work", "1000000", graphPath)
			if !bytes.Equal(first, second) {
				t.Fatalf("ASSERT_PROGRAM_C_I02_NORMATIVE_REPLAY_%s", tc.operation)
			}
			params := map[string]any{"input": string(raw), "operation": tc.operation, "filter": []string{"CALLS"}, "max_work": json.Number("1000000")}
			call := runMCPProcess(t, mcp, nil, []map[string]any{callRequest(1, tc.tool, params)})[0]
			got := inlineArtifactBytes(t, decodeProcessCall(t, call).env)
			if !bytes.Equal(first, got) {
				t.Fatalf("ASSERT_PROGRAM_C_I02_NORMATIVE_CLI_MCP_PARITY_%s", tc.operation)
			}
		}
	})
}
