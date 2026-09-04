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
