package programcadmission

import (
	"bytes"
	"reflect"
	"runtime/trace"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/programccompose"
)

// These representation guards deliberately inspect private slices, not the
// accessors: accessor normalization is precisely the regression under test.
func TestEmptyAdmissionCloneNodeRepresentation(t *testing.T) {
	for _, tc := range []struct {
		name string
		ids  []string
	}{
		{"nil", nil},
		{"empty", make([]string, 0)},
		{"empty-with-capacity", make([]string, 0, 2)},
		{"nonempty", []string{"a", "b"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := CompositeProjectionAdmission{nodeIDs: tc.ids}
			got := cloneCompositeProjectionAdmission(in)
			if (got.nodeIDs == nil) != (in.nodeIDs == nil) || !reflect.DeepEqual(got.nodeIDs, in.nodeIDs) {
				t.Fatalf("ASSERT_EMPTY_CLONE_NODES FAIL gotNil=%t wantNil=%t got=%v want=%v", got.nodeIDs == nil, in.nodeIDs == nil, got.nodeIDs, in.nodeIDs)
			}
			t.Log("ASSERT_EMPTY_CLONE_NODES PASS")
		})
	}
}

func TestEmptyAdmissionCloneOccurrenceRepresentation(t *testing.T) {
	for _, tc := range []struct {
		name string
		occ  []Occurrence
	}{
		{"nil", nil},
		{"empty", make([]Occurrence, 0)},
		{"empty-with-capacity", make([]Occurrence, 0, 2)},
		{"nonempty", []Occurrence{{Identity: "first", Ordinal: 1}, {Identity: "second", Ordinal: 0}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := CompositeProjectionAdmission{occurrences: tc.occ}
			got := cloneCompositeProjectionAdmission(in)
			if (got.occurrences == nil) != (in.occurrences == nil) || !reflect.DeepEqual(got.occurrences, in.occurrences) {
				t.Fatalf("ASSERT_EMPTY_CLONE_OCCURRENCES FAIL gotNil=%t wantNil=%t got=%v want=%v", got.occurrences == nil, in.occurrences == nil, got.occurrences, in.occurrences)
			}
			t.Log("ASSERT_EMPTY_CLONE_OCCURRENCES PASS")
		})
	}
}

// Synthetic helper-only values; these do not fabricate a usable admission or
// enter the typed boundary. Independently constructed snapshots detect aliasing.
func TestEmptyAdmissionCloneDefensiveIndependence(t *testing.T) {
	makeInput := func() CompositeProjectionAdmission {
		return CompositeProjectionAdmission{
			nodeIDs: []string{"a", "b"}, occurrences: []Occurrence{{Identity: "first", Weight: 1}, {Identity: "second", Ordinal: 1}},
			claimCeiling: "pure-claim", canonicalBytes: []byte("canonical"),
			source: CompositeSourceBinding{CompositeID: "pure-source", Constituents: []ConstituentReference{{Identity: "pure-constituent"}}},
		}
	}
	for _, tc := range []struct {
		name   string
		mutate func(*CompositeProjectionAdmission)
	}{
		{"nodes", func(a *CompositeProjectionAdmission) { a.nodeIDs[0] = "changed" }},
		{"occurrences", func(a *CompositeProjectionAdmission) { a.occurrences[0].Identity = "changed" }},
		{"canonical", func(a *CompositeProjectionAdmission) { a.canonicalBytes[0] ^= 1 }},
		{"source", func(a *CompositeProjectionAdmission) { a.source.Constituents[0].Identity = "changed" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in, want := makeInput(), makeInput()
			got := cloneCompositeProjectionAdmission(in)
			tc.mutate(&got)
			if !reflect.DeepEqual(in, want) {
				t.Fatal("ASSERT_EMPTY_CLONE_INDEPENDENCE FAIL original mutated")
			}
			t.Log("ASSERT_EMPTY_CLONE_INDEPENDENCE PASS")
		})
	}
}

// capture/node are the existing, inspected in-memory fixture constructors in
// relation_index_test.go. CaptureV5 serializes/validates supplied DTO bytes; no
// native capture, provider, process, grant, job, worker or trace is started.
func TestEmptyAdmissionCloneZeroCallsTypedBoundary(t *testing.T) {
	if trace.IsEnabled() {
		t.Fatal("SETUP_EMPTY_CLONE_TRACE_ALREADY_ENABLED")
	}
	isolate := node("empty-clone-isolate")
	composite, err := programccompose.Compose([]programccompose.Input{
		capture(t, "empty-clone-a", []graph.Node{isolate}, nil),
		capture(t, "empty-clone-b", []graph.Node{isolate}, nil),
	})
	if err != nil {
		t.Fatalf("SETUP_EMPTY_CLONE_COMPOSE: %v", err)
	}
	admitted, err := Admit(composite.Bytes)
	if err != nil {
		t.Fatalf("SETUP_EMPTY_CLONE_ADMIT: %v", err)
	}
	if !admitted.Admission.Valid() || len(admitted.Admission.nodeIDs) != 1 || len(admitted.Admission.occurrences) != 0 || admitted.Admission.occurrences == nil {
		t.Fatal("SETUP_EMPTY_CLONE_EXPECT_ADMITTED_NONNIL_EMPTY_CALLS")
	}
	original, err := Admit(composite.Bytes)
	if err != nil {
		t.Fatalf("SETUP_EMPTY_CLONE_READMIT: %v", err)
	}
	in, err := PrepareTypedInput(TypedInputRequest{Calls: admitted.Admission, Kinds: []RelationKind{TypedCalls}})
	if err != nil {
		t.Fatalf("SETUP_EMPTY_CLONE_TYPED: %v", err)
	}
	seal := in.integrity.seal
	projection, err := PreparePrivateLeidenProjection(in)
	if err != nil {
		t.Fatalf("ASSERT_EMPTY_CLONE_TYPED_BOUNDARY FAIL projection: %v", err)
	}
	if !reflect.DeepEqual(in.integrity.callsAdmission, original.Admission) || !reflect.DeepEqual(projection.calls, original.Admission) || !reflect.DeepEqual(admitted.Admission, original.Admission) || !bytes.Equal(projection.RetainedCallsCanonicalBytes(), composite.Bytes) || !reflect.DeepEqual(projection.CompositeSourceBinding(), original.Admission.SourceBinding()) || in.integrity.seal != seal || typedInputSeal(in) != seal {
		t.Fatal("ASSERT_EMPTY_CLONE_TYPED_BOUNDARY FAIL canonical/source/admission/seal drift")
	}
	if len(in.Occurrences) != 0 || len(projection.Occurrences()) != 0 || len(projection.Nodes()) != 1 || !reflect.DeepEqual(projection.SelectedKinds(), []RelationKind{TypedCalls}) || projection.CombinedClaimCeiling() != original.Admission.ClaimCeiling() || in.Authority != 0 || in.SourceGraphComplete != "UNKNOWN" {
		t.Fatal("ASSERT_EMPTY_CLONE_TYPED_BOUNDARY FAIL selection/ceiling drift")
	}
	projection.calls.canonicalBytes[0] ^= 1
	projection.calls.nodeIDs[0] = "changed"
	projection.calls.source.Constituents[0].Identity = "changed"
	if !reflect.DeepEqual(in.integrity.callsAdmission, original.Admission) || !reflect.DeepEqual(admitted.Admission, original.Admission) || in.integrity.seal != seal || typedInputSeal(in) != seal {
		t.Fatal("ASSERT_EMPTY_CLONE_TYPED_BOUNDARY FAIL input mutated through projection")
	}
	tampered := in
	tampered.integrity.callsAdmission = cloneCompositeProjectionAdmission(in.integrity.callsAdmission)
	tampered.integrity.callsAdmission.canonicalBytes[0] ^= 1
	if _, err := PreparePrivateLeidenProjection(tampered); err == nil {
		t.Fatal("ASSERT_EMPTY_CLONE_TYPED_BOUNDARY FAIL tampered canonical accepted")
	}
	if _, err := PrepareTypedInput(TypedInputRequest{Calls: admitted.Admission}); err == nil {
		t.Fatal("ASSERT_EMPTY_CLONE_TYPED_BOUNDARY FAIL empty selection accepted")
	}
	if trace.IsEnabled() {
		t.Fatal("ASSERT_EMPTY_CLONE_TYPED_BOUNDARY FAIL trace enabled")
	}
	t.Log("ASSERT_EMPTY_CLONE_TYPED_BOUNDARY PASS")
}
