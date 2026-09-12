package programctestfixture

import (
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
)

const OpaqueMarker = "OPAQUE_ITEM_DATA_MARKER"
const SourceBodyMarker = "RETAINED_SOURCE_BODY_MARKER"

// ValidV5 returns bounded, exact Graph Provenance V5 envelope bytes with
// qualified server-reported calls. It is test support shared across packages.
func ValidV5(t testing.TB) []byte {
	return validV5(t, false)
}

// DifferentCallsV5 preserves ValidV5's node identities while changing an admitted CALLS edge.
func DifferentCallsV5(t testing.TB) []byte {
	return validV5(t, true)
}

func validV5(t testing.TB, differentCalls bool) []byte {
	t.Helper()
	node := func(name string, line uint32, data json.RawMessage) graph.Node {
		r := graph.Range{Start: graph.Position{Line: line}, End: graph.Position{Line: line, Character: 2}}
		return graph.NewNode(graph.Item{Name: name, Kind: 12, Detail: "Fixture", URI: "file:///fixture/input.go", Range: r, SelectionRange: r, Data: data})
	}
	a := node("alpha", 0, json.RawMessage(`{"opaque":"`+OpaqueMarker+`","source_body":"`+SourceBodyMarker+`"}`))
	b := node("beta", 2, nil)
	c := node("gamma", 4, nil)
	siteAB := graph.Range{Start: graph.Position{Line: 0, Character: 1}, End: graph.Position{Line: 0, Character: 2}}
	siteBC := graph.Range{Start: graph.Position{Line: 2, Character: 1}, End: graph.Position{Line: 2, Character: 2}}
	seed := graph.InvocationSeed{Label: "seed", At: "input.go:1:1", ResolvedURI: "file:///fixture/input.go", ContentSHA256: "sha256:" + strings.Repeat("a", 64), LanguageID: "go"}
	edges := []graph.Edge{{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{siteAB}}, {CallerNodeID: b.ID, CalleeNodeID: c.ID, CallSites: []graph.Range{siteBC}}}
	if differentCalls {
		edges[1].CallerNodeID = a.ID
	}
	r := graph.Result{
		SchemaVersion: graph.SchemaVersionV5,
		Invocation:    graph.Invocation{Server: graph.ServerInvocation{Command: "fixture-ls"}, Seeds: []graph.InvocationSeed{seed}, Provenance: graph.InvocationProvenance{InvocationID: "fixture-session", SourceRevision: "fixture-revision", ServerVersion: "fixture-ls@1"}, Expansion: graph.ExpansionConfig{TopmostSiblings: true}},
		Nodes:         []graph.Node{c, a, b},
		Edges:         edges,
		Seeds:         []graph.SeedResult{{Label: "seed"}}, Summary: graph.Summary{Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true},
	}
	native, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := graphprovenance.CaptureV5(native, "fixture-session", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}})
	if err != nil {
		t.Fatalf("build exact V5 fixture: %v", err)
	}
	return raw
}
