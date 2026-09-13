package transientstructural

import "sort"

func analyze(request AnalysisRequest, projection admittedProjection, bounds BoundsBinding) AnalysisResult {
	if request.Kind == AnalysisNeighborhood {
		return AnalysisResult{
			Kind: AnalysisNeighborhood, MaxDepth: max(bounds.UpDepth, bounds.DownDepth),
			Nodes: cloneNodeFacts(projection.nodes), Occurrences: cloneOccurrenceFacts(projection.occurrences),
		}
	}
	result := AnalysisResult{Kind: AnalysisImpact, Direction: request.Direction, MaxDepth: request.MaxDepth}
	for _, node := range projection.nodes {
		if node.ID == projection.targetID {
			continue
		}
		if witnesses := matchingWitnesses(node.Witnesses, request.Direction, request.MaxDepth); len(witnesses) != 0 {
			result.Nodes = append(result.Nodes, NodeFact{ID: node.ID, Witnesses: witnesses})
		}
	}
	for _, occurrence := range projection.occurrences {
		if witnesses := matchingWitnesses(occurrence.Witnesses, request.Direction, request.MaxDepth); len(witnesses) != 0 {
			result.Occurrences = append(result.Occurrences, OccurrenceFact{ID: occurrence.ID, CallerID: occurrence.CallerID, CalleeID: occurrence.CalleeID, Witnesses: witnesses})
		}
	}
	sort.Slice(result.Nodes, func(i, j int) bool { return result.Nodes[i].ID < result.Nodes[j].ID })
	sort.Slice(result.Occurrences, func(i, j int) bool { return result.Occurrences[i].ID < result.Occurrences[j].ID })
	return result
}

func matchingWitnesses(witnesses []Witness, direction Direction, maxDepth int) []Witness {
	var out []Witness
	for _, witness := range witnesses {
		if witness.Direction == direction && witness.Depth <= maxDepth {
			out = append(out, witness)
		}
	}
	return out
}

func cloneNodeFacts(in []NodeFact) []NodeFact {
	out := make([]NodeFact, len(in))
	for i := range in {
		out[i] = NodeFact{ID: in[i].ID, Witnesses: append([]Witness(nil), in[i].Witnesses...)}
	}
	return out
}

func cloneOccurrenceFacts(in []OccurrenceFact) []OccurrenceFact {
	out := make([]OccurrenceFact, len(in))
	for i := range in {
		out[i] = OccurrenceFact{ID: in[i].ID, CallerID: in[i].CallerID, CalleeID: in[i].CalleeID, Witnesses: append([]Witness(nil), in[i].Witnesses...)}
	}
	return out
}
