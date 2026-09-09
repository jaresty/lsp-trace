package normativeanalytics

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
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
	for _, d := range LocalDescriptors() {
		if d.Protocol != "" {
			t.Fatal("ASSERT_LOCAL_DESCRIPTORS_UNREGISTERED")
		}
	}
}

func graphBytes(t *testing.T, nodes []string, edges []retainedEdgeJSON) []byte {
	t.Helper()
	nodes = append([]string{}, nodes...)
	edges = append([]retainedEdgeJSON{}, edges...)
	sort.Strings(nodes)
	sort.Slice(edges, func(i, j int) bool { return edgeLess(edges[i], edges[j]) })
	b, err := json.Marshal(retainedGraphJSON{RetainedGraphSchema, "rev", nodes, edges})
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func edge(id, from, to, rel, prov, group string) retainedEdgeJSON {
	return retainedEdgeJSON{id, from, to, rel, ValidatedAuthority, CustodyProviderVerified, prov, group}
}
func unqualifiedEdge(id, from, to, rel, authority, custody, prov, group string) retainedEdgeJSON {
	return retainedEdgeJSON{id, from, to, rel, authority, custody, prov, group}
}
func localRequest(op Operation, raw []byte, rels ...string) LocalRequest {
	return LocalRequest{op, "rev", raw, rels, LocalPolicy{math.MaxInt64}}
}
func localEval(t *testing.T, op Operation, raw []byte, rels ...string) LocalResult {
	t.Helper()
	r, err := EvaluateLocal(localRequest(op, raw, rels...))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestStrictDecoderCanonicalRelationsAndIdentity(t *testing.T) {
	good := graphBytes(t, []string{"b", "a"}, []retainedEdgeJSON{edge("e", "a", "b", "CALLS", "p", "g")})
	g, digest, err := DecodeRetainedGraph(good)
	if err != nil || g.BuildRevision != "rev" || len(g.Edges) != 1 || !validSHA256(digest) {
		t.Fatalf("ASSERT_STRICT_DECODER_PASS: %#v %s %v", g, digest, err)
	}
	bad := [][]byte{
		[]byte(`{"schema_version":"lsp-trace.normative-retained-graph.v1","schema_version":"lsp-trace.normative-retained-graph.v1","build_revision":"rev","nodes":[],"edges":[]}`),
		append(append([]byte{}, good...), ' '), bytes.Replace(good, []byte(`"nodes"`), []byte(`"unknown":0,"nodes"`), 1), bytes.Replace(good, []byte(RetainedGraphSchema), []byte("wrong"), 1), append([]byte{0xff}, good...),
	}
	for i, b := range bad {
		if _, _, err := DecodeRetainedGraph(b); err == nil {
			t.Fatalf("ASSERT_STRICT_DECODER_REJECT_%d", i)
		}
	}
	var wire retainedGraphJSON
	json.Unmarshal(good, &wire)
	wire.Nodes = []string{"b", "a"}
	unordered, _ := json.Marshal(wire)
	if _, _, err := DecodeRetainedGraph(unordered); err == nil {
		t.Fatal("ASSERT_RAW_NODE_ORDER_REJECT")
	}
	wire.Nodes = []string{"a", "b"}
	wire.Edges = []retainedEdgeJSON{edge("z", "a", "b", "CALLS", "p2", ""), edge("a", "a", "b", "CALLS", "p1", "")}
	unordered, _ = json.Marshal(wire)
	if _, _, err := DecodeRetainedGraph(unordered); err == nil {
		t.Fatal("ASSERT_RAW_EDGE_ORDER_REJECT")
	}
	if _, err := EvaluateLocal(LocalRequest{Analysis, "other", good, []string{"CALLS"}, LocalPolicy{math.MaxInt64}}); err == nil {
		t.Fatal("ASSERT_REVISION_SUBSTITUTION_REJECTED")
	}
	for _, rel := range []string{"CALLS", "BINDS_ARGUMENT", "PASSES_CALLBACK", "INVOKES_TASK", "TRIGGERS_RELOAD", "UPDATES_STATE", "RENDERS_FROM"} {
		raw := graphBytes(t, []string{"a", "b"}, []retainedEdgeJSON{edge("e", "a", "b", rel, "p", "")})
		if localEval(t, Analysis, raw, rel).Analysis == nil {
			t.Fatalf("ASSERT_RELATION_%s", rel)
		}
	}
	for _, rel := range []string{"SUPPORTS", "UNKNOWN"} {
		raw, _ := json.Marshal(retainedGraphJSON{RetainedGraphSchema, "rev", []string{"a"}, []retainedEdgeJSON{edge("e", "a", "a", rel, "p", "")}})
		if _, _, err := DecodeRetainedGraph(raw); err == nil {
			t.Fatalf("ASSERT_NONCANONICAL_RELATION_%s", rel)
		}
	}
}

func TestMultiplicityQualifiedSupportRankingAndDensity(t *testing.T) {
	raw := graphBytes(t, []string{"a", "b", "c"}, []retainedEdgeJSON{
		edge("e1", "a", "b", "CALLS", "p1", "alias"), edge("e2", "a", "b", "CALLS", "p1", "alias"),
		edge("e3", "a", "b", "CALLS", "p2", "alias"), edge("e4", "c", "b", "CALLS", "p3", "other"),
		unqualifiedEdge("e5", "c", "b", "CALLS", UnvalidatedAuthority, CustodyProviderVerified, "p4", "bare"),
		unqualifiedEdge("e6", "c", "b", "CALLS", ValidatedAuthority, CustodyCallerAsserted, "p5", "bare"),
	})
	m := localEval(t, Metrics, raw, "CALLS").Metrics
	if m.EdgeInstanceCount != 6 || m.UniqueStructuralRelationCount != 2 || m.IndependentSupportCount != 3 || m.Density == nil || *m.Density != (LocalRational{2, 6}) || m.Nodes[1] != (LocalNodeMetrics{"b", 6, 0}) {
		t.Fatalf("ASSERT_QUALIFIED_MULTIPLICITY_METRICS: %#v", m)
	}
	r := localEval(t, Ranking, raw, "CALLS").Ranking
	if r.Ordered[0] != (LocalRankedNode{"b", 3, 6, 4}) || r.Ordered[1].Node != "a" || r.Ordered[2].Node != "c" {
		t.Fatalf("ASSERT_RANKING_TARGET_SCOPE_TIEBREAK: %#v", r)
	}
	// The same qualified support is independent again for a different target.
	raw = graphBytes(t, []string{"a", "b", "c"}, []retainedEdgeJSON{edge("e1", "a", "b", "CALLS", "p", "g"), edge("e2", "a", "c", "CALLS", "p", "g")})
	r = localEval(t, Ranking, raw, "CALLS").Ranking
	if r.Ordered[0].SupportGroupWeight != 1 || r.Ordered[1].SupportGroupWeight != 1 {
		t.Fatalf("ASSERT_SUPPORT_SCOPED_BY_TARGET: %#v", r)
	}
}

func TestIncrementalWorkAccountingExactBoundaries(t *testing.T) {
	raw := graphBytes(t, []string{"a", "b"}, []retainedEdgeJSON{edge("e", "a", "b", "CALLS", "p", "g")})
	// bytes + nodes + edge admission + selection + kernel(nodes+selected)
	exact := int64(len(raw) + 2 + 1 + 1 + 3)
	for _, delta := range []int64{-1, 0, 1} {
		q := localRequest(Analysis, raw, "CALLS")
		q.Policy.MaxWork = exact + delta
		got, err := EvaluateLocal(q)
		if err != nil {
			t.Fatal(err)
		}
		want := Complete
		if delta < 0 {
			want = Limit
		}
		if got.Status != want {
			t.Fatalf("ASSERT_MAXWORK_%+d: %#v", delta, got)
		}
		a := got.Accounting
		if a.Units != a.DecoderUnits+a.NodeUnits+a.EdgeUnits+a.SelectionUnits+a.KernelUnits {
			t.Fatal("ASSERT_ACCOUNTING_COMPONENT_SUM")
		}
		if got.Status == Limit && a.Units != a.Limit+1 {
			t.Fatalf("ASSERT_INCREMENTAL_STOP_AT_LIMIT: %#v", a)
		}
	}
	q := localRequest(Analysis, raw, "CALLS")
	q.Policy.MaxWork = int64(len(raw) - 1)
	got, err := EvaluateLocal(q)
	if err != nil || got.Status != Limit || got.Accounting.DecoderUnits != int64(len(raw)) || got.Accounting.NodeUnits != 0 || got.Accounting.Units != int64(len(raw)) {
		t.Fatalf("ASSERT_BYTE_PRECHARGE_LIMIT_WITHOUT_PARSE: %#v %v", got, err)
	}
	for _, limit := range []int64{0, -1} {
		q.Policy.MaxWork = limit
		if _, err := EvaluateLocal(q); err == nil {
			t.Fatal("ASSERT_MAXWORK_NONPOSITIVE_REJECT")
		}
	}
}

func TestResultValidationClosedReplayAndNestedMutations(t *testing.T) {
	raw := graphBytes(t, []string{"a", "b"}, []retainedEdgeJSON{edge("e", "a", "b", "CALLS", "p", "g")})
	for _, op := range []Operation{Analysis, Metrics, Ranking} {
		base := localEval(t, op, raw, "CALLS")
		encoded, err := MarshalLocalResult(base)
		if err != nil || ValidateLocalResultJSON(encoded, raw) != nil {
			t.Fatalf("ASSERT_RESULT_VALID_%s: %v", op, err)
		}
		if ValidateLocalResultJSON(encoded) == nil {
			t.Fatal("ASSERT_REPLAY_BYTES_REQUIRED")
		}
		mutations := map[string]func(*LocalResult){
			"operation": func(r *LocalResult) { r.Operation = Operation("OTHER") }, "status": func(r *LocalResult) { r.Status = Status("OTHER") },
			"limit-payload": func(r *LocalResult) { r.Status = Limit }, "extra-payload": func(r *LocalResult) {
				r.Analysis = &LocalAnalysisEvidence{Schema: LocalAnalysisSchema, Findings: []LocalFinding{}}
			},
			"input-digest": func(r *LocalResult) { r.InputDigest = "sha256:" + strings.Repeat("0", 64) }, "digest-shape": func(r *LocalResult) { r.Digest = "SHA256:" + strings.Repeat("0", 64) },
			"accounting": func(r *LocalResult) { r.Accounting.Units++ },
		}
		switch op {
		case Analysis:
			mutations["schema"] = func(r *LocalResult) { r.Analysis.Schema = "wrong" }
			mutations["finding-count"] = func(r *LocalResult) { r.Analysis.Findings[0].Count++ }
		case Metrics:
			mutations["schema"] = func(r *LocalResult) { r.Metrics.Schema = "wrong" }
			mutations["denominator"] = func(r *LocalResult) { r.Metrics.Density.Denominator++ }
			mutations["count-mode"] = func(r *LocalResult) { r.Metrics.CountingMode = "wrong" }
		case Ranking:
			mutations["schema"] = func(r *LocalResult) { r.Ranking.Schema = "wrong" }
			mutations["tie-break"] = func(r *LocalResult) { r.Ranking.TieBreak = "wrong" }
			mutations["score"] = func(r *LocalResult) { r.Ranking.Ordered[0].Score++ }
		}
		for name, mutate := range mutations {
			t.Run(string(op)+"/"+name, func(t *testing.T) {
				x := base
				x.Analysis = cloneAnalysis(base.Analysis)
				x.Metrics = cloneMetrics(base.Metrics)
				x.Ranking = cloneRanking(base.Ranking)
				mutate(&x)
				if name != "digest-shape" {
					x.Digest = localDigest(&x)
				}
				b, _ := json.Marshal(x)
				if ValidateLocalResultJSON(b, raw) == nil {
					t.Fatal("ASSERT_REHASHED_RESULT_MUTATION_REJECT")
				}
			})
		}
	}
	base := localEval(t, Analysis, raw, "CALLS")
	encoded, _ := MarshalLocalResult(base)
	badRaw := [][]byte{append(append([]byte{}, encoded...), ' '), bytes.Replace(encoded, []byte(`"Family"`), []byte(`"Unknown":0,"Family"`), 1), bytes.Replace(encoded, []byte(`"Accounting":{`), []byte(`"Accounting":{"Units":0,`), 1), bytes.Replace(encoded, []byte(`"Scope":"`), []byte(`"Scope":null,"Discard":"`), 1)}
	for i, b := range badRaw {
		if ValidateLocalResultJSON(b, raw) == nil {
			t.Fatalf("ASSERT_RAW_RESULT_MUTATION_%d", i)
		}
	}
}
func cloneAnalysis(x *LocalAnalysisEvidence) *LocalAnalysisEvidence {
	if x == nil {
		return nil
	}
	b, _ := json.Marshal(x)
	var y LocalAnalysisEvidence
	json.Unmarshal(b, &y)
	return &y
}
func cloneMetrics(x *LocalMetricsEvidence) *LocalMetricsEvidence {
	if x == nil {
		return nil
	}
	b, _ := json.Marshal(x)
	var y LocalMetricsEvidence
	json.Unmarshal(b, &y)
	return &y
}
func cloneRanking(x *LocalRankingEvidence) *LocalRankingEvidence {
	if x == nil {
		return nil
	}
	b, _ := json.Marshal(x)
	var y LocalRankingEvidence
	json.Unmarshal(b, &y)
	return &y
}

func TestExactCollectionAndStringBounds(t *testing.T) {
	nodes := make([]string, MaxRetainedNodes)
	for i := range nodes {
		nodes[i] = fmt.Sprintf("n%04d", i)
	}
	raw := graphBytes(t, nodes, nil)
	if _, _, err := DecodeRetainedGraph(raw); err != nil {
		t.Fatalf("ASSERT_EXACT_NODE_BOUND: %v", err)
	}
	nodes = append(nodes, "overflow")
	if _, _, err := DecodeRetainedGraph(graphBytes(t, nodes, nil)); err == nil {
		t.Fatal("ASSERT_NODE_ONE_OVER")
	}

	edges := make([]retainedEdgeJSON, MaxRetainedEdges)
	for i := range edges {
		edges[i] = edge(fmt.Sprintf("e%04d", i), "a", "a", "CALLS", fmt.Sprintf("p%04d", i), "")
	}
	raw = graphBytes(t, []string{"a"}, edges)
	if len(raw) > MaxRetainedBytes {
		t.Fatalf("ASSERT_EDGE_BOUND_REACHABLE: %d", len(raw))
	}
	if _, _, err := DecodeRetainedGraph(raw); err != nil {
		t.Fatalf("ASSERT_EXACT_EDGE_BOUND: %v", err)
	}
	edges = append(edges, edge("z-overflow", "a", "a", "CALLS", "z-overflow", ""))
	if _, _, err := DecodeRetainedGraph(graphBytes(t, []string{"a"}, edges)); err == nil {
		t.Fatal("ASSERT_EDGE_ONE_OVER")
	}

	long := strings.Repeat("x", MaxRetainedString)
	if _, _, err := DecodeRetainedGraph(graphBytes(t, []string{long}, nil)); err != nil {
		t.Fatal("ASSERT_EXACT_STRING_BOUND")
	}
	if _, _, err := DecodeRetainedGraph(graphBytes(t, []string{long + "x"}, nil)); err == nil {
		t.Fatal("ASSERT_STRING_ONE_OVER")
	}

	paddingEdges := make([]retainedEdgeJSON, 2000)
	for i := range paddingEdges {
		paddingEdges[i] = edge(fmt.Sprintf("b%04d", i), "a", "a", "CALLS", fmt.Sprintf("q%04d", i), "")
	}
	base := graphBytes(t, []string{"a"}, paddingEdges)
	remaining := MaxRetainedBytes - len(base)
	for i := range paddingEdges {
		n := remaining
		if n > MaxRetainedString {
			n = MaxRetainedString
		}
		paddingEdges[i].SupportGroup = strings.Repeat("x", n)
		remaining -= n
		if remaining == 0 {
			break
		}
	}
	if remaining != 0 {
		t.Fatalf("ASSERT_EXACT_BYTE_FIXTURE_CAPACITY: %d", remaining)
	}
	raw = graphBytes(t, []string{"a"}, paddingEdges)
	if len(raw) != MaxRetainedBytes {
		t.Fatalf("ASSERT_EXACT_BYTE_LENGTH: %d", len(raw))
	}
	if _, _, err := DecodeRetainedGraph(raw); err != nil {
		t.Fatalf("ASSERT_EXACT_BYTE_BOUND: %v", err)
	}
	if _, _, err := DecodeRetainedGraph(append(raw, ' ')); err == nil {
		t.Fatal("ASSERT_BYTE_ONE_OVER")
	}
}

func TestSemanticBuilderPermutationAndCanonicalAdmission(t *testing.T) {
	a := []retainedEdgeJSON{edge("z", "a", "b", "CALLS", "p2", "g2"), edge("a", "b", "a", "CALLS", "p1", "g1")}
	one := graphBytes(t, []string{"b", "a"}, a)
	two := graphBytes(t, []string{"a", "b"}, []retainedEdgeJSON{a[1], a[0]})
	if !bytes.Equal(one, two) {
		t.Fatal("ASSERT_SEMANTIC_BUILDER_PERMUTATION_CANONICAL")
	}
	first := localEval(t, Metrics, one, "RENDERS_FROM", "CALLS")
	second := localEval(t, Metrics, two, "CALLS", "RENDERS_FROM")
	if !reflect.DeepEqual(first, second) {
		t.Fatal("ASSERT_DETERMINISTIC_REPLAY")
	}
	var wire retainedGraphJSON
	json.Unmarshal(one, &wire)
	wire.Edges[0], wire.Edges[1] = wire.Edges[1], wire.Edges[0]
	noncanonical, _ := json.Marshal(wire)
	if _, _, err := DecodeRetainedGraph(noncanonical); err == nil {
		t.Fatal("ASSERT_CANONICAL_BYTE_ADMISSION_REJECTS_PERMUTATION")
	}
}
