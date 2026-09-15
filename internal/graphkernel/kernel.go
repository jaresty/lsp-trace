// Package graphkernel provides custody-neutral deterministic graph mechanics.
//
// Callers own admission, authority, scope, identity provenance, resource caps,
// and claim ceilings. This package accepts only opaque canonical node identities
// and directed weighted arcs and does not infer any of those caller concerns.
package graphkernel

import (
	"fmt"
	"math"
	"sort"

	ggraph "gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/iterator"
	"gonum.org/v1/gonum/graph/simple"
)

// Arc is a directed weighted arc between two caller-supplied identities.
type Arc struct {
	From, To string
	Weight   float64
}

// RobertMartinCoupling is the node-local coupling metric described by Robert
// Martin. Ca and Ce count distinct non-self incoming and outgoing neighbors.
type RobertMartinCoupling struct {
	Node        string
	Ca          int
	Ce          int
	Instability float64
}

type pair struct{ from, to int64 }

type weightedEdge struct {
	from, to ggraph.Node
	weight   float64
}

func (e weightedEdge) From() ggraph.Node { return e.from }
func (e weightedEdge) To() ggraph.Node   { return e.to }
func (e weightedEdge) ReversedEdge() ggraph.Edge {
	return weightedEdge{from: e.to, to: e.from, weight: e.weight}
}
func (e weightedEdge) Weight() float64 { return e.weight }

// DirectedWeighted is a deterministic Gonum directed weighted graph. Numeric
// node IDs are assigned by lexical ordering of the opaque node identities.
type DirectedWeighted struct {
	identities []string
	nodes      map[int64]ggraph.Node
	from, to   map[int64][]int64
	weights    map[pair]float64
}

// NewDirectedWeighted constructs a graph from opaque node identities and arcs.
// Input order is not significant. Parallel arc weights are summed.
func NewDirectedWeighted(nodeIdentities []string, arcs []Arc) (*DirectedWeighted, error) {
	identities := append([]string(nil), nodeIdentities...)
	sort.Strings(identities)
	ids := make(map[string]int64, len(identities))
	for i, identity := range identities {
		if identity == "" {
			return nil, fmt.Errorf("empty node identity")
		}
		if i > 0 && identities[i-1] == identity {
			return nil, fmt.Errorf("duplicate node identity %q", identity)
		}
		ids[identity] = int64(i)
	}

	g := &DirectedWeighted{
		identities: identities,
		nodes:      make(map[int64]ggraph.Node, len(identities)),
		from:       make(map[int64][]int64),
		to:         make(map[int64][]int64),
		weights:    make(map[pair]float64),
	}
	for i := range identities {
		g.nodes[int64(i)] = simple.Node(i)
	}
	for _, arc := range arcs {
		from, fromOK := ids[arc.From]
		to, toOK := ids[arc.To]
		if !fromOK || !toOK {
			return nil, fmt.Errorf("arc endpoint missing: %q -> %q", arc.From, arc.To)
		}
		if math.IsNaN(arc.Weight) || math.IsInf(arc.Weight, 0) || arc.Weight <= 0 {
			return nil, fmt.Errorf("arc weight must be finite and positive: %q -> %q", arc.From, arc.To)
		}
		p := pair{from: from, to: to}
		if _, exists := g.weights[p]; !exists {
			g.from[from] = append(g.from[from], to)
			g.to[to] = append(g.to[to], from)
		}
		g.weights[p] += arc.Weight
	}
	for id := range g.nodes {
		sort.Slice(g.from[id], func(i, j int) bool { return g.from[id][i] < g.from[id][j] })
		sort.Slice(g.to[id], func(i, j int) bool { return g.to[id][i] < g.to[id][j] })
	}
	return g, nil
}

// NodeIdentities returns the deterministic numeric-ID-to-identity ordering.
func (g *DirectedWeighted) NodeIdentities() []string {
	return append([]string(nil), g.identities...)
}

// RobertMartinCoupling returns one canonical-order record for every node.
func (g *DirectedWeighted) RobertMartinCoupling() []RobertMartinCoupling {
	out := make([]RobertMartinCoupling, len(g.identities))
	for i, identity := range g.identities {
		id := int64(i)
		ca, ce := nonSelfCount(g.to[id], id), nonSelfCount(g.from[id], id)
		instability := 0.0
		if denominator := ca + ce; denominator != 0 {
			instability = float64(ce) / float64(denominator)
		}
		out[i] = RobertMartinCoupling{Node: identity, Ca: ca, Ce: ce, Instability: instability}
	}
	return out
}

func nonSelfCount(neighbors []int64, self int64) int {
	count := 0
	for _, neighbor := range neighbors {
		if neighbor != self {
			count++
		}
	}
	return count
}

func (g *DirectedWeighted) Node(id int64) ggraph.Node { return g.nodes[id] }
func (g *DirectedWeighted) Nodes() ggraph.Nodes {
	nodes := make([]ggraph.Node, len(g.identities))
	for i := range nodes {
		nodes[i] = g.nodes[int64(i)]
	}
	return iterator.NewOrderedNodes(nodes)
}
func (g *DirectedWeighted) From(id int64) ggraph.Nodes { return g.nodesFor(g.from[id]) }
func (g *DirectedWeighted) To(id int64) ggraph.Nodes   { return g.nodesFor(g.to[id]) }
func (g *DirectedWeighted) nodesFor(ids []int64) ggraph.Nodes {
	if len(ids) == 0 {
		return ggraph.Empty
	}
	nodes := make([]ggraph.Node, len(ids))
	for i, id := range ids {
		nodes[i] = g.nodes[id]
	}
	return iterator.NewOrderedNodes(nodes)
}
func (g *DirectedWeighted) HasEdgeBetween(x, y int64) bool {
	return g.HasEdgeFromTo(x, y) || g.HasEdgeFromTo(y, x)
}
func (g *DirectedWeighted) HasEdgeFromTo(x, y int64) bool {
	_, ok := g.weights[pair{from: x, to: y}]
	return ok
}
func (g *DirectedWeighted) Edge(x, y int64) ggraph.Edge { return g.WeightedEdge(x, y) }
func (g *DirectedWeighted) WeightedEdge(x, y int64) ggraph.WeightedEdge {
	weight, ok := g.weights[pair{from: x, to: y}]
	if !ok {
		return nil
	}
	return weightedEdge{from: g.nodes[x], to: g.nodes[y], weight: weight}
}
func (g *DirectedWeighted) Weight(x, y int64) (float64, bool) {
	weight, ok := g.weights[pair{from: x, to: y}]
	return weight, ok
}

var _ ggraph.WeightedDirected = (*DirectedWeighted)(nil)
