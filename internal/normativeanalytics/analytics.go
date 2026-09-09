// Package normativeanalytics defines dormant, unshipped Program B analytics v2 contracts.
package normativeanalytics

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
)

const (
	Family         = "normative-analytics"
	Version        = "v2"
	Scope          = "NORMATIVE_PROGRAM_B_ONLY"
	AnalysisSchema = "lsp-trace.normative-analysis.v2"
	MetricsSchema  = "lsp-trace.normative-metrics.v2"
	RankingSchema  = "lsp-trace.normative-ranking.v2"
	SchemaVersion  = AnalysisSchema // retained compatibility; results carry operation-specific schemas.
)

type Operation string

const (
	Analysis Operation = "ANALYSIS"
	Metrics  Operation = "METRICS"
	Ranking  Operation = "RANKING"
)

type Status string

const (
	Complete Status = "COMPLETE"
	Limit    Status = "LIMIT"
)

var ErrInvalidRequest = errors.New("normative analytics v2: invalid request")

type Descriptor struct {
	Operation         Operation
	Command, Protocol string
}

var descriptors = [...]Descriptor{{Analysis, "normative-analysis", "lsp_trace_v2_normative_analysis"}, {Metrics, "normative-metrics", "lsp_trace_v2_normative_metrics"}, {Ranking, "normative-ranking", "lsp_trace_v2_normative_ranking"}}

func Descriptors() []Descriptor {
	out := make([]Descriptor, len(descriptors))
	copy(out, descriptors[:])
	return out
}

// Edge is retained evidence. Relation is descriptive and is never inferred or rewritten as CALLS.
type Edge struct{ ID, From, To, Relation string }
type Graph struct {
	Nodes []string
	Edges []Edge
}
type Policy struct{ MaxWork int64 }
type Accounting struct{ Units, Limit int64 }
type Finding struct {
	Kind, Node     string
	Count          int
	WitnessEdgeIDs []string
}
type AnalysisEvidence struct {
	Schema   string
	Findings []Finding
	Digest   string
}
type Rational struct{ Numerator, Denominator int }
type Omission struct{ Metric, Reason string }
type NodeMetrics struct {
	Node                string
	InDegree, OutDegree int
}
type MetricsEvidence struct {
	Schema               string
	NodeCount, EdgeCount int
	Density              *Rational
	Omissions            []Omission
	Nodes                []NodeMetrics
	Digest               string
}
type RankedNode struct {
	Node  string
	Score int
}
type RankingEvidence struct {
	Schema, TieBreak string
	Ordered          []RankedNode
	Digest           string
}
type Result struct {
	Family, Version, Scope string
	Operation              Operation
	Status                 Status
	Accounting             Accounting
	Analysis               *AnalysisEvidence
	Metrics                *MetricsEvidence
	Ranking                *RankingEvidence
}
type Request struct {
	Operation     Operation
	BuildRevision string
	Graph         Graph
	Policy        Policy
}

type budget struct{ used, max int64 }

func (b *budget) take() bool {
	if b.used == b.max {
		return false
	}
	b.used++
	return true
}
func canonicalDigest(domain string, value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(append(append([]byte(domain), 0), raw...))
	return "sha256:" + hex.EncodeToString(sum[:])
}
func normalizeGraph(g Graph) (Graph, error) {
	g.Nodes = append([]string(nil), g.Nodes...)
	g.Edges = append([]Edge(nil), g.Edges...)
	sort.Strings(g.Nodes)
	sort.Slice(g.Edges, func(i, j int) bool { return g.Edges[i].ID < g.Edges[j].ID })
	seen := map[string]bool{}
	for _, n := range g.Nodes {
		if n == "" || seen[n] {
			return Graph{}, ErrInvalidRequest
		}
		seen[n] = true
	}
	edges := map[string]bool{}
	for _, e := range g.Edges {
		if e.ID == "" || edges[e.ID] || !seen[e.From] || !seen[e.To] || e.Relation == "" {
			return Graph{}, ErrInvalidRequest
		}
		edges[e.ID] = true
	}
	return g, nil
}

// Evaluate is a local computation over caller-supplied retained evidence. Admission
// is not execution permission; provenance claims are validated separately.
func Evaluate(request Request) (Result, error) { return evaluate(request) }
func evaluate(r Request) (Result, error) {
	if !validOperation(r.Operation) || r.BuildRevision == "" || r.Policy.MaxWork < 1 {
		return Result{}, ErrInvalidRequest
	}
	g, err := normalizeGraph(r.Graph)
	if err != nil {
		return Result{}, err
	}
	b := budget{max: r.Policy.MaxWork}
	result := Result{Family: Family, Version: Version, Scope: Scope, Operation: r.Operation, Status: Complete}
	switch r.Operation {
	case Analysis:
		result.Analysis = analyze(g, &b)
	case Metrics:
		result.Metrics = measure(g, &b)
	case Ranking:
		result.Ranking = rank(g, &b)
	}
	result.Accounting = Accounting{b.used, b.max}
	if operationEvidenceMissing(result) {
		result.Status = Limit
	}
	return result, nil
}
func analyze(g Graph, b *budget) *AnalysisEvidence {
	in, out := map[string][]string{}, map[string][]string{}
	for _, e := range g.Edges {
		if !b.take() {
			return nil
		}
		out[e.From] = append(out[e.From], e.ID)
		in[e.To] = append(in[e.To], e.ID)
	}
	findings := []Finding{}
	for _, n := range g.Nodes {
		if !b.take() {
			return nil
		}
		if len(in[n]) == 0 {
			findings = append(findings, Finding{"ROOT", n, len(out[n]), append([]string(nil), out[n]...)})
		}
		if len(out[n]) == 0 {
			findings = append(findings, Finding{"LEAF", n, len(in[n]), append([]string(nil), in[n]...)})
		}
	}
	for i := range findings {
		sort.Strings(findings[i].WitnessEdgeIDs)
	}
	e := &AnalysisEvidence{Schema: AnalysisSchema, Findings: findings}
	e.Digest = canonicalDigest(AnalysisSchema, e)
	return e
}
func measure(g Graph, b *budget) *MetricsEvidence {
	nodes := make([]NodeMetrics, len(g.Nodes))
	idx := map[string]int{}
	for i, n := range g.Nodes {
		if !b.take() {
			return nil
		}
		nodes[i].Node = n
		idx[n] = i
	}
	pairs := map[[2]string]bool{}
	for _, e := range g.Edges {
		if !b.take() {
			return nil
		}
		nodes[idx[e.From]].OutDegree++
		nodes[idx[e.To]].InDegree++
		if e.From != e.To {
			pairs[[2]string{e.From, e.To}] = true
		}
	}
	m := &MetricsEvidence{Schema: MetricsSchema, NodeCount: len(g.Nodes), EdgeCount: len(g.Edges), Nodes: nodes, Omissions: []Omission{}}
	if len(g.Nodes) < 2 {
		m.Omissions = append(m.Omissions, Omission{"directed_density", "requires at least two nodes"})
	} else {
		m.Density = &Rational{len(pairs), len(g.Nodes) * (len(g.Nodes) - 1)}
	}
	m.Digest = canonicalDigest(MetricsSchema, m)
	return m
}
func rank(g Graph, b *budget) *RankingEvidence {
	scores := make([]RankedNode, len(g.Nodes))
	idx := map[string]int{}
	for i, n := range g.Nodes {
		if !b.take() {
			return nil
		}
		scores[i].Node = n
		idx[n] = i
	}
	for _, e := range g.Edges {
		if !b.take() {
			return nil
		}
		scores[idx[e.To]].Score++
	}
	sort.Slice(scores, func(i, j int) bool {
		if scores[i].Score == scores[j].Score {
			return scores[i].Node < scores[j].Node
		}
		return scores[i].Score > scores[j].Score
	})
	e := &RankingEvidence{Schema: RankingSchema, TieBreak: "score-desc,node-asc", Ordered: scores}
	e.Digest = canonicalDigest(RankingSchema, e)
	return e
}
func operationEvidenceMissing(r Result) bool {
	switch r.Operation {
	case Analysis:
		return r.Analysis == nil
	case Metrics:
		return r.Metrics == nil
	case Ranking:
		return r.Ranking == nil
	}
	return true
}
func validOperation(o Operation) bool { return o == Analysis || o == Metrics || o == Ranking }
