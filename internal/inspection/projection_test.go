package inspection

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"

	"lsp-trace/internal/graph"
)

func projectionFixture(t *testing.T) []byte {
	t.Helper()
	n1 := graph.NewNode(graph.Item{Name: "one", URI: "file:///one.go"})
	n2 := graph.NewNode(graph.Item{Name: "two", URI: "file:///two.go"})
	edges := graph.MergeEdge(nil, graph.Edge{CallerNodeID: n1.ID, CalleeNodeID: n2.ID})
	bundle := Bundle{
		SchemaVersion: graph.SchemaVersionV3,
		Invocation:    graph.Invocation{Seeds: []graph.InvocationSeed{{Label: "second"}, {Label: "first"}}},
		Nodes:         []graph.Node{n1, n2}, Edges: edges,
		Diagnostics: []graph.Diagnostic{{NodeID: n2.ID, Message: "reached"}},
		Seeds: []graph.SeedResult{
			{Label: "first", ReachedNodeIDs: []string{n1.ID}},
			{Label: "second", ReachedNodeIDs: []string{n2.ID}, ReachedRelationIDs: []string{edges[0].RelationID}},
		},
	}
	data, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestProjectSeedPreservesAuthorityNormalizationAndNativeOrder(t *testing.T) {
	got, err := ProjectSeed(projectionFixture(t), "second")
	if err != nil {
		t.Fatal(err)
	}
	if got.Authority != "NON_AUTHORITATIVE_DERIVED_VIEW" || got.Seed.Label != "second" || len(got.Nodes) != 1 || got.Nodes[0].Name != "two" || len(got.Relations) != 1 || got.Seed.PreparedTargetIDs == nil || got.Global.Diagnostics == nil || len(got.DiagnosticsOnReachedNodes.Diagnostics) != 1 {
		t.Fatalf("ASSERT_SINGLE_SEED_PROJECTION_PARITY: %#v", got)
	}
}

func TestProjectAllSeedsUsesInvocationOrderAndRejectsDuplicateResults(t *testing.T) {
	data := projectionFixture(t)
	got, err := ProjectAllSeeds(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Seeds) != 2 || got.Seeds[0].Seed.Label != "second" || got.Seeds[1].Seed.Label != "first" || got.Accounting.RequestedSeedCount != 2 {
		t.Fatalf("ASSERT_ALL_SEED_INVOCATION_ORDER: %#v", got)
	}
	var bundle Bundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		t.Fatal(err)
	}
	bundle.Seeds = append(bundle.Seeds, bundle.Seeds[0])
	bad, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ProjectAllSeeds(bad); err == nil || !strings.Contains(err.Error(), `seed label "first" has 2 results; expected exactly one`) {
		t.Fatalf("ASSERT_ALL_SEED_DUPLICATE_RESULT_REJECTED: %v", err)
	}
}

func TestProjectAllSeedsV5IncludesExactSiblingEndpointsPerMembership(t *testing.T) {
	origin := graph.NewNode(graph.Item{Name: "origin", URI: "file:///source.go"})
	candidate := graph.NewNode(graph.Item{Name: "candidate", URI: "file:///source.go"})
	relation := graph.SiblingCandidate{RelationID: "sibling-1", SeedLabel: "left", Origin: origin, Candidate: candidate, Direction: "SIBLING", Kind: "TOPMOST_SIBLING"}
	bundle := Bundle{
		SchemaVersion:     graph.SchemaVersionV5,
		Invocation:        graph.Invocation{Seeds: []graph.InvocationSeed{{Label: "left"}, {Label: "right"}}},
		Seeds:             []graph.SeedResult{{Label: "left"}, {Label: "right"}},
		SiblingCandidates: []graph.SiblingCandidate{relation},
		SeedMemberships:   []graph.SeedMembership{{SeedLabel: "left", EvidenceKind: "SIBLING_CANDIDATE", EndpointID: relation.RelationID}},
	}
	raw, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ProjectAllSeeds(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Seeds[0].NativeNodeIDs, []string{origin.ID, candidate.ID}) || len(got.Seeds[1].NativeNodeIDs) != 0 {
		t.Fatalf("ASSERT_V5_SIBLING_ENDPOINTS_PARTITIONED_PER_SEED: %#v", got.Seeds)
	}
	if len(got.Records.SiblingCandidates) != 1 || !reflect.DeepEqual(got.Records.SiblingCandidates[0], relation) {
		t.Fatalf("ASSERT_V5_SIBLING_CORRESPONDENCE_PRESERVED: %#v", got.Records.SiblingCandidates)
	}
}

func TestProjectionConcurrentReadOnlyDeterminism(t *testing.T) {
	data := projectionFixture(t)
	before := append([]byte(nil), data...)
	const workers = 32
	outputs := make([][]byte, workers)
	var wg sync.WaitGroup
	for i := range outputs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			got, err := ProjectAllSeeds(data)
			if err != nil {
				t.Errorf("ASSERT_CONCURRENT_PROJECTION_SUCCEEDS: %v", err)
				return
			}
			outputs[i], err = json.Marshal(got)
			if err != nil {
				t.Errorf("ASSERT_CONCURRENT_PROJECTION_MARSHALS: %v", err)
			}
		}(i)
	}
	wg.Wait()
	for i := 1; i < workers; i++ {
		if !bytes.Equal(outputs[0], outputs[i]) {
			t.Fatalf("ASSERT_CONCURRENT_PROJECTION_DETERMINISTIC: output %d differs", i)
		}
	}
	if !bytes.Equal(data, before) {
		t.Fatal("ASSERT_PROJECTION_INPUT_READ_ONLY: input mutated")
	}
}
