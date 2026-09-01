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
		t.Errorf("ASSERT_DISCLOSURE_AUTOMATIC_REDACTION_FALSE: policy=%v", policy)
	}
	wantCovered := []any{"invocation_arguments", "explicit_environment_names", "workspace_source_output_paths", "opaque_node_data", "diagnostics", "captured_server_stderr", "trace_transcripts"}
	if covered, _ := policy["covered"].([]any); !reflect.DeepEqual(covered, wantCovered) || policy["access_control_responsibility"] != "BUNDLE_CUSTODIAN" || policy["ambient_process_environment_recorded"] != false {
		t.Errorf("ASSERT_P9_EXACT_SENSITIVITY_POLICY: policy=%v", policy)
	}
	traceReceipt, ok := got["trace_receipt"].(map[string]any)
	if !ok || traceReceipt["semantic_commitment_digest"] == nil {
		t.Errorf("ASSERT_DIGEST_ROLE_SEMANTIC_COMMITMENT: receipt=%v", got["trace_receipt"])
	}
	if traceReceipt["content_digest"] != nil {
		t.Errorf("ASSERT_DIGEST_ROLE_GENERIC_NAMES_ABSENT: receipt=%v", traceReceipt)
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
	effectiveDigest, effectiveOK := processContext["effective_environment_process_context_digest"].(string)
	workingDigest, workingOK := processContext["working_directory_process_context_digest"].(string)
	if strings.Contains(string(encoded), "ambient-secret") || strings.Contains(string(encoded), "/private/workspace") || server["environment"] != nil || processContext["ambient_environment_state"] != "IDENTIFIED_NOT_EMBEDDED" || processContext["effective_environment_variable_count"] != float64(2) || !effectiveOK || !workingOK || !strings.HasPrefix(effectiveDigest, "sha256:") || !strings.HasPrefix(workingDigest, "sha256:") {
		t.Errorf("ASSERT_DIGEST_ROLE_PROCESS_CONTEXT: %s", encoded)
	}
	environmentEntries, _ := processContext["environment"].([]any)
	if processContext["effective_environment_identity"] != nil || processContext["working_directory_identity"] != nil || len(environmentEntries) != 1 || environmentEntries[0].(map[string]any)["environment_name_process_context_digest"] == nil || environmentEntries[0].(map[string]any)["identity"] != nil {
		t.Errorf("ASSERT_DIGEST_ROLE_GENERIC_NAMES_ABSENT: process_context=%v", processContext)
	}
	identity, _ := got["identity"].(map[string]any)
	if identity["resolved_seed_contents_digest"] == nil || identity["aggregate_fingerprint"] != nil {
		t.Errorf("ASSERT_DIGEST_ROLE_GENERIC_NAMES_ABSENT: identity=%v", identity)
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
		if a["state"] == "PRESENT" && a["replay_input_content_digest"] == nil || a["digest"] != nil {
			t.Errorf("ASSERT_DIGEST_ROLE_GENERIC_NAMES_ABSENT: artifact=%v", a)
		}
	}
	if manifest["replay_input_manifest_digest"] == nil || states["SOURCE_ARTIFACT"] != "PRESENT" || states["PROTOCOL_TRANSCRIPT"] != "ABSENT" || states["SERVER_STDERR"] != "ABSENT" {
		t.Errorf("ASSERT_DIGEST_ROLE_REPLAY_INPUT_CONTENT: %#v", manifest)
	}
	if manifest["manifest_id"] != nil {
		t.Errorf("ASSERT_DIGEST_ROLE_GENERIC_NAMES_ABSENT: manifest=%v", manifest)
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
	bundle.Identity.ResolvedSeedContentsDigest = "sha256:forged"
	canonical, err := json.Marshal(bundle.semanticV3)
	if err != nil {
		t.Fatal(err)
	}
	bundle.TraceReceipt = semanticReceiptV3{"lsp-trace.semantic-receipt.v1", domainDigest(SemanticDigestDomain, canonical), SemanticDigestScope}
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
	bundle.TraceReceipt = semanticReceiptV3{"lsp-trace.semantic-receipt.v1", domainDigest(SemanticDigestDomain, canonical), SemanticDigestScope}
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
	bundle.TraceReceipt = semanticReceiptV3{"lsp-trace.semantic-receipt.v1", domainDigest(SemanticDigestDomain, canonical), SemanticDigestScope}
	mutated, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateSemanticBundle(mutated); err == nil || !strings.Contains(err.Error(), "derived semantic mismatch") {
		t.Fatalf("ASSERT_P2_VERIFY_RECOMPUTES_DERIVED_SEMANTICS: %v", err)
	}
}

func TestValidateSemanticBundleRejectsRehashedFalseCompleteness(t *testing.T) {
	encoded, err := json.Marshal(Result{SchemaVersion: SchemaVersionV3, Summary: Summary{Complete: true}})
	if err != nil {
		t.Fatal(err)
	}
	var bundle bundleV3
	if err := json.Unmarshal(encoded, &bundle); err != nil {
		t.Fatal(err)
	}
	bundle.Summary.TraversalComplete = false
	canonical, _ := json.Marshal(bundle.semanticV3)
	bundle.TraceReceipt = semanticReceiptV3{"lsp-trace.semantic-receipt.v1", domainDigest(SemanticDigestDomain, canonical), SemanticDigestScope}
	mutated, _ := json.Marshal(bundle)
	if err := ValidateSemanticBundle(mutated); err == nil || !strings.Contains(err.Error(), "derived semantic mismatch") {
		t.Fatalf("ASSERT_SYMMETRIC_COMPLETENESS_REJECTS_FALSE: %v", err)
	}
}

func TestValidateSemanticBundleRecomputesEveryCapabilityCounter(t *testing.T) {
	n1 := NewNode(Item{Name: "caller", URI: "file:///w/a.go"})
	n2 := NewNode(Item{Name: "callee", URI: "file:///w/b.go"})
	r := Result{SchemaVersion: SchemaVersionV3, Capabilities: Capabilities{CallHierarchyProvider: true}, CapabilityQuality: CapabilityQuality{Advertised: true, PrepareSucceeded: true, IncomingRequestSuccesses: 1, CrossModuleEdges: Unknown}, Targets: []string{n2.ID}, Nodes: []Node{n1, n2}, Edges: []Edge{{CallerNodeID: n1.ID, CalleeNodeID: n2.ID}}, Summary: Summary{Complete: true}}
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateSemanticBundle(encoded); err != nil {
		t.Fatalf("valid capability baseline rejected: %v", err)
	}
	var base bundleV3
	if err := json.Unmarshal(encoded, &base); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func(*CapabilityQuality)
	}{
		{"prepare_succeeded", func(q *CapabilityQuality) { q.PrepareSucceeded = !q.PrepareSucceeded }},
		{"incoming_request_successes", func(q *CapabilityQuality) { q.IncomingRequestSuccesses++ }},
		{"incoming_edges", func(q *CapabilityQuality) { q.IncomingEdges++ }},
		{"cross_file_edges", func(q *CapabilityQuality) { q.CrossFileEdges++ }},
		{"cross_module_edges", func(q *CapabilityQuality) { q.CrossModuleEdges = "FORGED" }},
		{"unresolved_calls", func(q *CapabilityQuality) { q.UnresolvedCalls++ }},
		{"dynamic_calls", func(q *CapabilityQuality) { q.DynamicCalls++ }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bundle := base
			tc.mutate(&bundle.CapabilityQuality)
			canonical, _ := json.Marshal(bundle.semanticV3)
			bundle.TraceReceipt = semanticReceiptV3{"lsp-trace.semantic-receipt.v1", domainDigest(SemanticDigestDomain, canonical), SemanticDigestScope}
			mutated, _ := json.Marshal(bundle)
			if err := ValidateSemanticBundle(mutated); err == nil || !strings.Contains(err.Error(), "derived semantic mismatch") {
				t.Fatalf("ASSERT_ALL_CAPABILITY_COUNTERS_RECOMPUTED_%s: %v", strings.ToUpper(tc.name), err)
			}
		})
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
	bundle.ReplayInputManifest.ReplayInputManifestDigest = "sha256:forged"
	canonical, err := json.Marshal(bundle.semanticV3)
	if err != nil {
		t.Fatal(err)
	}
	bundle.TraceReceipt = semanticReceiptV3{"lsp-trace.semantic-receipt.v1", domainDigest(SemanticDigestDomain, canonical), SemanticDigestScope}
	mutated, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateSemanticBundle(mutated); err == nil || !strings.Contains(err.Error(), "replay identity mismatch") {
		t.Fatalf("ASSERT_P5_VERIFY_RECOMPUTES_REPLAY_IDENTITY: %v", err)
	}
}

func TestV3CanonicalProjectionRoundTripsAndRejectsDuplicateEdges(t *testing.T) {
	n1 := NewNode(Item{Name: "caller", URI: "file:///w/a.go"})
	n2 := NewNode(Item{Name: "callee", URI: "file:///w/b.go"})
	r := Result{SchemaVersion: SchemaVersionV3, Nodes: []Node{n2, n1}, Edges: []Edge{{CallerNodeID: n1.ID, CalleeNodeID: n2.ID}}, Summary: Summary{Complete: true}}
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateSemanticBundle(encoded); err != nil {
		t.Fatalf("ASSERT_SEMANTIC_CANONICAL_ROUND_TRIP: %v", err)
	}
	r.Edges = append(r.Edges, r.Edges[0])
	if _, err := json.Marshal(r); err == nil || !strings.Contains(err.Error(), "duplicate canonical edge") {
		t.Fatalf("ASSERT_DUPLICATE_CANONICAL_EDGE_REJECTED: %v", err)
	}
}

func TestRelationIdentityIsSemanticAndSeedIndependent(t *testing.T) {
	a := newEvidenceRelation("SIBLING_CANDIDATE", "DISCOVERY", "file:///w/a.go", "rev-a", "file:///w/a.go", "seed-a", "candidate", "", "", "", "")
	b := newEvidenceRelation("SIBLING_CANDIDATE", "DISCOVERY", "file:///w/a.go", "rev-b", "file:///w/a.go", "seed-b", "candidate", "", "", "", "")
	if a.RelationID != b.RelationID {
		t.Fatalf("ASSERT_SEMANTIC_RELATION_ID_SEED_INDEPENDENT: %s != %s", a.RelationID, b.RelationID)
	}
}

func TestProcessContextNormalizesEquivalentWorkingDirectories(t *testing.T) {
	a := projectProcessContext(nil, nil, "/tmp/work/../work")
	b := projectProcessContext(nil, nil, "/tmp/work")
	if a.WorkingDirectoryProcessContextDigest != b.WorkingDirectoryProcessContextDigest {
		t.Fatalf("ASSERT_PORTABLE_CWD_IDENTITY: %s != %s", a.WorkingDirectoryProcessContextDigest, b.WorkingDirectoryProcessContextDigest)
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
