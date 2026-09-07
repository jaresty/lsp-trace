package acquisition

import (
	"encoding/json"
	"errors"
	"reflect"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/lsp"
)

func (c *runner) observe(requestID string, e graph.Edge) {
	for i := range c.result.EdgeObservations {
		o := &c.result.EdgeObservations[i]
		if o.RequestID == requestID && o.RelationID == e.RelationID {
			old := e
			old.CallSites = o.CallSites
			o.CallSites = graph.MergeEdge([]graph.Edge{old}, e)[0].CallSites
			return
		}
	}
	c.result.EdgeObservations = append(c.result.EdgeObservations, EdgeObservation{RequestID: requestID, RelationID: e.RelationID, CallSites: e.CallSites})
}

// observedEdges recomputes only what a retained successful response actually
// supports. It does not infer memberships or a second independent observation.
func observedEdges(rec RequestRecord) ([]graph.Edge, error) {
	if rec.Outcome != "SUCCESS" {
		return nil, errors.New("not a successful response")
	}
	var params struct {
		Item lsp.CallHierarchyItem `json:"item"`
	}
	if err := json.Unmarshal(rec.Params, &params); err != nil {
		return nil, err
	}
	if node(params.Item).ID != rec.NodeID {
		return nil, errors.New("queried item identity mismatch")
	}
	var out []graph.Edge
	add := func(i lsp.CallHierarchyItem, sites []lsp.Range) {
		if graph.ValidateItem(node(i).Item) != nil || !canonicalURI(i.URI) || i.Kind < 1 || i.Kind > 26 {
			return
		}
		edge := graph.Edge{CallerNodeID: rec.NodeID, CalleeNodeID: node(i).ID, CallSites: ranges(sites)}
		if rec.Method == "callHierarchy/incomingCalls" {
			edge.CallerNodeID, edge.CalleeNodeID = edge.CalleeNodeID, edge.CallerNodeID
		}
		for _, r := range edge.CallSites {
			if graph.ValidateRange(r) != nil {
				return
			}
		}
		out = graph.MergeEdge(out, edge)
	}
	switch rec.Method {
	case "callHierarchy/incomingCalls":
		var calls []lsp.CallHierarchyIncomingCall
		if err := json.Unmarshal(rec.Response, &calls); err != nil {
			return nil, err
		}
		for _, call := range calls {
			add(call.From, call.FromRanges)
		}
	case "callHierarchy/outgoingCalls":
		var calls []lsp.CallHierarchyOutgoingCall
		if err := json.Unmarshal(rec.Response, &calls); err != nil {
			return nil, err
		}
		for _, call := range calls {
			add(call.To, call.FromRanges)
		}
	default:
		return nil, errors.New("not a native neighbor method")
	}
	return out, nil
}
func validateObservedJoins(r Result, records map[string]RequestRecord) error {
	supported := map[string]bool{}
	seen := map[string]bool{}
	for _, o := range r.EdgeObservations {
		key := o.RequestID + "/" + o.RelationID
		if seen[key] {
			return errors.New("duplicate response observation")
		}
		seen[key] = true
		rec, ok := records[o.RequestID]
		if !ok {
			return errors.New("missing observation request")
		}
		edges, err := observedEdges(rec)
		if err != nil {
			return err
		}
		matched := false
		for _, edge := range edges {
			if edge.RelationID == o.RelationID && reflect.DeepEqual(edge.CallSites, o.CallSites) {
				matched = true
			}
		}
		if !matched {
			return errors.New("observation not supported by exact response")
		}
		supported[o.RelationID] = true
	}
	for _, edge := range r.Graph.Edges {
		if !supported[edge.RelationID] {
			return errors.New("retained edge lacks acquisition support")
		}
	}
	return nil
}
