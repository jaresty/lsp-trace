package programcadmission

import (
	"reflect"
	"testing"

	"lsp-trace/internal/graph"
)

func TestPreparePrivateLeidenProjectionDefinitionsAndReferencesExactSelectedEndpoints(t *testing.T) {
	for _, tc := range []struct {
		name            string
		kinds           []RelationKind
		wantOccurrences int
		wantNodes       int
	}{
		{"references", []RelationKind{TypedReferencesSymbol}, 3, 3},
		{"definitions", []RelationKind{TypedResolvesToDefinition}, 2, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := typedMethodLeidenInput(t, tc.kinds)
			got, err := PreparePrivateLeidenProjection(in)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Occurrences()) != tc.wantOccurrences || len(got.Nodes()) != tc.wantNodes {
				t.Fatalf("ASSERT_PRIVATE_LEIDEN_D_R_EXACT_SELECTED_ENDPOINTS nodes=%v occurrences=%v", got.Nodes(), got.Occurrences())
			}
			if !reflect.DeepEqual(got.SelectedKinds(), tc.kinds) {
				t.Fatalf("ASSERT_PRIVATE_LEIDEN_SELECTED_KIND got=%v want=%v", got.SelectedKinds(), tc.kinds)
			}
			for _, occurrence := range got.Occurrences() {
				if occurrence.Kind != tc.kinds[0] || occurrence.RelationID != "" {
					t.Fatalf("ASSERT_PRIVATE_LEIDEN_NO_FABRICATED_CALL_METADATA occurrence=%+v", occurrence)
				}
			}
			if !reflect.DeepEqual(got.CompositeSourceBinding(), CompositeSourceBinding{}) {
				t.Fatalf("ASSERT_PRIVATE_LEIDEN_METHODS_ZERO_COMPOSITE_SOURCE source=%+v", got.CompositeSourceBinding())
			}
			if got.MethodClaimCeiling() != MethodEvidenceClaimCeiling || got.MethodAuthority() != 0 || got.MethodAccepted() || got.MethodCompleteness() != "UNKNOWN" {
				t.Fatalf("ASSERT_PRIVATE_LEIDEN_METHOD_CLAIM_EXACT ceiling=%q authority=%d accepted=%t completeness=%q", got.MethodClaimCeiling(), got.MethodAuthority(), got.MethodAccepted(), got.MethodCompleteness())
			}
		})
	}
}

func TestPreparePrivateLeidenProjectionMixedRetainsExactSourceAndClaimCeiling(t *testing.T) {
	in := mixedRelationIndexInput(t, false)
	got, err := PreparePrivateLeidenProjection(in)
	if err != nil {
		t.Fatal(err)
	}
	const wantCeiling = "PRIVATE_PROGRAM_C_TYPED_RELATION_QUALIFICATION;STRUCTURAL_PARTITION_ONLY;AUTHORITY_0;ACCEPTED_FALSE;COMPLETENESS_UNKNOWN;FEATURE_IDENTITY_UNRESOLVED;NO_PRODUCER_AUTHENTICATION;NO_EXECUTION_CLAIM"
	if got.CombinedClaimCeiling() != wantCeiling {
		t.Fatalf("ASSERT_PRIVATE_LEIDEN_EXACT_CLAIM_CEILING got=%q", got.CombinedClaimCeiling())
	}
	if !reflect.DeepEqual(got.CompositeSourceBinding(), in.retainedCallsAdmission().SourceBinding()) {
		t.Fatalf("ASSERT_PRIVATE_LEIDEN_MIXED_FULL_COMPOSITE_SOURCE got=%+v want=%+v", got.CompositeSourceBinding(), in.retainedCallsAdmission().SourceBinding())
	}
	for _, forbidden := range []string{"AUTHORITY_1", "ACCEPTED_TRUE", "COMPLETENESS_COMPLETE", "FEATURE_IDENTITY_RESOLVED", "PRODUCER_AUTHENTICATED", "EXECUTION_CONFIRMED"} {
		if containsClaim(got.CombinedClaimCeiling(), forbidden) {
			t.Fatalf("ASSERT_PRIVATE_LEIDEN_FORBIDS_STRONGER_CLAIM ceiling=%q forbidden=%q", got.CombinedClaimCeiling(), forbidden)
		}
	}
}

func TestTypedInputAndBridgeRetainExactAdmittedCallsPrivately(t *testing.T) {
	in := mixedRelationIndexInput(t, false)
	want := in.retainedCallsAdmission()
	if !want.Valid() {
		t.Fatal("ASSERT_TYPED_INPUT_RETAINS_ADMITTED_CALLS")
	}
	bridge, err := PreparePrivateLeidenProjection(in)
	if err != nil {
		t.Fatal(err)
	}
	got := bridge.retainedCallsAdmission()
	if !reflect.DeepEqual(got.NodeIdentities(), want.NodeIdentities()) || !reflect.DeepEqual(got.Occurrences(), want.Occurrences()) || !reflect.DeepEqual(got.SourceBinding(), want.SourceBinding()) || got.ClaimCeiling() != want.ClaimCeiling() {
		t.Fatalf("ASSERT_BRIDGE_RETAINS_EXACT_ADMITTED_CALLS got=%+v want=%+v", got, want)
	}
}

func TestPreparePrivateLeidenProjectionRejectsResealedRetainedCallsCorruptionAtomically(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(*CompositeProjectionAdmission)
	}{
		{"duplicate occurrence identity and multiplicity", func(calls *CompositeProjectionAdmission) {
			calls.occurrences = append(calls.occurrences, calls.occurrences[0])
		}},
		{"noncanonical retained node IDs", func(calls *CompositeProjectionAdmission) {
			calls.nodeIDs[0], calls.nodeIDs[1] = calls.nodeIDs[1], calls.nodeIDs[0]
		}},
		{"duplicate retained node IDs", func(calls *CompositeProjectionAdmission) {
			calls.nodeIDs[1] = calls.nodeIDs[0]
		}},
		{"missing retained node ID", func(calls *CompositeProjectionAdmission) {
			calls.nodeIDs = calls.nodeIDs[:len(calls.nodeIDs)-1]
		}},
		{"extra retained node ID", func(calls *CompositeProjectionAdmission) {
			calls.nodeIDs = append(calls.nodeIDs, "zz-extra-retained-node")
		}},
		{"retained occurrence identity mismatch", func(calls *CompositeProjectionAdmission) {
			calls.occurrences[0].Identity += "-tampered"
		}},
		{"retained occurrence relation ID mismatch", func(calls *CompositeProjectionAdmission) {
			calls.occurrences[0].RelationID += "-tampered"
		}},
		{"retained occurrence provider mismatch", func(calls *CompositeProjectionAdmission) {
			calls.occurrences[0].Provider += "-tampered"
		}},
		{"retained occurrence provider version mismatch", func(calls *CompositeProjectionAdmission) {
			calls.occurrences[0].ProviderVersion += "-tampered"
		}},
		{"retained occurrence language mismatch", func(calls *CompositeProjectionAdmission) {
			calls.occurrences[0].Language += "-tampered"
		}},
		{"retained occurrence from mismatch", func(calls *CompositeProjectionAdmission) {
			calls.occurrences[0].From++
		}},
		{"retained occurrence to mismatch", func(calls *CompositeProjectionAdmission) {
			calls.occurrences[0].To++
		}},
		{"retained occurrence ordinal mismatch", func(calls *CompositeProjectionAdmission) {
			calls.occurrences[0].Ordinal++
		}},
		{"retained occurrence call site mismatch", func(calls *CompositeProjectionAdmission) {
			calls.occurrences[0].CallSite.End.Character++
		}},
		{"retained occurrence weight mismatch", func(calls *CompositeProjectionAdmission) {
			calls.occurrences[0].Weight++
		}},
		{"retained admission claim mismatch", func(calls *CompositeProjectionAdmission) {
			calls.claimCeiling += ";tampered"
		}},
		{"retained full source binding mismatch", func(calls *CompositeProjectionAdmission) {
			calls.source.CompositeOutputSHA256 += "-tampered"
		}},
		{"retained admission validity mismatch", func(calls *CompositeProjectionAdmission) {
			calls.valid = false
		}},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			in := mixedRelationIndexInput(t, false)
			if !in.integrity.callsAdmission.Valid() {
				t.Fatal("ASSERT_RETAINED_CALLS_BASELINE_VALID")
			}
			tc.mutate(&in.integrity.callsAdmission)
			in.integrity.seal = typedInputSeal(in)

			got, err := PreparePrivateLeidenProjection(in)
			if err == nil || err.Error() != "invalid private typed input integrity" {
				t.Fatalf("ASSERT_RESEALED_RETAINED_CALLS_EXACT_INTEGRITY_ERROR err=%v", err)
			}
			if !reflect.DeepEqual(got, privateLeidenProjection{}) {
				t.Fatalf("ASSERT_RESEALED_RETAINED_CALLS_ATOMIC_ZERO projection=%+v", got)
			}
		})
	}
}

func TestPreparePrivateLeidenProjectionRejectsCoordinatedRetainedCallsTamperAfterReseal(t *testing.T) {
	in := mixedRelationIndexInput(t, false)
	calls := &in.integrity.callsAdmission
	renamed := make(map[string]string, len(calls.nodeIDs))
	for i, id := range calls.nodeIDs {
		renamed[id] = "tampered-" + id
		calls.nodeIDs[i] = renamed[id]
	}
	for i := range in.Occurrences {
		in.Occurrences[i].From = renamed[in.Occurrences[i].From]
		in.Occurrences[i].To = renamed[in.Occurrences[i].To]
	}
	for i, id := range in.integrity.nodes {
		in.integrity.nodes[i] = renamed[id]
	}
	weights := make(map[TypedPair]int, len(in.PairWeights))
	for pair, weight := range in.PairWeights {
		pair.From = renamed[pair.From]
		pair.To = renamed[pair.To]
		weights[pair] = weight
	}
	in.PairWeights = weights
	in.integrity.seal = typedInputSeal(in)

	got, err := PreparePrivateLeidenProjection(in)
	if err == nil || err.Error() != "invalid private typed input integrity" {
		t.Fatalf("ASSERT_COORDINATED_RESEALED_CALLS_TAMPER_EXACT_INTEGRITY_ERROR err=%v", err)
	}
	if !reflect.DeepEqual(got, privateLeidenProjection{}) {
		t.Fatalf("ASSERT_COORDINATED_RESEALED_CALLS_TAMPER_ATOMIC_ZERO projection=%+v", got)
	}
}

func TestPreparePrivateLeidenProjectionRejectsNestedCompositeSourceTamperAfterReseal(t *testing.T) {
	in := mixedRelationIndexInput(t, false)
	if len(in.integrity.callsAdmission.source.Constituents) == 0 || len(in.integrity.callsAdmission.source.Constituents[0].Summary) == 0 {
		t.Fatal("ASSERT_NESTED_COMPOSITE_SOURCE_FIXTURE")
	}
	in.integrity.callsAdmission.source.Constituents[0].Summary = append([]byte(nil), in.integrity.callsAdmission.source.Constituents[0].Summary...)
	in.integrity.callsAdmission.source.Constituents[0].Summary[0] ^= 1
	in.integrity.seal = typedInputSeal(in)

	got, err := PreparePrivateLeidenProjection(in)
	if err == nil || err.Error() != "invalid private typed input integrity" {
		t.Fatalf("ASSERT_NESTED_COMPOSITE_SOURCE_RESEALED_EXACT_INTEGRITY_ERROR err=%v", err)
	}
	if !reflect.DeepEqual(got, privateLeidenProjection{}) {
		t.Fatalf("ASSERT_NESTED_COMPOSITE_SOURCE_RESEALED_ATOMIC_ZERO projection=%+v", got)
	}
}

func TestPreparePrivateLeidenProjectionRecomputesAllSealedInvariantsAndFailsZero(t *testing.T) {
	base := mixedRelationIndexInput(t, false)
	mutations := []struct {
		name   string
		mutate func(*TypedInput)
	}{
		{"duplicate identity", func(in *TypedInput) { in.Occurrences[1].Identity = in.Occurrences[0].Identity }},
		{"noncanonical nodes", func(in *TypedInput) {
			in.integrity.nodes[0], in.integrity.nodes[1] = in.integrity.nodes[1], in.integrity.nodes[0]
		}},
		{"pair map", func(in *TypedInput) {
			for p := range in.PairWeights {
				in.PairWeights[p]++
				break
			}
		}},
		{"zero weight", func(in *TypedInput) {
			for p := range in.PairWeights {
				in.PairWeights[p] = 0
				break
			}
		}},
		{"negative weight", func(in *TypedInput) {
			for p := range in.PairWeights {
				in.PairWeights[p] = -1
				break
			}
		}},
		{"endpoint membership", func(in *TypedInput) { in.Occurrences[0].To = "not-selected" }},
		{"selected kind", func(in *TypedInput) { in.Occurrences[0].Kind = RelationKind("UNKNOWN") }},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			in := cloneTypedInputForTamper(base)
			tc.mutate(&in)
			in.integrity.seal = typedInputSeal(in)
			got, err := PreparePrivateLeidenProjection(in)
			if err == nil {
				t.Fatal("ASSERT_PRIVATE_LEIDEN_RECOMPUTED_VALIDATION_REJECTED")
			}
			if !reflect.DeepEqual(got, privateLeidenProjection{}) {
				t.Fatalf("ASSERT_PRIVATE_LEIDEN_FAILURE_ZERO_OPAQUE_PROJECTION projection=%+v", got)
			}
		})
	}
}

func TestPrivateLeidenProjectionConstructionDoesNotMutateTypedInput(t *testing.T) {
	in := mixedRelationIndexInput(t, false)
	before := cloneTypedInputForTamper(in)
	got, err := PreparePrivateLeidenProjection(in)
	if err != nil {
		t.Fatal(err)
	}
	_ = got.Nodes()
	_ = got.Occurrences()
	_ = got.PairWeights()
	_ = got.SelectedKinds()
	_ = got.CompositeSourceBinding()
	if !reflect.DeepEqual(in, before) {
		t.Fatalf("ASSERT_PRIVATE_LEIDEN_CONSTRUCTION_INPUT_IMMUTABLE before=%+v after=%+v", before, in)
	}
}

func TestTypedLeidenFixtureDigestsRemainCallerAssertedOnly(t *testing.T) {
	fixture := capture(t, "typed-custody", []graph.Node{node("typed-custody-node")}, nil)
	if fixture.ExactMetadata.RevisionCustody != "CALLER_ASSERTED" {
		t.Fatalf("ASSERT_TYPED_FIXTURE_DIGEST_CALLER_ASSERTED_ONLY custody=%q", fixture.ExactMetadata.RevisionCustody)
	}
}

func typedMethodLeidenInput(t *testing.T, kinds []RelationKind) TypedInput {
	t.Helper()
	a, b, c := node("leiden-method-a"), node("leiden-method-b"), node("leiden-method-isolate")
	envelope := validUntrustedMethodEvidenceEnvelope()
	envelope.Occurrences = []MethodOccurrenceEvidence{
		methodOccurrence("leiden-reference-1", TypedReferencesSymbol, 1, a.ID, b.ID),
		methodOccurrence("leiden-definition-0", TypedResolvesToDefinition, 0, a.ID, b.ID),
		methodOccurrence("leiden-reference-self", TypedReferencesSymbol, 2, c.ID, c.ID),
		methodOccurrence("leiden-reference-0", TypedReferencesSymbol, 0, a.ID, b.ID),
		methodOccurrence("leiden-definition-1", TypedResolvesToDefinition, 1, a.ID, b.ID),
	}
	admitted, err := AdmitUntrustedMethodEvidenceEnvelope(envelope)
	if err != nil {
		t.Fatal(err)
	}
	in, err := PrepareTypedInput(TypedInputRequest{Methods: admitted, EnableMethods: true, Kinds: kinds})
	if err != nil {
		t.Fatal(err)
	}
	return in
}

func cloneTypedInputForTamper(in TypedInput) TypedInput {
	out := in
	out.Occurrences = append([]TypedOccurrence(nil), in.Occurrences...)
	out.PairWeights = make(map[TypedPair]int, len(in.PairWeights))
	for pair, weight := range in.PairWeights {
		out.PairWeights[pair] = weight
	}
	out.integrity.nodes = append([]string(nil), in.integrity.nodes...)
	out.integrity.selectedKinds = append([]RelationKind(nil), in.integrity.selectedKinds...)
	return out
}

func containsClaim(claim, forbidden string) bool {
	for i := 0; i+len(forbidden) <= len(claim); i++ {
		if claim[i:i+len(forbidden)] == forbidden {
			return true
		}
	}
	return false
}
