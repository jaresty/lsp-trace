package programc

import (
	"encoding/json"
	"math"
	"testing"

	"lsp-trace/internal/graph"
)

func metricRun(id string, cs ...[]string) InstabilityRun {
	c := make([]Community, len(cs))
	for i, m := range cs {
		x := append([]string(nil), m...)
		c[i] = Community{Members: x}
	}
	return InstabilityRun{RunID: id, Status: "COMPLETE", Community: Outcome{Communities: c}}
}
func TestInstabilityIdenticalAndPermutation(t *testing.T) {
	a := metricRun("a", []string{"a", "b"}, []string{"c"})
	b := metricRun("b", []string{"c"}, []string{"a", "b"})
	p, e := comparePartitions(a, b)
	if e != nil || p.Outcome != "STABLE" || p.NodeReassignmentNumerator != 0 || p.UnmatchedCommunityNumerator != 0 || p.VariationOfInformationBits != 0 || p.PairwiseJaccardMinimum != 1 {
		t.Fatalf("ASSERT_IDENTICAL_PERMUTATION %+v %v", p, e)
	}
}
func TestInstabilitySplitMergeAndUnequalUniverse(t *testing.T) {
	a := metricRun("a", []string{"a", "b", "c"})
	b := metricRun("b", []string{"a"}, []string{"b", "c"}, []string{"d"})
	p, e := comparePartitions(a, b)
	if e != nil {
		t.Fatal(e)
	}
	if p.Outcome != "UNSTABLE" || p.NodeReassignmentNumerator != 4 || p.NodeReassignmentDenominator != 4 || p.UnmatchedCommunityNumerator != 2 || p.UnmatchedCommunityDenominator != 4 || p.VariationOfInformationNodeDenominator != 4 {
		t.Fatalf("ASSERT_SPLIT_UNEQUAL %+v", p)
	}
}
func TestInstabilitySingletonEdgelessAndZeroDenominators(t *testing.T) {
	a := metricRun("a", []string{"a"}, []string{"b"})
	b := metricRun("b", []string{"a"}, []string{"b"})
	p, e := comparePartitions(a, b)
	if e != nil || p.Outcome != "STABLE" {
		t.Fatalf("ASSERT_SINGLETON %+v %v", p, e)
	}
	emptyA, emptyB := metricRun("x"), metricRun("y")
	p, e = comparePartitions(emptyA, emptyB)
	if e != nil || p.NodeReassignmentDenominator != 0 || p.UnmatchedCommunityDenominator != 0 || p.MatchedPairDenominator != 0 || math.IsNaN(p.VariationOfInformationBits) {
		t.Fatalf("ASSERT_ZERO_DENOM %+v %v", p, e)
	}
}
func TestInstabilityUnmatchedSingletonIsReassigned(t *testing.T) {
	left := metricRun("left", []string{"x"})
	right := metricRun("right")
	pair, err := comparePartitions(left, right)
	if err != nil {
		t.Fatal(err)
	}
	if pair.NodeReassignmentNumerator != 1 || pair.NodeReassignmentDenominator != 1 {
		t.Fatalf("ASSERT_UNMATCHED_SINGLETON_REASSIGNED numerator=%d denominator=%d", pair.NodeReassignmentNumerator, pair.NodeReassignmentDenominator)
	}
}

func TestInstabilityMatchingTieDeterministic(t *testing.T) {
	a := metricRun("a", []string{"a"}, []string{"b"})
	b := metricRun("b", []string{"a", "b"}, []string{"c"})
	x, e := maximumMatching(a.Community.Communities, b.Community.Communities)
	if e != nil {
		t.Fatal(e)
	}
	y, e := maximumMatching(a.Community.Communities, b.Community.Communities)
	if e != nil || string(canonicalInstability(x)) != string(canonicalInstability(y)) {
		t.Fatal("ASSERT_TIE_DETERMINISM")
	}
	if len(x) != 2 || compareStrings(x[0].l, []string{"a"}) != 0 || compareStrings(x[0].r, []string{"a", "b"}) != 0 {
		t.Fatalf("ASSERT_TIE_LEXICOGRAPHIC_MEMBER_SETS pairs=%+v", x)
	}
}
func campaignFixture(t *testing.T) InstabilityRequest {
	t.Helper()
	a, b := node("a", 0), node("b", 1)
	input := validV5(t, []graph.Node{a, b}, []graph.Edge{{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{}}}})
	base, failure := Compute(input, 1)
	if failure != nil {
		t.Fatal(failure)
	}
	id := InstabilityIdentity{
		AdmittedGraphSHA256:       base.Source.InputSHA256,
		ProjectionSHA256:          ProfileDigest,
		ProjectionPolicyID:        ProfileID,
		AlgorithmName:             algorithm,
		AlgorithmVersion:          "gonum-v0.17.1",
		ParametersCanonicalSHA256: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		ResourcePolicySHA256:      "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
	}
	r := InstabilityRequest{Identity: id, DeclaredSeeds: []uint64{1, 2}}
	for _, seed := range r.DeclaredSeeds {
		for replay := 0; replay < 3; replay++ {
			o, failure := Compute(input, seed)
			if failure != nil {
				t.Fatal(failure)
			}
			cw := expectedCommunityWire(o, id)
			cb, _ := json.Marshal(cw)
			ba, e := ComputeBoundary(o, BoundaryRequest{1, 1})
			if e != nil {
				t.Fatal(e)
			}
			bb, _ := json.Marshal(ba)
			r.Runs = append(r.Runs, InstabilityRun{RunID: string(rune('a' + len(r.Runs))), Seed: seed, Status: "COMPLETE", Community: o, CommunityArtifact: cb, BoundaryArtifact: bb, BoundaryRequest: BoundaryRequest{1, 1}})
		}
	}
	return r
}
func TestComputeAndValidateInstability(t *testing.T) {
	r := campaignFixture(t)
	a, e := ComputeInstability(r)
	if e != nil {
		t.Fatal(e)
	}
	if a.Outcome != "STABLE" || a.RunAccounting.ExpectedPairCount != 15 || a.RunAccounting.CompletedPairCount != 15 {
		t.Fatalf("ASSERT_ACCOUNTING %+v", a)
	}
	repeated, e := ComputeInstability(r)
	if e != nil || string(canonicalInstability(a)) != string(canonicalInstability(repeated)) {
		t.Fatal("ASSERT_ARTIFACT_DETERMINISM")
	}
	raw, _ := json.Marshal(a)
	if e = ValidateInstability(raw, r); e != nil {
		t.Fatal(e)
	}
	a.PairwiseComparisons[0].NodeReassignmentDenominator++
	raw, _ = json.Marshal(a)
	if ValidateInstability(raw, r) == nil {
		t.Fatal("ASSERT_ARITHMETIC_MUTATION")
	}
}
func TestInstabilityPartitionHashesArePairBound(t *testing.T) {
	r := campaignFixture(t)
	a, err := ComputeInstability(r)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(a)
	var doc map[string]any
	_ = json.Unmarshal(raw, &doc)
	identity := doc["comparison_identity"].(map[string]any)
	if _, ok := identity["left_community_artifact_sha256"]; ok {
		t.Fatal("ASSERT_EXACT_COMPARISON_IDENTITY")
	}
	pair := doc["pairwise_comparisons"].([]any)[0].(map[string]any)
	if pair["left_community_artifact_sha256"] == "" || pair["right_community_artifact_sha256"] == "" {
		t.Fatal("ASSERT_PAIR_PARTITION_BINDINGS")
	}
}

func TestInstabilityIncompleteFailedAccounting(t *testing.T) {
	r := campaignFixture(t)
	r.Runs[0].Status = "FAILED"
	r.Runs[1].Status = "INCOMPLETE"
	a, e := ComputeInstability(r)
	if e != nil {
		t.Fatal(e)
	}
	if a.Outcome != "INCOMPLETE" || a.RunAccounting.BlockedPairCount != 9 || a.RunAccounting.CompletedPairCount != 6 || a.RunAccounting.FailedRunCount != 1 || a.RunAccounting.IncompleteRunCount != 1 {
		t.Fatalf("ASSERT_BLOCKED_ACCOUNTING %+v", a.RunAccounting)
	}
}
func TestInstabilityRejectsCrossRunSessionBinding(t *testing.T) {
	r := campaignFixture(t)
	r.Runs[1].Community.Source.SessionID = "other-session"
	boundary, err := ComputeBoundary(r.Runs[1].Community, r.Runs[1].BoundaryRequest)
	if err != nil {
		t.Fatal(err)
	}
	r.Runs[1].BoundaryArtifact, _ = json.Marshal(boundary)
	if _, err := ComputeInstability(r); err == nil {
		t.Fatal("ASSERT_CROSS_RUN_SESSION_BINDING")
	}
}

func TestInstabilityRejectsResealedSourceScalarMutation(t *testing.T) {
	r := campaignFixture(t)
	for i := range r.Runs {
		r.Runs[i].Community.Source.SessionID = "forged-session"
		r.Runs[i].Community.Projection.Source = r.Runs[i].Community.Source
		boundary, err := ComputeBoundary(r.Runs[i].Community, r.Runs[i].BoundaryRequest)
		if err != nil {
			t.Fatal(err)
		}
		r.Runs[i].BoundaryArtifact, _ = json.Marshal(boundary)
	}
	if _, err := ComputeInstability(r); err == nil {
		t.Fatal("ASSERT_SOURCE_INPUT_SEMANTIC_RECOMPUTATION")
	}
}

func TestInstabilityRejectsForeignAndPolicyCaps(t *testing.T) {
	r := campaignFixture(t)
	r.Runs[0].Community.Communities[0].Members[0] = "foreign"
	if _, e := ComputeInstability(r); e == nil {
		t.Fatal("ASSERT_FOREIGN_ID")
	}
	cs := make([]Community, 63)
	for i := range cs {
		cs[i] = Community{[]string{string(rune(i + 1))}}
	}
	if _, e := maximumMatching(cs, cs); e == nil {
		t.Fatal("ASSERT_WORK_CAP")
	}
}
func TestInstabilityPolicyIdentity(t *testing.T) {
	const digest = "sha256:b44cef6e72f2e808202961b94f91a45d97e8bcaded8e0e91ebf6cda64a5bd67c"
	if InstabilityPolicyDigest() != digest {
		t.Fatalf("ASSERT_POLICY_DIGEST bytes=%s digest=%s", InstabilityPolicyBytes(), InstabilityPolicyDigest())
	}
}
func TestValidateInstabilityMutationMatrix(t *testing.T) {
	r := campaignFixture(t)
	a, err := ComputeInstability(r)
	if err != nil {
		t.Fatal(err)
	}
	baseline, _ := json.Marshal(a)
	if err := ValidateInstability(baseline, r); err != nil {
		t.Fatalf("ASSERT_MUTATION_BASELINE_ACCEPTED: %v", err)
	}
	mutations := map[string]func(map[string]any){
		"identity":       func(m map[string]any) { m["comparison_identity"].(map[string]any)["algorithm_version"] = "forged" },
		"policy-version": func(m map[string]any) { m["policy_version"] = "forged" },
		"policy":         func(m map[string]any) { m["policy"].(map[string]any)["matching"] = "greedy" },
		"policy-digest":  func(m map[string]any) { m["policy_sha256"] = "sha256:" + string(make([]byte, 64)) },
		"accounting":     func(m map[string]any) { m["run_accounting"].(map[string]any)["completed_pair_count"] = float64(14) },
		"partition-binding": func(m map[string]any) {
			m["pairwise_comparisons"].([]any)[0].(map[string]any)["left_community_artifact_sha256"] = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		},
		"claim":            func(m map[string]any) { m["claim_ceiling"] = "semantic communities" },
		"structural-claim": func(m map[string]any) { m["structural_claim_ceiling"] = "unbounded" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			var doc map[string]any
			_ = json.Unmarshal(baseline, &doc)
			mutate(doc)
			raw, _ := json.Marshal(doc)
			if ValidateInstability(raw, r) == nil {
				t.Fatalf("ASSERT_MUTATION_REJECTED_%s", name)
			}
		})
	}
}

func TestValidateInstabilityRejectsUnknownField(t *testing.T) {
	r := campaignFixture(t)
	a, _ := ComputeInstability(r)
	raw, _ := json.Marshal(a)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	m["foreign"] = true
	raw, _ = json.Marshal(m)
	if ValidateInstability(raw, r) == nil {
		t.Fatal("ASSERT_UNKNOWN_FIELD")
	}
}
