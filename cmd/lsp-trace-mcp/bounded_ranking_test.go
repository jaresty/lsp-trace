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

func TestRankingPublicRED(t *testing.T) {
	if _, err := schema.BytesFor("bounded-retained-ranking", "v1"); err != nil {
		t.Error("ASSERT_RANKING_SCHEMA", err)
	}
	r := mcp.NewRegistry(false)
	for _, name := range []string{"lsp_trace_v1_bounded_retained_ranking", "lsp_trace_bounded_retained_ranking"} {
		if _, ok := r.Resolve(name); !ok {
			t.Error("ASSERT_RANKING_TOOL", name)
		}
	}
}
func TestRankingAlgorithmPublicRED(t *testing.T) {
	raw, _ := boundedFixture(t)
	cli := buildBinary(t, "lsp-trace", "./cmd/lsp-trace")
	file := filepath.Join(t.TempDir(), "retained.json")
	if err := os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(cli, "bounded-retained-ranking", "--algorithm", "PAGERANK", file).CombinedOutput()
	if err != nil {
		t.Fatalf("ASSERT_RANKING_EXECUTABLE: %v %s", err, out)
	}
	var e struct {
		Status string `json:"status"`
		Scores []struct {
			ID    string  `json:"node_id"`
			Score float64 `json:"score"`
		} `json:"scores"`
		Residual *float64 `json:"residual"`
	}
	if err = json.Unmarshal(out, &e); err != nil {
		t.Fatal(err)
	}
	mass := 0.
	for _, s := range e.Scores {
		mass += s.Score
	}
	if e.Status != "COMPLETE" || len(e.Scores) != 7 || e.Residual == nil || *e.Residual > 1e-9 || mass < .999999999 || mass > 1.000000001 {
		t.Fatal("ASSERT_RANKING_STATIONARY_MASS", e, mass)
	}
}
