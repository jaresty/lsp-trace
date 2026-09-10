package graph

import (
	"encoding/json"
	"fmt"
	"sort"
)

type semanticLocation struct {
	Name           string
	Kind           int
	URI            string
	SelectionRange Range
}

type exactDeclaration struct {
	semanticLocation
	Range Range
}

// SameSemanticLocation reports whether server presentations identify the same
// symbol location while allowing response-specific detail, range, and data.
func SameSemanticLocation(a, b Node) bool {
	return semanticLocation{a.Name, a.Kind, a.URI, a.SelectionRange} ==
		semanticLocation{b.Name, b.Kind, b.URI, b.SelectionRange}
}

// ReconcileIncomingAliases canonicalizes unambiguous incoming call-hierarchy
// aliases to the outgoing presentation before independently composed results
// are merged. Native graph identity remains unchanged everywhere else.
func ReconcileIncomingAliases(outgoing Result, incoming *Result) {
	if incoming == nil {
		return
	}
	byDeclaration := make(map[exactDeclaration][]Node, len(incoming.Nodes))
	for _, node := range incoming.Nodes {
		key := exactDeclaration{semanticLocation{node.Name, node.Kind, node.URI, node.SelectionRange}, node.Range}
		byDeclaration[key] = append(byDeclaration[key], node)
	}
	replacements := map[string]Node{}
	for _, aliases := range byDeclaration {
		if len(aliases) < 2 {
			continue
		}
		sort.Slice(aliases, func(i, j int) bool { return aliases[i].ID < aliases[j].ID })
		canonical := aliases[0]
		for _, alias := range aliases[1:] {
			if alias.ID == canonical.ID {
				continue
			}
			replacements[alias.ID] = canonical
			presentation, _ := json.Marshal(alias.Item)
			incoming.Diagnostics = append(incoming.Diagnostics, Diagnostic{
				Phase: "slice-reconcile", Method: "callHierarchy/incomingCalls", NodeID: canonical.ID,
				Message: fmt.Sprintf("INCOMING_SYMBOL_ALIAS_RECONCILED: incoming=%s", presentation),
			})
		}
	}

	byLocation := make(map[semanticLocation][]Node, len(outgoing.Nodes))
	for _, node := range outgoing.Nodes {
		key := semanticLocation{node.Name, node.Kind, node.URI, node.SelectionRange}
		byLocation[key] = append(byLocation[key], node)
	}
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
	nodes := make([]Node, 0, len(incoming.Nodes))
	seenNodes := map[string]bool{}
	for _, node := range incoming.Nodes {
		if replacement, ok := replacements[node.ID]; ok {
			node = replacement
		}
		if !seenNodes[node.ID] {
			seenNodes[node.ID] = true
			nodes = append(nodes, node)
		}
	}
	incoming.Nodes = nodes
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
