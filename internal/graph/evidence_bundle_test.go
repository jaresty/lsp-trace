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
	if policy["automatic_redaction"] != true {
		t.Errorf("ASSERT_P1_EXPLICIT_AUTOMATIC_REDACTION: policy=%v", policy)
	}
	wantCovered := []any{"invocation_arguments", "explicit_environment_names", "workspace_source_output_paths", "opaque_node_data", "diagnostics", "captured_server_stderr", "trace_transcripts"}
	if covered, _ := policy["covered"].([]any); !reflect.DeepEqual(covered, wantCovered) || policy["access_control_responsibility"] != "BUNDLE_CUSTODIAN" || policy["ambient_process_environment_recorded"] != false {
		t.Errorf("ASSERT_P9_EXACT_SENSITIVITY_POLICY: policy=%v", policy)
	}
	if _, ok := got["trace_receipt"]; !ok {
		t.Errorf("ASSERT_P5_EMBEDDED_SEMANTIC_RECEIPT: missing")
	}
}

func TestV3SemanticReplayIdentityContract(t *testing.T) {
	n1 := NewNode(Item{Name: "caller", URI: "file:///w/a.go", Range: Range{End: Position{Line: 1}}, SelectionRange: Range{End: Position{Line: 1}}})
	n2 := NewNode(Item{Name: "callee", URI: "file:///w/b.go", Range: Range{Start: Position{Line: 2}, End: Position{Line: 3}}, SelectionRange: Range{Start: Position{Line: 2}, End: Position{Line: 3}}})
	r := Result{
		SchemaVersion: SchemaVersionV3,
		Invocation: Invocation{
			WorkingDirectory:     "/private/workspace",
			EffectiveEnvironment: []string{"PATH=/bin", "TOKEN=ambient-secret", "PATH=/usr/bin"},
			Server:               ServerInvocation{Command: "server", Environment: map[string]string{"TOKEN": "ambient-secret"}},
			Trace:                TraceConfig{Enabled: false},
			Seeds: []InvocationSeed{
				{Label: "first", At: "a.go:1:1", ResolvedURI: "file:///w/a.go", ContentSHA256: "sha256:first", LanguageID: "go"},
				{Label: "second", At: "a.go:2:1", ResolvedURI: "file:///w/a.go", ContentSHA256: "sha256:first", LanguageID: "go"},
			},
			Provenance: InvocationProvenance{SourceRevision: "rev"},
		},
		Nodes: []Node{n1, n2}, Targets: []string{n2.ID}, Edges: []Edge{{CallerNodeID: n1.ID, CalleeNodeID: n2.ID}},
		Seeds: []SeedResult{
			{Label: "first", ReachedNodeIDs: []string{n1.ID}, ReachedEdges: []Edge{{CallerNodeID: n1.ID, CalleeNodeID: n2.ID}}},
			{Label: "second", ReachedNodeIDs: []string{n1.ID}, ReachedEdges: []Edge{{CallerNodeID: n1.ID, CalleeNodeID: n2.ID}}},
		},
		SiblingCandidates: []SiblingCandidate{
			{SeedURI: "file:///w/a.go", SeedLabel: "first", Candidate: n1},
			{SeedURI: "file:///w/a.go", SeedLabel: "second", Candidate: n2},
		},
		DispatchRelationships: []DispatchRelationship{{SeedLabel: "first", Interface: n1, Implementation: n2}},
	}
	r.Canonicalize()
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	inv := got["invocation"].(map[string]any)
	server := inv["server"].(map[string]any)
	processContext, _ := got["process_context"].(map[string]any)
	if strings.Contains(string(encoded), "ambient-secret") || strings.Contains(string(encoded), "/private/workspace") || server["environment"] != nil || processContext["ambient_environment_state"] != "IDENTIFIED_NOT_EMBEDDED" || processContext["effective_environment_variable_count"] != float64(2) || !strings.HasPrefix(processContext["effective_environment_identity"].(string), "sha256:") || !strings.HasPrefix(processContext["working_directory_identity"].(string), "sha256:") {
		t.Errorf("ASSERT_P1_EFFECTIVE_PROCESS_IDENTITY_SECRET_SAFE: %s", encoded)
	}
	receipt, _ := got["evidence_receipt"].(map[string]any)
	relations, _ := receipt["relations"].([]any)
	seen := map[string]bool{}
	for _, raw := range relations {
		id, _ := raw.(map[string]any)["relation_id"].(string)
		if id == "" || seen[id] {
			t.Errorf("ASSERT_P3_GLOBAL_RELATION_IDENTITIES: %#v", relations)
		}
		seen[id] = true
	}
	if len(relations) != 4 {
		t.Errorf("ASSERT_P3_CALL_DISPATCH_SIBLING_RELATIONS: %#v", receipt)
	}
	memberships, _ := got["seed_memberships"].([]any)
	membershipIDs, kinds := map[string]bool{}, map[string]bool{}
	for _, raw := range memberships {
		membership := raw.(map[string]any)
		membershipIDs[membership["membership_id"].(string)] = true
		kinds[membership["evidence_kind"].(string)] = true
	}
	if len(memberships) != 7 || len(membershipIDs) != 7 || !kinds["REACHED_NODE"] || !kinds["CALL_RELATION"] || !kinds["SIBLING_CANDIDATE"] || !kinds["DISPATCH_ASSOCIATION"] {
		t.Errorf("ASSERT_P4_EXACT_SAME_FILE_SEED_RELATION_MEMBERSHIP: %#v", memberships)
	}
	manifest, _ := got["replay_input_manifest"].(map[string]any)
	artifacts, _ := manifest["artifacts"].([]any)
	states := map[string]string{}
	for _, raw := range artifacts {
		a := raw.(map[string]any)
		states[a["kind"].(string)] = a["state"].(string)
	}
	if manifest["manifest_id"] == nil || states["SOURCE_ARTIFACT"] != "PRESENT" || states["PROTOCOL_TRANSCRIPT"] != "ABSENT" || states["SERVER_STDERR"] != "ABSENT" {
		t.Errorf("ASSERT_P5_REPLAY_MANIFEST_EXPLICIT_STATES: %#v", manifest)
	}
	locators, _ := got["portable_locators"].([]any)
	if len(locators) == 0 || !strings.HasPrefix(locators[0].(map[string]any)["locator"].(string), "file:///w/") || locators[0].(map[string]any)["redaction"] == nil {
		t.Errorf("ASSERT_P8_PORTABLE_LOCATOR_REDACTION: %#v", locators)
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

func TestValidateSemanticBundleRejectsRehashedDerivedSemanticMismatch(t *testing.T) {
	r := Result{SchemaVersion: SchemaVersionV3, Summary: Summary{Complete: true}}
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var bundle bundleV3
	if err := json.Unmarshal(encoded, &bundle); err != nil {
		t.Fatal(err)
	}
	bundle.Summary.NodeCount = 9
	bundle.CapabilityQuality.IncomingEdges = 7
	canonical, err := json.Marshal(bundle.semanticV3)
	if err != nil {
		t.Fatal(err)
	}
	bundle.TraceReceipt = TraceReceipt{"lsp-trace.semantic-receipt.v1", domainDigest(SemanticDigestDomain, canonical), SemanticDigestScope}
	mutated, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateSemanticBundle(mutated); err == nil || !strings.Contains(err.Error(), "derived semantic mismatch") {
		t.Fatalf("ASSERT_P2_VERIFY_RECOMPUTES_DERIVED_SEMANTICS: %v", err)
	}
}

func TestValidateSemanticBundleRejectsRehashedReplayIdentityMismatch(t *testing.T) {
	encoded, err := json.Marshal(Result{SchemaVersion: SchemaVersionV3})
	if err != nil {
		t.Fatal(err)
	}
	var bundle bundleV3
	if err := json.Unmarshal(encoded, &bundle); err != nil {
		t.Fatal(err)
	}
	bundle.ReplayInputManifest.ManifestID = "sha256:forged"
	canonical, err := json.Marshal(bundle.semanticV3)
	if err != nil {
		t.Fatal(err)
	}
	bundle.TraceReceipt = TraceReceipt{"lsp-trace.semantic-receipt.v1", domainDigest(SemanticDigestDomain, canonical), SemanticDigestScope}
	mutated, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateSemanticBundle(mutated); err == nil || !strings.Contains(err.Error(), "replay identity mismatch") {
		t.Fatalf("ASSERT_P5_VERIFY_RECOMPUTES_REPLAY_IDENTITY: %v", err)
	}
}

func TestMergeResultsPreservesRequestedHistoricalSchema(t *testing.T) {
	merged := MergeResults(Result{SchemaVersion: SchemaVersionV2}, Result{SchemaVersion: SchemaVersionV3})
	if merged.SchemaVersion != SchemaVersionV2 {
		t.Fatalf("ASSERT_P6_MERGE_SCHEMA: got %q", merged.SchemaVersion)
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
