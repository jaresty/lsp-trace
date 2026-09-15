package transientstructural

import (
	"testing"

	"lsp-trace/internal/graph"
)

func TestNeighborhoodAnalysisPreservesLocatorFacts(t *testing.T) {
	wantRange := graph.Range{Start: graph.Position{Line: 3, Character: 4}, End: graph.Position{Line: 3, Character: 9}}
	projection := admittedProjection{
		targetID:    "root",
		nodes:       []NodeFact{{ID: "root", Name: "Run", Kind: 12, URI: "file:///workspace/main.go", Range: wantRange}},
		occurrences: []OccurrenceFact{{ID: "call", CallerID: "root", CalleeID: "root", URI: "file:///workspace/main.go", Range: wantRange}},
	}
	got := analyze(AnalysisRequest{Kind: AnalysisNeighborhood}, projection, BoundsBinding{})
	if len(got.Nodes) != 1 || got.Nodes[0].Name != "Run" || got.Nodes[0].Kind != 12 || got.Nodes[0].URI != "file:///workspace/main.go" || got.Nodes[0].Range != wantRange {
		t.Fatalf("ASSERT_V2_NEIGHBORHOOD_PRESERVES_NODE_LOCATOR: %+v", got.Nodes)
	}
	if len(got.Occurrences) != 1 || got.Occurrences[0].URI != "file:///workspace/main.go" || got.Occurrences[0].Range != wantRange {
		t.Fatalf("ASSERT_V2_NEIGHBORHOOD_PRESERVES_CALL_SITE: %+v", got.Occurrences)
	}
}
