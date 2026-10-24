package programc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
)

var (
	_ func(InstabilityRequest) (InstabilityArtifact, error)                       = ComputeInstability
	_ func([]byte, InstabilityRequest) error                                      = ValidateInstability
	_ func(InstabilityRequest) (InstabilityArtifact, error)                       = ComputeInstabilityV2
	_ func([]byte, InstabilityRequest) error                                      = ValidateInstabilityV2
	_ func([]byte, []uint64, string, string, string) (InstabilityArtifact, error) = ComputeInstabilityV5
)

func unstableInstabilityRequest(t *testing.T) InstabilityRequest {
	t.Helper()
	nodes := make([]graph.Node, 4)
	for i := range nodes {
		nodes[i] = node(fmt.Sprintf("n%d", i), uint32(i))
	}
	edges := make([]graph.Edge, len(nodes))
	for i := range nodes {
		edges[i] = graph.Edge{CallerNodeID: nodes[i].ID, CalleeNodeID: nodes[(i+1)%len(nodes)].ID, CallSites: []graph.Range{{}}}
	}
	input := validV5(t, nodes, edges)
	base, failure := Compute(input, 1)
	if failure != nil {
		t.Fatal(failure)
	}
	identity := InstabilityIdentity{
		AdmittedGraphSHA256:       base.Source.InputSHA256,
		ProjectionSHA256:          ProfileDigest,
		ProjectionPolicyID:        ProfileID,
		AlgorithmName:             algorithm,
		AlgorithmVersion:          "gonum-v0.17.1",
		ParametersCanonicalSHA256: "sha256:" + strings.Repeat("c", 64),
		ResourcePolicySHA256:      "sha256:" + strings.Repeat("d", 64),
	}
	request := InstabilityRequest{Identity: identity, DeclaredSeeds: []uint64{1, 2}}
	for _, seed := range request.DeclaredSeeds {
		for replay := 0; replay < 3; replay++ {
			outcome, failure := Compute(input, seed)
			if failure != nil {
				t.Fatal(failure)
			}
			communityArtifact, err := json.Marshal(expectedCommunityWire(outcome, identity))
			if err != nil {
				t.Fatal(err)
			}
			boundaryRequest := BoundaryRequest{PageRankTopK: 1, HubTopK: 1}
			boundary, err := ComputeBoundary(outcome, boundaryRequest)
			if err != nil {
				t.Fatal(err)
			}
			boundaryArtifact, err := json.Marshal(boundary)
			if err != nil {
				t.Fatal(err)
			}
			request.Runs = append(request.Runs, InstabilityRun{
				RunID:             fmt.Sprintf("seed-%d-replay-%d", seed, replay+1),
				Seed:              seed,
				Status:            "COMPLETE",
				Community:         outcome,
				CommunityArtifact: communityArtifact,
				BoundaryArtifact:  boundaryArtifact,
				BoundaryRequest:   boundaryRequest,
			})
		}
	}
	return request
}

func TestInstabilityV1HistoricalUnstableClaimUnchangedAndValid(t *testing.T) {
	request := unstableInstabilityRequest(t)
	artifact, err := ComputeInstability(request)
	if err != nil {
		t.Fatal(err)
	}
	if artifact.Outcome != "UNSTABLE" || artifact.SchemaVersion != InstabilityVersion || artifact.ClaimCeiling != InstabilityClaimCeiling {
		t.Fatalf("ASSERT_V1_HISTORICAL_UNSTABLE_CLAIM artifact=%+v", artifact)
	}
	raw, err := json.Marshal(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateInstability(raw, request); err != nil {
		t.Fatalf("ASSERT_V1_HISTORICAL_UNSTABLE_VALID: %v", err)
	}
}

func TestInstabilityV2UnstableClaimAndComputationalParity(t *testing.T) {
	request := unstableInstabilityRequest(t)
	v1, err := ComputeInstability(request)
	if err != nil {
		t.Fatal(err)
	}
	v2, err := ComputeInstabilityV2(request)
	if err != nil {
		t.Fatal(err)
	}
	if v2.Outcome != "UNSTABLE" || v2.SchemaVersion != InstabilityVersionV2 || v2.ClaimCeiling != InstabilityClaimCeilingV2 {
		t.Fatalf("ASSERT_V2_UNSTABLE_COHERENT artifact=%+v", v2)
	}
	if !strings.Contains(v2.ClaimCeiling, "UNSTABLE means at least one completed comparison did not meet them") || !strings.Contains(v2.ClaimCeiling, "INCOMPLETE means blocked comparisons prevented a complete determination") {
		t.Fatalf("ASSERT_V2_CLAIM_COVERS_OUTCOMES: %q", v2.ClaimCeiling)
	}
	v1.SchemaVersion, v2.SchemaVersion = "", ""
	v1.ClaimCeiling, v2.ClaimCeiling = "", ""
	if !reflect.DeepEqual(v1, v2) {
		t.Fatalf("ASSERT_V1_V2_COMPUTATIONAL_PAYLOAD_PARITY\nv1=%s\nv2=%s", canonicalInstability(v1), canonicalInstability(v2))
	}
}

func TestValidateInstabilityV1V2CrossRejectionAndV2Mutations(t *testing.T) {
	request := unstableInstabilityRequest(t)
	v1, err := ComputeInstability(request)
	if err != nil {
		t.Fatal(err)
	}
	v2, err := ComputeInstabilityV2(request)
	if err != nil {
		t.Fatal(err)
	}
	v1Raw, _ := json.Marshal(v1)
	v2Raw, _ := json.Marshal(v2)
	if err := ValidateInstability(v1Raw, request); err != nil {
		t.Fatalf("ASSERT_V1_BASELINE_ACCEPTED: %v", err)
	}
	if err := ValidateInstabilityV2(v2Raw, request); err != nil {
		t.Fatalf("ASSERT_V2_BASELINE_ACCEPTED: %v", err)
	}
	if ValidateInstabilityV2(v1Raw, request) == nil || ValidateInstability(v2Raw, request) == nil {
		t.Fatal("ASSERT_V1_V2_CROSS_REJECTION")
	}

	var relabeled map[string]any
	_ = json.Unmarshal(v1Raw, &relabeled)
	relabeled["schema_version"] = InstabilityVersionV2
	relabeledRaw, _ := json.Marshal(relabeled)
	if ValidateInstabilityV2(relabeledRaw, request) == nil {
		t.Fatal("ASSERT_RELABELED_V1_REJECTED_BY_V2")
	}

	mutations := map[string]func(map[string]any){
		"schema-version": func(m map[string]any) { m["schema_version"] = InstabilityVersion },
		"claim":          func(m map[string]any) { m["claim_ceiling"] = InstabilityClaimCeiling },
		"identity":       func(m map[string]any) { m["comparison_identity"].(map[string]any)["algorithm_version"] = "forged" },
		"outcome":        func(m map[string]any) { m["outcome"] = "STABLE" },
		"pair": func(m map[string]any) {
			m["pairwise_comparisons"].([]any)[0].(map[string]any)["node_reassignment_denominator"] = float64(999)
		},
		"structural-claim": func(m map[string]any) { m["structural_claim_ceiling"] = "semantic authority" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			var document map[string]any
			_ = json.Unmarshal(v2Raw, &document)
			mutate(document)
			raw, _ := json.Marshal(document)
			if ValidateInstabilityV2(raw, request) == nil {
				t.Fatalf("ASSERT_V2_MUTATION_REJECTED_%s", name)
			}
		})
	}
}

func chunk105InstabilityRequest(t *testing.T) InstabilityRequest {
	t.Helper()
	componentSizes := []int{1, 2, 34, 68}
	nodes := make([]graph.Node, 0, 105)
	edges := make([]graph.Edge, 0, 104)
	for component, size := range componentSizes {
		start := len(nodes)
		for i := 0; i < size; i++ {
			nodes = append(nodes, node(fmt.Sprintf("chunk-105-c%02d-n%03d", component, i), uint32(len(nodes))))
		}
		if size > 1 {
			for i := 0; i < size; i++ {
				edges = append(edges, graph.Edge{
					CallerNodeID: nodes[start+i].ID,
					CalleeNodeID: nodes[start+(i+1)%size].ID,
					CallSites:    []graph.Range{{}},
				})
			}
		}
	}
	input := validV5(t, nodes, edges)
	base, failure := Compute(input, 1)
	if failure != nil {
		t.Fatal(failure)
	}
	identity := InstabilityIdentity{
		AdmittedGraphSHA256:       base.Source.InputSHA256,
		ProjectionSHA256:          ProfileDigest,
		ProjectionPolicyID:        ProfileID,
		AlgorithmName:             algorithm,
		AlgorithmVersion:          "gonum-v0.17.1",
		ParametersCanonicalSHA256: "sha256:" + strings.Repeat("c", 64),
		ResourcePolicySHA256:      "sha256:" + strings.Repeat("d", 64),
	}
	request := InstabilityRequest{Identity: identity, DeclaredSeeds: []uint64{1, 2}}
	for _, seed := range request.DeclaredSeeds {
		for replay := 0; replay < 3; replay++ {
			outcome, computeFailure := Compute(input, seed)
			if computeFailure != nil {
				t.Fatal(computeFailure)
			}
			communityArtifact, err := json.Marshal(expectedCommunityWire(outcome, identity))
			if err != nil {
				t.Fatal(err)
			}
			boundaryRequest := BoundaryRequest{PageRankTopK: 1, HubTopK: 1}
			boundary, err := ComputeBoundary(outcome, boundaryRequest)
			if err != nil {
				t.Fatal(err)
			}
			boundaryArtifact, err := json.Marshal(boundary)
			if err != nil {
				t.Fatal(err)
			}
			request.Runs = append(request.Runs, InstabilityRun{
				RunID:             fmt.Sprintf("seed-%d-replay-%d", seed, replay+1),
				Seed:              seed,
				Status:            "COMPLETE",
				Community:         outcome,
				CommunityArtifact: communityArtifact,
				BoundaryArtifact:  boundaryArtifact,
				BoundaryRequest:   boundaryRequest,
			})
		}
	}
	return request
}

func chunk105RequestPermutations(request InstabilityRequest) []InstabilityRequest {
	permutations := []InstabilityRequest{request}
	reversedRuns := request
	reversedRuns.Runs = slices.Clone(request.Runs)
	slices.Reverse(reversedRuns.Runs)
	permutations = append(permutations, reversedRuns)
	reversedSeeds := request
	reversedSeeds.DeclaredSeeds = slices.Clone(request.DeclaredSeeds)
	slices.Reverse(reversedSeeds.DeclaredSeeds)
	permutations = append(permutations, reversedSeeds)
	both := reversedRuns
	both.DeclaredSeeds = slices.Clone(reversedSeeds.DeclaredSeeds)
	permutations = append(permutations, both)
	return permutations
}

func TestInstabilityV2Chunk105CompleteRequestCanonicalBytes(t *testing.T) {
	request := chunk105InstabilityRequest(t)
	var baseline []byte
	for permutationIndex, permutation := range chunk105RequestPermutations(request) {
		artifact, err := ComputeInstabilityV2(permutation)
		if err != nil {
			t.Fatalf("ASSERT_V2_CHUNK_105_COMPLETE_REQUEST permutation=%d: %v", permutationIndex, err)
		}
		raw := canonicalInstability(artifact)
		if err := ValidateInstabilityV2(raw, permutation); err != nil {
			t.Fatalf("ASSERT_V2_CHUNK_105_PUBLIC_VALIDATE permutation=%d: %v", permutationIndex, err)
		}
		if baseline == nil {
			baseline = bytes.Clone(raw)
		} else if !bytes.Equal(raw, baseline) {
			t.Fatalf("ASSERT_V2_CHUNK_105_CANONICAL_BYTES permutation=%d", permutationIndex)
		}
	}

	artifact, err := ComputeInstabilityV2(request)
	if err != nil {
		t.Fatal(err)
	}
	foundEquivalentAuthenticPair := false
	for _, pair := range artifact.PairwiseComparisons {
		if strings.HasPrefix(pair.LeftRunID, "seed-1-") && strings.HasPrefix(pair.RightRunID, "seed-1-") && pair.LeftCommunityArtifactSHA256 == pair.RightCommunityArtifactSHA256 {
			foundEquivalentAuthenticPair = true
			break
		}
	}
	if !foundEquivalentAuthenticPair {
		t.Fatal("ASSERT_V2_AUTHENTIC_EQUIVALENT_COMMUNITY_DIGEST_PAIR")
	}
}

func TestInstabilityV2Chunk105LegacyComparatorCounterfactual(t *testing.T) {
	one := 1.0
	halfULP := math.Ldexp(1, -53)
	sequential := one
	sequential += halfULP
	sequential += halfULP
	grouped := one + (halfULP + halfULP)
	if sequential == grouped || sequential != one || grouped != math.Nextafter(one, math.Inf(1)) {
		t.Fatal("ASSERT_LEGACY_BINARY64_ACCUMULATION_ORDER_SENSITIVE")
	}

	request := chunk105InstabilityRequest(t)
	artifactFor := func(advanceVI bool) []byte {
		t.Helper()
		compare := func(left, right InstabilityRun) (InstabilityPair, error) {
			variationInformation := variationInformationCanonical
			if advanceVI {
				variationInformation = func(a, b []Community) (float64, int) {
					value, denominator := variationInformationCanonical(a, b)
					return math.Nextafter(value, math.Inf(1)), denominator
				}
			}
			return comparePartitionsWithVI(left, right, variationInformation)
		}
		artifact, err := computeInstability(request, InstabilityVersionV2, InstabilityClaimCeilingV2, compare)
		if err != nil {
			t.Fatal(err)
		}
		return canonicalInstability(artifact)
	}

	canonical := artifactFor(false)
	oneULPAdvanced := artifactFor(true)
	if bytes.Equal(canonical, oneULPAdvanced) {
		t.Fatal("ASSERT_AUTHENTIC_COMPLETE_ARTIFACT_ONE_ULP_SENSITIVE")
	}
	t.Logf("RED_LEGACY_CONTROLLED_ONE_ULP_ARTIFACTS_UNEQUAL canonical_bytes=%d advanced_bytes=%d", len(canonical), len(oneULPAdvanced))
}

func TestComputeInstabilityV2FromGraphProvenanceV5(t *testing.T) {
	a, b := node("a", 0), node("b", 1)
	input := validV5(t, []graph.Node{a, b}, []graph.Edge{{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{}}}})
	artifact, err := ComputeInstabilityV2FromGraphProvenanceV5(input, []uint64{1, 2}, "gonum-v0.17.1", "sha256:"+strings.Repeat("c", 64), "sha256:"+strings.Repeat("d", 64))
	if err != nil {
		t.Fatal(err)
	}
	if artifact.SchemaVersion != InstabilityVersionV2 || artifact.ClaimCeiling != InstabilityClaimCeilingV2 || artifact.StructuralClaimCeiling != InstabilityStructuralCeiling {
		t.Fatalf("ASSERT_V2_V5_WRAPPER %+v", artifact)
	}
}
