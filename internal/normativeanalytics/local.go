package normativeanalytics

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"sort"
	"unicode/utf8"
)

const (
	RetainedGraphSchema     = "lsp-trace.normative-retained-graph.v1"
	LocalAnalysisSchema     = "lsp-trace.local-normative-analysis.v1"
	LocalMetricsSchema      = "lsp-trace.local-normative-metrics.v1"
	LocalRankingSchema      = "lsp-trace.local-normative-ranking.v1"
	LocalScope              = "LOCAL_SYNTHETIC_FIXTURE_QUALIFIED_PACKAGE_PRIVATE_UNSHIPPED"
	MaxRetainedBytes        = 2 << 20
	MaxRetainedNodes        = 4096
	MaxRetainedEdges        = 8192
	MaxRetainedString       = 1024
	ValidatedAuthority      = "VALIDATED_AUTHORITY"
	UnvalidatedAuthority    = "UNVALIDATED"
	CustodyProviderVerified = "PROVIDER_VERIFIED"
	CustodyCallerAsserted   = "CALLER_ASSERTED"
	CustodyUnknown          = "UNKNOWN"
	metricsCountingMode     = "edge-instances; structural-relations=unique(from,to,relation); independent-support=unique(validated-authority,custody,provenance-id,support-group)"
	densityMode             = "ordered-distinct-node-pairs n*(n-1)"
	rankingCountingMode     = "qualified-support-by-target plus capped-selected-instance-presence"
	rankingDenominator      = "selected-edge-instances and unique(validated-authority,custody,provenance-id,support-group,target)"
	rankingTieBreak         = "score-desc,node-asc"
)

var ErrInvalidLocalRequest = errors.New("normative analytics local v1: invalid request")

var supportedRelations = map[string]struct{}{
	"CALLS": {}, "BINDS_ARGUMENT": {}, "PASSES_CALLBACK": {}, "INVOKES_TASK": {},
	"TRIGGERS_RELOAD": {}, "UPDATES_STATE": {}, "RENDERS_FROM": {},
}

type RetainedEdge struct {
	ID, From, To, Relation, ProvenanceAuthority, ProvenanceCustody, ProvenanceID, SupportGroup string
}
type retainedEdgeJSON struct {
	ID                  string `json:"id"`
	From                string `json:"from"`
	To                  string `json:"to"`
	Relation            string `json:"relation"`
	ProvenanceAuthority string `json:"provenance_authority"`
	ProvenanceCustody   string `json:"provenance_custody"`
	ProvenanceID        string `json:"provenance_id"`
	SupportGroup        string `json:"support_group"`
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
type localWorkObserver struct {
	HashBytes       int
	ParsedNodes     int
	ParsedEdges     int
	DecoderOffset   int64
	StoppedBefore   string
	UnprocessedTail bool
}
type LocalRequest struct {
	Operation     Operation
	BuildRevision string
	RetainedJSON  []byte
	Relations     []string
	Policy        LocalPolicy
}
type LocalAccounting struct {
	DecoderUnits, NodeUnits, EdgeUnits, SelectionUnits, KernelUnits, Units, Limit int64
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

func validString(s string) bool    { return s != "" && len(s) <= MaxRetainedString && utf8.ValidString(s) }
func validAuthority(s string) bool { return s == ValidatedAuthority || s == UnvalidatedAuthority }
func validCustody(s string) bool {
	return s == CustodyProviderVerified || s == CustodyCallerAsserted || s == CustodyUnknown
}
func qualifiedSupport(e RetainedEdge) bool {
	return e.ProvenanceAuthority == ValidatedAuthority && e.ProvenanceCustody == CustodyProviderVerified && validString(e.ProvenanceID) && validString(e.SupportGroup)
}
func supportKey(e RetainedEdge) string {
	return e.ProvenanceAuthority + "\x00" + e.ProvenanceCustody + "\x00" + e.ProvenanceID + "\x00" + e.SupportGroup
}

func DecodeRetainedGraph(raw []byte) (LocalGraph, string, error) {
	if len(raw) == 0 || len(raw) > MaxRetainedBytes || !utf8.Valid(raw) || hasDuplicateJSONKey(raw) {
		return LocalGraph{}, "", ErrInvalidLocalRequest
	}
	var wire retainedGraphJSON
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&wire) != nil || requireEOF(d) != nil {
		return LocalGraph{}, "", ErrInvalidLocalRequest
	}
	canonical, err := json.Marshal(wire)
	if err != nil || !bytes.Equal(raw, canonical) || wire.SchemaVersion != RetainedGraphSchema || !validString(wire.BuildRevision) || wire.Nodes == nil || wire.Edges == nil || len(wire.Nodes) > MaxRetainedNodes || len(wire.Edges) > MaxRetainedEdges {
		return LocalGraph{}, "", ErrInvalidLocalRequest
	}
	g := LocalGraph{BuildRevision: wire.BuildRevision, Nodes: append([]string(nil), wire.Nodes...), Edges: make([]RetainedEdge, len(wire.Edges))}
	seenNodes := map[string]struct{}{}
	for i, n := range g.Nodes {
		if !validString(n) || (i > 0 && g.Nodes[i-1] >= n) {
			return LocalGraph{}, "", ErrInvalidLocalRequest
		}
		seenNodes[n] = struct{}{}
	}
	seenIdentity := map[string]struct{}{}
	for i, e := range wire.Edges {
		if _, ok := supportedRelations[e.Relation]; !ok || !validString(e.ID) || !validString(e.ProvenanceID) || !validString(e.From) || !validString(e.To) || !validAuthority(e.ProvenanceAuthority) || !validCustody(e.ProvenanceCustody) || len(e.SupportGroup) > MaxRetainedString || !utf8.ValidString(e.SupportGroup) {
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
		g.Edges[i] = RetainedEdge{e.ID, e.From, e.To, e.Relation, e.ProvenanceAuthority, e.ProvenanceCustody, e.ProvenanceID, e.SupportGroup}
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

type workCounter struct{ accounting LocalAccounting }

func (w *workCounter) charge(component *int64, n int64) bool {
	if n < 0 || w.accounting.Units > int64(^uint64(0)>>1)-n {
		return false
	}
	*component += n
	w.accounting.Units += n
	return w.accounting.Units <= w.accounting.Limit
}
func limitResult(r LocalRequest, relations []string, digest string, a LocalAccounting) LocalResult {
	res := LocalResult{Family: Family, Version: "local-v1", Scope: LocalScope, BuildRevision: r.BuildRevision, InputDigest: digest, Operation: r.Operation, SelectedRelations: relations, Status: Limit, Accounting: a}
	res.Digest = localDigest(&res)
	return res
}

// A byte-precharge LIMIT has processed no retained input, so InputDigest is the
// SHA-256 of the empty processed prefix. This keeps LIMIT lexical shape stable
// without scanning bytes the work policy did not admit.
var emptyInputDigest = func() string {
	sum := sha256.Sum256(nil)
	return "sha256:" + hex.EncodeToString(sum[:])
}()

func observeStop(observer *localWorkObserver, d *json.Decoder, raw []byte, before string) {
	if observer == nil {
		return
	}
	observer.DecoderOffset = d.InputOffset()
	observer.StoppedBefore = before
	observer.UnprocessedTail = observer.DecoderOffset < int64(len(raw))
}

func decodeRetainedGraphIncremental(raw []byte, w *workCounter, relations []string, observer *localWorkObserver) (LocalGraph, []RetainedEdge, bool, error) {
	if len(raw) == 0 || len(raw) > MaxRetainedBytes || !utf8.Valid(raw) {
		return LocalGraph{}, nil, false, ErrInvalidLocalRequest
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	expect := func(want any) bool {
		got, err := d.Token()
		return err == nil && reflect.DeepEqual(got, want)
	}
	if !expect(json.Delim('{')) || !expect("schema_version") {
		return LocalGraph{}, nil, false, ErrInvalidLocalRequest
	}
	var schemaVersion string
	if d.Decode(&schemaVersion) != nil || schemaVersion != RetainedGraphSchema || !expect("build_revision") {
		return LocalGraph{}, nil, false, ErrInvalidLocalRequest
	}
	var buildRevision string
	if d.Decode(&buildRevision) != nil || !validString(buildRevision) || !expect("nodes") || !expect(json.Delim('[')) {
		return LocalGraph{}, nil, false, ErrInvalidLocalRequest
	}
	g := LocalGraph{BuildRevision: buildRevision, Nodes: []string{}, Edges: []RetainedEdge{}}
	seenNodes := map[string]struct{}{}
	for d.More() {
		if len(g.Nodes) >= MaxRetainedNodes {
			return LocalGraph{}, nil, false, ErrInvalidLocalRequest
		}
		if !w.charge(&w.accounting.NodeUnits, 1) {
			observeStop(observer, d, raw, "node")
			return LocalGraph{}, nil, true, nil
		}
		var node string
		if d.Decode(&node) != nil || !validString(node) || (len(g.Nodes) > 0 && g.Nodes[len(g.Nodes)-1] >= node) {
			return LocalGraph{}, nil, false, ErrInvalidLocalRequest
		}
		g.Nodes = append(g.Nodes, node)
		seenNodes[node] = struct{}{}
		if observer != nil {
			observer.ParsedNodes++
		}
	}
	if !expect(json.Delim(']')) || !expect("edges") || !expect(json.Delim('[')) {
		return LocalGraph{}, nil, false, ErrInvalidLocalRequest
	}
	selected := []RetainedEdge{}
	seenIdentity := map[string]struct{}{}
	var previous retainedEdgeJSON
	for d.More() {
		if len(g.Edges) >= MaxRetainedEdges {
			return LocalGraph{}, nil, false, ErrInvalidLocalRequest
		}
		if !w.charge(&w.accounting.EdgeUnits, 1) {
			observeStop(observer, d, raw, "edge")
			return LocalGraph{}, nil, true, nil
		}
		if !w.charge(&w.accounting.SelectionUnits, 1) {
			observeStop(observer, d, raw, "edge-selection")
			return LocalGraph{}, nil, true, nil
		}
		var wire retainedEdgeJSON
		if d.Decode(&wire) != nil || !validRetainedEdge(wire, seenNodes) {
			return LocalGraph{}, nil, false, ErrInvalidLocalRequest
		}
		key := wire.ID + "\x00" + wire.ProvenanceID
		if _, duplicate := seenIdentity[key]; duplicate || (len(g.Edges) > 0 && edgeLess(wire, previous)) {
			return LocalGraph{}, nil, false, ErrInvalidLocalRequest
		}
		seenIdentity[key] = struct{}{}
		previous = wire
		edge := RetainedEdge{wire.ID, wire.From, wire.To, wire.Relation, wire.ProvenanceAuthority, wire.ProvenanceCustody, wire.ProvenanceID, wire.SupportGroup}
		g.Edges = append(g.Edges, edge)
		if contains(relations, edge.Relation) {
			selected = append(selected, edge)
		}
		if observer != nil {
			observer.ParsedEdges++
		}
	}
	if !expect(json.Delim(']')) || !expect(json.Delim('}')) || requireEOF(d) != nil {
		return LocalGraph{}, nil, false, ErrInvalidLocalRequest
	}
	canonical, err := json.Marshal(retainedGraphJSON{SchemaVersion: schemaVersion, BuildRevision: buildRevision, Nodes: g.Nodes, Edges: retainedEdgesJSON(g.Edges)})
	if err != nil || !bytes.Equal(raw, canonical) {
		return LocalGraph{}, nil, false, ErrInvalidLocalRequest
	}
	if observer != nil {
		observer.DecoderOffset = d.InputOffset()
	}
	return g, selected, false, nil
}

func validRetainedEdge(e retainedEdgeJSON, nodes map[string]struct{}) bool {
	_, relationOK := supportedRelations[e.Relation]
	_, fromOK := nodes[e.From]
	_, toOK := nodes[e.To]
	return relationOK && validString(e.ID) && validString(e.ProvenanceID) && validString(e.From) && validString(e.To) && validAuthority(e.ProvenanceAuthority) && validCustody(e.ProvenanceCustody) && len(e.SupportGroup) <= MaxRetainedString && utf8.ValidString(e.SupportGroup) && fromOK && toOK
}

func retainedEdgesJSON(edges []RetainedEdge) []retainedEdgeJSON {
	out := make([]retainedEdgeJSON, len(edges))
	for i, e := range edges {
		out[i] = retainedEdgeJSON{e.ID, e.From, e.To, e.Relation, e.ProvenanceAuthority, e.ProvenanceCustody, e.ProvenanceID, e.SupportGroup}
	}
	return out
}

func EvaluateLocal(r LocalRequest) (LocalResult, error) {
	return evaluateLocalObserved(r, nil)
}

func evaluateLocalObserved(r LocalRequest, observer *localWorkObserver) (LocalResult, error) {
	if !validOperation(r.Operation) || !validString(r.BuildRevision) || r.Policy.MaxWork < 1 {
		return LocalResult{}, ErrInvalidLocalRequest
	}
	relations, err := normalizeRelations(r.Relations)
	if err != nil {
		return LocalResult{}, err
	}
	w := workCounter{accounting: LocalAccounting{Limit: r.Policy.MaxWork}}
	if !w.charge(&w.accounting.DecoderUnits, int64(len(r.RetainedJSON))) {
		if observer != nil {
			observer.StoppedBefore = "hash"
			observer.UnprocessedTail = len(r.RetainedJSON) > 0
		}
		return limitResult(r, relations, emptyInputDigest, w.accounting), nil
	}
	sum := sha256.Sum256(r.RetainedJSON)
	inputDigest := "sha256:" + hex.EncodeToString(sum[:])
	if observer != nil {
		observer.HashBytes = len(r.RetainedJSON)
	}
	g, selected, limited, err := decodeRetainedGraphIncremental(r.RetainedJSON, &w, relations, observer)
	if err != nil || (!limited && g.BuildRevision != r.BuildRevision) {
		return LocalResult{}, ErrInvalidLocalRequest
	}
	if limited {
		return limitResult(r, relations, inputDigest, w.accounting), nil
	}
	for range g.Nodes {
		if !w.charge(&w.accounting.KernelUnits, 1) {
			return limitResult(r, relations, inputDigest, w.accounting), nil
		}
	}
	for range selected {
		if !w.charge(&w.accounting.KernelUnits, 1) {
			return limitResult(r, relations, inputDigest, w.accounting), nil
		}
	}
	res := LocalResult{Family: Family, Version: "local-v1", Scope: LocalScope, BuildRevision: r.BuildRevision, InputDigest: inputDigest, Operation: r.Operation, SelectedRelations: relations, Status: Complete, Accounting: w.accounting}
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
	m := &LocalMetricsEvidence{Schema: LocalMetricsSchema, CountingMode: metricsCountingMode, DensityDenominatorMode: densityMode, NodeCount: len(nodes), EdgeInstanceCount: len(edges), Omissions: []LocalOmission{}, Nodes: make([]LocalNodeMetrics, len(nodes))}
	idx := map[string]int{}
	for i, n := range nodes {
		m.Nodes[i].Node = n
		idx[n] = i
	}
	structural, support, pairs := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, e := range edges {
		m.Nodes[idx[e.From]].OutDegree++
		m.Nodes[idx[e.To]].InDegree++
		structural[e.From+"\x00"+e.To+"\x00"+e.Relation] = true
		if qualifiedSupport(e) {
			support[supportKey(e)] = true
		}
		if e.From != e.To {
			pairs[e.From+"\x00"+e.To] = true
		}
	}
	m.UniqueStructuralRelationCount, m.IndependentSupportCount = len(structural), len(support)
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
		if qualifiedSupport(e) {
			if groups[e.To] == nil {
				groups[e.To] = map[string]bool{}
			}
			groups[e.To][supportKey(e)] = true
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
	return &LocalRankingEvidence{LocalRankingSchema, rankingCountingMode, rankingDenominator, rankingTieBreak, scores}
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
func validSHA256(s string) bool {
	if len(s) != 71 || s[:7] != "sha256:" {
		return false
	}
	_, err := hex.DecodeString(s[7:])
	return err == nil && s == "sha256:"+string(bytes.ToLower([]byte(s[7:])))
}
func MarshalLocalResult(r LocalResult) ([]byte, error) {
	if !validSHA256(r.Digest) || r.Digest != localDigest(&r) {
		return nil, ErrInvalidLocalRequest
	}
	return json.Marshal(r)
}

func validateResultShape(r LocalResult) error {
	if r.Family != Family || r.Version != "local-v1" || r.Scope != LocalScope || !validString(r.BuildRevision) || !validSHA256(r.InputDigest) || !validSHA256(r.Digest) || !validOperation(r.Operation) || (r.Status != Complete && r.Status != Limit) || r.Accounting.Limit < 1 {
		return ErrInvalidLocalRequest
	}
	rels, err := normalizeRelations(r.SelectedRelations)
	if err != nil || !equalStrings(rels, r.SelectedRelations) {
		return ErrInvalidLocalRequest
	}
	sum := r.Accounting.DecoderUnits + r.Accounting.NodeUnits + r.Accounting.EdgeUnits + r.Accounting.SelectionUnits + r.Accounting.KernelUnits
	if minAccounting(r.Accounting) < 0 || sum != r.Accounting.Units || r.Accounting.DecoderUnits < 1 {
		return ErrInvalidLocalRequest
	}
	payloads := 0
	if r.Analysis != nil {
		payloads++
	}
	if r.Metrics != nil {
		payloads++
	}
	if r.Ranking != nil {
		payloads++
	}
	if r.Status == Limit {
		if payloads != 0 || r.Accounting.Units <= r.Accounting.Limit {
			return ErrInvalidLocalRequest
		}
		return nil
	}
	if payloads != 1 || r.Accounting.Units > r.Accounting.Limit {
		return ErrInvalidLocalRequest
	}
	switch r.Operation {
	case Analysis:
		if r.Analysis == nil || r.Analysis.Schema != LocalAnalysisSchema || r.Metrics != nil || r.Ranking != nil {
			return ErrInvalidLocalRequest
		}
		for _, f := range r.Analysis.Findings {
			if (f.Kind != "ROOT" && f.Kind != "LEAF") || !validString(f.Node) || f.Count != len(f.WitnessEdgeIDs) || !sort.StringsAreSorted(f.WitnessEdgeIDs) {
				return ErrInvalidLocalRequest
			}
		}
	case Metrics:
		m := r.Metrics
		if m == nil || m.Schema != LocalMetricsSchema || m.CountingMode != metricsCountingMode || m.DensityDenominatorMode != densityMode || r.Analysis != nil || r.Ranking != nil || m.NodeCount != len(m.Nodes) || m.EdgeInstanceCount < 0 || m.UniqueStructuralRelationCount < 0 || m.UniqueStructuralRelationCount > m.EdgeInstanceCount || m.IndependentSupportCount < 0 || m.IndependentSupportCount > m.EdgeInstanceCount {
			return ErrInvalidLocalRequest
		}
		for i, n := range m.Nodes {
			if !validString(n.Node) || n.InDegree < 0 || n.OutDegree < 0 || (i > 0 && m.Nodes[i-1].Node >= n.Node) {
				return ErrInvalidLocalRequest
			}
		}
		if m.NodeCount < 2 {
			if m.Density != nil || !reflect.DeepEqual(m.Omissions, []LocalOmission{{"directed_density", "requires at least two nodes"}}) {
				return ErrInvalidLocalRequest
			}
		} else {
			if m.Density == nil || len(m.Omissions) != 0 || m.Density.Numerator < 0 || m.Density.Denominator != int64(m.NodeCount)*int64(m.NodeCount-1) || m.Density.Numerator > m.Density.Denominator {
				return ErrInvalidLocalRequest
			}
		}
	case Ranking:
		x := r.Ranking
		if x == nil || x.Schema != LocalRankingSchema || x.CountingMode != rankingCountingMode || x.Denominator != rankingDenominator || x.TieBreak != rankingTieBreak || r.Analysis != nil || r.Metrics != nil {
			return ErrInvalidLocalRequest
		}
		seen := map[string]bool{}
		for i, n := range x.Ordered {
			if !validString(n.Node) || seen[n.Node] || n.SupportGroupWeight < 0 || n.InstanceMultiplicity < 0 || n.Score != n.SupportGroupWeight+min(n.InstanceMultiplicity, 1) || (i > 0 && (x.Ordered[i-1].Score < n.Score || (x.Ordered[i-1].Score == n.Score && x.Ordered[i-1].Node >= n.Node))) {
				return ErrInvalidLocalRequest
			}
			seen[n.Node] = true
		}
	}
	return nil
}
func minAccounting(a LocalAccounting) int64 {
	x := a.DecoderUnits
	for _, n := range []int64{a.NodeUnits, a.EdgeUnits, a.SelectionUnits, a.KernelUnits} {
		if n < x {
			x = n
		}
	}
	return x
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func ValidateLocalResultJSON(raw []byte, retained ...[]byte) error {
	if !utf8.Valid(raw) || hasDuplicateJSONKey(raw) {
		return ErrInvalidLocalRequest
	}
	var r LocalResult
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&r) != nil || requireEOF(d) != nil {
		return ErrInvalidLocalRequest
	}
	canon, err := json.Marshal(r)
	if err != nil || !bytes.Equal(raw, canon) || validateResultShape(r) != nil || r.Digest != localDigest(&r) || len(retained) != 1 {
		return ErrInvalidLocalRequest
	}
	expectedInputDigest := emptyInputDigest
	if r.Accounting.DecoderUnits <= r.Accounting.Limit {
		sum := sha256.Sum256(retained[0])
		expectedInputDigest = "sha256:" + hex.EncodeToString(sum[:])
	}
	if r.InputDigest != expectedInputDigest {
		return ErrInvalidLocalRequest
	}
	replay, err := EvaluateLocal(LocalRequest{Operation: r.Operation, BuildRevision: r.BuildRevision, RetainedJSON: retained[0], Relations: r.SelectedRelations, Policy: LocalPolicy{MaxWork: r.Accounting.Limit}})
	if err != nil || !reflect.DeepEqual(r, replay) {
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
