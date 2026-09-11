package programc

import (
	"encoding/json"
	"testing"

	"lsp-trace/internal/schema"
)

func boundaryFixture() Outcome {
	ids := []string{"a", "b", "c", "d", "e"}
	p := Projection{NodeIdentities: ids, NodeIDs: map[string]int64{"a": 0, "b": 1, "c": 2, "d": 3, "e": 4}, PairWeights: map[Pair]float64{}, Occurrences: []Occurrence{
		{Identity: "o1", From: 0, To: 0, Weight: 1}, {Identity: "o2", From: 0, To: 1, Weight: 1}, {Identity: "o3", From: 0, To: 1, Weight: 1}, {Identity: "o4", From: 1, To: 2, Weight: 2}, {Identity: "o5", From: 2, To: 3, Weight: 1}}}
	for _, e := range p.Occurrences {
		p.PairWeights[Pair{e.From, e.To}] += e.Weight
	}
	return Outcome{ProfileID: ProfileID, ProfileDigest: ProfileDigest, Algorithm: algorithm, Resolution: 1, Seed: 7, LogicalDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Communities: []Community{{Members: []string{"a", "b"}}, {Members: []string{"c", "d"}}, {Members: []string{"e"}}}, Projection: p, Source: p.Source}
}
func TestBoundaryPartitionAndConductance(t *testing.T) {
	a, err := ComputeBoundary(boundaryFixture(), BoundaryRequest{PageRankTopK: 1, HubTopK: 1})
	if err != nil {
		t.Fatal(err)
	}
	if a.Accounting.AdmittedOccurrences != 5 || a.Accounting.IntraOccurrences != 4 || a.Accounting.CrossingOccurrences != 1 {
		t.Fatalf("ASSERT_PARTITION %+v", a.Accounting)
	}
	var measure BoundaryMeasure
	for _, community := range a.Communities {
		if len(community.Members) == 2 && community.Members[0] == "a" {
			measure = community.Conductance
		}
	}
	if measure.Outcome != "VALUE" || measure.Numerator != 2 || measure.Denominator != 1 {
		t.Fatalf("ASSERT_CONDUCTANCE %+v", measure)
	}
}
func TestBoundaryRankAndHubTies(t *testing.T) {
	o := boundaryFixture()
	o.Projection.Occurrences = []Occurrence{{Identity: "x", From: 0, To: 2, Weight: 1}, {Identity: "y", From: 1, To: 3, Weight: 1}}
	a, err := ComputeBoundary(o, BoundaryRequest{PageRankTopK: 1, HubTopK: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(a.HighCentralityCrossingNodes) < 2 || len(a.HubCrossingNodes) != 4 {
		t.Fatalf("ASSERT_TOP_K_TIES rank=%v hub=%v", a.HighCentralityCrossingNodes, a.HubCrossingNodes)
	}
}
func TestBoundaryWeakMultigraph(t *testing.T) {
	a, err := ComputeBoundary(boundaryFixture(), BoundaryRequest{PageRankTopK: 1, HubTopK: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Bridges) != 2 || a.Bridges[0] != "o4" || a.Bridges[1] != "o5" {
		t.Fatalf("ASSERT_MULTIGRAPH_BRIDGES %v", a.Bridges)
	}
	if len(a.ArticulationPoints) != 2 || a.ArticulationPoints[0] != "b" || a.ArticulationPoints[1] != "c" {
		t.Fatalf("ASSERT_ARTICULATION %v", a.ArticulationPoints)
	}
}
func TestBoundaryCrossingWitness(t *testing.T) {
	a, _ := ComputeBoundary(boundaryFixture(), BoundaryRequest{PageRankTopK: 1, HubTopK: 1})
	if len(a.CrossingWitnesses) != 1 || a.CrossingWitnesses[0].OccurrenceID != "o4" {
		t.Fatalf("ASSERT_CROSSING_WITNESS %v", a.CrossingWitnesses)
	}
}
func TestBoundaryRejectsInvalidRequest(t *testing.T) {
	o := boundaryFixture()
	for _, r := range []BoundaryRequest{{}, {PageRankTopK: 6, HubTopK: 1}, {PageRankTopK: 1, HubTopK: 6}} {
		if _, e := ComputeBoundary(o, r); e == nil {
			t.Fatalf("ASSERT_INVALID_TOP_K %+v", r)
		}
	}
	o.Communities[0].Members = append(o.Communities[0].Members, "c")
	if _, e := ComputeBoundary(o, BoundaryRequest{1, 1}); e == nil {
		t.Fatal("ASSERT_INVALID_PARTITION")
	}
}
func TestBoundaryEmptyAndUnavailable(t *testing.T) {
	o := boundaryFixture()
	o.Projection.Occurrences = nil
	o.Projection.PairWeights = map[Pair]float64{}
	a, e := ComputeBoundary(o, BoundaryRequest{1, 1})
	if e != nil {
		t.Fatal(e)
	}
	if a.Outcome != "EMPTY" || a.Communities[0].Conductance.Outcome != "UNAVAILABLE" {
		t.Fatalf("ASSERT_EMPTY_UNAVAILABLE %+v", a)
	}
}
func TestBoundarySchema(t *testing.T) {
	a, err := ComputeBoundary(boundaryFixture(), BoundaryRequest{1, 1})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(a)
	if _, err = schema.ValidateFor(b, schema.FamilyCommunityBoundary, "v1"); err != nil {
		t.Fatal("ASSERT_BOUNDARY_SCHEMA", err)
	}
}

func TestValidateBoundaryMutations(t *testing.T) {
	o := boundaryFixture()
	a, e := ComputeBoundary(o, BoundaryRequest{1, 1})
	if e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(a)
	if e = ValidateBoundary(b, o, BoundaryRequest{1, 1}); e != nil {
		t.Fatal(e)
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	m["digest"] = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	bad, _ := json.Marshal(m)
	if ValidateBoundary(bad, o, BoundaryRequest{1, 1}) == nil {
		t.Fatal("ASSERT_DIGEST_MUTATION")
	}
}
