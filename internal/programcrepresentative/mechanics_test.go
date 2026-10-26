package programcrepresentative_test

import (
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/programc"
	"lsp-trace/internal/programcrepresentative"
)

func TestNewSingletonMechanicsRejectsMultipleMembershipGroups(t *testing.T) {
	node := func(name string, line uint32) graph.Node {
		r := graph.Range{Start: graph.Position{Line: line}, End: graph.Position{Line: line, Character: 2}}
		return graph.NewNode(graph.Item{Name: name, Kind: 12, URI: "file:///fixture/input.go", Range: r, SelectionRange: r})
	}
	caller, callee := node("caller", 0), node("callee", 2)
	edge := graph.Edge{CallerNodeID: caller.ID, CalleeNodeID: callee.ID, CallSites: []graph.Range{{Start: graph.Position{Line: 0, Character: 1}, End: graph.Position{Line: 0, Character: 2}}}}
	canonical := graph.Result{SchemaVersion: graph.SchemaVersionV5, Nodes: []graph.Node{caller, callee}, Edges: []graph.Edge{edge}}
	canonical.Canonicalize()
	edge = canonical.Edges[0]
	seedA := graph.InvocationSeed{Label: "seed-a", At: "input.go:1:1", ResolvedURI: "file:///fixture/input.go", ContentSHA256: "sha256:" + strings.Repeat("a", 64), LanguageID: "go"}
	seedB := seedA
	seedB.Label, seedB.At = "seed-b", "input.go:2:1"
	result := graph.Result{
		SchemaVersion: graph.SchemaVersionV5,
		Invocation:    graph.Invocation{Server: graph.ServerInvocation{Command: "fixture-ls"}, Seeds: []graph.InvocationSeed{seedA, seedB}, Provenance: graph.InvocationProvenance{InvocationID: "fixture-session", SourceRevision: "fixture-revision", ServerVersion: "fixture-ls@1"}},
		Nodes:         []graph.Node{caller, callee}, Edges: []graph.Edge{edge},
		Seeds: []graph.SeedResult{
			{Label: seedA.Label, PreparedTargetIDs: []string{caller.ID}, ReachedNodeIDs: []string{caller.ID, callee.ID}, ReachedRelationIDs: []string{edge.RelationID}},
			{Label: seedB.Label, PreparedTargetIDs: []string{caller.ID}, ReachedNodeIDs: []string{caller.ID, callee.ID}, ReachedRelationIDs: []string{edge.RelationID}},
		},
		Summary: graph.Summary{Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true},
	}
	native, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	retained, err := graphprovenance.CaptureV5(native, "fixture-session", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}})
	if err != nil {
		t.Fatal(err)
	}
	outcome, failure := programc.Compute(retained, 1)
	if failure != nil || len(outcome.Communities) == 0 {
		t.Fatalf("fixture outcome=%+v failure=%v", outcome, failure)
	}
	_, err = programcrepresentative.NewSingletonMechanics(programcrepresentative.SingletonMechanicsInput{
		WorkflowKey: "workflow", TerminalProgramCHeadSelector: "sha256:" + strings.Repeat("1", 64),
		ReceiptSelector: "sha256:" + strings.Repeat("2", 64), ReceiptOrdinal: 1,
		ConstituentOrdinal: 0, PublicationSelector: "sha256:" + strings.Repeat("3", 64),
		ConstituentSelector: "sha256:" + strings.Repeat("4", 64), Source: outcome.Source,
		ProgramCAlgorithmVersion: "fixture-version",
		RunID:                    "seed-00000000000000000001-replay-1", Seed: 1, ReplayOrdinal: 1,
		CommunityOrdinal: 0, CommunityArtifactSHA256: "sha256:" + strings.Repeat("5", 64),
		CommunityMembers: outcome.Communities[0].Members,
	})
	if err == nil {
		t.Fatal("ASSERT_PROGRAM_C_STAGE6_MEMBERSHIP_MULTIPLICITY_REJECTED")
	}
}
