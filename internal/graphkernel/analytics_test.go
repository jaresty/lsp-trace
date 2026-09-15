package graphkernel

import (
	"math"
	"reflect"
	"testing"
)

func analyticsFixture(t *testing.T) *DirectedWeighted {
	t.Helper()
	return mustGraph(t, []string{"isolate", "d", "c", "b", "a"}, []Arc{
		{From: "a", To: "b", Weight: 1}, {From: "b", To: "a", Weight: 1},
		{From: "b", To: "c", Weight: 1}, {From: "b", To: "c", Weight: 2},
		{From: "c", To: "d", Weight: 1}, {From: "d", To: "d", Weight: 1},
	})
}

func TestStrongComponentsCanonicalAndExplicitCycles(t *testing.T) {
	got := analyticsFixture(t).StrongComponents()
	want := []StrongComponent{{Nodes: []string{"a", "b"}, Cyclic: true}, {Nodes: []string{"c"}, Cyclic: false}, {Nodes: []string{"d"}, Cyclic: true}, {Nodes: []string{"isolate"}, Cyclic: false}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ASSERT_SCC_CANONICAL_EXPLICIT_CYCLE got=%v want=%v", got, want)
	}
}

func TestWeakCriticalProjectionCollapsesParallelAndIgnoresLoops(t *testing.T) {
	bridges, points := analyticsFixture(t).WeakCritical()
	if !reflect.DeepEqual(bridges, []WeakBridge{{A: "a", B: "b"}, {A: "b", B: "c"}, {A: "c", B: "d"}}) {
		t.Fatalf("ASSERT_WEAK_BRIDGES_CANONICAL got=%v", bridges)
	}
	if !reflect.DeepEqual(points, []string{"b", "c"}) {
		t.Fatalf("ASSERT_WEAK_ARTICULATION_CANONICAL got=%v", points)
	}
	if WeakProjectionPolicy != "DIRECTED_ARCS_COLLAPSED_TO_SIMPLE_UNDIRECTED_PAIRS; SELF_LOOPS_IGNORED; PARALLEL_AND_ANTIPARALLEL_ARCS_COLLAPSED" {
		t.Fatal("ASSERT_WEAK_PROJECTION_POLICY_FROZEN")
	}
}

func TestRankingsCanonicalFiniteAndFrozen(t *testing.T) {
	g := analyticsFixture(t)
	if PageRankDamping != 0.85 || AnalyticsTolerance != 1e-12 {
		t.Fatal("ASSERT_RANKING_PARAMETERS_FROZEN")
	}
	pr, hits := g.PageRank(), g.HITS()
	if len(pr) != 5 || len(hits) != 5 {
		t.Fatalf("ASSERT_RANKINGS_INCLUDE_ISOLATES pr=%v hits=%v", pr, hits)
	}
	for i := range pr {
		if pr[i].Node != []string{"a", "b", "c", "d", "isolate"}[i] || math.IsNaN(pr[i].Score) || math.IsInf(pr[i].Score, 0) {
			t.Fatalf("ASSERT_PAGERANK_CANONICAL_FINITE %+v", pr)
		}
		if hits[i].Node != pr[i].Node || math.IsNaN(hits[i].Hub) || math.IsInf(hits[i].Hub, 0) || math.IsNaN(hits[i].Authority) || math.IsInf(hits[i].Authority, 0) {
			t.Fatalf("ASSERT_HITS_CANONICAL_FINITE %+v", hits)
		}
	}
}

func TestWeakBridgeEndpointsCanonicalIndependentOfDFSParent(t *testing.T) {
	g := mustGraph(t, []string{"a", "b", "c"}, []Arc{{From: "a", To: "c", Weight: 1}, {From: "c", To: "b", Weight: 1}})
	bridges, _ := g.WeakCritical()
	want := []WeakBridge{{A: "a", B: "c"}, {A: "b", B: "c"}}
	if !reflect.DeepEqual(bridges, want) {
		t.Fatalf("ASSERT_WEAK_BRIDGE_ENDPOINTS_LEXICAL got=%v want=%v", bridges, want)
	}
}
