package graph

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

type Position struct {
	Line      uint32 `json:"line"`
	Character uint32 `json:"character"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Item struct {
	Name           string          `json:"name"`
	Kind           int             `json:"kind"`
	Detail         string          `json:"detail,omitempty"`
	URI            string          `json:"uri"`
	Range          Range           `json:"range"`
	SelectionRange Range           `json:"selection_range"`
	Data           json.RawMessage `json:"data,omitempty"`
}

type Node struct {
	ID string `json:"id"`
	Item
}

type Edge struct {
	CallerNodeID string  `json:"caller_node_id"`
	CalleeNodeID string  `json:"callee_node_id"`
	CallSites    []Range `json:"call_sites"`
}

func NewNode(item Item) Node {
	identity := struct {
		Name           string `json:"name"`
		Kind           int    `json:"kind"`
		Detail         string `json:"detail"`
		URI            string `json:"uri"`
		Range          Range  `json:"range"`
		SelectionRange Range  `json:"selection_range"`
	}{item.Name, item.Kind, item.Detail, item.URI, item.Range, item.SelectionRange}
	encoded, _ := json.Marshal(identity)
	sum := sha256.Sum256(encoded)
	return Node{ID: hex.EncodeToString(sum[:]), Item: item}
}

func MergeEdge(edges []Edge, edge Edge) []Edge {
	for i := range edges {
		if edges[i].CallerNodeID == edge.CallerNodeID && edges[i].CalleeNodeID == edge.CalleeNodeID {
			edges[i].CallSites = mergeRanges(edges[i].CallSites, edge.CallSites)
			return edges
		}
	}
	edge.CallSites = mergeRanges(nil, edge.CallSites)
	return append(edges, edge)
}

func mergeRanges(a, b []Range) []Range {
	seen := make(map[Range]struct{}, len(a)+len(b))
	for _, r := range append(append([]Range(nil), a...), b...) {
		seen[r] = struct{}{}
	}
	out := make([]Range, 0, len(seen))
	for r := range seen {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return lessRange(out[i], out[j]) })
	return out
}

func lessRange(a, b Range) bool {
	if a.Start.Line != b.Start.Line {
		return a.Start.Line < b.Start.Line
	}
	if a.Start.Character != b.Start.Character {
		return a.Start.Character < b.Start.Character
	}
	if a.End.Line != b.End.Line {
		return a.End.Line < b.End.Line
	}
	return a.End.Character < b.End.Character
}
