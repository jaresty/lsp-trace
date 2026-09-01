package graph

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
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
	RelationID   string  `json:"relation_id"`
	CallerNodeID string  `json:"caller_node_id"`
	CalleeNodeID string  `json:"callee_node_id"`
	CallSites    []Range `json:"call_sites"`
}

// ValidateItem validates identity-bearing call hierarchy item fields.
func ValidateItem(item Item) error {
	if item.Name == "" {
		return errors.New("missing item name")
	}
	u, err := url.Parse(item.URI)
	if err != nil || !u.IsAbs() {
		return errors.New("invalid item URI")
	}
	if lessPosition(item.Range.End, item.Range.Start) {
		return errors.New("item range end precedes start")
	}
	if lessPosition(item.SelectionRange.End, item.SelectionRange.Start) ||
		lessPosition(item.SelectionRange.Start, item.Range.Start) ||
		lessPosition(item.Range.End, item.SelectionRange.End) {
		return errors.New("selection range is outside item range")
	}
	return nil
}

// ValidateRange rejects structurally invalid ranges.
func ValidateRange(r Range) error {
	if lessPosition(r.End, r.Start) {
		return errors.New("call-site range end precedes start")
	}
	return nil
}

// RangeContains reports whether inner is enclosed by outer. Some servers return
// identifier-only CallHierarchyItem ranges, so a false result is diagnostic but
// does not invalidate an otherwise well-formed incoming-call relation.
func RangeContains(outer, inner Range) bool {
	return !lessPosition(inner.Start, outer.Start) && !lessPosition(outer.End, inner.End)
}

// SameNodeIdentity reports whether two nodes have identical stable identity fields.
func SameNodeIdentity(a, b Node) bool {
	return a.ID == b.ID &&
		a.Name == b.Name && a.Kind == b.Kind && a.Detail == b.Detail && a.URI == b.URI &&
		a.Range == b.Range && a.SelectionRange == b.SelectionRange
}

func lessPosition(a, b Position) bool {
	return a.Line < b.Line || (a.Line == b.Line && a.Character < b.Character)
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
	edge.RelationID = canonicalRelationID("CALL_RELATION", "CALLER_TO_CALLEE", edge.CallerNodeID+"->"+edge.CalleeNodeID, "", "", "", edge.CallerNodeID, edge.CalleeNodeID)
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
