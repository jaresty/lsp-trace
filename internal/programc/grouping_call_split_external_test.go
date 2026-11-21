package programc_test

import (
	"reflect"
	"testing"

	pc "lsp-trace/internal/programc"
	"lsp-trace/internal/programcadmission"
)

func TestMixedGroupingPolicyCallSplit(t *testing.T) {
	kinds := []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}
	makeCase := func(split bool) pc.GroupingEvaluationCasePrivate {
		t.Helper()
		input, _ := bundleFixtureInputWithCallSplit(t, kinds, split, true, true)
		c, err := pc.NewGroupingEvaluationCasePrivate(pc.GroupingEvaluationCaseRequestPrivate{CaseID: "split-control", RunID: "run-7", SelectorID: "declared-not-executed", SelectorBytes: []byte("controlled selection\n"), Seed: 7, Input: input, Policy: evaluationPolicy(t, "N"), ProjectionLimits: evaluationLimits()})
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	type pair struct {
		kind, from, to         string
		numerator, denominator int64
	}
	summarize := func(c pc.GroupingEvaluationCasePrivate) map[pair]int {
		out := map[pair]int{}
		for _, p := range c.Projection().PairContributions() {
			out[pair{p.Kind, p.From, p.To, int64(p.Weight.Numerator), int64(p.Weight.Denominator)}]++
		}
		return out
	}
	a, b := makeCase(false), makeCase(true)
	want, got := summarize(a), summarize(b)
	for _, k := range kinds {
		found := false
		for p := range want {
			found = found || p.kind == string(k)
		}
		if !found {
			t.Fatalf("missing projected family %s", k)
		}
	}
	if !reflect.DeepEqual(a.Projection().NodeIdentities(), b.Projection().NodeIdentities()) || !reflect.DeepEqual(want, got) {
		t.Fatalf("ASSERT_POLICY_CALL_SPLIT: projected evidence changed: %v vs %v", want, got)
	}
	left, err := pc.EvaluateGroupingCasePrivate(a)
	if err != nil {
		t.Fatal(err)
	}
	right, err := pc.EvaluateGroupingCasePrivate(b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(left.Partition(), right.Partition()) {
		t.Fatal("ASSERT_POLICY_CALL_SPLIT_PARTITION: evaluated partition changed")
	}
}
