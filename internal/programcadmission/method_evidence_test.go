package programcadmission_test

import (
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/programccompose"
)

const (
	methodEvidenceFamily       = "lsp-trace.private.program-c-method-evidence.v0"
	methodEvidenceClaimCeiling = "STRUCTURAL_AND_CUSTODY_BINDINGS_ONLY;AUTHORITY_0;ACCEPTED_FALSE;COMPLETENESS_UNKNOWN;NO_PRODUCER_AUTHENTICATION"
)

func methodOccurrence(identity string, kind programcadmission.RelationKind, ordinal int, from, to string) programcadmission.MethodOccurrenceEvidence {
	return programcadmission.MethodOccurrenceEvidence{
		Identity: identity,
		Kind:     kind,
		Ordinal:  ordinal,
		From:     from,
		To:       to,
		SourceRole: map[programcadmission.RelationKind]string{
			programcadmission.TypedReferencesSymbol:     "REFERENCING_OCCURRENCE",
			programcadmission.TypedResolvesToDefinition: "QUERY_SYMBOL",
		}[kind],
		TargetRole: map[programcadmission.RelationKind]string{
			programcadmission.TypedReferencesSymbol:     "REFERENCED_SYMBOL",
			programcadmission.TypedResolvesToDefinition: "DEFINITION_TARGET",
		}[kind],
		Method: map[programcadmission.RelationKind]string{
			programcadmission.TypedReferencesSymbol:     "textDocument/references",
			programcadmission.TypedResolvesToDefinition: "textDocument/definition",
		}[kind],
		QuerySelector:    "queries/" + identity + ".json",
		QueryDigest:      "sha256:" + strings.Repeat("1", 64),
		QueryLength:      17,
		EvidenceSelector: "evidence/" + identity + ".json",
		EvidenceDigest:   "sha256:" + strings.Repeat("2", 64),
		EvidenceLength:   29,
		Provider:         "gopls",
		ProviderVersion:  "v0.17.1",
		SessionID:        "session-1",
		Generation:       7,
		Revision:         "commit-1",
		Custody:          "CALLER_ASSERTED",
		PositionEncoding: "utf-16",
	}
}

func validUntrustedMethodEvidenceEnvelope() programcadmission.UntrustedMethodEvidenceEnvelope {
	return programcadmission.UntrustedMethodEvidenceEnvelope{
		Family:           methodEvidenceFamily,
		Version:          methodEvidenceFamily,
		ClaimCeiling:     methodEvidenceClaimCeiling,
		Authority:        0,
		Accepted:         false,
		Completeness:     "UNKNOWN",
		Provider:         "gopls",
		ProviderVersion:  "v0.17.1",
		SessionID:        "session-1",
		Generation:       7,
		Revision:         "commit-1",
		Custody:          "CALLER_ASSERTED",
		PositionEncoding: "utf-16",
		Occurrences: []programcadmission.MethodOccurrenceEvidence{
			methodOccurrence("reference-1", programcadmission.TypedReferencesSymbol, 1, "file:///w/use.go#1", "file:///w/decl.go#target"),
			methodOccurrence("definition-1", programcadmission.TypedResolvesToDefinition, 0, "file:///w/use.go#query", "file:///w/decl.go#definition"),
			methodOccurrence("reference-0", programcadmission.TypedReferencesSymbol, 0, "file:///w/use.go#1", "file:///w/decl.go#target"),
		},
	}
}

func opaqueOccurrences(t *testing.T, admitted any) []programcadmission.MethodOccurrenceEvidence {
	t.Helper()
	method := reflect.ValueOf(admitted).MethodByName("Occurrences")
	if !method.IsValid() {
		t.Fatal("ASSERT_METHOD_ADMISSION_OPAQUE_OCCURRENCES_ACCESSOR")
	}
	results := method.Call(nil)
	if len(results) != 1 {
		t.Fatal("ASSERT_METHOD_ADMISSION_OPAQUE_OCCURRENCES_SIGNATURE")
	}
	got, ok := results[0].Interface().([]programcadmission.MethodOccurrenceEvidence)
	if !ok {
		t.Fatal("ASSERT_METHOD_ADMISSION_OPAQUE_OCCURRENCES_TYPE")
	}
	return got
}

func TestUntrustedMethodEvidenceEnvelopeProducesOpaqueStructuralCustodyAdmission(t *testing.T) {
	req := validUntrustedMethodEvidenceEnvelope()
	admitted, err := programcadmission.AdmitUntrustedMethodEvidenceEnvelope(req)
	if err != nil {
		t.Fatal(err)
	}
	if got := reflect.TypeOf(admitted).Name(); got != "MethodProjectionAdmission" {
		t.Fatalf("ASSERT_OPAQUE_METHOD_PROJECTION_ADMISSION got=%q", got)
	}
	if admitted.ClaimCeiling() != methodEvidenceClaimCeiling || admitted.Authority() != 0 || admitted.Accepted() || admitted.Completeness() != "UNKNOWN" {
		t.Fatalf("ASSERT_METHOD_ADMISSION_STRUCTURAL_CUSTODY_CEILING_ONLY admitted=%+v", admitted)
	}
	got := opaqueOccurrences(t, admitted)
	if len(got) != len(req.Occurrences) {
		t.Fatalf("ASSERT_METHOD_ADMISSION_MULTIPLICITY got=%d want=%d", len(got), len(req.Occurrences))
	}
	before := got[0]
	req.Occurrences[0].Identity = "tampered-input"
	got[0].Identity = "tampered-output"
	if stable := opaqueOccurrences(t, admitted); stable[0] != before {
		t.Fatal("ASSERT_METHOD_ADMISSION_DEFENSIVE_COPY")
	}
}

func TestUntrustedMethodEvidenceEnvelopeStableErrorCategories(t *testing.T) {
	base := validUntrustedMethodEvidenceEnvelope()
	cases := []struct {
		name string
		want error
		edit func(*programcadmission.UntrustedMethodEvidenceEnvelope)
	}{
		{"envelope family", programcadmission.ErrMethodEvidenceEnvelope, func(r *programcadmission.UntrustedMethodEvidenceEnvelope) { r.Family = "wrong" }},
		{"version", programcadmission.ErrMethodEvidenceVersion, func(r *programcadmission.UntrustedMethodEvidenceEnvelope) { r.Version = "wrong" }},
		{"claim ceiling", programcadmission.ErrMethodEvidenceClaim, func(r *programcadmission.UntrustedMethodEvidenceEnvelope) { r.ClaimCeiling = "wrong" }},
		{"claim authority", programcadmission.ErrMethodEvidenceClaim, func(r *programcadmission.UntrustedMethodEvidenceEnvelope) { r.Authority = 1 }},
		{"claim accepted", programcadmission.ErrMethodEvidenceClaim, func(r *programcadmission.UntrustedMethodEvidenceEnvelope) { r.Accepted = true }},
		{"claim completeness", programcadmission.ErrMethodEvidenceClaim, func(r *programcadmission.UntrustedMethodEvidenceEnvelope) { r.Completeness = "COMPLETE" }},
		{"unsupported kind", programcadmission.ErrMethodEvidenceUnsupported, func(r *programcadmission.UntrustedMethodEvidenceEnvelope) {
			r.Occurrences[0].Kind = programcadmission.TypedCalls
		}},
		{"unsupported method role", programcadmission.ErrMethodEvidenceUnsupported, func(r *programcadmission.UntrustedMethodEvidenceEnvelope) {
			r.Occurrences[0].Method = "textDocument/definition"
		}},
		{"malformed binding", programcadmission.ErrMethodEvidenceMalformedBinding, func(r *programcadmission.UntrustedMethodEvidenceEnvelope) {
			r.Occurrences[0].SourceRole = "QUERY_SYMBOL"
		}},
		{"duplicate identity", programcadmission.ErrMethodEvidenceDuplicateIdentity, func(r *programcadmission.UntrustedMethodEvidenceEnvelope) {
			r.Occurrences[1].Identity = r.Occurrences[0].Identity
		}},
		{"resource bound", programcadmission.ErrMethodEvidenceResourceBound, func(r *programcadmission.UntrustedMethodEvidenceEnvelope) {
			r.Occurrences = make([]programcadmission.MethodOccurrenceEvidence, programcadmission.MaxMethodOccurrences+1)
			for i := range r.Occurrences {
				r.Occurrences[i] = methodOccurrence("bounded-"+strings.Repeat("x", i%3)+string(rune(i+1)), programcadmission.TypedReferencesSymbol, i, "from", "to")
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := base
			req.Occurrences = append([]programcadmission.MethodOccurrenceEvidence(nil), base.Occurrences...)
			tc.edit(&req)
			zero, err := programcadmission.AdmitUntrustedMethodEvidenceEnvelope(req)
			if !errors.Is(err, tc.want) || len(opaqueOccurrences(t, zero)) != 0 {
				t.Fatalf("ASSERT_METHOD_ADMISSION_STABLE_ERROR_CATEGORY want=%v err=%v", tc.want, err)
			}
		})
	}
}

func TestUntrustedMethodEvidenceEnvelopeRejectsMalformedBindings(t *testing.T) {
	base := validUntrustedMethodEvidenceEnvelope()
	cases := []struct {
		name string
		edit func(*programcadmission.MethodOccurrenceEvidence)
	}{
		{"empty identity", func(o *programcadmission.MethodOccurrenceEvidence) { o.Identity = "" }},
		{"empty from", func(o *programcadmission.MethodOccurrenceEvidence) { o.From = "" }},
		{"empty to", func(o *programcadmission.MethodOccurrenceEvidence) { o.To = "" }},
		{"negative ordinal", func(o *programcadmission.MethodOccurrenceEvidence) { o.Ordinal = -1 }},
		{"query selector empty", func(o *programcadmission.MethodOccurrenceEvidence) { o.QuerySelector = "" }},
		{"query digest malformed", func(o *programcadmission.MethodOccurrenceEvidence) { o.QueryDigest = "sha256:bad" }},
		{"query length zero", func(o *programcadmission.MethodOccurrenceEvidence) { o.QueryLength = 0 }},
		{"query length negative", func(o *programcadmission.MethodOccurrenceEvidence) { o.QueryLength = -1 }},
		{"evidence selector empty", func(o *programcadmission.MethodOccurrenceEvidence) { o.EvidenceSelector = "" }},
		{"evidence digest malformed", func(o *programcadmission.MethodOccurrenceEvidence) { o.EvidenceDigest = "sha256:bad" }},
		{"evidence length zero", func(o *programcadmission.MethodOccurrenceEvidence) { o.EvidenceLength = 0 }},
		{"evidence length negative", func(o *programcadmission.MethodOccurrenceEvidence) { o.EvidenceLength = -1 }},
		{"references target role", func(o *programcadmission.MethodOccurrenceEvidence) { o.TargetRole = "DEFINITION_TARGET" }},
		{"definition source role", func(o *programcadmission.MethodOccurrenceEvidence) {
			*o = methodOccurrence(o.Identity, programcadmission.TypedResolvesToDefinition, o.Ordinal, o.From, o.To)
			o.SourceRole = "REFERENCING_OCCURRENCE"
		}},
		{"definition target role", func(o *programcadmission.MethodOccurrenceEvidence) {
			*o = methodOccurrence(o.Identity, programcadmission.TypedResolvesToDefinition, o.Ordinal, o.From, o.To)
			o.TargetRole = "REFERENCED_SYMBOL"
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := base
			req.Occurrences = append([]programcadmission.MethodOccurrenceEvidence(nil), base.Occurrences...)
			tc.edit(&req.Occurrences[0])
			zero, err := programcadmission.AdmitUntrustedMethodEvidenceEnvelope(req)
			if !errors.Is(err, programcadmission.ErrMethodEvidenceMalformedBinding) || len(opaqueOccurrences(t, zero)) != 0 {
				t.Fatalf("ASSERT_METHOD_BINDING_REJECTED err=%v", err)
			}
		})
	}
}

func TestMethodEvidenceNumericLengthsAcceptBoundaryAndRejectOverBound(t *testing.T) {
	for _, tc := range []struct {
		name  string
		limit int
		edit  func(*programcadmission.MethodOccurrenceEvidence, int)
	}{
		{"query length", programcadmission.MaxMethodQueryBytes, func(o *programcadmission.MethodOccurrenceEvidence, n int) { o.QueryLength = n }},
		{"evidence length", programcadmission.MaxMethodEvidenceBytes, func(o *programcadmission.MethodOccurrenceEvidence, n int) { o.EvidenceLength = n }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, n := range []int{1, tc.limit} {
				req := validUntrustedMethodEvidenceEnvelope()
				tc.edit(&req.Occurrences[0], n)
				if _, err := programcadmission.AdmitUntrustedMethodEvidenceEnvelope(req); err != nil {
					t.Fatalf("ASSERT_METHOD_NUMERIC_LENGTH_BOUNDARY_ACCEPTED length=%d err=%v", n, err)
				}
			}
			req := validUntrustedMethodEvidenceEnvelope()
			tc.edit(&req.Occurrences[0], tc.limit+1)
			if _, err := programcadmission.AdmitUntrustedMethodEvidenceEnvelope(req); !errors.Is(err, programcadmission.ErrMethodEvidenceResourceBound) {
				t.Fatalf("ASSERT_METHOD_NUMERIC_LENGTH_OVER_BOUND_REJECTED limit=%d err=%v", tc.limit, err)
			}
		})
	}
}

func TestMethodEvidenceTextFieldsAcceptBoundaryAndRejectOverBound(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*programcadmission.UntrustedMethodEvidenceEnvelope, string)
	}{
		{"identity", func(r *programcadmission.UntrustedMethodEvidenceEnvelope, s string) { r.Occurrences[0].Identity = s }},
		{"from endpoint", func(r *programcadmission.UntrustedMethodEvidenceEnvelope, s string) { r.Occurrences[0].From = s }},
		{"to endpoint", func(r *programcadmission.UntrustedMethodEvidenceEnvelope, s string) { r.Occurrences[0].To = s }},
		{"query selector", func(r *programcadmission.UntrustedMethodEvidenceEnvelope, s string) {
			r.Occurrences[0].QuerySelector = s
		}},
		{"evidence selector", func(r *programcadmission.UntrustedMethodEvidenceEnvelope, s string) {
			r.Occurrences[0].EvidenceSelector = s
		}},
		{"provider", func(r *programcadmission.UntrustedMethodEvidenceEnvelope, s string) {
			r.Provider = s
			for i := range r.Occurrences {
				r.Occurrences[i].Provider = s
			}
		}},
		{"provider version", func(r *programcadmission.UntrustedMethodEvidenceEnvelope, s string) {
			r.ProviderVersion = s
			for i := range r.Occurrences {
				r.Occurrences[i].ProviderVersion = s
			}
		}},
		{"session", func(r *programcadmission.UntrustedMethodEvidenceEnvelope, s string) {
			r.SessionID = s
			for i := range r.Occurrences {
				r.Occurrences[i].SessionID = s
			}
		}},
		{"revision", func(r *programcadmission.UntrustedMethodEvidenceEnvelope, s string) {
			r.Revision = s
			for i := range r.Occurrences {
				r.Occurrences[i].Revision = s
			}
		}},
		{"custody", func(r *programcadmission.UntrustedMethodEvidenceEnvelope, s string) {
			r.Custody = s
			for i := range r.Occurrences {
				r.Occurrences[i].Custody = s
			}
		}},
		{"position encoding", func(r *programcadmission.UntrustedMethodEvidenceEnvelope, s string) {
			r.PositionEncoding = s
			for i := range r.Occurrences {
				r.Occurrences[i].PositionEncoding = s
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			boundary := strings.Repeat("x", programcadmission.MaxMethodFieldBytes)
			req := validUntrustedMethodEvidenceEnvelope()
			tc.edit(&req, boundary)
			if _, err := programcadmission.AdmitUntrustedMethodEvidenceEnvelope(req); err != nil {
				t.Fatalf("ASSERT_METHOD_TEXT_FIELD_BOUNDARY_ACCEPTED err=%v", err)
			}
			tc.edit(&req, boundary+"x")
			if _, err := programcadmission.AdmitUntrustedMethodEvidenceEnvelope(req); !errors.Is(err, programcadmission.ErrMethodEvidenceResourceBound) {
				t.Fatalf("ASSERT_METHOD_TEXT_FIELD_OVER_BOUND_REJECTED err=%v", err)
			}
		})
	}
}

func TestMethodEvidenceOversizedMalformedDigestsReject(t *testing.T) {
	oversizedMalformed := "sha256:" + strings.Repeat("z", programcadmission.MaxMethodFieldBytes+1)
	for _, edit := range []func(*programcadmission.MethodOccurrenceEvidence){
		func(o *programcadmission.MethodOccurrenceEvidence) { o.QueryDigest = oversizedMalformed },
		func(o *programcadmission.MethodOccurrenceEvidence) { o.EvidenceDigest = oversizedMalformed },
	} {
		req := validUntrustedMethodEvidenceEnvelope()
		edit(&req.Occurrences[0])
		if _, err := programcadmission.AdmitUntrustedMethodEvidenceEnvelope(req); !errors.Is(err, programcadmission.ErrMethodEvidenceMalformedBinding) && !errors.Is(err, programcadmission.ErrMethodEvidenceResourceBound) {
			t.Fatalf("ASSERT_OVERSIZED_MALFORMED_METHOD_DIGEST_REJECTED err=%v", err)
		}
	}
}

func TestMethodEvidenceEnvelopeCustodyMustBeNonEmptyAndMatchEveryOccurrence(t *testing.T) {
	type custodyField struct {
		name     string
		empty    func(*programcadmission.UntrustedMethodEvidenceEnvelope)
		mismatch func(*programcadmission.MethodOccurrenceEvidence)
	}
	fields := []custodyField{
		{"provider", func(r *programcadmission.UntrustedMethodEvidenceEnvelope) { r.Provider = "" }, func(o *programcadmission.MethodOccurrenceEvidence) { o.Provider = "other" }},
		{"provider version", func(r *programcadmission.UntrustedMethodEvidenceEnvelope) { r.ProviderVersion = "" }, func(o *programcadmission.MethodOccurrenceEvidence) { o.ProviderVersion = "other" }},
		{"session", func(r *programcadmission.UntrustedMethodEvidenceEnvelope) { r.SessionID = "" }, func(o *programcadmission.MethodOccurrenceEvidence) { o.SessionID = "other" }},
		{"generation", func(r *programcadmission.UntrustedMethodEvidenceEnvelope) { r.Generation = 0 }, func(o *programcadmission.MethodOccurrenceEvidence) { o.Generation++ }},
		{"revision", func(r *programcadmission.UntrustedMethodEvidenceEnvelope) { r.Revision = "" }, func(o *programcadmission.MethodOccurrenceEvidence) { o.Revision = "other" }},
		{"custody", func(r *programcadmission.UntrustedMethodEvidenceEnvelope) { r.Custody = "" }, func(o *programcadmission.MethodOccurrenceEvidence) { o.Custody = "other" }},
		{"position encoding", func(r *programcadmission.UntrustedMethodEvidenceEnvelope) { r.PositionEncoding = "" }, func(o *programcadmission.MethodOccurrenceEvidence) { o.PositionEncoding = "other" }},
	}
	for _, tc := range fields {
		t.Run(tc.name+" empty", func(t *testing.T) {
			req := validUntrustedMethodEvidenceEnvelope()
			tc.empty(&req)
			if _, err := programcadmission.AdmitUntrustedMethodEvidenceEnvelope(req); !errors.Is(err, programcadmission.ErrMethodEvidenceMalformedBinding) {
				t.Fatalf("ASSERT_METHOD_ENVELOPE_CUSTODY_NONEMPTY err=%v", err)
			}
		})
		for _, occurrence := range []struct {
			name  string
			index int
		}{{"references", 0}, {"definitions", 1}} {
			t.Run(tc.name+" "+occurrence.name+" mismatch", func(t *testing.T) {
				req := validUntrustedMethodEvidenceEnvelope()
				tc.mismatch(&req.Occurrences[occurrence.index])
				if _, err := programcadmission.AdmitUntrustedMethodEvidenceEnvelope(req); !errors.Is(err, programcadmission.ErrMethodEvidenceMalformedBinding) {
					t.Fatalf("ASSERT_METHOD_OCCURRENCE_CUSTODY_MATCHES_ENVELOPE err=%v", err)
				}
			})
		}
	}
	if req := validUntrustedMethodEvidenceEnvelope(); req.ClaimCeiling != methodEvidenceClaimCeiling || !strings.Contains(req.ClaimCeiling, "NO_PRODUCER_AUTHENTICATION") {
		t.Fatalf("ASSERT_METHOD_CUSTODY_DOES_NOT_CLAIM_PRODUCER_AUTHENTICATION ceiling=%q", req.ClaimCeiling)
	}
}

func TestTypedMethodEvidenceSelectionsMultiplicityAndDirectedWalkCandidates(t *testing.T) {
	methods, err := programcadmission.AdmitUntrustedMethodEvidenceEnvelope(validUntrustedMethodEvidenceEnvelope())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		kinds []programcadmission.RelationKind
		want  int
	}{
		{"references only", []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol}, 2},
		{"definitions only", []programcadmission.RelationKind{programcadmission.TypedResolvesToDefinition}, 1},
		{"both mappings", []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := programcadmission.PrepareTypedInput(programcadmission.TypedInputRequest{Methods: methods, EnableMethods: true, Kinds: tc.kinds})
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Occurrences) != tc.want || !sort.SliceIsSorted(got.Occurrences, func(i, j int) bool { return got.Occurrences[i].Identity < got.Occurrences[j].Identity }) {
				t.Fatalf("ASSERT_METHOD_OCCURRENCE_DETERMINISTIC_ORDER_AND_SELECTION %+v", got.Occurrences)
			}
			candidates, err := programcadmission.DirectedWalkCandidates(got)
			if err != nil || len(candidates) != len(got.Occurrences) {
				t.Fatalf("ASSERT_DIRECTED_WALK_ONE_PER_OCCURRENCE candidates=%+v err=%v", candidates, err)
			}
			for i, candidate := range candidates {
				occurrence := got.Occurrences[i]
				want := programcadmission.MethodCandidate{Identity: occurrence.Identity, Kind: occurrence.Kind, From: occurrence.From, To: occurrence.To}
				if candidate != want {
					t.Fatalf("ASSERT_DIRECTED_WALK_ALL_AVAILABLE_FIELDS got=%+v want=%+v", candidate, want)
				}
			}
		})
	}

	both, err := programcadmission.PrepareTypedInput(programcadmission.TypedInputRequest{Methods: methods, EnableMethods: true, Kinds: []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}})
	if err != nil {
		t.Fatal(err)
	}
	pair := programcadmission.TypedPair{From: "file:///w/use.go#1", To: "file:///w/decl.go#target"}
	if both.PairWeights[pair] != 2 {
		t.Fatalf("ASSERT_METHOD_PAIR_WEIGHT_PRESERVES_OCCURRENCE_MULTIPLICITY weights=%+v", both.PairWeights)
	}
	crossKind := validUntrustedMethodEvidenceEnvelope()
	crossKind.Occurrences[1].From, crossKind.Occurrences[1].To = pair.From, pair.To
	crossAdmission, err := programcadmission.AdmitUntrustedMethodEvidenceEnvelope(crossKind)
	if err != nil {
		t.Fatal(err)
	}
	cross, err := programcadmission.PrepareTypedInput(programcadmission.TypedInputRequest{Methods: crossAdmission, EnableMethods: true, Kinds: []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}})
	if err != nil || cross.PairWeights[pair] != 3 {
		t.Fatalf("ASSERT_CROSS_KIND_EQUAL_ENDPOINT_PAIR_MULTIPLICITY input=%+v err=%v", cross, err)
	}
}

func TestDirectedWalkCandidatesConversionFailureIsAtomicZero(t *testing.T) {
	invalid := programcadmission.TypedInput{Occurrences: []programcadmission.TypedOccurrence{{Identity: "internal-invalid-marker", Kind: programcadmission.RelationKind("INTERNAL_INVALID"), From: "from", To: "to"}}}
	candidates, err := programcadmission.DirectedWalkCandidates(invalid)
	if err == nil || len(candidates) != 0 {
		t.Fatalf("ASSERT_DIRECTED_WALK_CONVERSION_FAILURE_ATOMIC_ZERO candidates=%+v err=%v", candidates, err)
	}
}

func TestTypedMethodEvidenceDefensiveCopiesKindsOccurrencesAndPairWeights(t *testing.T) {
	methods, err := programcadmission.AdmitUntrustedMethodEvidenceEnvelope(validUntrustedMethodEvidenceEnvelope())
	if err != nil {
		t.Fatal(err)
	}
	kinds := []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol}
	got, err := programcadmission.PrepareTypedInput(programcadmission.TypedInputRequest{Methods: methods, EnableMethods: true, Kinds: kinds})
	if err != nil {
		t.Fatal(err)
	}
	kinds[0] = programcadmission.TypedCalls
	beforeOccurrence, beforeWeight := got.Occurrences[0], got.PairWeights[programcadmission.TypedPair{From: got.Occurrences[0].From, To: got.Occurrences[0].To}]
	got.Occurrences[0].Identity = "tampered"
	got.PairWeights[programcadmission.TypedPair{From: beforeOccurrence.From, To: beforeOccurrence.To}] = 999
	stable, err := programcadmission.PrepareTypedInput(programcadmission.TypedInputRequest{Methods: methods, EnableMethods: true, Kinds: []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol}})
	if err != nil || stable.Occurrences[0] != beforeOccurrence || stable.PairWeights[programcadmission.TypedPair{From: beforeOccurrence.From, To: beforeOccurrence.To}] != beforeWeight {
		t.Fatalf("ASSERT_TYPED_METHOD_DEFENSIVE_COPY stable=%+v err=%v", stable, err)
	}
}

func TestMethodsPresentButDisabledCannotAffectCallsAndRawCandidatesRemainRejected(t *testing.T) {
	a, b := node("typed-method-a"), node("typed-method-b")
	e := edge(a, b)
	e.CallSites = []graph.Range{{End: graph.Position{Character: 1}}, {End: graph.Position{Character: 2}}}
	composite, err := programccompose.Compose([]programccompose.Input{
		capture(t, "typed-method", []graph.Node{a, b}, []graph.Edge{e}),
		capture(t, "typed-method-context", []graph.Node{a, b}, nil),
	})
	if err != nil {
		t.Fatal(err)
	}
	calls, err := programcadmission.Admit(composite.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	callsOnly, err := programcadmission.PrepareTypedInput(programcadmission.TypedInputRequest{Calls: calls.Admission, Kinds: []programcadmission.RelationKind{programcadmission.TypedCalls}})
	if err != nil {
		t.Fatal(err)
	}
	methods, err := programcadmission.AdmitUntrustedMethodEvidenceEnvelope(validUntrustedMethodEvidenceEnvelope())
	if err != nil {
		t.Fatal(err)
	}
	disabled, err := programcadmission.PrepareTypedInput(programcadmission.TypedInputRequest{Calls: calls.Admission, Methods: methods, EnableMethods: false, Kinds: []programcadmission.RelationKind{programcadmission.TypedCalls}})
	if err != nil || !reflect.DeepEqual(disabled, callsOnly) {
		t.Fatalf("ASSERT_DISABLED_METHODS_CANNOT_AFFECT_CALLS disabled=%+v calls=%+v err=%v", disabled, callsOnly, err)
	}
	raw := programcadmission.TypedInputRequest{Calls: calls.Admission, Methods: methods, EnableMethods: false, Kinds: []programcadmission.RelationKind{programcadmission.TypedCalls}, MethodCandidates: []programcadmission.MethodCandidate{{Identity: "raw", Kind: programcadmission.TypedReferencesSymbol, From: a.ID, To: b.ID}}}
	if out, err := programcadmission.PrepareTypedInput(raw); !errors.Is(err, programcadmission.ErrMethodEvidenceUnadmitted) || len(out.Occurrences) != 0 || len(out.PairWeights) != 0 {
		t.Fatalf("ASSERT_RAW_METHOD_CANDIDATES_REMAIN_REJECTED out=%+v err=%v", out, err)
	}
}

func TestTypedMixedCallsReferencesAndDefinitionsPreservesCallsAndAddsMethods(t *testing.T) {
	a, b := node("mixed-typed-a"), node("mixed-typed-b")
	e := edge(a, b)
	e.CallSites = []graph.Range{{End: graph.Position{Character: 1}}, {End: graph.Position{Character: 2}}}
	composite, err := programccompose.Compose([]programccompose.Input{
		capture(t, "mixed-typed", []graph.Node{a, b}, []graph.Edge{e}),
		capture(t, "mixed-typed-context", []graph.Node{a, b}, nil),
	})
	if err != nil {
		t.Fatal(err)
	}
	calls, err := programcadmission.Admit(composite.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	callsOnly, err := programcadmission.PrepareTypedInput(programcadmission.TypedInputRequest{Calls: calls.Admission, Kinds: []programcadmission.RelationKind{programcadmission.TypedCalls}})
	if err != nil {
		t.Fatal(err)
	}
	methods, err := programcadmission.AdmitUntrustedMethodEvidenceEnvelope(validUntrustedMethodEvidenceEnvelope())
	if err != nil {
		t.Fatal(err)
	}
	mixed, err := programcadmission.PrepareTypedInput(programcadmission.TypedInputRequest{
		Calls: calls.Admission, Methods: methods, EnableMethods: true,
		Kinds: []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(mixed.Occurrences) != len(callsOnly.Occurrences)+3 || !sort.SliceIsSorted(mixed.Occurrences, func(i, j int) bool { return mixed.Occurrences[i].Identity < mixed.Occurrences[j].Identity }) {
		t.Fatalf("ASSERT_MIXED_TYPED_OCCURRENCES_ADDITIVE_DETERMINISTIC got=%+v", mixed.Occurrences)
	}
	mixedCalls := make([]programcadmission.TypedOccurrence, 0, len(callsOnly.Occurrences))
	for _, occurrence := range mixed.Occurrences {
		if occurrence.Kind == programcadmission.TypedCalls {
			mixedCalls = append(mixedCalls, occurrence)
		}
	}
	if !reflect.DeepEqual(mixedCalls, callsOnly.Occurrences) || mixed.Version != callsOnly.Version || mixed.CompositeID != callsOnly.CompositeID || mixed.CompositeOutputSHA256 != callsOnly.CompositeOutputSHA256 || mixed.ClaimCeiling != callsOnly.ClaimCeiling || mixed.Authority != callsOnly.Authority || mixed.SourceGraphComplete != callsOnly.SourceGraphComplete {
		t.Fatalf("ASSERT_MIXED_TYPED_CALLS_FIELDS_AND_METADATA_UNCHANGED mixed=%+v calls=%+v", mixed, callsOnly)
	}
	wantWeights := make(map[programcadmission.TypedPair]int, len(callsOnly.PairWeights)+3)
	for pair, weight := range callsOnly.PairWeights {
		wantWeights[pair] = weight
	}
	for _, occurrence := range opaqueOccurrences(t, methods) {
		wantWeights[programcadmission.TypedPair{From: occurrence.From, To: occurrence.To}]++
	}
	if !reflect.DeepEqual(mixed.PairWeights, wantWeights) {
		t.Fatalf("ASSERT_MIXED_TYPED_PAIR_WEIGHTS_ADDITIVE got=%+v want=%+v", mixed.PairWeights, wantWeights)
	}
}

func TestTypedMethodSelectionErrorsAndCallsIdentityCollision(t *testing.T) {
	methods, err := programcadmission.AdmitUntrustedMethodEvidenceEnvelope(validUntrustedMethodEvidenceEnvelope())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		req  programcadmission.TypedInputRequest
	}{
		{"unsupported kind selection", programcadmission.TypedInputRequest{Methods: methods, EnableMethods: true, Kinds: []programcadmission.RelationKind{"UNKNOWN"}}},
		{"duplicate kind selection", programcadmission.TypedInputRequest{Methods: methods, EnableMethods: true, Kinds: []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedReferencesSymbol}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := programcadmission.PrepareTypedInput(tc.req)
			if !errors.Is(err, programcadmission.ErrMethodEvidenceInvalidSelection) || len(out.Occurrences) != 0 || len(out.PairWeights) != 0 {
				t.Fatalf("ASSERT_TYPED_METHOD_INVALID_SELECTION out=%+v err=%v", out, err)
			}
		})
	}

	a, b := node("identity-collision-a"), node("identity-collision-b")
	composite, err := programccompose.Compose([]programccompose.Input{
		capture(t, "identity-collision", []graph.Node{a, b}, []graph.Edge{edge(a, b)}),
		capture(t, "identity-collision-context", []graph.Node{a, b}, nil),
	})
	if err != nil {
		t.Fatal(err)
	}
	calls, err := programcadmission.Admit(composite.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	callsOnly, err := programcadmission.PrepareTypedInput(programcadmission.TypedInputRequest{Calls: calls.Admission, Kinds: []programcadmission.RelationKind{programcadmission.TypedCalls}})
	if err != nil || len(callsOnly.Occurrences) == 0 {
		t.Fatalf("calls fixture: %+v %v", callsOnly, err)
	}
	collision := validUntrustedMethodEvidenceEnvelope()
	collision.Occurrences[0].Identity = callsOnly.Occurrences[0].Identity
	collisionMethods, err := programcadmission.AdmitUntrustedMethodEvidenceEnvelope(collision)
	if err != nil {
		t.Fatal(err)
	}
	out, err := programcadmission.PrepareTypedInput(programcadmission.TypedInputRequest{Calls: calls.Admission, Methods: collisionMethods, EnableMethods: true, Kinds: []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol}})
	if !errors.Is(err, programcadmission.ErrMethodEvidenceDuplicateIdentity) || len(out.Occurrences) != 0 || len(out.PairWeights) != 0 {
		t.Fatalf("ASSERT_TRUE_CALLS_METHOD_IDENTITY_COLLISION out=%+v err=%v", out, err)
	}
}

func TestMethodEvidenceContractRemainsPrivateToInternalPackage(t *testing.T) {
	for _, value := range []any{
		programcadmission.UntrustedMethodEvidenceEnvelope{},
		programcadmission.MethodOccurrenceEvidence{},
		programcadmission.TypedInputRequest{},
	} {
		if got := reflect.TypeOf(value).PkgPath(); got != "lsp-trace/internal/programcadmission" {
			t.Fatalf("ASSERT_METHOD_CONTRACT_INTERNAL_ONLY type=%T package=%s", value, got)
		}
	}
}
