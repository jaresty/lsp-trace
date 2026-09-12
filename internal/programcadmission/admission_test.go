package programcadmission_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
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
