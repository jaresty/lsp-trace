package programcrepresentative

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"

	"lsp-trace/internal/graph"
)

// RepresentativeComposition is the admitted, production-owned stage-5 carrier.
type RepresentativeComposition struct {
	Family, Version                      string
	InputID, InputDigest                 string
	InputLength                          int
	PolicyID, PolicyDigest               string
	PolicyLength                         int
	CompositionID, Digest                string
	QualificationID, QualificationDigest string
	QualificationLength                  int
	Occurrences                          []RepresentativeEvidence
	Accounting                           RepresentativeAccounting
	ModelJudgment, ModelAdvice           string
}

type RepresentativeGrouping struct {
	Family, Version, Algorithm, Profile, NumericContract string
	LimitsDigest, CompositionDigest                      string
	LimitsLength, CompositionLength                      int
	Seed                                                 uint64
	Partition                                            [][]string
	SelectionContexts                                    []RepresentativeSelectionContext
	Digest                                               string
}

// RepresentativeSelectionContext is the immutable Stage6 projection of one
// historical seed context. Stage6 requires exactly one context for each
// community; multiple historical seeds must remain separate communities or the
// grouping is rejected rather than silently collapsed.
type RepresentativeSelectionContext struct {
	Identity, ConstituentIdentity, ExecutionBundleID, SeedLabel, SeedAt string
	ConstituentOrdinal                                                  int
	CommunityMembers, PreparedTargets, ReachedNodes                     []string
	RelationIDs, OccurrenceIDs                                          []string
}

type RepresentativeEvidence struct {
	Identity, RelationID, Kind, From, To, Disposition string
	SourcePredecessor, Custody                        string
	Ordinal                                           int
	CallSite                                          graph.Range
}

type RepresentativeAccounting struct {
	Input, Terminal                                    int
	Included, Excluded, Downweighted, Capped           int
	Normalized, SeparateLayer                          int
	EligibleCalls, IneligibleCalls, NonCalls           int
	SelfLoops, RepeatedCalls, Duplicates               int
	Selected, NoCalls, CallsIneligible, EmptyCommunity int
	Truncated                                          bool
}

type RepresentativeQualification struct {
	Family, CompositionDigest, EvidenceDigest, Trust string
	ValidFrom, ValidThrough                          uint64
}

type RepresentativeLimits struct {
	MaxCommunities, MaxMembers, MaxOccurrences, MaxCandidates int
	MaxCanonicalBytes, MaxStringBytes, MaxWork                int
}

type RepresentativePredecessor struct {
	OccurrenceID, RelationID, CallerID, TargetID string
	CallSite                                     graph.Range
}

type RepresentativeRecord struct {
	Outcome              string
	SelectedNode         string
	Distance             int
	SCCMembers           []string
	IncomingPredecessors []RepresentativePredecessor
}

type RepresentativeResult struct {
	Digest                string
	outcome, selectedNode string
	canonical             []byte
	communities           [][]string
	evidence              []RepresentativeEvidence
	representatives       []RepresentativeRecord
	accounting            RepresentativeAccounting
	limits                RepresentativeLimits
}

func (r RepresentativeResult) Outcome() string         { return r.outcome }
func (r RepresentativeResult) SelectedNode() string    { return r.selectedNode }
func (r RepresentativeResult) CanonicalBytes() []byte  { return append([]byte(nil), r.canonical...) }
func (r RepresentativeResult) Communities() [][]string { return cloneCommunities(r.communities) }
func (r RepresentativeResult) VisibleEvidence() []RepresentativeEvidence {
	return append([]RepresentativeEvidence(nil), r.evidence...)
}
func (r RepresentativeResult) Representatives() []RepresentativeRecord {
	return cloneRecords(r.representatives)
}
func (r RepresentativeResult) Accounting() RepresentativeAccounting { return r.accounting }

func computeQualifiedRepresentativeResult(c RepresentativeComposition, g RepresentativeGrouping, q RepresentativeQualification, verify func(RepresentativeQualification) bool, limits RepresentativeLimits) (RepresentativeResult, error) {
	zero := RepresentativeResult{}
	if verify == nil || !verify(q) {
		return zero, fmt.Errorf("representative qualification refused")
	}
	if err := validateLimits(limits); err != nil {
		return zero, err
	}
	if err := validateComposition(c, limits); err != nil {
		return zero, err
	}
	if err := validateGrouping(g, c, limits); err != nil {
		return zero, err
	}
	if q.CompositionDigest != c.Digest || q.EvidenceDigest != g.Digest {
		return zero, fmt.Errorf("qualification binding mismatch")
	}

	evidence := append([]RepresentativeEvidence(nil), c.Occurrences...)
	a := c.Accounting
	a.EligibleCalls, a.IneligibleCalls, a.NonCalls, a.SelfLoops, a.RepeatedCalls = 0, 0, 0, 0, 0
	a.Selected, a.NoCalls, a.CallsIneligible, a.EmptyCommunity = 0, 0, 0, 0
	calls := make([]RepresentativeEvidence, 0)
	allCallCount := 0
	seenPair := map[string]int{}
	for _, e := range evidence {
		if e.Kind != "CALLS" {
			if err := checkedIncrement(&a.NonCalls); err != nil {
				return zero, err
			}
			continue
		}
		if err := checkedIncrement(&allCallCount); err != nil {
			return zero, err
		}
		if e.Disposition != "INCLUDED" || e.From == e.To {
			if err := checkedIncrement(&a.IneligibleCalls); err != nil {
				return zero, err
			}
			if e.From == e.To {
				if err := checkedIncrement(&a.SelfLoops); err != nil {
					return zero, err
				}
			}
			continue
		}
		if err := checkedIncrement(&a.EligibleCalls); err != nil {
			return zero, err
		}
		key := e.From + "\x00" + e.To
		count, err := checkedAdd(seenPair[key], 1)
		if err != nil {
			return zero, err
		}
		seenPair[key] = count
		if count > 1 {
			if err := checkedIncrement(&a.RepeatedCalls); err != nil {
				return zero, err
			}
		}
		calls = append(calls, e)
	}

	communities := cloneCommunities(g.Partition)
	records := make([]RepresentativeRecord, 0, len(communities))
	for i := range communities {
		context := g.SelectionContexts[i]
		r := RepresentativeRecord{}
		switch {
		case allCallCount == 0:
			r.Outcome = "NO_CALLS_REPRESENTATIVE"
			if err := checkedIncrement(&a.NoCalls); err != nil {
				return zero, err
			}
		default:
			var ok bool
			r, ok = selectRecord(context, calls)
			if !ok {
				r = RepresentativeRecord{Outcome: "CALLS_EVIDENCE_INELIGIBLE"}
				if err := checkedIncrement(&a.CallsIneligible); err != nil {
					return zero, err
				}
			} else if err := checkedIncrement(&a.Selected); err != nil {
				return zero, err
			}
		}
		records = append(records, r)
	}
	outcome, selected := "", ""
	if len(records) == 0 {
		outcome = "EMPTY_COMMUNITY"
		a.EmptyCommunity = 1
	} else if len(records) == 1 {
		outcome, selected = records[0].Outcome, records[0].SelectedNode
	} else {
		outcome = aggregateOutcome(records)
	}

	body := representativeCanonical{Composition: c, Grouping: g, Qualification: q, Limits: limits, Outcome: outcome, SelectedNode: selected, Communities: communities, VisibleEvidence: evidence, Representatives: records, Accounting: a}
	raw, err := canonicalJSON(body)
	if err != nil {
		return zero, err
	}
	if len(raw) > limits.MaxCanonicalBytes {
		return zero, fmt.Errorf("canonical byte limit exceeded")
	}
	return RepresentativeResult{Digest: digestBytes(raw), outcome: outcome, selectedNode: selected, canonical: raw, communities: communities, evidence: evidence, representatives: records, accounting: a, limits: limits}, nil
}

func replayQualifiedRepresentativeResult(c RepresentativeComposition, g RepresentativeGrouping, q RepresentativeQualification, verify func(RepresentativeQualification) bool, limits RepresentativeLimits, prior RepresentativeResult) (RepresentativeResult, error) {
	got, err := computeQualifiedRepresentativeResult(c, g, q, verify, limits)
	if err != nil {
		return RepresentativeResult{}, err
	}
	if prior.Digest == "" || prior.Digest != digestBytes(prior.canonical) || prior.limits != limits || !bytes.Equal(got.canonical, prior.canonical) || got.Digest != prior.Digest {
		return RepresentativeResult{}, fmt.Errorf("representative replay mismatch")
	}
	return got, nil
}

type representativeCanonical struct {
	Composition           RepresentativeComposition
	Grouping              RepresentativeGrouping
	Qualification         RepresentativeQualification
	Limits                RepresentativeLimits
	Outcome, SelectedNode string
	Communities           [][]string
	VisibleEvidence       []RepresentativeEvidence
	Representatives       []RepresentativeRecord
	Accounting            RepresentativeAccounting
}

func validateComposition(c RepresentativeComposition, limits RepresentativeLimits) error {
	if c.Family == "" || c.Version == "" || c.InputID == "" || c.PolicyID == "" || c.CompositionID == "" || c.QualificationID == "" || c.InputLength < 0 || c.PolicyLength < 0 || c.QualificationLength < 0 {
		return fmt.Errorf("incomplete composition")
	}
	if len(c.Occurrences) > limits.MaxOccurrences {
		return fmt.Errorf("occurrence limit exceeded")
	}
	if c.InputLength > limits.MaxCanonicalBytes || c.PolicyLength > limits.MaxCanonicalBytes || c.QualificationLength > limits.MaxCanonicalBytes {
		return fmt.Errorf("declared byte length limit exceeded")
	}
	if err := validateCompositionStrings(c, limits.MaxStringBytes); err != nil {
		return err
	}
	copy := c
	copy.Digest = ""
	raw, err := canonicalJSON(copy)
	if err != nil {
		return err
	}
	if c.Digest != digestBytes(raw) {
		return fmt.Errorf("composition digest mismatch")
	}
	if err := validateAccounting(c.Accounting, c.Occurrences); err != nil {
		return err
	}
	ids := map[string]bool{}
	for _, e := range c.Occurrences {
		if e.Identity == "" || e.Kind == "" || e.From == "" || e.To == "" || e.Disposition == "" || e.Ordinal < 0 || ids[e.Identity] {
			return fmt.Errorf("invalid occurrence")
		}
		ids[e.Identity] = true
		if e.Kind == "CALLS" && e.RelationID == "" {
			return fmt.Errorf("CALLS relation required")
		}
	}
	return nil
}

func validateCompositionStrings(c RepresentativeComposition, max int) error {
	strings := []string{c.Family, c.Version, c.InputID, c.InputDigest, c.PolicyID, c.PolicyDigest, c.CompositionID, c.Digest, c.QualificationID, c.QualificationDigest, c.ModelJudgment, c.ModelAdvice}
	for _, e := range c.Occurrences {
		strings = append(strings, e.Identity, e.RelationID, e.Kind, e.From, e.To, e.Disposition, e.SourcePredecessor, e.Custody)
	}
	for _, value := range strings {
		if len(value) > max {
			return fmt.Errorf("string byte limit exceeded")
		}
	}
	return nil
}

func validateGrouping(g RepresentativeGrouping, c RepresentativeComposition, l RepresentativeLimits) error {
	if g.Family == "" || g.Version == "" || g.Algorithm == "" || g.Profile == "" || g.NumericContract == "" || g.LimitsDigest == "" || g.LimitsLength < 0 || g.CompositionLength < 0 || g.CompositionDigest != c.Digest {
		return fmt.Errorf("invalid grouping binding")
	}
	copy := g
	copy.Digest = ""
	raw, err := canonicalJSON(copy)
	if err != nil {
		return err
	}
	if g.Digest != digestBytes(raw) {
		return fmt.Errorf("grouping digest mismatch")
	}
	if len(g.Partition) > l.MaxCommunities {
		return fmt.Errorf("community limit exceeded")
	}
	for _, value := range []string{g.Family, g.Version, g.Algorithm, g.Profile, g.NumericContract, g.LimitsDigest, g.CompositionDigest, g.Digest} {
		if len(value) > l.MaxStringBytes {
			return fmt.Errorf("string byte limit exceeded")
		}
	}
	if len(g.SelectionContexts) != len(g.Partition) {
		return fmt.Errorf("exactly one selection context per community required")
	}
	seenCommunities := map[string]bool{}
	seenMembers := map[string]bool{}
	seenContexts := map[string]bool{}
	members := 0
	previousCommunity := ""
	for communityIndex, community := range g.Partition {
		if len(community) == 0 {
			return fmt.Errorf("empty partition community")
		}
		if len(community) > l.MaxCandidates {
			return fmt.Errorf("candidate limit exceeded")
		}
		for i, member := range community {
			if member == "" || len(member) > l.MaxStringBytes || (i > 0 && community[i-1] >= member) {
				return fmt.Errorf("noncanonical partition")
			}
			if seenMembers[member] {
				return fmt.Errorf("partition member repeated across communities")
			}
			seenMembers[member] = true
			var err error
			members, err = checkedAdd(members, 1)
			if err != nil {
				return err
			}
		}
		context := g.SelectionContexts[communityIndex]
		if err := validateSelectionContext(context, community, c, l); err != nil {
			return err
		}
		if seenContexts[context.Identity] {
			return fmt.Errorf("duplicate selection context identity")
		}
		seenContexts[context.Identity] = true
		key := string(mustJSON(community))
		if previousCommunity != "" && previousCommunity >= key {
			return fmt.Errorf("noncanonical community order")
		}
		previousCommunity = key
		if seenCommunities[key] {
			return fmt.Errorf("duplicate community")
		}
		seenCommunities[key] = true
	}
	if members > l.MaxMembers {
		return fmt.Errorf("member limit exceeded")
	}
	work, err := checkedAdd(len(c.Occurrences), members)
	if err != nil {
		return err
	}
	if work > l.MaxWork {
		return fmt.Errorf("representative work limit exceeded")
	}
	return nil
}

func validateAccounting(a RepresentativeAccounting, occurrences []RepresentativeEvidence) error {
	values := []int{a.Input, a.Terminal, a.Included, a.Excluded, a.Downweighted, a.Capped, a.Normalized, a.SeparateLayer, a.EligibleCalls, a.IneligibleCalls, a.NonCalls, a.SelfLoops, a.RepeatedCalls, a.Duplicates, a.Selected, a.NoCalls, a.CallsIneligible, a.EmptyCommunity}
	for _, value := range values {
		if value < 0 {
			return fmt.Errorf("negative composition accounting")
		}
	}
	if a.EligibleCalls != 0 || a.IneligibleCalls != 0 || a.NonCalls != 0 || a.SelfLoops != 0 || a.RepeatedCalls != 0 || a.Duplicates != 0 || a.Selected != 0 || a.NoCalls != 0 || a.CallsIneligible != 0 || a.EmptyCommunity != 0 {
		return fmt.Errorf("prepopulated derived composition accounting")
	}
	total := 0
	for _, value := range []int{a.Included, a.Excluded, a.Downweighted, a.Capped, a.Normalized, a.SeparateLayer} {
		var err error
		total, err = checkedAdd(total, value)
		if err != nil {
			return err
		}
	}
	actual := map[string]int{}
	for _, occurrence := range occurrences {
		switch occurrence.Disposition {
		case "INCLUDED", "EXCLUDED", "DOWNWEIGHTED", "CAPPED", "NORMALIZED", "SEPARATE_LAYER":
			next, err := checkedAdd(actual[occurrence.Disposition], 1)
			if err != nil {
				return err
			}
			actual[occurrence.Disposition] = next
		default:
			return fmt.Errorf("unknown occurrence disposition")
		}
	}
	if a.Input != len(occurrences) || a.Terminal != a.Input || total != a.Terminal || a.Included != actual["INCLUDED"] || a.Excluded != actual["EXCLUDED"] || a.Downweighted != actual["DOWNWEIGHTED"] || a.Capped != actual["CAPPED"] || a.Normalized != actual["NORMALIZED"] || a.SeparateLayer != actual["SEPARATE_LAYER"] || a.Truncated {
		return fmt.Errorf("unbalanced composition accounting")
	}
	return nil
}

func validateLimits(l RepresentativeLimits) error {
	if l.MaxCommunities <= 0 || l.MaxMembers <= 0 || l.MaxOccurrences <= 0 || l.MaxCandidates <= 0 || l.MaxCanonicalBytes <= 0 || l.MaxStringBytes <= 0 || l.MaxWork <= 0 {
		return fmt.Errorf("invalid representative limits")
	}
	return nil
}

func validateSelectionContext(x RepresentativeSelectionContext, community []string, c RepresentativeComposition, l RepresentativeLimits) error {
	if x.Identity == "" || x.ConstituentIdentity == "" || x.ExecutionBundleID == "" || x.SeedLabel == "" || x.SeedAt == "" || x.ConstituentOrdinal < 0 || !equalStrings(x.CommunityMembers, community) {
		return fmt.Errorf("invalid selection context identity or community")
	}
	for _, value := range []string{x.Identity, x.ConstituentIdentity, x.ExecutionBundleID, x.SeedLabel, x.SeedAt} {
		if len(value) > l.MaxStringBytes {
			return fmt.Errorf("string byte limit exceeded")
		}
	}
	for _, values := range [][]string{x.CommunityMembers, x.PreparedTargets, x.ReachedNodes, x.RelationIDs, x.OccurrenceIDs} {
		if !strictSortedNonempty(values, l.MaxStringBytes) {
			return fmt.Errorf("noncanonical selection context")
		}
	}
	selectedRelations := make(map[string]bool, len(x.RelationIDs))
	for _, id := range x.RelationIDs {
		selectedRelations[id] = true
	}
	// Historical selection admits a relation atomically: every eligible CALLS
	// occurrence belonging to that relation participates. Require the retained
	// occurrence inventory to be the exact closure, not an independently
	// selectable subset.
	closure := make([]string, 0, len(x.OccurrenceIDs))
	seenRelation := make(map[string]bool, len(x.RelationIDs))
	for _, e := range c.Occurrences {
		if e.Kind == "CALLS" && selectedRelations[e.RelationID] {
			closure = append(closure, e.Identity)
			seenRelation[e.RelationID] = true
		}
	}
	sort.Strings(closure)
	if !equalStrings(closure, x.OccurrenceIDs) {
		return fmt.Errorf("selection context relation occurrence closure mismatch")
	}
	for _, id := range x.RelationIDs {
		if !seenRelation[id] {
			return fmt.Errorf("selection context relation has no admitted CALLS occurrence")
		}
	}
	return nil
}

func selectRecord(context RepresentativeSelectionContext, calls []RepresentativeEvidence) (RepresentativeRecord, bool) {
	active, prepared, reached := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, n := range context.PreparedTargets {
		active[n], prepared[n] = true, true
	}
	for _, n := range context.ReachedNodes {
		active[n], reached[n] = true, true
	}
	relations, occurrences := map[string]bool{}, map[string]bool{}
	for _, id := range context.RelationIDs {
		relations[id] = true
	}
	for _, id := range context.OccurrenceIDs {
		occurrences[id] = true
	}
	adj := map[string][]string{}
	eligibleCalls := make([]RepresentativeEvidence, 0)
	for _, e := range calls {
		if !relations[e.RelationID] || !occurrences[e.Identity] || !active[e.From] || !active[e.To] {
			continue
		}
		adj[e.From] = append(adj[e.From], e.To)
		eligibleCalls = append(eligibleCalls, e)
	}
	for n := range adj {
		sort.Strings(adj[n])
	}
	comp, groups := scc(active, adj)
	roots := map[int]bool{}
	for n := range prepared {
		roots[comp[n]] = true
	}
	if len(roots) == 0 {
		return RepresentativeRecord{}, false
	}
	cadj := map[int]map[int]bool{}
	for from, tos := range adj {
		for _, to := range tos {
			a, b := comp[from], comp[to]
			if a != b {
				if cadj[a] == nil {
					cadj[a] = map[int]bool{}
				}
				cadj[a][b] = true
			}
		}
	}
	dist, queue := map[int]int{}, []int{}
	for root := range roots {
		dist[root], queue = 0, append(queue, root)
	}
	sort.Ints(queue)
	for len(queue) > 0 {
		x := queue[0]
		queue = queue[1:]
		next := []int{}
		for y := range cadj[x] {
			next = append(next, y)
		}
		sort.Ints(next)
		for _, y := range next {
			if _, ok := dist[y]; !ok {
				dist[y], queue = dist[x]+1, append(queue, y)
			}
		}
	}
	best, bestD := "", int(^uint(0)>>1)
	for _, n := range context.CommunityMembers {
		if !reached[n] {
			continue
		}
		d, ok := dist[comp[n]]
		if ok && (d < bestD || d == bestD && (best == "" || n < best)) {
			best, bestD = n, d
		}
	}
	if best == "" {
		return RepresentativeRecord{}, false
	}
	pred := []RepresentativePredecessor{}
	for _, e := range eligibleCalls {
		if e.To == best && e.From != best {
			pred = append(pred, RepresentativePredecessor{e.Identity, e.RelationID, e.From, e.To, e.CallSite})
		}
	}
	sort.Slice(pred, func(i, j int) bool {
		if pred[i].RelationID != pred[j].RelationID {
			return pred[i].RelationID < pred[j].RelationID
		}
		return pred[i].OccurrenceID < pred[j].OccurrenceID
	})
	return RepresentativeRecord{Outcome: "SELECTED", SelectedNode: best, Distance: bestD, SCCMembers: append([]string(nil), groups[comp[best]]...), IncomingPredecessors: pred}, true
}

func aggregateOutcome(records []RepresentativeRecord) string {
	outcome := records[0].Outcome
	for _, record := range records[1:] {
		if record.Outcome != outcome {
			return "MIXED_COMMUNITY_OUTCOMES"
		}
	}
	return outcome
}

func checkedIncrement(value *int) error {
	next, err := checkedAdd(*value, 1)
	if err == nil {
		*value = next
	}
	return err
}
func checkedAdd(a, b int) (int, error) {
	max := int(^uint(0) >> 1)
	if a < 0 || b < 0 || a > max-b {
		return 0, fmt.Errorf("representative accounting overflow")
	}
	return a + b, nil
}
func strictSortedNonempty(values []string, max int) bool {
	for i, value := range values {
		if value == "" || len(value) > max || i > 0 && values[i-1] >= value {
			return false
		}
	}
	return true
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

func canonicalJSON(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}
func mustJSON(v any) []byte         { b, _ := json.Marshal(v); return b }
func digestBytes(raw []byte) string { sum := sha256.Sum256(raw); return fmt.Sprintf("sha256:%x", sum) }
func cloneCommunities(in [][]string) [][]string {
	out := make([][]string, len(in))
	for i := range in {
		out[i] = append([]string(nil), in[i]...)
	}
	return out
}
func cloneRecords(in []RepresentativeRecord) []RepresentativeRecord {
	out := make([]RepresentativeRecord, len(in))
	copy(out, in)
	for i := range out {
		out[i].SCCMembers = append([]string(nil), in[i].SCCMembers...)
		out[i].IncomingPredecessors = append([]RepresentativePredecessor(nil), in[i].IncomingPredecessors...)
	}
	return out
}
