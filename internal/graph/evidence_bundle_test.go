package graph

import (
	"bytes"
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
	if identity["aggregate_scope"] != "RESOLVED_SEED_CONTENTS" {
		t.Errorf("ASSERT_WORKSPACE_AUTHORITY_RESOLVED_SEED_SCOPE: identity=%v", identity)
	}
	if identity["tool_version_provenance_class"] != "CALLER_ASSERTED" {
		t.Errorf("ASSERT_TOOL_VERSION_CALLER_ASSERTED: identity=%v", identity)
	}
	if identity["server_version_provenance_class"] != "CALLER_ASSERTED" {
		t.Errorf("ASSERT_SERVER_VERSION_CALLER_ASSERTED: identity=%v", identity)
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
			{Label: "first", ReachedNodeIDs: []string{n1.ID}, ReachedRelationIDs: []string{canonicalRelationID("CALL_RELATION", "CALLER_TO_CALLEE", n1.ID+"->"+n2.ID, "", "", "", n1.ID, n2.ID)}, ReachedEdges: []Edge{{CallerNodeID: n1.ID, CalleeNodeID: n2.ID}}},
			{Label: "second", ReachedNodeIDs: []string{n1.ID}, ReachedRelationIDs: []string{canonicalRelationID("CALL_RELATION", "CALLER_TO_CALLEE", n1.ID+"->"+n2.ID, "", "", "", n1.ID, n2.ID)}, ReachedEdges: []Edge{{CallerNodeID: n1.ID, CalleeNodeID: n2.ID}}},
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
	for _, raw := range locators {
		locator := raw.(map[string]any)
		provenance, _ := locator["provenance"].(map[string]any)
		source, _ := provenance["source"].(map[string]any)
		derivation, _ := provenance["derivation"].(map[string]any)
		authority, _ := provenance["authority"].(map[string]any)
		semantics, _ := provenance["semantics"].(map[string]any)
		if source["node_id"] != locator["node_id"] || source["selection_range"] == nil {
			t.Errorf("ASSERT_LOCATOR_PROVENANCE_SOURCE_IDENTITY: %#v", locator)
		}
		if derivation["method"] != "CANONICAL_URI_WITH_SELECTION_RANGE" || derivation["version"] != "1" {
			t.Errorf("ASSERT_LOCATOR_PROVENANCE_DERIVATION_IDENTITY: %#v", locator)
		}
		if authority["class"] != "TOOL_DERIVED" || authority["tool"] != "lsp-trace" {
			t.Errorf("ASSERT_LOCATOR_PROVENANCE_TOOL_AUTHORITY: %#v", locator)
		}
		if semantics["establishes_runtime_behavior"] != false || semantics["establishes_feature_correspondence"] != false {
			t.Errorf("ASSERT_LOCATOR_PROVENANCE_SEMANTIC_CEILING: %#v", locator)
		}
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
	r := Result{SchemaVersion: SchemaVersionV3, Invocation: Invocation{Seeds: []InvocationSeed{{Label: "a", At: "a.go:1:1", ResolvedURI: "file:///a.go", ContentSHA256: "sha256:abc", LanguageID: "go"}}}, Seeds: []SeedResult{{Label: "a"}}}
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

func TestValidateSemanticBundleRejectsRehashedAuthorityMismatch(t *testing.T) {
	encoded, err := json.Marshal(Result{SchemaVersion: SchemaVersionV3})
	if err != nil {
		t.Fatal(err)
	}
	var base bundleV3
	if err := json.Unmarshal(encoded, &base); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func(*BundleIdentity)
	}{
		{"workspace_scope", func(identity *BundleIdentity) { identity.AggregateScope = "WHOLE_WORKSPACE" }},
		{"tool_version", func(identity *BundleIdentity) { identity.ToolVersionProvenanceClass = "TOOL_DERIVED" }},
		{"server_version", func(identity *BundleIdentity) { identity.ServerVersionProvenanceClass = "TOOL_DERIVED" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bundle := base
			tc.mutate(&bundle.Identity)
			canonical, marshalErr := json.Marshal(bundle.semanticV3)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			bundle.TraceReceipt = semanticReceiptV3{"lsp-trace.semantic-receipt.v1", domainDigest(SemanticDigestDomain, canonical), SemanticDigestScope}
			mutated, marshalErr := json.Marshal(bundle)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			if verifyErr := ValidateSemanticBundle(mutated); verifyErr == nil || !strings.Contains(verifyErr.Error(), "bundle identity mismatch") {
				t.Fatalf("ASSERT_AUTHORITY_MISMATCH_REJECTED_%s: %v", strings.ToUpper(tc.name), verifyErr)
			}
		})
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

func TestValidateSemanticBundleRejectsRehashedExactJoinAndMetadataMutations(t *testing.T) {
	n1 := NewNode(Item{Name: "caller-a", URI: "file:///w/a.go"})
	n2 := NewNode(Item{Name: "callee-a", URI: "file:///w/a.go"})
	n3 := NewNode(Item{Name: "caller-b", URI: "file:///w/b.go"})
	n4 := NewNode(Item{Name: "callee-b", URI: "file:///w/b.go"})
	r := Result{
		SchemaVersion: SchemaVersionV3,
		Invocation: Invocation{
			Server:     ServerInvocation{Environment: map[string]string{"TOKEN": "secret"}},
			Seeds:      []InvocationSeed{{Label: "a", At: "a.go:1:1"}, {Label: "b", At: "b.go:1:1"}},
			Provenance: InvocationProvenance{SourceRevision: "rev"},
		},
		Nodes: []Node{n1, n2, n3, n4},
		Edges: []Edge{{CallerNodeID: n1.ID, CalleeNodeID: n2.ID}, {CallerNodeID: n3.ID, CalleeNodeID: n4.ID}},
		Seeds: []SeedResult{
			{Label: "a", ReachedNodeIDs: []string{n1.ID, n2.ID}, ReachedRelationIDs: []string{canonicalRelationID("CALL_RELATION", "CALLER_TO_CALLEE", n1.ID+"->"+n2.ID, "", "", "", n1.ID, n2.ID)}, ReachedEdges: []Edge{{CallerNodeID: n1.ID, CalleeNodeID: n2.ID}}},
			{Label: "b", ReachedNodeIDs: []string{n3.ID, n4.ID}, ReachedRelationIDs: []string{canonicalRelationID("CALL_RELATION", "CALLER_TO_CALLEE", n3.ID+"->"+n4.ID, "", "", "", n3.ID, n4.ID)}, ReachedEdges: []Edge{{CallerNodeID: n3.ID, CalleeNodeID: n4.ID}}},
		},
		Summary: Summary{Complete: true},
	}
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var base bundleV3
	if err := json.Unmarshal(encoded, &base); err != nil {
		t.Fatal(err)
	}
	rehash := func(bundle *bundleV3) []byte {
		t.Helper()
		canonical, err := json.Marshal(bundle.semanticV3)
		if err != nil {
			t.Fatal(err)
		}
		bundle.TraceReceipt = semanticReceiptV3{"lsp-trace.semantic-receipt.v1", domainDigest(SemanticDigestDomain, canonical), SemanticDigestScope}
		mutated, err := json.Marshal(bundle)
		if err != nil {
			t.Fatal(err)
		}
		return mutated
	}
	cases := []struct {
		name   string
		want   string
		mutate func(*bundleV3)
	}{
		{"missing result", "invocation/result cardinality", func(b *bundleV3) { b.Seeds = nil; b.SeedMemberships = nil }},
		{"sensitivity policy", "sensitivity policy mismatch", func(b *bundleV3) { b.SensitivityPolicy.AutomaticRedaction = true }},
		{"evidence semantics", "evidence semantics mismatch", func(b *bundleV3) { b.EvidenceSemantics.CallEdges.SupportContribution = 99 }},
		{"unknown native relation kind", `unknown native relation kind "UNKNOWN_RELATION"`, func(b *bundleV3) {
			b.EvidenceReceipt.Relations[0].RelationKind = "UNKNOWN_RELATION"
		}},
		{"unknown derived evidence kind", `unknown derived evidence kind "UNKNOWN_MEMBERSHIP"`, func(b *bundleV3) {
			b.SeedMemberships[0].EvidenceKind = "UNKNOWN_MEMBERSHIP"
		}},
		{"portable locator derivation", "replay identity mismatch: portable locators", func(b *bundleV3) {
			b.PortableLocators[0].Provenance.Derivation.Version = "forged"
		}},
		{"duplicate explicit environment name", "duplicate explicit environment name", func(b *bundleV3) {
			b.ProcessContext.Environment = append(b.ProcessContext.Environment, b.ProcessContext.Environment[0])
		}},
		{"unknown primary relation", "unknown call relation", func(b *bundleV3) {
			b.Seeds[0].ReachedRelationIDs = []string{"sha256:unknown"}
			b.SeedMemberships = projectSeedMemberships(b.ExecutionBundleID, b.Invocation.Seeds, b.Seeds, b.Edges, b.SiblingCandidates, b.DispatchRelationships, b.Invocation.Provenance.SourceRevision)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bundle := base
			bundle.Seeds = append([]SeedResult(nil), base.Seeds...)
			bundle.SeedMemberships = append([]SeedMembership(nil), base.SeedMemberships...)
			if base.EvidenceReceipt != nil {
				receipt := *base.EvidenceReceipt
				receipt.Relations = append([]EvidenceRelation(nil), base.EvidenceReceipt.Relations...)
				bundle.EvidenceReceipt = &receipt
			}
			bundle.ProcessContext.Environment = append([]EnvironmentIdentity(nil), base.ProcessContext.Environment...)
			tc.mutate(&bundle)
			if err := ValidateSemanticBundle(rehash(&bundle)); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("mutation accepted or wrong error: %v", err)
			}
		})
	}
}

func TestMarshalV3RejectsInvalidExactSeedJoin(t *testing.T) {
	r := Result{SchemaVersion: SchemaVersionV3, Invocation: Invocation{Seeds: []InvocationSeed{{Label: "expected", At: "a.go:1:1"}}}}
	if _, err := json.Marshal(r); err == nil || !strings.Contains(err.Error(), "seed join mismatch") {
		t.Fatalf("ASSERT_MARSHAL_V3_EXACT_SEED_JOIN: %v", err)
	}
}

func TestV3MarshalDoesNotInferSeedRelationsFromMergedEdges(t *testing.T) {
	caller := NewNode(Item{Name: "caller", URI: "file:///w/caller.go"})
	calleeA := NewNode(Item{Name: "callee-a", URI: "file:///w/a.go"})
	calleeB := NewNode(Item{Name: "callee-b", URI: "file:///w/b.go"})
	edgeA := Edge{CallerNodeID: caller.ID, CalleeNodeID: calleeA.ID}
	edgeB := Edge{CallerNodeID: caller.ID, CalleeNodeID: calleeB.ID}
	wrongID := canonicalRelationID("CALL_RELATION", "CALLER_TO_CALLEE", caller.ID+"->"+calleeB.ID, "", "", "", caller.ID, calleeB.ID)
	r := Result{
		SchemaVersion: SchemaVersionV3,
		Invocation:    Invocation{Seeds: []InvocationSeed{{Label: "seed-a", At: "a.go:1:1"}}},
		Nodes:         []Node{caller, calleeA, calleeB},
		Edges:         []Edge{edgeA, edgeB},
		Seeds: []SeedResult{{
			Label: "seed-a", ReachedNodeIDs: []string{caller.ID, calleeA.ID},
			ReachedEdges: []Edge{edgeA}, ReachedRelationIDs: []string{wrongID},
		}},
	}
	if _, err := json.Marshal(r); err == nil || !strings.Contains(err.Error(), `seed join mismatch: exact call relations for "seed-a"`) {
		t.Fatalf("ASSERT_MARSHAL_DOES_NOT_INFER_FROM_MERGED_EDGES: %v", err)
	}
}

func TestCanonicalizeDoesNotCreateEmptyDiscoveryMembershipLabels(t *testing.T) {
	node := NewNode(Item{Name: "candidate", URI: "file:///w/a.go"})
	r := Result{SiblingCandidates: []SiblingCandidate{{Candidate: node}}, DispatchRelationships: []DispatchRelationship{{Interface: node, Implementation: node}}}
	r.Canonicalize()
	if len(r.SiblingCandidates[0].SeedLabels) != 0 || len(r.DispatchRelationships[0].SeedLabels) != 0 {
		t.Fatalf("ASSERT_NO_EMPTY_DISCOVERY_SEED_MEMBERSHIPS: siblings=%#v dispatch=%#v", r.SiblingCandidates, r.DispatchRelationships)
	}
}

func TestSharedCallerDistinctCalleesPreserveSeedRelationsOrderInvariant(t *testing.T) {
	caller := NewNode(Item{Name: "caller", URI: "file:///w/caller.go"})
	calleeA := NewNode(Item{Name: "callee-a", URI: "file:///w/a.go"})
	calleeB := NewNode(Item{Name: "callee-b", URI: "file:///w/b.go"})
	edgeA := Edge{CallerNodeID: caller.ID, CalleeNodeID: calleeA.ID}
	edgeB := Edge{CallerNodeID: caller.ID, CalleeNodeID: calleeB.ID}
	part := func(label string, edge Edge) Result {
		id := canonicalRelationID("CALL_RELATION", "CALLER_TO_CALLEE", edge.CallerNodeID+"->"+edge.CalleeNodeID, "", "", "", edge.CallerNodeID, edge.CalleeNodeID)
		return Result{SchemaVersion: SchemaVersionV3, Nodes: []Node{caller, calleeA, calleeB}, Edges: []Edge{edge}, Seeds: []SeedResult{{Label: label, ReachedNodeIDs: []string{caller.ID, edge.CalleeNodeID}, ReachedEdges: []Edge{edge}, ReachedRelationIDs: []string{id}}}, Summary: Summary{Complete: true}}
	}
	marshal := func(invocation []InvocationSeed, results ...Result) bundleV3 {
		merged := MergeResults(results...)
		merged.Invocation.Seeds = invocation
		encoded, err := json.Marshal(merged)
		if err != nil {
			t.Fatal(err)
		}
		var bundle bundleV3
		if err := json.Unmarshal(encoded, &bundle); err != nil {
			t.Fatal(err)
		}
		return bundle
	}
	seedA := InvocationSeed{Label: "a", At: "a.go:1:1"}
	seedB := InvocationSeed{Label: "b", At: "b.go:1:1"}
	forward := marshal([]InvocationSeed{seedA, seedB}, part("a", edgeA), part("b", edgeB))
	reverse := marshal([]InvocationSeed{seedB, seedA}, part("b", edgeB), part("a", edgeA))
	if !reflect.DeepEqual(forward.Edges, reverse.Edges) || !reflect.DeepEqual(forward.Seeds, reverse.Seeds) || !reflect.DeepEqual(forward.SeedMemberships, reverse.SeedMemberships) {
		t.Fatalf("ASSERT_SHARED_CALLER_DISTINCT_CALLEES_ORDER_INVARIANT: forward=%#v reverse=%#v", forward, reverse)
	}
	bySeed := map[string][]string{}
	for _, membership := range forward.SeedMemberships {
		if membership.EvidenceKind == "CALL_RELATION" {
			bySeed[membership.SeedLabel] = append(bySeed[membership.SeedLabel], membership.EndpointID)
		}
	}
	global := map[string]struct{}{}
	for _, edge := range forward.Edges {
		global[edge.RelationID] = struct{}{}
	}
	if len(global) != 2 || len(bySeed["a"]) != 1 || len(bySeed["b"]) != 1 || bySeed["a"][0] == bySeed["b"][0] {
		t.Fatalf("ASSERT_SHARED_CALLER_EXACT_PER_SEED_MEMBERSHIPS: global=%#v memberships=%#v", global, bySeed)
	}
	for _, ids := range bySeed {
		if _, exists := global[ids[0]]; !exists {
			t.Fatalf("ASSERT_SHARED_CALLER_MEMBERSHIP_GLOBAL_FOREIGN_KEY: global=%#v memberships=%#v", global, bySeed)
		}
	}
}

func TestV3SliceEvidenceRejectsDanglingNativeReferences(t *testing.T) {
	n := NewNode(Item{Name: "start", URI: "file:///w/a.go"})
	r := Result{SchemaVersion: SchemaVersionV3, Nodes: []Node{n}, Slice: &SliceEvidence{
		SourceURI: "file:///w/a.go", StartingNodeIDs: []string{"missing"},
		Layers: []SliceLayer{{Depth: 0, NodeIDs: []string{n.ID}}}, FrontierNodeIDs: []string{n.ID},
	}}
	if _, err := json.Marshal(r); err == nil || !strings.Contains(err.Error(), "dangling slice starting node id") {
		t.Fatalf("ASSERT_SLICE_NATIVE_REFERENCES_VALIDATED: %v", err)
	}
}

func TestV3SliceEvidenceRejectsIncompleteOrFailedSeedMemberships(t *testing.T) {
	a := NewNode(Item{Name: "a", URI: "file:///w/a.go"})
	b := NewNode(Item{Name: "b", URI: "file:///w/b.go"})
	edge := Edge{CallerNodeID: a.ID, CalleeNodeID: b.ID}
	relationID := canonicalRelationID("CALL_RELATION", "CALLER_TO_CALLEE", a.ID+"->"+b.ID, "", "", "", a.ID, b.ID)
	base := Result{
		SchemaVersion: SchemaVersionV3,
		Invocation:    Invocation{Seeds: []InvocationSeed{{Label: "seed", At: "a.go:1:1"}}},
		Nodes:         []Node{a, b}, Edges: []Edge{edge},
		Slice: &SliceEvidence{SourceURI: "file:///w/a.go", DownDepth: 1, StartingNodeIDs: []string{a.ID}, Layers: []SliceLayer{{Depth: 0, NodeIDs: []string{a.ID}}, {Depth: 1, NodeIDs: []string{b.ID}}}, FrontierNodeIDs: []string{b.ID}, OutgoingTerminalNodeIDs: []string{}, UpwardStartNodeIDs: []string{b.ID}, OutgoingRelationIDs: []string{relationID}},
		Seeds: []SeedResult{{Label: "seed", PreparedTargetIDs: []string{a.ID}, ReachedNodeIDs: []string{a.ID}, ReachedRelationIDs: []string{relationID}, ReachedEdges: []Edge{edge}}},
	}
	if _, err := json.Marshal(base); err == nil || !strings.Contains(err.Error(), "slice seed memberships do not cover union nodes") {
		t.Fatalf("ASSERT_SLICE_SEED_NODE_MEMBERSHIP_UNION_VALIDATED: %v", err)
	}
	base.Seeds[0].ReachedNodeIDs = []string{a.ID, b.ID}
	base.Seeds[0].Failure = &SeedFailure{Phase: "slice-prepare", Message: "failed"}
	if _, err := json.Marshal(base); err == nil || !strings.Contains(err.Error(), "failed slice seed has non-empty membership") {
		t.Fatalf("ASSERT_SLICE_FAILED_SEED_MEMBERSHIP_REJECTED: %v", err)
	}
}

func TestV3SliceEvidenceRejectsDanglingTerminalAndInvalidUpwardUnion(t *testing.T) {
	a := NewNode(Item{Name: "a", URI: "file:///w/a.go"})
	b := NewNode(Item{Name: "b", URI: "file:///w/b.go"})
	base := SliceEvidence{
		SourceURI: "file:///w/a.go", DownDepth: 1,
		StartingNodeIDs: []string{a.ID}, Layers: []SliceLayer{{Depth: 0, NodeIDs: []string{a.ID}}},
		OutgoingTerminalNodeIDs: []string{a.ID}, UpwardStartNodeIDs: []string{a.ID},
	}

	r := Result{SchemaVersion: SchemaVersionV3, Invocation: Invocation{Seeds: []InvocationSeed{{Label: "seed", At: "a.go:1:1"}}}, Nodes: []Node{a, b}, Seeds: []SeedResult{{Label: "seed", PreparedTargetIDs: []string{a.ID}, ReachedNodeIDs: []string{a.ID, b.ID}}}, Slice: &base}
	r.Slice.OutgoingTerminalNodeIDs = []string{"missing"}
	if _, err := json.Marshal(r); err == nil || !strings.Contains(err.Error(), "dangling slice outgoing terminal node id") {
		t.Fatalf("ASSERT_SLICE_OUTGOING_TERMINAL_NATIVE_REFERENCE: %v", err)
	}

	r.Slice.OutgoingTerminalNodeIDs = []string{a.ID}
	r.Slice.FrontierNodeIDs = []string{b.ID}
	r.Slice.Layers = append(r.Slice.Layers, SliceLayer{Depth: 1, NodeIDs: []string{b.ID}})
	r.Slice.UpwardStartNodeIDs = []string{a.ID}
	if _, err := json.Marshal(r); err == nil || !strings.Contains(err.Error(), "slice upward starts do not equal frontier and outgoing-terminal union") {
		t.Fatalf("ASSERT_SLICE_UPWARD_START_UNION_VALIDATED: %v", err)
	}

	r.Slice.UpwardStartNodeIDs = []string{b.ID, a.ID}
	reversed, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("ASSERT_SLICE_UPWARD_START_CANONICAL_BYTES: reversed marshal: %v", err)
	}
	r.Slice.UpwardStartNodeIDs = []string{a.ID, b.ID}
	sorted, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("ASSERT_SLICE_UPWARD_START_CANONICAL_BYTES: sorted marshal: %v", err)
	}
	if !bytes.Equal(reversed, sorted) {
		t.Fatalf("ASSERT_SLICE_UPWARD_START_CANONICAL_BYTES: reversed=%s sorted=%s", reversed, sorted)
	}
}

func TestV3SliceAllowsEmptyFrontierWhenGraphEndsBeforeDepth(t *testing.T) {
	n := NewNode(Item{Name: "start", URI: "file:///w/a.go"})
	r := Result{SchemaVersion: SchemaVersionV3, Invocation: Invocation{Seeds: []InvocationSeed{{Label: "seed", At: "a.go:1:1"}}}, Nodes: []Node{n}, Seeds: []SeedResult{{Label: "seed", PreparedTargetIDs: []string{n.ID}, ReachedNodeIDs: []string{n.ID}}}, Slice: &SliceEvidence{
		SourceURI: "file:///w/a.go", DownDepth: 1, StartingNodeIDs: []string{n.ID},
		Layers: []SliceLayer{{Depth: 0, NodeIDs: []string{n.ID}}}, FrontierNodeIDs: []string{},
		OutgoingTerminalNodeIDs: []string{n.ID}, UpwardStartNodeIDs: []string{n.ID},
	}}
	if _, err := json.Marshal(r); err != nil {
		t.Fatalf("ASSERT_SLICE_EMPTY_EXACT_DEPTH_VALID: %v", err)
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

func TestPrimaryRelationIDsAreV3Only(t *testing.T) {
	caller := NewNode(Item{Name: "caller", URI: "file:///w/a.go"})
	callee := NewNode(Item{Name: "callee", URI: "file:///w/b.go"})
	base := Result{
		Invocation:            Invocation{Seeds: []InvocationSeed{{Label: "seed", At: "a.go:1:1"}}},
		Seeds:                 []SeedResult{{Label: "seed"}},
		Nodes:                 []Node{caller, callee},
		Edges:                 MergeEdge(nil, Edge{CallerNodeID: caller.ID, CalleeNodeID: callee.ID}),
		SiblingCandidates:     []SiblingCandidate{{RelationID: canonicalRelationID("SIBLING_CANDIDATE", "DISCOVERY", "file:///w/a.go", caller.ID, "", "", "", ""), SeedLabel: "seed", Candidate: caller}},
		DispatchRelationships: []DispatchRelationship{{RelationID: canonicalRelationID("DISPATCH_ASSOCIATION", "INTERFACE_TO_IMPLEMENTATION", caller.ID+"->"+callee.ID, "", caller.ID, callee.ID, "", ""), SeedLabel: "seed", Interface: caller, Implementation: callee}},
		Summary:               Summary{Complete: true},
	}
	for _, schema := range []string{SchemaVersionV1, SchemaVersionV2, SchemaVersionV3} {
		r := base
		r.SchemaVersion = schema
		encoded, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		var document map[string]any
		if err := json.Unmarshal(encoded, &document); err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{"edges", "sibling_candidates", "dispatch_relationships"} {
			records, _ := document[field].([]any)
			for _, raw := range records {
				record, _ := raw.(map[string]any)
				_, hasID := record["relation_id"]
				if schema == SchemaVersionV3 && !hasID {
					t.Fatalf("ASSERT_V3_PRIMARY_RELATION_IDS_PRESENT: field=%s document=%s", field, encoded)
				}
				if schema != SchemaVersionV3 && hasID {
					t.Fatalf("ASSERT_HISTORICAL_PRIMARY_RELATION_IDS_ABSENT_%s: field=%s document=%s", schema, field, encoded)
				}
			}
		}
	}
}

func TestRelationIdentityIsSemanticAndSeedIndependent(t *testing.T) {
	caller := NewNode(Item{Name: "caller", URI: "file:///w/caller.go"})
	callee := NewNode(Item{Name: "callee", URI: "file:///w/callee.go"})
	candidate := NewNode(Item{Name: "candidate", URI: "file:///w/candidate.go"})
	r := Result{
		SchemaVersion: SchemaVersionV3,
		Invocation:    Invocation{Seeds: []InvocationSeed{{Label: "seed-a", At: "a.go:1:1"}, {Label: "seed-b", At: "b.go:1:1"}}, Provenance: InvocationProvenance{SourceRevision: "rev"}},
		Nodes:         []Node{caller, callee},
		Edges:         []Edge{{CallerNodeID: caller.ID, CalleeNodeID: callee.ID}},
		Seeds: []SeedResult{
			{Label: "seed-a", ReachedRelationIDs: []string{canonicalRelationID("CALL_RELATION", "CALLER_TO_CALLEE", caller.ID+"->"+callee.ID, "", "", "", caller.ID, callee.ID)}, ReachedEdges: []Edge{{CallerNodeID: caller.ID, CalleeNodeID: callee.ID}}},
			{Label: "seed-b", ReachedRelationIDs: []string{canonicalRelationID("CALL_RELATION", "CALLER_TO_CALLEE", caller.ID+"->"+callee.ID, "", "", "", caller.ID, callee.ID)}, ReachedEdges: []Edge{{CallerNodeID: caller.ID, CalleeNodeID: callee.ID}}},
		},
		SiblingCandidates:     []SiblingCandidate{{SeedURI: "file:///w/a.go", SeedLabel: "seed-a", Candidate: candidate}, {SeedURI: "file:///w/b.go", SeedLabel: "seed-b", Candidate: candidate}},
		DispatchRelationships: []DispatchRelationship{{SeedLabel: "seed-a", Interface: caller, Implementation: callee}, {SeedLabel: "seed-b", Interface: caller, Implementation: callee}},
		Summary:               Summary{Complete: true},
	}
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var bundle bundleV3
	if err := json.Unmarshal(encoded, &bundle); err != nil {
		t.Fatal(err)
	}
	if len(bundle.Edges) != 1 || bundle.Edges[0].RelationID == "" || len(bundle.SiblingCandidates) != 1 || bundle.SiblingCandidates[0].RelationID == "" || len(bundle.DispatchRelationships) != 1 || bundle.DispatchRelationships[0].RelationID == "" {
		t.Fatalf("ASSERT_PRIMARY_CANONICAL_RELATION_IDS: edges=%#v siblings=%#v dispatch=%#v", bundle.Edges, bundle.SiblingCandidates, bundle.DispatchRelationships)
	}
	members := map[string]int{}
	for _, membership := range bundle.SeedMemberships {
		members[membership.EvidenceKind+":"+membership.EndpointID]++
	}
	if members["CALL_RELATION:"+bundle.Edges[0].RelationID] != 2 || members["SIBLING_CANDIDATE:"+bundle.SiblingCandidates[0].RelationID] != 2 || members["DISPATCH_ASSOCIATION:"+bundle.DispatchRelationships[0].RelationID] != 2 {
		t.Fatalf("ASSERT_MULTI_SEED_CANONICAL_RELATION_MEMBERSHIP: %#v", bundle.SeedMemberships)
	}
	bundle.Edges[0].RelationID = "sha256:forged"
	canonical, _ := json.Marshal(bundle.semanticV3)
	bundle.TraceReceipt.SemanticCommitmentDigest = domainDigest(SemanticDigestDomain, canonical)
	mutated, _ := json.Marshal(bundle)
	if err := ValidateSemanticBundle(mutated); err == nil || !strings.Contains(err.Error(), "relation") {
		t.Fatalf("ASSERT_PRIMARY_RECEIPT_RELATION_CONSISTENCY: %v", err)
	}
}

func TestV3RelationsAndMembershipsShareStableExecutionBundleIdentity(t *testing.T) {
	caller := NewNode(Item{Name: "caller", URI: "file:///w/caller.go"})
	callee := NewNode(Item{Name: "callee", URI: "file:///w/callee.go"})
	candidate := NewNode(Item{Name: "candidate", URI: "file:///w/candidate.go"})
	edge := Edge{CallerNodeID: caller.ID, CalleeNodeID: callee.ID}
	seed := InvocationSeed{Label: "seed", At: "a.go:1:1", ResolvedURI: "file:///w/a.go", ContentSHA256: "sha256:content"}
	result := Result{
		SchemaVersion: SchemaVersionV3,
		Invocation: Invocation{Seeds: []InvocationSeed{seed}, Provenance: InvocationProvenance{
			InvocationID: "invocation-1", Caller: "test", Source: "fixture", SourceRevision: "rev-1",
		}},
		Nodes: []Node{caller, callee}, Edges: []Edge{edge},
		Seeds:                 []SeedResult{{Label: seed.Label, ReachedEdges: []Edge{edge}, ReachedRelationIDs: []string{canonicalRelationID("CALL_RELATION", "CALLER_TO_CALLEE", caller.ID+"->"+callee.ID, "", "", "", caller.ID, callee.ID)}}},
		SiblingCandidates:     []SiblingCandidate{{SeedLabel: seed.Label, Candidate: candidate}},
		DispatchRelationships: []DispatchRelationship{{SeedLabel: seed.Label, Interface: caller, Implementation: callee}},
		Summary:               Summary{Complete: true},
	}
	marshal := func(r Result) bundleV3 {
		t.Helper()
		encoded, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		var bundle bundleV3
		if err := json.Unmarshal(encoded, &bundle); err != nil {
			t.Fatal(err)
		}
		return bundle
	}

	first := marshal(result)
	if first.ExecutionBundleID == "" || len(first.Edges) != 1 || len(first.SiblingCandidates) != 1 || len(first.DispatchRelationships) != 1 || len(first.SeedMemberships) != 3 {
		t.Fatalf("ASSERT_EXECUTION_BUNDLE_ID_PRESENT: bundle=%#v", first)
	}
	if first.Edges[0].ExecutionBundleID != first.ExecutionBundleID || first.SiblingCandidates[0].ExecutionBundleID != first.ExecutionBundleID || first.DispatchRelationships[0].ExecutionBundleID != first.ExecutionBundleID {
		t.Fatalf("ASSERT_SHARED_EXECUTION_BUNDLE_RELATION_ID: bundle=%q edge=%q sibling=%q dispatch=%q", first.ExecutionBundleID, first.Edges[0].ExecutionBundleID, first.SiblingCandidates[0].ExecutionBundleID, first.DispatchRelationships[0].ExecutionBundleID)
	}
	for _, membership := range first.SeedMemberships {
		if membership.ExecutionBundleID != first.ExecutionBundleID {
			t.Fatalf("ASSERT_SHARED_EXECUTION_BUNDLE_MEMBERSHIP_ID: bundle=%q membership=%#v", first.ExecutionBundleID, membership)
		}
	}
	originalRelationID, originalMembershipID := first.Edges[0].RelationID, first.SeedMemberships[0].MembershipID
	second := marshal(result)
	if second.ExecutionBundleID != first.ExecutionBundleID || second.Edges[0].RelationID != originalRelationID || second.SeedMemberships[0].MembershipID != originalMembershipID {
		t.Fatalf("ASSERT_EXECUTION_BUNDLE_ID_STABLE_AND_NON_CIRCULAR: first=%#v second=%#v", first, second)
	}
	result.Invocation.Provenance.InvocationID = "invocation-2"
	distinct := marshal(result)
	if distinct.ExecutionBundleID == first.ExecutionBundleID {
		t.Fatalf("ASSERT_DISTINCT_EXECUTION_BUNDLE_ID: %q", first.ExecutionBundleID)
	}
	firstSemantics, _ := json.Marshal(first.EvidenceSemantics)
	distinctSemantics, _ := json.Marshal(distinct.EvidenceSemantics)
	if !bytes.Equal(distinctSemantics, firstSemantics) {
		t.Fatalf("ASSERT_EXECUTION_BUNDLE_SEMANTIC_NEUTRALITY: first=%s distinct=%s", firstSemantics, distinctSemantics)
	}
	legacy := first
	legacy.ExecutionBundleID = ""
	legacy.Edges = append([]Edge(nil), first.Edges...)
	legacy.SiblingCandidates = append([]SiblingCandidate(nil), first.SiblingCandidates...)
	legacy.DispatchRelationships = append([]DispatchRelationship(nil), first.DispatchRelationships...)
	legacy.SeedMemberships = append([]SeedMembership(nil), first.SeedMemberships...)
	for i := range legacy.Edges {
		legacy.Edges[i].ExecutionBundleID = ""
	}
	for i := range legacy.SiblingCandidates {
		legacy.SiblingCandidates[i].ExecutionBundleID = ""
	}
	for i := range legacy.DispatchRelationships {
		legacy.DispatchRelationships[i].ExecutionBundleID = ""
	}
	for i := range legacy.SeedMemberships {
		legacy.SeedMemberships[i].ExecutionBundleID = ""
	}
	legacyCanonical, _ := json.Marshal(legacy.semanticV3)
	legacy.TraceReceipt.SemanticCommitmentDigest = domainDigest(SemanticDigestDomain, legacyCanonical)
	legacyBytes, _ := json.Marshal(legacy)
	if err := ValidateSemanticBundle(legacyBytes); err != nil {
		t.Fatalf("ASSERT_HISTORICAL_V3_WITHOUT_EXECUTION_BUNDLE_ID: %v", err)
	}

	first.Edges[0].ExecutionBundleID = "sha256:forged"
	canonical, _ := json.Marshal(first.semanticV3)
	first.TraceReceipt.SemanticCommitmentDigest = domainDigest(SemanticDigestDomain, canonical)
	forged, _ := json.Marshal(first)
	if err := ValidateSemanticBundle(forged); err == nil || !strings.Contains(err.Error(), "execution bundle relation mismatch") {
		t.Fatalf("ASSERT_EXECUTION_BUNDLE_FOREIGN_KEY_VALIDATION: %v", err)
	}
}

func TestProcessContextNormalizesEquivalentWorkingDirectories(t *testing.T) {
	a := projectProcessContext(nil, nil, "/tmp/work/../work")
	b := projectProcessContext(nil, nil, "/tmp/work")
	if a.WorkingDirectoryProcessContextDigest != b.WorkingDirectoryProcessContextDigest {
		t.Fatalf("ASSERT_PORTABLE_CWD_IDENTITY: %s != %s", a.WorkingDirectoryProcessContextDigest, b.WorkingDirectoryProcessContextDigest)
	}
}

func TestValidateSemanticBundleAcceptsHistoricalV3WithoutAdditiveProvenance(t *testing.T) {
	n := NewNode(Item{Name: "legacy", URI: "file:///w/legacy.go", SelectionRange: Range{End: Position{Line: 1}}})
	encoded, err := json.Marshal(Result{
		SchemaVersion: SchemaVersionV3,
		Invocation:    Invocation{Seeds: []InvocationSeed{{Label: "seed", At: "legacy.go:1:1"}}},
		Nodes:         []Node{n},
		Seeds:         []SeedResult{{Label: "seed", ReachedNodeIDs: []string{n.ID}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var legacy bundleV3
	if err := json.Unmarshal(encoded, &legacy); err != nil {
		t.Fatal(err)
	}
	legacy.ExecutionBundleID = ""
	legacy.Identity.ToolVersionProvenanceClass = ""
	legacy.Identity.ServerVersionProvenanceClass = ""
	for i := range legacy.Edges {
		legacy.Edges[i].ExecutionBundleID = ""
	}
	for i := range legacy.SiblingCandidates {
		legacy.SiblingCandidates[i].ExecutionBundleID = ""
	}
	for i := range legacy.DispatchRelationships {
		legacy.DispatchRelationships[i].ExecutionBundleID = ""
	}
	for i := range legacy.SeedMemberships {
		legacy.SeedMemberships[i].ExecutionBundleID = ""
	}
	for i := range legacy.PortableLocators {
		legacy.PortableLocators[i].Provenance = nil
	}
	canonical, err := json.Marshal(legacy.semanticV3)
	if err != nil {
		t.Fatal(err)
	}
	legacy.TraceReceipt = semanticReceiptV3{"lsp-trace.semantic-receipt.v1", domainDigest(SemanticDigestDomain, canonical), SemanticDigestScope}
	legacyEncoded, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateSemanticBundle(legacyEncoded); err != nil {
		t.Fatalf("ASSERT_HISTORICAL_V3_WITHOUT_ADDITIVE_PROVENANCE: %v", err)
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
