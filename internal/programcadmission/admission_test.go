package programcadmission_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/programc"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/programccompose"
)

func capture(t *testing.T, suffix string, nodes []graph.Node, edges []graph.Edge) programccompose.Input {
	t.Helper()
	seed := graph.InvocationSeed{Label: "seed" + suffix, At: "a.go:1:1", ResolvedURI: "file:///w/a.go", ContentSHA256: "sha256:" + strings.Repeat("a", 64), LanguageID: "go"}
	r := graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{WorkspaceURI: "file:///w", Server: graph.ServerInvocation{Command: "gopls"}, LanguageID: "go", Seeds: []graph.InvocationSeed{seed}, Provenance: graph.InvocationProvenance{InvocationID: "invocation-" + suffix, SourceRevision: "commit", ServerVersion: "gopls@1"}}, Nodes: nodes, Edges: edges, Seeds: []graph.SeedResult{{Label: seed.Label}}, Summary: graph.Summary{Complete: false}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
	native, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := graphprovenance.CaptureV5(native, "session", 7, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable})
	if err != nil {
		t.Fatal(err)
	}
	s := sha256.Sum256(raw)
	return programccompose.Input{Bytes: raw, Identity: "capture-" + suffix, SHA256: fmt.Sprintf("sha256:%x", s), ByteLength: len(raw), ExactMetadata: programccompose.ExactMetadata{WorkspaceIdentity: "workspace-1", RevisionCustody: "CALLER_ASSERTED", PositionEncoding: "utf-16", AcquisitionSemantics: "managed-lsp-v1", PrivacyPolicy: "bundle-custodian-v1"}}
}

func node(name string) graph.Node {
	return graph.NewNode(graph.Item{Name: name, Kind: 12, URI: "file:///w/a.go", Range: graph.Range{End: graph.Position{Character: uint32(len(name))}}, SelectionRange: graph.Range{End: graph.Position{Character: uint32(len(name))}}})
}
func edge(a, b graph.Node) graph.Edge {
	return graph.Edge{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{End: graph.Position{Character: 1}}}}
}

func TestAdmissionDeterministicSeparateConservativeProjection(t *testing.T) {
	a, b, isolate := node("a"), node("b"), node("isolate")
	x := capture(t, "x", []graph.Node{a, isolate}, nil)
	y := capture(t, "y", []graph.Node{a, b}, []graph.Edge{edge(a, b)})
	one, err := programccompose.Compose([]programccompose.Input{x, y})
	if err != nil {
		t.Fatal(err)
	}
	two, err := programccompose.Compose([]programccompose.Input{y, x})
	if err != nil {
		t.Fatal(err)
	}
	validated, err := programccompose.Validate(one.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	got, err := programcadmission.Admit(one.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	permuted, err := programcadmission.Admit(two.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes, permuted.Bytes) || got.Artifact.AdmissionID != permuted.Artifact.AdmissionID {
		t.Fatal("ASSERT_COMPOSITE_ADMISSION_DETERMINISTIC")
	}
	if got.Artifact.AdmissionID == got.Artifact.CompositeID || got.Artifact.Authority != 0 || got.Artifact.SourceGraphComplete != "UNKNOWN" {
		t.Fatal("ASSERT_SEPARATE_ZERO_AUTHORITY_UNKNOWN_ADMISSION")
	}
	if len(got.Artifact.Constituents) != 2 || got.Artifact.Constituents[0].SHA256 == "" || got.Artifact.Constituents[0].Identity == "" || got.Artifact.Constituents[0].RevisionCustody == "" {
		t.Fatal("ASSERT_CONSTITUENT_IDENTITY_DIGEST_CUSTODY_RETAINED")
	}
	if got.Artifact.Constituents[0].BytesBase64 != validated.Constituents[0].BytesBase64 || got.Artifact.Constituents[0].SourcePolicy != validated.Constituents[0].SourcePolicy || got.Artifact.Constituents[0].WorkspaceURI != validated.Constituents[0].WorkspaceURI || got.Artifact.Constituents[0].AnalyzedVersion != validated.Constituents[0].AnalyzedVersion || got.Artifact.Constituents[0].DependencyCompleteness != validated.Constituents[0].DependencyCompleteness || got.Artifact.Constituents[0].CaptureBudget != validated.Constituents[0].CaptureBudget || !reflect.DeepEqual(got.Artifact.Constituents[0].Supplies, validated.Constituents[0].Supplies) || !reflect.DeepEqual(got.Artifact.Constituents[0].Captures, validated.Constituents[0].Captures) || !reflect.DeepEqual(got.Artifact.Constituents[0].Bindings, validated.Constituents[0].Bindings) || !reflect.DeepEqual(got.Artifact.Constituents[0].Invocation, validated.Constituents[0].Invocation) || !reflect.DeepEqual(got.Artifact.Constituents[0].Seeds, validated.Constituents[0].Seeds) || !reflect.DeepEqual(got.Artifact.Constituents[0].Frontier, validated.Constituents[0].Frontier) || !reflect.DeepEqual(got.Artifact.Constituents[0].Diagnostics, validated.Constituents[0].Diagnostics) || !reflect.DeepEqual(got.Artifact.Constituents[0].Summary, validated.Constituents[0].Summary) || !reflect.DeepEqual(got.Artifact.Constituents[0].Slice, validated.Constituents[0].Slice) {
		t.Fatal("ASSERT_COMPLETE_ORDERED_CONSTITUENT_BINDINGS_RETAINED")
	}
	for i, constituent := range validated.Constituents {
		if got.Artifact.Constituents[i].Identity != constituent.Identity || got.Artifact.Constituents[i].SHA256 != constituent.SHA256 || got.Artifact.Constituents[i].InvocationID != constituent.InvocationID {
			t.Fatal("ASSERT_CONSTITUENT_CANONICAL_ORDER_RETAINED")
		}
	}
	exposed := got.Admission.SourceBinding()
	exposed.Constituents[0].BytesBase64 = "tampered"
	if len(exposed.Constituents[0].Frontier) > 0 {
		exposed.Constituents[0].Frontier[0] ^= 1
	}
	stable := got.Admission.SourceBinding()
	if stable.Constituents[0].BytesBase64 != validated.Constituents[0].BytesBase64 || !reflect.DeepEqual(stable.Constituents[0].Frontier, validated.Constituents[0].Frontier) {
		t.Fatal("ASSERT_OPAQUE_ADMISSION_SOURCE_BINDING_DEEP_CLONED")
	}
	if len(got.Artifact.NodeIDs) != 3 || len(got.Artifact.Calls) != 1 {
		t.Fatalf("ASSERT_EXACT_CALL_UNION_AND_ISOLATES nodes=%d calls=%d", len(got.Artifact.NodeIDs), len(got.Artifact.Calls))
	}
	outcome, failure := programc.ComputeComposite(got.Admission, 19)
	if failure != nil {
		t.Fatal(failure)
	}
	foundIsolate := false
	for _, community := range outcome.Communities {
		for _, id := range community.Members {
			foundIsolate = foundIsolate || id == isolate.ID
		}
	}
	if !foundIsolate {
		t.Fatal("ASSERT_ISOLATE_SURVIVES_LEIDEN")
	}
	if outcome.Source.InputLength != 0 || len(outcome.Source.InputBytes()) != 0 {
		t.Fatal("ASSERT_NO_NATIVE_SINGLE_CAPTURE_RECEIPT_FORGED")
	}
	if outcome.ClaimCeiling != programccompose.ClaimCeiling {
		t.Fatal("ASSERT_COMPOSITE_CLAIM_CEILING_RETAINED")
	}
	if outcome.Projection.CompositeSource.CompositeID != got.Artifact.CompositeID || outcome.CompositeSource.CompositeID != got.Artifact.CompositeID {
		t.Fatal("ASSERT_COMPOSITE_SOURCE_IDENTITY_RETAINED")
	}
	if outcome.Projection.CompositeSource.ClaimCeiling != programccompose.ClaimCeiling || outcome.CompositeSource.ClaimCeiling != programccompose.ClaimCeiling {
		t.Fatal("ASSERT_COMPOSITE_SOURCE_CLAIM_CEILING_RETAINED")
	}
	if outcome.CompositeSource.Authority != 0 || outcome.CompositeSource.SourceGraphComplete != "UNKNOWN" || outcome.CompositeSource.Completeness.WholeWorkspace {
		t.Fatal("ASSERT_COMPOSITE_SOURCE_CONSERVATIVE_CEILINGS_RETAINED")
	}
	if !reflect.DeepEqual(outcome.CompositeSource.Constituents, got.Artifact.Constituents) || !reflect.DeepEqual(outcome.CompositeSource.Completeness.PerInput, got.Artifact.Completeness.PerInput) {
		t.Fatal("ASSERT_ORDERED_CONSTITUENT_AND_PER_INPUT_COMPLETENESS_RETAINED")
	}
}

func TestAdmissionPreservesDirectedOccurrencesAndSelfLoops(t *testing.T) {
	a, b, isolate := node("a"), node("b"), node("isolate")
	forward := edge(a, b)
	forward.CallSites = append(forward.CallSites, graph.Range{End: graph.Position{Character: 2}})
	reverse := edge(b, a)
	loop := edge(a, a)
	one := capture(t, "directed-one", []graph.Node{a, b, isolate}, []graph.Edge{forward, loop})
	two := capture(t, "directed-two", []graph.Node{a, b}, []graph.Edge{reverse})
	composite, err := programccompose.Compose([]programccompose.Input{two, one})
	if err != nil {
		t.Fatal(err)
	}
	admitted, err := programcadmission.Admit(composite.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	outcome, failure := programc.ComputeComposite(admitted.Admission, 23)
	if failure != nil {
		t.Fatal(failure)
	}
	if len(outcome.Projection.Occurrences) != 4 || len(admitted.Artifact.Calls) != 4 {
		t.Fatalf("ASSERT_CALL_SITE_MULTIPLICITY_RETAINED occurrences=%d calls=%d", len(outcome.Projection.Occurrences), len(admitted.Artifact.Calls))
	}
	ids := outcome.Projection.NodeIDs
	if outcome.Projection.PairWeights[programc.Pair{From: ids[a.ID], To: ids[b.ID]}] != 2 || outcome.Projection.PairWeights[programc.Pair{From: ids[b.ID], To: ids[a.ID]}] != 1 || outcome.Projection.PairWeights[programc.Pair{From: ids[a.ID], To: ids[a.ID]}] != 1 {
		t.Fatal("ASSERT_DIRECTED_AND_SELF_LOOP_CALLS_RETAINED")
	}
	foundIsolate := false
	for _, community := range outcome.Communities {
		for _, id := range community.Members {
			foundIsolate = foundIsolate || id == isolate.ID
		}
	}
	if !foundIsolate {
		t.Fatal("ASSERT_ISOLATE_RETAINED_WITH_DIRECTED_CALLS")
	}
}

func TestAdmissionRejectsTamperConflictAndCrossCaptureInference(t *testing.T) {
	a, b := node("a"), node("b")
	x, y := capture(t, "x", []graph.Node{a}, nil), capture(t, "y", []graph.Node{b}, nil)
	composite, err := programccompose.Compose([]programccompose.Input{x, y})
	if err != nil {
		t.Fatal(err)
	}
	got, err := programcadmission.Admit(composite.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Artifact.Calls) != 0 {
		t.Fatal("ASSERT_NO_CROSS_CAPTURE_CALL_INFERENCE")
	}
	tampered := append([]byte(nil), composite.Bytes...)
	tampered[len(tampered)/2] ^= 1
	if _, err := programcadmission.Admit(tampered); err == nil {
		t.Fatal("ASSERT_EXACT_COMPOSITE_REVALIDATION")
	}
	conflicting := x
	conflicting.Identity = y.Identity
	if _, err := programccompose.Compose([]programccompose.Input{y, conflicting}); err == nil {
		t.Fatal("ASSERT_CONSTITUENT_CONFLICT_REJECTED")
	}
	if _, failure := programc.Compute(composite.Bytes, 1); failure == nil {
		t.Fatal("ASSERT_NATIVE_ADMISSION_REMAINS_V5_ONLY")
	}
	if _, failure := programc.ComputeComposite(programcadmission.CompositeProjectionAdmission{}, 1); failure == nil {
		t.Fatal("ASSERT_ZERO_OPAQUE_ADMISSION_REJECTED")
	}
}
