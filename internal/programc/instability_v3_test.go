package programc

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestComputeAndValidateInstabilityV3OrdinaryAndPrecedence(t *testing.T) {
	request := campaignFixture(t)
	artifact, err := ComputeInstabilityV3(request)
	if err != nil {
		t.Fatal(err)
	}
	if artifact.SchemaVersion != InstabilityVersionV3 || artifact.Outcome != "STABLE" || artifact.RunAccounting.CompletedPairCount != 15 || artifact.RunAccounting.BlockedMatchingResourcePairCount != 0 {
		t.Fatalf("ASSERT_V3_ORDINARY_ACCOUNTING artifact=%+v", artifact)
	}
	raw, _ := json.Marshal(artifact)
	if err := ValidateInstabilityV3(raw, request); err != nil {
		t.Fatalf("ASSERT_V3_ORDINARY_VALIDATES err=%v", err)
	}

	request.Runs[0].Status = "FAILED"
	request.Runs[1].Status = "INCOMPLETE"
	artifact, err = ComputeInstabilityV3(request)
	if err != nil {
		t.Fatal(err)
	}
	if artifact.Outcome != "INCOMPLETE" || artifact.RunAccounting.BlockedFailedRunPairCount != 5 || artifact.RunAccounting.BlockedIncompleteRunPairCount != 4 || artifact.RunAccounting.CompletedPairCount != 6 || len(artifact.PairwiseComparisons) != 15 {
		t.Fatalf("ASSERT_V3_BLOCKED_PRECEDENCE artifact=%+v", artifact)
	}
	for _, pair := range artifact.PairwiseComparisons {
		if (pair.LeftRunID == request.Runs[0].RunID || pair.RightRunID == request.Runs[0].RunID) && pair.Outcome != "BLOCKED_FAILED_RUN" {
			t.Fatalf("ASSERT_V3_FAILED_PRECEDENCE pair=%+v", pair)
		}
	}
}

func TestInstabilityV3DeterministicPermutationAndMutationRejection(t *testing.T) {
	request := campaignFixture(t)
	first, err := ComputeInstabilityV3(request)
	if err != nil {
		t.Fatal(err)
	}
	for left, right := 0, len(request.Runs)-1; left < right; left, right = left+1, right-1 {
		request.Runs[left], request.Runs[right] = request.Runs[right], request.Runs[left]
	}
	second, err := ComputeInstabilityV3(request)
	if err != nil || !bytes.Equal(canonicalInstability(first), canonicalInstability(second)) {
		t.Fatalf("ASSERT_V3_PERMUTATION_DETERMINISTIC err=%v", err)
	}
	raw, _ := json.Marshal(second)
	mutations := map[string]func(map[string]any){
		"mixed-version": func(m map[string]any) { m["schema_version"] = InstabilityVersionV2 },
		"accounting":    func(m map[string]any) { m["run_accounting"].(map[string]any)["completed_pair_count"] = float64(14) },
		"resource-on-completed": func(m map[string]any) {
			m["pairwise_comparisons"].([]any)[0].(map[string]any)["guard_kind"] = "MATCHING_WORK_LIMIT"
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			var document map[string]any
			_ = json.Unmarshal(raw, &document)
			mutate(document)
			mutated, _ := json.Marshal(document)
			if ValidateInstabilityV3(mutated, request) == nil {
				t.Fatalf("ASSERT_V3_MUTATION_REJECTED name=%s", name)
			}
		})
	}
}

func TestComparePartitionsV3MatchingStateWidthPrecedesWork(t *testing.T) {
	communities := func(width int) [][]string {
		out := make([][]string, width)
		for i := range out {
			out[i] = []string{string(rune('a' + i))}
		}
		return out
	}
	pair, err := comparePartitionsV3(metricRun("left", communities(1)...), metricRun("right", communities(63)...))
	if err != nil || pair.Outcome != "BLOCKED_MATCHING_RESOURCE" || pair.GuardKind != "MATCHING_STATE_WIDTH" || pair.MatchingSmallerCommunityCount != 1 || pair.MatchingLargerCommunityCount != 63 || pair.MatchingStateCount != nil || pair.MatchingWork != nil || pair.MatchingWorkLimit != instabilityMaxMatchingWork {
		t.Fatalf("ASSERT_V3_MATCHING_STATE_WIDTH pair=%+v err=%v", pair, err)
	}
}

func matchingResourceDispositionFixture(t *testing.T) []byte {
	t.Helper()
	pairs := make([]InstabilityPairV3, 0, 15)
	for left := 'a'; left <= 'f'; left++ {
		for right := left + 1; right <= 'f'; right++ {
			states, work := uint64(131072), uint64(2228224)
			pairs = append(pairs, InstabilityPairV3{LeftRunID: string(left), RightRunID: string(right), Outcome: "BLOCKED_MATCHING_RESOURCE", GuardKind: "MATCHING_WORK_LIMIT", MatchingSmallerCommunityCount: 17, MatchingLargerCommunityCount: 17, MatchingStateCount: &states, MatchingWork: &work, MatchingWorkLimit: instabilityMaxMatchingWork})
		}
	}
	digest := "sha256:" + strings.Repeat("a", 64)
	artifact := InstabilityArtifactV3{SchemaVersion: InstabilityVersionV3, ComparisonIdentity: InstabilityIdentity{AdmittedGraphSHA256: digest, ProjectionSHA256: digest, ProjectionPolicyID: "calls-v1", AlgorithmName: "leiden", AlgorithmVersion: "v", ParametersCanonicalSHA256: digest, ResourcePolicySHA256: digest}, Outcome: "INCOMPLETE", Thresholds: exactInstabilityThresholds, PolicyVersion: InstabilityPolicyVersion, Policy: InstabilityPolicy, PolicySHA256: InstabilityPolicyDigest(), RunAccounting: InstabilityAccountingV3{DeclaredSeedCount: 2, DeclaredRunCount: 6, CompletedRunCount: 6, ExpectedPairCount: 15, BlockedMatchingResourcePairCount: 15}, PairwiseComparisons: pairs, ClaimCeiling: InstabilityClaimCeilingV3, StructuralClaimCeiling: InstabilityStructuralCeiling}
	raw, _ := json.Marshal(artifact)
	return raw
}

func TestValidateInstabilityV3MatchingResourceDispositionMutations(t *testing.T) {
	raw := matchingResourceDispositionFixture(t)
	if err := ValidateInstabilityV3MatchingResourceDisposition(raw); err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(map[string]any){
		"accounting": func(m map[string]any) {
			m["run_accounting"].(map[string]any)["blocked_matching_resource_pair_count"] = float64(14)
		},
		"limit": func(m map[string]any) {
			m["pairwise_comparisons"].([]any)[0].(map[string]any)["matching_work_limit"] = float64(999999)
		},
		"fabricated-metric": func(m map[string]any) {
			m["pairwise_comparisons"].([]any)[0].(map[string]any)["pairwise_jaccard_mean"] = float64(0)
		},
		"duplicate-pair": func(m map[string]any) { pairs := m["pairwise_comparisons"].([]any); pairs[1] = pairs[0] },
		"foreign-missing-pair": func(m map[string]any) {
			m["pairwise_comparisons"].([]any)[0].(map[string]any)["right_run_id"] = "z"
		},
		"reordered-pairs": func(m map[string]any) {
			pairs := m["pairwise_comparisons"].([]any)
			pairs[0], pairs[1] = pairs[1], pairs[0]
		},
		"wrong-outcome": func(m map[string]any) { m["outcome"] = "STABLE" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			var document map[string]any
			_ = json.Unmarshal(raw, &document)
			mutate(document)
			mutated, _ := json.Marshal(document)
			if ValidateInstabilityV3MatchingResourceDisposition(mutated) == nil {
				t.Fatalf("ASSERT_V3_DISPOSITION_MUTATION_REJECTED name=%s", name)
			}
		})
	}
}

func TestComparePartitionsV3MatchingResourceBlocked(t *testing.T) {
	communities := func(prefix string, width int) [][]string {
		out := make([][]string, width)
		for i := range out {
			out[i] = []string{prefix + string(rune('a'+i))}
		}
		return out
	}
	left := metricRun("left", communities("left-", 17)...)
	right := metricRun("right", communities("right-", 17)...)

	pair, err := comparePartitionsV3(left, right)
	if err != nil {
		t.Fatal(err)
	}
	if pair.Outcome != "BLOCKED_MATCHING_RESOURCE" || pair.GuardKind != "MATCHING_WORK_LIMIT" ||
		pair.MatchingSmallerCommunityCount != 17 || pair.MatchingLargerCommunityCount != 17 ||
		pair.MatchingStateCount == nil || *pair.MatchingStateCount != 131072 ||
		pair.MatchingWork == nil || *pair.MatchingWork != 2228224 || pair.MatchingWorkLimit != 1000000 {
		t.Fatalf("ASSERT_V3_MATCHING_RESOURCE_DIMENSIONS pair=%+v", pair)
	}
	raw, err := json.Marshal(pair)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"node_reassignment_numerator", "node_reassignment_denominator",
		"unmatched_community_numerator", "unmatched_community_denominator",
		"variation_of_information_bits", "variation_of_information_node_denominator",
		"pairwise_jaccard_minimum", "pairwise_jaccard_median", "pairwise_jaccard_mean",
		"matched_pair_denominator",
	} {
		if _, ok := document[forbidden]; ok {
			t.Fatalf("ASSERT_V3_BLOCKED_PAIR_NO_FABRICATED_METRIC field=%s raw=%s", forbidden, raw)
		}
	}
}
