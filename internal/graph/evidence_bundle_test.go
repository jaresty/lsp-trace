package graph

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestV3EvidenceBundleContract(t *testing.T) {
	r := Result{SchemaVersion: SchemaVersion, Invocation: Invocation{Server: ServerInvocation{Command: "server", Arguments: []string{"--stdio"}}, Limits: Limits{MaxDepth: 2, MaxNodes: 3, TimeoutMS: 4000}}, Summary: Summary{Complete: true}}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["schema_version"] != "lsp-trace.graph.v3" {
		t.Errorf("ASSERT_P1_V3_MAJOR: schema_version=%v", got["schema_version"])
	}
	inv, _ := got["invocation"].(map[string]any)
	for _, key := range []string{"request_timeout_ms", "concurrency", "language_id", "expansion", "trace", "output_mode", "output_path", "seeds"} {
		if _, ok := inv[key]; !ok {
			t.Errorf("ASSERT_P2_EFFECTIVE_INVOCATION_%s: missing", strings.ToUpper(key))
		}
	}
	identity, _ := got["identity"].(map[string]any)
	if identity["caller_provenance_class"] != "CALLER_ASSERTED" {
		t.Errorf("ASSERT_P3_CALLER_ASSERTED: identity=%v", identity)
	}
	policy, _ := got["sensitivity_policy"].(map[string]any)
	if policy["automatic_redaction"] != false {
		t.Errorf("ASSERT_P9_NO_AUTOMATIC_REDACTION: policy=%v", policy)
	}
	wantCovered := []any{"invocation_arguments", "explicit_environment_values", "workspace_source_output_paths", "opaque_node_data", "diagnostics", "captured_server_stderr", "trace_transcripts"}
	if covered, _ := policy["covered"].([]any); !reflect.DeepEqual(covered, wantCovered) || policy["access_control_responsibility"] != "BUNDLE_CUSTODIAN" || policy["ambient_process_environment_recorded"] != false {
		t.Errorf("ASSERT_P9_EXACT_SENSITIVITY_POLICY: policy=%v", policy)
	}
	if _, ok := got["trace_receipt"]; !ok {
		t.Errorf("ASSERT_P5_EMBEDDED_SEMANTIC_RECEIPT: missing")
	}
}

func TestReceiptIssuanceRejectsStructurallyInvalidGraph(t *testing.T) {
	n := NewNode(Item{Name: "n", URI: "file:///n"})
	cases := []Result{
		{SchemaVersion: SchemaVersion, Nodes: []Node{n, n}},
		{SchemaVersion: SchemaVersion, Nodes: []Node{n}, Targets: []string{"missing"}},
		{SchemaVersion: SchemaVersion, Nodes: []Node{n}, Edges: []Edge{{CallerNodeID: n.ID, CalleeNodeID: "missing"}}},
		{SchemaVersion: SchemaVersion, Nodes: []Node{n}, Terminals: []Boundary{{NodeID: "missing", Reason: MaxDepth}}},
		{SchemaVersion: SchemaVersion, Nodes: []Node{n}, Seeds: []SeedResult{{Label: "s", PreparedTargetIDs: []string{"missing"}}}},
	}
	for i, r := range cases {
		if _, err := json.Marshal(r); err == nil {
			t.Errorf("ASSERT_P4_STRUCTURAL_REJECTION_%d: marshal succeeded", i)
		}
	}
}

func TestValidateSemanticBundleRejectsRehashedIdentityMismatch(t *testing.T) {
	r := Result{SchemaVersion: SchemaVersionV3, Invocation: Invocation{Seeds: []InvocationSeed{{Label: "a", At: "a.go:1:1", ResolvedURI: "file:///a.go", ContentSHA256: "sha256:abc", LanguageID: "go"}}}}
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var bundle bundleV3
	if err := json.Unmarshal(encoded, &bundle); err != nil {
		t.Fatal(err)
	}
	bundle.Identity.AggregateFingerprint = "sha256:forged"
	canonical, err := json.Marshal(bundle.semanticV3)
	if err != nil {
		t.Fatal(err)
	}
	bundle.TraceReceipt = TraceReceipt{"lsp-trace.semantic-receipt.v1", domainDigest(SemanticDigestDomain, canonical), SemanticDigestScope}
	mutated, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateSemanticBundle(mutated); err == nil || !strings.Contains(err.Error(), "identity mismatch") {
		t.Fatalf("ASSERT_P3_VERIFY_REJECTS_REHASHED_IDENTITY_MISMATCH: %v", err)
	}
}

func TestValidateSemanticBundleRejectsRehashedDanglingReference(t *testing.T) {
	r := Result{SchemaVersion: SchemaVersionV3}
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var bundle bundleV3
	if err := json.Unmarshal(encoded, &bundle); err != nil {
		t.Fatal(err)
	}
	bundle.Targets = []string{"missing"}
	canonical, err := json.Marshal(bundle.semanticV3)
	if err != nil {
		t.Fatal(err)
	}
	bundle.TraceReceipt = TraceReceipt{"lsp-trace.semantic-receipt.v1", domainDigest(SemanticDigestDomain, canonical), SemanticDigestScope}
	mutated, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateSemanticBundle(mutated); err == nil || !strings.Contains(err.Error(), "dangling target") {
		t.Fatalf("ASSERT_P4_VERIFY_REJECTS_REHASHED_DANGLING_REFERENCE: %v", err)
	}
}

func TestV2SelectionRetainsLegacyProjection(t *testing.T) {
	r := Result{SchemaVersion: SchemaVersionV2, Summary: Summary{Complete: true}}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"schema_version":"lsp-trace.graph.v2"`) || strings.Contains(string(b), `"identity"`) {
		t.Errorf("ASSERT_P1_V2_EXACT_PROJECTION: %s", b)
	}
}
