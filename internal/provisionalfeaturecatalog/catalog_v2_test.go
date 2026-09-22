package provisionalfeaturecatalog

import (
	"bytes"
	"strings"
	"testing"

	"lsp-trace/internal/describeworker"
)

func TestCatalogV2EmptyAndMixedPreparationFailureRoundTrip(t *testing.T) {
	empty, err := BuildV2WithPreparationFailures(0, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if empty.Outcome() != OutcomeComplete || empty.Accounting().Total != 0 || empty.Accounting().NominationTotal != 0 {
		t.Fatalf("ASSERT_CATALOG_V2_EMPTY_ACCOUNTING: outcome=%s accounting=%+v", empty.Outcome(), empty.Accounting())
	}
	raw, err := empty.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseV2(raw)
	if err != nil || parsed.ID() != empty.ID() || !bytes.Equal(raw, mustCatalogV2Bytes(t, parsed)) {
		t.Fatalf("ASSERT_CATALOG_V2_EMPTY_STRICT_ROUND_TRIP: err=%v", err)
	}
	if _, err := ParseV2(append(raw, '\n')); err == nil {
		t.Fatal("ASSERT_CATALOG_V2_STRICT_PARSE_REJECTS_NONCANONICAL")
	}

	response := catalogV2Response(t)
	failure := PreparationFailureInput{FailureID: "sha256:" + strings.Repeat("b", 64), NominationID: "nomination-failed", PacketIntentID: "sha256:" + strings.Repeat("c", 64), Role: "TARGET", Code: "EXACT_ENDPOINT_SOURCE_UNAVAILABLE", EvidenceIDs: []string{"sha256:" + strings.Repeat("d", 64)}}
	mixed, err := BuildV2WithPreparationFailures(2, []PreparationFailureInput{failure}, []describeworker.ResponseRecordV2{response})
	if err != nil {
		t.Fatal(err)
	}
	accounting := mixed.Accounting()
	if mixed.Outcome() != OutcomeDegraded || accounting.Total != 2 || accounting.Complete != 1 || accounting.PreRequestFailed != 1 || accounting.NominationTotal != 2 || len(mixed.PreparationFailures()) != 1 {
		t.Fatalf("ASSERT_CATALOG_V2_MIXED_ACCOUNTING: outcome=%s accounting=%+v failures=%+v", mixed.Outcome(), accounting, mixed.PreparationFailures())
	}
}

func mustCatalogV2Bytes(t *testing.T, catalog CatalogV2) []byte {
	t.Helper()
	raw, err := catalog.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func catalogV2Response(t *testing.T) describeworker.ResponseRecordV2 {
	t.Helper()
	digest := "sha256:" + strings.Repeat("a", 64)
	host := describeworker.ResponseHostV2{RequestRecordID: "request", MessageID: "message", AttemptID: "attempt", Consumer: describeworker.ConsumerIdentityV2{Resolution: describeworker.ConsumerUnresolvedV2, AlternativeID: "OUTWARD_CONSUMER_UNRESOLVED"}, Pins: describeworker.HostPinsV2{WorkerSHA256: digest, ModelSHA256: digest, GrammarSHA256: digest, PromptSHA256: digest}, Provenance: describeworker.HostProvenanceV2{PacketID: "packet", RequestLineageIdentity: "lineage"}, Custody: describeworker.HostCustodyV2{GraphDigest: digest, CaptureID: "capture"}}
	response, err := describeworker.NewResponseRecordV2([]byte(`{"verdict":"COMPLETE","target_role":"target","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"behavior","consumer_relative":false},"boundary_contribution":"boundary","limitations":[]}`), host)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func TestCatalogV2UsesHostConsumerAndRendersBasisAndSuggestions(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	host := describeworker.ResponseHostV2{RequestRecordID: "request", MessageID: "message", AttemptID: "attempt", Consumer: describeworker.ConsumerIdentityV2{Resolution: describeworker.ConsumerUnresolvedV2, AlternativeID: "OUTWARD_CONSUMER_UNRESOLVED"}, Pins: describeworker.HostPinsV2{WorkerSHA256: digest, ModelSHA256: digest, GrammarSHA256: digest, PromptSHA256: digest}, Provenance: describeworker.HostProvenanceV2{PacketID: "packet", RequestLineageIdentity: "lineage"}, Custody: describeworker.HostCustodyV2{GraphDigest: digest, CaptureID: "capture"}}
	semantic := `{"verdict":"COMPLETE","target_role":"target","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"behavior","consumer_relative":false},"boundary_contribution":"boundary","limitations":[],"citation_suggestions":{"target_role":["C1"],"consumer_need":["UNRESOLVED_CUSTODY"],"provided_behavior":["C1"],"boundary_contribution":["C1"],"limitations":["PACKET_SCOPE"]}}`
	response, err := describeworker.NewResponseRecordV2([]byte(semantic), host)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := BuildV2([]describeworker.ResponseRecordV2{response})
	if err != nil {
		t.Fatal(err)
	}
	review, err := RenderReviewV2(catalog)
	if err != nil || !strings.HasPrefix(review, "EXPERIMENTAL PROVISIONAL — semantic richness not qualified\n") || strings.Contains(strings.ToLower(review), "semantic winner") {
		t.Fatalf("ASSERT_V2_EXPERIMENTAL_PROVISIONAL_WARNING: err=%v review=%q", err, review)
	}
	for _, want := range []string{"OUTWARD_CONSUMER_UNRESOLVED", "admissible_evidence_basis", "admissibility only; not proof", "MODEL_ATTRIBUTED_NON_AUTHORITATIVE", "not support"} {
		if !strings.Contains(review, want) {
			t.Fatalf("ASSERT_V2_VERSION_AWARE_CATALOG_RENDERING missing %q", want)
		}
	}
}
