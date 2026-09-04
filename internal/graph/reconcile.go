package graph

import (
	"encoding/json"
	"fmt"
)

type semanticLocation struct {
	Name           string
	Kind           int
	URI            string
	SelectionRange Range
}

// ReconcileIncomingAliases canonicalizes unambiguous incoming call-hierarchy
// aliases to the outgoing presentation before independently composed results
// are merged. Native graph identity remains unchanged everywhere else.
func ReconcileIncomingAliases(outgoing Result, incoming *Result) {
	if incoming == nil {
		return
	}
	byLocation := make(map[semanticLocation][]Node, len(outgoing.Nodes))
	for _, node := range outgoing.Nodes {
		key := semanticLocation{node.Name, node.Kind, node.URI, node.SelectionRange}
		byLocation[key] = append(byLocation[key], node)
	}
	replacements := map[string]Node{}
	for _, node := range incoming.Nodes {
		key := semanticLocation{node.Name, node.Kind, node.URI, node.SelectionRange}
		matches := byLocation[key]
		if len(matches) != 1 || matches[0].ID == node.ID {
			continue
		}
		canonical := matches[0]
		replacements[node.ID] = canonical
		presentation, _ := json.Marshal(node.Item)
		incoming.Diagnostics = append(incoming.Diagnostics, Diagnostic{
			Phase: "slice-reconcile", Method: "callHierarchy/incomingCalls", NodeID: canonical.ID,
			Message: fmt.Sprintf("INCOMING_SYMBOL_ALIAS_RECONCILED: incoming=%s", presentation),
		})
	}
	if len(replacements) == 0 {
		return
	}
	rewrite := func(id string) string {
		if node, ok := replacements[id]; ok {
			return node.ID
		}
		return id
	}
	for i := range incoming.Nodes {
		if node, ok := replacements[incoming.Nodes[i].ID]; ok {
			incoming.Nodes[i] = node
		}
	}
	for i := range incoming.Edges {
		incoming.Edges[i].CallerNodeID = rewrite(incoming.Edges[i].CallerNodeID)
		incoming.Edges[i].CalleeNodeID = rewrite(incoming.Edges[i].CalleeNodeID)
	}
	for i := range incoming.Targets {
		incoming.Targets[i] = rewrite(incoming.Targets[i])
	}
	for i := range incoming.Terminals {
		incoming.Terminals[i].NodeID = rewrite(incoming.Terminals[i].NodeID)
	}
	for i := range incoming.Frontier {
		incoming.Frontier[i].NodeID = rewrite(incoming.Frontier[i].NodeID)
	}
	for i := range incoming.Diagnostics {
		incoming.Diagnostics[i].NodeID = rewrite(incoming.Diagnostics[i].NodeID)
	}
	for i := range incoming.Seeds {
		for j := range incoming.Seeds[i].PreparedTargetIDs {
			incoming.Seeds[i].PreparedTargetIDs[j] = rewrite(incoming.Seeds[i].PreparedTargetIDs[j])
		}
		for j := range incoming.Seeds[i].ReachedNodeIDs {
			incoming.Seeds[i].ReachedNodeIDs[j] = rewrite(incoming.Seeds[i].ReachedNodeIDs[j])
		}
		for j := range incoming.Seeds[i].ReachedEdges {
			incoming.Seeds[i].ReachedEdges[j].CallerNodeID = rewrite(incoming.Seeds[i].ReachedEdges[j].CallerNodeID)
			incoming.Seeds[i].ReachedEdges[j].CalleeNodeID = rewrite(incoming.Seeds[i].ReachedEdges[j].CalleeNodeID)
		}
	}
	incoming.Canonicalize()
}
