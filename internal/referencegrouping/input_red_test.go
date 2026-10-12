package referencegrouping

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/programccompose"
)

const (
	wantFamily  = "REFERENCE_GROUPING_INPUT"
	wantVersion = "v1"
	wantClaim   = "STRUCTURAL_OCCURRENCE_AND_CUSTODY_COMMITMENTS_ONLY;AUTHORITY_0;ACCEPTED_FALSE;COMPLETENESS_UNKNOWN;NO_PRODUCER_AUTHENTICATION;NO_POLICY_QUALIFICATION;NO_FEATURE_ACCEPTANCE"
)

func TestReferenceGroupingInputIndependentGoldenAndRoundTrip(t *testing.T) {
	f := referenceFixture(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol})
	artifact, err := BuildInput(f.request())
	if err != nil {
		t.Fatal(err)
	}
	// This literal is deliberately independent of BuildInput and any production
	// canonicalizer. Changing the producer cannot update this oracle implicitly.
	const want = "{\"family\":\"REFERENCE_GROUPING_INPUT\",\"version\":\"v1\",\"claim_ceiling\":\"STRUCTURAL_OCCURRENCE_AND_CUSTODY_COMMITMENTS_ONLY;AUTHORITY_0;ACCEPTED_FALSE;COMPLETENESS_UNKNOWN;NO_PRODUCER_AUTHENTICATION;NO_POLICY_QUALIFICATION;NO_FEATURE_ACCEPTANCE\",\"authority\":0,\"accepted\":false,\"completeness\":\"UNKNOWN\",\"occurrence_admission_policy_id\":\"occurrence-admission-v1\",\"composition_policy_id\":\"composition-identity-v1\",\"privacy_policy\":\"private-test-v1\",\"eligible_kinds\":[\"CALLS\",\"REFERENCES_SYMBOL\",\"RESOLVES_TO_DEFINITION\"],\"resource_limits\":{\"max_artifacts\":16,\"max_nodes\":32,\"max_occurrences\":32,\"max_contributions\":32,\"max_canonical_bytes\":1048576,\"max_string_bytes\":4096},\"predecessors\":[{\"role\":\"METHOD_EVIDENCE\",\"selector\":\"methods/evidence.json\",\"schema\":\"lsp-trace.private.program-c-method-evidence.v0\",\"byte_length\":2782,\"digest\":\"sha256:e9c9d00f036a2e1919d2548e38a6139eb0d4b7c40ca8d808cd09e127164286ba\"},{\"role\":\"METHOD_MAPPING_RECEIPT\",\"selector\":\"methods/mapping-receipt.json\",\"schema\":\"lsp-trace.private.program-c-method-mapping-receipt.v1\",\"byte_length\":657,\"digest\":\"sha256:8fd290ce5777bbc5481c0b016c9fada26d05284e68d531a0e8f99e28fec6d55b\"},{\"role\":\"METHOD_QUERY\",\"selector\":\"methods/query.json\",\"schema\":\"lsp-trace.private.program-c-method-query.v0\",\"byte_length\":15,\"digest\":\"sha256:162f050b51a53ff51f80243091b7a0e122f6aca22437417e9a34fdb4d4d0a39d\"},{\"role\":\"OCCURRENCE_LEDGER\",\"selector\":\"ledgers/occurrences.json\",\"schema\":\"lsp-trace.private.program-c-occurrence-ledger.v1\",\"byte_length\":578,\"digest\":\"sha256:8081acb156f9370528a9b47295467cfa5e7a108d255580c39f6049aefb1e8aa2\"}],\"endpoint_custody\":[{\"endpoint\":\"0239bf21c0a0a5531d45b74853babd1fcf38b358f3887382b9229e130ddb7206\",\"occurrence_identity\":\"reference-0\",\"endpoint_role\":\"REFERENCED_SYMBOL\",\"predecessor_selector\":\"methods/evidence.json\",\"custody\":\"CALLER_ASSERTED\"},{\"endpoint\":\"cc7c1e5aa55dafb3779b2bfdef0aa3dbc9687ea11214e42af36124ed598bf8cc\",\"occurrence_identity\":\"reference-0\",\"endpoint_role\":\"REFERENCING_OCCURRENCE\",\"predecessor_selector\":\"methods/evidence.json\",\"custody\":\"CALLER_ASSERTED\"},{\"endpoint\":\"cc7c1e5aa55dafb3779b2bfdef0aa3dbc9687ea11214e42af36124ed598bf8cc\",\"occurrence_identity\":\"reference-loop-1\",\"endpoint_role\":\"REFERENCED_SYMBOL\",\"predecessor_selector\":\"methods/evidence.json\",\"custody\":\"CALLER_ASSERTED\"},{\"endpoint\":\"cc7c1e5aa55dafb3779b2bfdef0aa3dbc9687ea11214e42af36124ed598bf8cc\",\"occurrence_identity\":\"reference-loop-1\",\"endpoint_role\":\"REFERENCING_OCCURRENCE\",\"predecessor_selector\":\"methods/evidence.json\",\"custody\":\"CALLER_ASSERTED\"}],\"contributions\":[{\"kind\":\"REFERENCES_SYMBOL\",\"from\":\"cc7c1e5aa55dafb3779b2bfdef0aa3dbc9687ea11214e42af36124ed598bf8cc\",\"to\":\"0239bf21c0a0a5531d45b74853babd1fcf38b358f3887382b9229e130ddb7206\",\"ordinal\":0,\"occurrence_identity\":\"reference-0\",\"method_evidence_selector\":\"methods/evidence.json\",\"query_selector\":\"methods/query.json\",\"evidence_selector\":\"methods/evidence.json\",\"mapping_receipt_selector\":\"methods/mapping-receipt.json\"},{\"kind\":\"REFERENCES_SYMBOL\",\"from\":\"cc7c1e5aa55dafb3779b2bfdef0aa3dbc9687ea11214e42af36124ed598bf8cc\",\"to\":\"cc7c1e5aa55dafb3779b2bfdef0aa3dbc9687ea11214e42af36124ed598bf8cc\",\"ordinal\":1,\"occurrence_identity\":\"reference-loop-1\",\"method_evidence_selector\":\"methods/evidence.json\",\"query_selector\":\"methods/query.json\",\"evidence_selector\":\"methods/evidence.json\",\"mapping_receipt_selector\":\"methods/mapping-receipt.json\"}],\"candidate_outcomes\":[{\"occurrence_identity\":\"reference-0\",\"disposition\":\"ADMITTED\"},{\"occurrence_identity\":\"reference-loop-1\",\"disposition\":\"ADMITTED\"}],\"accounting\":{\"candidates\":2,\"included\":2,\"excluded\":0,\"duplicates\":0,\"terminal\":true,\"truncated\":false}}\n"
	const wantByteLength = 3646
	const wantID = "sha256:454ece618c125f0004998f735d6f232aabafd048924d19ee6ecbeac5e457ff85"
	if len(want) != wantByteLength {
		t.Fatalf("ASSERT_REFERENCE_GROUPING_INPUT_LITERAL_BYTE_LENGTH got=%d want=%d", len(want), wantByteLength)
	}
	if got := artifact.CanonicalBytes(); !bytes.Equal(got, []byte(want)) {
		t.Fatalf("ASSERT_REFERENCE_GROUPING_INPUT_LITERAL_GOLDEN\n got=%s\nwant=%s", got, want)
	}
	if artifact.ID() != wantID || artifact.Digest() != wantID {
		t.Fatalf("ASSERT_REFERENCE_GROUPING_INPUT_LITERAL_ID_DIGEST id=%q digest=%q", artifact.ID(), artifact.Digest())
	}
	parsed, err := ParseInput([]byte(want), generousParseLimits())
	if err != nil || !bytes.Equal(parsed.CanonicalBytes(), []byte(want)) {
		t.Fatalf("ASSERT_REFERENCE_GROUPING_INPUT_LITERAL_STRUCTURAL_PARSE parsed=%+v err=%v", parsed, err)
	}
	admitted, err := AdmitInput(parsed, f.predecessors)
	if err != nil || !bytes.Equal(admitted.CanonicalBytes(), []byte(want)) || admitted.ID() != wantID || admitted.Digest() != wantID {
		t.Fatalf("ASSERT_REFERENCE_GROUPING_INPUT_LITERAL_ADMISSION admitted=%+v err=%v", admitted, err)
	}
	independent := sha256.Sum256([]byte(want))
	if got := fmt.Sprintf("sha256:%x", independent); got != wantID {
		t.Fatalf("ASSERT_REFERENCE_GROUPING_INPUT_LITERAL_INDEPENDENT_DIGEST got=%q want=%q", got, wantID)
	}
}

func TestReferenceGroupingInputExactKindsMultiplicityOrdinalsLoopsAndBindings(t *testing.T) {
	for _, tc := range []struct {
		name  string
		kinds []programcadmission.RelationKind
		want  []Contribution
	}{
		{"calls-only", []programcadmission.RelationKind{programcadmission.TypedCalls}, nil},
		{"definitions-only", []programcadmission.RelationKind{programcadmission.TypedResolvesToDefinition}, []Contribution{{Kind: "RESOLVES_TO_DEFINITION", From: referenceNode("node-a", 0).ID, To: referenceNode("node-b", 1).ID, Ordinal: 0, OccurrenceIdentity: "definition-0", MethodEvidenceSelector: "methods/evidence.json", QuerySelector: "methods/query.json", EvidenceSelector: "methods/evidence.json", MappingReceiptSelector: "methods/mapping-receipt.json"}}},
		{"references-only", []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol}, []Contribution{{Kind: "REFERENCES_SYMBOL", From: referenceNode("node-a", 0).ID, To: referenceNode("node-b", 1).ID, Ordinal: 0, OccurrenceIdentity: "reference-0", MethodEvidenceSelector: "methods/evidence.json", QuerySelector: "methods/query.json", EvidenceSelector: "methods/evidence.json", MappingReceiptSelector: "methods/mapping-receipt.json"}, {Kind: "REFERENCES_SYMBOL", From: referenceNode("node-a", 0).ID, To: referenceNode("node-a", 0).ID, Ordinal: 1, OccurrenceIdentity: "reference-loop-1", MethodEvidenceSelector: "methods/evidence.json", QuerySelector: "methods/query.json", EvidenceSelector: "methods/evidence.json", MappingReceiptSelector: "methods/mapping-receipt.json"}}},
		{"mixed-cross-kind", []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := referenceFixture(t, tc.kinds)
			artifact, err := BuildInput(f.request())
			if err != nil {
				t.Fatal(err)
			}
			got := artifact.Contributions()
			if tc.want != nil && !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ASSERT_REFERENCE_GROUPING_EXACT_CONTRIBUTIONS got=%+v want=%+v", got, tc.want)
			}
			if len(got) != len(f.typed.Occurrences) {
				t.Fatalf("ASSERT_REFERENCE_GROUPING_ONE_PER_TYPED_OCCURRENCE got=%d want=%d", len(got), len(f.typed.Occurrences))
			}
			byIdentity := make(map[string]Contribution, len(got))
			for _, c := range got {
				byIdentity[c.OccurrenceIdentity] = c
			}
			for _, occurrence := range f.typed.Occurrences {
				c, ok := byIdentity[occurrence.Identity]
				if !ok || c.Kind != string(occurrence.Kind) || c.From != occurrence.From || c.To != occurrence.To || c.Ordinal != occurrence.Ordinal {
					t.Fatalf("ASSERT_REFERENCE_GROUPING_EXACT_TYPED_OCCURRENCE_IDENTITY occurrence=%+v contributions=%+v", occurrence, got)
				}
			}
			seenKinds := map[string]bool{}
			for _, c := range got {
				seenKinds[c.Kind] = true
				if c.Action != "" || c.Weight != 0 {
					t.Fatalf("ASSERT_REFERENCE_GROUPING_NO_ACTION_OR_WEIGHT contribution=%+v", c)
				}
				if c.From == "node-a" || c.To == "node-b" {
					t.Fatalf("ASSERT_REFERENCE_GROUPING_HASHED_TYPED_ENDPOINT_IDS_NOT_ALIASES contribution=%+v", c)
				}
				if c.Kind == "CALLS" && (c.MethodEvidenceSelector != "" || c.QuerySelector != "" || c.EvidenceSelector != "" || c.MappingReceiptSelector != "" || c.AcquisitionReceiptSelector != "calls/acquisition.json" || c.RetainedCompositeSelector != "calls/composite.json") {
					t.Fatalf("ASSERT_REFERENCE_GROUPING_CALLS_ACQUISITION_AND_COMPOSITE_BINDINGS contribution=%+v", c)
				}
				if c.Kind != "CALLS" && (c.MappingReceiptSelector != "methods/mapping-receipt.json" || c.MappingReceiptSelector == c.EvidenceSelector) {
					t.Fatalf("ASSERT_REFERENCE_GROUPING_DISTINCT_METHOD_MAPPING_RECEIPT contribution=%+v", c)
				}
			}
			for _, kind := range tc.kinds {
				if !seenKinds[string(kind)] {
					t.Fatalf("ASSERT_REFERENCE_GROUPING_CROSS_KIND_PRESERVED missing=%s got=%+v", kind, got)
				}
			}
		})
	}
}

func TestReferenceGroupingInputDistinctCallsWithSameOrdinalRetainOpaqueIdentities(t *testing.T) {
	f := referenceFixture(t, []programcadmission.RelationKind{programcadmission.TypedCalls})
	if len(f.typed.Occurrences) != 2 || f.typed.Occurrences[0].Ordinal != f.typed.Occurrences[1].Ordinal || f.typed.Occurrences[0].RelationID == f.typed.Occurrences[1].RelationID || f.typed.Occurrences[0].Identity == f.typed.Occurrences[1].Identity {
		t.Fatalf("BLOCKED_NOT_RED distinct same-ordinal CALLS fixture=%+v", f.typed.Occurrences)
	}
	artifact, err := BuildInput(f.request())
	if err != nil {
		t.Fatal(err)
	}
	got := artifact.Contributions()
	if len(got) != 2 || got[0].OccurrenceIdentity == got[1].OccurrenceIdentity {
		t.Fatalf("ASSERT_REFERENCE_GROUPING_SAME_ORDINAL_CALLS_MULTIPLICITY got=%+v", got)
	}
	want := map[string]bool{f.typed.Occurrences[0].Identity: true, f.typed.Occurrences[1].Identity: true}
	for _, contribution := range got {
		if !want[contribution.OccurrenceIdentity] {
			t.Fatalf("ASSERT_REFERENCE_GROUPING_CALLS_OPAQUE_IDENTITY got=%q want=%v", contribution.OccurrenceIdentity, want)
		}
	}
}

func TestReferenceGroupingInputCanonicalPermutationStabilityAndSourceBytesSensitivity(t *testing.T) {
	f := referenceFixture(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition})
	a, err := BuildInput(f.request())
	if err != nil {
		t.Fatal(err)
	}
	permuted := f.request()
	reverse(permuted.Predecessors)
	reverse(permuted.EndpointCustody)
	reverse(permuted.CandidateOutcomes)
	b, err := BuildInput(permuted)
	if err != nil || a.ID() != b.ID() || !bytes.Equal(a.CanonicalBytes(), b.CanonicalBytes()) {
		t.Fatalf("ASSERT_REFERENCE_GROUPING_CANONICAL_PERMUTATION_STABILITY a=%q b=%q err=%v", a.ID(), b.ID(), err)
	}
	changed := f.request()
	changedIndex := -1
	for i := range changed.Predecessors {
		if changed.Predecessors[i].Role == "METHOD_QUERY" {
			changedIndex = i
			break
		}
	}
	if changedIndex < 0 {
		t.Fatal("BLOCKED_NOT_RED method query predecessor absent")
	}
	changed.Predecessors[changedIndex].Bytes = append([]byte(nil), changed.Predecessors[changedIndex].Bytes...)
	changed.Predecessors[changedIndex].Bytes[0] ^= 1
	changed.Predecessors[changedIndex].ClaimedDigest = a.Digest() // must be ignored
	if got, err := BuildInput(changed); err == nil || !reflect.DeepEqual(got, Artifact{}) {
		t.Fatalf("ASSERT_REFERENCE_GROUPING_METHOD_DIGEST_BINDS_EXACT_BYTES got=%+v err=%v", got, err)
	}
	identityChanged := f.request()
	for i := range identityChanged.Predecessors {
		if identityChanged.Predecessors[i].Role == "METHOD_MAPPING_RECEIPT" {
			identityChanged.Predecessors[i].Bytes = append([]byte(nil), identityChanged.Predecessors[i].Bytes...)
			identityChanged.Predecessors[i].Bytes[0] ^= 1
			break
		}
	}
	if got, err := BuildInput(identityChanged); err == nil || !reflect.DeepEqual(got, Artifact{}) {
		t.Fatalf("ASSERT_REFERENCE_GROUPING_BUILD_REJECTS_MAPPING_RECEIPT_SUBSTITUTION got=%+v err=%v", got, err)
	}
	ledgerChanged := f.request()
	ledgerChanged.OccurrenceLedgerBytes = append([]byte(nil), ledgerChanged.OccurrenceLedgerBytes...)
	ledgerChanged.OccurrenceLedgerBytes[0] ^= 1
	if got, err := BuildInput(ledgerChanged); err == nil || !reflect.DeepEqual(got, Artifact{}) {
		t.Fatalf("ASSERT_REFERENCE_GROUPING_LEDGER_BYTES_EXACT_MATCH got=%+v err=%v", got, err)
	}
}

func TestReferenceGroupingInputMixedBuildParseAdmitRoundTrip(t *testing.T) {
	f := referenceFixture(t, []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition})
	artifact, err := BuildInput(f.request())
	if err != nil {
		t.Fatal(err)
	}
	admitted, err := ParseAndAdmit(artifact.CanonicalBytes(), f.predecessors, generousParseLimits())
	if err != nil || admitted.ID() != artifact.ID() || !bytes.Equal(admitted.CanonicalBytes(), artifact.CanonicalBytes()) {
		t.Fatalf("ASSERT_REFERENCE_GROUPING_MIXED_BUILD_PARSE_ADMIT_ROUNDTRIP artifact=%q admitted=%q err=%v", artifact.ID(), admitted.ID(), err)
	}
}

func TestReferenceGroupingInputParseRejectsEveryNoncanonicalArrayPermutation(t *testing.T) {
	f := referenceFixture(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition})
	artifact, err := BuildInput(f.request())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*document)
	}{
		{"predecessors", func(d *document) { reverse(d.Predecessors) }},
		{"custody", func(d *document) { reverse(d.EndpointCustody) }},
		{"contributions", func(d *document) { reverse(d.Contributions) }},
		{"outcomes", func(d *document) { reverse(d.CandidateOutcomes) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := artifact.doc
			tc.mutate(&doc)
			noncanonical, err := artifactFromDocument(doc, 1<<20)
			if err != nil || bytes.Equal(noncanonical.CanonicalBytes(), artifact.CanonicalBytes()) {
				t.Fatalf("BLOCKED_NOT_RED valid array permutation name=%s err=%v", tc.name, err)
			}
			if got, err := ParseInput(noncanonical.CanonicalBytes(), generousParseLimits()); err == nil || !reflect.DeepEqual(got, Artifact{}) {
				t.Fatalf("ASSERT_REFERENCE_GROUPING_PARSE_REJECTS_NONCANONICAL_%s got=%+v err=%v", tc.name, got, err)
			}
		})
	}
}

func TestReferenceGroupingInputStrictParserMutationCorpus(t *testing.T) {
	f := referenceFixture(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol})
	artifact, err := BuildInput(f.request())
	if err != nil {
		t.Fatal(err)
	}
	raw := artifact.CanonicalBytes()
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatal(err)
	}
	marshal := func(m map[string]any) []byte { b, _ := json.Marshal(m); return append(b, '\n') }
	clone := func() map[string]any { var m map[string]any; _ = json.Unmarshal(raw, &m); return m }
	mutations := map[string]func() []byte{
		"unknown-top":    func() []byte { m := clone(); m["unknown"] = true; return marshal(m) },
		"unknown-nested": func() []byte { m := clone(); m["accounting"].(map[string]any)["unknown"] = true; return marshal(m) },
		"reordered-full-object": func() []byte {
			body := bytes.TrimSuffix(raw, []byte("\n"))
			prefix := []byte(`{"family":"REFERENCE_GROUPING_INPUT","version":"v1",`)
			if !bytes.HasPrefix(body, prefix) {
				t.Fatal("fixture is not in expected canonical order")
			}
			return append(append([]byte(`{"version":"v1","family":"REFERENCE_GROUPING_INPUT",`), body[len(prefix):]...), '\n')
		},
		"duplicate-top-key": func() []byte {
			return bytes.Replace(raw, []byte(`{"family":`), []byte(`{"family":"REFERENCE_GROUPING_INPUT","family":`), 1)
		},
		"duplicate-accounting-key": func() []byte {
			return bytes.Replace(raw, []byte("\"terminal\":true"), []byte("\"terminal\":true,\"terminal\":true"), 1)
		},
		"duplicate-predecessor-key": func() []byte {
			return bytes.Replace(raw, []byte(`"role":"METHOD_EVIDENCE"`), []byte(`"role":"METHOD_EVIDENCE","role":"METHOD_EVIDENCE"`), 1)
		},
		"duplicate-custody-key": func() []byte {
			return bytes.Replace(raw, []byte(`"endpoint":`), []byte(`"endpoint":"duplicate","endpoint":`), 1)
		},
		"duplicate-contribution-key": func() []byte {
			return bytes.Replace(raw, []byte(`"kind":"REFERENCES_SYMBOL"`), []byte(`"kind":"REFERENCES_SYMBOL","kind":"REFERENCES_SYMBOL"`), 1)
		},
		"trailing-value": func() []byte { return append(append([]byte(nil), raw...), []byte("{}\n")...) },
		"no-lf":          func() []byte { return append([]byte(nil), raw[:len(raw)-1]...) },
		"extra-lf":       func() []byte { return append(append([]byte(nil), raw...), '\n') },
		"crlf":           func() []byte { return append(append([]byte(nil), raw[:len(raw)-1]...), '\r', '\n') },
		"family":         func() []byte { return bytes.Replace(raw, []byte(wantFamily), []byte("OTHER_GROUPING_INPUT"), 1) },
		"version":        func() []byte { return bytes.Replace(raw, []byte(wantVersion), []byte("v2"), 1) },
		"kind":           func() []byte { return bytes.Replace(raw, []byte("REFERENCES_SYMBOL"), []byte("UNKNOWN_KIND"), 1) },
		"missing-selector": func() []byte {
			m := clone()
			delete(m["predecessors"].([]any)[0].(map[string]any), "selector")
			return marshal(m)
		},
		"missing-custody": func() []byte {
			m := clone()
			delete(m["endpoint_custody"].([]any)[0].(map[string]any), "custody")
			return marshal(m)
		},
		"missing-endpoint": func() []byte {
			m := clone()
			delete(m["contributions"].([]any)[0].(map[string]any), "to")
			return marshal(m)
		},
		"duplicate-contribution": func() []byte {
			m := clone()
			a := m["contributions"].([]any)
			m["contributions"] = append(a, a[0])
			return marshal(m)
		},
		"cross-kind-substitution": func() []byte {
			return bytes.Replace(raw, []byte("REFERENCES_SYMBOL"), []byte("RESOLVES_TO_DEFINITION"), 1)
		},
		"source-digest-substitution": func() []byte {
			m := clone()
			m["predecessors"].([]any)[0].(map[string]any)["digest"] = "sha256:" + strings.Repeat("0", 64)
			return marshal(m)
		},
		"source-length-substitution": func() []byte {
			m := clone()
			m["predecessors"].([]any)[0].(map[string]any)["byte_length"] = float64(999)
			return marshal(m)
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			if got, err := ParseAndAdmit(mutate(), f.predecessors, generousParseLimits()); err == nil || !reflect.DeepEqual(got, AdmittedInput{}) {
				t.Fatalf("ASSERT_REFERENCE_GROUPING_STRICT_%s got=%+v err=%v", name, got, err)
			}
		})
	}
}

func TestReferenceGroupingInputAccountingAndClaimCeiling(t *testing.T) {
	f := referenceFixture(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol})
	artifact, err := BuildInput(f.request())
	if err != nil {
		t.Fatal(err)
	}
	got := artifact.Accounting()
	want := Accounting{Candidates: len(f.typed.Occurrences), Included: len(f.typed.Occurrences), Excluded: 0, Duplicates: 0, Terminal: true, Truncated: false}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ASSERT_REFERENCE_GROUPING_FULL_TERMINAL_ACCOUNTING got=%+v want=%+v", got, want)
	}
	if artifact.Family() != wantFamily || artifact.Version() != wantVersion || artifact.ClaimCeiling() != wantClaim || artifact.Authority() != 0 || artifact.Accepted() || artifact.Completeness() != "UNKNOWN" {
		t.Fatalf("ASSERT_REFERENCE_GROUPING_CONSERVATIVE_CLAIMS artifact=%+v", artifact)
	}
	for _, bad := range []Accounting{
		{Candidates: 2, Included: 1, Excluded: 0, Terminal: true},
		{Candidates: 1, Included: 1, Excluded: 0, Duplicates: 1, Terminal: true},
		{Candidates: 1, Included: 0, Excluded: 1, Terminal: true},
		{Candidates: 1, Included: 1, Terminal: false},
	} {
		req := f.request()
		req.Accounting = bad
		if got, err := BuildInput(req); err == nil || !reflect.DeepEqual(got, Artifact{}) {
			t.Fatalf("ASSERT_REFERENCE_GROUPING_ACCOUNTING_IMBALANCE_REJECTED bad=%+v got=%+v err=%v", bad, got, err)
		}
	}
}

func TestReferenceGroupingInputBoundsExactAndPlusOne(t *testing.T) {
	f := referenceFixture(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol})
	base := f.request()
	artifact, err := BuildInput(base)
	if err != nil {
		t.Fatal(err)
	}
	raw := artifact.CanonicalBytes()
	limits := generousParseLimits()
	limits.MaxRawBytes = len(raw)
	limits.MaxArtifacts = len(base.Predecessors)
	limits.MaxNodes = len(base.EndpointCustody)
	limits.MaxOccurrences = len(f.typed.Occurrences)
	limits.MaxContributions = len(f.typed.Occurrences)
	limits.MaxCanonicalBytes = len(raw)
	limits.MaxStringBytes = maxStringBytes(raw)
	if _, err := ParseAndAdmit(raw, f.predecessors, limits); err != nil {
		t.Fatalf("ASSERT_REFERENCE_GROUPING_LIMITS_EXACT_ACCEPTED err=%v", err)
	}
	for _, tc := range []struct {
		name string
		cut  func(*ParseLimits)
	}{
		{"raw", func(l *ParseLimits) { l.MaxRawBytes-- }},
		{"artifacts", func(l *ParseLimits) { l.MaxArtifacts-- }},
		{"nodes", func(l *ParseLimits) { l.MaxNodes-- }},
		{"occurrences", func(l *ParseLimits) { l.MaxOccurrences-- }},
		{"contributions", func(l *ParseLimits) { l.MaxContributions-- }},
		{"canonical", func(l *ParseLimits) { l.MaxCanonicalBytes-- }},
		{"strings", func(l *ParseLimits) { l.MaxStringBytes-- }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tooSmall := limits
			tc.cut(&tooSmall)
			if got, err := ParseAndAdmit(raw, f.predecessors, tooSmall); err == nil || !reflect.DeepEqual(got, AdmittedInput{}) {
				t.Fatalf("ASSERT_REFERENCE_GROUPING_LIMIT_PLUS_ONE_REJECTED got=%+v err=%v", got, err)
			}
		})
	}
	for _, extreme := range []string{"18446744073709551616", "9223372036854775808", "1e1000000"} {
		bad := bytes.Replace(raw, []byte("\"candidates\":2"), []byte("\"candidates\":"+extreme), 1)
		if got, err := ParseAndAdmit(bad, f.predecessors, generousParseLimits()); err == nil || !reflect.DeepEqual(got, AdmittedInput{}) {
			t.Fatalf("ASSERT_REFERENCE_GROUPING_EXTREME_INTEGER_ATOMIC_ZERO value=%s got=%+v err=%v", extreme, got, err)
		}
	}
}

func TestReferenceGroupingInputEmbeddedLimitsAreAuthoritative(t *testing.T) {
	f := referenceFixture(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol})
	base := f.request()
	for _, tc := range []struct {
		name   string
		mutate func(*InputRequest)
	}{
		{"nodes", func(r *InputRequest) { r.Limits.MaxNodes = 1 }},
		{"strings", func(r *InputRequest) { r.Limits.MaxStringBytes = 1 }},
		{"contributions", func(r *InputRequest) { r.Limits.MaxContributions = 1 }},
		{"occurrences", func(r *InputRequest) { r.Limits.MaxOccurrences = 1 }},
	} {
		t.Run("build-"+tc.name, func(t *testing.T) {
			req := base
			tc.mutate(&req)
			if got, err := BuildInput(req); err == nil || !reflect.DeepEqual(got, Artifact{}) {
				t.Fatalf("ASSERT_REFERENCE_GROUPING_BUILD_EMBEDDED_LIMIT_%s got=%+v err=%v", tc.name, got, err)
			}
		})
	}

	artifact, err := BuildInput(base)
	if err != nil {
		t.Fatal(err)
	}
	raw := artifact.CanonicalBytes()
	bad := bytes.Replace(raw, []byte(`"max_contributions":32`), []byte(`"max_contributions":0`), 1)
	if bytes.Equal(raw, bad) {
		t.Fatal("BLOCKED_NOT_RED embedded limit mutation anchor absent")
	}
	if got, err := ParseAndAdmit(bad, f.predecessors, generousParseLimits()); err == nil || !reflect.DeepEqual(got, AdmittedInput{}) {
		t.Fatalf("ASSERT_REFERENCE_GROUPING_PARSED_EMBEDDED_LIMIT_AUTHORITY got=%+v err=%v", got, err)
	}
}

func TestReferenceGroupingInputDefensiveCopiesAndDeterminism(t *testing.T) {
	f := referenceFixture(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol})
	before := append([]byte(nil), f.predecessors[0].Bytes...)
	a, err := BuildInput(f.request())
	if err != nil {
		t.Fatal(err)
	}
	first, second := a.CanonicalBytes(), a.CanonicalBytes()
	first[0] ^= 0xff
	contributions := a.Contributions()
	contributions[0].From = "mutated"
	if bytes.Equal(first, second) || !bytes.Equal(second, a.CanonicalBytes()) || a.Contributions()[0].From == "mutated" || !bytes.Equal(before, f.predecessors[0].Bytes) {
		t.Fatal("ASSERT_REFERENCE_GROUPING_OPAQUE_DEFENSIVE_COPIES")
	}
	b, err := BuildInput(f.request())
	if err != nil || a.ID() != b.ID() || !bytes.Equal(a.CanonicalBytes(), b.CanonicalBytes()) {
		t.Fatalf("ASSERT_REFERENCE_GROUPING_NO_MAP_SERIALIZATION_OR_NONDETERMINISM a=%q b=%q err=%v", a.ID(), b.ID(), err)
	}
	for _, forbidden := range [][]byte{[]byte("pair_weights"), []byte("leiden"), []byte("policy_qualified"), []byte("map[")} {
		if bytes.Contains(bytes.ToLower(a.CanonicalBytes()), forbidden) {
			t.Fatalf("ASSERT_REFERENCE_GROUPING_NO_PAIRWEIGHTS_LEIDEN_OR_POLICY_QUALIFICATION token=%q bytes=%s", forbidden, a.CanonicalBytes())
		}
	}
}

func TestReferenceGroupingInputRejectsMethodPredecessorDigestAndLengthMismatch(t *testing.T) {
	f := referenceFixture(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition})
	for _, role := range []string{"METHOD_QUERY", "METHOD_EVIDENCE"} {
		t.Run(role, func(t *testing.T) {
			req := f.request()
			for i := range req.Predecessors {
				if req.Predecessors[i].Role == role {
					req.Predecessors[i].Bytes = append(req.Predecessors[i].Bytes, ' ')
				}
			}
			if got, err := BuildInput(req); err == nil || !reflect.DeepEqual(got, Artifact{}) {
				t.Fatalf("ASSERT_REFERENCE_GROUPING_METHOD_EXACT_BYTES_%s got=%+v err=%v", role, got, err)
			}
		})
	}
}

func TestReferenceGroupingInputOccurrenceLedgerDerivedOrVerified(t *testing.T) {
	f := referenceFixture(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol})
	req := f.request()
	req.OccurrenceLedgerBytes = nil
	if _, err := BuildInput(req); err != nil {
		t.Fatalf("ASSERT_REFERENCE_GROUPING_LEDGER_DERIVED_WHEN_OMITTED err=%v", err)
	}
	req = f.request()
	fake := []byte("{\"domain\":\"unrelated\",\"version\":\"v1\",\"claim\":\"fake\",\"occurrences\":[]}\n")
	req.OccurrenceLedgerBytes = fake
	for i := range req.Predecessors {
		if req.Predecessors[i].Role == "OCCURRENCE_LEDGER" {
			req.Predecessors[i].Bytes = fake
		}
	}
	if got, err := BuildInput(req); err == nil || !reflect.DeepEqual(got, Artifact{}) {
		t.Fatalf("ASSERT_REFERENCE_GROUPING_UNRELATED_LEDGER_REJECTED got=%+v err=%v", got, err)
	}
}

func TestReferenceGroupingInputAdmissionReplaysCanonicalPredecessors(t *testing.T) {
	f := referenceFixture(t, []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition})
	artifact, err := BuildInput(f.request())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseInput(artifact.CanonicalBytes(), generousParseLimits())
	if err != nil {
		t.Fatal(err)
	}
	admitted, err := AdmitInput(parsed, f.predecessors)
	if err != nil {
		t.Fatalf("ASSERT_REFERENCE_GROUPING_CANONICAL_PREDECESSOR_REPLAY err=%v", err)
	}
	if !reflect.DeepEqual(admitted.Contributions(), artifact.Contributions()) || !reflect.DeepEqual(admitted.EndpointCustody(), artifact.EndpointCustody()) {
		t.Fatalf("ASSERT_REFERENCE_GROUPING_REPLAY_EXACT_OUTPUT admitted=%+v artifact=%+v", admitted, artifact)
	}
}

func TestReferenceGroupingInputAdmissionRejectsCoordinatedArtifactForgery(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*document)
	}{
		{"identity", func(d *document) {
			d.Contributions[0].OccurrenceIdentity += "-forged"
			d.CandidateOutcomes[0].OccurrenceIdentity = d.Contributions[0].OccurrenceIdentity
			for i := range d.EndpointCustody {
				if d.EndpointCustody[i].OccurrenceIdentity == "reference-0" {
					d.EndpointCustody[i].OccurrenceIdentity = d.Contributions[0].OccurrenceIdentity
				}
			}
		}},
		{"kind", func(d *document) {
			d.Contributions[0].Kind = "RESOLVES_TO_DEFINITION"
			for i := range d.EndpointCustody {
				if d.EndpointCustody[i].OccurrenceIdentity == d.Contributions[0].OccurrenceIdentity {
					if d.EndpointCustody[i].EndpointRole == "REFERENCING_OCCURRENCE" {
						d.EndpointCustody[i].EndpointRole = "QUERY_OCCURRENCE"
					} else {
						d.EndpointCustody[i].EndpointRole = "DEFINITION_TARGET"
					}
				}
			}
		}},
		{"from", func(d *document) { d.Contributions[0].From = d.Contributions[0].To }},
		{"to", func(d *document) { d.Contributions[0].To = d.Contributions[0].From }},
		{"ordinal", func(d *document) { d.Contributions[0].Ordinal++ }},
		{"multiplicity-deletion", func(d *document) {
			removed := d.Contributions[len(d.Contributions)-1].OccurrenceIdentity
			d.Contributions = d.Contributions[:len(d.Contributions)-1]
			d.CandidateOutcomes = d.CandidateOutcomes[:len(d.CandidateOutcomes)-1]
			filtered := d.EndpointCustody[:0]
			for _, c := range d.EndpointCustody {
				if c.OccurrenceIdentity != removed {
					filtered = append(filtered, c)
				}
			}
			d.EndpointCustody = filtered
			d.Accounting.Candidates--
			d.Accounting.Included--
		}},
		{"multiplicity-addition", func(d *document) {
			c := d.Contributions[0]
			c.OccurrenceIdentity += "-extra"
			c.Ordinal += 100
			d.Contributions = append(d.Contributions, c)
			d.CandidateOutcomes = append(d.CandidateOutcomes, outcomeJSON{c.OccurrenceIdentity, "ADMITTED"})
			d.Accounting.Candidates++
			d.Accounting.Included++
		}},
		{"mapping-substitution", func(d *document) {
			a, b := d.Contributions[0].OccurrenceIdentity, d.Contributions[1].OccurrenceIdentity
			d.Contributions[0].OccurrenceIdentity, d.Contributions[1].OccurrenceIdentity = b, a
			for i := range d.CandidateOutcomes {
				if d.CandidateOutcomes[i].OccurrenceIdentity == a {
					d.CandidateOutcomes[i].OccurrenceIdentity = b
				} else if d.CandidateOutcomes[i].OccurrenceIdentity == b {
					d.CandidateOutcomes[i].OccurrenceIdentity = a
				}
			}
			for i := range d.EndpointCustody {
				if d.EndpointCustody[i].OccurrenceIdentity == a {
					d.EndpointCustody[i].OccurrenceIdentity = b
				} else if d.EndpointCustody[i].OccurrenceIdentity == b {
					d.EndpointCustody[i].OccurrenceIdentity = a
				}
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := referenceFixture(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol})
			artifact, err := BuildInput(f.request())
			if err != nil {
				t.Fatal(err)
			}
			doc := artifact.doc
			doc.Predecessors = append([]predecessorJSON(nil), artifact.doc.Predecessors...)
			doc.EndpointCustody = append([]custodyJSON(nil), artifact.doc.EndpointCustody...)
			doc.Contributions = append([]contributionJSON(nil), artifact.doc.Contributions...)
			doc.CandidateOutcomes = append([]outcomeJSON(nil), artifact.doc.CandidateOutcomes...)
			tc.mutate(&doc)
			sort.Slice(doc.Contributions, func(i, j int) bool {
				return doc.Contributions[i].OccurrenceIdentity < doc.Contributions[j].OccurrenceIdentity
			})
			sort.Slice(doc.CandidateOutcomes, func(i, j int) bool {
				return doc.CandidateOutcomes[i].OccurrenceIdentity < doc.CandidateOutcomes[j].OccurrenceIdentity
			})
			sort.Slice(doc.EndpointCustody, func(i, j int) bool {
				if doc.EndpointCustody[i].Endpoint != doc.EndpointCustody[j].Endpoint {
					return doc.EndpointCustody[i].Endpoint < doc.EndpointCustody[j].Endpoint
				}
				if doc.EndpointCustody[i].OccurrenceIdentity != doc.EndpointCustody[j].OccurrenceIdentity {
					return doc.EndpointCustody[i].OccurrenceIdentity < doc.EndpointCustody[j].OccurrenceIdentity
				}
				return doc.EndpointCustody[i].EndpointRole < doc.EndpointCustody[j].EndpointRole
			})
			forged, err := artifactFromDocument(doc, 1<<20)
			if err != nil {
				t.Skipf("mutation is structurally invalid before semantic replay: %v", err)
			}
			if got, err := AdmitInput(forged, f.predecessors); err == nil || !reflect.DeepEqual(got, AdmittedInput{}) {
				t.Fatalf("ASSERT_REFERENCE_GROUPING_COORDINATED_FORGERY_REJECTED got=%+v err=%v", got, err)
			}
		})
	}
}

func TestReferenceGroupingInputAdmissionRejectsMethodPredecessorSemanticSubstitution(t *testing.T) {
	f := referenceFixture(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol})
	artifact, err := BuildInput(f.request())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseInput(artifact.CanonicalBytes(), generousParseLimits())
	if err != nil {
		t.Fatal(err)
	}
	supplied := append([]PredecessorRecord(nil), f.predecessors...)
	for i := range supplied {
		if supplied[i].Role == "METHOD_MAPPING_RECEIPT" {
			var receipt methodMappingReceipt
			if err := json.Unmarshal(supplied[i].Bytes, &receipt); err != nil {
				t.Fatal(err)
			}
			receipt.Mappings[0].To = receipt.Mappings[0].From
			supplied[i].Bytes = canonicalMethodMappingReceiptLiteral(t, receipt.Mappings)
			for j := range parsed.doc.Predecessors {
				if parsed.doc.Predecessors[j].Role == supplied[i].Role {
					h := sha256.Sum256(supplied[i].Bytes)
					parsed.doc.Predecessors[j].ByteLength = len(supplied[i].Bytes)
					parsed.doc.Predecessors[j].Digest = fmt.Sprintf("sha256:%x", h)
				}
			}
		}
	}
	forged, err := artifactFromDocument(parsed.doc, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := AdmitInput(forged, supplied); err == nil || !reflect.DeepEqual(got, AdmittedInput{}) {
		t.Fatalf("ASSERT_REFERENCE_GROUPING_METHOD_PREDECESSOR_SEMANTIC_SUBSTITUTION_REJECTED got=%+v err=%v", got, err)
	}
}

func TestReferenceGroupingInputAdmissionRejectsIndependentPredecessorSubstitutions(t *testing.T) {
	for _, kinds := range [][]programcadmission.RelationKind{{programcadmission.TypedCalls}, {programcadmission.TypedReferencesSymbol}, {programcadmission.TypedResolvesToDefinition}} {
		f := referenceFixture(t, kinds)
		artifact, err := BuildInput(f.request())
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := ParseInput(artifact.CanonicalBytes(), generousParseLimits())
		if err != nil {
			t.Fatal(err)
		}
		for i := range f.predecessors {
			supplies := append([]PredecessorRecord(nil), f.predecessors...)
			supplies[i].Bytes = append([]byte(nil), supplies[i].Bytes...)
			supplies[i].Bytes[0] ^= 1
			if got, err := AdmitInput(parsed, supplies); err == nil || !reflect.DeepEqual(got, AdmittedInput{}) {
				t.Fatalf("ASSERT_REFERENCE_GROUPING_INDEPENDENT_PREDECESSOR_SUBSTITUTION role=%s got=%+v err=%v", supplies[i].Role, got, err)
			}
		}
	}
}

func TestReferenceGroupingInputPredecessorRoleSelectorSchemaDigestCausality(t *testing.T) {
	f := referenceFixture(t, []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol})
	base := f.request()
	for _, tc := range []struct {
		name   string
		mutate func(*InputRequest)
	}{
		{"duplicate-role", func(r *InputRequest) { r.Predecessors[1].Role = r.Predecessors[0].Role }},
		{"wrong-role", func(r *InputRequest) { r.Predecessors[0].Role = "METHOD_EVIDENCE" }},
		{"dangling-role", func(r *InputRequest) { r.Predecessors = r.Predecessors[:len(r.Predecessors)-1] }},
		{"duplicate-selector", func(r *InputRequest) { r.Predecessors[1].Selector = r.Predecessors[0].Selector }},
		{"wrong-selector", func(r *InputRequest) { r.Predecessors[0].Selector = "wrong/ledger.json" }},
		{"dangling-selector", func(r *InputRequest) { r.EndpointCustody[0].PredecessorSelector = "missing.json" }},
		{"wrong-schema", func(r *InputRequest) { r.Predecessors[0].Schema = "lsp-trace.private.wrong.v1" }},
		{"wrong-digest", func(r *InputRequest) { r.Predecessors[0].ClaimedDigest = "sha256:" + strings.Repeat("0", 64) }},
		{"duplicate-custody-input", func(r *InputRequest) { r.EndpointCustody = append(r.EndpointCustody, r.EndpointCustody[0]) }},
		{"caller-occurrence-substitution", func(r *InputRequest) { r.EndpointCustody[0].OccurrenceIdentity = "caller-supplied" }},
		{"caller-role-substitution", func(r *InputRequest) { r.EndpointCustody[0].EndpointRole = "REFERENCED_SYMBOL" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := base
			req.Predecessors = append([]PredecessorRecord(nil), base.Predecessors...)
			req.EndpointCustody = append([]EndpointCustody(nil), base.EndpointCustody...)
			tc.mutate(&req)
			if got, err := BuildInput(req); err == nil || !reflect.DeepEqual(got, Artifact{}) {
				t.Fatalf("ASSERT_REFERENCE_GROUPING_PREDECESSOR_CAUSALITY_%s got=%+v err=%v", tc.name, got, err)
			}
		})
	}
}

func TestReferenceGroupingInputPreallocationNumericCorpus(t *testing.T) {
	f := referenceFixture(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol})
	artifact, err := BuildInput(f.request())
	if err != nil {
		t.Fatal(err)
	}
	raw := artifact.CanonicalBytes()
	for name, replacement := range map[string]string{"negative": "-1", "fractional": "1.5", "exponent": "1e2", "oversized": "18446744073709551616"} {
		t.Run(name, func(t *testing.T) {
			bad := bytes.Replace(raw, []byte(`"candidates":2`), []byte(`"candidates":`+replacement), 1)
			if got, err := ParseAndAdmit(bad, f.predecessors, generousParseLimits()); err == nil || !reflect.DeepEqual(got, AdmittedInput{}) {
				t.Fatalf("ASSERT_REFERENCE_GROUPING_PREALLOCATION_NUMBER_ATOMIC_ZERO got=%+v err=%v", got, err)
			}
		})
	}
	for _, tc := range []struct {
		name     string
		field    string
		setLimit func(*ParseLimits)
	}{
		{"predecessors", "predecessors", func(l *ParseLimits) { l.MaxArtifacts = 1 }},
		{"endpoint-custody", "endpoint_custody", func(l *ParseLimits) { l.MaxNodes = 1 }},
		{"contributions", "contributions", func(l *ParseLimits) { l.MaxContributions = 1 }},
		{"candidate-outcomes", "candidate_outcomes", func(l *ParseLimits) { l.MaxOccurrences = 1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			oversizedArray := bytes.Replace(raw, []byte(`"`+tc.field+`":[`), []byte(`"`+tc.field+`":[{},{},`), 1)
			limits := generousParseLimits()
			tc.setLimit(&limits)
			if got, err := ParseAndAdmit(oversizedArray, f.predecessors, limits); err == nil || !reflect.DeepEqual(got, AdmittedInput{}) {
				t.Fatalf("ASSERT_REFERENCE_GROUPING_PER_ARRAY_PREALLOCATION_LIMIT_%s got=%+v err=%v", tc.name, got, err)
			}
		})
	}
	overflow := generousParseLimits()
	overflow.MaxArtifacts = int(^uint(0) >> 1)
	overflow.MaxNodes = int(^uint(0) >> 1)
	if got, err := ParseAndAdmit(raw, f.predecessors, overflow); err == nil || !reflect.DeepEqual(got, AdmittedInput{}) {
		t.Fatalf("ASSERT_REFERENCE_GROUPING_PREALLOCATION_LIMIT_SUM_OVERFLOW got=%+v err=%v", got, err)
	}
}

func TestReferenceGroupingInputMixedCustodyUsesExactRelationFamilyPredecessor(t *testing.T) {
	f := referenceFixture(t, []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition})
	artifact, err := BuildInput(f.request())
	if err != nil {
		t.Fatal(err)
	}
	for _, custody := range artifact.EndpointCustody() {
		switch custody.EndpointRole {
		case "CALLER_OCCURRENCE", "CALLEE_TARGET":
			if custody.PredecessorSelector != "calls/acquisition.json" {
				t.Fatalf("ASSERT_REFERENCE_GROUPING_CALLS_CUSTODY_FAMILY selector=%q custody=%+v", custody.PredecessorSelector, custody)
			}
		case "REFERENCING_OCCURRENCE", "REFERENCED_SYMBOL", "QUERY_OCCURRENCE", "DEFINITION_TARGET":
			if custody.PredecessorSelector != "methods/evidence.json" {
				t.Fatalf("ASSERT_REFERENCE_GROUPING_METHOD_CUSTODY_FAMILY selector=%q custody=%+v", custody.PredecessorSelector, custody)
			}
		default:
			t.Fatalf("ASSERT_REFERENCE_GROUPING_CLOSED_ENDPOINT_ROLE custody=%+v", custody)
		}
	}
}

func TestReferenceGroupingInputAdmissionRejectsCrossFamilySelectorSubstitution(t *testing.T) {
	f := referenceFixture(t, []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol})
	artifact, err := BuildInput(f.request())
	if err != nil {
		t.Fatal(err)
	}
	raw := artifact.CanonicalBytes()
	var decoded struct {
		EndpointCustody []struct {
			Endpoint string `json:"endpoint"`
		} `json:"endpoint_custody"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil || len(decoded.EndpointCustody) == 0 {
		t.Fatalf("BLOCKED_NOT_RED custody endpoint fixture: %v", err)
	}
	firstEndpoint := decoded.EndpointCustody[0].Endpoint
	for name, bad := range map[string][]byte{
		"calls-contribution":  bytes.Replace(raw, []byte(`"acquisition_receipt_selector":"calls/acquisition.json"`), []byte(`"acquisition_receipt_selector":"methods/evidence.json"`), 1),
		"method-contribution": bytes.Replace(raw, []byte(`"method_evidence_selector":"methods/evidence.json"`), []byte(`"method_evidence_selector":"calls/acquisition.json"`), 1),
		"calls-custody":       bytes.Replace(raw, []byte(`"endpoint_role":"CALLER_OCCURRENCE","predecessor_selector":"calls/acquisition.json"`), []byte(`"endpoint_role":"CALLER_OCCURRENCE","predecessor_selector":"methods/evidence.json"`), 1),
		"method-custody":      bytes.Replace(raw, []byte(`"endpoint_role":"REFERENCING_OCCURRENCE","predecessor_selector":"methods/evidence.json"`), []byte(`"endpoint_role":"REFERENCING_OCCURRENCE","predecessor_selector":"calls/acquisition.json"`), 1),
		"custody-endpoint":    bytes.Replace(raw, []byte(`"endpoint":"`+firstEndpoint+`"`), []byte(`"endpoint":"substituted-endpoint"`), 1),
	} {
		t.Run(name, func(t *testing.T) {
			if bytes.Equal(bad, raw) {
				t.Fatalf("BLOCKED_NOT_RED selector mutation anchor absent: %s", name)
			}
			if got, err := ParseAndAdmit(bad, f.predecessors, generousParseLimits()); err == nil || !reflect.DeepEqual(got, AdmittedInput{}) {
				t.Fatalf("ASSERT_REFERENCE_GROUPING_CROSS_FAMILY_SELECTOR_REJECTED got=%+v err=%v", got, err)
			}
		})
	}

	const original, substituted = "methods/evidence.json", "methods/alternate.json"
	bad := bytes.ReplaceAll(raw, []byte(original), []byte(substituted))
	supplies := append([]PredecessorRecord(nil), f.predecessors...)
	for i := range supplies {
		if supplies[i].Selector == original {
			supplies[i].Selector = substituted
		}
	}
	if got, err := ParseAndAdmit(bad, supplies, generousParseLimits()); err == nil || !reflect.DeepEqual(got, AdmittedInput{}) {
		t.Fatalf("ASSERT_REFERENCE_GROUPING_SAME_FAMILY_SELECTOR_CONTRACT_REJECTED got=%+v err=%v", got, err)
	}
}

func TestReferenceGroupingInputEndpointRolesAndClosedCandidateDispositions(t *testing.T) {
	f := referenceFixture(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition})
	artifact, err := BuildInput(f.request())
	if err != nil {
		t.Fatal(err)
	}
	raw := artifact.CanonicalBytes()
	for _, required := range []string{"REFERENCING_OCCURRENCE", "REFERENCED_SYMBOL", "QUERY_OCCURRENCE", "DEFINITION_TARGET", "mapping_receipt_selector", "ADMITTED", "eligible_kinds", "resource_limits", "candidate_outcomes"} {
		if !bytes.Contains(raw, []byte(required)) {
			t.Fatalf("ASSERT_REFERENCE_GROUPING_OCCURRENCE_BOUND_CUSTODY_AND_DISPOSITION missing=%q bytes=%s", required, raw)
		}
	}
	for _, forbidden := range []string{"\"endpoint_role\":\"CALLER\"", "\"endpoint_role\":\"CALLEE\"", "QUERY_SYMBOL", "POLICY_QUALIFIED"} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("ASSERT_REFERENCE_GROUPING_NO_HISTORICAL_ROLE_OR_POLICY_UPGRADE token=%q bytes=%s", forbidden, raw)
		}
	}
}

type referenceTestFixture struct {
	typed        programcadmission.TypedInput
	predecessors []PredecessorRecord
	custody      []EndpointCustody
	ledger       []byte
}

func (f referenceTestFixture) request() InputRequest {
	predecessors := append([]PredecessorRecord(nil), f.predecessors...)
	for i := range predecessors {
		predecessors[i].Bytes = append([]byte(nil), predecessors[i].Bytes...)
	}
	custody := append([]EndpointCustody(nil), f.custody...)
	outcomes := make([]CandidateOutcome, len(f.typed.Occurrences))
	for i, occurrence := range f.typed.Occurrences {
		outcomes[i] = CandidateOutcome{OccurrenceIdentity: occurrence.Identity, Disposition: "ADMITTED"}
	}
	return InputRequest{
		TypedInput:                  f.typed,
		Predecessors:                predecessors,
		OccurrenceLedgerBytes:       append([]byte(nil), f.ledger...),
		EndpointCustody:             custody,
		OccurrenceAdmissionPolicyID: "occurrence-admission-v1",
		CompositionPolicyID:         "composition-identity-v1",
		PrivacyPolicy:               "private-test-v1",
		EligibleKinds:               []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition},
		Limits:                      ResourceLimits{MaxArtifacts: 16, MaxNodes: 32, MaxOccurrences: 32, MaxContributions: 32, MaxCanonicalBytes: 1 << 20, MaxStringBytes: 4096},
		CandidateOutcomes:           outcomes,
		Accounting:                  Accounting{Candidates: len(f.typed.Occurrences), Included: len(f.typed.Occurrences), Excluded: 0, Duplicates: 0, Terminal: true, Truncated: false},
	}
}

func referenceFixture(t *testing.T, kinds []programcadmission.RelationKind) referenceTestFixture {
	t.Helper()
	a, b := referenceNode("node-a", 0), referenceNode("node-b", 1)
	callsNeeded, methodsNeeded := false, false
	for _, kind := range kinds {
		callsNeeded = callsNeeded || kind == programcadmission.TypedCalls
		methodsNeeded = methodsNeeded || kind != programcadmission.TypedCalls
	}
	var calls programcadmission.CompositeProjectionAdmission
	if callsNeeded {
		edges := []graph.Edge{
			{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{End: graph.Position{Character: 1}}}},
			{CallerNodeID: b.ID, CalleeNodeID: a.ID, CallSites: []graph.Range{{Start: graph.Position{Line: 1}, End: graph.Position{Line: 1, Character: 1}}}},
		}
		input := referenceCapture(t, []graph.Node{a, b}, edges)
		empty := referenceCapture(t, nil, nil)
		empty.Identity = "capture-empty"
		composite, err := programccompose.Compose([]programccompose.Input{input, empty})
		if err != nil {
			t.Fatal(err)
		}
		admitted, err := programcadmission.Admit(composite.Bytes)
		if err != nil {
			t.Fatal(err)
		}
		calls = admitted.Admission
	}
	var methods programcadmission.MethodProjectionAdmission
	if methodsNeeded {
		occurrences := []programcadmission.MethodOccurrenceEvidence{
			referenceMethodOccurrence("reference-0", programcadmission.TypedReferencesSymbol, 0, a.ID, b.ID),
			referenceMethodOccurrence("reference-loop-1", programcadmission.TypedReferencesSymbol, 1, a.ID, a.ID),
			referenceMethodOccurrence("definition-0", programcadmission.TypedResolvesToDefinition, 0, a.ID, b.ID),
		}
		envelope := programcadmission.UntrustedMethodEvidenceEnvelope{Family: programcadmission.MethodEvidenceFamily, Version: programcadmission.MethodEvidenceFamily, ClaimCeiling: programcadmission.MethodEvidenceClaimCeiling, Authority: 0, Accepted: false, Completeness: "UNKNOWN", Provider: "gopls", ProviderVersion: "v1", SessionID: "session", Generation: 1, Revision: "commit", Custody: "CALLER_ASSERTED", PositionEncoding: "utf-16", Occurrences: occurrences}
		var err error
		methods, err = programcadmission.AdmitUntrustedMethodEvidenceEnvelope(envelope)
		if err != nil {
			t.Fatal(err)
		}
	}
	typed, err := programcadmission.PrepareTypedInput(programcadmission.TypedInputRequest{Calls: calls, Methods: methods, EnableMethods: methodsNeeded, Kinds: kinds})
	if err != nil {
		t.Fatal(err)
	}
	ledger := canonicalOccurrenceLedgerLiteral(t, typed.Occurrences)
	predecessors := []PredecessorRecord{
		{Role: "OCCURRENCE_LEDGER", Selector: "ledgers/occurrences.json", Schema: "lsp-trace.private.program-c-occurrence-ledger.v1", Bytes: ledger, ClaimedDigest: "sha256:" + strings.Repeat("f", 64)},
	}
	if callsNeeded {
		projection, err := programcadmission.PreparePrivateLeidenProjection(typed)
		if err != nil {
			t.Fatal(err)
		}
		predecessors = append(predecessors, PredecessorRecord{Role: "ACQUISITION_RECEIPT", Selector: "calls/acquisition.json", Schema: "lsp-trace.private.program-c-acquisition-receipt.v1", Bytes: canonicalCompositeSourceBindingLiteral(t, projection.CompositeSourceBinding())}, PredecessorRecord{Role: "RETAINED_COMPOSITE", Selector: "calls/composite.json", Schema: "lsp-trace.private.program-c-composite.v1", Bytes: projection.RetainedCallsCanonicalBytes()})
	}
	if methodsNeeded {
		projection, err := programcadmission.PreparePrivateLeidenProjection(typed)
		if err != nil {
			t.Fatal(err)
		}
		mappings := make([]methodOccurrenceMapping, 0, len(projection.MethodEvidence()))
		for _, occurrence := range projection.MethodEvidence() {
			mappings = append(mappings, methodOccurrenceMapping{Identity: occurrence.Identity, Kind: string(occurrence.Kind), From: occurrence.From, To: occurrence.To, Ordinal: occurrence.Ordinal, QuerySelector: occurrence.QuerySelector, EvidenceSelector: occurrence.EvidenceSelector})
		}
		predecessors = append(predecessors, PredecessorRecord{Role: "METHOD_QUERY", Selector: "methods/query.json", Schema: "lsp-trace.private.program-c-method-query.v0", Bytes: []byte("{\"query\":true}\n")}, PredecessorRecord{Role: "METHOD_EVIDENCE", Selector: "methods/evidence.json", Schema: programcadmission.MethodEvidenceFamily, Bytes: projection.MethodCanonicalBytes()}, PredecessorRecord{Role: "METHOD_MAPPING_RECEIPT", Selector: "methods/mapping-receipt.json", Schema: "lsp-trace.private.program-c-method-mapping-receipt.v1", Bytes: canonicalMethodMappingReceiptLiteral(t, mappings)})
	}
	custody := make([]EndpointCustody, 0, 4)
	if callsNeeded {
		custody = append(custody,
			EndpointCustody{Endpoint: a.ID, PredecessorSelector: "calls/acquisition.json", Custody: "CALLER_ASSERTED"},
			EndpointCustody{Endpoint: b.ID, PredecessorSelector: "calls/acquisition.json", Custody: "CALLER_ASSERTED"},
		)
	}
	if methodsNeeded {
		custody = append(custody,
			EndpointCustody{Endpoint: a.ID, PredecessorSelector: "methods/evidence.json", Custody: "CALLER_ASSERTED"},
			EndpointCustody{Endpoint: b.ID, PredecessorSelector: "methods/evidence.json", Custody: "CALLER_ASSERTED"},
		)
	}
	return referenceTestFixture{typed: typed, predecessors: predecessors, custody: custody, ledger: ledger}
}

func evidenceSelector(methods bool) string {
	if methods {
		return "methods/evidence.json"
	}
	return "calls/acquisition.json"
}
func referenceNode(name string, line uint32) graph.Node {
	r := graph.Range{Start: graph.Position{Line: line}, End: graph.Position{Line: line, Character: 1}}
	return graph.NewNode(graph.Item{Name: name, Kind: 12, URI: "file:///w/a.go", Range: r, SelectionRange: r})
}
func referenceCapture(t *testing.T, nodes []graph.Node, edges []graph.Edge) programccompose.Input {
	t.Helper()
	seed := graph.InvocationSeed{Label: "seed", At: "a.go:1:1", ResolvedURI: "file:///w/a.go", ContentSHA256: "sha256:" + strings.Repeat("a", 64), LanguageID: "go"}
	r := graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{WorkspaceURI: "file:///w", Server: graph.ServerInvocation{Command: "gopls"}, LanguageID: "go", Seeds: []graph.InvocationSeed{seed}, Provenance: graph.InvocationProvenance{InvocationID: "invocation", SourceRevision: "commit", ServerVersion: "gopls@1"}}, Nodes: nodes, Edges: edges, Seeds: []graph.SeedResult{{Label: seed.Label}}, Summary: graph.Summary{Complete: false}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
	native, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := graphprovenance.CaptureV5(native, "session", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable})
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	return programccompose.Input{Bytes: raw, Identity: "capture", SHA256: fmt.Sprintf("sha256:%x", sum), ByteLength: len(raw), ExactMetadata: programccompose.ExactMetadata{WorkspaceIdentity: "workspace", RevisionCustody: "CALLER_ASSERTED", PositionEncoding: "utf-16", AcquisitionSemantics: "managed-lsp-v1", PrivacyPolicy: "private-test-v1"}}
}
func referenceMethodOccurrence(identity string, kind programcadmission.RelationKind, ordinal int, from, to string) programcadmission.MethodOccurrenceEvidence {
	method := map[programcadmission.RelationKind]string{programcadmission.TypedReferencesSymbol: "textDocument/references", programcadmission.TypedResolvesToDefinition: "textDocument/definition"}[kind]
	source := map[programcadmission.RelationKind]string{programcadmission.TypedReferencesSymbol: "REFERENCING_OCCURRENCE", programcadmission.TypedResolvesToDefinition: "QUERY_SYMBOL"}[kind]
	target := map[programcadmission.RelationKind]string{programcadmission.TypedReferencesSymbol: "REFERENCED_SYMBOL", programcadmission.TypedResolvesToDefinition: "DEFINITION_TARGET"}[kind]
	queryBytes := []byte("{\"query\":true}\n")
	evidenceBytes := []byte("{\"evidence\":true}\n")
	queryDigest := sha256.Sum256(queryBytes)
	evidenceDigest := sha256.Sum256(evidenceBytes)
	return programcadmission.MethodOccurrenceEvidence{Identity: identity, Kind: kind, Ordinal: ordinal, From: from, To: to, SourceRole: source, TargetRole: target, Method: method, QuerySelector: "methods/query.json", QueryDigest: fmt.Sprintf("sha256:%x", queryDigest), QueryLength: len(queryBytes), EvidenceSelector: "methods/evidence.json", EvidenceDigest: fmt.Sprintf("sha256:%x", evidenceDigest), EvidenceLength: len(evidenceBytes), Provider: "gopls", ProviderVersion: "v1", SessionID: "session", Generation: 1, Revision: "commit", Custody: "CALLER_ASSERTED", PositionEncoding: "utf-16"}
}
func reverse[T any](values []T) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}

type methodOccurrenceMapping struct {
	Identity         string `json:"identity"`
	Kind             string `json:"kind"`
	From             string `json:"from"`
	To               string `json:"to"`
	Ordinal          int    `json:"ordinal"`
	QuerySelector    string `json:"query_selector"`
	EvidenceSelector string `json:"evidence_selector"`
}

type methodMappingReceipt struct {
	Family   string                    `json:"family"`
	Version  string                    `json:"version"`
	Mappings []methodOccurrenceMapping `json:"mappings"`
}

func canonicalMethodMappingReceiptLiteral(t *testing.T, mappings []methodOccurrenceMapping) []byte {
	t.Helper()
	rows := append([]methodOccurrenceMapping(nil), mappings...)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Identity < rows[j].Identity })
	raw, err := json.Marshal(methodMappingReceipt{Family: "METHOD_MAPPING_RECEIPT", Version: "v1", Mappings: rows})
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

func canonicalOccurrenceLedgerLiteral(t *testing.T, occurrences []programcadmission.TypedOccurrence) []byte {
	t.Helper()
	type ledgerOccurrence struct {
		Identity   string `json:"identity"`
		Kind       string `json:"kind"`
		From       string `json:"from"`
		To         string `json:"to"`
		Ordinal    int    `json:"ordinal"`
		RelationID string `json:"relation_id"`
	}
	type ledgerDocument struct {
		Domain      string             `json:"domain"`
		Version     string             `json:"version"`
		Claim       string             `json:"claim"`
		Occurrences []ledgerOccurrence `json:"occurrences"`
	}
	rows := make([]ledgerOccurrence, len(occurrences))
	for i, occurrence := range occurrences {
		rows[i] = ledgerOccurrence{occurrence.Identity, string(occurrence.Kind), occurrence.From, occurrence.To, occurrence.Ordinal, occurrence.RelationID}
	}
	raw, err := json.Marshal(ledgerDocument{"PROGRAM_C_TYPED_OCCURRENCES", "v1", "ADMITTED_TYPED_OCCURRENCES_EXACT", rows})
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

func canonicalCompositeSourceBindingLiteral(t *testing.T, binding programcadmission.CompositeSourceBinding) []byte {
	t.Helper()
	raw, err := json.Marshal(binding)
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

func generousParseLimits() ParseLimits {
	return ParseLimits{MaxRawBytes: 1 << 20, MaxArtifacts: 64, MaxNodes: 128, MaxOccurrences: 128, MaxContributions: 128, MaxCanonicalBytes: 1 << 20, MaxStringBytes: 4096}
}
func maxStringBytes(raw []byte) int {
	var value any
	_ = json.Unmarshal(raw, &value)
	max := 0
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case string:
			if len(x) > max {
				max = len(x)
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		case map[string]any:
			for k, e := range x {
				if len(k) > max {
					max = len(k)
				}
				walk(e)
			}
		}
	}
	walk(value)
	return max
}
