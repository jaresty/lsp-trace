package main

import "testing"

func TestGraphProvenanceCLIOptIn(t *testing.T) {
	_, err := parseSlice([]string{"--workspace", t.TempDir(), "--server", "gopls", "--at", "seed.go:2:6", "--graph-provenance"})
	if err != nil {
		t.Fatalf("ASSERT_GRAPH_PROVENANCE_CLI_ROUTE: %v", err)
	}
}
