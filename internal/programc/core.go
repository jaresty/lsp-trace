// Package programc provides the deliberately narrow Program C computation core.
package programc

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"reflect"
	"sort"

	lgraph "lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"

	ggraph "gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/community"
	"gonum.org/v1/gonum/graph/iterator"
	"gonum.org/v1/gonum/graph/simple"
)

const (
	ProfileID      = "calls-v1"
	ProfileDigest  = "sha256:f4fb309c6e849b5a8e6057f355b1db6ee3c53414c3c430b67be17f68fe9e97f9"
	MaxNodes       = 10000
	MaxOccurrences = 100000
	ClaimCeiling   = "Structural communities inherit the exact Graph Provenance V5 source claim ceiling; they do not establish runtime execution, feature identity, whole-source completeness, independent source confirmation, provider authentication, permission, or production authority."
	algorithm      = "gonum.org/v1/gonum/graph/community.Leiden"
)

type FailureCode string

const (
	CodeInvalidProvenance FailureCode = "INVALID_PROVENANCE"
	CodeInvalidCalls      FailureCode = "INVALID_CALLS"
	CodeResourceLimit     FailureCode = "RESOURCE_LIMIT"
	CodeComputation       FailureCode = "COMPUTATION_FAILED"
)

type Failure struct {
	Code    FailureCode
	Message string
}

func (f *Failure) Error() string { return string(f.Code) + ": " + f.Message }

type Pair struct{ From, To int64 }
type Occurrence struct {
	Identity, RelationID, Provider, ProviderVersion, Language string
	From, To                                                  int64
	CallSite                                                  lgraph.Range
	Weight                                                    float64
}
type Projection struct {
	NodeIdentities []string
	NodeIDs        map[string]int64
	Occurrences    []Occurrence
	PairWeights    map[Pair]float64
	Source         SourceBinding
}
type Community struct{ Members []string }
type Outcome struct {
	ProfileID, ProfileDigest, Algorithm, LogicalDigest, ClaimCeiling string
	Resolution                                                       float64
	Seed                                                             uint64
	Communities                                                      []Community
	Projection                                                       Projection
	Source                                                           SourceBinding
}

// SemanticReceipt identifies the validated native Graph V5 semantic commitment.
type SemanticReceipt struct {
	ReceiptVersion, SemanticCommitmentDigest, DigestScope string
}

// SourceCompleteness retains the native graph's validated completeness ceiling.
type SourceCompleteness struct {
	TraversalComplete   bool
	SourceGraphComplete string
	CompletenessScope   string
	Truncated           bool
}

// SourceBinding is the immutable admission record for a Program C projection.
// Graph Provenance V5 has no graph-wide custody or separate qualification field:
// CustodyAvailable and QualificationIdentityAvailable therefore remain false.
// CallEvidenceClass and CallEvidenceRole are evidence labels, not custody.
type SourceBinding struct {
	inputBytes, graphV5Bytes                                 []byte
	sensitivity                                              lgraph.SensitivityPolicy
	replayManifest                                           lgraph.ReplayInputManifest
	semantics                                                lgraph.EvidenceSemantics
	admissionReceipt                                         lgraph.EvidenceReceipt
	bundleIdentity                                           lgraph.BundleIdentity
	InputLength, GraphV5Length                               int
	InputSHA256, GraphV5SHA256                               string
	EnvelopeSchemaVersion, GraphSchemaID, GraphSchemaVersion string
	EnvelopeSchemaID                                         string
	EnvelopeSchemaIDAvailable                                bool
	SessionID                                                string
	Generation                                               uint64
	ExecutionBundleID                                        string
	SemanticReceipt                                          SemanticReceipt
	DiagnosticsStatus                                        manageddiagnostic.QueryStatus
	DiagnosticsOmittedRecords, DiagnosticsEvictedRecords     uint64
	Completeness                                             SourceCompleteness
	CustodyAvailable, QualificationIdentityAvailable         bool
	CustodyClass, CallEvidenceClass, CallEvidenceRole        string
}

func (s SourceBinding) InputBytes() []byte   { return append([]byte(nil), s.inputBytes...) }
func (s SourceBinding) GraphV5Bytes() []byte { return append([]byte(nil), s.graphV5Bytes...) }
func (s SourceBinding) Sensitivity() lgraph.SensitivityPolicy {
	v := s.sensitivity
	v.Covered = append([]string(nil), v.Covered...)
	return v
}
func (s SourceBinding) ReplayManifest() lgraph.ReplayInputManifest {
	v := s.replayManifest
	v.Artifacts = append([]lgraph.ReplayArtifact(nil), v.Artifacts...)
	return v
}
func (s SourceBinding) Semantics() lgraph.EvidenceSemantics {
	v := s.semantics
	v.CallEdges.Supports = append([]string(nil), v.CallEdges.Supports...)
	v.CallEdges.DoesNotSupport = append([]string(nil), v.CallEdges.DoesNotSupport...)
	v.DiscoveryRelations.Supports = append([]string(nil), v.DiscoveryRelations.Supports...)
	v.DiscoveryRelations.DoesNotSupport = append([]string(nil), v.DiscoveryRelations.DoesNotSupport...)
	return v
}
func (s SourceBinding) AdmissionReceipt() lgraph.EvidenceReceipt {
	v := s.admissionReceipt
	v.Relations = append([]lgraph.EvidenceRelation(nil), v.Relations...)
	return v
}
func (s SourceBinding) BundleIdentity() lgraph.BundleIdentity {
	v := s.bundleIdentity
	v.ResolvedSeeds = append([]lgraph.InvocationSeed(nil), v.ResolvedSeeds...)
	return v
}

type nativeSemanticReceipt struct {
	ReceiptVersion           string `json:"receipt_version"`
	SemanticCommitmentDigest string `json:"semantic_commitment_digest"`
	DigestScope              string `json:"digest_scope"`
}

type nativeSummary struct {
	TraversalComplete   bool   `json:"traversal_complete"`
	SourceGraphComplete string `json:"source_graph_complete"`
	CompletenessScope   string `json:"completeness_scope"`
	Truncated           bool   `json:"truncated"`
}

type native struct {
	SchemaVersion     string `json:"schema_version"`
	ExecutionBundleID string `json:"execution_bundle_id"`
	Invocation        struct {
		Server     lgraph.ServerInvocation     `json:"server"`
		LanguageID string                      `json:"language_id"`
		Seeds      []lgraph.InvocationSeed     `json:"seeds"`
		Provenance lgraph.InvocationProvenance `json:"provenance"`
	} `json:"invocation"`
	Identity            lgraph.BundleIdentity      `json:"identity"`
	SensitivityPolicy   lgraph.SensitivityPolicy   `json:"sensitivity_policy"`
	ReplayInputManifest lgraph.ReplayInputManifest `json:"replay_input_manifest"`
	Nodes               []lgraph.Node              `json:"nodes"`
	Edges               []lgraph.Edge              `json:"edges"`
	EvidenceSemantics   lgraph.EvidenceSemantics   `json:"evidence_semantics"`
	EvidenceReceipt     *lgraph.EvidenceReceipt    `json:"evidence_receipt"`
	TraceReceipt        nativeSemanticReceipt      `json:"trace_receipt"`
	Summary             nativeSummary              `json:"summary"`
}

func Project(input []byte) (Projection, *Failure) {
	if version, err := graphprovenance.ValidateFor(input, graphprovenance.Family, "v5"); err != nil || version != graphprovenance.VersionV5 {
		return Projection{}, &Failure{Code: CodeInvalidProvenance, Message: fmt.Sprint(err)}
	}
	var envelope graphprovenance.EvidenceV5
	if err := json.Unmarshal(input, &envelope); err != nil {
		return Projection{}, &Failure{Code: CodeInvalidProvenance, Message: err.Error()}
	}
	exact, err := base64.StdEncoding.DecodeString(envelope.GraphV5)
	if err != nil {
		return Projection{}, &Failure{Code: CodeInvalidProvenance, Message: err.Error()}
	}
	var doc native
	if err := json.Unmarshal(exact, &doc); err != nil {
		return Projection{}, &Failure{Code: CodeInvalidProvenance, Message: err.Error()}
	}
	if doc.EvidenceReceipt == nil || doc.EvidenceSemantics.CallEdges.EvidenceClass != "SERVER_REPORTED_CALL_HIERARCHY" || doc.EvidenceSemantics.CallEdges.SupportContribution != 1 {
		return Projection{}, &Failure{Code: CodeInvalidCalls, Message: "exact server-reported call-hierarchy semantics required"}
	}
	receipts := make(map[string]lgraph.EvidenceRelation, len(doc.EvidenceReceipt.Relations))
	for _, r := range doc.EvidenceReceipt.Relations {
		receipts[r.RelationID] = r
	}
	ids := make([]string, len(doc.Nodes))
	for i, n := range doc.Nodes {
		ids[i] = n.ID
	}
	sort.Strings(ids)
	for i := 1; i < len(ids); i++ {
		if ids[i] == ids[i-1] {
			return Projection{}, &Failure{Code: CodeInvalidCalls, Message: "duplicate node identity"}
		}
	}
	if failure := enforceCaps(len(ids), 0); failure != nil {
		return Projection{}, failure
	}
	nodeIDs := make(map[string]int64, len(ids))
	for i, id := range ids {
		nodeIDs[id] = int64(i)
	}
	provider := doc.Invocation.Server.Command
	providerVersion := doc.Invocation.Provenance.ServerVersion
	language := doc.Invocation.LanguageID
	if language == "" && len(doc.Invocation.Seeds) == 1 {
		language = doc.Invocation.Seeds[0].LanguageID
	}
	inputSum := sha256.Sum256(input)
	callRole := ""
	for _, relation := range doc.EvidenceReceipt.Relations {
		if relation.RelationKind == "CALL_RELATION" {
			callRole = relation.EvidenceRole
			break
		}
	}
	binding := SourceBinding{
		inputBytes: append([]byte(nil), input...), graphV5Bytes: append([]byte(nil), exact...),
		InputLength: len(input), InputSHA256: "sha256:" + hex.EncodeToString(inputSum[:]),
		GraphV5Length: len(exact), GraphV5SHA256: envelope.GraphV5SHA256,
		EnvelopeSchemaVersion: envelope.SchemaVersion, GraphSchemaID: envelope.GraphV5SchemaID, GraphSchemaVersion: doc.SchemaVersion,
		SessionID: envelope.SessionID, Generation: envelope.Generation, ExecutionBundleID: doc.ExecutionBundleID,
		SemanticReceipt:   SemanticReceipt{doc.TraceReceipt.ReceiptVersion, doc.TraceReceipt.SemanticCommitmentDigest, doc.TraceReceipt.DigestScope},
		DiagnosticsStatus: envelope.Diagnostics.Status, DiagnosticsOmittedRecords: envelope.Diagnostics.OmittedRecords, DiagnosticsEvictedRecords: envelope.Diagnostics.EvictedRecords,
		Completeness:      SourceCompleteness{doc.Summary.TraversalComplete, doc.Summary.SourceGraphComplete, doc.Summary.CompletenessScope, doc.Summary.Truncated},
		CallEvidenceClass: doc.EvidenceSemantics.CallEdges.EvidenceClass, CallEvidenceRole: callRole,
		sensitivity: doc.SensitivityPolicy, replayManifest: doc.ReplayInputManifest, semantics: doc.EvidenceSemantics, admissionReceipt: *doc.EvidenceReceipt, bundleIdentity: doc.Identity,
	}
	p := Projection{NodeIdentities: ids, NodeIDs: nodeIDs, PairWeights: make(map[Pair]float64), Source: binding}
	for _, edge := range doc.Edges {
		r, ok := receipts[edge.RelationID]
		if !ok || r.RelationKind != "CALL_RELATION" || r.EvidenceClass != "SERVER_REPORTED_CALL_HIERARCHY" || r.EvidenceRole != "CALL_SUPPORT" || r.Direction != "CALLER_TO_CALLEE" || r.SupportContribution != 1 || r.CallerNodeID != edge.CallerNodeID || r.CalleeNodeID != edge.CalleeNodeID || len(edge.CallSites) == 0 {
			return Projection{}, &Failure{Code: CodeInvalidCalls, Message: "unqualified or incomplete CALLS occurrence"}
		}
		from, fromOK := nodeIDs[edge.CallerNodeID]
		to, toOK := nodeIDs[edge.CalleeNodeID]
		if !fromOK || !toOK {
			return Projection{}, &Failure{Code: CodeInvalidCalls, Message: "CALLS endpoint missing"}
		}
		for ordinal, site := range edge.CallSites {
			identity := occurrenceIdentity(edge.RelationID, ordinal, site)
			p.Occurrences = append(p.Occurrences, Occurrence{Identity: identity, RelationID: edge.RelationID, Provider: provider, ProviderVersion: providerVersion, Language: language, From: from, To: to, CallSite: site, Weight: 1})
			p.PairWeights[Pair{From: from, To: to}]++
			if failure := enforceCaps(len(ids), len(p.Occurrences)); failure != nil {
				return Projection{}, failure
			}
		}
	}
	sort.Slice(p.Occurrences, func(i, j int) bool { return p.Occurrences[i].Identity < p.Occurrences[j].Identity })
	return p, nil
}

func occurrenceIdentity(relation string, ordinal int, site lgraph.Range) string {
	b, _ := json.Marshal(struct {
		Relation string       `json:"relation_id"`
		Ordinal  int          `json:"ordinal"`
		Site     lgraph.Range `json:"call_site"`
	}{relation, ordinal, site})
	h := sha256.Sum256(append([]byte("lsp-trace:program-c:calls-v1:occurrence:v1\x00"), b...))
	return "sha256:" + hex.EncodeToString(h[:])
}

func enforceCaps(nodes, occurrences int) *Failure {
	if nodes > MaxNodes {
		return &Failure{Code: CodeResourceLimit, Message: fmt.Sprintf("node cap exceeded: %d > %d", nodes, MaxNodes)}
	}
	if occurrences > MaxOccurrences {
		return &Failure{Code: CodeResourceLimit, Message: fmt.Sprintf("occurrence cap exceeded: %d > %d", occurrences, MaxOccurrences)}
	}
	return nil
}

func Compute(input []byte, seed uint64) (Outcome, *Failure) {
	p, failure := Project(input)
	if failure != nil {
		return Outcome{}, failure
	}
	g := newDirected(p)
	var reduced community.ReducedGraph
	func() {
		defer func() {
			if recover() != nil {
				reduced = nil
			}
		}()
		reduced = community.Leiden(g, 1, rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	}()
	if reduced == nil {
		return Outcome{}, &Failure{Code: CodeComputation, Message: "Gonum Leiden computation failed"}
	}
	communities := reduced.Communities()
	canonical := make([]Community, len(communities))
	for i, members := range communities {
		for _, n := range members {
			canonical[i].Members = append(canonical[i].Members, p.NodeIdentities[n.ID()])
		}
		sort.Strings(canonical[i].Members)
	}
	sort.Slice(canonical, func(i, j int) bool { return compareStrings(canonical[i].Members, canonical[j].Members) < 0 })
	return Outcome{ProfileID: ProfileID, ProfileDigest: ProfileDigest, Algorithm: algorithm, Resolution: 1, Seed: seed, Communities: canonical, LogicalDigest: logicalDigest(canonical), ClaimCeiling: ClaimCeiling, Projection: p, Source: p.Source}, nil
}

func logicalDigest(c []Community) string {
	h := sha256.New()
	h.Write([]byte("lsp-trace:program-c:canonical-communities:v1\x00"))
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(c)))
	h.Write(size[:])
	for _, community := range c {
		binary.BigEndian.PutUint64(size[:], uint64(len(community.Members)))
		h.Write(size[:])
		for _, member := range community.Members {
			binary.BigEndian.PutUint64(size[:], uint64(len(member)))
			h.Write(size[:])
			h.Write([]byte(member))
		}
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
func equalCommunities(a, b []Community) bool { return reflect.DeepEqual(a, b) }
func compareStrings(a, b []string) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}

type weightedEdge struct {
	f, t ggraph.Node
	w    float64
}

func (e weightedEdge) From() ggraph.Node         { return e.f }
func (e weightedEdge) To() ggraph.Node           { return e.t }
func (e weightedEdge) ReversedEdge() ggraph.Edge { return weightedEdge{f: e.t, t: e.f, w: e.w} }
func (e weightedEdge) Weight() float64           { return e.w }

type directed struct {
	nodes    map[int64]ggraph.Node
	from, to map[int64][]int64
	weights  map[Pair]float64
}

func newDirected(p Projection) *directed {
	g := &directed{nodes: make(map[int64]ggraph.Node), from: make(map[int64][]int64), to: make(map[int64][]int64), weights: p.PairWeights}
	for i := range p.NodeIdentities {
		g.nodes[int64(i)] = simple.Node(i)
	}
	for pair := range p.PairWeights {
		g.from[pair.From] = append(g.from[pair.From], pair.To)
		g.to[pair.To] = append(g.to[pair.To], pair.From)
	}
	for id := range g.nodes {
		sort.Slice(g.from[id], func(i, j int) bool { return g.from[id][i] < g.from[id][j] })
		sort.Slice(g.to[id], func(i, j int) bool { return g.to[id][i] < g.to[id][j] })
	}
	return g
}
func (g *directed) Node(id int64) ggraph.Node { return g.nodes[id] }
func (g *directed) Nodes() ggraph.Nodes {
	ns := make([]ggraph.Node, 0, len(g.nodes))
	for _, n := range g.nodes {
		ns = append(ns, n)
	}
	sort.Slice(ns, func(i, j int) bool { return ns[i].ID() < ns[j].ID() })
	return iterator.NewOrderedNodes(ns)
}
func (g *directed) From(id int64) ggraph.Nodes { return g.nodesFor(g.from[id]) }
func (g *directed) To(id int64) ggraph.Nodes   { return g.nodesFor(g.to[id]) }
func (g *directed) nodesFor(ids []int64) ggraph.Nodes {
	if len(ids) == 0 {
		return ggraph.Empty
	}
	ns := make([]ggraph.Node, len(ids))
	for i, id := range ids {
		ns[i] = g.nodes[id]
	}
	return iterator.NewOrderedNodes(ns)
}
func (g *directed) HasEdgeBetween(x, y int64) bool {
	return g.HasEdgeFromTo(x, y) || g.HasEdgeFromTo(y, x)
}
func (g *directed) HasEdgeFromTo(x, y int64) bool { _, ok := g.weights[Pair{x, y}]; return ok }
func (g *directed) Edge(x, y int64) ggraph.Edge   { return g.WeightedEdge(x, y) }
func (g *directed) WeightedEdge(x, y int64) ggraph.WeightedEdge {
	w, ok := g.weights[Pair{x, y}]
	if !ok {
		return nil
	}
	return weightedEdge{g.nodes[x], g.nodes[y], w}
}
func (g *directed) Weight(x, y int64) (float64, bool) { w, ok := g.weights[Pair{x, y}]; return w, ok }

var _ ggraph.WeightedDirected = (*directed)(nil)
