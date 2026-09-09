package normativeanalytics

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"unicode/utf8"
)

const (
	RetainedGraphSchema = "lsp-trace.normative-retained-graph.v1"
	LocalAnalysisSchema = "lsp-trace.local-normative-analysis.v1"
	LocalMetricsSchema  = "lsp-trace.local-normative-metrics.v1"
	LocalRankingSchema  = "lsp-trace.local-normative-ranking.v1"
	LocalScope          = "LOCAL_SYNTHETIC_FIXTURE_QUALIFIED_PACKAGE_PRIVATE_UNSHIPPED"
	MaxRetainedBytes    = 1 << 20
	MaxRetainedNodes    = 4096
	MaxRetainedEdges    = 8192
)

var ErrInvalidLocalRequest = errors.New("normative analytics local v1: invalid request")

var supportedRelations = map[string]struct{}{
	"CALLS": {}, "BINDS_ARGUMENT": {}, "PASSES_CALLBACK": {}, "INVOKES_TASK": {},
	"TRIGGERS_RELOAD": {}, "UPDATES_STATE": {}, "RENDERS_FROM": {}, "SUPPORTS": {},
}

type RetainedEdge struct {
	ID, From, To, Relation, ProvenanceID, SupportGroup string
}
type retainedEdgeJSON struct {
	ID           string `json:"id"`
	From         string `json:"from"`
	To           string `json:"to"`
	Relation     string `json:"relation"`
	ProvenanceID string `json:"provenance_id"`
	SupportGroup string `json:"support_group"`
}
type retainedGraphJSON struct {
	SchemaVersion string             `json:"schema_version"`
	BuildRevision string             `json:"build_revision"`
	Nodes         []string           `json:"nodes"`
	Edges         []retainedEdgeJSON `json:"edges"`
}
type LocalGraph struct {
	BuildRevision string
	Nodes         []string
	Edges         []RetainedEdge
}
type LocalPolicy struct{ MaxWork int64 }
type LocalRequest struct {
	Operation     Operation
	BuildRevision string
	RetainedJSON  []byte
	Relations     []string
	Policy        LocalPolicy
}
type LocalAccounting struct {
	DecoderUnits, NodeUnits, SelectedEdgeUnits, Units, Limit int64
}
type LocalFinding struct {
	Kind, Node     string
	Count          int
	WitnessEdgeIDs []string
}
type LocalAnalysisEvidence struct {
	Schema   string
	Findings []LocalFinding
}
type LocalRational struct{ Numerator, Denominator int64 }
type LocalOmission struct{ Metric, Reason string }
type LocalNodeMetrics struct {
	Node                string
	InDegree, OutDegree int
}
type LocalMetricsEvidence struct {
	Schema, CountingMode, DensityDenominatorMode                                         string
	NodeCount, EdgeInstanceCount, UniqueStructuralRelationCount, IndependentSupportCount int
	Density                                                                              *LocalRational
	Omissions                                                                            []LocalOmission
	Nodes                                                                                []LocalNodeMetrics
}
type LocalRankedNode struct {
	Node                                            string
	SupportGroupWeight, InstanceMultiplicity, Score int
}
type LocalRankingEvidence struct {
	Schema, CountingMode, Denominator, TieBreak string
	Ordered                                     []LocalRankedNode
}
type LocalResult struct {
	Family, Version, Scope, BuildRevision, InputDigest string
	Operation                                          Operation
	SelectedRelations                                  []string
	Status                                             Status
	Accounting                                         LocalAccounting
	Analysis                                           *LocalAnalysisEvidence
	Metrics                                            *LocalMetricsEvidence
	Ranking                                            *LocalRankingEvidence
	Digest                                             string
}

func LocalDescriptors() []Descriptor {
	return []Descriptor{{Analysis, "local-normative-analysis", ""}, {Metrics, "local-normative-metrics", ""}, {Ranking, "local-normative-ranking", ""}}
}

func DecodeRetainedGraph(raw []byte) (LocalGraph, string, error) {
	if len(raw) == 0 || len(raw) > MaxRetainedBytes || !utf8.Valid(raw) || hasDuplicateJSONKey(raw) {
		return LocalGraph{}, "", ErrInvalidLocalRequest
	}
	var wire retainedGraphJSON
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&wire); err != nil {
		return LocalGraph{}, "", ErrInvalidLocalRequest
	}
	if err := requireEOF(d); err != nil {
		return LocalGraph{}, "", ErrInvalidLocalRequest
	}
	canonical, err := json.Marshal(wire)
	if err != nil || !bytes.Equal(raw, canonical) || wire.SchemaVersion != RetainedGraphSchema || wire.BuildRevision == "" || wire.Nodes == nil || wire.Edges == nil || len(wire.Nodes) > MaxRetainedNodes || len(wire.Edges) > MaxRetainedEdges {
		return LocalGraph{}, "", ErrInvalidLocalRequest
	}
	g := LocalGraph{BuildRevision: wire.BuildRevision, Nodes: append([]string(nil), wire.Nodes...), Edges: make([]RetainedEdge, len(wire.Edges))}
	seenNodes := map[string]struct{}{}
	for i, n := range g.Nodes {
		if n == "" || (i > 0 && g.Nodes[i-1] >= n) {
			return LocalGraph{}, "", ErrInvalidLocalRequest
		}
		seenNodes[n] = struct{}{}
	}
	seenIdentity := map[string]struct{}{}
	for i, e := range wire.Edges {
		if _, ok := supportedRelations[e.Relation]; !ok || e.ID == "" || e.ProvenanceID == "" || e.From == "" || e.To == "" {
			return LocalGraph{}, "", ErrInvalidLocalRequest
		}
		if _, ok := seenNodes[e.From]; !ok {
			return LocalGraph{}, "", ErrInvalidLocalRequest
		}
		if _, ok := seenNodes[e.To]; !ok {
			return LocalGraph{}, "", ErrInvalidLocalRequest
		}
		key := e.ID + "\x00" + e.ProvenanceID
		if _, ok := seenIdentity[key]; ok {
			return LocalGraph{}, "", ErrInvalidLocalRequest
		}
		seenIdentity[key] = struct{}{}
		if i > 0 && edgeLess(e, wire.Edges[i-1]) {
			return LocalGraph{}, "", ErrInvalidLocalRequest
		}
		g.Edges[i] = RetainedEdge{e.ID, e.From, e.To, e.Relation, e.ProvenanceID, e.SupportGroup}
	}
	s := sha256.Sum256(raw)
	return g, "sha256:" + hex.EncodeToString(s[:]), nil
}
func edgeLess(a, b retainedEdgeJSON) bool {
	if a.ID != b.ID {
		return a.ID < b.ID
	}
	return a.ProvenanceID < b.ProvenanceID
}
func requireEOF(d *json.Decoder) error {
	var x any
	if err := d.Decode(&x); err != io.EOF {
		return ErrInvalidLocalRequest
	}
	return nil
}
func hasDuplicateJSONKey(raw []byte) bool {
	d := json.NewDecoder(bytes.NewReader(raw))
	var walk func() bool
	walk = func() bool {
		t, err := d.Token()
		if err != nil {
			return true
		}
		v, ok := t.(json.Delim)
		if !ok {
			return false
		}
		switch v {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				k, err := d.Token()
				if err != nil {
					return true
				}
				s, ok := k.(string)
				if !ok || seen[s] {
					return true
				}
				seen[s] = true
				if walk() {
					return true
				}
			}
			_, err = d.Token()
			return err != nil
		case '[':
			for d.More() {
				if walk() {
					return true
				}
			}
			_, err = d.Token()
			return err != nil
		}
		return false
	}
	return walk()
}

func EvaluateLocal(r LocalRequest) (LocalResult, error) {
	if !validOperation(r.Operation) || r.BuildRevision == "" || r.Policy.MaxWork < 1 {
		return LocalResult{}, ErrInvalidLocalRequest
	}
	relations, err := normalizeRelations(r.Relations)
	if err != nil {
		return LocalResult{}, err
	}
	g, inputDigest, err := DecodeRetainedGraph(r.RetainedJSON)
	if err != nil || g.BuildRevision != r.BuildRevision {
		return LocalResult{}, ErrInvalidLocalRequest
	}
	selected := make([]RetainedEdge, 0, len(g.Edges))
	for _, e := range g.Edges {
		if contains(relations, e.Relation) {
			selected = append(selected, e)
		}
	}
	decoderUnits := int64(len(r.RetainedJSON))
	required := decoderUnits + int64(len(g.Nodes)) + int64(len(selected))
	if required < decoderUnits {
		return LocalResult{}, ErrInvalidLocalRequest
	}
	res := LocalResult{Family: Family, Version: "local-v1", Scope: LocalScope, BuildRevision: r.BuildRevision, InputDigest: inputDigest, Operation: r.Operation, SelectedRelations: relations, Status: Complete, Accounting: LocalAccounting{DecoderUnits: decoderUnits, NodeUnits: int64(len(g.Nodes)), SelectedEdgeUnits: int64(len(selected)), Units: required, Limit: r.Policy.MaxWork}}
	if required > r.Policy.MaxWork {
		res.Status = Limit
		res.Accounting.Units = r.Policy.MaxWork
		res.Digest = localDigest(&res)
		return res, nil
	}
	switch r.Operation {
	case Analysis:
		res.Analysis = localAnalyze(g.Nodes, selected)
	case Metrics:
		res.Metrics = localMeasure(g.Nodes, selected)
	case Ranking:
		res.Ranking = localRank(g.Nodes, selected)
	}
	res.Digest = localDigest(&res)
	return res, nil
}
func normalizeRelations(in []string) ([]string, error) {
	if len(in) == 0 {
		return nil, ErrInvalidLocalRequest
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, r := range in {
		if _, ok := supportedRelations[r]; !ok {
			return nil, ErrInvalidLocalRequest
		}
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	sort.Strings(out)
	return out, nil
}
func contains(a []string, s string) bool {
	i := sort.SearchStrings(a, s)
	return i < len(a) && a[i] == s
}
func localAnalyze(nodes []string, edges []RetainedEdge) *LocalAnalysisEvidence {
	in, out := map[string][]string{}, map[string][]string{}
	for _, e := range edges {
		out[e.From] = append(out[e.From], e.ID)
		in[e.To] = append(in[e.To], e.ID)
	}
	f := []LocalFinding{}
	for _, n := range nodes {
		if len(in[n]) == 0 {
			sort.Strings(out[n])
			f = append(f, LocalFinding{"ROOT", n, len(out[n]), out[n]})
		}
		if len(out[n]) == 0 {
			sort.Strings(in[n])
			f = append(f, LocalFinding{"LEAF", n, len(in[n]), in[n]})
		}
	}
	return &LocalAnalysisEvidence{LocalAnalysisSchema, f}
}
func localMeasure(nodes []string, edges []RetainedEdge) *LocalMetricsEvidence {
	m := &LocalMetricsEvidence{Schema: LocalMetricsSchema, CountingMode: "edge-instances; structural-relations=unique(from,to,relation); independent-support=unique-qualified-support-group", DensityDenominatorMode: "ordered-distinct-node-pairs n*(n-1)", NodeCount: len(nodes), EdgeInstanceCount: len(edges), Omissions: []LocalOmission{}, Nodes: make([]LocalNodeMetrics, len(nodes))}
	idx := map[string]int{}
	for i, n := range nodes {
		m.Nodes[i].Node = n
		idx[n] = i
	}
	structural := map[string]bool{}
	support := map[string]bool{}
	pairs := map[string]bool{}
	for _, e := range edges {
		m.Nodes[idx[e.From]].OutDegree++
		m.Nodes[idx[e.To]].InDegree++
		structural[e.From+"\x00"+e.To+"\x00"+e.Relation] = true
		if e.SupportGroup != "" {
			support[e.SupportGroup] = true
		}
		if e.From != e.To {
			pairs[e.From+"\x00"+e.To] = true
		}
	}
	m.UniqueStructuralRelationCount = len(structural)
	m.IndependentSupportCount = len(support)
	if len(nodes) < 2 {
		m.Omissions = append(m.Omissions, LocalOmission{"directed_density", "requires at least two nodes"})
	} else {
		m.Density = &LocalRational{int64(len(pairs)), int64(len(nodes)) * int64(len(nodes)-1)}
	}
	return m
}
func localRank(nodes []string, edges []RetainedEdge) *LocalRankingEvidence {
	scores := make([]LocalRankedNode, len(nodes))
	idx := map[string]int{}
	for i, n := range nodes {
		scores[i].Node = n
		idx[n] = i
	}
	groups := map[string]map[string]bool{}
	for _, e := range edges {
		x := &scores[idx[e.To]]
		x.InstanceMultiplicity++
		if e.SupportGroup != "" {
			if groups[e.To] == nil {
				groups[e.To] = map[string]bool{}
			}
			groups[e.To][e.SupportGroup] = true
		}
	}
	for i := range scores {
		scores[i].SupportGroupWeight = len(groups[scores[i].Node])
		bonus := scores[i].InstanceMultiplicity
		if bonus > 1 {
			bonus = 1
		}
		scores[i].Score = scores[i].SupportGroupWeight + bonus
	}
	sort.Slice(scores, func(i, j int) bool {
		if scores[i].Score != scores[j].Score {
			return scores[i].Score > scores[j].Score
		}
		return scores[i].Node < scores[j].Node
	})
	return &LocalRankingEvidence{LocalRankingSchema, "support-group-weight plus capped-instance-presence", "selected-edge-instances and unique-qualified-support-groups", "score-desc,node-asc", scores}
}
func localSchema(op Operation) string {
	if op == Analysis {
		return LocalAnalysisSchema
	}
	if op == Metrics {
		return LocalMetricsSchema
	}
	return LocalRankingSchema
}
func localDigest(r *LocalResult) string {
	copy := *r
	copy.Digest = ""
	raw, _ := json.Marshal(copy)
	sum := sha256.Sum256(append(append([]byte(localSchema(r.Operation)), 0), raw...))
	return "sha256:" + hex.EncodeToString(sum[:])
}
func MarshalLocalResult(r LocalResult) ([]byte, error) {
	if r.Digest == "" || r.Digest != localDigest(&r) {
		return nil, ErrInvalidLocalRequest
	}
	return json.Marshal(r)
}
func ValidateLocalResultJSON(raw []byte) error {
	if !utf8.Valid(raw) || hasDuplicateJSONKey(raw) {
		return ErrInvalidLocalRequest
	}
	var r LocalResult
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&r) != nil || requireEOF(d) != nil {
		return ErrInvalidLocalRequest
	}
	canon, e := json.Marshal(r)
	if e != nil || !bytes.Equal(raw, canon) || r.Family != Family || r.Version != "local-v1" || r.Scope != LocalScope || r.BuildRevision == "" || r.InputDigest == "" || len(r.SelectedRelations) == 0 || r.Digest != localDigest(&r) {
		return ErrInvalidLocalRequest
	}
	rels, e := normalizeRelations(r.SelectedRelations)
	if e != nil || !equalStrings(rels, r.SelectedRelations) {
		return ErrInvalidLocalRequest
	}
	return nil
}
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
