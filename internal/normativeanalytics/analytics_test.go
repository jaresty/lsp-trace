package normativeanalytics

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"sort"
	"testing"
)

type fixedExecutor int64

func (f fixedExecutor) Execute(Operation) (int64, error) { return int64(f), nil }

func TestFrozenHistoricalContract(t *testing.T) {
	if SchemaVersion != "lsp-trace.normative-analytics.v2" || Family != "normative-analytics" || Version != "v2" || Scope != "NORMATIVE_PROGRAM_B_ONLY" {
		t.Fatal("ASSERT_FROZEN_HISTORICAL_CONSTANTS")
	}
	if _, err := Evaluate(Request{Operation: Analysis, BuildRevision: "r", Limit: 1, Executor: fixedExecutor(1)}); !errors.Is(err, ErrProgramBNotAdmitted) {
		t.Fatalf("ASSERT_FROZEN_ADMISSION_SEMANTICS: %v", err)
	}
	want := []Descriptor{{Analysis, "normative-analysis", "lsp_trace_v2_normative_analysis"}, {Metrics, "normative-metrics", "lsp_trace_v2_normative_metrics"}, {Ranking, "normative-ranking", "lsp_trace_v2_normative_ranking"}}
	if !reflect.DeepEqual(Descriptors(), want) {
		t.Fatal("ASSERT_FROZEN_DESCRIPTORS")
	}
}

func graphBytes(t *testing.T, nodes []string, edges []retainedEdgeJSON) []byte {
	t.Helper()
	sort.Strings(nodes)
	sort.Slice(edges, func(i, j int) bool { return edgeLess(edges[i], edges[j]) })
	b, err := json.Marshal(retainedGraphJSON{RetainedGraphSchema, "rev", nodes, edges})
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func edge(id, from, to, rel, prov, group string) retainedEdgeJSON {
	return retainedEdgeJSON{id, from, to, rel, prov, group}
}
func localRequest(t *testing.T, op Operation, raw []byte, rels ...string) LocalRequest {
	return LocalRequest{op, "rev", raw, rels, LocalPolicy{math.MaxInt64}}
}

func TestStrictDecoderAndExactByteBinding(t *testing.T) {
	good := graphBytes(t, []string{"b", "a"}, []retainedEdgeJSON{edge("e", "a", "b", "CALLS", "p", "")})
	g, digest, err := DecodeRetainedGraph(good)
	if err != nil || g.BuildRevision != "rev" || len(g.Edges) != 1 || digest == "" {
		t.Fatalf("ASSERT_STRICT_DECODER_PASS: %#v %s %v", g, digest, err)
	}
	bad := [][]byte{
		[]byte(`{"schema_version":"lsp-trace.normative-retained-graph.v1","schema_version":"lsp-trace.normative-retained-graph.v1","build_revision":"rev","nodes":[],"edges":[]}`),
		append(append([]byte{}, good...), []byte(` `)...),
		bytes.Replace(good, []byte(`"nodes"`), []byte(`"unknown":0,"nodes"`), 1),
		bytes.Replace(good, []byte(RetainedGraphSchema), []byte("wrong"), 1),
		append([]byte{0xff}, good...),
	}
	for i, b := range bad {
		if _, _, err := DecodeRetainedGraph(b); err == nil {
			t.Fatalf("ASSERT_STRICT_DECODER_REJECT_%d", i)
		}
	}
	if _, err := EvaluateLocal(LocalRequest{Analysis, "other", good, []string{"CALLS"}, LocalPolicy{math.MaxInt64}}); err == nil {
		t.Fatal("ASSERT_REVISION_SUBSTITUTION_REJECTED")
	}
}

func TestMultiplicityFiltersMetricsAndRanking(t *testing.T) {
	raw := graphBytes(t, []string{"a", "b", "c"}, []retainedEdgeJSON{
		edge("e1", "a", "b", "SUPPORTS", "p1", "g1"), edge("e2", "a", "b", "SUPPORTS", "p2", "g1"), edge("e3", "a", "b", "SUPPORTS", "p3", "g2"), edge("e4", "c", "b", "CALLS", "p4", ""),
	})
	m, err := EvaluateLocal(localRequest(t, Metrics, raw, "SUPPORTS", "SUPPORTS"))
	if err != nil || m.Metrics == nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(m.SelectedRelations, []string{"SUPPORTS"}) || m.Metrics.EdgeInstanceCount != 3 || m.Metrics.UniqueStructuralRelationCount != 1 || m.Metrics.IndependentSupportCount != 2 || m.Metrics.Density == nil || *m.Metrics.Density != (LocalRational{1, 6}) {
		t.Fatalf("ASSERT_MULTIPLICITY_COUNTS: %#v", m.Metrics)
	}
	r, err := EvaluateLocal(localRequest(t, Ranking, raw, "SUPPORTS"))
	if err != nil || r.Ranking == nil || r.Ranking.Ordered[0] != (LocalRankedNode{"b", 2, 3, 3}) {
		t.Fatalf("ASSERT_BOUNDED_MULTIPLICITY_RANKING: %#v %v", r.Ranking, err)
	}
	a, err := EvaluateLocal(localRequest(t, Analysis, raw, "CALLS"))
	if err != nil || a.Analysis == nil || a.Accounting.SelectedEdgeUnits != 1 {
		t.Fatalf("ASSERT_RELATION_EXCLUSION: %#v %v", a, err)
	}
	for _, rels := range [][]string{nil, {"UNKNOWN"}} {
		if _, err := EvaluateLocal(localRequest(t, Metrics, raw, rels...)); err == nil {
			t.Fatal("ASSERT_CLOSED_NONEMPTY_RELATIONS")
		}
	}
}

func TestIdentityProvenanceAndEndpoints(t *testing.T) {
	cases := [][]retainedEdgeJSON{
		{edge("e", "a", "b", "CALLS", "p", ""), edge("e", "a", "b", "CALLS", "p", "")},
		{edge("e", "a", "missing", "CALLS", "p", "")},
		{edge("e", "missing", "a", "CALLS", "p", "")},
	}
	for i, edges := range cases {
		raw := graphBytes(t, []string{"a", "b"}, edges)
		if _, _, err := DecodeRetainedGraph(raw); err == nil {
			t.Fatalf("ASSERT_IDENTITY_ENDPOINT_REJECT_%d", i)
		}
	}
	// Same structure with distinct identity/provenance is legitimate multiplicity.
	raw := graphBytes(t, []string{"a", "b"}, []retainedEdgeJSON{edge("e1", "a", "b", "CALLS", "p1", ""), edge("e2", "a", "b", "CALLS", "p2", "")})
	if _, _, err := DecodeRetainedGraph(raw); err != nil {
		t.Fatalf("ASSERT_PARALLEL_INSTANCES_PRESERVED: %v", err)
	}
}

func TestBoundsDigestsValidatorAndPermutation(t *testing.T) {
	edges := []retainedEdgeJSON{edge("z", "a", "b", "CALLS", "p2", ""), edge("a", "b", "a", "SUPPORTS", "p1", "g")}
	raw := graphBytes(t, []string{"b", "a"}, edges)
	base := localRequest(t, Metrics, raw, "SUPPORTS", "CALLS")
	exact := int64(len(raw) + 2 + 2)
	for _, delta := range []int64{-1, 0, 1} {
		q := base
		q.Policy.MaxWork = exact + delta
		got, err := EvaluateLocal(q)
		if err != nil {
			t.Fatal(err)
		}
		if delta < 0 && got.Status != Limit {
			t.Fatal("ASSERT_ONE_BELOW_LIMIT")
		}
		if delta >= 0 && got.Status != Complete {
			t.Fatal("ASSERT_EXACT_ABOVE_COMPLETE")
		}
	}
	for _, limit := range []int64{0, -1} {
		q := base
		q.Policy.MaxWork = limit
		if _, err := EvaluateLocal(q); err == nil {
			t.Fatal("ASSERT_ZERO_NEGATIVE_REJECT")
		}
	}
	first, _ := EvaluateLocal(base)
	second, _ := EvaluateLocal(localRequest(t, Metrics, graphBytes(t, []string{"a", "b"}, []retainedEdgeJSON{edges[1], edges[0]}), "CALLS", "SUPPORTS"))
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("ASSERT_PERMUTATION_INVARIANT: %#v %#v", first, second)
	}
	encoded, err := MarshalLocalResult(first)
	if err != nil || ValidateLocalResultJSON(encoded) != nil {
		t.Fatalf("ASSERT_RESULT_VALIDATOR_PASS: %v", err)
	}
	if ValidateLocalResultJSON(append(encoded, []byte(` `)...)) == nil {
		t.Fatal("ASSERT_RESULT_TRAILING_REJECT")
	}
	mut := append([]byte{}, encoded...)
	mut = bytes.Replace(mut, []byte(`"BuildRevision":"rev"`), []byte(`"BuildRevision":"other"`), 1)
	if ValidateLocalResultJSON(mut) == nil {
		t.Fatal("ASSERT_RESULT_DIGEST_REVISION_MUTATION")
	}
	other, _ := EvaluateLocal(localRequest(t, Analysis, raw, "CALLS", "SUPPORTS"))
	if first.Digest == other.Digest {
		t.Fatal("ASSERT_OPERATION_DOMAIN_DISTINCT")
	}
}

func TestEmptyLoopsCyclesAndAliases(t *testing.T) {
	empty := graphBytes(t, []string{}, []retainedEdgeJSON{})
	for _, op := range []Operation{Analysis, Metrics, Ranking} {
		q := localRequest(t, op, empty, "CALLS")
		got, err := EvaluateLocal(q)
		if err != nil || got.Status != Complete {
			t.Fatalf("ASSERT_EMPTY_%s: %v", op, err)
		}
	}
	raw := graphBytes(t, []string{"a", "b"}, []retainedEdgeJSON{edge("e1", "a", "a", "SUPPORTS", "p1", "same"), edge("e2", "a", "b", "SUPPORTS", "p2", "same"), edge("e3", "b", "a", "SUPPORTS", "p3", "same")})
	got, err := EvaluateLocal(localRequest(t, Metrics, raw, "SUPPORTS"))
	if err != nil || got.Metrics.IndependentSupportCount != 1 {
		t.Fatalf("ASSERT_ALIAS_SUPPORT_NOT_MULTIPLIED: %#v %v", got.Metrics, err)
	}
}
