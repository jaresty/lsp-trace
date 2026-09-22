package describeworker

import (
	"bytes"
	"strings"
	"testing"
)

func validSemanticV2() string {
	return `{
  "verdict": "COMPLETE",
  "target_role": "validates a bounded response",
  "consumer_need": {"status": "UNRESOLVED", "value": ""},
  "provided_behavior": {"value": "strictly validates semantic JSON", "consumer_relative": false},
  "boundary_contribution": "keeps model judgment separate from host identity",
  "limitations": ["consumer is unresolved"]
}`
}

func hostV2() ResponseHostV2 {
	return ResponseHostV2{
		RequestRecordID: "request-1", MessageID: "message-1", AttemptID: "attempt-1",
		Consumer:   ConsumerIdentityV2{Resolution: "UNRESOLVED", AlternativeID: "OUTWARD_CONSUMER_UNRESOLVED", AlternativeOrdinal: 0},
		Pins:       HostPinsV2{WorkerSHA256: digestOfV2("worker"), ModelSHA256: digestOfV2("model"), GrammarSHA256: digestOfV2("grammar"), PromptSHA256: digestOfV2("prompt")},
		Provenance: HostProvenanceV2{PacketID: "packet-1", RequestLineageIdentity: "lineage-1"},
		Custody:    HostCustodyV2{GraphDigest: digestOfV2("graph"), CaptureID: "capture-1"},
	}
}

func TestSemanticResponseV2StrictCanonicalization(t *testing.T) {
	_, canonical, err := ParseSemanticResponseV2([]byte(validSemanticV2()))
	if err != nil {
		t.Fatalf("ASSERT_V2_PRETTY_JSON_ACCEPTED: %v", err)
	}
	permuted := `{"limitations":["consumer is unresolved"],"boundary_contribution":"keeps model judgment separate from host identity","provided_behavior":{"consumer_relative":false,"value":"strictly validates semantic JSON"},"consumer_need":{"value":"","status":"UNRESOLVED"},"target_role":"validates a bounded response","verdict":"COMPLETE"}`
	_, canonical2, err := ParseSemanticResponseV2([]byte(permuted))
	if err != nil || !bytes.Equal(canonical, canonical2) {
		t.Fatalf("ASSERT_V2_PERMUTATION_INVARIANT: err=%v\n%s\n%s", err, canonical, canonical2)
	}
	validSuggestions := strings.Replace(validSemanticV2(), `"limitations":`, `"citation_suggestions":{"target_role":["C1"],"consumer_need":["UNRESOLVED_CUSTODY"],"provided_behavior":["C1"],"boundary_contribution":["C1"],"limitations":["PACKET_SCOPE"]},"limitations":`, 1)
	if _, _, err := ParseSemanticResponseV2([]byte(validSuggestions)); err != nil {
		t.Fatalf("ASSERT_V2_OPTIONAL_SUGGESTIONS_ACCEPTED: %v", err)
	}
	for name, raw := range map[string]string{
		"duplicate":            strings.Replace(validSemanticV2(), `"verdict":`, `"verdict":"COMPLETE","verdict":`, 1),
		"unknown":              strings.Replace(validSemanticV2(), `"verdict":`, `"nearest_outward_consumer":"invented","verdict":`, 1),
		"legacy citations":     strings.Replace(validSemanticV2(), `"verdict":`, `"citations":["C1"],"verdict":`, 1),
		"unknown suggestion":   strings.Replace(validSuggestions, `["C1"]`, `["INVENTED"]`, 1),
		"duplicate suggestion": strings.Replace(validSuggestions, `["C1"]`, `["C1","C1"]`, 1),
		"null suggestions":     strings.Replace(validSemanticV2(), `"limitations":`, `"citation_suggestions":null,"limitations":`, 1),
		"trailing":             validSemanticV2() + `{}`,
		"fence":                "```json\n" + validSemanticV2() + "\n```",
	} {
		if _, _, err := ParseSemanticResponseV2([]byte(raw)); err == nil {
			t.Errorf("ASSERT_V2_REJECT_%s", name)
		}
	}
}

func TestResponseRecordV2HostEnrichmentAndRawCustody(t *testing.T) {
	host := hostV2()
	a, err := NewResponseRecordV2([]byte(validSemanticV2()), host)
	if err != nil {
		t.Fatal(err)
	}
	compact := strings.ReplaceAll(strings.ReplaceAll(validSemanticV2(), "\n", ""), "  ", "")
	b, err := NewResponseRecordV2([]byte(compact), host)
	if err != nil {
		t.Fatal(err)
	}
	if a.CanonicalSemanticDigest() != b.CanonicalSemanticDigest() || a.ID() == b.ID() || a.RawStdoutDigest() == b.RawStdoutDigest() {
		t.Fatal("ASSERT_V2_RAW_DIGEST_DISTINCT_CANONICAL_EQUAL")
	}
	if a.RawStdoutLength() != len(validSemanticV2()) || a.Consumer().AlternativeID != "OUTWARD_CONSUMER_UNRESOLVED" {
		t.Fatal("ASSERT_V2_RAW_LENGTH_AND_UNRESOLVED_CONSUMER")
	}
	basis := a.AdmissibleEvidenceBasis()
	if basis.Label != "admissible_evidence_basis" || strings.Join(basis.TargetRole, ",") != "C1" || strings.Join(basis.ConsumerNeed, ",") != "UNRESOLVED_CUSTODY" || strings.Join(basis.ProvidedBehavior, ",") != "C1" || strings.Join(basis.BoundaryContribution, ",") != "C1" || strings.Join(basis.Limitations, ",") != "PACKET_SCOPE" {
		t.Fatalf("ASSERT_V2_HOST_BASIS_MAPPING_NOT_PROOF: %+v", basis)
	}
	raw, err := a.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(validSemanticV2())) {
		t.Fatal("ASSERT_V2_RAW_BODY_NOT_RETAINED")
	}
	parsed, err := ParseResponseRecordV2(raw)
	if err != nil || parsed.ID() != a.ID() {
		t.Fatalf("ASSERT_V2_EXACT_REPLAY: %v", err)
	}
	withSuggestions := strings.Replace(validSemanticV2(), `"limitations":`, `"citation_suggestions":{"target_role":["C1"],"consumer_need":["UNRESOLVED_CUSTODY"],"provided_behavior":["C1"],"boundary_contribution":["C1"],"limitations":["PACKET_SCOPE"]},"limitations":`, 1)
	suggested, err := NewResponseRecordV2([]byte(withSuggestions), host)
	if err != nil || suggested.CitationSuggestions().Label != "MODEL_ATTRIBUTED_NON_AUTHORITATIVE" || suggested.AdmissibleEvidenceBasis().Label != "admissible_evidence_basis" {
		t.Fatalf("ASSERT_V2_SUGGESTIONS_SEPARATE_NONAUTHORITATIVE: %v %+v", err, suggested.CitationSuggestions())
	}
	substituted := bytes.Replace(raw, []byte("OUTWARD_CONSUMER_UNRESOLVED"), []byte("OUTWARD_CONSUMER_SUBSTITUTED"), 1)
	if _, err := ParseResponseRecordV2(substituted); err == nil {
		t.Fatal("ASSERT_V2_COORDINATED_SUBSTITUTION_REJECTED")
	}
}
