package programc

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"lsp-trace/internal/graphkernel"
)

const (
	relationScopedCouplingSchema     = "lsp-trace.relation-scoped-coupling.v1"
	stage7CExactNumericPolicy        = "EXACT_REDUCED_NONNEGATIVE_INTEGER_RATIONAL_V1"
	CombinedQualificationUnavailable = "COMBINED_QUALIFICATION_UNAVAILABLE"
	Stage7CLimitExceeded             = "STAGE7C_LIMIT_EXCEEDED"
	ZeroDenominatorTyped             = "UNDEFINED_ZERO_DENOMINATOR"
)

type RelationScope string

const (
	RelationScopeCalls                RelationScope = "CALLS"
	RelationScopeReferencesSymbol     RelationScope = "REFERENCES_SYMBOL"
	RelationScopeResolvesToDefinition RelationScope = "RESOLVES_TO_DEFINITION"
	RelationScopeCombined             RelationScope = "COMBINED"
)

type ExactRational struct {
	Numerator, Denominator uint64
}

type CouplingRational struct {
	Numerator, Denominator uint64
	ZeroDenominator        string
}
type Stage7CFailure struct {
	Code, Message string
	Details       []string
	Effects       uint64
}
type ExactRetainedEvidence struct {
	ID, Schema, SHA256 string
	ByteLength         int
	Canonical          []byte
}

type QualificationCustody struct {
	Receipt      ExactRetainedEvidence
	Evidence     []ExactRetainedEvidence
	TrustAnchors []ExactRetainedEvidence
	Epoch        uint64
}

type RetainedStage4AndStage5Evidence struct {
	Qualification *QualificationCustody
	Composition   ExactRetainedEvidence
	Readiness     ExactRetainedEvidence
}

type Stage7CPredecessors struct {
	Stage3Canonical                                                             []byte
	Stage3SHA256                                                                string
	Stage3ByteLength                                                            int
	Stage4PolicyCanonical                                                       []byte
	Stage4PolicyID, Stage4PolicySHA256                                          string
	Stage4PolicyByteLength                                                      int
	Stage4InputCanonical                                                        []byte
	Stage4InputID, Stage4InputSHA256                                            string
	Stage4InputByteLength                                                       int
	Stage4QualificationCanonical                                                []byte
	Stage4QualificationID, Stage4QualificationSchema, Stage4QualificationSHA256 string
	Stage4QualificationByteLength                                               int
	Stage5CompositionCanonical                                                  []byte
	Stage5CompositionID, Stage5CompositionSchema, Stage5CompositionSHA256       string
	Stage5CompositionByteLength                                                 int
	Stage5ReadinessCanonical                                                    []byte
	Stage5ReadinessID, Stage5ReadinessSchema, Stage5ReadinessSHA256             string
	Stage5ReadinessByteLength                                                   int
	QualificationCustody                                                        *QualificationCustody
	Epoch                                                                       uint64
}

func (p Stage7CPredecessors) DeepCopy() Stage7CPredecessors {
	p.Stage3Canonical = clone7(p.Stage3Canonical)
	p.Stage4PolicyCanonical = clone7(p.Stage4PolicyCanonical)
	p.Stage4InputCanonical = clone7(p.Stage4InputCanonical)
	p.Stage4QualificationCanonical = clone7(p.Stage4QualificationCanonical)
	p.Stage5CompositionCanonical = clone7(p.Stage5CompositionCanonical)
	p.Stage5ReadinessCanonical = clone7(p.Stage5ReadinessCanonical)
	p.QualificationCustody = cloneQualificationCustody7(p.QualificationCustody)
	return p
}

func cloneEvidence7(e ExactRetainedEvidence) ExactRetainedEvidence {
	e.Canonical = clone7(e.Canonical)
	return e
}
func cloneQualificationCustody7(q *QualificationCustody) *QualificationCustody {
	if q == nil {
		return nil
	}
	o := *q
	o.Receipt = cloneEvidence7(q.Receipt)
	o.Evidence = append([]ExactRetainedEvidence(nil), q.Evidence...)
	for i := range o.Evidence {
		o.Evidence[i] = cloneEvidence7(o.Evidence[i])
	}
	o.TrustAnchors = append([]ExactRetainedEvidence(nil), q.TrustAnchors...)
	for i := range o.TrustAnchors {
		o.TrustAnchors[i] = cloneEvidence7(o.TrustAnchors[i])
	}
	return &o
}

func validEvidence7(e ExactRetainedEvidence) bool {
	return e.ID != "" && e.Schema != "" && e.ByteLength == len(e.Canonical) && e.ByteLength > 0 && e.SHA256 == hash7(e.Canonical)
}

type qualificationCommitment7 struct {
	ID, SHA256 string
	ByteLength int
}
type qualificationReviewer7 struct{ Identity, IdentityDomain, IdentityClass, Custody, Decision string }
type qualificationReceipt7 struct {
	Schema, Outcome                   string
	Policy, Input                     qualificationCommitment7
	Evidence                          []qualificationCommitment7
	Reviewers                         []qualificationReviewer7
	RequiredReviewerClass             string
	RequiredAcceptances               int
	ValidFromEpoch, ValidThroughEpoch uint64
}
type qualificationAnchor7 struct{ Identity, IdentityDomain, IdentityClass, Custody, Decision, RecordCommitment string }
type stage5Link7 struct {
	Schema                       string
	Epoch                        uint64
	Policy, Input, Qualification qualificationCommitment7
	Composition                  *qualificationCommitment7
}

func validateQualificationCustody7(q QualificationCustody, policy, input qualificationCommitment7, epoch uint64) error {
	if q.Epoch != epoch || !validEvidence7(q.Receipt) {
		return fmt.Errorf("qualification custody commitment")
	}
	var r qualificationReceipt7
	if f := strict7(q.Receipt.Canonical, &r); f != nil || r.Schema != q.Receipt.Schema || r.Outcome != "QUALIFIED" || epoch < r.ValidFromEpoch || epoch > r.ValidThroughEpoch {
		return fmt.Errorf("qualification receipt invalid")
	}
	if r.Policy != policy || r.Input != input {
		return fmt.Errorf("qualification policy/input binding")
	}
	byEvidence := map[string]ExactRetainedEvidence{}
	for _, e := range q.Evidence {
		if !validEvidence7(e) {
			return fmt.Errorf("qualification evidence commitment")
		}
		byEvidence[e.ID] = e
	}
	if len(byEvidence) != len(q.Evidence) || len(r.Evidence) != len(q.Evidence) {
		return fmt.Errorf("qualification evidence inventory")
	}
	for _, c := range r.Evidence {
		e, ok := byEvidence[c.ID]
		if !ok || c != (qualificationCommitment7{e.ID, e.SHA256, e.ByteLength}) {
			return fmt.Errorf("qualification evidence mismatch")
		}
	}
	byTrust := map[string]ExactRetainedEvidence{}
	for _, e := range q.TrustAnchors {
		if !validEvidence7(e) {
			return fmt.Errorf("qualification trust commitment")
		}
		byTrust[e.ID] = e
	}
	accept := 0
	for _, reviewer := range r.Reviewers {
		if reviewer.IdentityClass != r.RequiredReviewerClass || reviewer.Custody != "REVIEWER_ASSERTED" {
			return fmt.Errorf("qualification reviewer class")
		}
		var a qualificationAnchor7
		e, ok := byTrust[reviewer.Identity]
		if !ok || strict7(e.Canonical, &a) != nil || a.Identity != reviewer.Identity || a.IdentityDomain != reviewer.IdentityDomain || a.IdentityClass != reviewer.IdentityClass || a.Custody != reviewer.Custody || a.Decision != reviewer.Decision {
			return fmt.Errorf("qualification reviewer trust")
		}
		record := mustJSON7(struct {
			Receipt, Policy, Input qualificationCommitment7
			Evidence               []qualificationCommitment7
			Reviewer               qualificationReviewer7
		}{qualificationCommitment7{q.Receipt.ID, q.Receipt.SHA256, q.Receipt.ByteLength}, r.Policy, r.Input, r.Evidence, reviewer})
		if a.RecordCommitment != hash7(record) {
			return fmt.Errorf("qualification reviewer record")
		}
		if reviewer.Decision == "ACCEPT" {
			accept++
		}
	}
	if accept < r.RequiredAcceptances || len(byTrust) != len(r.Reviewers) {
		return fmt.Errorf("qualification acceptance")
	}
	return nil
}

func validateStage5Links7(c, r ExactRetainedEvidence, q QualificationCustody, policy, input qualificationCommitment7, epoch uint64) error {
	if !validEvidence7(c) || !validEvidence7(r) {
		return fmt.Errorf("stage5 retained commitment")
	}
	var cd, rd stage5Link7
	if strict7(c.Canonical, &cd) != nil || strict7(r.Canonical, &rd) != nil {
		return fmt.Errorf("stage5 canonical")
	}
	pc, ic, qc := policy, input, qualificationCommitment7{q.Receipt.ID, q.Receipt.SHA256, q.Receipt.ByteLength}
	if cd.Schema != c.Schema || cd.Epoch != epoch || cd.Policy != pc || cd.Input != ic || cd.Qualification != qc || cd.Composition != nil {
		return fmt.Errorf("stage5 composition links")
	}
	cc := qualificationCommitment7{c.ID, c.SHA256, c.ByteLength}
	if rd.Schema != r.Schema || rd.Epoch != epoch || rd.Policy != pc || rd.Input != ic || rd.Qualification != qc || rd.Composition == nil || *rd.Composition != cc {
		return fmt.Errorf("stage5 readiness links")
	}
	return nil
}

type retainedStage7C struct {
	q, c, r ExactRetainedEvidence
	custody *QualificationCustody
}

func (x retainedStage7C) QualificationCanonicalBytes() []byte { return clone7(x.q.Canonical) }
func (x retainedStage7C) CompositionCanonicalBytes() []byte   { return clone7(x.c.Canonical) }
func (x retainedStage7C) ReadinessCanonicalBytes() []byte     { return clone7(x.r.Canonical) }
func (x retainedStage7C) QualificationCustody() *QualificationCustody {
	return cloneQualificationCustody7(x.custody)
}
func LoadRetainedStage4AndStage5Predecessors(policy, input interface {
	CanonicalBytes() []byte
	ID() string
	Digest() string
	ByteLength() int
}, stage3 []byte, epoch uint64, supplied RetainedStage4AndStage5Evidence) (retainedStage7C, error) {
	if len(policy.CanonicalBytes()) == 0 || len(input.CanonicalBytes()) == 0 || len(stage3) == 0 {
		return retainedStage7C{}, fmt.Errorf("missing predecessor")
	}
	if supplied.Qualification == nil {
		return retainedStage7C{}, fmt.Errorf("qualification not issued")
	}
	if err := validateQualificationCustody7(*supplied.Qualification, qualificationCommitment7{policy.ID(), policy.Digest(), policy.ByteLength()}, qualificationCommitment7{input.ID(), input.Digest(), input.ByteLength()}, epoch); err != nil {
		return retainedStage7C{}, err
	}
	if err := validateStage5Links7(supplied.Composition, supplied.Readiness, *supplied.Qualification, qualificationCommitment7{policy.ID(), policy.Digest(), policy.ByteLength()}, qualificationCommitment7{input.ID(), input.Digest(), input.ByteLength()}, epoch); err != nil {
		return retainedStage7C{}, err
	}
	return retainedStage7C{cloneEvidence7(supplied.Qualification.Receipt), cloneEvidence7(supplied.Composition), cloneEvidence7(supplied.Readiness), cloneQualificationCustody7(supplied.Qualification)}, nil
}

type RelationScopedCouplingLimits struct{ MaxInputBytes, MaxNodes, MaxRelations, MaxWork uint64 }
type RelationScopedCouplingInput struct {
	Scope        RelationScope
	Predecessors Stage7CPredecessors
	Limits       RelationScopedCouplingLimits
}

func NewRelationScopedCouplingInputFromPredecessors(s RelationScope, p Stage7CPredecessors, l RelationScopedCouplingLimits) RelationScopedCouplingInput {
	return RelationScopedCouplingInput{s, p.DeepCopy(), l}
}

type stage7CIDWire struct {
	ID, Schema, SHA256 string
	ByteLength         int
}
type stage7CFamilyWire struct {
	Scope                                string
	Evidence                             []byte
	SHA256                               string
	ByteLength                           int
	Availability                         string
	Omissions                            []string
	Input, Included, Omitted, Duplicates uint64
	Terminal, Truncated                  bool
}
type CouplingNode struct {
	NodeID      string
	Ca, Ce      uint64
	Instability CouplingRational
}
type relationScopedCouplingDTO struct {
	Schema, Scope, Completeness, NumericPolicy string
	Graph, Projection                          stage7CIDWire
	Families                                   []stage7CFamilyWire
	Nodes                                      []CouplingNode
	Input, Included, Omitted, Duplicates       uint64
	Terminal, Truncated                        bool
	Limits                                     RelationScopedCouplingLimits
	Authority                                  int
	Accepted                                   bool
	Claims                                     []string
	Digest                                     string
}
type RelationScopedCouplingArtifact struct {
	dto                          relationScopedCouplingDTO
	canonical, graph, projection []byte
	families                     map[RelationScope][]byte
}

func ComputeRelationScopedCouplingArtifact(in RelationScopedCouplingInput) (RelationScopedCouplingArtifact, *Stage7CFailure) {
	if err := validatePredecessors7(in.Predecessors, in.Scope == RelationScopeCombined); err != nil {
		code := "INVALID_PREDECESSOR"
		if in.Scope == RelationScopeCombined {
			code = CombinedQualificationUnavailable
		}
		return RelationScopedCouplingArtifact{}, fail7(code, err.Error())
	}
	if uint64(len(in.Predecessors.Stage3Canonical)) > in.Limits.MaxInputBytes {
		return RelationScopedCouplingArtifact{}, fail7(Stage7CLimitExceeded, "input byte limit exceeded")
	}
	edges, nodes := stage7CEdges(in.Predecessors.Stage3Canonical, in.Scope)
	if uint64(len(edges)) > in.Limits.MaxRelations || uint64(len(nodes)) > in.Limits.MaxNodes || uint64(len(edges)+len(nodes)) > in.Limits.MaxWork {
		return RelationScopedCouplingArtifact{}, fail7(Stage7CLimitExceeded, "coupling resource limit exceeded")
	}
	ca, ce := map[string]uint64{}, map[string]uint64{}
	for _, e := range edges {
		ce[e[0]]++
		ca[e[1]]++
	}
	cn := make([]CouplingNode, 0, len(nodes))
	for _, n := range nodes {
		d := ca[n] + ce[n]
		r := CouplingRational{Numerator: ce[n], Denominator: d}
		if d == 0 {
			r.ZeroDenominator = ZeroDenominatorTyped
		} else {
			x := reduce7(ExactRational{r.Numerator, r.Denominator})
			r.Numerator, r.Denominator = x.Numerator, x.Denominator
		}
		cn = append(cn, CouplingNode{n, ca[n], ce[n], r})
	}
	graph := mustJSON7(struct {
		Nodes []string
		Edges [][2]string
	}{nodes, edges})
	projection := mustJSON7(struct {
		Scope RelationScope
		Nodes []string
	}{in.Scope, nodes})
	families := []stage7CFamilyWire{}
	fm := map[RelationScope][]byte{}
	scopes := []RelationScope{in.Scope}
	if in.Scope == RelationScopeCombined {
		scopes = []RelationScope{RelationScopeCalls, RelationScopeReferencesSymbol, RelationScopeResolvesToDefinition}
	}
	for _, s := range scopes {
		es, _ := stage7CEdges(in.Predecessors.Stage3Canonical, s)
		raw := mustJSON7(es)
		fm[s] = raw
		families = append(families, stage7CFamilyWire{Scope: string(s), Evidence: raw, SHA256: hash7(raw), ByteLength: len(raw), Availability: "AVAILABLE", Input: uint64(len(es)), Included: uint64(len(es)), Terminal: true})
	}
	d := relationScopedCouplingDTO{Schema: relationScopedCouplingSchema, Scope: string(in.Scope), Completeness: "UNKNOWN", NumericPolicy: stage7CExactNumericPolicy, Graph: id7("graph", relationScopedCouplingSchema+".graph", graph), Projection: id7("projection", relationScopedCouplingSchema+".projection", projection), Families: families, Nodes: cn, Input: uint64(len(edges)), Included: uint64(len(edges)), Terminal: true, Limits: in.Limits, Claims: []string{"STRUCTURAL_RELATION_SCOPED_COUPLING_ONLY", "NO_SEMANTIC_AUTHORITY", "NO_FEATURE_ACCEPTANCE"}}
	d.Digest = digestDTO7(d)
	raw := mustJSON7(d)
	return RelationScopedCouplingArtifact{d, raw, graph, projection, fm}, nil
}
func (a RelationScopedCouplingArtifact) DTO() relationScopedCouplingDTO {
	d := a.dto
	d.Families = append([]stage7CFamilyWire(nil), a.dto.Families...)
	for i := range d.Families {
		d.Families[i].Evidence = clone7(d.Families[i].Evidence)
		d.Families[i].Omissions = append([]string(nil), d.Families[i].Omissions...)
	}
	d.Nodes = append([]CouplingNode(nil), a.dto.Nodes...)
	d.Claims = append([]string(nil), a.dto.Claims...)
	return d
}
func (a RelationScopedCouplingArtifact) CanonicalBytes() []byte      { return clone7(a.canonical) }
func (a RelationScopedCouplingArtifact) GraphCanonicalBytes() []byte { return clone7(a.graph) }
func (a RelationScopedCouplingArtifact) ProjectionCanonicalBytes() []byte {
	return clone7(a.projection)
}
func (a RelationScopedCouplingArtifact) FamilyCanonicalBytes(s RelationScope) []byte {
	return clone7(a.families[s])
}
func (a RelationScopedCouplingArtifact) GraphID() string          { return a.dto.Graph.ID }
func (a RelationScopedCouplingArtifact) GraphSchema() string      { return a.dto.Graph.Schema }
func (a RelationScopedCouplingArtifact) ProjectionID() string     { return a.dto.Projection.ID }
func (a RelationScopedCouplingArtifact) ProjectionSchema() string { return a.dto.Projection.Schema }
func (a RelationScopedCouplingArtifact) Digest() string           { return a.dto.Digest }
func (a RelationScopedCouplingArtifact) Scope() RelationScope     { return RelationScope(a.dto.Scope) }
func (a RelationScopedCouplingArtifact) Authority() int           { return a.dto.Authority }
func (a RelationScopedCouplingArtifact) Accepted() bool           { return a.dto.Accepted }
func (a RelationScopedCouplingArtifact) Completeness() string     { return a.dto.Completeness }
func (a RelationScopedCouplingArtifact) Nodes() []CouplingNode {
	return append([]CouplingNode(nil), a.dto.Nodes...)
}
func (a RelationScopedCouplingArtifact) ReproducesHistoricalRobertMartinCoupling(h []graphkernel.RobertMartinCoupling) bool {
	if len(h) != len(a.dto.Nodes) {
		return false
	}
	for i, n := range a.dto.Nodes {
		x := h[i]
		if x.Node != n.NodeID || uint64(x.Ca) != n.Ca || uint64(x.Ce) != n.Ce {
			return false
		}
	}
	return true
}
func ParseRelationScopedCouplingArtifact(raw []byte, l RelationScopedCouplingLimits) (RelationScopedCouplingArtifact, *Stage7CFailure) {
	var d relationScopedCouplingDTO
	if f := strict7(raw, &d); f != nil {
		return RelationScopedCouplingArtifact{}, f
	}
	if d.Limits != l || d.Digest != digestDTO7(d) {
		return RelationScopedCouplingArtifact{}, fail7("SEMANTIC_FORGERY", "coupling semantic validation failed")
	}
	var graph struct {
		Nodes []string
		Edges [][2]string
	}
	var projection struct {
		Scope RelationScope
		Nodes []string
	}
	// The exact predecessor bytes are private carriers, restored only after every
	// public identity and derived field has been recomputed.
	graphRaw := extractFamilyGraph7(d)
	if graphRaw == nil || strict7(graphRaw, &graph) != nil {
		return RelationScopedCouplingArtifact{}, fail7("SEMANTIC_FORGERY", "graph identity or bytes invalid")
	}
	projectionRaw := mustJSON7(struct {
		Scope RelationScope
		Nodes []string
	}{RelationScope(d.Scope), graph.Nodes})
	if d.Graph != id7("graph", relationScopedCouplingSchema+".graph", graphRaw) || d.Projection != id7("projection", relationScopedCouplingSchema+".projection", projectionRaw) || strict7(projectionRaw, &projection) != nil {
		return RelationScopedCouplingArtifact{}, fail7("SEMANTIC_FORGERY", "coupling identities invalid")
	}
	ca, ce := map[string]uint64{}, map[string]uint64{}
	for _, e := range graph.Edges {
		ce[e[0]]++
		ca[e[1]]++
	}
	wantNodes := make([]CouplingNode, 0, len(graph.Nodes))
	for _, n := range graph.Nodes {
		den := ca[n] + ce[n]
		r := CouplingRational{Numerator: ce[n], Denominator: den}
		if den == 0 {
			r.ZeroDenominator = ZeroDenominatorTyped
		} else {
			x := reduce7(ExactRational{r.Numerator, r.Denominator})
			r.Numerator, r.Denominator = x.Numerator, x.Denominator
		}
		wantNodes = append(wantNodes, CouplingNode{n, ca[n], ce[n], r})
	}
	wantClaims := []string{"STRUCTURAL_RELATION_SCOPED_COUPLING_ONLY", "NO_SEMANTIC_AUTHORITY", "NO_FEATURE_ACCEPTANCE"}
	if fmt.Sprint(d.Claims) != fmt.Sprint(wantClaims) {
		return RelationScopedCouplingArtifact{}, fail7("SEMANTIC_FORGERY", "claim ceiling invalid")
	}
	scope := RelationScope(d.Scope)
	if scope != RelationScopeCalls && scope != RelationScopeReferencesSymbol && scope != RelationScopeResolvesToDefinition && scope != RelationScopeCombined {
		return RelationScopedCouplingArtifact{}, fail7("SEMANTIC_FORGERY", "coupling scope invalid")
	}
	if !equalCouplingNodes7(d.Nodes, wantNodes) || d.Input != uint64(len(graph.Edges)) || d.Included != d.Input || d.Omitted != 0 || d.Duplicates != 0 || !d.Terminal || d.Truncated || d.Schema != relationScopedCouplingSchema || d.Completeness != "UNKNOWN" || d.NumericPolicy != stage7CExactNumericPolicy || d.Authority != 0 || d.Accepted {
		return RelationScopedCouplingArtifact{}, fail7("SEMANTIC_FORGERY", "coupling derivation invalid")
	}
	fm := map[RelationScope][]byte{}
	expectedFamilies := map[RelationScope]bool{scope: true}
	if scope == RelationScopeCombined {
		expectedFamilies = map[RelationScope]bool{RelationScopeCalls: true, RelationScopeReferencesSymbol: true, RelationScopeResolvesToDefinition: true}
	}
	for _, f := range d.Families {
		familyScope := RelationScope(f.Scope)
		if !expectedFamilies[familyScope] || fm[familyScope] != nil || f.Availability != "AVAILABLE" || len(f.Omissions) != 0 || f.SHA256 != hash7(f.Evidence) || f.ByteLength != len(f.Evidence) || f.Included != f.Input || f.Omitted != 0 || f.Duplicates != 0 || !f.Terminal || f.Truncated {
			return RelationScopedCouplingArtifact{}, fail7("SEMANTIC_FORGERY", "family derivation invalid")
		}
		var familyEdges [][2]string
		if strict7(f.Evidence, &familyEdges) != nil || f.Input != uint64(len(familyEdges)) {
			return RelationScopedCouplingArtifact{}, fail7("SEMANTIC_FORGERY", "family evidence accounting invalid")
		}
		fm[familyScope] = clone7(f.Evidence)
	}
	if len(fm) != len(expectedFamilies) {
		return RelationScopedCouplingArtifact{}, fail7("SEMANTIC_FORGERY", "family inventory incomplete")
	}
	return RelationScopedCouplingArtifact{dto: d, canonical: clone7(raw), graph: clone7(graphRaw), projection: projectionRaw, families: fm}, nil
}

func extractFamilyGraph7(d relationScopedCouplingDTO) []byte {
	if len(d.Families) == 0 {
		return nil
	}
	var edges [][2]string
	for _, f := range d.Families {
		var e [][2]string
		if strict7(f.Evidence, &e) != nil {
			return nil
		}
		edges = append(edges, e...)
	}
	seen := map[[2]string]bool{}
	out := edges[:0]
	nodes := map[string]bool{}
	for _, e := range edges {
		nodes[e[0]] = true
		nodes[e[1]] = true
		if !seen[e] {
			seen[e] = true
			out = append(out, e)
		}
	}
	ns := make([]string, 0, len(nodes))
	for n := range nodes {
		ns = append(ns, n)
	}
	sort.Strings(ns)
	sort.Slice(out, func(i, j int) bool {
		if out[i][0] != out[j][0] {
			return out[i][0] < out[j][0]
		}
		return out[i][1] < out[j][1]
	})
	return mustJSON7(struct {
		Nodes []string
		Edges [][2]string
	}{ns, out})
}
func equalCouplingNodes7(a, b []CouplingNode) bool {
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

func validCombined(p Stage7CPredecessors) bool { return validatePredecessors7(p, true) == nil }

func validatePredecessors7(p Stage7CPredecessors, combined bool) error {
	if p.Stage3ByteLength != len(p.Stage3Canonical) || p.Stage3SHA256 != hash7(p.Stage3Canonical) || p.Stage4PolicyByteLength != len(p.Stage4PolicyCanonical) || p.Stage4PolicySHA256 != hash7(p.Stage4PolicyCanonical) || p.Stage4InputByteLength != len(p.Stage4InputCanonical) || p.Stage4InputSHA256 != hash7(p.Stage4InputCanonical) || p.Stage4PolicyID == "" || p.Stage4InputID == "" {
		return fmt.Errorf("predecessor exact carrier mismatch")
	}
	if !combined {
		return nil
	}
	if p.QualificationCustody == nil {
		return fmt.Errorf("qualification not issued")
	}
	pc, ic := qualificationCommitment7{p.Stage4PolicyID, p.Stage4PolicySHA256, p.Stage4PolicyByteLength}, qualificationCommitment7{p.Stage4InputID, p.Stage4InputSHA256, p.Stage4InputByteLength}
	if err := validateQualificationCustody7(*p.QualificationCustody, pc, ic, p.Epoch); err != nil {
		return err
	}
	q := p.QualificationCustody.Receipt
	if !bytes.Equal(p.Stage4QualificationCanonical, q.Canonical) || p.Stage4QualificationID != q.ID || p.Stage4QualificationSchema != q.Schema || p.Stage4QualificationSHA256 != q.SHA256 || p.Stage4QualificationByteLength != q.ByteLength {
		return fmt.Errorf("qualification carrier mismatch")
	}
	c := ExactRetainedEvidence{p.Stage5CompositionID, p.Stage5CompositionSchema, p.Stage5CompositionSHA256, p.Stage5CompositionByteLength, p.Stage5CompositionCanonical}
	r := ExactRetainedEvidence{p.Stage5ReadinessID, p.Stage5ReadinessSchema, p.Stage5ReadinessSHA256, p.Stage5ReadinessByteLength, p.Stage5ReadinessCanonical}
	return validateStage5Links7(c, r, *p.QualificationCustody, pc, ic, p.Epoch)
}

type edge7 struct{ From, To, Kind string }

func stage7CEdges(raw []byte, scope RelationScope) ([][2]string, []string) {
	var v any
	_ = json.Unmarshal(raw, &v)
	var all []edge7
	walk7(v, &all)
	seen := map[[2]string]bool{}
	ns := map[string]bool{}
	var out [][2]string
	for _, e := range all {
		if scope != RelationScopeCombined && e.Kind != string(scope) {
			continue
		}
		ns[e.From] = true
		ns[e.To] = true
		p := [2]string{e.From, e.To}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	nodes := make([]string, 0, len(ns))
	for n := range ns {
		nodes = append(nodes, n)
	}
	sort.Strings(nodes)
	sort.Slice(out, func(i, j int) bool {
		if out[i][0] != out[j][0] {
			return out[i][0] < out[j][0]
		}
		return out[i][1] < out[j][1]
	})
	return out, nodes
}
func walk7(v any, out *[]edge7) {
	switch x := v.(type) {
	case []any:
		for _, y := range x {
			walk7(y, out)
		}
	case map[string]any:
		f, _ := x["From"].(string)
		if f == "" {
			f, _ = x["from"].(string)
		}
		t, _ := x["To"].(string)
		if t == "" {
			t, _ = x["to"].(string)
		}
		k, _ := x["Kind"].(string)
		if k == "" {
			k, _ = x["kind"].(string)
		}
		if f != "" && t != "" {
			*out = append(*out, edge7{f, t, k})
		}
		for _, y := range x {
			walk7(y, out)
		}
	}
}
func canonicalNode7(i int) string {
	if i < 26 {
		return string(rune('a' + i))
	}
	return fmt.Sprintf("n-%d", i)
}

func reduce7(r ExactRational) ExactRational {
	a, b := r.Numerator, r.Denominator
	for b != 0 {
		a, b = b, a%b
	}
	if a > 1 {
		r.Numerator /= a
		r.Denominator /= a
	}
	return r
}
func mustJSON7(v any) []byte                        { b, _ := json.Marshal(v); return append(b, '\n') }
func hash7(b []byte) string                         { h := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(h[:]) }
func id7(id, s string, b []byte) stage7CIDWire      { return stage7CIDWire{id, s, hash7(b), len(b)} }
func digestDTO7(v relationScopedCouplingDTO) string { v.Digest = ""; return hash7(mustJSON7(v)) }
func clone7(b []byte) []byte                        { return append([]byte(nil), b...) }
func fail7(c, m string) *Stage7CFailure {
	return &Stage7CFailure{Code: c, Message: m, Details: []string{m}}
}
func strict7(raw []byte, v any) *Stage7CFailure {
	if len(raw) == 0 || raw[len(raw)-1] != '\n' || (len(raw) > 1 && raw[len(raw)-2] == '\n') || bytes.Contains(raw, []byte("\r\n")) {
		return fail7("INVALID_CANONICAL_JSON", "exact one LF required")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return fail7("INVALID_CANONICAL_JSON", err.Error())
	}
	var extra any
	if d.Decode(&extra) == nil {
		return fail7("INVALID_CANONICAL_JSON", "trailing value")
	}
	if !bytes.Equal(raw, mustJSON7(v)) {
		return fail7("INVALID_CANONICAL_JSON", "noncanonical JSON")
	}
	return nil
}
