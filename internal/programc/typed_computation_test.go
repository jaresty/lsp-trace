package programc

import (
	"fmt"
	"reflect"
	"testing"

	"lsp-trace/internal/programcadmission"
)

func TestComputeTypedCarrierCallsEqualsHistoricalForTwoSeeds(t *testing.T) {
	fixture := typedComputeFixture(t, false)
	for _, seed := range []uint64{19, 23} {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			want, failure := ComputeComposite(fixture.calls.Admission, seed)
			if failure != nil {
				t.Fatal(failure)
			}
			got, failure := ComputeTyped(fixture.callsOnly, seed)
			if failure != nil {
				t.Fatal(failure)
			}
			if !reflect.DeepEqual(got.Outcome(), want) {
				t.Fatalf("ASSERT_TYPED_COMPUTATION_CALLS_FULL_HISTORICAL_EQUAL seed=%d got=%#v want=%#v", seed, got.Outcome(), want)
			}
			if !reflect.DeepEqual(got.Occurrences(), fixture.callsOnly.Occurrences) || !reflect.DeepEqual(got.SelectedKinds(), []programcadmission.RelationKind{programcadmission.TypedCalls}) || len(got.MethodEvidence()) != 0 || len(got.MethodCommitment()) != 0 {
				t.Fatalf("ASSERT_TYPED_COMPUTATION_CALLS_EXACT_SOURCE gotOccurrences=%#v gotKinds=%#v gotMethods=%#v commitment=%x", got.Occurrences(), got.SelectedKinds(), got.MethodEvidence(), got.MethodCommitment())
			}
		})
	}
}

func TestComputeTypedCarrierParallelMethodsRetainOneToOneEvidenceAndPairMultiplicity(t *testing.T) {
	fixture := typedComputeFixture(t, false)
	computed, failure := ComputeTyped(fixture.referencesOnly, 31)
	if failure != nil {
		t.Fatal(failure)
	}
	occurrences, methods := computed.Occurrences(), computed.MethodEvidence()
	if len(occurrences) != len(methods) || len(methods) < 3 {
		t.Fatalf("ASSERT_TYPED_CARRIER_PARALLEL_ONE_TO_ONE_COUNTS occurrences=%d methods=%d", len(occurrences), len(methods))
	}
	byIdentity := make(map[string]programcadmission.MethodOccurrenceEvidence, len(methods))
	for _, method := range methods {
		byIdentity[method.Identity] = method
	}
	for _, occurrence := range occurrences {
		method, ok := byIdentity[occurrence.Identity]
		if !ok || method.Kind != occurrence.Kind || method.From != occurrence.From || method.To != occurrence.To || method.Ordinal != occurrence.Ordinal {
			t.Fatalf("ASSERT_TYPED_CARRIER_PARALLEL_EXACT_MAPPING occurrence=%+v method=%+v ok=%t", occurrence, method, ok)
		}
	}
	for pair, want := range fixture.referencesOnly.PairWeights {
		if got := computed.PairWeights()[pair]; got != want {
			t.Fatalf("ASSERT_TYPED_CARRIER_PARALLEL_PAIR_MULTIPLICITY pair=%+v got=%d want=%d", pair, got, want)
		}
	}
	assertTypedCarrierDefensiveCopies(t, computed)
}

func TestRecomputeTypedUsesRetainedSourceAndSeedInsteadOfCachedOutcome(t *testing.T) {
	fixture := typedComputeFixture(t, false)
	computed, failure := ComputeTyped(fixture.mixed, 41)
	if failure != nil {
		t.Fatal(failure)
	}
	want, failure := computeTypedProjection(mustPrepareTypedProjection(t, fixture.mixed), mixed, 41)
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
	base, failure := ComputeTyped(fixture.mixed, 43)
	if failure != nil {
		t.Fatal(failure)
	}
	cases := []struct {
		name   string
		mutate func(*TypedComputation)
	}{
		{"method commitment", func(in *TypedComputation) { in.methodCommitment[0] ^= 0xff }},
		{"method evidence", func(in *TypedComputation) { in.methodEvidence[0].EvidenceDigest = "sha256:" + repeatCharacter("f", 64) }},
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

func TestTypedComputationOutcomeAndSourceAccessorsAreDeepDefensiveCopies(t *testing.T) {
	fixture := typedComputeFixture(t, false)
	computed, failure := ComputeTyped(fixture.mixed, 47)
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
	methods[0].Identity = "mutated-method-evidence"
	kinds[0] = "MUTATED"
	for pair := range pairs {
		pairs[pair]++
		break
	}
	commitment[0] ^= 0xff

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
