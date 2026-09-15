package graphkernel

import (
	"math"
	"sort"

	"gonum.org/v1/gonum/graph/network"
	"gonum.org/v1/gonum/graph/topo"
)

const (
	PageRankDamping      = 0.85
	AnalyticsTolerance   = 1e-12
	WeakProjectionPolicy = "DIRECTED_ARCS_COLLAPSED_TO_SIMPLE_UNDIRECTED_PAIRS; SELF_LOOPS_IGNORED; PARALLEL_AND_ANTIPARALLEL_ARCS_COLLAPSED"
)

type StrongComponent struct {
	Nodes  []string
	Cyclic bool
}
type WeakBridge struct{ A, B string }
type NodeScore struct {
	Node  string
	Score float64
}
type HubAuthorityScore struct {
	Node           string
	Hub, Authority float64
}

func (g *DirectedWeighted) StrongComponents() []StrongComponent {
	parts := topo.TarjanSCC(g)
	out := make([]StrongComponent, 0, len(parts))
	for _, part := range parts {
		nodes := make([]string, len(part))
		cyclic := len(part) > 1
		for i, n := range part {
			nodes[i] = g.identities[n.ID()]
		}
		sort.Strings(nodes)
		if len(part) == 1 && g.HasEdgeFromTo(part[0].ID(), part[0].ID()) {
			cyclic = true
		}
		out = append(out, StrongComponent{Nodes: nodes, Cyclic: cyclic})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Nodes[0] < out[j].Nodes[0] })
	return out
}

// WeakCritical projects every non-self directed arc to one simple undirected
// pair. Direction, weight, parallel arcs, and antiparallel arcs do not affect
// bridge or articulation membership; isolates remain vertices.
func (g *DirectedWeighted) WeakCritical() ([]WeakBridge, []string) {
	n := len(g.identities)
	adj := make([][]int, n)
	for p := range g.weights {
		a, b := int(p.from), int(p.to)
		if a == b {
			continue
		}
		if a > b {
			a, b = b, a
		}
		if !containsInt(adj[a], b) {
			adj[a] = append(adj[a], b)
			adj[b] = append(adj[b], a)
		}
	}
	for i := range adj {
		sort.Ints(adj[i])
	}
	disc, low, parent := make([]int, n), make([]int, n), make([]int, n)
	for i := range parent {
		parent[i] = -1
	}
	time := 0
	articulation := make([]bool, n)
	bridges := []WeakBridge{}
	var visit func(int)
	visit = func(u int) {
		time++
		disc[u] = time
		low[u] = time
		children := 0
		for _, v := range adj[u] {
			if disc[v] == 0 {
				parent[v] = u
				children++
				visit(v)
				if low[v] < low[u] {
					low[u] = low[v]
				}
				if parent[u] == -1 && children > 1 {
					articulation[u] = true
				}
				if parent[u] != -1 && low[v] >= disc[u] {
					articulation[u] = true
				}
				if low[v] > disc[u] {
					a, b := g.identities[u], g.identities[v]
					if a > b {
						a, b = b, a
					}
					bridges = append(bridges, WeakBridge{A: a, B: b})
				}
			} else if v != parent[u] && disc[v] < low[u] {
				low[u] = disc[v]
			}
		}
	}
	for i := 0; i < n; i++ {
		if disc[i] == 0 {
			visit(i)
		}
	}
	sort.Slice(bridges, func(i, j int) bool {
		if bridges[i].A != bridges[j].A {
			return bridges[i].A < bridges[j].A
		}
		return bridges[i].B < bridges[j].B
	})
	points := []string{}
	for i, yes := range articulation {
		if yes {
			points = append(points, g.identities[i])
		}
	}
	return bridges, points
}
func containsInt(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func (g *DirectedWeighted) PageRank() []NodeScore {
	values := network.PageRank(g, PageRankDamping, AnalyticsTolerance)
	out := make([]NodeScore, len(g.identities))
	for i, node := range g.identities {
		out[i] = NodeScore{Node: node, Score: finite(values[int64(i)])}
	}
	return out
}
func (g *DirectedWeighted) HITS() []HubAuthorityScore {
	if len(g.weights) == 0 {
		out := make([]HubAuthorityScore, len(g.identities))
		for i, node := range g.identities {
			out[i].Node = node
		}
		return out
	}
	values := network.HITS(g, AnalyticsTolerance)
	out := make([]HubAuthorityScore, len(g.identities))
	for i, node := range g.identities {
		v := values[int64(i)]
		out[i] = HubAuthorityScore{Node: node, Hub: finite(v.Hub), Authority: finite(v.Authority)}
	}
	return out
}
func finite(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	if v == 0 {
		return 0
	}
	return v
}
