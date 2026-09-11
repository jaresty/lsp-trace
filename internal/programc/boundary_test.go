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

func validBoundary(t *testing.T) (Outcome, BoundaryRequest, BoundaryArtifact) {
	t.Helper()
	o, r := boundaryFixture(), BoundaryRequest{1, 1}
	a, err := ComputeBoundary(o, r)
	if err != nil {
		t.Fatal(err)
	}
	return o, r, a
}
func requireBoundaryReject(t *testing.T, o Outcome, r BoundaryRequest, a BoundaryArtifact) {
	t.Helper()
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	if ValidateBoundary(b, o, r) == nil {
		t.Fatal("mutation accepted")
	}
}
func TestBoundaryPolicyCanonicalBytesAndDigest(t *testing.T) {
	const wantBytes = `{"conductance":"cut_weight(C)/(min(directed_out_volume(C),directed_out_volume(V\\C)))","conductance_zero_outcome":"UNAVAILABLE:zero minimum directed out-volume","crossing_witness_semantics":"direct-community-pair-minimum-occurrence-identity/v1","hub_score":"weighted-directed-in-plus-out-occurrence-sum/v1","hub_selection":"required-top-k-inclusive-ties-then-crossing/v1","id":"program-c-a-07-boundary-accounting/v2","pagerank":{"alpha":0.85,"arithmetic":"Go-binary64-explicit-rounding-sequential/v1","convergence":"stationary-L1-inclusive/v1","max_iterations":1000,"max_work":1000000,"score":"positive-binary64-node-score/v1","tolerance":1e-9,"work":"evaluation-3n+m/v1"},"pagerank_selection":"required-top-k-inclusive-ties-then-crossing/v1","weak_critical_semantics":"weak-undirected-occurrence-multigraph-bridge-articulation/v1"}`
	const wantDigest = "sha256:f6d68b67ff8117c573f795b95697accd85e345998228cb3ce0b59864c967d8d7"
	if string(BoundaryPolicyBytes()) != wantBytes || BoundaryPolicyDigest() != wantDigest {
		t.Fatalf("ASSERT_POLICY_IDENTITY bytes=%s digest=%s", BoundaryPolicyBytes(), BoundaryPolicyDigest())
	}
}
func TestBoundaryPageRankEnforcesExactMaxWork(t *testing.T) {
	const n = 10000
	es := make([]Occurrence, n-1)
	out := make([]float64, n)
	for i := 0; i < n-1; i++ {
		es[i] = Occurrence{Identity: string(rune(i + 1)), From: int64(i), To: int64(i + 1), Weight: 1}
		out[i] = 1
	}
	scores, got := pageRank(n, es, out)
	if got.Status != "INCOMPLETE" || got.Reason != "LIMIT" || got.Work != boundaryPageRankMaxWork || scores != nil {
		t.Fatalf("ASSERT_PAGERANK_MAX_WORK %+v scores=%v", got, scores)
	}
}
func TestValidateBoundaryAcceptsRecomputedArtifact(t *testing.T) {
	o, r, a := validBoundary(t)
	b, _ := json.Marshal(a)
	if err := ValidateBoundary(b, o, r); err != nil {
		t.Fatal(err)
	}
}
func TestValidateBoundaryRejectsArithmeticCountMutation(t *testing.T) {
	o, r, a := validBoundary(t)
	a.Accounting.CrossingOccurrences++
	requireBoundaryReject(t, o, r, a)
}
func TestValidateBoundaryRejectsArithmeticDenominatorMutation(t *testing.T) {
	o, r, a := validBoundary(t)
	a.Communities[0].Conductance.Denominator++
	requireBoundaryReject(t, o, r, a)
}
func TestValidateBoundaryRejectsCanonicalOrderingMutation(t *testing.T) {
	o, r, a := validBoundary(t)
	a.Communities[0], a.Communities[1] = a.Communities[1], a.Communities[0]
	requireBoundaryReject(t, o, r, a)
}
func TestValidateBoundaryRejectsSourceDigestBindingMutation(t *testing.T) {
	o, r, a := validBoundary(t)
	a.Bindings.SourceSHA256 = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	requireBoundaryReject(t, o, r, a)
}
func TestValidateBoundaryRejectsSessionBindingMutation(t *testing.T) {
	o, r, a := validBoundary(t)
	a.Bindings.SessionID = "foreign"
	requireBoundaryReject(t, o, r, a)
}
func TestValidateBoundaryRejectsGenerationBindingMutation(t *testing.T) {
	o, r, a := validBoundary(t)
	a.Bindings.Generation++
	requireBoundaryReject(t, o, r, a)
}
func TestValidateBoundaryRejectsClaimCeilingBindingMutation(t *testing.T) {
	o, r, a := validBoundary(t)
	a.ClaimCeiling = "foreign"
	requireBoundaryReject(t, o, r, a)
}
func TestValidateBoundaryRejectsPolicyAlphaMutation(t *testing.T) {
	o, r, a := validBoundary(t)
	a.Policy.PageRank.Alpha = .9
	requireBoundaryReject(t, o, r, a)
}
func TestValidateBoundaryRejectsPolicyToleranceMutation(t *testing.T) {
	o, r, a := validBoundary(t)
	a.Policy.PageRank.Tolerance = 1e-8
	requireBoundaryReject(t, o, r, a)
}
func TestValidateBoundaryRejectsPolicyMaxIterationsMutation(t *testing.T) {
	o, r, a := validBoundary(t)
	a.Policy.PageRank.MaxIterations--
	requireBoundaryReject(t, o, r, a)
}
func TestValidateBoundaryRejectsPolicyMaxWorkMutation(t *testing.T) {
	o, r, a := validBoundary(t)
	a.Policy.PageRank.MaxWork--
	requireBoundaryReject(t, o, r, a)
}
func TestValidateBoundaryRejectsPolicyArithmeticIdentityMutation(t *testing.T) {
	o, r, a := validBoundary(t)
	a.Policy.PageRank.Arithmetic = "foreign"
	requireBoundaryReject(t, o, r, a)
}
func TestValidateBoundaryRejectsPolicyScoreIdentityMutation(t *testing.T) {
	o, r, a := validBoundary(t)
	a.Policy.PageRank.Score = "foreign"
	requireBoundaryReject(t, o, r, a)
}
func TestValidateBoundaryRejectsPolicyDigestMutation(t *testing.T) {
	o, r, a := validBoundary(t)
	a.Bindings.PolicySHA256 = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	requireBoundaryReject(t, o, r, a)
}
func TestValidateBoundaryRejectsTieForeignKeyMutation(t *testing.T) {
	o, r, a := validBoundary(t)
	a.HubCrossingNodes[0].NodeID = "foreign"
	requireBoundaryReject(t, o, r, a)
}
func TestValidateBoundaryRejectsWitnessForeignKeyMutation(t *testing.T) {
	o, r, a := validBoundary(t)
	a.CrossingWitnesses[0].OccurrenceID = "foreign"
	requireBoundaryReject(t, o, r, a)
}
func TestValidateBoundaryRejectsPageRankWorkMutation(t *testing.T) {
	o, r, a := validBoundary(t)
	a.PageRank.Work++
	requireBoundaryReject(t, o, r, a)
}
func TestValidateBoundaryRejectsPageRankIterationsMutation(t *testing.T) {
	o, r, a := validBoundary(t)
	a.PageRank.Iterations++
	requireBoundaryReject(t, o, r, a)
}
func TestValidateBoundaryRejectsPageRankResidualMutation(t *testing.T) {
	o, r, a := validBoundary(t)
	v := *a.PageRank.Residual + 1
	a.PageRank.Residual = &v
	requireBoundaryReject(t, o, r, a)
}
func TestValidateBoundaryRejectsPageRankConvergenceMutation(t *testing.T) {
	o, r, a := validBoundary(t)
	a.PageRank.Status = "INCOMPLETE"
	a.PageRank.Reason = "NOT_CONVERGED"
	requireBoundaryReject(t, o, r, a)
}
func TestValidateBoundaryRejectsUnknownField(t *testing.T) {
	o, r, a := validBoundary(t)
	b, _ := json.Marshal(a)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	m["unknown"] = true
	b, _ = json.Marshal(m)
	if ValidateBoundary(b, o, r) == nil {
		t.Fatal("unknown field accepted")
	}
}
