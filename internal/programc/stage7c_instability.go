package programc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/bits"
	"sort"
)

const (
	successorInstabilitySchema           = "lsp-trace.successor-instability.v1"
	SuccessorRunComplete                 = "COMPLETE"
	SuccessorRunFailed                   = "FAILED"
	SuccessorRunIncomplete               = "INCOMPLETE"
	SuccessorReadinessReady              = "READY"
	SuccessorReadinessPublicationBlocked = "PUBLICATION_BLOCKED"
	ThresholdInclusive                   = "INCLUSIVE"
	ThresholdExclusive                   = "EXCLUSIVE"
)

type SuccessorThreshold struct {
	Numerator, Denominator uint64
	Boundary               string
}
type SuccessorInstabilityLimits struct{ MaxInputBytes, MaxRuns, MaxPairs, MaxCommunities, MaxNodes, MaxWork uint64 }
type SuccessorRun struct {
	Seed, Ordinal           uint64
	RunID, Status, ResultID string
	Result                  []byte
	ResultSHA256            string
	ResultByteLength        int
}
type SuccessorInstabilityInput struct {
	Predecessors                 Stage7CPredecessors
	Seeds                        []uint64
	Threshold                    SuccessorThreshold
	Limits                       SuccessorInstabilityLimits
	Runs                         []SuccessorRun
	historical, historicalPolicy []byte
	historicalDigest             string
	historicalRequest            InstabilityRequest
}
type SuccessorRunAccounting struct{ Expected, Complete, Failed, Incomplete, Missing, Terminal uint64 }
type SuccessorPairAccounting struct{ Expected, Complete, Blocked uint64 }
type SuccessorPartitionPair struct{ Left, Right [][]string }
type SuccessorPartitionComparison struct {
	Matched, Reassigned, Unmatched, CommunityDenominator, NodeIntersection, NodeUnion uint64
	Jaccard, UnmatchedRate                                                            ExactRational
	Stable                                                                            bool
}
type SuccessorPair struct {
	LeftRunID, RightRunID                                                             string
	Matched, Reassigned, Unmatched, CommunityDenominator, NodeIntersection, NodeUnion uint64
	Jaccard, UnmatchedRate                                                            CouplingRational
	Stable                                                                            bool
}
type stage7CExactWire struct {
	ID, Schema, SHA256 string
	ByteLength         int
	Bytes              []byte
}
type successorInstabilityDTO struct {
	Schema, Algorithm, Profile, NumericPolicy                                                                                     string
	Stage3, Stage4Policy, Stage4Input, Stage4Qualification, Stage5Composition, Stage5Readiness                                    stage7CExactWire
	Parameters                                                                                                                    []byte
	DeclaredSeeds                                                                                                                 []uint64
	ParametersSHA256                                                                                                              string
	Runs                                                                                                                          []SuccessorRun
	Pairs                                                                                                                         []SuccessorPair
	ExpectedRuns, CompleteRuns, FailedRuns, IncompleteRuns, MissingRuns, TerminalRuns, ExpectedPairs, CompletePairs, BlockedPairs uint64
	Outcome                                                                                                                       string
	PublicationAllowed, PublicationBlocked                                                                                        bool
	Limits                                                                                                                        SuccessorInstabilityLimits
	Authority                                                                                                                     int
	Accepted                                                                                                                      bool
	Completeness, Digest                                                                                                          string
	HistoricalArtifact, HistoricalPolicy                                                                                          []byte
	HistoricalArtifactSHA256, HistoricalPolicySHA256                                                                              string
	QualificationEvidence, QualificationTrust                                                                                     []ExactRetainedEvidence
	Epoch                                                                                                                         uint64
}
type SuccessorInstabilityArtifact struct {
	dto                                                                  successorInstabilityDTO
	canonical, stage3, qualification, composition, readiness, parameters []byte
	historicalDigest                                                     string
	historical, historicalPolicy                                         []byte
}

func NewSuccessorInstabilityInputFromPredecessors(p Stage7CPredecessors, seeds []uint64, t SuccessorThreshold, l SuccessorInstabilityLimits) SuccessorInstabilityInput {
	return SuccessorInstabilityInput{Predecessors: p.DeepCopy(), Seeds: append([]uint64(nil), seeds...), Threshold: t, Limits: l}
}
func NewSuccessorInstabilityInputFromHistoricalV1(p Stage7CPredecessors, request InstabilityRequest, raw, policy []byte, digest string, runs []SuccessorRun, l SuccessorInstabilityLimits) SuccessorInstabilityInput {
	in := NewSuccessorInstabilityInputFromPredecessors(p, append([]uint64(nil), request.DeclaredSeeds...), SuccessorThreshold{1, 2, ThresholdInclusive}, l)
	in.Runs = append([]SuccessorRun(nil), runs...)
	in.historical = clone7(raw)
	in.historicalPolicy = clone7(policy)
	in.historicalDigest = digest
	in.historicalRequest = request
	return in
}
func (in SuccessorInstabilityInput) DeepCopy() SuccessorInstabilityInput {
	in.Predecessors = in.Predecessors.DeepCopy()
	in.Seeds = append([]uint64(nil), in.Seeds...)
	in.Runs = append([]SuccessorRun(nil), in.Runs...)
	for i := range in.Runs {
		in.Runs[i].Result = clone7(in.Runs[i].Result)
	}
	in.historical = clone7(in.historical)
	in.historicalPolicy = clone7(in.historicalPolicy)
	return in
}

func ComputeSuccessorInstabilityArtifact(in SuccessorInstabilityInput) (SuccessorInstabilityArtifact, *Stage7CFailure) {
	if err := validatePredecessors7(in.Predecessors, true); err != nil {
		return SuccessorInstabilityArtifact{}, fail7(CombinedQualificationUnavailable, err.Error())
	}
	if len(in.historical) > 0 && (in.historicalDigest != InstabilityPolicyDigest() || !bytes.Equal(in.historicalPolicy, InstabilityPolicyBytes()) || ValidateInstability(in.historical, in.historicalRequest) != nil) {
		return SuccessorInstabilityArtifact{}, fail7("INVALID_HISTORICAL_INSTABILITY", "historical instability validation failed")
	}
	if uint64(len(in.Predecessors.Stage3Canonical)) > in.Limits.MaxInputBytes || uint64(len(in.Seeds)) > in.Limits.MaxRuns {
		return SuccessorInstabilityArtifact{}, fail7(Stage7CLimitExceeded, "instability input limit exceeded")
	}
	expected := uint64(len(in.Seeds))
	pairsExpected := expected * (expected - 1) / 2
	if pairsExpected > in.Limits.MaxPairs {
		return SuccessorInstabilityArtifact{}, fail7(Stage7CLimitExceeded, "pair limit exceeded")
	}
	_, nodes := stage7CEdges(in.Predecessors.Stage3Canonical, RelationScopeCombined)
	if uint64(len(nodes)) > in.Limits.MaxNodes || uint64(len(nodes)) > in.Limits.MaxCommunities || expected*uint64(len(nodes)+1) > in.Limits.MaxWork {
		return SuccessorInstabilityArtifact{}, fail7(Stage7CLimitExceeded, "successor work limit exceeded")
	}
	seeds := append([]uint64(nil), in.Seeds...)
	sort.Slice(seeds, func(i, j int) bool { return seeds[i] < seeds[j] })
	by := map[uint64]SuccessorRun{}
	for _, r := range in.Runs {
		if _, exists := by[r.Seed]; exists {
			return SuccessorInstabilityArtifact{}, fail7("INVALID_RUN_INVENTORY", "duplicate seed run")
		}
		by[r.Seed] = r
	}
	runs := make([]SuccessorRun, 0, len(seeds))
	acct := SuccessorRunAccounting{Expected: expected}
	parts := map[string][][]string{}
	for _, s := range seeds {
		r, ok := by[s]
		if !ok {
			acct.Missing++
			continue
		}
		if r.RunID == "" {
			r.RunID = fmt.Sprintf("run-%d", s)
		}
		switch r.Status {
		case SuccessorRunComplete:
			acct.Complete++
			acct.Terminal++
		case SuccessorRunFailed:
			acct.Failed++
			acct.Terminal++
		case SuccessorRunIncomplete:
			acct.Incomplete++
		default:
			acct.Incomplete++
		}
		if r.Status == SuccessorRunComplete {
			var p [][]string
			if r.RunID == "" || r.ResultID == "" || r.ResultSHA256 != hash7(r.Result) || r.ResultByteLength != len(r.Result) || strict7(r.Result, &p) != nil || !validPartition7(p, nodes) {
				return SuccessorInstabilityArtifact{}, fail7("INVALID_RUN_RESULT", "complete run result custody invalid")
			}
			parts[r.RunID] = canonParts7(p)
		} else if len(r.Result) != 0 || r.ResultID != "" || r.ResultSHA256 != "" || r.ResultByteLength != 0 {
			return SuccessorInstabilityArtifact{}, fail7("INVALID_RUN_RESULT", "non-complete run carries result")
		}
		runs = append(runs, r)
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i].RunID < runs[j].RunID })
	pa := SuccessorPairAccounting{Expected: pairsExpected}
	var ps []SuccessorPair
	for i := 0; i < len(runs); i++ {
		for j := i + 1; j < len(runs); j++ {
			if runs[i].Status != SuccessorRunComplete || runs[j].Status != SuccessorRunComplete {
				pa.Blocked++
				continue
			}
			c, f := CompareSuccessorPartitions(SuccessorPartitionPair{parts[runs[i].RunID], parts[runs[j].RunID]})
			if f != nil {
				return SuccessorInstabilityArtifact{}, f
			}
			pa.Complete++
			ps = append(ps, SuccessorPair{runs[i].RunID, runs[j].RunID, c.Matched, c.Reassigned, c.Unmatched, c.CommunityDenominator, c.NodeIntersection, c.NodeUnion, wireRational7(c.Jaccard), wireRational7(c.UnmatchedRate), EvaluateSuccessorThreshold(c.UnmatchedRate, in.Threshold)})
		}
	}
	pa.Blocked = pa.Expected - pa.Complete
	params := mustJSON7(in.Threshold)
	qualified := validCombined(in.Predecessors)
	mode := mode7(in.Predecessors.Stage3Canonical)
	if !qualified {
		runs = nil
		ps = nil
		acct = SuccessorRunAccounting{}
		pa = SuccessorPairAccounting{}
	}
	allowed := qualified && mode == "calls-only" && acct.Complete == acct.Expected && pa.Complete == pa.Expected
	for _, p := range ps {
		allowed = allowed && p.Stable
	}
	outcome := SuccessorReadinessPublicationBlocked
	if allowed {
		outcome = SuccessorReadinessReady
	}
	idraw := func(id, s string, b []byte) stage7CExactWire {
		return stage7CExactWire{id, s, hash7(b), len(b), clone7(b)}
	}
	d := successorInstabilityDTO{Schema: successorInstabilitySchema, Algorithm: "DETERMINISTIC_SUCCESSOR_PARTITION_V1", Profile: mode, NumericPolicy: stage7CExactNumericPolicy, Stage3: idraw("stage3", "typed-occurrences", in.Predecessors.Stage3Canonical), Stage4Policy: idraw(in.Predecessors.Stage4PolicyID, "stage4-policy", in.Predecessors.Stage4PolicyCanonical), Stage4Input: idraw(in.Predecessors.Stage4InputID, "stage4-input", in.Predecessors.Stage4InputCanonical), Stage4Qualification: idraw(in.Predecessors.Stage4QualificationID, in.Predecessors.Stage4QualificationSchema, in.Predecessors.Stage4QualificationCanonical), Stage5Composition: idraw(in.Predecessors.Stage5CompositionID, in.Predecessors.Stage5CompositionSchema, in.Predecessors.Stage5CompositionCanonical), Stage5Readiness: idraw(in.Predecessors.Stage5ReadinessID, in.Predecessors.Stage5ReadinessSchema, in.Predecessors.Stage5ReadinessCanonical), Parameters: params, DeclaredSeeds: seeds, ParametersSHA256: hash7(params), Runs: runs, Pairs: ps, ExpectedRuns: acct.Expected, CompleteRuns: acct.Complete, FailedRuns: acct.Failed, IncompleteRuns: acct.Incomplete, MissingRuns: acct.Missing, TerminalRuns: acct.Terminal, ExpectedPairs: pa.Expected, CompletePairs: pa.Complete, BlockedPairs: pa.Blocked, Outcome: outcome, PublicationAllowed: allowed, PublicationBlocked: !allowed, Limits: in.Limits, Completeness: "UNKNOWN"}
	d.HistoricalArtifact = clone7(in.historical)
	d.HistoricalPolicy = clone7(in.historicalPolicy)
	d.Epoch = in.Predecessors.Epoch
	if in.Predecessors.QualificationCustody != nil {
		d.QualificationEvidence = cloneEvidenceSlice7(in.Predecessors.QualificationCustody.Evidence)
		d.QualificationTrust = cloneEvidenceSlice7(in.Predecessors.QualificationCustody.TrustAnchors)
	}
	d.HistoricalArtifactSHA256 = hashOptional7(in.historical)
	d.HistoricalPolicySHA256 = in.historicalDigest
	d.Digest = digestInst7(d)
	raw := mustJSON7(d)
	return SuccessorInstabilityArtifact{d, raw, clone7(in.Predecessors.Stage3Canonical), clone7(in.Predecessors.Stage4QualificationCanonical), clone7(in.Predecessors.Stage5CompositionCanonical), clone7(in.Predecessors.Stage5ReadinessCanonical), params, in.historicalDigest, clone7(in.historical), clone7(in.historicalPolicy)}, nil
}
func CompareSuccessorPartitions(p SuccessorPartitionPair) (SuccessorPartitionComparison, *Stage7CFailure) {
	l := canonParts7(p.Left)
	r := canonParts7(p.Right)
	lm, rm := map[string]int{}, map[string]int{}
	for i, c := range l {
		for _, n := range c {
			lm[n] = i
		}
	}
	for i, c := range r {
		for _, n := range c {
			rm[n] = i
		}
	}
	union := map[string]bool{}
	for n := range lm {
		union[n] = true
	}
	for n := range rm {
		union[n] = true
	}
	inter := uint64(0)
	for n := range lm {
		if _, ok := rm[n]; ok {
			inter++
		}
	}
	matched := maximumCommunityMatching7(l, r)
	den := uint64(len(l))
	if len(r) > len(l) {
		den = uint64(len(r))
	}
	unmatched := uint64(len(l)+len(r)) - 2*matched
	reassigned := uint64(0)
	for n, li := range lm {
		if ri, ok := rm[n]; ok && !equalStr7(l[li], r[ri]) {
			reassigned++
		}
	}
	jr := ExactRational{inter, uint64(len(union))}
	if len(union) == 0 {
		jr = ExactRational{0, 1}
	} else {
		jr = reduce7(jr)
	}
	ur := ExactRational{unmatched, den}
	if den == 0 {
		ur = ExactRational{0, 1}
	} else {
		ur = reduce7(ur)
	}
	return SuccessorPartitionComparison{Matched: matched, Reassigned: reassigned, Unmatched: unmatched, CommunityDenominator: den, NodeIntersection: inter, NodeUnion: uint64(len(union)), Jaccard: jr, UnmatchedRate: ur}, nil
}

func maximumCommunityMatching7(left, right [][]string) uint64 {
	matchedRight := make([]int, len(right))
	for i := range matchedRight {
		matchedRight[i] = -1
	}
	var search func(int, []bool) bool
	search = func(li int, seen []bool) bool {
		for ri := range right {
			if seen[ri] || !overlap7(left[li], right[ri]) {
				continue
			}
			seen[ri] = true
			if matchedRight[ri] < 0 || search(matchedRight[ri], seen) {
				matchedRight[ri] = li
				return true
			}
		}
		return false
	}
	var matched uint64
	for li := range left {
		if search(li, make([]bool, len(right))) {
			matched++
		}
	}
	return matched
}

func overlap7(a, b []string) bool {
	for i, j := 0, 0; i < len(a) && j < len(b); {
		if a[i] == b[j] {
			return true
		}
		if a[i] < b[j] {
			i++
		} else {
			j++
		}
	}
	return false
}
func EvaluateSuccessorThreshold(v ExactRational, t SuccessorThreshold) bool {
	if v.Denominator == 0 || t.Denominator == 0 {
		return false
	}
	ahi, alo := bits.Mul64(v.Numerator, t.Denominator)
	bhi, blo := bits.Mul64(t.Numerator, v.Denominator)
	cmp := 0
	if ahi < bhi || (ahi == bhi && alo < blo) {
		cmp = -1
	} else if ahi > bhi || (ahi == bhi && alo > blo) {
		cmp = 1
	}
	if t.Boundary == ThresholdInclusive {
		return cmp <= 0
	}
	return cmp < 0
}
func canonParts7(x [][]string) [][]string {
	o := make([][]string, len(x))
	for i := range x {
		o[i] = append([]string(nil), x[i]...)
		sort.Strings(o[i])
	}
	sort.Slice(o, func(i, j int) bool { return fmt.Sprint(o[i]) < fmt.Sprint(o[j]) })
	return o
}
func equalStr7(a, b []string) bool {
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
func mode7(raw []byte) string {
	var v any
	_ = json.Unmarshal(raw, &v)
	var es []edge7
	walk7(v, &es)
	k := map[string]bool{}
	for _, e := range es {
		k[e.Kind] = true
	}
	c, r, d := k["CALLS"], k["REFERENCES_SYMBOL"], k["RESOLVES_TO_DEFINITION"]
	switch {
	case !c && !r && !d:
		return "empty-zero"
	case c && !r && !d:
		return "calls-only"
	case !c && r && !d:
		return "refs-only"
	case !c && !r && d:
		return "defs-only"
	case !c && r && d:
		return "refs-defs"
	case c && r && !d:
		return "calls-refs"
	case c && !r && d:
		return "calls-defs"
	default:
		return "all"
	}
}
func digestInst7(v successorInstabilityDTO) string                      { v.Digest = ""; return hash7(mustJSON7(v)) }
func (a SuccessorInstabilityArtifact) DTO() successorInstabilityDTO     { b := a.DeepCopy(); return b.dto }
func (a SuccessorInstabilityArtifact) CanonicalBytes() []byte           { return clone7(a.canonical) }
func (a SuccessorInstabilityArtifact) ParametersCanonicalBytes() []byte { return clone7(a.parameters) }
func (a SuccessorInstabilityArtifact) Stage3CanonicalBytes() []byte     { return clone7(a.stage3) }
func (a SuccessorInstabilityArtifact) QualificationCanonicalBytes() []byte {
	return clone7(a.qualification)
}
func (a SuccessorInstabilityArtifact) CompositionCanonicalBytes() []byte {
	return clone7(a.composition)
}
func (a SuccessorInstabilityArtifact) ReadinessCanonicalBytes() []byte { return clone7(a.readiness) }
func (a SuccessorInstabilityArtifact) Runs() []SuccessorRun {
	o := append([]SuccessorRun(nil), a.dto.Runs...)
	for i := range o {
		o[i].Result = clone7(o[i].Result)
	}
	return o
}
func (r SuccessorRun) ResultCanonicalBytes() []byte { return clone7(r.Result) }
func (a SuccessorInstabilityArtifact) Pairs() []SuccessorPair {
	return append([]SuccessorPair(nil), a.dto.Pairs...)
}
func (a SuccessorInstabilityArtifact) Claims() []string {
	return []string{"STRUCTURAL_ONLY", "NO_SEMANTIC_AUTHORITY"}
}
func (a SuccessorInstabilityArtifact) RunAccounting() SuccessorRunAccounting {
	return SuccessorRunAccounting{a.dto.ExpectedRuns, a.dto.CompleteRuns, a.dto.FailedRuns, a.dto.IncompleteRuns, a.dto.MissingRuns, a.dto.TerminalRuns}
}
func (a SuccessorInstabilityArtifact) PairAccounting() SuccessorPairAccounting {
	return SuccessorPairAccounting{a.dto.ExpectedPairs, a.dto.CompletePairs, a.dto.BlockedPairs}
}
func (a SuccessorInstabilityArtifact) PublicationAllowed() bool { return a.dto.PublicationAllowed }
func (a SuccessorInstabilityArtifact) PublicationBlocked() bool { return a.dto.PublicationBlocked }
func (a SuccessorInstabilityArtifact) Outcome() string          { return a.dto.Outcome }
func (a SuccessorInstabilityArtifact) Mode() string             { return a.dto.Profile }
func (a SuccessorInstabilityArtifact) Authority() int           { return a.dto.Authority }
func (a SuccessorInstabilityArtifact) Accepted() bool           { return a.dto.Accepted }
func (a SuccessorInstabilityArtifact) Completeness() string     { return a.dto.Completeness }
func (a SuccessorInstabilityArtifact) AggregateInstability() ExactRational {
	var n, d uint64
	for _, p := range a.dto.Pairs {
		n += p.Unmatched
		d += p.CommunityDenominator
	}
	if d == 0 {
		return ExactRational{0, 1}
	}
	return reduce7(ExactRational{n, d})
}
func (a SuccessorInstabilityArtifact) DeepCopy() SuccessorInstabilityArtifact {
	b := a
	b.canonical = clone7(a.canonical)
	b.stage3 = clone7(a.stage3)
	b.qualification = clone7(a.qualification)
	b.composition = clone7(a.composition)
	b.readiness = clone7(a.readiness)
	b.parameters = clone7(a.parameters)
	b.historical = clone7(a.historical)
	b.historicalPolicy = clone7(a.historicalPolicy)
	b.dto.DeclaredSeeds = append([]uint64(nil), a.dto.DeclaredSeeds...)
	b.dto.Stage3.Bytes = clone7(a.dto.Stage3.Bytes)
	b.dto.Stage4Policy.Bytes = clone7(a.dto.Stage4Policy.Bytes)
	b.dto.Stage4Input.Bytes = clone7(a.dto.Stage4Input.Bytes)
	b.dto.Stage4Qualification.Bytes = clone7(a.dto.Stage4Qualification.Bytes)
	b.dto.Stage5Composition.Bytes = clone7(a.dto.Stage5Composition.Bytes)
	b.dto.Stage5Readiness.Bytes = clone7(a.dto.Stage5Readiness.Bytes)
	b.dto.HistoricalArtifact = clone7(a.dto.HistoricalArtifact)
	b.dto.HistoricalPolicy = clone7(a.dto.HistoricalPolicy)
	b.dto.QualificationEvidence = cloneEvidenceSlice7(a.dto.QualificationEvidence)
	b.dto.QualificationTrust = cloneEvidenceSlice7(a.dto.QualificationTrust)
	b.dto.Runs = a.Runs()
	b.dto.Pairs = a.Pairs()
	return b
}
func (a SuccessorInstabilityArtifact) ReproducesHistoricalInstabilityV1(raw []byte, d string) bool {
	return len(raw) > 0 && bytes.Equal(raw, a.historical) && hash7(raw) == a.dto.HistoricalArtifactSHA256 && bytes.Equal(a.historicalPolicy, InstabilityPolicyBytes()) && d == InstabilityPolicyDigest() && d == a.historicalDigest && d == a.dto.HistoricalPolicySHA256
}
func ParseSuccessorInstabilityArtifact(raw []byte, l SuccessorInstabilityLimits) (SuccessorInstabilityArtifact, *Stage7CFailure) {
	var d successorInstabilityDTO
	if f := strict7(raw, &d); f != nil {
		return SuccessorInstabilityArtifact{}, f
	}
	if d.Limits != l || d.Digest != digestInst7(d) || !validInstabilityDTO7(d) {
		return SuccessorInstabilityArtifact{}, fail7("SEMANTIC_FORGERY", "instability semantic validation failed")
	}
	return SuccessorInstabilityArtifact{dto: d, canonical: clone7(raw), stage3: clone7(d.Stage3.Bytes), qualification: clone7(d.Stage4Qualification.Bytes), composition: clone7(d.Stage5Composition.Bytes), readiness: clone7(d.Stage5Readiness.Bytes), parameters: clone7(d.Parameters), historicalDigest: d.HistoricalPolicySHA256, historical: clone7(d.HistoricalArtifact), historicalPolicy: clone7(d.HistoricalPolicy)}, nil
}
func validInstabilityDTO7(d successorInstabilityDTO) bool {
	if d.Schema != successorInstabilitySchema || d.Algorithm != "DETERMINISTIC_SUCCESSOR_PARTITION_V1" || d.NumericPolicy != stage7CExactNumericPolicy || d.ParametersSHA256 != hash7(d.Parameters) || d.Authority != 0 || d.Accepted || d.Completeness != "UNKNOWN" {
		return false
	}
	q := QualificationCustody{Receipt: ExactRetainedEvidence{d.Stage4Qualification.ID, d.Stage4Qualification.Schema, d.Stage4Qualification.SHA256, d.Stage4Qualification.ByteLength, d.Stage4Qualification.Bytes}, Evidence: cloneEvidenceSlice7(d.QualificationEvidence), TrustAnchors: cloneEvidenceSlice7(d.QualificationTrust), Epoch: d.Epoch}
	pc, ic := qualificationCommitment7{d.Stage4Policy.ID, d.Stage4Policy.SHA256, d.Stage4Policy.ByteLength}, qualificationCommitment7{d.Stage4Input.ID, d.Stage4Input.SHA256, d.Stage4Input.ByteLength}
	if validateQualificationCustody7(q, pc, ic, d.Epoch) != nil || validateStage5Links7(ExactRetainedEvidence{d.Stage5Composition.ID, d.Stage5Composition.Schema, d.Stage5Composition.SHA256, d.Stage5Composition.ByteLength, d.Stage5Composition.Bytes}, ExactRetainedEvidence{d.Stage5Readiness.ID, d.Stage5Readiness.Schema, d.Stage5Readiness.SHA256, d.Stage5Readiness.ByteLength, d.Stage5Readiness.Bytes}, q, pc, ic, d.Epoch) != nil {
		return false
	}
	if len(d.HistoricalArtifact) == 0 {
		if d.HistoricalArtifactSHA256 != "" || len(d.HistoricalPolicy) != 0 || d.HistoricalPolicySHA256 != "" {
			return false
		}
	} else if d.HistoricalArtifactSHA256 != hash7(d.HistoricalArtifact) || !bytes.Equal(d.HistoricalPolicy, InstabilityPolicyBytes()) || d.HistoricalPolicySHA256 != InstabilityPolicyDigest() {
		return false
	}
	for _, c := range []stage7CExactWire{d.Stage3, d.Stage4Policy, d.Stage4Input, d.Stage4Qualification, d.Stage5Composition, d.Stage5Readiness} {
		if c.ID == "" || c.Schema == "" || c.ByteLength != len(c.Bytes) || c.ByteLength == 0 || c.SHA256 != hash7(c.Bytes) {
			return false
		}
	}
	for i, s := range d.DeclaredSeeds {
		if i > 0 && d.DeclaredSeeds[i-1] >= s {
			return false
		}
	}

	var threshold SuccessorThreshold
	if strict7(d.Parameters, &threshold) != nil {
		return false
	}
	scope := RelationScopeCombined
	switch d.Profile {
	case "calls-only":
		scope = RelationScopeCalls
	case "refs-only":
		scope = RelationScopeReferencesSymbol
	case "defs-only":
		scope = RelationScopeResolvesToDefinition
	}
	_, admittedNodes := stage7CEdges(d.Stage3.Bytes, scope)
	parts := make(map[string][][]string, len(d.Runs))
	for i, run := range d.Runs {
		if i > 0 && d.Runs[i-1].RunID >= run.RunID {
			return false
		}
		if run.Status == SuccessorRunComplete {
			var partition [][]string
			if run.ResultSHA256 != hash7(run.Result) || run.ResultByteLength != len(run.Result) || run.ResultID != run.RunID+"-result" || strict7(run.Result, &partition) != nil || !validPartition7(partition, admittedNodes) {
				return false
			}
			parts[run.RunID] = canonParts7(partition)
		} else if len(run.Result) != 0 || run.ResultID != "" || run.ResultSHA256 != "" || run.ResultByteLength != 0 {
			return false
		}
	}
	var pairs []SuccessorPair
	for i := 0; i < len(d.Runs); i++ {
		for j := i + 1; j < len(d.Runs); j++ {
			if d.Runs[i].Status != SuccessorRunComplete || d.Runs[j].Status != SuccessorRunComplete {
				continue
			}
			comparison, failure := CompareSuccessorPartitions(SuccessorPartitionPair{parts[d.Runs[i].RunID], parts[d.Runs[j].RunID]})
			if failure != nil {
				return false
			}
			pairs = append(pairs, SuccessorPair{d.Runs[i].RunID, d.Runs[j].RunID, comparison.Matched, comparison.Reassigned, comparison.Unmatched, comparison.CommunityDenominator, comparison.NodeIntersection, comparison.NodeUnion, wireRational7(comparison.Jaccard), wireRational7(comparison.UnmatchedRate), EvaluateSuccessorThreshold(comparison.UnmatchedRate, threshold)})
		}
	}
	if !equalPairs7(d.Pairs, pairs) {
		return false
	}
	expectedRuns := uint64(len(d.DeclaredSeeds))
	expectedPairs := expectedRuns * (expectedRuns - 1) / 2
	var complete, failed, incomplete uint64
	seen := map[uint64]bool{}
	declared := map[uint64]bool{}
	for _, s := range d.DeclaredSeeds {
		declared[s] = true
	}
	for _, r := range d.Runs {
		if seen[r.Seed] || !declared[r.Seed] {
			return false
		}
		seen[r.Seed] = true
		switch r.Status {
		case SuccessorRunComplete:
			complete++
		case SuccessorRunFailed:
			failed++
		case SuccessorRunIncomplete:
			incomplete++
		default:
			return false
		}
	}
	missing := expectedRuns - uint64(len(d.Runs))
	terminal := complete + failed
	if d.ExpectedRuns != expectedRuns || d.CompleteRuns != complete || d.FailedRuns != failed || d.IncompleteRuns != incomplete || d.MissingRuns != missing || d.TerminalRuns != terminal || d.ExpectedPairs != expectedPairs || d.CompletePairs != uint64(len(pairs)) || d.BlockedPairs != expectedPairs-uint64(len(pairs)) {
		return false
	}
	if d.Profile != mode7(d.Stage3.Bytes) {
		return false
	}
	allowed := d.Profile == "calls-only" && complete == expectedRuns && d.CompletePairs == d.ExpectedPairs
	for _, pair := range pairs {
		allowed = allowed && pair.Stable
	}
	wantOutcome := SuccessorReadinessPublicationBlocked
	if allowed {
		wantOutcome = SuccessorReadinessReady
	}
	return d.PublicationAllowed == allowed && d.PublicationBlocked == !allowed && d.Outcome == wantOutcome
}

func cloneEvidenceSlice7(in []ExactRetainedEvidence) []ExactRetainedEvidence {
	out := append([]ExactRetainedEvidence(nil), in...)
	for i := range out {
		out[i] = cloneEvidence7(out[i])
	}
	return out
}
func hashOptional7(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return hash7(b)
}
func validPartition7(p [][]string, nodes []string) bool {
	seen := map[string]bool{}
	for _, c := range p {
		if len(c) == 0 {
			return false
		}
		for _, n := range c {
			if seen[n] {
				return false
			}
			seen[n] = true
		}
	}
	if len(seen) != len(nodes) {
		return false
	}
	for _, n := range nodes {
		if !seen[n] {
			return false
		}
	}
	return true
}

func wireRational7(r ExactRational) CouplingRational {
	return CouplingRational{Numerator: r.Numerator, Denominator: r.Denominator}
}

func equalPairs7(a, b []SuccessorPair) bool {
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

func (a *SuccessorInstabilityArtifact) ResealForTest() {
	a.dto.Digest = digestInst7(a.dto)
	a.canonical = mustJSON7(a.dto)
}
func (a *SuccessorInstabilityArtifact) TestOnlyMutateStage3Length(n int) { a.dto.Stage3.ByteLength = n }
func (a *SuccessorInstabilityArtifact) TestOnlyMutateQualificationDigest(s string) {
	a.dto.Stage4Qualification.SHA256 = s
}
func (a *SuccessorInstabilityArtifact) TestOnlyMutateCompositionDigest(s string) {
	a.dto.Stage5Composition.SHA256 = s
}
func (a *SuccessorInstabilityArtifact) TestOnlyMutateReadinessDigest(s string) {
	a.dto.Stage5Readiness.SHA256 = s
}
func (a *SuccessorInstabilityArtifact) TestOnlyMutateParameters() {
	a.dto.Parameters = append(a.dto.Parameters, 'x')
}
func (a *SuccessorInstabilityArtifact) TestOnlyMutateRunBytes(i int) {
	if i < len(a.dto.Runs) {
		a.dto.Runs[i].Result = append(a.dto.Runs[i].Result, 'x')
	}
}
func (a *SuccessorInstabilityArtifact) TestOnlyReversePairs() {
	for i, j := 0, len(a.dto.Pairs)-1; i < j; i, j = i+1, j-1 {
		a.dto.Pairs[i], a.dto.Pairs[j] = a.dto.Pairs[j], a.dto.Pairs[i]
	}
}
func (a *SuccessorInstabilityArtifact) TestOnlyMutateAccounting() { a.dto.CompleteRuns++ }
func (a *SuccessorInstabilityArtifact) TestOnlyMutatePublication() {
	a.dto.PublicationAllowed = !a.dto.PublicationAllowed
}
func (a *SuccessorInstabilityArtifact) TestOnlyMutateLimits() { a.dto.Limits.MaxWork++ }

var _ = bytes.Equal
