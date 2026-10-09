package programcadmission

import (
	"crypto/sha256"
	"errors"
	"reflect"
	"testing"
)

func TestMethodProjectionAdmissionCanonicalCommitmentIsDeterministicAndDefensive(t *testing.T) {
	envelope := parallelMethodEnvelope()
	first, err := AdmitUntrustedMethodEvidenceEnvelope(envelope)
	if err != nil {
		t.Fatal(err)
	}
	permuted := envelope
	permuted.Occurrences = append([]MethodOccurrenceEvidence(nil), envelope.Occurrences...)
	permuted.Occurrences[0], permuted.Occurrences[1] = permuted.Occurrences[1], permuted.Occurrences[0]
	second, err := AdmitUntrustedMethodEvidenceEnvelope(permuted)
	if err != nil {
		t.Fatal(err)
	}

	firstBytes, firstDigest := first.CanonicalBytes(), first.CommitmentDigest()
	secondBytes, secondDigest := second.CanonicalBytes(), second.CommitmentDigest()
	if len(firstBytes) == 0 || len(firstDigest) != sha256.Size {
		t.Fatalf("ASSERT_METHOD_ADMISSION_COMMITMENT_PRESENT bytes=%d digest=%x", len(firstBytes), firstDigest)
	}
	if !reflect.DeepEqual(firstBytes, secondBytes) || !reflect.DeepEqual(firstDigest, secondDigest) {
		t.Fatalf("ASSERT_METHOD_ADMISSION_CANONICAL_PERMUTATION_EQUIVALENT first=%x/%x second=%x/%x", firstBytes, firstDigest, secondBytes, secondDigest)
	}
	want := sha256.Sum256(append(append([]byte(nil), methodAdmissionCommitmentDomain...), firstBytes...))
	if !reflect.DeepEqual(firstDigest, want[:]) {
		t.Fatalf("ASSERT_METHOD_ADMISSION_DOMAIN_SEPARATED_SHA256 got=%x want=%x", firstDigest, want)
	}

	firstBytes[0] ^= 0xff
	firstDigest[0] ^= 0xff
	if reflect.DeepEqual(first.CanonicalBytes(), firstBytes) || reflect.DeepEqual(first.CommitmentDigest(), firstDigest) {
		t.Fatal("ASSERT_METHOD_ADMISSION_COMMITMENT_ACCESSORS_DEFENSIVE")
	}
	if first.ClaimCeiling() != MethodEvidenceClaimCeiling || first.Authority() != 0 || first.Accepted() || first.Completeness() != "UNKNOWN" {
		t.Fatalf("ASSERT_METHOD_ADMISSION_INTEGRITY_AND_ORIGIN_ONLY admission=%+v", first)
	}
}

func TestMethodProjectionAdmissionRejectsMissingZeroAndMismatchedCommitment(t *testing.T) {
	admitted, err := AdmitUntrustedMethodEvidenceEnvelope(parallelMethodEnvelope())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*MethodProjectionAdmission)
	}{
		{"missing canonical bytes", func(in *MethodProjectionAdmission) { in.canonicalBytes = nil }},
		{"zero digest", func(in *MethodProjectionAdmission) { in.commitmentDigest = [sha256.Size]byte{} }},
		{"mismatched digest", func(in *MethodProjectionAdmission) { in.commitmentDigest[0] ^= 0xff }},
		{"mismatched bytes", func(in *MethodProjectionAdmission) { in.canonicalBytes[0] ^= 0xff }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mutated := admitted
			mutated.canonicalBytes = append([]byte(nil), admitted.canonicalBytes...)
			tc.mutate(&mutated)
			got, err := PrepareTypedInput(TypedInputRequest{Methods: mutated, EnableMethods: true, Kinds: []RelationKind{TypedReferencesSymbol}})
			if !errors.Is(err, ErrMethodEvidenceIntegrity) || !reflect.DeepEqual(got, TypedInput{}) {
				t.Fatalf("ASSERT_METHOD_ADMISSION_COMMITMENT_REJECTED_ATOMICALLY got=%+v err=%v", got, err)
			}
		})
	}
}

func TestMethodProjectionAdmissionParallelOccurrencesMapOneToOneWithExactMultiplicity(t *testing.T) {
	admitted, err := AdmitUntrustedMethodEvidenceEnvelope(parallelMethodEnvelope())
	if err != nil {
		t.Fatal(err)
	}
	in, err := PrepareTypedInput(TypedInputRequest{Methods: admitted, EnableMethods: true, Kinds: []RelationKind{TypedReferencesSymbol}})
	if err != nil {
		t.Fatal(err)
	}
	projection, err := PreparePrivateLeidenProjection(in)
	if err != nil {
		t.Fatal(err)
	}
	methods := projection.MethodEvidence()
	if len(methods) != 3 || len(projection.Occurrences()) != 3 {
		t.Fatalf("ASSERT_PARALLEL_METHOD_ONE_TO_ONE_COUNTS methods=%d occurrences=%d", len(methods), len(projection.Occurrences()))
	}
	seen := make(map[string]MethodOccurrenceEvidence, len(methods))
	for _, method := range methods {
		seen[method.Identity] = method
	}
	for _, occurrence := range projection.Occurrences() {
		method, ok := seen[occurrence.Identity]
		if !ok || method.Kind != occurrence.Kind || method.From != occurrence.From || method.To != occurrence.To || method.Ordinal != occurrence.Ordinal {
			t.Fatalf("ASSERT_PARALLEL_METHOD_EXACT_ONE_TO_ONE occurrence=%+v method=%+v ok=%t", occurrence, method, ok)
		}
	}
	pair := TypedPair{From: "file:///parallel/from", To: "file:///parallel/to"}
	if projection.PairWeights()[pair] != 3 {
		t.Fatalf("ASSERT_PARALLEL_METHOD_PAIR_MULTIPLICITY got=%v", projection.PairWeights())
	}
}

func TestMethodProjectionAdmissionRejectsOccurrenceDeletionDuplicationSubstitutionAndNoncanonicalPermutation(t *testing.T) {
	admitted, err := AdmitUntrustedMethodEvidenceEnvelope(parallelMethodEnvelope())
	if err != nil {
		t.Fatal(err)
	}
	base, err := PrepareTypedInput(TypedInputRequest{Methods: admitted, EnableMethods: true, Kinds: []RelationKind{TypedReferencesSymbol}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*TypedInput)
	}{
		{"deletion", func(in *TypedInput) { in.Occurrences = in.Occurrences[1:] }},
		{"duplication", func(in *TypedInput) { in.Occurrences = append(in.Occurrences, in.Occurrences[0]) }},
		{"substitution", func(in *TypedInput) { in.Occurrences[0].Identity = "parallel-substitute" }},
		{"permutation", func(in *TypedInput) { in.Occurrences[0], in.Occurrences[1] = in.Occurrences[1], in.Occurrences[0] }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mutated := base
			mutated.Occurrences = append([]TypedOccurrence(nil), base.Occurrences...)
			tc.mutate(&mutated)
			got, err := PreparePrivateLeidenProjection(mutated)
			if err == nil || err.Error() != "invalid private typed input integrity" || !reflect.DeepEqual(got, privateLeidenProjection{}) {
				t.Fatalf("ASSERT_METHOD_OCCURRENCE_TAMPER_REJECTED_ATOMICALLY got=%+v err=%v", got, err)
			}
		})
	}
}

func parallelMethodEnvelope() UntrustedMethodEvidenceEnvelope {
	envelope := validUntrustedMethodEvidenceEnvelope()
	base := envelope.Occurrences[0]
	envelope.Occurrences = []MethodOccurrenceEvidence{
		parallelMethodOccurrence(base, "parallel-2", 2, "sha256:"+repeatHex("2")),
		parallelMethodOccurrence(base, "parallel-0", 0, "sha256:"+repeatHex("0")),
		parallelMethodOccurrence(base, "parallel-1", 1, "sha256:"+repeatHex("1")),
	}
	return envelope
}

func parallelMethodOccurrence(base MethodOccurrenceEvidence, identity string, ordinal int, evidenceDigest string) MethodOccurrenceEvidence {
	base.Identity = identity
	base.Ordinal = ordinal
	base.From = "file:///parallel/from"
	base.To = "file:///parallel/to"
	base.QuerySelector = "queries/" + identity + ".json"
	base.EvidenceSelector = "evidence/" + identity + ".json"
	base.EvidenceDigest = evidenceDigest
	return base
}

func repeatHex(value string) string {
	out := ""
	for len(out) < 64 {
		out += value
	}
	return out[:64]
}
