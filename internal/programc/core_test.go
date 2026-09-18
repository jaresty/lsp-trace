package programc

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
)

func TestCloneOutcomeDoesNotAliasExportedProjectionState(t *testing.T) {
	in := Outcome{
		Communities: []Community{{Members: []string{"a"}}},
		Projection: Projection{
			NodeIdentities: []string{"a", "b"},
			NodeIDs:        map[string]int64{"a": 0, "b": 1},
			Occurrences:    []Occurrence{{Identity: "occurrence", From: 0, To: 1}},
			PairWeights:    map[Pair]float64{{From: 0, To: 1}: 1},
		},
	}
	out := CloneOutcome(in)
	out.Communities[0].Members[0] = "changed"
	out.Projection.NodeIdentities[0] = "changed"
	out.Projection.NodeIDs["a"] = 9
	out.Projection.Occurrences[0].Identity = "changed"
	out.Projection.PairWeights[Pair{From: 0, To: 1}] = 9
	if in.Communities[0].Members[0] != "a" || in.Projection.NodeIdentities[0] != "a" || in.Projection.NodeIDs["a"] != 0 || in.Projection.Occurrences[0].Identity != "occurrence" || in.Projection.PairWeights[Pair{From: 0, To: 1}] != 1 {
		t.Fatal("ASSERT_CLONE_OUTCOME_EXPORTED_PROJECTION_NO_ALIAS")
	}
}

func validV5(t *testing.T, nodes []graph.Node, edges []graph.Edge) []byte {
	t.Helper()
	seed := graph.InvocationSeed{Label: "seed", At: "a.go:1:1", ResolvedURI: "file:///w/a.go", ContentSHA256: "sha256:" + strings.Repeat("a", 64), LanguageID: "go"}
	r := graph.Result{
		SchemaVersion: graph.SchemaVersionV5,
		Invocation:    graph.Invocation{Server: graph.ServerInvocation{Command: "gopls"}, Seeds: []graph.InvocationSeed{seed}, Provenance: graph.InvocationProvenance{InvocationID: "session", SourceRevision: "commit", ServerVersion: "gopls@1"}, Expansion: graph.ExpansionConfig{TopmostSiblings: true}},
		Nodes:         nodes, Edges: edges, Seeds: []graph.SeedResult{{Label: "seed"}}, Summary: graph.Summary{Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true},
	}
	native, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := graphprovenance.CaptureV5(native, "session", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}})
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	return raw
}

func decodeV5(t *testing.T, raw []byte) (graphprovenance.EvidenceV5, []byte) {
	t.Helper()
	var envelope graphprovenance.EvidenceV5
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	native, err := base64.StdEncoding.DecodeString(envelope.GraphV5)
	if err != nil {
		t.Fatal(err)
	}
	return envelope, native
}

func node(name string, line uint32) graph.Node {
	r := graph.Range{Start: graph.Position{Line: line}, End: graph.Position{Line: line, Character: 1}}
	return graph.NewNode(graph.Item{Name: name, Kind: 12, URI: "file:///w/a.go", Range: r, SelectionRange: r})
}

func TestAdmissionRejectsMalformedAndUnqualifiedCalls(t *testing.T) {
	if _, err := Compute([]byte(`{"schema_version":"lsp-trace.graph-provenance.v5"`), 7); err == nil || err.Code != CodeInvalidProvenance {
		t.Fatalf("ASSERT_V5_MALFORMED_REJECTED: err=%#v", err)
	}
	a, b := node("a", 0), node("b", 1)
	raw := validV5(t, []graph.Node{a, b}, []graph.Edge{{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{Start: graph.Position{Line: 4}, End: graph.Position{Line: 4, Character: 1}}}}})
	var envelope map[string]any
	_ = json.Unmarshal(raw, &envelope)
	native, _ := base64.StdEncoding.DecodeString(envelope["graph_v5"].(string))
	var doc map[string]any
	_ = json.Unmarshal(native, &doc)
	receipt := doc["evidence_receipt"].(map[string]any)
	receipt["relations"].([]any)[0].(map[string]any)["evidence_class"] = "DISCOVERY_NOMINATION"
	changed, _ := json.Marshal(doc)
	envelope["graph_v5"] = base64.StdEncoding.EncodeToString(changed)
	sum := sha256.Sum256(changed)
	envelope["graph_v5_sha256"] = fmt.Sprintf("sha256:%x", sum)
	bad, _ := json.Marshal(envelope)
	if _, err := Compute(bad, 7); err == nil || err.Code != CodeInvalidProvenance {
		t.Fatalf("ASSERT_CALLS_CUSTODY_EVIDENCE_REJECTED: err=%#v", err)
	}
}

func TestSourceBindingRetainsExactValidatedV5Evidence(t *testing.T) {
	a, b := node("a", 0), node("b", 1)
	raw := validV5(t, []graph.Node{a, b}, []graph.Edge{{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{}}}})
	envelope, native := decodeV5(t, raw)

	projection, failure := Project(raw)
	if failure != nil {
		t.Fatal(failure)
	}
	binding := projection.Source
	inputSum := sha256.Sum256(raw)
	if !bytes.Equal(binding.InputBytes(), raw) || binding.InputLength != len(raw) || binding.InputSHA256 != fmt.Sprintf("sha256:%x", inputSum) ||
		!bytes.Equal(binding.GraphV5Bytes(), native) || binding.GraphV5Length != len(native) || binding.GraphV5SHA256 != envelope.GraphV5SHA256 {
		t.Fatalf("ASSERT_SOURCE_EXACT_BYTE_DIGEST_LENGTH_IDENTITY: binding=%#v", binding)
	}
	if binding.EnvelopeSchemaVersion != graphprovenance.VersionV5 || binding.GraphSchemaVersion != graph.SchemaVersionV5 || binding.GraphSchemaID != graphprovenance.GraphV5SchemaID ||
		binding.EnvelopeSchemaIDAvailable || binding.EnvelopeSchemaID != "" || binding.SessionID != "session" || binding.Generation != 1 {
		t.Fatalf("ASSERT_SOURCE_SESSION_GENERATION_SCHEMA_IDENTITIES: binding=%#v", binding)
	}
	if binding.Sensitivity().AccessControlResponsibility != "BUNDLE_CUSTODIAN" || binding.Completeness.SourceGraphComplete != graph.Unknown ||
		!binding.Completeness.TraversalComplete || binding.Completeness.Truncated || binding.DiagnosticsStatus != manageddiagnostic.QueryUnavailable ||
		binding.ReplayManifest().ReplayInputManifestDigest == "" || binding.Semantics().CallEdges.EvidenceClass != "SERVER_REPORTED_CALL_HIERARCHY" ||
		binding.SemanticReceipt.SemanticCommitmentDigest == "" || binding.ExecutionBundleID == "" || binding.BundleIdentity().CallerProvenanceClass != "CALLER_ASSERTED" || len(binding.AdmissionReceipt().Relations) != 1 {
		t.Fatalf("ASSERT_SOURCE_PRIVACY_COMPLETENESS_ADMISSION_IDENTITIES: access=%q source_complete=%q traversal=%t truncated=%t diagnostic=%q replay=%q evidence=%q semantic=%q bundle=%q relations=%d",
			binding.Sensitivity().AccessControlResponsibility, binding.Completeness.SourceGraphComplete, binding.Completeness.TraversalComplete,
			binding.Completeness.Truncated, binding.DiagnosticsStatus, binding.ReplayManifest().ReplayInputManifestDigest,
			binding.Semantics().CallEdges.EvidenceClass, binding.SemanticReceipt.SemanticCommitmentDigest, binding.ExecutionBundleID, len(binding.AdmissionReceipt().Relations))
	}
	if binding.CustodyAvailable || binding.CustodyClass != "" || binding.QualificationIdentityAvailable || binding.CallEvidenceClass != "SERVER_REPORTED_CALL_HIERARCHY" || binding.CallEvidenceRole != "CALL_SUPPORT" {
		t.Fatalf("ASSERT_CALL_EVIDENCE_NOT_PROMOTED_TO_CUSTODY: binding=%#v", binding)
	}

	wantRetained := append([]byte(nil), raw...)
	wantNative := append([]byte(nil), native...)
	returned := binding.InputBytes()
	returnedNative := binding.GraphV5Bytes()
	raw[0] ^= 0xff
	returned[1] ^= 0xff
	returnedNative[0] ^= 0xff
	if bytes.Equal(binding.InputBytes(), raw) || bytes.Equal(binding.InputBytes(), returned) || !bytes.Equal(binding.InputBytes(), wantRetained) ||
		bytes.Equal(binding.GraphV5Bytes(), returnedNative) || !bytes.Equal(binding.GraphV5Bytes(), wantNative) {
		t.Fatalf("ASSERT_SOURCE_INPUT_DEFENSIVE_COPY: retained bytes changed")
	}
	out, failure := Compute(validV5(t, []graph.Node{a, b}, []graph.Edge{{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{}}}}), 7)
	if failure != nil || out.Source.SessionID != "session" || out.Projection.Source.SessionID != out.Source.SessionID {
		t.Fatalf("ASSERT_OUTCOME_SOURCE_BINDING_PRESERVED: outcome=%#v failure=%v", out, failure)
	}
}

func TestAdmissionRejectsResealedSemanticTampering(t *testing.T) {
	a, b := node("a", 0), node("b", 1)
	raw := validV5(t, []graph.Node{a, b}, []graph.Edge{{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{}}}})
	var envelope map[string]any
	_ = json.Unmarshal(raw, &envelope)
	native, _ := base64.StdEncoding.DecodeString(envelope["graph_v5"].(string))
	var doc map[string]any
	_ = json.Unmarshal(native, &doc)
	doc["sensitivity_policy"].(map[string]any)["automatic_redaction"] = true
	changed, _ := json.Marshal(doc)
	envelope["graph_v5"] = base64.StdEncoding.EncodeToString(changed)
	sum := sha256.Sum256(changed)
	envelope["graph_v5_sha256"] = fmt.Sprintf("sha256:%x", sum)
	bad, _ := json.Marshal(envelope)
	if _, failure := Project(bad); failure == nil || failure.Code != CodeInvalidProvenance {
		t.Fatalf("ASSERT_RESEALED_SEMANTIC_TAMPERING_REJECTED: failure=%#v", failure)
	}
}

func TestProjectionPreservesOccurrencesLoopsMultiplicityAndStableIDs(t *testing.T) {
	z, a := node("z", 2), node("a", 0)
	sites := []graph.Range{{Start: graph.Position{Line: 3}, End: graph.Position{Line: 3, Character: 1}}, {Start: graph.Position{Line: 5}, End: graph.Position{Line: 5, Character: 1}}}
	raw := validV5(t, []graph.Node{z, a}, []graph.Edge{{CallerNodeID: z.ID, CalleeNodeID: z.ID, CallSites: sites}, {CallerNodeID: z.ID, CalleeNodeID: a.ID, CallSites: sites[:1]}})
	got, failure := Project(raw)
	if failure != nil {
		t.Fatal(failure)
	}
	if len(got.Occurrences) != 3 || got.PairWeights[Pair{From: got.NodeIDs[z.ID], To: got.NodeIDs[z.ID]}] != 2 || got.Occurrences[0].Weight != 1 {
		t.Fatalf("ASSERT_OCCURRENCES_SELF_LOOP_MULTIPLICITY_PRESERVED: projection=%#v", got)
	}
	if got.NodeIDs[a.ID] != 0 || got.NodeIDs[z.ID] != 1 {
		t.Fatalf("ASSERT_STABLE_SORTED_NODE_IDS: ids=%#v", got.NodeIDs)
	}
}

func TestCapsAcceptEqualityRejectExcess(t *testing.T) {
	if failure := enforceCaps(MaxNodes, MaxOccurrences); failure != nil {
		t.Fatalf("ASSERT_CAP_EQUALITY_ACCEPTED_EXCESS_REJECTED: equality=%v", failure)
	}
	for _, tc := range []struct{ n, o int }{{MaxNodes + 1, 0}, {0, MaxOccurrences + 1}} {
		if failure := enforceCaps(tc.n, tc.o); failure == nil || failure.Code != CodeResourceLimit {
			t.Fatalf("ASSERT_CAP_EQUALITY_ACCEPTED_EXCESS_REJECTED: n=%d o=%d failure=%#v", tc.n, tc.o, failure)
		}
	}
}

func TestComputeEdgelessIsExactTypedEmptySingletonPartition(t *testing.T) {
	a, b, c := node("a", 0), node("b", 1), node("c", 2)
	raw := validV5(t, []graph.Node{c, a, b}, nil)
	out, failure := Compute(raw, 19)
	if failure != nil {
		t.Fatalf("ASSERT_EDGELESS_TYPED_EMPTY_PARTITION: failure=%v", failure)
	}
	if len(out.Projection.Occurrences) != 0 || !bytes.Equal(out.Source.InputBytes(), raw) {
		t.Fatalf("ASSERT_EDGELESS_SOURCE_NO_INVENTED_CALLS: occurrences=%d source_equal=%t", len(out.Projection.Occurrences), bytes.Equal(out.Source.InputBytes(), raw))
	}
	want := []Community{{Members: []string{a.ID}}, {Members: []string{b.ID}}, {Members: []string{c.ID}}}
	sort.Slice(want, func(i, j int) bool { return compareStrings(want[i].Members, want[j].Members) < 0 })
	if !equalCommunities(out.Communities, want) {
		t.Fatalf("ASSERT_EDGELESS_TYPED_EMPTY_PARTITION: got=%#v want=%#v", out.Communities, want)
	}
	if out.Outcome != "EMPTY" {
		t.Fatalf("ASSERT_EDGELESS_TYPED_EMPTY_PARTITION: outcome=%q", out.Outcome)
	}
}

func TestComputeCanonicalizesGonumEmptySlotsWithoutDroppingIsolates(t *testing.T) {
	a, b, c, d := node("a", 0), node("b", 1), node("c", 2), node("d", 3)
	edge := graph.Edge{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{}}}
	first, failure := Compute(validV5(t, []graph.Node{d, b, c, a}, []graph.Edge{edge}), 19)
	if failure != nil {
		t.Fatalf("ASSERT_ISOLATE_CANONICAL_COVERAGE: failure=%v", failure)
	}
	second, failure := Compute(validV5(t, []graph.Node{a, c, b, d}, []graph.Edge{edge}), 19)
	if failure != nil {
		t.Fatalf("ASSERT_ISOLATE_CANONICAL_COVERAGE: permuted failure=%v", failure)
	}
	if !validCanonicalCommunities(first.Communities, first.Projection.NodeIdentities) || !equalCommunities(first.Communities, second.Communities) || first.LogicalDigest != second.LogicalDigest {
		t.Fatalf("ASSERT_ISOLATE_CANONICAL_COVERAGE: first=%#v second=%#v", first.Communities, second.Communities)
	}
	for _, community := range first.Communities {
		if len(community.Members) == 0 {
			t.Fatal("ASSERT_EMPTY_GONUM_SLOT_REMOVED: empty community retained")
		}
	}
}

func TestProgramCSharedGraphKernelNoDrift(t *testing.T) {
	a, b, c := node("a", 0), node("b", 1), node("c", 2)
	edges := []graph.Edge{
		{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{}}},
		{CallerNodeID: b.ID, CalleeNodeID: c.ID, CallSites: []graph.Range{{}}},
	}
	out, failure := Compute(validV5(t, []graph.Node{c, a, b}, edges), 19)
	if failure != nil {
		t.Fatal(failure)
	}
	const wantDigest = "sha256:d8ae702930e8ddaa109dec1103fd09196e3a19fe33bb24694e3ac7aad1ec1fd0"
	if out.LogicalDigest != wantDigest ||
		out.Algorithm != algorithm || out.ProfileID != ProfileID || out.ProfileDigest != ProfileDigest || out.ClaimCeiling != ClaimCeiling {
		t.Fatalf("ASSERT_PROGRAM_C_SHARED_KERNEL_NO_DRIFT outcome=%#v", out)
	}
}

func TestComputeDeterministicAcrossRepeatAndPermutation(t *testing.T) {
	a, b, c := node("a", 0), node("b", 1), node("c", 2)
	edges := []graph.Edge{{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{}}}, {CallerNodeID: b.ID, CalleeNodeID: c.ID, CallSites: []graph.Range{{}}}}
	first, failure := Compute(validV5(t, []graph.Node{c, a, b}, edges), 19)
	if failure != nil {
		t.Fatal(failure)
	}
	second, failure := Compute(validV5(t, []graph.Node{b, c, a}, []graph.Edge{edges[1], edges[0]}), 19)
	if failure != nil {
		t.Fatal(failure)
	}
	if first.LogicalDigest != second.LogicalDigest || !equalCommunities(first.Communities, second.Communities) {
		t.Fatalf("ASSERT_REPEAT_PERMUTATION_DETERMINISTIC: first=%#v second=%#v", first, second)
	}
	if first.Algorithm != "gonum.org/v1/gonum/graph/community.Leiden" || first.Resolution != 1 || first.Seed != 19 {
		t.Fatalf("ASSERT_EXACT_GONUM_LEIDEN_KERNEL: %#v", first)
	}
	const wantDigest = "sha256:d8ae702930e8ddaa109dec1103fd09196e3a19fe33bb24694e3ac7aad1ec1fd0"
	if first.LogicalDigest != wantDigest {
		t.Fatalf("ASSERT_CANONICAL_LOGICAL_DIGEST: got=%q want=%q", first.LogicalDigest, wantDigest)
	}
	if first.ProfileID != ProfileID || first.ProfileDigest != ProfileDigest || first.ClaimCeiling != ClaimCeiling {
		t.Fatalf("ASSERT_TYPED_CLAIM_CEILING_AND_SURFACE_UNCHANGED: %#v", first)
	}
}
