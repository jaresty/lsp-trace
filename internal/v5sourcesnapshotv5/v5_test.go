package v5sourcesnapshotv5

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
)

func TestDirectV5SelectedCustodyContract(t *testing.T) {
	workspace := t.TempDir()
	targetPath := filepath.Join(workspace, "target.go")
	callerPath := filepath.Join(workspace, "caller.go")
	if err := os.WriteFile(targetPath, []byte("package p\nfunc Target() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(callerPath, []byte("package p\nfunc Caller() { Target() }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	carrier, target, caller, relationID := directFixture(t, workspace, targetPath, callerPath)
	limits := testLimits()
	result, err := CaptureSelected(CaptureInput{GraphV5Bytes: carrier, Workspace: workspace, PositionEncoding: "utf-16", Nominations: []Nomination{{ID: "n1", TargetNodeID: target.ID, Incoming: []Occurrence{{RelationID: relationID, OccurrenceID: "o1", CallerNodeID: caller.ID, CalleeNodeID: target.ID, Range: graph.Range{Start: graph.Position{Line: 1, Character: 16}, End: graph.Position{Line: 1, Character: 22}}}}}}, Limits: limits})
	if err != nil {
		t.Fatalf("ASSERT_V5_DIRECT_CAPTURE: %v", err)
	}
	if len(result.Outcomes) != 2 || result.CapturedCount != 2 || result.UnavailableCount != 0 {
		t.Fatalf("ASSERT_V5_DIRECT_ENDPOINT_CLOSURE: %#v", result)
	}
	if version, err := Validate(result.Raw, limits); err != nil || version != Version {
		t.Fatalf("ASSERT_V5_DIRECT_VALIDATE: %q %v", version, err)
	}
	var artifact Artifact
	if err := json.Unmarshal(result.Raw, &artifact); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(artifact.GraphV5Bytes, carrier) || artifact.SessionID != "session" || artifact.Generation != 1 || artifact.PositionEncoding != "utf-16" || artifact.Authority != 0 || artifact.Accepted || artifact.Completeness != "UNKNOWN" {
		t.Fatalf("ASSERT_V5_DIRECT_EXACT_IDENTITY_AUTHORITY: %#v", artifact)
	}
	mutated := append([]byte(nil), result.Raw...)
	mutated = bytes.Replace(mutated, []byte(`"generation":1`), []byte(`"generation":2`), 1)
	if _, err := Validate(mutated, limits); err == nil {
		t.Fatal("ASSERT_V5_DIRECT_COORDINATED_SUBSTITUTION_REJECTED")
	}
}

func TestDirectV5MixedAvailabilityAndReplayAfterWorkspaceDeletion(t *testing.T) {
	workspace := t.TempDir()
	targetPath := filepath.Join(workspace, "target.go")
	callerPath := filepath.Join(workspace, "caller.go")
	if err := os.WriteFile(targetPath, []byte("package p\nfunc Target() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(callerPath, []byte("package p\nfunc Caller() { Target() }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	carrier, target, caller, relationID := directFixture(t, workspace, targetPath, callerPath)
	if err := os.Remove(callerPath); err != nil {
		t.Fatal(err)
	}
	limits := testLimits()
	result, err := CaptureSelected(CaptureInput{GraphV5Bytes: carrier, Workspace: workspace, PositionEncoding: "utf-16", Nominations: []Nomination{{ID: "n1", TargetNodeID: target.ID, Incoming: []Occurrence{{RelationID: relationID, OccurrenceID: "o1", CallerNodeID: caller.ID, CalleeNodeID: target.ID, Range: graph.Range{Start: graph.Position{Line: 1, Character: 16}, End: graph.Position{Line: 1, Character: 22}}}}}}, Limits: limits})
	if err != nil {
		t.Fatalf("ASSERT_V5_DIRECT_MIXED_CAPTURE: %v", err)
	}
	if result.CapturedCount != 1 || result.UnavailableCount != 1 || len(result.Outcomes) != 2 {
		t.Fatalf("ASSERT_V5_DIRECT_MIXED_CLOSURE: %#v", result)
	}
	if err := os.RemoveAll(workspace); err != nil {
		t.Fatal(err)
	}
	raw, lookup, err := Replay(result.Raw, limits)
	if err != nil || !bytes.Equal(raw, result.Raw) {
		t.Fatalf("ASSERT_V5_DIRECT_WORKSPACE_FREE_REPLAY: %v", err)
	}
	if _, err := lookup.Get(result.Objects[0].Identity); err != nil {
		t.Fatalf("ASSERT_V5_DIRECT_EMBEDDED_RECEIPT_REPLAY: %v", err)
	}
}

func directFixture(t *testing.T, workspace, targetPath, callerPath string) ([]byte, graph.Node, graph.Node, string) {
	t.Helper()
	targetURI := "file://" + targetPath
	callerURI := "file://" + callerPath
	target := graph.NewNode(graph.Item{Name: "Target", Kind: 12, URI: targetURI, Range: graph.Range{Start: graph.Position{Line: 1}, End: graph.Position{Line: 1, Character: 16}}, SelectionRange: graph.Range{Start: graph.Position{Line: 1, Character: 5}, End: graph.Position{Line: 1, Character: 11}}})
	caller := graph.NewNode(graph.Item{Name: "Caller", Kind: 12, URI: callerURI, Range: graph.Range{Start: graph.Position{Line: 1}, End: graph.Position{Line: 1, Character: 26}}, SelectionRange: graph.Range{Start: graph.Position{Line: 1, Character: 5}, End: graph.Position{Line: 1, Character: 11}}})
	edge := graph.Edge{RelationID: "r1", CallerNodeID: caller.ID, CalleeNodeID: target.ID, CallSites: []graph.Range{{Start: graph.Position{Line: 1, Character: 16}, End: graph.Position{Line: 1, Character: 22}}}}
	native, err := json.Marshal(graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{Server: graph.ServerInvocation{Command: "fake-lsp"}, Provenance: graph.InvocationProvenance{InvocationID: "session", SourceRevision: "commit", ServerVersion: "fake@1"}}, Nodes: []graph.Node{caller, target}, Edges: []graph.Edge{edge}, Summary: graph.Summary{Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}})
	if err != nil {
		t.Fatal(err)
	}
	source := graphprovenance.EvidenceV2{SchemaVersion: graphprovenance.VersionV2, Policy: graphprovenance.PolicyV2, WorkspaceURI: "file://" + workspace, AnalyzedVersion: graphprovenance.Unverified, DependencyCompleteness: "UNKNOWN_INCOMPLETE", Supplies: []graphprovenance.SupplyReceiptV2{}, Captures: []graphprovenance.Receipt{}, Bindings: []graphprovenance.BindingV2{}}
	carrier, err := graphprovenance.CaptureV5(native, "session", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}}, &source)
	if err != nil {
		t.Fatal(err)
	}
	var retained graphprovenance.EvidenceV5
	if err := json.Unmarshal(carrier, &retained); err != nil {
		t.Fatal(err)
	}
	retainedNative, err := base64.StdEncoding.DecodeString(retained.GraphV5)
	if err != nil {
		t.Fatal(err)
	}
	var retainedGraph graph.Result
	if err := json.Unmarshal(retainedNative, &retainedGraph); err != nil {
		t.Fatal(err)
	}
	if len(retainedGraph.Edges) != 1 {
		t.Fatalf("fixture edge count: %d", len(retainedGraph.Edges))
	}
	return carrier, target, caller, retainedGraph.Edges[0].RelationID
}

func testLimits() Limits {
	return Limits{MaxArtifactBytes: 16 << 20, MaxGraphBytes: 8 << 20, MaxReceipts: 100, MaxSourceBytes: 1 << 20, MaxTotalSourceBytes: 4 << 20, MaxBindings: 100, MaxOutcomes: 100, MaxWork: 1000}
}
