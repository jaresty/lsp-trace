package presentation

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
)

func makeV5TraceFixture(t *testing.T, complete bool, truncated bool, nodes []graph.Node, edges []graph.Edge, targets []string) []byte {
	t.Helper()
	return makeV5TraceResult(t, graph.Result{
		SchemaVersion: graph.SchemaVersionV5,
		Invocation: graph.Invocation{
			WorkspaceURI: "file:///w",
			Target:       graph.Target{URI: "file:///w/a.go", Line: 0, Column: 0},
			Server:       graph.ServerInvocation{Command: "fake-lsp"},
			Provenance:   graph.InvocationProvenance{InvocationID: "session", SourceRevision: graph.Unknown, ServerVersion: "fake@1"},
		},
		Targets: targets,
		Nodes:   nodes,
		Edges:   edges,
		Summary: graph.Summary{NodeCount: len(nodes), EdgeCount: len(edges), Complete: complete, Truncated: truncated},
	})
}

func makeV5TraceResult(t *testing.T, result graph.Result) []byte {
	t.Helper()
	native, err := json.Marshal(result)
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
		"collapsed peer nodes: 2",
		"collapsed direct relations: 2",
		"test peer nodes: 1",
		"external/outside-workspace peer nodes: 1",
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
	if !strings.Contains(got, "call-site: c_test.go:6:5-6:7") {
		t.Fatalf("expected test caller call-site, got: %s", got)
	}
	if strings.Contains(got, "collapsed peer nodes") {
		t.Fatalf("expected no omitted section when expanded, got: %s", got)
	}
	if !strings.Contains(got, "C") || !strings.Contains(got, "D") {
		t.Fatalf("expected explicit test and external nodes when expanded: %s", got)
	}
}

func TestRenderTraceV5AuthoritativeTargetsOnly(t *testing.T) {
	a := makeNode("A", "file:///w/a.go", 0, 0)
	b := makeNode("B", "file:///w/b.go", 2, 0)
	other := makeNode("Other", "file:///w/other.go", 4, 0)
	raw := makeV5TraceFixture(t, true, false, []graph.Node{a, b, other}, nil, []string{b.ID, a.ID})
	for _, requested := range [][]string{{"A"}, {other.ID}, {" "}} {
		if _, err := RenderTraceV5(raw, TraceV5Options{Targets: requested}); err == nil {
			t.Fatalf("expected target rejection for %#v", requested)
		}
	}
	got, err := RenderTraceV5(raw, TraceV5Options{Targets: []string{b.ID, a.ID, b.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(got, "\nTARGET ") != 2 || strings.Index(got, "TARGET A") > strings.Index(got, "TARGET B") {
		t.Fatalf("target order/dedup:\n%s", got)
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
		result := graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: inv, Targets: nil, Nodes: nil, Edges: nil, Diagnostics: []graph.Diagnostic{{Phase: "initialize", Message: "no graph"}}, Summary: summary}
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
		{name: "incomplete-empty", raw: emptyRaw, want: "status: PARTIAL"},
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

func TestRenderTraceV5CallSitesIncludeSortedCallerLocation(t *testing.T) {
	a := makeNode("A", "file:///w/a.go", 0, 0)
	b := makeNode("B", "file:///w/sub/b.go", 2, 0)
	edges := []graph.Edge{
		{CallerNodeID: b.ID, CalleeNodeID: a.ID, CallSites: []graph.Range{{Start: graph.Position{Line: 9, Character: 2}, End: graph.Position{Line: 9, Character: 5}}, {Start: graph.Position{Line: 3, Character: 1}, End: graph.Position{Line: 3, Character: 4}}}},
		{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{Start: graph.Position{Line: 7}, End: graph.Position{Line: 7, Character: 2}}}},
	}
	got, err := RenderTraceV5(makeV5TraceFixture(t, true, false, []graph.Node{a, b}, edges, []string{a.ID, b.ID}), TraceV5Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"call-site: sub/b.go:4:2-4:5", "call-site: sub/b.go:10:3-10:6", "call-site: a.go:8:1-8:3"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}
	if strings.Index(got, "sub/b.go:4:2-4:5") > strings.Index(got, "sub/b.go:10:3-10:6") {
		t.Fatalf("call sites not sorted:\n%s", got)
	}
}

func TestRenderTraceV5CollapsedAccountingAndClassification(t *testing.T) {
	a := makeNode("A", "file:///w/a.go", 0, 0)
	testNode := makeNode("T", "file:///w/tests/t.spec.ts", 0, 0)
	edges := []graph.Edge{{CallerNodeID: testNode.ID, CalleeNodeID: a.ID}, {CallerNodeID: testNode.ID, CalleeNodeID: a.ID, CallSites: []graph.Range{{Start: graph.Position{Line: 2}, End: graph.Position{Line: 2, Character: 1}}}}}
	got, err := RenderTraceV5(makeV5TraceFixture(t, true, false, []graph.Node{a, testNode}, edges, []string{a.ID}), TraceV5Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"collapsed peer nodes: 1", "collapsed direct relations: 2", "test peer nodes: 1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}
	cases := []struct {
		uri, want, reject string
	}{
		{"file:///w/test/a.ts", "test peer nodes: 1", "external/outside-workspace peer nodes"},
		{"file:///w/tests/a.cs", "test peer nodes: 1", "external/outside-workspace peer nodes"},
		{"file:///w/a_test.go", "test peer nodes: 1", "external/outside-workspace peer nodes"},
		{"file:///w/a.test.js", "test peer nodes: 1", "external/outside-workspace peer nodes"},
		{"file:///w/a.spec.ts", "test peer nodes: 1", "external/outside-workspace peer nodes"},
		{"file:///w/contest/a.go", "N@contest/a.go", "collapsed peer nodes"},
		{"file:///else/tests/a_test.go", "external/outside-workspace peer nodes: 1", "test peer nodes"},
		{"https://example.test/a.spec.js", "external/outside-workspace peer nodes: 1", "test peer nodes"},
	}
	for _, tc := range cases {
		peer := makeNode("N", tc.uri, 0, 0)
		out, renderErr := RenderTraceV5(makeV5TraceFixture(t, true, false, []graph.Node{a, peer}, []graph.Edge{{CallerNodeID: peer.ID, CalleeNodeID: a.ID}}, []string{a.ID}), TraceV5Options{})
		if renderErr != nil {
			t.Fatal(renderErr)
		}
		if !strings.Contains(out, tc.want) || strings.Contains(out, tc.reject) {
			t.Errorf("%s: want %q reject %q:\n%s", tc.uri, tc.want, tc.reject, out)
		}
	}
}

func TestRenderTraceV5BoundariesDiagnosticsAndLimitations(t *testing.T) {
	a := makeNode("A", "file:///w/a.go", 0, 0)
	r := graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{WorkspaceURI: "file:///w", Target: graph.Target{URI: a.URI}, Server: graph.ServerInvocation{Command: "fake-lsp"}, Provenance: graph.InvocationProvenance{InvocationID: "session", SourceRevision: graph.Unknown, ServerVersion: "fake@1"}}, Targets: []string{a.ID}, Nodes: []graph.Node{a}, Terminals: []graph.Boundary{{NodeID: a.ID, Reason: graph.RequestTimeout, Message: "terminal timeout"}}, Frontier: []graph.Boundary{{NodeID: a.ID, Reason: graph.MaxDepth, Message: "depth 2"}}, Diagnostics: []graph.Diagnostic{{Phase: "incoming", Method: "callHierarchy/incomingCalls", NodeID: a.ID, Category: graph.UnresolvedCall, Message: "dynamic dispatch"}}, Summary: graph.Summary{Complete: false}}
	got, err := RenderTraceV5(makeV5TraceResult(t, r), TraceV5Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"status: PARTIAL", "terminals: 1", "REQUEST_TIMEOUT", "terminal timeout", "frontier: 1", "MAX_DEPTH", "depth 2", "graph diagnostics: 1", "UNRESOLVED_CALL", "dynamic dispatch", "dependency_completeness:", "analyzed_version:", "source_policy:", "source_graph_complete: UNKNOWN", "COMPLETE means bounded traversal complete only; it never means workspace or source complete"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}
}

func resealNativeMutation(t *testing.T, raw []byte, mutate func(map[string]any)) []byte {
	t.Helper()
	var env graphprovenance.EvidenceV5
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	native, err := base64.StdEncoding.DecodeString(env.GraphV5)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(native, &document); err != nil {
		t.Fatal(err)
	}
	mutate(document)
	native, err = json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(native)
	env.GraphV5 = base64.StdEncoding.EncodeToString(native)
	env.GraphV5SHA256 = fmt.Sprintf("sha256:%x", sum)
	out, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestRenderTraceV5CyclesAndCanonicalAdmission(t *testing.T) {
	a := makeNode("A", "file:///w/a.go", 0, 0)
	b := makeNode("B", "file:///w/b.go", 0, 0)
	got, err := RenderTraceV5(makeV5TraceFixture(t, true, false, []graph.Node{a, b}, []graph.Edge{{CallerNodeID: a.ID, CalleeNodeID: b.ID}, {CallerNodeID: b.ID, CalleeNodeID: a.ID}, {CallerNodeID: a.ID, CalleeNodeID: a.ID}}, []string{a.ID}), TraceV5Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "cycles: 1") {
		t.Fatalf("missing cycle count:\n%s", got)
	}
	valid := makeV5TraceFixture(t, true, false, []graph.Node{a}, nil, []string{a.ID})
	mutations := map[string]func([]byte) []byte{
		"trailing": func(b []byte) []byte { return append(b, []byte(" true")...) },
		"unknown": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`{"schema_version"`), []byte(`{"unknown":true,"schema_version"`), 1)
		},
		"duplicate": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`{"schema_version"`), []byte(`{"schema_version":"lsp-trace.graph-provenance.v5","schema_version"`), 1)
		},
		"wrong-schema": func(b []byte) []byte {
			return bytes.Replace(b, []byte(graphprovenance.VersionV5), []byte("lsp-trace.graph-provenance.v4"), 1)
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			if _, err := RenderTraceV5(mutate(append([]byte(nil), valid...)), TraceV5Options{}); err == nil {
				t.Fatal("expected canonical rejection")
			}
		})
	}
	semanticMutations := map[string]func(map[string]any){
		"duplicate-node-id": func(d map[string]any) { nodes := d["nodes"].([]any); d["nodes"] = append(nodes, nodes[0]) },
		"dangling-edge": func(d map[string]any) {
			d["edges"] = []any{map[string]any{"caller_node_id": a.ID, "callee_node_id": "missing", "call_sites": []any{}, "relation_id": "sha256:" + strings.Repeat("0", 64), "execution_bundle_id": d["execution_bundle_id"]}}
		},
		"malformed-semantic-receipt": func(d map[string]any) {
			d["trace_receipt"].(map[string]any)["semantic_commitment_digest"] = "sha256:" + strings.Repeat("0", 64)
		},
	}
	for name, mutate := range semanticMutations {
		t.Run(name, func(t *testing.T) {
			if _, err := RenderTraceV5(resealNativeMutation(t, valid, mutate), TraceV5Options{}); err == nil {
				t.Fatal("expected canonical semantic rejection")
			}
		})
	}
}
