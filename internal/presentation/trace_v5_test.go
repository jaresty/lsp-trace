package presentation

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
)

func makeV5TraceFixture(t *testing.T, complete bool, truncated bool, nodes []graph.Node, edges []graph.Edge, targets []string) []byte {
	t.Helper()
	summary := graph.Summary{NodeCount: len(nodes), EdgeCount: len(edges), TerminalCount: 0, CycleCount: 0, Complete: complete, Truncated: truncated}
	inv := graph.Invocation{
		WorkspaceURI: "file:///w",
		Target:       graph.Target{URI: "file:///w/a.go", Line: 0, Column: 0},
		Server:       graph.ServerInvocation{Command: "fake-lsp"},
		Provenance:   graph.InvocationProvenance{InvocationID: "session", SourceRevision: graph.Unknown, ServerVersion: "fake@1"},
	}
	native, err := json.Marshal(graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: inv, Targets: targets, Nodes: nodes, Edges: edges, Summary: summary})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := graphprovenance.CaptureV5(native, "session", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func makeNode(name, uri string, line, char uint32) graph.Node {
	return graph.NewNode(graph.Item{Name: name, Kind: 6, URI: uri, Range: graph.Range{Start: graph.Position{Line: line, Character: char}, End: graph.Position{Line: line, Character: char + 1}}, SelectionRange: graph.Range{Start: graph.Position{Line: line, Character: char}, End: graph.Position{Line: line, Character: char + 1}}})
}

func TestRenderTraceV5SummaryAndDefaultCollapse(t *testing.T) {
	a := makeNode("A", "file:///w/a.go", 0, 0)
	b := makeNode("B", "file:///w/b.go", 10, 2)
	c := makeNode("C", "file:///w/c_test.go", 0, 0)
	d := makeNode("D", "file:///usr/local/go/src/d.go", 1, 0)
	edges := []graph.Edge{
		{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{Start: graph.Position{Line: 3, Character: 1}, End: graph.Position{Line: 3, Character: 2}}}},
		{CallerNodeID: c.ID, CalleeNodeID: a.ID, CallSites: []graph.Range{{Start: graph.Position{Line: 5, Character: 4}, End: graph.Position{Line: 5, Character: 6}}}},
		{CallerNodeID: d.ID, CalleeNodeID: a.ID, CallSites: []graph.Range{{Start: graph.Position{Line: 2, Character: 9}, End: graph.Position{Line: 2, Character: 11}}}},
	}
	raw := makeV5TraceFixture(t, true, false, []graph.Node{a, b, c, d}, edges, []string{a.ID})

	got, err := RenderTraceV5(raw, TraceV5Options{})
	if err != nil {
		t.Fatal(err)
	}
	assertContains := []string{
		"lsp-trace presentation only — derived view; not authoritative evidence",
		"status: COMPLETE",
		"nodes: 4",
		"edges: 3",
		"source_graph_complete: UNKNOWN",
		"completeness_scope: SERVER_REPORTED_CALL_HIERARCHY",
		"direct incoming: 2",
		"omitted collapsed nodes",
		"1 test",
		"1 external/outside-workspace",
		"direct outgoing: 1",
	}
	for _, want := range assertContains {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in output:\n%s", want, got)
		}
	}
}

func TestRenderTraceV5ExpandOptions(t *testing.T) {
	a := makeNode("A", "file:///w/a.go", 0, 0)
	b := makeNode("B", "file:///w/b.go", 10, 2)
	c := makeNode("C", "file:///w/c_test.go", 0, 0)
	d := makeNode("D", "file:///usr/local/go/src/d.go", 1, 0)
	edges := []graph.Edge{
		{CallerNodeID: c.ID, CalleeNodeID: a.ID, CallSites: []graph.Range{{Start: graph.Position{Line: 5, Character: 4}, End: graph.Position{Line: 5, Character: 6}}}},
		{CallerNodeID: d.ID, CalleeNodeID: a.ID, CallSites: []graph.Range{{Start: graph.Position{Line: 2, Character: 9}, End: graph.Position{Line: 2, Character: 11}}}},
		{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{Start: graph.Position{Line: 3, Character: 1}, End: graph.Position{Line: 3, Character: 2}}}},
	}
	raw := makeV5TraceFixture(t, true, false, []graph.Node{a, b, c, d}, edges, []string{a.ID})

	got, err := RenderTraceV5(raw, TraceV5Options{ExpandTestNodes: true, ExpandExternalNodes: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "call-site: 6:5-6:7") {
		t.Fatalf("expected test caller call-site, got: %s", got)
	}
	if strings.Contains(got, "omitted collapsed nodes") {
		t.Fatalf("expected no omitted section when expanded, got: %s", got)
	}
	if !strings.Contains(got, "C") || !strings.Contains(got, "D") {
		t.Fatalf("expected explicit test and external nodes when expanded: %s", got)
	}
}

func TestRenderTraceV5DuplicateNamesAndDisambiguation(t *testing.T) {
	a1 := makeNode("Dup", "file:///w/dup1.go", 0, 0)
	a2 := makeNode("Dup", "file:///w/dup2.go", 4, 0)
	b := makeNode("B", "file:///w/b.go", 2, 0)
	edges := []graph.Edge{
		{CallerNodeID: a1.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{Start: graph.Position{Line: 1, Character: 1}, End: graph.Position{Line: 1, Character: 2}}}},
		{CallerNodeID: a2.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{Start: graph.Position{Line: 1, Character: 1}, End: graph.Position{Line: 1, Character: 2}}}},
	}
	raw := makeV5TraceFixture(t, true, false, []graph.Node{a1, a2, b}, edges, []string{b.ID})

	if _, err := RenderTraceV5(raw, TraceV5Options{Targets: []string{"Dup"}}); err == nil {
		t.Fatal("expected ambiguous target error")
	} else if !strings.Contains(err.Error(), "ambiguous target") {
		t.Fatalf("expected ambiguous target error, got: %v", err)
	}
	got, err := RenderTraceV5(raw, TraceV5Options{Targets: []string{"Dup@file:///w/dup1.go:1:1", "Dup@file:///w/dup2.go:5:1"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Dup (dup1.go:1:1)") || !strings.Contains(got, "Dup (dup2.go:5:1)") {
		t.Fatalf("expected disambiguated duplicate outputs, got: %s", got)
	}
}

func TestRenderTraceV5MalformedInputAndDigestMismatch(t *testing.T) {
	a := makeNode("A", "file:///w/a.go", 0, 0)
	raw := makeV5TraceFixture(t, true, false, []graph.Node{a}, nil, []string{a.ID})

	var e graphprovenance.EvidenceV5
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}

	if _, err := RenderTraceV5(append([]byte("{bad"), raw...), TraceV5Options{}); err == nil {
		t.Fatal("expected malformed provenance JSON error")
	}
	e.GraphV5SHA256 = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	mutated, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RenderTraceV5(mutated, TraceV5Options{}); err == nil {
		t.Fatal("expected embedded graph_v5 digest mismatch")
	}
}

func TestRenderTraceV5ImmutabilityAndDeterminism(t *testing.T) {
	a := makeNode("A", "file:///w/a.go", 0, 0)
	b := makeNode("B", "file:///w/b.go", 1, 0)
	c := makeNode("C", "file:///w/c_test.go", 4, 0)

	edges := []graph.Edge{{CallerNodeID: a.ID, CalleeNodeID: b.ID}, {CallerNodeID: c.ID, CalleeNodeID: b.ID}}
	raw := makeV5TraceFixture(t, false, true, []graph.Node{a, b, c}, edges, []string{a.ID, b.ID})
	orig := append([]byte(nil), raw...)
	got1, err := RenderTraceV5(raw, TraceV5Options{})
	if err != nil {
		t.Fatal(err)
	}
	got2, err := RenderTraceV5(raw, TraceV5Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(orig, raw) {
		t.Fatal("renderer mutated envelope bytes")
	}
	if got1 != got2 {
		t.Fatalf("renderer output not deterministic\nfirst:\n%s\nsecond:\n%s", got1, got2)
	}
}

func TestRenderTraceV5StatusTransitions(t *testing.T) {
	a := makeNode("A", "file:///w/a.go", 0, 0)
	partial := makeV5TraceFixture(t, false, true, []graph.Node{a}, nil, []string{a.ID})
	complete := makeV5TraceFixture(t, true, false, []graph.Node{a}, nil, []string{a.ID})
	emptyRaw := func() []byte {
		inv := graph.Invocation{WorkspaceURI: "file:///w", Server: graph.ServerInvocation{Command: "fake-lsp"}}
		summary := graph.Summary{NodeCount: 0, EdgeCount: 0, TerminalCount: 0, CycleCount: 0, Complete: false, Truncated: false}
		result := graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: inv, Targets: nil, Nodes: nil, Edges: nil, Summary: summary}
		native, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := graphprovenance.CaptureV5(native, "session", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}})
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}()

	for _, c := range []struct {
		name string
		raw  []byte
		want string
	}{
		{name: "partial", raw: partial, want: "status: PARTIAL"},
		{name: "complete", raw: complete, want: "status: COMPLETE"},
		{name: "empty", raw: emptyRaw, want: "status: EMPTY"},
	} {
		got, err := RenderTraceV5(c.raw, TraceV5Options{})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !strings.Contains(got, c.want) {
			t.Fatalf("%s: missing %q in %q", c.name, c.want, got)
		}
	}
}
