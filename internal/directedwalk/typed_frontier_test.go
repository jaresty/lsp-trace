package directedwalk_test

import (
	"testing"

	"lsp-trace/internal/directedwalk"
)

func TestTypedWalkDoesNotClaimDirectionalPathThroughOmittedNode(t *testing.T) {
	got, err := directedwalk.Walk(directedwalk.Request{
		Root: "A", Kinds: []directedwalk.Kind{directedwalk.Calls, directedwalk.ReferencesSymbol, directedwalk.ResolvesToDefinition},
		DownDepth: 1, UpDepth: 3, MaxNodes: 3, MaxCandidates: 8, MaxWork: directedwalk.MaxWorkLimit,
		Candidates: []directedwalk.Candidate{
			{ID: "down-x", Kind: directedwalk.Calls, From: "A", To: "X"},
			{ID: "down-z", Kind: directedwalk.Calls, From: "A", To: "Z"},
			{ID: "up-x", Kind: directedwalk.ReferencesSymbol, From: "X", To: "A"},
			{ID: "up-y", Kind: directedwalk.ResolvesToDefinition, From: "Y", To: "X"},
			{ID: "up-z", Kind: directedwalk.ReferencesSymbol, From: "Z", To: "Y"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.NodesOmitted != 1 || len(got.Nodes) != 3 || got.Nodes[0].ID != "A" ||
		got.Nodes[1].ID != "X" || got.Nodes[2].ID != "Z" || got.Nodes[2].DownDepth == nil ||
		*got.Nodes[2].DownDepth != 1 || got.Nodes[2].UpDepth != nil ||
		byID(got)["up-z"].State != directedwalk.NodeBound ||
		byKind(got)[directedwalk.ReferencesSymbol].UpNodeBound != 1 {
		t.Fatalf("ASSERT_TYPED_NO_GHOST_UP_PATH: nodes=%v occurrences=%v counts=%v", got.Nodes, got.Occurrences, got.Counts)
	}
	t.Log("ASSERT_TYPED_NO_GHOST_UP_PATH: PASS")
}

func TestTypedWalkDoesNotDeduplicateKindsAtSharedEndpoints(t *testing.T) {
	req := mixedRequest()
	req.Candidates = append(req.Candidates, directedwalk.Candidate{ID: "ref-at-call", Kind: directedwalk.ReferencesSymbol, From: "A", To: "B"})
	got, err := directedwalk.Walk(req)
	if err != nil {
		t.Fatal(err)
	}
	if byID(got)["ref-at-call"].State != directedwalk.Witnessed ||
		byID(got)["call-1"].State != directedwalk.Witnessed ||
		byKind(got)[directedwalk.ReferencesSymbol].Total != 3 ||
		byKind(got)[directedwalk.ReferencesSymbol].Witnessed != 1 ||
		byKind(got)[directedwalk.Calls].Witnessed != 1 {
		t.Fatalf("ASSERT_TYPED_CROSS_KIND_MULTIPLICITY: occurrences=%v counts=%v", got.Occurrences, got.Counts)
	}
	t.Log("ASSERT_TYPED_CROSS_KIND_MULTIPLICITY: PASS")
}
