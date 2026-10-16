package adr0011adr0007compositionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	adr0007 "lsp-trace/internal/adr0007sourcetextsearchv4private"
	composition "lsp-trace/internal/adr0011adr0007composition"
	"lsp-trace/internal/directionalraw"
)

func validAttemptPrivate() adr0007.Attempt {
	content := []byte("needle alpha needle beta needle\n")
	digest := adr0007.Digest(content)
	return adr0007.Attempt{SchemaVersion: adr0007.AttemptSchema, AttemptID: "attempt-noncanonical-raw-custody", Request: adr0007.Request{SchemaVersion: adr0007.RequestSchema, Query: "needle", Sources: []adr0007.SourceRef{{Ordinal: 1, Path: "tmp/adr0011-coordinator.go", Revision: "rev", FileDigest: digest, ObjectDigest: digest}}, Policy: adr0007.Policy{SchemaVersion: adr0007.PolicySchema, LiteralMode: "byte-literal"}, Limits: adr0007.Limits{SchemaVersion: adr0007.LimitsSchema, MaxFiles: 1, MaxMatches: 10, MaxWork: 1_000_000, MaxOutputBytes: 100_000, MaxSourceBytes: 1000, MaxTotalBytes: 1000, MaxPathBytes: 1000}, LocationPin: adr0007.LocationPin{SchemaVersion: adr0007.LocationPinSchema, Operation: "RANGE_UNION", ExecutedLocation: false, DesignCommit: "c943a484060462121c6f0929d4182b053ff95ab5", DesignRootSha256: "sha256:1195a420cc2ae215ff1627dbf23b606caaa243aa9fc0ae234242acceddafb48d", ExecutionCommit: "f0f8b49aa368bea9b3e6d105eef5cb2614221067", SealCommit: "16f40dcb03a234b00db80059a7eef400495e9d97", FinalSealSha256: "sha256:f4981045d3489f5ef0633eb4ce6b4a17ab6de1ddc73c4b729f9dd524106b1fd6", SourceAdmissionPin: adr0007.SourceAdmissionPin{RepositoryCommit: "af2ce89321afc94c937636f841bb98b8977b6496", Path: "internal/sourceadmissionv2/admission.go", Bytes: 3519, SHA256: "sha256:da74770d5b36f63e6f1265ba78e2404e13f2d1f1451a4e7aa405f448e47fe7da", GitBlobSHA1: "4953fab89d911e2352fa6c2253497c07c8e9a777", Symbols: []string{"Admit", "CanonicalPath", "Digest"}}, CompleteLocationPins: []string{"design_commit", "design_root_sha256", "execution_commit", "seal_commit", "final_seal_sha256", "source_admission_pin"}}}, ExecutionControl: adr0007.ExecutionControl{SchemaVersion: adr0007.ExecutionControlSchema}, SourceInputs: []adr0007.SourceInput{{SchemaVersion: adr0007.SourceInputSchema, Ordinal: 1, Path: "tmp/adr0011-coordinator.go", Revision: "rev", FileDigest: digest, ObjectDigest: digest, BytesBase64: "bmVlZGxlIGFscGhhIG5lZWRsZSBiZXRhIG5lZWRsZQo="}}}
}

func noncanonicalRawAttemptsPrivate(t *testing.T, attempt adr0007.Attempt) [][]byte {
	t.Helper()
	structOrder, err := json.Marshal(attempt)
	if err != nil {
		t.Fatal(err)
	}
	whitespaceVariant := append([]byte(" \n\t"), structOrder...)
	whitespaceVariant = append(whitespaceVariant, '\n')
	escapeVariant := bytes.Replace(whitespaceVariant, []byte(`"query":"needle"`), []byte(`"query":"n\u0065edle"`), 1)
	if bytes.Equal(whitespaceVariant, escapeVariant) || bytes.Equal(bytes.TrimSpace(whitespaceVariant), bytes.TrimSpace(adr0007.Canon(attempt))) {
		t.Fatal("ASSERT_NONCANONICAL_VARIANTS_HAVE_DISTINCT_BYTES")
	}
	return [][]byte{whitespaceVariant, escapeVariant}
}

func validCoordinateRequestPrivate(t *testing.T, attempt adr0007.Attempt, raw []byte) CoordinateRequestPrivate {
	t.Helper()
	terminal := adr0007.EvaluateRaw(raw)
	if terminal.Failure != nil || len(terminal.RangeUnionCandidate.MemberMatchIDs) < 2 {
		t.Fatalf("ASSERT_VALID_NONCANONICAL_ATTEMPT_EVALUATES: %+v", terminal)
	}
	ids := terminal.RangeUnionCandidate.MemberMatchIDs
	root, err := directionalraw.NewBidirectionalRootPrivate(directionalraw.NewReservation("coordinator-valid-noncanonical"), directionalraw.BidirectionalPlanPrivate{IncomingBytes: 32 << 20, OutgoingBytes: 32 << 20})
	if err != nil {
		t.Fatal(err)
	}
	marks := []composition.DependencyMark{{Direction: "incoming", MatchID: ids[0], Action: composition.ActionOrder, CandidateOrdinal: 0, OneFinalDispositionOrdinals: composition.CanonicalOneFinalDispositionOrdinals, OrdinalIdentitySHA256: composition.IncomingOrdinalIdentitySHA256, RelationKind: "CALLS", ProfileID: composition.IncomingProfileID, EvidenceSHA256: composition.IncomingEvidenceSHA256}, {Direction: "outgoing", MatchID: ids[1], Action: composition.ActionOrder, CandidateOrdinal: 1, OneFinalDispositionOrdinals: composition.CanonicalOneFinalDispositionOrdinals, OrdinalIdentitySHA256: composition.OutgoingOrdinalIdentitySHA256, RelationKind: "CALLS", ProfileID: composition.OutgoingProfileID, EvidenceSHA256: composition.OutgoingEvidenceSHA256}}
	facets := []FacetObservationPrivate{{Facet: "D", ObservationState: "OBSERVED_NONEMPTY"}, {Facet: "R", ObservationState: "OBSERVED_EMPTY"}, {Facet: "CALLS_INCOMING", ObservationState: "OBSERVED_NONEMPTY"}, {Facet: "CALLS_OUTGOING", ObservationState: "OBSERVED_NONEMPTY"}}
	return CoordinateRequestPrivate{Attempt: attempt, RawAttempt: raw, Envelopes: []composition.DirectionEnvelope{composition.AcceptedSuccessEnvelope("incoming"), composition.AcceptedSuccessEnvelope("outgoing")}, Marks: marks, Facets: facets, Root: root}
}

func TestCoordinatorProductionAPIAndRawCustody(t *testing.T) {
	coordinator, err := NewCoordinatorPrivate()
	if errors.Is(err, ErrBehaviorNotImplementedPrivate) {
		t.Fatal("ASSERT_SKELETON_ERR_BEHAVIOR_NOT_IMPLEMENTED_COORDINATOR")
	}
	if err != nil {
		t.Fatal(err)
	}
	attempt := validAttemptPrivate()
	digests := map[string]struct{}{}
	for variantIndex, raw := range noncanonicalRawAttemptsPrivate(t, attempt) {
		parsed, parseErr := adr0007.StrictParseAttempt(raw)
		if parseErr != "" || !bytes.Equal(adr0007.Canon(parsed), adr0007.Canon(attempt)) {
			t.Fatalf("ASSERT_VALID_NONCANONICAL_TYPED_RAW_EQUAL[%d]: parsed=%+v err=%v", variantIndex, parsed, parseErr)
		}
		request := validCoordinateRequestPrivate(t, attempt, raw)
		got, err := coordinator.CoordinatePrivate(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		if got.RefusalPrivate() != nil || !got.AcceptedPrivate() || got.CandidatePrivate() == nil || got.CandidatePrivate().Authority != 0 || got.CandidatePrivate().Accepted {
			t.Fatalf("ASSERT_COORDINATOR_ACCEPTED_REAL_COMPOSE_CANDIDATE[%d]: %+v", variantIndex, got)
		}
		expectedRawDigest := adr0007.Digest(raw)
		if got.RawAttemptSHA256Private() != expectedRawDigest || got.ParsedAttemptSHA256Private() == "" || got.TerminalRawSHA256Private() != expectedRawDigest {
			t.Fatalf("ASSERT_RAW_ATTEMPT_BYTE_DIGEST_PRESERVED[%d]: want=%s raw=%s parsed=%s terminal=%s", variantIndex, expectedRawDigest, got.RawAttemptSHA256Private(), got.ParsedAttemptSHA256Private(), got.TerminalRawSHA256Private())
		}
		digests[got.RawAttemptSHA256Private()] = struct{}{}
		if got.CandidatePrivate().CandidateDigest == "" || got.CandidatePrivate().AdmissionDigest == "" || len(got.CandidatePrivate().AppliedMarks) != 2 {
			t.Fatalf("ASSERT_SEALED_FIELDS_NOT_INPUT_ECHO_OR_CONSTANT[%d]: %+v", variantIndex, got.CandidatePrivate())
		}
		if got.CandidatePrivate().Completeness != "UNKNOWN" || got.CandidatePrivate().FeatureIdentity != "UNRESOLVED" || !got.LeaseStatePrivate().Released || got.LeaseStatePrivate().RootReleaseCount != 1 {
			t.Fatalf("ASSERT_UNKNOWN_UNRESOLVED_RELEASE_ONCE[%d]: candidate=%+v lease=%+v", variantIndex, got.CandidatePrivate(), got.LeaseStatePrivate())
		}
	}
	if len(digests) != 2 {
		t.Fatalf("ASSERT_NONCANONICAL_RAW_VARIANT_DIGESTS_DISTINCT: %+v", digests)
	}
}

func TestCoordinatorInvalidFacetReleasesLiveRootPrivate(t *testing.T) {
	reservation := directionalraw.NewReservation("invalid-facet-release")
	terminalCalls := 0
	if !reservation.AttachPrivate(func(string, string, int) ([]byte, error) { return nil, nil }, func(directionalraw.Observation) {}, func() bool {
		terminalCalls++
		return true
	}, func() {}) {
		t.Fatal("ASSERT_INVALID_FACET_ATTACH")
	}
	root, err := directionalraw.NewBidirectionalRootPrivate(reservation, directionalraw.BidirectionalPlanPrivate{})
	if err != nil {
		t.Fatal(err)
	}
	child, _ := root.ChildPrivate("incoming")
	if err := child.ReservePrivate(9); err != nil {
		t.Fatal(err)
	}
	coordinator, _ := NewCoordinatorPrivate()
	_, err = coordinator.CoordinatePrivate(context.Background(), CoordinateRequestPrivate{Root: root, Facets: []FacetObservationPrivate{{Facet: "UNKNOWN", ObservationState: "UNAVAILABLE"}}})
	if err == nil {
		t.Fatal("ASSERT_INVALID_FACET_REJECTED")
	}
	state, _ := root.SnapshotPrivate()
	if terminalCalls != 1 || !state.Released || state.ReleaseCount != 1 || state.LiveBytes != 0 || state.RootReleasedBytes != 9 {
		t.Fatalf("ASSERT_INVALID_FACET_RELEASE_EXACT_ONCE: state=%+v calls=%d", state, terminalCalls)
	}
}

func TestFacetCompatibilityMatrixPrivate(t *testing.T) {
	cases := []struct {
		name  string
		input FacetObservationPrivate
		want  FacetDispositionPrivate
	}{
		{name: "D-OBSERVED_NONEMPTY", input: FacetObservationPrivate{Facet: "D", ObservationState: "OBSERVED_NONEMPTY"}, want: FacetDispositionPrivate{Facet: "D", State: "OBSERVED_NONEMPTY", Count: 1, Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED"}},
		{name: "D-OBSERVED_EMPTY", input: FacetObservationPrivate{Facet: "D", ObservationState: "OBSERVED_EMPTY"}, want: FacetDispositionPrivate{Facet: "D", State: "OBSERVED_EMPTY", Count: 0, Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED"}},
		{name: "D-UNAVAILABLE", input: FacetObservationPrivate{Facet: "D", ObservationState: "UNAVAILABLE"}, want: FacetDispositionPrivate{Facet: "D", State: "UNAVAILABLE", Count: 0, FailureFamily: "PROVIDER_ONLY", FailureCode: "DEFINITION_PROVIDER_UNAVAILABLE", Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED"}},
		{name: "R-OBSERVED_NONEMPTY", input: FacetObservationPrivate{Facet: "R", ObservationState: "OBSERVED_NONEMPTY"}, want: FacetDispositionPrivate{Facet: "R", State: "OBSERVED_NONEMPTY", Count: 1, Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED"}},
		{name: "R-OBSERVED_EMPTY", input: FacetObservationPrivate{Facet: "R", ObservationState: "OBSERVED_EMPTY"}, want: FacetDispositionPrivate{Facet: "R", State: "OBSERVED_EMPTY", Count: 0, Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED"}},
		{name: "R-UNAVAILABLE", input: FacetObservationPrivate{Facet: "R", ObservationState: "UNAVAILABLE"}, want: FacetDispositionPrivate{Facet: "R", State: "UNAVAILABLE", Count: 0, FailureFamily: "MIXED_PROVIDER_GENERIC", FailureCode: "REFERENCES_MIXED_PROVIDER_GENERIC_UNAVAILABLE", Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED"}},
		{name: "CALLS_INCOMING-OBSERVED_NONEMPTY", input: FacetObservationPrivate{Facet: "CALLS_INCOMING", ObservationState: "OBSERVED_NONEMPTY"}, want: FacetDispositionPrivate{Facet: "CALLS_INCOMING", State: "OBSERVED_NONEMPTY", Count: 1, Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED"}},
		{name: "CALLS_INCOMING-OBSERVED_EMPTY", input: FacetObservationPrivate{Facet: "CALLS_INCOMING", ObservationState: "OBSERVED_EMPTY"}, want: FacetDispositionPrivate{Facet: "CALLS_INCOMING", State: "OBSERVED_EMPTY", Count: 0, Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED"}},
		{name: "CALLS_INCOMING-UNAVAILABLE", input: FacetObservationPrivate{Facet: "CALLS_INCOMING", ObservationState: "UNAVAILABLE"}, want: FacetDispositionPrivate{Facet: "CALLS_INCOMING", State: "UNAVAILABLE", Count: 0, FailureFamily: "GENERIC", FailureCode: "CALLS_INCOMING_UNAVAILABLE", Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED"}},
		{name: "CALLS_OUTGOING-OBSERVED_NONEMPTY", input: FacetObservationPrivate{Facet: "CALLS_OUTGOING", ObservationState: "OBSERVED_NONEMPTY"}, want: FacetDispositionPrivate{Facet: "CALLS_OUTGOING", State: "OBSERVED_NONEMPTY", Count: 1, Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED"}},
		{name: "CALLS_OUTGOING-OBSERVED_EMPTY", input: FacetObservationPrivate{Facet: "CALLS_OUTGOING", ObservationState: "OBSERVED_EMPTY"}, want: FacetDispositionPrivate{Facet: "CALLS_OUTGOING", State: "OBSERVED_EMPTY", Count: 0, Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED"}},
		{name: "CALLS_OUTGOING-UNAVAILABLE", input: FacetObservationPrivate{Facet: "CALLS_OUTGOING", ObservationState: "UNAVAILABLE"}, want: FacetDispositionPrivate{Facet: "CALLS_OUTGOING", State: "UNAVAILABLE", Count: 0, FailureFamily: "GENERIC", FailureCode: "CALLS_OUTGOING_UNAVAILABLE", Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED"}},
	}
	if len(cases) != 12 {
		t.Fatalf("ASSERT_COMPAT_MATRIX_EXACT_TWELVE_ROWS: got=%d", len(cases))
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observations := []FacetObservationPrivate{{Facet: "D", ObservationState: "OBSERVED_EMPTY"}, {Facet: "R", ObservationState: "OBSERVED_EMPTY"}, {Facet: "CALLS_INCOMING", ObservationState: "OBSERVED_EMPTY"}, {Facet: "CALLS_OUTGOING", ObservationState: "OBSERVED_EMPTY"}}
			for i := range observations {
				if observations[i].Facet == tc.input.Facet {
					observations[i] = tc.input
				}
			}
			dispositions, err := ValidateFacetsPrivate(observations)
			if errors.Is(err, ErrBehaviorNotImplementedPrivate) {
				t.Fatal("ASSERT_SKELETON_ERR_BEHAVIOR_NOT_IMPLEMENTED_FACET_VALIDATION")
			}
			if err != nil {
				t.Fatalf("ASSERT_COMPAT_PRODUCTION_VALIDATE_FACETS[%s]: %v", tc.name, err)
			}
			if len(dispositions) != 4 {
				t.Fatalf("ASSERT_COMPAT_EXACT_FOUR_DERIVED_DISPOSITIONS[%s]: got=%d", tc.name, len(dispositions))
			}
			var got FacetDispositionPrivate
			for i := range dispositions {
				if dispositions[i].Facet == tc.input.Facet {
					got = dispositions[i]
					observations[i] = FacetObservationPrivate{Facet: "MUTATED_INPUT", ObservationState: "UNAVAILABLE"}
				}
			}
			if got.Facet == "MUTATED_INPUT" {
				t.Fatalf("ASSERT_COMPAT_DISPOSITION_NOT_INPUT_ALIAS_OR_ECHO[%s]: got=%+v", tc.name, got)
			}
			if got.Facet != tc.want.Facet || got.State != tc.want.State || got.Count != tc.want.Count {
				t.Fatalf("ASSERT_COMPAT_EXACT_TYPED_FACET_STATE[%s]: got=%+v want=%+v", tc.name, got, tc.want)
			}
			if got.FailureFamily != tc.want.FailureFamily || got.FailureCode != tc.want.FailureCode {
				t.Fatalf("ASSERT_COMPAT_EXACT_FAILURE_FAMILY_CODE[%s]: got=%+v want=%+v", tc.name, got, tc.want)
			}
			if got.Completeness != tc.want.Completeness || got.FeatureIdentity != tc.want.FeatureIdentity || got.Completeness != "UNKNOWN" || got.FeatureIdentity != "UNRESOLVED" {
				t.Fatalf("ASSERT_COMPAT_EXACT_UNKNOWN_UNRESOLVED[%s]: got=%+v want=%+v", tc.name, got, tc.want)
			}
		})
	}
}

func TestCoordinatorTypedSuppressionAndCancellationPrivate(t *testing.T) {
	facets := []FacetObservationPrivate{{Facet: "D", ObservationState: "OBSERVED_EMPTY"}, {Facet: "R", ObservationState: "OBSERVED_EMPTY"}, {Facet: "CALLS_INCOMING", ObservationState: "OBSERVED_EMPTY"}, {Facet: "CALLS_OUTGOING", ObservationState: "OBSERVED_EMPTY"}}
	result, err := CoordinatePrivate(context.Background(), CoordinateRequestPrivate{Facets: facets})
	if errors.Is(err, ErrBehaviorNotImplementedPrivate) {
		t.Fatal("ASSERT_SKELETON_ERR_BEHAVIOR_NOT_IMPLEMENTED_COORDINATE_FUNCTION")
	}
	if err != nil {
		t.Fatal(err)
	}
	if result.CandidatePrivate() != nil || result.SuppressionPrivate() == nil || result.SuppressionPrivate().Code != "ATTEMPT_MISMATCH" || result.SuppressionPrivate().Stage != "typed_raw_pair" || result.SuppressionPrivate().Direction != "" || result.SuppressionPrivate().Family != "GENERIC" {
		t.Fatalf("ASSERT_TYPED_CANDIDATE_SUPPRESSION_EXACT_FIELDS: %+v", result)
	}
	if !result.LeaseStatePrivate().Released || result.LeaseStatePrivate().RootReleaseCount != 1 || result.LeaseStatePrivate().IncomingLive != 0 || result.LeaseStatePrivate().OutgoingLive != 0 || result.LeaseStatePrivate().RootLive != 0 {
		t.Fatalf("ASSERT_COORDINATOR_CANCELLATION_RELEASE_LIVE_ZERO: %+v", result.LeaseStatePrivate())
	}
}
