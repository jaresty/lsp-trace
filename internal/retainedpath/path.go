// Package retainedpath implements bounded directed paths over already-admitted
// retained edges. Identifiers and witnesses are opaque; this package performs no
// acquisition, artifact admission, hashing, or source authentication.
package retainedpath

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

// Edge retains a unit-group directed edge and opaque provenance. Weight is
// carried, not interpreted: shortest paths minimize group hops, not weight.
type Edge struct {
	GroupID           string   `json:"group_id"`
	ContextID         string   `json:"context_id"`
	ExecutionBundleID string   `json:"execution_bundle_id"`
	Caller            string   `json:"caller"`
	Callee            string   `json:"callee"`
	Weight            int      `json:"weight"`
	CallsiteState     string   `json:"callsite_state"`
	OccurrenceIDs     []string `json:"occurrence_ids"`
}
type Path struct {
	Nodes         []string   `json:"nodes"`
	GroupIDs      []string   `json:"group_ids"`
	OccurrenceIDs [][]string `json:"occurrence_ids"`
}
type Result struct {
	Status, Reason string
	Path           Path
}

// Budget is caller-owned and may be shared across sequential queries. Index
// construction and independent proof do not consume path-search work, matching
// the historical policy. Cancellation is checked before the remaining work.
type Budget struct {
	Context context.Context
	Left    int
	Reason  string
}

func (b *Budget) Tick() bool {
	if b.Context.Err() != nil {
		b.Reason = "CANCELLED"
		return false
	}
	if b.Left <= 0 {
		b.Reason = "LIMIT"
		return false
	}
	b.Left--
	return true
}

type Adjacency map[string][]Edge

// Indexes constructs new adjacency slices without reordering the input edges.
func Indexes(edges []Edge) (Adjacency, Adjacency) {
	out, in := Adjacency{}, Adjacency{}
	for _, edge := range edges {
		out[edge.Caller] = append(out[edge.Caller], edge)
		in[edge.Callee] = append(in[edge.Callee], edge)
	}
	for _, es := range out {
		sort.Slice(es, func(i, j int) bool {
			if es[i].Callee != es[j].Callee {
				return es[i].Callee < es[j].Callee
			}
			return es[i].GroupID < es[j].GroupID
		})
	}
	for _, es := range in {
		sort.Slice(es, func(i, j int) bool {
			if es[i].Caller != es[j].Caller {
				return es[i].Caller < es[j].Caller
			}
			return es[i].GroupID < es[j].GroupID
		})
	}
	return out, in
}
func emptyPath() Path { return Path{[]string{}, []string{}, [][]string{}} }

// Search consumes a caller-owned shared work budget. Missing endpoints are
// admission errors, including equal missing IDs. The caller admits graph
// consistency and resource ceilings before calling. Edges preserve caller to
// callee orientation. A zero-hop path consumes one node tick. Exhaustion or
// cancellation clears the witness rather than publishing a partial path.
func Search(nodes []string, edges []Edge, start, end string, b *Budget) (Result, error) {
	for _, id := range []string{start, end} {
		found := false
		for _, n := range nodes {
			if n == id {
				found = true
				break
			}
		}
		if !found {
			return Result{}, fmt.Errorf("missing exact node ID %q", id)
		}
	}
	e := Result{Status: "COMPLETE", Path: emptyPath()}
	out, _ := Indexes(edges)
	if b.Context.Err() != nil {
		b.Reason = "CANCELLED"
	}
	if b.Reason != "" {
		e.Status = "INCOMPLETE"
		e.Reason = b.Reason
		return e, nil
	}
	shortest(&e, out, start, end, b)
	if b.Reason != "" {
		e.Status = "INCOMPLETE"
		e.Reason = b.Reason
		e.Path = emptyPath()
	}
	return e, nil
}
func shortest(e *Result, out Adjacency, start, end string, b *Budget) {
	queue := []string{start}
	seen := map[string]bool{start: true}
	prev := map[string]Edge{}
	found := false
	for head := 0; head < len(queue); head++ {
		if !b.Tick() {
			return
		}
		v := queue[head]
		if v == end {
			found = true
			break
		}
		for _, edge := range out[v] {
			if !b.Tick() {
				return
			}
			if !seen[edge.Callee] {
				seen[edge.Callee] = true
				prev[edge.Callee] = edge
				queue = append(queue, edge.Callee)
			}
		}
	}
	if !found {
		e.Status = "NOT_FOUND_IN_RETAINED_GRAPH"
		return
	}
	e.Status = "FOUND"
	nodes := []string{end}
	edges := []Edge{}
	for v := end; v != start; {
		edge := prev[v]
		edges = append(edges, edge)
		v = edge.Caller
		nodes = append(nodes, v)
	}
	for i := len(nodes) - 1; i >= 0; i-- {
		e.Path.Nodes = append(e.Path.Nodes, nodes[i])
	}
	for i := len(edges) - 1; i >= 0; i-- {
		e.Path.GroupIDs = append(e.Path.GroupIDs, edges[i].GroupID)
		e.Path.OccurrenceIDs = append(e.Path.OccurrenceIDs, edges[i].OccurrenceIDs)
	}
}

// Prove independently checks shortestness using reverse distances, not the
// producer's predecessor BFS. Only completed queries are proof candidates;
// callers separately validate endpoint admission, empty non-found payloads,
// resource accounting, and deterministic replay. Nil and empty witness slices
// remain distinct, matching the historical JSON comparison.
func Prove(edges []Edge, start, end, status string, path Path) error {
	out, in := Indexes(edges)
	dist := map[string]int{end: 0}
	q := []string{end}
	for h := 0; h < len(q); h++ {
		for _, edge := range in[q[h]] {
			if _, ok := dist[edge.Caller]; !ok {
				dist[edge.Caller] = dist[q[h]] + 1
				q = append(q, edge.Caller)
			}
		}
	}
	n, ok := dist[start]
	if !ok {
		if status != "NOT_FOUND_IN_RETAINED_GRAPH" {
			return errors.New("false path reachability")
		}
		return nil
	}
	if status != "FOUND" || len(path.Nodes) != n+1 || len(path.GroupIDs) != n || len(path.OccurrenceIDs) != n {
		return errors.New("path not shortest")
	}
	v := start
	for i := 0; i < n; i++ {
		if path.Nodes[i] != v {
			return errors.New("path endpoint mismatch")
		}
		matched := false
		for _, edge := range out[v] {
			if d, ok := dist[edge.Callee]; ok && d == n-i-1 {
				got, _ := json.Marshal(path.OccurrenceIDs[i])
				want, _ := json.Marshal(edge.OccurrenceIDs)
				if path.GroupIDs[i] != edge.GroupID || !bytes.Equal(got, want) {
					return errors.New("path lexical tie/witness mismatch")
				}
				v = edge.Callee
				matched = true
				break
			}
		}
		if !matched {
			return errors.New("path continuity mismatch")
		}
	}
	if path.Nodes[n] != end || v != end {
		return errors.New("path end mismatch")
	}
	return nil
}
