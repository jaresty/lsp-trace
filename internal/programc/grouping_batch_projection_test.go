package programc

import (
	"reflect"
	"testing"

	rg "lsp-trace/internal/referencegrouping"
)

// This fixture starts at the policy's selected contributions, not acquisition.
func TestMixedGroupingProjectionBatchOrder(t *testing.T) {
	nodes := []string{"a.go#entry", "a.go#helper", "b.go#target", "b.go#unused"}
	weight := rg.Rational{Numerator: 1, Denominator: 1}
	local := rg.PairContribution{OccurrenceIdentity: "local", From: nodes[0], To: nodes[1], Kind: "CALLS", Weight: weight}
	cross := rg.PairContribution{OccurrenceIdentity: "cross", From: nodes[1], To: nodes[2], Kind: "REFERENCES_SYMBOL", Weight: weight}
	resolve := rg.PairContribution{OccurrenceIdentity: "resolve", From: nodes[0], To: nodes[2], Kind: "RESOLVES_TO_DEFINITION", Weight: weight}
	project := func(batches ...[]rg.PairContribution) Projection {
		t.Helper()
		var all []rg.PairContribution
		for _, batch := range batches {
			all = append(all, batch...)
		}
		p, _, err := groupingNumericalProjectionPrivate(nodes, all)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	one := project([]rg.PairContribution{local, cross, resolve})
	two := project([]rg.PairContribution{cross}, []rg.PairContribution{resolve, local})
	sameGraph := func(a, b Projection) bool {
		return reflect.DeepEqual(a.NodeIdentities, b.NodeIdentities) && reflect.DeepEqual(a.PairWeights, b.PairWeights)
	}
	if !sameGraph(one, two) {
		t.Fatal("ASSERT_MIXED_BATCH_GRAPH: regrouping changed effective graph")
	}
	missing := project([]rg.PairContribution{resolve, local})
	if sameGraph(one, missing) {
		t.Fatal("ASSERT_MIXED_BATCH_GRAPH_CONTROL: dropped cross-file reference undetected")
	}
	a, af := partitionMath(one, 9)
	b, bf := partitionMath(two, 9)
	if af != nil || bf != nil {
		t.Fatalf("partition failures: %v, %v", af, bf)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("ASSERT_MIXED_BATCH_PARTITION: partition changed for the same evidence")
	}
}
