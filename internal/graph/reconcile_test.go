package graph

import (
	"encoding/json"
	"strings"
	"testing"
)

func reconciliationNode(name string, line, rangeEnd uint32, data string) Node {
	sel := Range{Start: Position{Line: line}, End: Position{Line: line, Character: 1}}
	return NewNode(Item{Name: name, Kind: 12, URI: "file:///w/a.go", Range: Range{Start: Position{Line: line}, End: Position{Line: rangeEnd, Character: 1}}, SelectionRange: sel, Data: json.RawMessage(data)})
}

func TestReconcileIncomingAliasesUsesSemanticLocationAndPreservesPresentation(t *testing.T) {
	const assertion = "ASSERT_RECONCILE_SEMANTIC_LOCATION_NOT_OPAQUE_DATA"
	out := reconciliationNode("root", 0, 2, `{"surface":"outgoing"}`)
	in := reconciliationNode("root", 0, 0, `{"surface":"incoming"}`)
	incoming := Result{Nodes: []Node{in}, Edges: []Edge{{CallerNodeID: in.ID, CalleeNodeID: "leaf"}}}
	ReconcileIncomingAliases(Result{Nodes: []Node{out}}, &incoming)
	if len(incoming.Nodes) != 1 || incoming.Nodes[0].ID != out.ID || len(incoming.Diagnostics) != 1 || !strings.Contains(incoming.Diagnostics[0].Message, `"surface":"incoming"`) {
		t.Fatalf("%s: result=%+v", assertion, incoming)
	}
}

func TestReconcileIncomingAliasesFallsBackOnAmbiguity(t *testing.T) {
	const assertion = "ASSERT_RECONCILE_AMBIGUITY_FALLBACK"
	first := reconciliationNode("root", 0, 1, `1`)
	second := reconciliationNode("root", 0, 2, `2`)
	in := reconciliationNode("root", 0, 3, `3`)
	incoming := Result{Nodes: []Node{in}}
	ReconcileIncomingAliases(Result{Nodes: []Node{first, second}}, &incoming)
	if incoming.Nodes[0].ID != in.ID || len(incoming.Diagnostics) != 0 {
		t.Fatalf("%s: result=%+v", assertion, incoming)
	}
}

func TestReconcileIncomingAliasesCollapsesExactDeclarationsAndPreservesEdges(t *testing.T) {
	const assertion = "ASSERT_RECONCILE_EXACT_INCOMING_DECLARATION_ALIASES"
	t.Log("ASSERTION: " + assertion)
	r := Range{Start: Position{Line: 10, Character: 2}, End: Position{Line: 20, Character: 1}}
	s := Range{Start: Position{Line: 10, Character: 7}, End: Position{Line: 10, Character: 14}}
	first := NewNode(Item{Name: "Execute", Kind: 12, Detail: "encoding/json", URI: "file:///w/executor.go", Range: r, SelectionRange: s, Data: json.RawMessage(`{"surface":"first"}`)})
	second := NewNode(Item{Name: "Execute", Kind: 12, Detail: "internal/graph", URI: "file:///w/executor.go", Range: r, SelectionRange: s, Data: json.RawMessage(`{"surface":"second"}`)})
	left, right := reconciliationNode("Marshal", 30, 31, `1`), reconciliationNode("Canonicalize", 40, 41, `2`)
	incoming := Result{Nodes: []Node{first, second, left, right}, Edges: []Edge{
		{CallerNodeID: first.ID, CalleeNodeID: left.ID, CallSites: []Range{{Start: Position{Line: 12}, End: Position{Line: 12, Character: 1}}}},
		{CallerNodeID: second.ID, CalleeNodeID: right.ID, CallSites: []Range{{Start: Position{Line: 15}, End: Position{Line: 15, Character: 1}}}},
	}}
	ReconcileIncomingAliases(Result{}, &incoming)
	declarations := 0
	canonicalID := ""
	for _, node := range incoming.Nodes {
		if node.Name == "Execute" {
			declarations++
			canonicalID = node.ID
		}
	}
	if declarations != 1 || len(incoming.Edges) != 2 {
		t.Fatalf("%s: declarations=%d edges=%+v nodes=%+v", assertion, declarations, incoming.Edges, incoming.Nodes)
	}
	for _, edge := range incoming.Edges {
		if edge.CallerNodeID != canonicalID || len(edge.CallSites) != 1 {
			t.Fatalf("%s: canonical=%s edge=%+v", assertion, canonicalID, edge)
		}
	}
	if len(incoming.Diagnostics) != 1 || !strings.Contains(incoming.Diagnostics[0].Message, "INCOMING_SYMBOL_ALIAS_RECONCILED") {
		t.Fatalf("%s: diagnostics=%+v", assertion, incoming.Diagnostics)
	}
}

func TestReconcileIncomingAliasesKeepsDistinctSelectionRanges(t *testing.T) {
	const assertion = "ASSERT_RECONCILE_DISTINCT_SELECTION_RANGES"
	out := reconciliationNode("root", 0, 2, `1`)
	in := reconciliationNode("root", 1, 2, `2`)
	incoming := Result{Nodes: []Node{in}}
	ReconcileIncomingAliases(Result{Nodes: []Node{out}}, &incoming)
	if incoming.Nodes[0].ID != in.ID {
		t.Fatalf("%s: result=%+v", assertion, incoming)
	}
}
