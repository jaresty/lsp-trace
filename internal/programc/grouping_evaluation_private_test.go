package programc

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"

	rg "lsp-trace/internal/referencegrouping"
)

func numericalContribution(id, kind, from, to string, n, d int) rg.PairContribution {
	return rg.PairContribution{OccurrenceIdentity: id, Kind: kind, From: from, To: to, Weight: rg.Rational{Numerator: n, Denominator: d}}
}
func TestGroupingEvaluationPrivateNumericalKernelDifferential(t *testing.T) {
	nodes := []string{"a", "b", "c", "d", "inactive"}
	contributions := []rg.PairContribution{
		numericalContribution("r1", "REFERENCES_SYMBOL", "a", "b", 1, 4), numericalContribution("d1", "RESOLVES_TO_DEFINITION", "a", "b", 1, 4),
		numericalContribution("r2", "REFERENCES_SYMBOL", "b", "a", 1, 4),
		numericalContribution("d2", "RESOLVES_TO_DEFINITION", "c", "d", 1, 4), numericalContribution("r3", "REFERENCES_SYMBOL", "d", "c", 1, 4),
	}
	p, pairs, e := groupingNumericalProjectionPrivate(nodes, contributions)
	if e != nil {
		t.Fatal(e)
	}
	if len(pairs) != 4 || p.PairWeights[Pair{0, 1}] != 0.5 || len(p.Occurrences) != 5 {
		t.Fatal("ASSERT_EVALUATION_EXACT_CROSS_KIND_RATIONAL_SUM")
	}
	for j, o := range p.Occurrences {
		if o.Identity != contributions[j].OccurrenceIdentity || o.Provider != "" || o.ProviderVersion != "" || o.Language != "" || o.RelationID != "" || !reflect.DeepEqual(o.CallSite, Occurrence{}.CallSite) || o.Weight != 0.25 {
			t.Fatal("ASSERT_EVALUATION_NO_FABRICATED_CALLS_METADATA")
		}
	}
	// Independent numerical projection, not derived from adapter PairWeights.
	expected := Projection{NodeIdentities: nodes, Occurrences: []Occurrence{{Identity: "r1", From: 0, To: 1, Weight: .25}, {Identity: "d1", From: 0, To: 1, Weight: .25}, {Identity: "r2", From: 1, To: 0, Weight: .25}, {Identity: "d2", From: 2, To: 3, Weight: .25}, {Identity: "r3", From: 3, To: 2, Weight: .25}}, PairWeights: map[Pair]float64{{0, 1}: .5, {1, 0}: .25, {2, 3}: .25, {3, 2}: .25}}
	for _, seed := range []uint64{0, 1, 73, math.MaxUint64} {
		got, f := partitionMath(p, seed)
		if f != nil {
			t.Fatal(f)
		}
		want, f := partitionMath(expected, seed)
		if f != nil {
			t.Fatal(f)
		}
		if !reflect.DeepEqual(got, want) || got.Outcome != "COMPLETE" || len(got.Communities) >= len(nodes) {
			t.Fatal("ASSERT_EVALUATION_USES_REAL_WEIGHTED_KERNEL")
		}
		if !reflect.DeepEqual(got.Communities[len(got.Communities)-1], Community{Members: []string{"inactive"}}) {
			t.Fatal("ASSERT_EVALUATION_INACTIVE_SINGLETON")
		}
	}
	reverse := append([]rg.PairContribution(nil), contributions...)
	for a, b := 0, len(reverse)-1; a < b; a, b = a+1, b-1 {
		reverse[a], reverse[b] = reverse[b], reverse[a]
	}
	permuted, permutedPairs, e := groupingNumericalProjectionPrivate(nodes, reverse)
	if e != nil {
		t.Fatal(e)
	}
	a, _ := partitionMath(p, 73)
	b, _ := partitionMath(permuted, 73)
	if !reflect.DeepEqual(a, b) || !reflect.DeepEqual(pairs, permutedPairs) {
		t.Fatal("ASSERT_EVALUATION_NUMERICAL_PERMUTATION")
	}
	empty, _, e := groupingNumericalProjectionPrivate(nodes, nil)
	if e != nil {
		t.Fatal(e)
	}
	out, f := partitionMath(empty, 73)
	if f != nil || out.Outcome != "EMPTY" || len(out.Communities) != len(nodes) || len(empty.Occurrences) != 0 {
		t.Fatal("ASSERT_EVALUATION_EMPTY_REAL_UNIVERSE")
	}
	t.Log("ASSERT_EVALUATION_USES_REAL_WEIGHTED_KERNEL: PASS")
}
func TestGroupingEvaluationPrivateRationalFloatAndCountBounds(t *testing.T) {
	for _, r := range []rg.Rational{{Numerator: 0, Denominator: 1}, {Numerator: -1, Denominator: 1}, {Numerator: 1, Denominator: 0}, {Numerator: 2, Denominator: 4}, {Numerator: groupingNumericMax + 1, Denominator: 1}, {Numerator: 1, Denominator: groupingDenominatorMax + 1}, {Numerator: math.MinInt, Denominator: 1}} {
		if _, e := groupingFloatPrivate(r); e == nil {
			t.Fatalf("ASSERT_EVALUATION_RATIONAL_REJECTION %+v", r)
		}
	}
	for _, f := range []float64{0, -1, math.Inf(1), math.Inf(-1), math.NaN(), math.SmallestNonzeroFloat64 / 2} {
		if groupingFinitePositivePrivate(f) == nil {
			t.Fatal("ASSERT_EVALUATION_FLOAT_REJECTION")
		}
	}
	if groupingFinitePositivePrivate(math.SmallestNonzeroFloat64) != nil {
		t.Fatal("ASSERT_EVALUATION_POSITIVE_SUBNORMAL_ALLOWED")
	}
	// Bounded integer ratios cannot underflow: minimum admitted ratio is 1/2^30.
	if _, e := groupingFloatPrivate(rg.Rational{Numerator: 1, Denominator: groupingDenominatorMax}); e != nil {
		t.Fatal("ASSERT_EVALUATION_MINIMUM_RATIO", e)
	}
	for _, pair := range [][2]rg.Rational{{{Numerator: groupingNumericMax, Denominator: 1}, {Numerator: 1, Denominator: 1}}, {{Numerator: groupingNumericMax, Denominator: 1073741823}, {Numerator: groupingNumericMax, Denominator: 1073741821}}, {{Numerator: 1, Denominator: 1073741823}, {Numerator: 1, Denominator: 1073741821}}} {
		if _, e := groupingAddRationalPrivate(pair[0], pair[1]); e == nil {
			t.Fatal("ASSERT_EVALUATION_CHECKED_RATIONAL_OVERFLOW")
		}
	}
	r, e := groupingAddRationalPrivate(rg.Rational{Numerator: 1, Denominator: 4}, rg.Rational{Numerator: 1, Denominator: 2})
	if e != nil || r != (rg.Rational{Numerator: 3, Denominator: 4}) {
		t.Fatal("ASSERT_EVALUATION_EXACT_RATIONAL_CONTROL")
	}
	good := numericalContribution("real", "REFERENCES_SYMBOL", "a", "b", 1, 4)
	for _, tc := range []struct {
		nodes   []string
		records []rg.PairContribution
	}{
		{[]string{"b", "a"}, []rg.PairContribution{good}},
		{[]string{"a"}, []rg.PairContribution{good}},
		{[]string{"a", "b"}, []rg.PairContribution{good, good}},
		{make([]string, rg.EvaluationMaxNodesPrivate+1), nil},
		{[]string{"a", "b"}, make([]rg.PairContribution, rg.EvaluationMaxOccurrencesPrivate+1)},
	} {
		if _, _, e := groupingNumericalProjectionPrivate(tc.nodes, tc.records); e == nil {
			t.Fatal("ASSERT_EVALUATION_NUMERIC_BOUNDS_ENDPOINT_DUPLICATE")
		}
	}
	for _, mutate := range []func(*rg.PairContribution){func(c *rg.PairContribution) { c.OccurrenceIdentity = "" }, func(c *rg.PairContribution) { c.Kind = "FAKE" }, func(c *rg.PairContribution) { c.Layer = "layer" }, func(c *rg.PairContribution) { c.Weight = rg.Rational{Numerator: 1, Denominator: 0} }} {
		c := good
		mutate(&c)
		if _, _, e := groupingNumericalProjectionPrivate([]string{"a", "b"}, []rg.PairContribution{c}); e == nil {
			t.Fatal("ASSERT_EVALUATION_INVALID_NUMERIC_RECORD")
		}
	}
	t.Log("ASSERT_EVALUATION_SAFE_NUMERIC_CONVERSION: PASS")
}
func TestGroupingEvaluationPrivateHistoricalCallsMathUnchanged(t *testing.T) {
	nodes := []string{"a", "b", "isolated"}
	occurrences := []Occurrence{{Identity: "historical-1", RelationID: "r1", Provider: "controlled-existing-calls-fixture", From: 0, To: 1, Weight: 1}, {Identity: "historical-2", RelationID: "r2", Provider: "controlled-existing-calls-fixture", From: 1, To: 0, Weight: 1}}
	legacy := CallsMathPrivate{NodeIdentities: nodes, Occurrences: occurrences, PairWeights: map[Pair]float64{{0, 1}: 1, {1, 0}: 1}}
	before, f := PartitionCallsMathPrivate(legacy, 73)
	if f != nil {
		t.Fatal(f)
	}
	beforeBytes, e := json.Marshal(before)
	if e != nil {
		t.Fatal(e)
	}
	detached, _, e := groupingNumericalProjectionPrivate(nodes, []rg.PairContribution{numericalContribution("historical-1", "CALLS", "a", "b", 1, 1), numericalContribution("historical-2", "CALLS", "b", "a", 1, 1)})
	if e != nil {
		t.Fatal(e)
	}
	got, f := partitionMath(detached, 73)
	if f != nil {
		t.Fatal(f)
	}
	after, f := PartitionCallsMathPrivate(legacy, 73)
	if f != nil {
		t.Fatal(f)
	}
	afterBytes, e := json.Marshal(after)
	if e != nil {
		t.Fatal(e)
	}
	if string(beforeBytes) != string(afterBytes) || !reflect.DeepEqual(before, got) || ProfileID != "calls-v1" || ProfileDigest != "sha256:f4fb309c6e849b5a8e6057f355b1db6ee3c53414c3c430b67be17f68fe9e97f9" {
		t.Fatal("ASSERT_EVALUATION_HISTORICAL_CALLS_BYTES_DEFAULTS")
	}
	t.Log("ASSERT_EVALUATION_HISTORICAL_CALLS_BYTES_DEFAULTS: PASS")
}
