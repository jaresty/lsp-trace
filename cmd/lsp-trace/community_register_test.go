package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/programcpresentation"
	"lsp-trace/internal/programctestfixture"
)

func leidenPartition(t *testing.T, raw []byte, seed uint64) []byte {
	t.Helper()
	a, err := programcpresentation.Handle(programcpresentation.Request{Input: raw, Seed: seed, PageRankTopK: 2, HubTopK: 2})
	if err != nil {
		t.Fatal(err)
	}
	b, err := programcpresentation.JSON(a)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestCommunityRegisterEntryPointsAndCompatibility(t *testing.T) {
	graph := programctestfixture.ValidV5(t)
	partition := leidenPartition(t, graph, 19)
	d := t.TempDir()
	graphPath, partitionPath, registerPath := filepath.Join(d, "graph.json"), filepath.Join(d, "partition.json"), filepath.Join(d, "register.json")
	_ = os.WriteFile(graphPath, graph, 0600)
	_ = os.WriteFile(partitionPath, partition, 0600)

	var baseline, emitted, stderr strings.Builder
	args := []string{"--seed", "19", "--pagerank-top-k", "2", "--hub-top-k", "2", "--format", "json", graphPath}
	if runProgramCLeiden(args, strings.NewReader(""), &baseline, &stderr) != 0 {
		t.Fatal(stderr.String())
	}
	stderr.Reset()
	withRegister := append([]string{"--emit-community-register", registerPath}, args...)
	if runProgramCLeiden(withRegister, strings.NewReader(""), &emitted, &stderr) != 0 {
		t.Fatal(stderr.String())
	}
	if baseline.String() != emitted.String() {
		t.Fatal("ASSERT_DEFAULT_LEIDEN_BYTES_UNCHANGED")
	}
	registerBytes, err := os.ReadFile(registerPath)
	if err != nil {
		t.Fatal("ASSERT_REGISTER_SEPARATE_ARTIFACT", err)
	}

	var standalone, standaloneErr strings.Builder
	if code := runAggregateCommunities([]string{"--graph", graphPath, "--partition", partitionPath}, &standalone, &standaloneErr); code != 0 {
		t.Fatalf("ASSERT_REGISTER_SCHEMA_AND_TWO_ENTRY_POINTS: %d %s", code, standaloneErr.String())
	}
	if standalone.String() != string(registerBytes) {
		t.Fatal("ASSERT_DETERMINISTIC_GRAPH_BOUND_MEMBER_ID")
	}
	var doc map[string]any
	if json.Unmarshal(registerBytes, &doc) != nil {
		t.Fatal("ASSERT_REGISTER_SCHEMA_AND_TWO_ENTRY_POINTS")
	}
	if doc["authority"] != float64(0) || doc["source_graph_complete"] != "UNKNOWN" {
		t.Fatal("ASSERT_ZERO_AUTHORITY_UNKNOWN_COMPLETENESS")
	}
	stability := doc["stability"].(map[string]any)
	if stability["status"] != "UNKNOWN" || stability["reason"] != "INSUFFICIENT_COMPARABLE_PARTITIONS" || stability["denominator"] != float64(0) {
		t.Fatal("ASSERT_SINGLE_PARTITION_UNKNOWN")
	}
	if _, ok := stability["policy"]; !ok {
		t.Fatal("ASSERT_TYPED_STABILITY_POLICY_DENOMINATOR")
	}
	if strings.Contains(string(registerBytes), "ownership") || strings.Contains(string(registerBytes), "feature") || strings.Contains(string(registerBytes), "CALLS") {
		t.Fatal("ASSERT_NO_SEMANTIC_CLAIMS")
	}
	communities := doc["communities"].([]any)
	if len(communities) == 0 {
		t.Fatal("ASSERT_SINGLETONS_PRESERVED")
	}
	occurrence := communities[0].(map[string]any)["occurrences"].([]any)[0].(map[string]any)
	if occurrence["partition_id"] == "" || occurrence["local_community_id"] == "" || occurrence["seed"] != float64(19) {
		t.Fatal("ASSERT_OCCURRENCE_PROVENANCE")
	}

	standalone.Reset()
	standaloneErr.Reset()
	if runAggregateCommunities([]string{"--graph", graphPath, "--partition", partitionPath, "--partition", partitionPath}, &standalone, &standaloneErr) == 0 {
		t.Fatal("ASSERT_PARTITION_COVERAGE_AND_DUPLICATES_REJECTED")
	}
	badGraph := filepath.Join(d, "bad.json")
	_ = os.WriteFile(badGraph, []byte(`{"schema_version":"lsp-trace.graph-provenance.v4"}`), 0600)
	standalone.Reset()
	standaloneErr.Reset()
	if runAggregateCommunities([]string{"--graph", badGraph, "--partition", partitionPath}, &standalone, &standaloneErr) == 0 {
		t.Fatal("ASSERT_EXACT_V5_REJECTED")
	}
}
