package normativeanalytics

import (
	"reflect"
	"testing"
)

func fixture() Graph {
	return Graph{Nodes: []string{"c", "a", "b"}, Edges: []Edge{{"e2", "b", "c", "DEPENDS_ON"}, {"e1", "a", "b", "REFERENCES"}}}
}
func request(op Operation, limit int64) Request {
	return Request{Operation: op, BuildRevision: "local-revision", Graph: fixture(), Policy: Policy{MaxWork: limit}}
}

func TestContractIdentityAndDescriptorsRemainDormant(t *testing.T) {
	if Family != "normative-analytics" || Version != "v2" || Scope != "NORMATIVE_PROGRAM_B_ONLY" {
		t.Fatal("ASSERT_NORMATIVE_V2_IDENTITY")
	}
	want := []Descriptor{{Analysis, "normative-analysis", "lsp_trace_v2_normative_analysis"}, {Metrics, "normative-metrics", "lsp_trace_v2_normative_metrics"}, {Ranking, "normative-ranking", "lsp_trace_v2_normative_ranking"}}
	if !reflect.DeepEqual(Descriptors(), want) {
		t.Fatal("ASSERT_SYNTHETIC_FIXTURE_QUALIFIED_PACKAGE_PRIVATE_UNSHIPPED")
	}
}

func TestAnalysisIsDeterministicStructuralAndWitnessed(t *testing.T) {
	first, err := Evaluate(request(Analysis, 5))
	second, err2 := Evaluate(request(Analysis, 5))
	if err != nil || err2 != nil || !reflect.DeepEqual(first, second) || first.Status != Complete || first.Accounting != (Accounting{5, 5}) {
		t.Fatalf("ASSERT_ANALYSIS_DETERMINISTIC: %#v %#v %v %v", first, second, err, err2)
	}
	want := []Finding{{"ROOT", "a", 1, []string{"e1"}}, {"LEAF", "c", 1, []string{"e2"}}}
	if first.Analysis == nil || first.Analysis.Schema != AnalysisSchema || !reflect.DeepEqual(first.Analysis.Findings, want) || first.Analysis.Digest == "" {
		t.Fatalf("ASSERT_STRUCTURAL_FINDINGS_RETAINED_WITNESSES: %#v", first.Analysis)
	}
	limited, err := Evaluate(request(Analysis, 4))
	if err != nil || limited.Status != Limit || limited.Accounting != (Accounting{4, 4}) || limited.Analysis != nil {
		t.Fatalf("ASSERT_ANALYSIS_LIMIT_DURING_WORK: %#v %v", limited, err)
	}
}

func TestMetricsDefinesDenominatorAndOmissions(t *testing.T) {
	got, err := Evaluate(request(Metrics, 5))
	if err != nil || got.Metrics == nil || got.Status != Complete {
		t.Fatalf("ASSERT_METRICS_COMPLETE: %#v %v", got, err)
	}
	if got.Metrics.Schema != MetricsSchema || got.Metrics.Density == nil || *got.Metrics.Density != (Rational{2, 6}) || len(got.Metrics.Omissions) != 0 {
		t.Fatalf("ASSERT_METRICS_DENOMINATOR: %#v", got.Metrics)
	}
	empty := Request{Operation: Metrics, BuildRevision: "local", Graph: Graph{Nodes: []string{"only"}}, Policy: Policy{MaxWork: 1}}
	got, err = Evaluate(empty)
	if err != nil || got.Metrics == nil || got.Metrics.Density != nil || !reflect.DeepEqual(got.Metrics.Omissions, []Omission{{"directed_density", "requires at least two nodes"}}) {
		t.Fatalf("ASSERT_METRICS_OMISSION: %#v %v", got, err)
	}
}

func TestRankingScoresAndLexicalTieBreak(t *testing.T) {
	got, err := Evaluate(request(Ranking, 5))
	if err != nil || got.Ranking == nil || got.Status != Complete {
		t.Fatalf("ASSERT_RANKING_COMPLETE: %#v %v", got, err)
	}
	want := []RankedNode{{"b", 1}, {"c", 1}, {"a", 0}}
	if got.Ranking.Schema != RankingSchema || got.Ranking.TieBreak != "score-desc,node-asc" || !reflect.DeepEqual(got.Ranking.Ordered, want) {
		t.Fatalf("ASSERT_RANKING_SCORE_TIEBREAK: %#v", got.Ranking)
	}
	if got.Ranking.Digest == got.MetricsDigestForTest() {
		t.Fatal("ASSERT_OPERATION_DIGEST_DOMAINS_DISTINCT")
	}
}

func (r Result) MetricsDigestForTest() string {
	got, _ := Evaluate(request(Metrics, 5))
	return got.Metrics.Digest
}

func TestInvalidRetainedGraphsFailClosedWithoutInferringCalls(t *testing.T) {
	for _, g := range []Graph{{Nodes: []string{"a", "a"}}, {Nodes: []string{"a"}, Edges: []Edge{{"e", "a", "missing", "CALLS"}}}, {Nodes: []string{"a"}, Edges: []Edge{{"e", "a", "a", ""}}}} {
		if got, err := Evaluate(Request{Operation: Analysis, BuildRevision: "local", Graph: g, Policy: Policy{MaxWork: 10}}); err == nil || got != (Result{}) {
			t.Fatalf("ASSERT_RETAINED_GRAPH_VALIDATION: %#v %v", got, err)
		}
	}
}
