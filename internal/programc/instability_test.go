package programc

import (
	"encoding/json"
	"math"
	"testing"
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
	if p.Outcome != "UNSTABLE" || p.NodeReassignmentNumerator != 3 || p.NodeReassignmentDenominator != 4 || p.UnmatchedCommunityNumerator != 2 || p.UnmatchedCommunityDenominator != 4 || p.VariationOfInformationNodeDenominator != 4 {
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
func TestInstabilityMatchingTieDeterministic(t *testing.T) {
	a := metricRun("a", []string{"a"}, []string{"b"})
	b := metricRun("b", []string{"a", "b"}, []string{"a", "b"})
	x, e := maximumMatching(a.Community.Communities, b.Community.Communities)
	if e != nil {
		t.Fatal(e)
	}
	y, e := maximumMatching(a.Community.Communities, b.Community.Communities)
	if e != nil || string(canonicalInstability(x)) != string(canonicalInstability(y)) {
		t.Fatal("ASSERT_TIE_DETERMINISM")
	}
}
func campaignFixture(t *testing.T) InstabilityRequest {
	t.Helper()
	base := boundaryFixture()
	base.Source.InputSHA256 = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	base.Source.GraphV5SHA256 = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	base.Source.SessionID = "s"
	base.Source.Generation = 1
	base.ProfileID = ProfileID
	base.ProfileDigest = ProfileDigest
	base.Algorithm = algorithm
	base.Outcome = "COMPLETE"
	id := InstabilityIdentity{base.Source.InputSHA256, ProfileDigest, ProfileID, "", "", algorithm, "gonum-v0.17.1", "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"}
	r := InstabilityRequest{Identity: id, DeclaredSeeds: []uint64{1, 2}}
	for _, seed := range r.DeclaredSeeds {
		for replay := 0; replay < 3; replay++ {
			o := base
			o.Seed = seed
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
	r.Identity.LeftCommunityArtifactSHA256 = rawSHA(r.Runs[0].CommunityArtifact)
	r.Identity.RightCommunityArtifactSHA256 = rawSHA(r.Runs[3].CommunityArtifact)
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
