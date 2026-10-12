package adr0011methodresult

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

// This RED fixes the producer write boundary without constructing a parallel
// target owner. The existing live restrictedTypedB4Input fixture remains the
// behavioral fixture; this source contract requires its production owner to use
// the accepted manager API and original candidate ordinal before provenance is
// made visible.
func TestRestrictedDefinitionB4C17TargetPublicationContract(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("BLOCKED_NOT_RED C17 producer source location")
	}
	production, err := os.ReadFile(strings.TrimSuffix(file, "c17_target_publication_red_private_test.go") + "restricted_definition_provenance_private.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(production)
	start := strings.Index(source, "func (m *restrictedOwnerManager) validateDefinitionProvenanceB4")
	end := strings.Index(source[start:], "\nfunc (a *restrictedAttempt) validatedHandle")
	if start < 0 || end < 0 {
		t.Fatal("BLOCKED_NOT_RED C17 producer function boundary")
	}
	body := source[start : start+end]

	for _, required := range []string{
		"CommitPrivateB4DefinitionBorrowedC17",
		"publish := func(commitBorrow sessionruntime.PrivateB4DefinitionBorrow) error",
		"commitBorrow.WithTargetAppendAdmission(uint64(candidate.Ordinal)",
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("ASSERT_C17_TARGET_PRODUCER_WIRING missing=%q", required)
		}
	}
	ordinalAdmission := strings.Index(body, "commitBorrow.WithTargetAppendAdmission(uint64(candidate.Ordinal)")
	candidateVisibility := strings.Index(body, "a.provenanceCandidateCount = uint16(prepared.candidateLen)")
	stateVisibility := strings.Index(body, "a.provenanceState = restrictedProvenanceValidated")
	if ordinalAdmission < 0 || candidateVisibility < ordinalAdmission || stateVisibility < candidateVisibility {
		t.Fatalf("ASSERT_C17_PROVENANCE_VISIBLE_ONLY_AFTER_TARGET_ADMISSION admission=%d candidates=%d state=%d", ordinalAdmission, candidateVisibility, stateVisibility)
	}
	for _, forbidden := range []string{
		"WithTargetAppendAdmission(uint64(i)",
		"a.provenanceState = restrictedProvenanceValidated\n\t\tfor i := 0; i < prepared.candidateLen",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("ASSERT_C17_ORIGINAL_ORDINAL_AND_DELAYED_VISIBILITY forbidden=%q", forbidden)
		}
	}
}

func TestRestrictedDefinitionB4C17ProductionBorrowPublishesCanonicalCandidate(t *testing.T) {
	_, m := newRestrictedRootManager(t, 0)
	p, err := m.reserveProcessing(1, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	in := restrictedTypedB4Input(t)
	r, err := m.reserveDefinitionProvenance(p, restrictedDefinitionLengths{Provenance: restrictedProvenanceRawBytes, Records: restrictedProvenanceRecordBytes})
	if err != nil {
		t.Fatal(err)
	}
	validated, err := m.validateDefinitionProvenanceB4(r, p, in)
	if err != nil {
		t.Fatalf("ASSERT_C17_PRODUCTION_BORROW_CANONICAL_APPEND err=%v", err)
	}
	a := &m.attempts[p.slot]
	if validated.candidateCount != 1 || a.provenanceState != restrictedProvenanceValidated || a.provenanceCandidateCount != 1 {
		t.Fatalf("ASSERT_C17_VALIDATED_CANDIDATE_VISIBLE handle=%+v state=%d collected=%d", validated, a.provenanceState, a.provenanceCandidateCount)
	}
	candidate := a.provenanceCandidates[0]
	if candidate.Ordinal != 0 || candidate.OccurrenceID == "" || candidate.QueryOccurrenceID == "" || candidate.TargetID == "" || candidate.TargetURI == "" {
		t.Fatalf("ASSERT_C17_CANONICAL_CANDIDATE_COLLECTED candidate=%+v", candidate)
	}
}

func TestRestrictedDefinitionB4C17PartialRefusalInvisibilityContract(t *testing.T) {
	t.Skip("BLOCKED_NOT_RED: production fixture exposes one target only; partial refusal after an earlier canonical candidate requires a production fixture, not a synthetic hook")
}
