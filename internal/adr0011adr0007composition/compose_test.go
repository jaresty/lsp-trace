package adr0011adr0007composition

import (
	"bytes"
	"context"
	"reflect"
	"testing"

	adr0007 "lsp-trace/internal/adr0007sourcetextsearchv4private"
)

func TestWrapperBaselineNoOptionalInputExactADR0007Forwarding(t *testing.T) {
	attempt := wrapperAttempt()
	raw := adr0007.Canon(attempt)
	direct := adr0007.Evaluate(attempt, raw)
	got, err := Compose(context.Background(), Request{Attempt: attempt, RawAttempt: raw})
	if err != nil || got.Failure != nil {
		t.Fatalf("ASSERT_WRAPPER_BASELINE_NO_FAILURE err=%v failure=%+v", err, got.Failure)
	}
	if !bytes.Equal(got.ADR0007CanonicalBytes, adr0007.Canon(direct)) || got.ADR0007CanonicalSHA256 != adr0007.Digest(adr0007.Canon(direct)) {
		t.Fatalf("ASSERT_WRAPPER_BASELINE_EXACT_FORWARDING digest=%s direct=%s", got.ADR0007CanonicalSHA256, adr0007.Digest(adr0007.Canon(direct)))
	}
	if got.CompositionCandidate != nil || len(got.DirectionDispositions) != 0 {
		t.Fatalf("ASSERT_WRAPPER_BASELINE_NO_PRIVATE_CANDIDATE got=%+v dispositions=%+v", got.CompositionCandidate, got.DirectionDispositions)
	}
}

func TestWrapperPinnedEvidenceChangesOnlySeparateCompositionCandidate(t *testing.T) {
	base := wrapperBaseline(t)
	ids := base.ADR0007Terminal.RangeUnionCandidate.MemberMatchIDs
	got := wrapperCompose(t, []DirectionEnvelope{AcceptedSuccessEnvelope("incoming"), AcceptedSuccessEnvelope("outgoing")}, []DependencyMark{
		incomingMark(ids[1], ActionOmit, 0),
		outgoingMark(ids[2], ActionOrder, 0),
	})
	if got.CompositionCandidate == nil {
		t.Fatalf("ASSERT_WRAPPER_PINNED_EVIDENCE_CANDIDATE_PRESENT failure=%+v", got.Failure)
	}
	if !reflect.DeepEqual(got.CompositionCandidate.MemberMatchIDs, []string{ids[2], ids[0]}) {
		t.Fatalf("ASSERT_WRAPPER_PINNED_EVIDENCE_MEMBERSHIP_ORDER got=%v", got.CompositionCandidate.MemberMatchIDs)
	}
	if got.ADR0007Terminal.RangeUnionCandidate.CandidateDigest != base.ADR0007Terminal.RangeUnionCandidate.CandidateDigest || !reflect.DeepEqual(got.ADR0007Terminal.RangeUnionCandidate.MemberMatchIDs, ids) {
		t.Fatalf("ASSERT_WRAPPER_DOES_NOT_MUTATE_ADR0007_CANDIDATE")
	}
	if got.CompositionCandidate.CandidateDigest == base.ADR0007Terminal.RangeUnionCandidate.CandidateDigest {
		t.Fatalf("ASSERT_WRAPPER_SEPARATE_CANDIDATE_DIGEST")
	}
}

func TestWrapperIncomingOutgoingRemovalIsolation(t *testing.T) {
	ids := wrapperBaseline(t).ADR0007Terminal.RangeUnionCandidate.MemberMatchIDs
	both := wrapperCompose(t, successPair(), []DependencyMark{incomingMark(ids[1], ActionOmit, 0), outgoingMark(ids[2], ActionOrder, 0)})
	incomingRemoved := wrapperCompose(t, successPair(), []DependencyMark{outgoingMark(ids[2], ActionOrder, 0)})
	outgoingRemoved := wrapperCompose(t, successPair(), []DependencyMark{incomingMark(ids[1], ActionOmit, 0)})
	if !reflect.DeepEqual(both.CompositionCandidate.MemberMatchIDs, []string{ids[2], ids[0]}) {
		t.Fatalf("ASSERT_WRAPPER_BOTH_DEPENDENT_FACTS got=%v", both.CompositionCandidate.MemberMatchIDs)
	}
	if !reflect.DeepEqual(incomingRemoved.CompositionCandidate.MemberMatchIDs, []string{ids[2], ids[0], ids[1]}) {
		t.Fatalf("ASSERT_WRAPPER_REMOVE_INCOMING_ONLY_INCOMING_DEPENDENT got=%v", incomingRemoved.CompositionCandidate.MemberMatchIDs)
	}
	if !reflect.DeepEqual(outgoingRemoved.CompositionCandidate.MemberMatchIDs, []string{ids[0], ids[2]}) {
		t.Fatalf("ASSERT_WRAPPER_REMOVE_OUTGOING_ONLY_OUTGOING_DEPENDENT got=%v", outgoingRemoved.CompositionCandidate.MemberMatchIDs)
	}
}

func TestWrapperDirectionFailureNonPositivePreservesPeer(t *testing.T) {
	ids := wrapperBaseline(t).ADR0007Terminal.RangeUnionCandidate.MemberMatchIDs
	got, err := Compose(context.Background(), Request{Attempt: wrapperAttempt(), RawAttempt: adr0007.Canon(wrapperAttempt()), Composition: &CompositionInput{Envelopes: []DirectionEnvelope{AcceptedFailureEnvelope("incoming"), AcceptedSuccessEnvelope("outgoing")}, Marks: []DependencyMark{outgoingMark(ids[2], ActionOrder, 0)}}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Failure == nil || got.Failure.Code != "FAILURE_WITH_POSITIVE_MARKS" || got.CompositionCandidate != nil {
		t.Fatalf("ASSERT_WRAPPER_FAILURE_WITH_MARKS_NON_POSITIVE failure=%+v candidate=%+v", got.Failure, got.CompositionCandidate)
	}
	if len(got.DirectionDispositions) != 2 || got.DirectionDispositions[0].Direction != "incoming" || got.DirectionDispositions[0].Status != StatusFailure || got.DirectionDispositions[1].Direction != "outgoing" || got.DirectionDispositions[1].Status != StatusSuccess {
		t.Fatalf("ASSERT_WRAPPER_FAILURE_PRESERVES_PEER_PROVENANCE dispositions=%+v", got.DirectionDispositions)
	}
	cleanFailure, err := Compose(context.Background(), Request{Attempt: wrapperAttempt(), RawAttempt: adr0007.Canon(wrapperAttempt()), Composition: &CompositionInput{Envelopes: []DirectionEnvelope{AcceptedFailureEnvelope("incoming"), AcceptedSuccessEnvelope("outgoing")}}})
	if err != nil || cleanFailure.Failure == nil || cleanFailure.Failure.Code != "DIRECTION_NON_POSITIVE" || cleanFailure.CompositionCandidate != nil {
		t.Fatalf("ASSERT_WRAPPER_DIRECTION_FAILURE_NO_EMPTY_SUCCESS result=%+v err=%v", cleanFailure, err)
	}
}

func TestWrapperUnorderedMarksStableAndProviderOrdinalsPreserved(t *testing.T) {
	ids := wrapperBaseline(t).ADR0007Terminal.RangeUnionCandidate.MemberMatchIDs
	left := wrapperCompose(t, successPair(), []DependencyMark{incomingMark(ids[1], ActionOmit, 0), outgoingMark(ids[2], ActionOrder, 0)})
	right := wrapperCompose(t, []DirectionEnvelope{AcceptedSuccessEnvelope("outgoing"), AcceptedSuccessEnvelope("incoming")}, []DependencyMark{outgoingMark(ids[2], ActionOrder, 0), incomingMark(ids[1], ActionOmit, 0)})
	if !bytes.Equal(adr0007.Canon(*left.CompositionCandidate), adr0007.Canon(*right.CompositionCandidate)) || left.CompositionCandidate.CandidateDigest != right.CompositionCandidate.CandidateDigest {
		t.Fatalf("ASSERT_WRAPPER_UNORDERED_REORDER_STABLE")
	}
	if left.CompositionCandidate.AppliedMarks[0].OrdinalIdentitySHA256 != IncomingOrdinalIdentitySHA256 || left.CompositionCandidate.AppliedMarks[1].OrdinalIdentitySHA256 != OutgoingOrdinalIdentitySHA256 {
		t.Fatalf("ASSERT_WRAPPER_CANONICAL_ORDINAL_IDENTITY_PRESERVED marks=%+v", left.CompositionCandidate.AppliedMarks)
	}
}

func TestWrapperPinTamperAndIdentityEscalationRefused(t *testing.T) {
	ids := wrapperBaseline(t).ADR0007Terminal.RangeUnionCandidate.MemberMatchIDs
	cases := map[string]func([]DirectionEnvelope, []DependencyMark){
		"composition-package": func(e []DirectionEnvelope, _ []DependencyMark) {
			e[0].CompositionPackageChecksumSHA256 = adr0007.Digest([]byte("fake"))
		},
		"artifact": func(e []DirectionEnvelope, _ []DependencyMark) {
			e[0].CompositionArtifactSHA256 = adr0007.Digest([]byte("fake"))
		},
		"profile":  func(_ []DirectionEnvelope, m []DependencyMark) { m[0].ProfileID = OutgoingProfileID },
		"evidence": func(_ []DirectionEnvelope, m []DependencyMark) { m[0].EvidenceSHA256 = OutgoingEvidenceSHA256 },
		"self-consistent-selected-ordinal": func(e []DirectionEnvelope, m []DependencyMark) {
			e[0].SelectedOrdinals = []uint64{8}
			m[0].SelectedOrdinal = 8
		},
		"self-consistent-ordinal-identity": func(e []DirectionEnvelope, m []DependencyMark) {
			e[0].OrdinalIdentitySHA256 = OutgoingOrdinalIdentitySHA256
			m[0].OrdinalIdentitySHA256 = OutgoingOrdinalIdentitySHA256
		},
		"mark-ordinal-identity": func(_ []DirectionEnvelope, m []DependencyMark) {
			m[0].OrdinalIdentitySHA256 = OutgoingOrdinalIdentitySHA256
		},
		"dr-retype":     func(_ []DirectionEnvelope, m []DependencyMark) { m[0].RelationKind = "REFERENCES_SYMBOL" },
		"merge-action":  func(_ []DirectionEnvelope, m []DependencyMark) { m[0].Action = "MERGE" },
		"unknown-match": func(_ []DirectionEnvelope, m []DependencyMark) { m[0].MatchID = adr0007.Digest([]byte("synthetic")) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			env := successPair()
			marks := []DependencyMark{incomingMark(ids[1], ActionOmit, 0)}
			mutate(env, marks)
			got, err := Compose(context.Background(), Request{Attempt: wrapperAttempt(), RawAttempt: adr0007.Canon(wrapperAttempt()), Composition: &CompositionInput{Envelopes: env, Marks: marks}})
			if err != nil {
				t.Fatal(err)
			}
			if got.Failure == nil || got.CompositionCandidate != nil {
				t.Fatalf("ASSERT_WRAPPER_TAMPER_PRE_EFFECT_REFUSAL case=%s result=%+v", name, got)
			}
		})
	}
	clean := wrapperCompose(t, successPair(), []DependencyMark{incomingMark(ids[1], ActionOmit, 0)})
	if clean.CompositionCandidate.Authority != 0 || clean.CompositionCandidate.Accepted || clean.CompositionCandidate.Completeness != "UNKNOWN" || clean.CompositionCandidate.FeatureIdentity != "UNRESOLVED" {
		t.Fatalf("ASSERT_WRAPPER_NO_IDENTITY_ACCEPTANCE_INFERENCE candidate=%+v", clean.CompositionCandidate)
	}
}

func TestWrapperBoundsAndCancellation(t *testing.T) {
	ids := wrapperBaseline(t).ADR0007Terminal.RangeUnionCandidate.MemberMatchIDs
	tooMany := successPair()
	tooMany = append(tooMany, AcceptedSuccessEnvelope("incoming"))
	got, err := Compose(context.Background(), Request{Attempt: wrapperAttempt(), RawAttempt: adr0007.Canon(wrapperAttempt()), Composition: &CompositionInput{Envelopes: tooMany, Marks: []DependencyMark{incomingMark(ids[1], ActionOmit, 0)}}})
	if err != nil || got.Failure == nil || got.Failure.Code != "DIRECTION_ENVELOPES_REQUIRED" {
		t.Fatalf("ASSERT_WRAPPER_ENVELOPE_BOUND_REFUSAL result=%+v err=%v", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cancelled, err := Compose(ctx, Request{Attempt: wrapperAttempt(), RawAttempt: adr0007.Canon(wrapperAttempt()), Composition: &CompositionInput{Envelopes: successPair(), Marks: []DependencyMark{incomingMark(ids[1], ActionOmit, 0)}}})
	if err == nil || cancelled.Failure == nil || cancelled.Failure.Code != "CANCELLED" || cancelled.ADR0007CanonicalBytes != nil {
		t.Fatalf("ASSERT_WRAPPER_PRE_CANCEL_NO_ADR0007 result=%+v err=%v", cancelled, err)
	}
	maxRefsPass := wrapperComposeWithLimits(t, successPair(), []DependencyMark{incomingMark(ids[1], ActionOmit, 0), outgoingMark(ids[2], ActionOrder, 0)}, Limits{MaxReferences: 4})
	if maxRefsPass.Failure != nil || maxRefsPass.Accounting.References != 4 {
		t.Fatalf("ASSERT_WRAPPER_MAX_REFERENCES_N_PASS result=%+v", maxRefsPass)
	}
	maxRefsFail := wrapperComposeWithLimits(t, successPair(), []DependencyMark{incomingMark(ids[1], ActionOmit, 0), outgoingMark(ids[2], ActionOrder, 0)}, Limits{MaxReferences: 3})
	if maxRefsFail.Failure == nil || maxRefsFail.Failure.Code != "RESOURCE_EXHAUSTED" || maxRefsFail.Failure.Stage != "references" || maxRefsFail.CompositionCandidate != nil || maxRefsFail.Accounting.References != 3 {
		t.Fatalf("ASSERT_WRAPPER_MAX_REFERENCES_N_PLUS_1_PRE_EFFECT result=%+v", maxRefsFail)
	}
	lowWork := wrapperComposeWithLimits(t, successPair(), []DependencyMark{incomingMark(ids[1], ActionOmit, 0), outgoingMark(ids[2], ActionOrder, 0)}, Limits{MaxWork: 1})
	if lowWork.Failure == nil || lowWork.Failure.Code != "RESOURCE_EXHAUSTED" || lowWork.CompositionCandidate != nil {
		t.Fatalf("ASSERT_WRAPPER_WORK_BOUND_PRE_EFFECT result=%+v", lowWork)
	}
}

func TestWrapperRepeatedIdenticalByteStable(t *testing.T) {
	ids := wrapperBaseline(t).ADR0007Terminal.RangeUnionCandidate.MemberMatchIDs
	left := wrapperCompose(t, successPair(), []DependencyMark{incomingMark(ids[1], ActionOmit, 0), outgoingMark(ids[2], ActionOrder, 0)})
	right := wrapperCompose(t, successPair(), []DependencyMark{incomingMark(ids[1], ActionOmit, 0), outgoingMark(ids[2], ActionOrder, 0)})
	if !bytes.Equal(adr0007.Canon(*left.CompositionCandidate), adr0007.Canon(*right.CompositionCandidate)) || left.CompositionCandidate.CandidateDigest != right.CompositionCandidate.CandidateDigest {
		t.Fatalf("ASSERT_WRAPPER_IDEMPOTENT_BYTE_STABLE")
	}
}

func wrapperBaseline(t *testing.T) Result {
	t.Helper()
	got, err := Compose(context.Background(), Request{Attempt: wrapperAttempt(), RawAttempt: adr0007.Canon(wrapperAttempt())})
	if err != nil || got.Failure != nil || got.ADR0007Terminal.Terminal != "COMPLETE" || got.ADR0007Terminal.RangeUnionCandidate == nil || len(got.ADR0007Terminal.RangeUnionCandidate.MemberMatchIDs) != 3 {
		t.Fatalf("ASSERT_WRAPPER_BASELINE_FIXTURE result=%+v err=%v", got, err)
	}
	return got
}

func wrapperCompose(t *testing.T, env []DirectionEnvelope, marks []DependencyMark) Result {
	t.Helper()
	return wrapperComposeWithLimits(t, env, marks, Limits{})
}

func wrapperComposeWithLimits(t *testing.T, env []DirectionEnvelope, marks []DependencyMark, limits Limits) Result {
	t.Helper()
	got, err := Compose(context.Background(), Request{Attempt: wrapperAttempt(), RawAttempt: adr0007.Canon(wrapperAttempt()), Composition: &CompositionInput{Envelopes: env, Marks: marks}, Limits: limits})
	if err != nil {
		t.Fatalf("ASSERT_WRAPPER_COMPOSE_NO_CONTEXT_ERROR err=%v result=%+v", err, got)
	}
	if limits.MaxWork != 1 && limits.MaxReferences != 3 && got.Failure != nil {
		t.Fatalf("ASSERT_WRAPPER_COMPOSE_POSITIVE failure=%+v result=%+v", got.Failure, got)
	}
	return got
}

func successPair() []DirectionEnvelope {
	return []DirectionEnvelope{AcceptedSuccessEnvelope("incoming"), AcceptedSuccessEnvelope("outgoing")}
}

func incomingMark(id, action string, candidateOrdinal uint64) DependencyMark {
	return DependencyMark{Direction: "incoming", MatchID: id, Action: action, CandidateOrdinal: candidateOrdinal, OneFinalDispositionOrdinals: CanonicalOneFinalDispositionOrdinals, OrdinalIdentitySHA256: IncomingOrdinalIdentitySHA256, RelationKind: "CALLS", ProfileID: IncomingProfileID, EvidenceSHA256: IncomingEvidenceSHA256}
}

func outgoingMark(id, action string, candidateOrdinal uint64) DependencyMark {
	return DependencyMark{Direction: "outgoing", MatchID: id, Action: action, CandidateOrdinal: candidateOrdinal, OneFinalDispositionOrdinals: CanonicalOneFinalDispositionOrdinals, OrdinalIdentitySHA256: OutgoingOrdinalIdentitySHA256, RelationKind: "CALLS", ProfileID: OutgoingProfileID, EvidenceSHA256: OutgoingEvidenceSHA256}
}

func wrapperAttempt() adr0007.Attempt {
	content := []byte("needle alpha needle beta needle\n")
	fileDigest := adr0007.Digest(content)
	return adr0007.Attempt{
		SchemaVersion: adr0007.AttemptSchema,
		AttemptID:     "attempt-adr0011-wrapper-composition",
		Request: adr0007.Request{
			SchemaVersion: adr0007.RequestSchema,
			Query:         "needle",
			Sources:       []adr0007.SourceRef{{Ordinal: 1, Path: "tmp/adr0011-wrapper.go", Revision: "rev", FileDigest: fileDigest, ObjectDigest: fileDigest}},
			Policy:        adr0007.Policy{SchemaVersion: adr0007.PolicySchema, LiteralMode: "byte-literal"},
			Limits:        adr0007.Limits{SchemaVersion: adr0007.LimitsSchema, MaxFiles: 1, MaxMatches: 10, MaxWork: 1_000_000, MaxOutputBytes: 100_000, MaxSourceBytes: 1000, MaxTotalBytes: 1000, MaxPathBytes: 1000},
			LocationPin:   adr0007.LocationPin{SchemaVersion: adr0007.LocationPinSchema, Operation: "RANGE_UNION", ExecutedLocation: false, DesignCommit: "c943a484060462121c6f0929d4182b053ff95ab5", DesignRootSha256: "sha256:1195a420cc2ae215ff1627dbf23b606caaa243aa9fc0ae234242acceddafb48d", ExecutionCommit: "f0f8b49aa368bea9b3e6d105eef5cb2614221067", SealCommit: "16f40dcb03a234b00db80059a7eef400495e9d97", FinalSealSha256: "sha256:f4981045d3489f5ef0633eb4ce6b4a17ab6de1ddc73c4b729f9dd524106b1fd6", SourceAdmissionPin: adr0007.SourceAdmissionPin{RepositoryCommit: "af2ce89321afc94c937636f841bb98b8977b6496", Path: "internal/sourceadmissionv2/admission.go", Bytes: 3519, SHA256: "sha256:da74770d5b36f63e6f1265ba78e2404e13f2d1f1451a4e7aa405f448e47fe7da", GitBlobSHA1: "4953fab89d911e2352fa6c2253497c07c8e9a777", Symbols: []string{"Admit", "CanonicalPath", "Digest"}}, CompleteLocationPins: []string{"design_commit", "design_root_sha256", "execution_commit", "seal_commit", "final_seal_sha256", "source_admission_pin"}},
		},
		ExecutionControl: adr0007.ExecutionControl{SchemaVersion: adr0007.ExecutionControlSchema},
		SourceInputs:     []adr0007.SourceInput{{SchemaVersion: adr0007.SourceInputSchema, Ordinal: 1, Path: "tmp/adr0011-wrapper.go", Revision: "rev", FileDigest: fileDigest, ObjectDigest: fileDigest, BytesBase64: "bmVlZGxlIGFscGhhIG5lZWRsZSBiZXRhIG5lZWRsZQo="}},
	}
}
