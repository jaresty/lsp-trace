package programc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"testing"

	"lsp-trace/internal/programcadmission"
)

func TestComputeTypedCarrierCallsEqualsHistoricalBytesForTwoSeedsWithoutMutation(t *testing.T) {
	fixture := typedComputeFixture(t, false)
	sourceBytes := append([]byte(nil), fixture.calls.Bytes...)
	for _, seed := range []uint64{19, 23} {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			before, failure := ComputeComposite(fixture.calls.Admission, seed)
			if failure != nil {
				t.Fatal(failure)
			}
			beforeBytes := marshalExactOutcomeJSON(t, before)

			got, failure := ComputeTyped(fixture.callsOnly, seed)
			if failure != nil {
				t.Fatal(failure)
			}
			gotBytes := marshalExactOutcomeJSON(t, got.Outcome())
			after, failure := ComputeComposite(fixture.calls.Admission, seed)
			if failure != nil {
				t.Fatal(failure)
			}
			afterBytes := marshalExactOutcomeJSON(t, after)
			if !bytes.Equal(gotBytes, beforeBytes) || !bytes.Equal(afterBytes, beforeBytes) {
				t.Fatalf("ASSERT_TYPED_COMPUTATION_CALLS_EXACT_HISTORICAL_BYTES seed=%d got=%s before=%s after=%s", seed, gotBytes, beforeBytes, afterBytes)
			}
			if !bytes.Equal(fixture.calls.Bytes, sourceBytes) {
				t.Fatal("ASSERT_TYPED_COMPUTATION_CALLS_SOURCE_BYTES_UNCHANGED")
			}
			if !reflect.DeepEqual(got.Occurrences(), fixture.callsOnly.Occurrences) || !reflect.DeepEqual(got.SelectedKinds(), []programcadmission.RelationKind{programcadmission.TypedCalls}) || len(got.MethodEvidence()) != 0 || len(got.MethodCommitment()) != 0 {
				t.Fatalf("ASSERT_TYPED_COMPUTATION_CALLS_EXACT_SOURCE gotOccurrences=%#v gotKinds=%#v gotMethods=%#v commitment=%x", got.Occurrences(), got.SelectedKinds(), got.MethodEvidence(), got.MethodCommitment())
			}
		})
	}
}

func marshalExactOutcomeJSON(t *testing.T, outcome Outcome) []byte {
	t.Helper()
	type pairWeight struct {
		Pair   Pair
		Weight float64
	}
	pairWeights := make([]pairWeight, 0, len(outcome.Projection.PairWeights))
	for pair, weight := range outcome.Projection.PairWeights {
		pairWeights = append(pairWeights, pairWeight{Pair: pair, Weight: weight})
	}
	sort.Slice(pairWeights, func(i, j int) bool {
		if pairWeights[i].Pair.From != pairWeights[j].Pair.From {
			return pairWeights[i].Pair.From < pairWeights[j].Pair.From
		}
		return pairWeights[i].Pair.To < pairWeights[j].Pair.To
	})
	type projectionJSON struct {
		NodeIdentities  []string
		NodeIDs         map[string]int64
		Occurrences     []Occurrence
		PairWeights     []pairWeight
		Source          SourceBinding
		CompositeSource programcadmission.CompositeSourceBinding
	}
	type outcomeJSON struct {
		Outcome, ProfileID, ProfileDigest, Algorithm, LogicalDigest, ClaimCeiling string
		Resolution                                                                float64
		Seed                                                                      uint64
		Communities                                                               []Community
		Projection                                                                projectionJSON
		Source                                                                    SourceBinding
		CompositeSource                                                           programcadmission.CompositeSourceBinding
	}
	raw, err := json.Marshal(outcomeJSON{
		Outcome:         outcome.Outcome,
		ProfileID:       outcome.ProfileID,
		ProfileDigest:   outcome.ProfileDigest,
		Algorithm:       outcome.Algorithm,
		LogicalDigest:   outcome.LogicalDigest,
		ClaimCeiling:    outcome.ClaimCeiling,
		Resolution:      outcome.Resolution,
		Seed:            outcome.Seed,
		Communities:     outcome.Communities,
		Projection:      projectionJSON{outcome.Projection.NodeIdentities, outcome.Projection.NodeIDs, outcome.Projection.Occurrences, pairWeights, outcome.Projection.Source, outcome.Projection.CompositeSource},
		Source:          outcome.Source,
		CompositeSource: outcome.CompositeSource,
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestComputeTypedTamperedMethodsAndMixedFailIntegrityBeforeGroupingPolicy(t *testing.T) {
	fixture := typedComputeFixture(t, false)
	methodsTampered := cloneTypedInputPublic(fixture.referencesOnly)
	methodsTampered.Occurrences[0].Identity = "tampered-public-occurrence"
	mixedTampered := cloneTypedInputPublic(fixture.mixed)
	for pair := range mixedTampered.PairWeights {
		mixedTampered.PairWeights[pair]++
		break
	}
	for _, tc := range []struct {
		name  string
		input programcadmission.TypedInput
	}{
		{name: "methods-only-public-occurrence", input: methodsTampered},
		{name: "mixed-public-pair", input: mixedTampered},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, failure := ComputeTyped(tc.input, 31)
			if failure == nil || failure.Code != codeTypedInputIntegrity || failure.Message != "TYPED_INPUT_INTEGRITY" || !reflect.DeepEqual(got, TypedComputation{}) {
				t.Fatalf("ASSERT_TYPED_INPUT_INTEGRITY_PRECEDES_GROUPING_POLICY got=%#v failure=%+v", got, failure)
			}
		})
	}
}

func TestComputeTypedGroupingPolicyGatePrecedesKernelAndCallsDelegatesOnce(t *testing.T) {
	fixture := typedComputeFixture(t, false)
	for _, tc := range []struct {
		name  string
		input programcadmission.TypedInput
	}{
		{name: "methods-only", input: fixture.referencesOnly},
		{name: "mixed", input: fixture.mixed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			got, failure := computeTypedWithKernel(tc.input, 29, func(typedProjectionInput, typedSelection, uint64) (Outcome, *Failure) {
				calls++
				panic("typed kernel invoked before grouping policy qualification")
			})
			if calls != 0 || failure == nil || failure.Code != FailureCode("GROUPING_POLICY_UNQUALIFIED") || failure.Message != "grouping policy unqualified" || !reflect.DeepEqual(got, TypedComputation{}) {
				t.Fatalf("ASSERT_GROUPING_POLICY_GATE_PRECEDES_TYPED_KERNEL calls=%d got=%#v failure=%+v", calls, got, failure)
			}
		})
	}

	calls := 0
	got, failure := computeTypedWithKernel(fixture.callsOnly, 29, func(in typedProjectionInput, selection typedSelection, seed uint64) (Outcome, *Failure) {
		calls++
		return computeTypedProjection(in, selection, seed)
	})
	want, wantFailure := computeTypedProjection(mustPrepareTypedProjection(t, fixture.callsOnly), callsOnly, 29)
	if calls != 1 || failure != nil || wantFailure != nil || !reflect.DeepEqual(got.Outcome(), want) {
		t.Fatalf("ASSERT_TYPED_CALLS_DELEGATES_KERNEL_EXACTLY_ONCE calls=%d gotFailure=%+v wantFailure=%+v", calls, failure, wantFailure)
	}
}

func TestComputeTypedValidMethodsAndMixedFailGroupingPolicyAtomically(t *testing.T) {
	fixture := typedComputeFixture(t, false)
	for _, tc := range []struct {
		name  string
		input programcadmission.TypedInput
	}{
		{name: "references-only", input: fixture.referencesOnly},
		{name: "definitions-only", input: fixture.definitionsOnly},
		{name: "mixed", input: fixture.mixed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, failure := ComputeTyped(tc.input, 31)
			if failure == nil || failure.Code != FailureCode("GROUPING_POLICY_UNQUALIFIED") || failure.Message != "grouping policy unqualified" || !reflect.DeepEqual(got, TypedComputation{}) {
				t.Fatalf("ASSERT_TYPED_GROUPING_POLICY_UNQUALIFIED_ATOMIC_ZERO got=%#v failure=%+v", got, failure)
			}
		})
	}
}

func TestRecomputeTypedUsesRetainedCallsSourceAndSeedInsteadOfCachedOutcome(t *testing.T) {
	fixture := typedComputeFixture(t, false)
	computed, failure := ComputeTyped(fixture.callsOnly, 41)
	if failure != nil {
		t.Fatal(failure)
	}
	want, failure := computeTypedProjection(mustPrepareTypedProjection(t, fixture.callsOnly), callsOnly, 41)
	if failure != nil {
		t.Fatal(failure)
	}

	computed.cachedOutcome.Outcome = "tampered-cached-outcome"
	computed.cachedOutcome.Seed++
	computed.cachedOutcome.Communities = []Community{{Members: []string{"tampered-member"}}}
	computed.cachedOutcome.Projection.NodeIdentities = []string{"tampered-node"}
	computed.cachedOutcome.Projection.NodeIDs = map[string]int64{"tampered-node": 99}
	computed.cachedOutcome.Projection.Occurrences = []Occurrence{{Identity: "tampered-occurrence"}}
	computed.cachedOutcome.Projection.PairWeights = map[Pair]float64{{From: 99, To: 99}: 99}

	recomputed, failure := RecomputeTyped(computed)
	if failure != nil {
		t.Fatal(failure)
	}
	if !reflect.DeepEqual(recomputed.Outcome(), want) || recomputed.Outcome().Outcome == "tampered-cached-outcome" {
		t.Fatalf("ASSERT_TYPED_RECOMPUTE_FROM_RETAINED_SOURCE_AND_SEED got=%#v want=%#v", recomputed.Outcome(), want)
	}
}

func TestTypedComputationIndependentTamperingHasExactIntegrityFailureAndAtomicZero(t *testing.T) {
	fixture := typedComputeFixture(t, false)
	base, failure := ComputeTyped(fixture.callsOnly, 43)
	if failure != nil {
		t.Fatal(failure)
	}
	cases := []struct {
		name   string
		mutate func(*TypedComputation)
	}{
		{"seed", func(in *TypedComputation) { in.seed++ }},
		{"kinds", func(in *TypedComputation) { in.selectedKinds[0] = programcadmission.TypedReferencesSymbol }},
		{"occurrences", func(in *TypedComputation) { in.occurrences[0].Identity = "tampered-occurrence" }},
		{"pair weights", func(in *TypedComputation) {
			for pair := range in.pairWeights {
				in.pairWeights[pair]++
				return
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mutated := cloneTypedComputationForTamper(base)
			tc.mutate(&mutated)
			got, failure := RecomputeTyped(mutated)
			if failure == nil || failure.Code != codeTypedInputIntegrity || failure.Message != "TYPED_INPUT_INTEGRITY" || !reflect.DeepEqual(got, TypedComputation{}) {
				t.Fatalf("ASSERT_TYPED_CARRIER_TAMPER_EXACT_FAILURE_AND_ZERO got=%#v failure=%+v", got, failure)
			}
		})
	}

	got, failure := RecomputeTyped(TypedComputation{})
	if failure == nil || failure.Code != codeTypedInputIntegrity || failure.Message != "TYPED_INPUT_INTEGRITY" || !reflect.DeepEqual(got, TypedComputation{}) {
		t.Fatalf("ASSERT_ZERO_TYPED_CARRIER_EXACT_FAILURE_AND_ZERO got=%#v failure=%+v", got, failure)
	}
}

func TestRecomputeTypedValidMethodsAndMixedFailGroupingPolicyAtomically(t *testing.T) {
	fixture := typedComputeFixture(t, false)
	for _, tc := range []struct {
		name  string
		input programcadmission.TypedInput
	}{
		{name: "references-only", input: fixture.referencesOnly},
		{name: "definitions-only", input: fixture.definitionsOnly},
		{name: "mixed", input: fixture.mixed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, failure := RecomputeTyped(internallyFormedTypedCarrier(t, tc.input, 47))
			if failure == nil || failure.Code != FailureCode("GROUPING_POLICY_UNQUALIFIED") || failure.Message != "grouping policy unqualified" || !reflect.DeepEqual(got, TypedComputation{}) {
				t.Fatalf("ASSERT_TYPED_RECOMPUTE_GROUPING_POLICY_UNQUALIFIED_ATOMIC_ZERO got=%#v failure=%+v", got, failure)
			}
		})
	}
}

func internallyFormedTypedCarrier(t *testing.T, input programcadmission.TypedInput, seed uint64) TypedComputation {
	t.Helper()
	projection, err := programcadmission.PreparePrivateLeidenProjection(input)
	if err != nil {
		t.Fatal(err)
	}
	computed := TypedComputation{
		source:           cloneTypedInputPublic(input),
		seed:             seed,
		occurrences:      projection.Occurrences(),
		methodEvidence:   projection.MethodEvidence(),
		selectedKinds:    projection.SelectedKinds(),
		pairWeights:      projection.PairWeights(),
		methodCommitment: projection.MethodCommitment(),
	}
	computed.seal = typedComputationSeal(computed)
	return computed
}

func TestTypedComputationOutcomeAndSourceAccessorsAreDeepDefensiveCopies(t *testing.T) {
	fixture := typedComputeFixture(t, false)
	computed, failure := ComputeTyped(fixture.callsOnly, 47)
	if failure != nil {
		t.Fatal(failure)
	}
	assertTypedCarrierDefensiveCopies(t, computed)
}

func assertTypedCarrierDefensiveCopies(t *testing.T, in TypedComputation) {
	t.Helper()
	wantOutcome, wantOccurrences := in.Outcome(), in.Occurrences()
	wantMethods, wantKinds, wantPairs := in.MethodEvidence(), in.SelectedKinds(), in.PairWeights()
	wantCommitment := in.MethodCommitment()

	outcome := in.Outcome()
	outcome.Communities[0].Members[0] = "mutated-member"
	outcome.Projection.NodeIdentities[0] = "mutated-node-identity"
	for identity := range outcome.Projection.NodeIDs {
		outcome.Projection.NodeIDs[identity]++
		break
	}
	outcome.Projection.Occurrences[0].Identity = "mutated-projection-occurrence"
	for pair := range outcome.Projection.PairWeights {
		outcome.Projection.PairWeights[pair]++
		break
	}
	mutateCompositeSourceBinding(&outcome.CompositeSource)
	mutateCompositeSourceBinding(&outcome.Projection.CompositeSource)

	occurrences, methods := in.Occurrences(), in.MethodEvidence()
	kinds, pairs, commitment := in.SelectedKinds(), in.PairWeights(), in.MethodCommitment()
	occurrences[0].Identity = "mutated-retained-occurrence"
	if len(methods) != 0 {
		methods[0].Identity = "mutated-method-evidence"
	}
	kinds[0] = "MUTATED"
	for pair := range pairs {
		pairs[pair]++
		break
	}
	if len(commitment) != 0 {
		commitment[0] ^= 0xff
	}

	if !reflect.DeepEqual(in.Outcome(), wantOutcome) || !reflect.DeepEqual(in.Occurrences(), wantOccurrences) || !reflect.DeepEqual(in.MethodEvidence(), wantMethods) || !reflect.DeepEqual(in.SelectedKinds(), wantKinds) || !reflect.DeepEqual(in.PairWeights(), wantPairs) || !reflect.DeepEqual(in.MethodCommitment(), wantCommitment) {
		t.Fatal("ASSERT_TYPED_COMPUTATION_ALL_ACCESSORS_DEEP_DEFENSIVE")
	}
}

func mutateCompositeSourceBinding(source *programcadmission.CompositeSourceBinding) {
	if len(source.Constituents) != 0 {
		source.Constituents[0].Identity = "mutated-constituent"
	}
	if len(source.Completeness.PerInput) != 0 {
		source.Completeness.PerInput[0][0] ^= 0xff
	}
}

func cloneTypedComputationForTamper(in TypedComputation) TypedComputation {
	out := in
	out.cachedOutcome = CloneOutcome(in.cachedOutcome)
	out.occurrences = append([]programcadmission.TypedOccurrence(nil), in.occurrences...)
	out.methodEvidence = append([]programcadmission.MethodOccurrenceEvidence(nil), in.methodEvidence...)
	out.selectedKinds = append([]programcadmission.RelationKind(nil), in.selectedKinds...)
	out.pairWeights = make(map[programcadmission.TypedPair]int, len(in.pairWeights))
	for pair, weight := range in.pairWeights {
		out.pairWeights[pair] = weight
	}
	out.methodCommitment = append([]byte(nil), in.methodCommitment...)
	return out
}

func mustPrepareTypedProjection(t *testing.T, in programcadmission.TypedInput) typedProjectionInput {
	t.Helper()
	projection, err := programcadmission.PreparePrivateLeidenProjection(in)
	if err != nil {
		t.Fatal(err)
	}
	return projection
}

func repeatCharacter(value string, count int) string {
	out := ""
	for len(out) < count {
		out += value
	}
	return out[:count]
}
