package adr0011adr0007compositionruntime

import (
	"reflect"
	"testing"

	composition "lsp-trace/internal/adr0011adr0007composition"
)

func TestCoordinateResultPrivateCompleteMutationIsolation(t *testing.T) {
	disposition := composition.DirectionDisposition{Direction: "incoming", SelectedOrdinals: []uint64{7}}
	candidate := &composition.CompositionCandidate{BaselineMatchIDs: []string{"base"}, MemberMatchIDs: []string{"member"}, DirectionDispositions: []composition.DirectionDisposition{disposition}, AppliedMarks: []composition.AppliedMark{{MatchID: "mark"}}}
	composed := composition.Result{ADR0007CanonicalBytes: []byte("canonical"), DirectionDispositions: []composition.DirectionDisposition{disposition}, CompositionCandidate: candidate, Failure: &composition.Failure{Code: "INNER"}}
	result := newCoordinateResultPrivate(coordinateResultSnapshotPrivate{Composition: composed, Candidate: candidate, Refusal: &composition.Failure{Code: "REFUSED"}, Suppression: &CandidateSuppressionPrivate{Code: "SUPPRESSED"}, Facets: []FacetDispositionPrivate{{Facet: "D", State: "OBSERVED_EMPTY"}}, Accepted: true})

	compositionCopy := result.CompositionPrivate()
	compositionCopy.ADR0007CanonicalBytes[0] = 'X'
	compositionCopy.DirectionDispositions[0].SelectedOrdinals[0] = 99
	compositionCopy.CompositionCandidate.MemberMatchIDs[0] = "mutated"
	compositionCopy.Failure.Code = "MUTATED"
	candidateCopy := result.CandidatePrivate()
	candidateCopy.BaselineMatchIDs[0] = "mutated"
	candidateCopy.DirectionDispositions[0].SelectedOrdinals[0] = 99
	candidateCopy.AppliedMarks[0].MatchID = "mutated"
	refusal := result.RefusalPrivate()
	refusal.Code = "MUTATED"
	suppression := result.SuppressionPrivate()
	suppression.Code = "MUTATED"
	facets := result.FacetsPrivate()
	facets[0].Facet = "MUTATED"

	again := result.CompositionPrivate()
	if string(again.ADR0007CanonicalBytes) != "canonical" || again.DirectionDispositions[0].SelectedOrdinals[0] != 7 || again.CompositionCandidate.MemberMatchIDs[0] != "member" || again.Failure.Code != "INNER" {
		t.Fatal("ASSERT_COORDINATE_COMPOSITION_DEEP_IMMUTABLE")
	}
	if result.CandidatePrivate().BaselineMatchIDs[0] != "base" || result.CandidatePrivate().DirectionDispositions[0].SelectedOrdinals[0] != 7 || result.CandidatePrivate().AppliedMarks[0].MatchID != "mark" || result.RefusalPrivate().Code != "REFUSED" || result.SuppressionPrivate().Code != "SUPPRESSED" || result.FacetsPrivate()[0].Facet != "D" {
		t.Fatal("ASSERT_COORDINATE_NESTED_RESULTS_DEEP_IMMUTABLE")
	}

	typeOf := reflect.TypeOf(result)
	for i := 0; i < typeOf.NumField(); i++ {
		if typeOf.Field(i).PkgPath == "" {
			t.Fatalf("ASSERT_COORDINATE_NO_EXPORTED_STATE: %s", typeOf.Field(i).Name)
		}
	}
}
