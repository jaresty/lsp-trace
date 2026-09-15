package graphkernel

import (
	"reflect"
	"testing"
)

func TestRobertMartinCouplingIsolate(t *testing.T) {
	g := mustGraph(t, []string{"isolate"}, nil)
	assertCoupling(t, g.RobertMartinCoupling(), []RobertMartinCoupling{{Node: "isolate", Ca: 0, Ce: 0, Instability: 0}})
}

func TestRobertMartinCouplingIncomingOnly(t *testing.T) {
	g := mustGraph(t, []string{"a", "b"}, []Arc{{From: "a", To: "b", Weight: 1}})
	assertCoupling(t, g.RobertMartinCoupling(), []RobertMartinCoupling{
		{Node: "a", Ca: 0, Ce: 1, Instability: 1},
		{Node: "b", Ca: 1, Ce: 0, Instability: 0},
	})
}

func TestRobertMartinCouplingOutgoingOnly(t *testing.T) {
	g := mustGraph(t, []string{"a", "b"}, []Arc{{From: "b", To: "a", Weight: 1}})
	assertCoupling(t, g.RobertMartinCoupling(), []RobertMartinCoupling{
		{Node: "a", Ca: 1, Ce: 0, Instability: 0},
		{Node: "b", Ca: 0, Ce: 1, Instability: 1},
	})
}

func TestRobertMartinCouplingBalanced(t *testing.T) {
	g := mustGraph(t, []string{"a", "b", "c"}, []Arc{
		{From: "a", To: "b", Weight: 1},
		{From: "b", To: "c", Weight: 1},
	})
	assertCoupling(t, g.RobertMartinCoupling(), []RobertMartinCoupling{
		{Node: "a", Ca: 0, Ce: 1, Instability: 1},
		{Node: "b", Ca: 1, Ce: 1, Instability: 0.5},
		{Node: "c", Ca: 1, Ce: 0, Instability: 0},
	})
}

func TestRobertMartinCouplingDeduplicatesParallelArcs(t *testing.T) {
	g := mustGraph(t, []string{"a", "b"}, []Arc{
		{From: "a", To: "b", Weight: 1},
		{From: "a", To: "b", Weight: 2},
	})
	assertCoupling(t, g.RobertMartinCoupling(), []RobertMartinCoupling{
		{Node: "a", Ca: 0, Ce: 1, Instability: 1},
		{Node: "b", Ca: 1, Ce: 0, Instability: 0},
	})
	if weight, ok := g.Weight(0, 1); !ok || weight != 3 {
		t.Fatalf("ASSERT_PARALLEL_ARC_WEIGHT_PRESERVED weight=%v ok=%t", weight, ok)
	}
}

func TestRobertMartinCouplingIgnoresSelfLoop(t *testing.T) {
	g := mustGraph(t, []string{"a"}, []Arc{{From: "a", To: "a", Weight: 4}})
	assertCoupling(t, g.RobertMartinCoupling(), []RobertMartinCoupling{{Node: "a", Ca: 0, Ce: 0, Instability: 0}})
	if weight, ok := g.Weight(0, 0); !ok || weight != 4 {
		t.Fatalf("ASSERT_SELF_LOOP_WEIGHT_PRESERVED weight=%v ok=%t", weight, ok)
	}
}

func TestKernelPermutationInvariantCanonicalOrdering(t *testing.T) {
	arcs := []Arc{{From: "z", To: "a", Weight: 2}, {From: "a", To: "m", Weight: 1}}
	first := mustGraph(t, []string{"z", "a", "m"}, arcs)
	second := mustGraph(t, []string{"m", "z", "a"}, []Arc{arcs[1], arcs[0]})
	if !reflect.DeepEqual(first.NodeIdentities(), []string{"a", "m", "z"}) || !reflect.DeepEqual(first.NodeIdentities(), second.NodeIdentities()) {
		t.Fatalf("ASSERT_CANONICAL_NODE_ORDER first=%v second=%v", first.NodeIdentities(), second.NodeIdentities())
	}
	assertCoupling(t, first.RobertMartinCoupling(), second.RobertMartinCoupling())
}

func mustGraph(t *testing.T, nodes []string, arcs []Arc) *DirectedWeighted {
	t.Helper()
	g, err := NewDirectedWeighted(nodes, arcs)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func assertCoupling(t *testing.T, got, want []RobertMartinCoupling) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ASSERT_ROBERT_MARTIN_COUPLING got=%+v want=%+v", got, want)
	}
}

func TestKernelRejectsParallelWeightOverflow(t *testing.T) {
	_, err := NewDirectedWeighted([]string{"a", "b"}, []Arc{
		{From: "a", To: "b", Weight: 1.7976931348623157e308},
		{From: "a", To: "b", Weight: 1.7976931348623157e308},
	})
	if err == nil {
		t.Fatal("ASSERT_AGGREGATE_ARC_WEIGHT_FINITE: overflow accepted")
	}
}
