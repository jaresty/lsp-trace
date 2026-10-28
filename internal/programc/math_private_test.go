package programc

import (
	"math"
	"reflect"
	"testing"

	"lsp-trace/internal/graph"
)

func TestPrivateMathMatchesOrdinaryAdmittedFixture(t *testing.T) {
	a, b := node("a", 0), node("b", 1)
	raw := validV5(t, []graph.Node{a, b}, []graph.Edge{{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{Start: graph.Position{Line: 4}, End: graph.Position{Line: 4, Character: 1}}, {Start: graph.Position{Line: 5}, End: graph.Position{Line: 5, Character: 1}}}}})
	_, native := decodeV5(t, raw)
	ordinary, f := Project(raw)
	if f != nil {
		t.Fatal(f)
	}
	projected, f := ProjectNativeCallsMathPrivate(native)
	if f != nil {
		t.Fatal(f)
	}
	if !reflect.DeepEqual(projected.NodeIdentities, ordinary.NodeIdentities) || !reflect.DeepEqual(projected.Occurrences, ordinary.Occurrences) || !reflect.DeepEqual(projected.PairWeights, ordinary.PairWeights) {
		t.Fatal("ASSERT_SHARED_CALLS_PROJECTION")
	}
	actual, f := PartitionCallsMathPrivate(projected, 42)
	if f != nil {
		t.Fatal(f)
	}
	expected, f := Compute(raw, 42)
	if f != nil {
		t.Fatal(f)
	}
	if !reflect.DeepEqual(actual.Communities, expected.Communities) || actual.LogicalDigest != expected.LogicalDigest || actual.Algorithm != expected.Algorithm || actual.Seed != expected.Seed || actual.Outcome != expected.Outcome || actual.Resolution != expected.Resolution {
		t.Fatal("ASSERT_SHARED_PARTITION_MATH")
	}
	t.Log("ASSERT_SHARED_CALLS_PROJECTION: PASS; ASSERT_SHARED_PARTITION_MATH: PASS")
}
func TestPrivateMathEmptyAndResourceDistinction(t *testing.T) {
	out, f := PartitionCallsMathPrivate(CallsMathPrivate{NodeIdentities: []string{"a", "b"}}, 42)
	if f != nil || out.Outcome != "EMPTY" || len(out.Communities) != 2 {
		t.Fatal("ASSERT_EMPTY_EXPLICIT", f)
	}
	nodes := make([]string, MaxNodes+1)
	if out, f := PartitionCallsMathPrivate(CallsMathPrivate{NodeIdentities: nodes}, 42); f == nil || f.Code != CodeResourceLimit || out.Outcome == "EMPTY" {
		t.Fatal("ASSERT_RESOURCE_NOT_EMPTY", f)
	}
	if _, f := PartitionCallsMathPrivate(CallsMathPrivate{Occurrences: make([]Occurrence, MaxOccurrences+1)}, 42); f == nil || f.Code != CodeResourceLimit {
		t.Fatal("ASSERT_OCCURRENCE_CAP", f)
	}
	if _, f := ProjectNativeCallsMathPrivate(make([]byte, (8<<20)+1)); f == nil || f.Code != CodeResourceLimit {
		t.Fatal("ASSERT_NATIVE_BYTE_CAP", f)
	}
	t.Log("ASSERT_EMPTY_EXPLICIT: PASS; ASSERT_RESOURCE_NOT_EMPTY: PASS")
}
func TestPrivateMathRejectsInvalidNumericalProjection(t *testing.T) {
	good := CallsMathPrivate{NodeIdentities: []string{"a", "b"}, Occurrences: []Occurrence{{Identity: "o", RelationID: "r", Provider: "fixture", From: 0, To: 1, Weight: 1}}, PairWeights: map[Pair]float64{{From: 0, To: 1}: 1}}
	if _, f := PartitionCallsMathPrivate(good, 42); f != nil {
		t.Fatal(f)
	}
	cases := []struct {
		name   string
		mutate func(*CallsMathPrivate)
	}{
		{"index", func(p *CallsMathPrivate) { p.Occurrences[0].To = 2 }},
		{"negative", func(p *CallsMathPrivate) { p.Occurrences[0].From = -1 }},
		{"weight", func(p *CallsMathPrivate) { p.Occurrences[0].Weight = 2 }},
		{"NaN", func(p *CallsMathPrivate) { p.PairWeights[Pair{0, 1}] = math.NaN() }},
		{"infinite", func(p *CallsMathPrivate) { p.PairWeights[Pair{0, 1}] = math.Inf(1) }},
		{"direction", func(p *CallsMathPrivate) { delete(p.PairWeights, Pair{0, 1}); p.PairWeights[Pair{1, 0}] = 1 }},
		{"duplicate-occurrence", func(p *CallsMathPrivate) {
			p.Occurrences = append(p.Occurrences, p.Occurrences[0])
			p.PairWeights[Pair{0, 1}] = 2
		}},
		{"order", func(p *CallsMathPrivate) { p.NodeIdentities = []string{"b", "a"} }},
		{"duplicate-node", func(p *CallsMathPrivate) { p.NodeIdentities = []string{"a", "a"} }},
		{"provider", func(p *CallsMathPrivate) { p.Occurrences[0].Provider = "" }},
		{"site", func(p *CallsMathPrivate) { p.Occurrences[0].CallSite.Start.Line = 1 }},
		{"extra-pair", func(p *CallsMathPrivate) { p.PairWeights[Pair{0, 0}] = 1 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := CloneCallsMathPrivate(good)
			tc.mutate(&p)
			if _, f := PartitionCallsMathPrivate(p, 42); f == nil || f.Code != CodeInvalidCalls {
				t.Fatal("ASSERT_INVALID_NUMERIC_CALLS", tc.name, f)
			}
		})
	}
	t.Log("ASSERT_INVALID_NUMERIC_CALLS: PASS")
}
