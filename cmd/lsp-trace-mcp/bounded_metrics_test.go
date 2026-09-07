package main

import (
	"encoding/json"
	"lsp-trace/internal/mcp"
	"lsp-trace/internal/schema"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMetricsPublicContractRED(t *testing.T) {
	if _, err := schema.BytesFor("bounded-retained-metrics", "v1"); err != nil {
		t.Error("ASSERT_METRICS_SEPARATE_SCHEMA", err)
	}
	r := mcp.NewRegistry(false)
	for _, name := range []string{"lsp_trace_v1_bounded_retained_metrics", "lsp_trace_bounded_retained_metrics"} {
		if _, ok := r.Resolve(name); !ok {
			t.Error("ASSERT_METRICS_PUBLIC_TOOL_ALIAS", name)
		}
	}
}
func TestMetricsAlgorithmPublicRED(t *testing.T) {
	raw, ids := boundedFixture(t)
	cli := buildBinary(t, "lsp-trace", "./cmd/lsp-trace")
	file := filepath.Join(t.TempDir(), "retained.json")
	if err := os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(cli, "bounded-retained-metrics", file).CombinedOutput()
	if err != nil {
		t.Fatalf("ASSERT_METRICS_EXECUTABLE_ALGORITHM: %v: %s", err, out)
	}
	var e struct {
		NodeCount  int `json:"node_count"`
		GroupCount int `json:"group_count"`
		Reported   int `json:"reported_occurrence_count"`
		Unreported int `json:"unreported_group_count"`
		Loops      int `json:"self_loop_group_count"`
		Pairs      int `json:"distinct_nonloop_pair_count"`
		Nodes      []struct {
			ID  string `json:"id"`
			In  int    `json:"in_group_degree"`
			Out int    `json:"out_group_degree"`
			NI  int    `json:"in_distinct_neighbors"`
			NO  int    `json:"out_distinct_neighbors"`
		} `json:"nodes"`
		Density struct {
			Policy string `json:"policy"`
			Status string `json:"status"`
			Value  *struct {
				Numerator   int `json:"numerator"`
				Denominator int `json:"denominator"`
			} `json:"value"`
		} `json:"density"`
	}
	if err = json.Unmarshal(out, &e); err != nil {
		t.Fatal(err)
	}
	if e.NodeCount != 7 || e.GroupCount != 7 || e.Reported != 2 || e.Unreported != 6 || e.Loops != 1 || e.Pairs != 6 {
		t.Fatal("ASSERT_METRICS_GLOBAL_COUNTERS", e)
	}
	if e.Density.Policy != "DIRECTED_DISTINCT_NONLOOP_PAIRS" || e.Density.Status != "DEFINED" || e.Density.Value == nil || e.Density.Value.Numerator != 6 || e.Density.Value.Denominator != 42 {
		t.Fatal("ASSERT_METRICS_EXACT_DENSITY", e.Density)
	}
	sums := [2]int{}
	found := false
	for _, n := range e.Nodes {
		sums[0] += n.In
		sums[1] += n.Out
		if n.ID == ids[6] {
			found = true
			if n.In+n.Out+n.NI+n.NO != 0 {
				t.Fatal("ASSERT_METRICS_ISOLATE_ZERO", n)
			}
		}
	}
	if !found || sums != [2]int{7, 7} {
		t.Fatal("ASSERT_METRICS_DEGREE_SUM_ISOLATE", sums, found)
	}
}
