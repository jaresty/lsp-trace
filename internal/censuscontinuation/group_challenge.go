package censuscontinuation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"unicode/utf8"
)

const (
	privateGroupChallengeSubmissionSchema = "lsp-trace.census-group-challenge-submission.private.v1"
	privateGroupChallengeArtifactSchema   = "lsp-trace.census-group-challenge.private.v1"
	privateGroupChallengeProfileID        = "census-group-challenge-private-v1"
	privateGroupChallengeProfileDomain    = "lsp-trace:census-group-challenge-profile:private:v1"
	privateGroupChallengeArtifactDomain   = "lsp-trace:census-group-challenge-artifact:private:v1"
	privateGroupChallengeRecordDomain     = "lsp-trace:census-group-challenge-record:private:v1"
	privateAlternativeChallengeDomain     = "lsp-trace:census-alternative-challenge-record:private:v1"
	privateGroupChallengeMaxBytes         = 1 << 20
	privateGroupChallengeMaxRefBytes      = 256
	privateGroupChallengeMaxGroups        = 1024
	privateGroupChallengeMaxAlternatives  = 4096
	privateGroupChallengeMaxCandidates    = 4096
	privateGroupChallengeMaxMemberships   = 4096
	privateGroupChallengeMaxWork          = 16_384
)

const privateGroupChallengeProfileDigestPin = "sha256:686743b48acb3bbc29ae9e66a83b24194deff07b42a50970968bbe66533c7b2e"

const (
	privateChallengeRetain     = "RETAIN"
	privateChallengeRevise     = "REVISE"
	privateChallengeSplit      = "SPLIT"
	privateChallengeWithdraw   = "WITHDRAW"
	privateChallengeUnresolved = "UNRESOLVED"
)

type privateGroupChallengeSubmissionWire struct {
	SchemaVersion         string                                        `json:"schema_version"`
	ProducerProcessRef    string                                        `json:"producer_process_ref"`
	ChallengerProcessRef  string                                        `json:"challenger_process_ref"`
	RecordRef             *string                                       `json:"record_ref"`
	GroupChallenges       []privateGroupChallengeSubmissionRecord       `json:"group_challenges"`
	AlternativeChallenges []privateAlternativeChallengeSubmissionRecord `json:"alternative_challenges"`
}
type privateGroupChallengeSubmissionRecord struct {
	TargetGroupID           string     `json:"target_group_id"`
	Verdict                 string     `json:"verdict"`
	ProposedCandidateGroups [][]string `json:"proposed_candidate_groups"`
}
type privateAlternativeChallengeSubmissionRecord struct {
	TargetAlternativeID string    `json:"target_alternative_id"`
	Verdict             string    `json:"verdict"`
	ProposedGroupRefs   *[]string `json:"proposed_group_refs"`
}
type privateGroupChallengeBounds struct {
	MaxGroups              int `json:"max_groups"`
	MaxAlternatives        int `json:"max_alternatives"`
	MaxCandidatesPerGroup  int `json:"max_candidates_per_group"`
	MaxProposalMemberships int `json:"max_proposal_memberships"`
	MaxProcessRefUTF8Bytes int `json:"max_process_ref_utf8_bytes"`
	MaxRecordRefUTF8Bytes  int `json:"max_record_ref_utf8_bytes"`
	MaxArtifactBytes       int `json:"max_artifact_bytes"`
	MaxWork                int `json:"max_work"`
}
type privateGroupChallengeRecord struct {
	RecordID                string     `json:"record_id"`
	TargetGroupID           string     `json:"target_group_id"`
	Verdict                 string     `json:"verdict"`
	ProposedCandidateGroups [][]string `json:"proposed_candidate_groups"`
}
type privateAlternativeChallengeRecord struct {
	RecordID            string    `json:"record_id"`
	TargetAlternativeID string    `json:"target_alternative_id"`
	Verdict             string    `json:"verdict"`
	ProposedGroupRefs   *[]string `json:"proposed_group_refs"`
}
type privateGroupChallengeAccounting struct {
	CandidateCount            int `json:"candidate_count"`
	GroupCount                int `json:"group_count"`
	AlternativeCount          int `json:"alternative_count"`
	GroupChallengeCount       int `json:"group_challenge_count"`
	AlternativeChallengeCount int `json:"alternative_challenge_count"`
	TotalChallengeCount       int `json:"total_challenge_count"`
}
type privateGroupChallengeArtifactWire struct {
	SchemaVersion               string                              `json:"schema_version"`
	ArtifactID                  string                              `json:"artifact_id"`
	Authority                   int                                 `json:"authority"`
	Accepted                    bool                                `json:"accepted"`
	Completeness                string                              `json:"completeness"`
	FeatureIdentity             string                              `json:"feature_identity"`
	AdjudicationStatus          string                              `json:"adjudication_status"`
	AutomaticProgressionAllowed bool                                `json:"automatic_progression_allowed"`
	GroupingMutated             bool                                `json:"grouping_mutated"`
	ChallengeProvenance         string                              `json:"challenge_provenance"`
	ChallengeAuthentication     string                              `json:"challenge_authentication"`
	ChallengeStatus             string                              `json:"challenge_status"`
	IndependenceStatus          string                              `json:"independence_status"`
	ProducerProcessRef          string                              `json:"producer_process_ref"`
	ChallengerProcessRef        string                              `json:"challenger_process_ref"`
	RecordRef                   *string                             `json:"record_ref"`
	GroupingSelector            string                              `json:"grouping_selector"`
	GroupingDigest              string                              `json:"grouping_digest"`
	GroupingByteLength          uint64                              `json:"grouping_byte_length"`
	GroupingArtifactID          string                              `json:"grouping_artifact_id"`
	ProfileID                   string                              `json:"profile_id"`
	ProfileDigest               string                              `json:"profile_digest"`
	Bounds                      privateGroupChallengeBounds         `json:"bounds"`
	GroupChallenges             []privateGroupChallengeRecord       `json:"group_challenges"`
	AlternativeChallenges       []privateAlternativeChallengeRecord `json:"alternative_challenges"`
	Accounting                  privateGroupChallengeAccounting     `json:"accounting"`
}
type privateGroupChallengeArtifact struct {
	wire privateGroupChallengeArtifactWire
}

func (a privateGroupChallengeArtifact) ID() string { return a.wire.ArtifactID }

func privateGroupChallengeFrozenBounds() privateGroupChallengeBounds {
	return privateGroupChallengeBounds{
		MaxGroups:              privateGroupChallengeMaxGroups,
		MaxAlternatives:        privateGroupChallengeMaxAlternatives,
		MaxCandidatesPerGroup:  privateGroupChallengeMaxCandidates,
		MaxProposalMemberships: privateGroupChallengeMaxMemberships,
		MaxProcessRefUTF8Bytes: privateGroupChallengeMaxRefBytes,
		MaxRecordRefUTF8Bytes:  privateGroupChallengeMaxRefBytes,
		MaxArtifactBytes:       privateGroupChallengeMaxBytes,
		MaxWork:                privateGroupChallengeMaxWork,
	}
}
func privateGroupChallengeProfileDigest() string { return privateGroupChallengeProfileDigestPin }
func privateGroupChallengeComputedProfileDigest() string {
	return identityJSON(privateGroupChallengeProfileDomain, privateGroupChallengeFrozenBounds(), func(*privateGroupChallengeBounds) {})
}
func privateChallengeVerdict(v string, split bool) bool {
	return v == privateChallengeRetain || v == privateChallengeRevise || (split && v == privateChallengeSplit) || v == privateChallengeWithdraw || v == privateChallengeUnresolved
}
func boundedChallengeRef(v string, max int) bool {
	return v != "" && utf8.ValidString(v) && len([]byte(v)) <= max
}

// Challenge work is one unit for each group challenge, alternative challenge,
// proposed candidate group, proposal candidate membership, and proposed group
// reference traversed by validation.
func validatePrivateGroupChallengeWork(groupChallenges int, proposedGroups [][][]string, alternativeChallenges int, proposedGroupRefs []*[]string, maxWork int) error {
	remaining := maxWork
	charge := func(units int) bool {
		if units < 0 || units > remaining {
			return false
		}
		remaining -= units
		return true
	}
	if !charge(groupChallenges) || !charge(alternativeChallenges) {
		return errors.New("private group challenge work limit")
	}
	for _, groups := range proposedGroups {
		if !charge(len(groups)) {
			return errors.New("private group challenge work limit")
		}
		for _, group := range groups {
			if !charge(len(group)) {
				return errors.New("private group challenge work limit")
			}
		}
	}
	for _, refs := range proposedGroupRefs {
		if refs != nil && !charge(len(*refs)) {
			return errors.New("private group challenge work limit")
		}
	}
	return nil
}

func validatePrivateGroupChallengeSubmissionWork(w privateGroupChallengeSubmissionWire, maxWork int) error {
	groups := make([][][]string, len(w.GroupChallenges))
	for i := range w.GroupChallenges {
		groups[i] = w.GroupChallenges[i].ProposedCandidateGroups
	}
	refs := make([]*[]string, len(w.AlternativeChallenges))
	for i := range w.AlternativeChallenges {
		refs[i] = w.AlternativeChallenges[i].ProposedGroupRefs
	}
	return validatePrivateGroupChallengeWork(len(w.GroupChallenges), groups, len(w.AlternativeChallenges), refs, maxWork)
}

func validatePrivateGroupChallengeArtifactWork(w privateGroupChallengeArtifactWire, maxWork int) error {
	groups := make([][][]string, len(w.GroupChallenges))
	for i := range w.GroupChallenges {
		groups[i] = w.GroupChallenges[i].ProposedCandidateGroups
	}
	refs := make([]*[]string, len(w.AlternativeChallenges))
	for i := range w.AlternativeChallenges {
		refs[i] = w.AlternativeChallenges[i].ProposedGroupRefs
	}
	return validatePrivateGroupChallengeWork(len(w.GroupChallenges), groups, len(w.AlternativeChallenges), refs, maxWork)
}

func parsePrivateGroupChallengeSubmission(raw []byte) (privateGroupChallengeSubmissionWire, error) {
	if len(raw) == 0 || len(raw) > privateGroupChallengeMaxBytes {
		return privateGroupChallengeSubmissionWire{}, errors.New("private group challenge submission byte limit")
	}
	var w privateGroupChallengeSubmissionWire
	if err := strictCanonical(raw, &w); err != nil {
		return w, err
	}
	b := privateGroupChallengeFrozenBounds()
	if w.SchemaVersion != privateGroupChallengeSubmissionSchema || !boundedChallengeRef(w.ProducerProcessRef, b.MaxProcessRefUTF8Bytes) || !boundedChallengeRef(w.ChallengerProcessRef, b.MaxProcessRefUTF8Bytes) || w.ProducerProcessRef == w.ChallengerProcessRef || w.GroupChallenges == nil || w.AlternativeChallenges == nil || len(w.GroupChallenges) > b.MaxGroups || len(w.AlternativeChallenges) > b.MaxAlternatives {
		return w, errors.New("private group challenge submission invalid")
	}
	if w.RecordRef != nil && !boundedChallengeRef(*w.RecordRef, b.MaxRecordRefUTF8Bytes) {
		return w, errors.New("private group challenge record ref invalid")
	}
	if err := validatePrivateGroupChallengeSubmissionWork(w, b.MaxWork); err != nil {
		return w, err
	}
	return w, nil
}

func validateProposedGroups(groups [][]string, target []string, verdict string, b privateGroupChallengeBounds) error {
	if groups == nil {
		return errors.New("private group challenge proposed groups null")
	}
	want := 0
	switch verdict {
	case privateChallengeRevise:
		want = 1
	case privateChallengeSplit:
		if len(groups) < 2 {
			return errors.New("private group challenge split proposal invalid")
		}
	default:
		if len(groups) != 0 {
			return errors.New("private group challenge unexpected proposal")
		}
		return nil
	}
	if want != 0 && len(groups) != want {
		return errors.New("private group challenge revise proposal invalid")
	}
	seen, total := map[string]bool{}, 0
	for groupIndex, group := range groups {
		if group == nil || len(group) == 0 || len(group) > b.MaxCandidatesPerGroup || (groupIndex > 0 && groups[groupIndex-1][0] >= group[0]) {
			return errors.New("private group challenge proposal shape")
		}
		for i, id := range group {
			if !validDigest(id) || (i > 0 && group[i-1] >= id) || seen[id] {
				return errors.New("private group challenge proposal canonicality")
			}
			seen[id] = true
			total++
		}
	}
	if total > b.MaxProposalMemberships || len(seen) != len(target) {
		return errors.New("private group challenge proposal coverage")
	}
	for _, id := range target {
		if !seen[id] {
			return errors.New("private group challenge proposal coverage")
		}
	}
	return nil
}

func (a privateGroupChallengeArtifact) Bytes() ([]byte, error) {
	if err := a.validate(); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(a.wire)
	if err != nil || len(raw) > privateGroupChallengeMaxBytes {
		return nil, errors.New("private group challenge artifact byte limit")
	}
	return raw, nil
}
func parsePrivateGroupChallengeArtifact(raw []byte) (privateGroupChallengeArtifact, error) {
	if len(raw) == 0 || len(raw) > privateGroupChallengeMaxBytes {
		return privateGroupChallengeArtifact{}, errors.New("private group challenge artifact byte limit")
	}
	var w privateGroupChallengeArtifactWire
	if err := strictCanonical(raw, &w); err != nil {
		return privateGroupChallengeArtifact{}, err
	}
	a := privateGroupChallengeArtifact{w}
	return a, a.validate()
}
func (a privateGroupChallengeArtifact) validate() error {
	w, b := a.wire, privateGroupChallengeFrozenBounds()
	if w.SchemaVersion != privateGroupChallengeArtifactSchema || privateGroupChallengeComputedProfileDigest() != privateGroupChallengeProfileDigestPin || !validDigest(w.ArtifactID) || w.Authority != 0 || w.Accepted || w.Completeness != CompletenessUnknown || w.FeatureIdentity != "UNRESOLVED" || w.AdjudicationStatus != "NOT_PERFORMED" || w.AutomaticProgressionAllowed || w.GroupingMutated || w.ChallengeProvenance != "CALLER_ASSERTED_DISTINCT_PROCESS_REFS" || w.ChallengeAuthentication != "UNAUTHENTICATED" || w.ChallengeStatus != "CALLER_ASSERTED_CHALLENGE_RETAINED" || w.IndependenceStatus != "NOT_ESTABLISHED" || !boundedChallengeRef(w.ProducerProcessRef, b.MaxProcessRefUTF8Bytes) || !boundedChallengeRef(w.ChallengerProcessRef, b.MaxProcessRefUTF8Bytes) || w.ProducerProcessRef == w.ChallengerProcessRef || (w.RecordRef != nil && !boundedChallengeRef(*w.RecordRef, b.MaxRecordRefUTF8Bytes)) || !validDigest(w.GroupingSelector) || w.GroupingDigest != w.GroupingSelector || w.GroupingByteLength == 0 || !validDigest(w.GroupingArtifactID) || w.ProfileID != privateGroupChallengeProfileID || w.ProfileDigest != privateGroupChallengeProfileDigest() || w.Bounds != b || w.GroupChallenges == nil || w.AlternativeChallenges == nil {
		return errors.New("private group challenge artifact integrity")
	}
	if identityJSON(privateGroupChallengeArtifactDomain, w, func(v *privateGroupChallengeArtifactWire) { v.ArtifactID = "" }) != w.ArtifactID {
		return errors.New("private group challenge artifact identity")
	}
	if len(w.GroupChallenges) > b.MaxGroups || len(w.AlternativeChallenges) > b.MaxAlternatives {
		return errors.New("private group challenge count limit")
	}
	if err := validatePrivateGroupChallengeArtifactWork(w, b.MaxWork); err != nil {
		return err
	}
	groupTargets := map[string]bool{}
	for i, r := range w.GroupChallenges {
		if !validDigest(r.RecordID) || !validDigest(r.TargetGroupID) || groupTargets[r.TargetGroupID] || (i > 0 && w.GroupChallenges[i-1].TargetGroupID >= r.TargetGroupID) || !privateChallengeVerdict(r.Verdict, true) || validateRetainedProposedGroups(r.ProposedCandidateGroups, r.Verdict, b) != nil || identityJSON(privateGroupChallengeRecordDomain, r, func(v *privateGroupChallengeRecord) { v.RecordID = "" }) != r.RecordID {
			return errors.New("private group challenge record invalid")
		}
		groupTargets[r.TargetGroupID] = true
	}
	alternativeTargets := map[string]bool{}
	for i, r := range w.AlternativeChallenges {
		if !validDigest(r.RecordID) || !validDigest(r.TargetAlternativeID) || alternativeTargets[r.TargetAlternativeID] || (i > 0 && w.AlternativeChallenges[i-1].TargetAlternativeID >= r.TargetAlternativeID) || !privateChallengeVerdict(r.Verdict, false) || (r.Verdict == privateChallengeRevise) != (r.ProposedGroupRefs != nil) || (r.ProposedGroupRefs != nil && (len(*r.ProposedGroupRefs) != 2 || (*r.ProposedGroupRefs)[0] >= (*r.ProposedGroupRefs)[1] || !validDigest((*r.ProposedGroupRefs)[0]) || !validDigest((*r.ProposedGroupRefs)[1]))) || identityJSON(privateAlternativeChallengeDomain, r, func(v *privateAlternativeChallengeRecord) { v.RecordID = "" }) != r.RecordID {
			return errors.New("private alternative challenge record invalid")
		}
		alternativeTargets[r.TargetAlternativeID] = true
	}
	if w.Accounting != (privateGroupChallengeAccounting{w.Accounting.CandidateCount, len(w.GroupChallenges), len(w.AlternativeChallenges), len(w.GroupChallenges), len(w.AlternativeChallenges), len(w.GroupChallenges) + len(w.AlternativeChallenges)}) || w.Accounting.CandidateCount <= 0 || w.Accounting.CandidateCount > privateGroupingMaxCandidates {
		return errors.New("private group challenge accounting")
	}
	return nil
}

// RetainPrivateGroupChallengeSelector retains a caller-asserted challenge to one exact immutable grouping.
func RetainPrivateGroupChallengeSelector(ctx context.Context, store Store, groupingSelector string, submission []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if store == nil || !validDigest(groupingSelector) {
		return "", errors.New("private group challenge input invalid")
	}
	in, err := parsePrivateGroupChallengeSubmission(submission)
	if err != nil {
		return "", err
	}
	groupingRaw, err := store.Get(ctx, groupingSelector)
	if err != nil || digestOf(groupingRaw) != groupingSelector {
		return "", errors.New("private group challenge grouping selector mismatch")
	}
	predecessor, err := parsePrivateGroupingArtifact(groupingRaw)
	if err != nil {
		return "", err
	}
	if len(in.GroupChallenges) != len(predecessor.wire.Groups) || len(in.AlternativeChallenges) != len(predecessor.wire.Alternatives) {
		return "", errors.New("private group challenge target set mismatch")
	}
	b := privateGroupChallengeFrozenBounds()
	groupByID := map[string]privateGroupingGroup{}
	semantic := map[string]bool{}
	for _, g := range predecessor.wire.Groups {
		groupByID[g.GroupID] = g
		semantic[g.GroupID] = g.Kind == privateGroupingSemantic
	}
	groups := make([]privateGroupChallengeRecord, len(in.GroupChallenges))
	seenGroups := map[string]bool{}
	for i, r := range in.GroupChallenges {
		g, ok := groupByID[r.TargetGroupID]
		if !ok || seenGroups[r.TargetGroupID] || (i > 0 && in.GroupChallenges[i-1].TargetGroupID >= r.TargetGroupID) || !privateChallengeVerdict(r.Verdict, true) || (g.Kind == privateGroupingSingleton && r.Verdict == privateChallengeSplit) || validateProposedGroups(r.ProposedCandidateGroups, g.CandidateIDs, r.Verdict, b) != nil {
			return "", errors.New("private group challenge group invalid")
		}
		seenGroups[r.TargetGroupID] = true
		row := privateGroupChallengeRecord{"", r.TargetGroupID, r.Verdict, cloneStringMatrix(r.ProposedCandidateGroups)}
		row.RecordID = identityJSON(privateGroupChallengeRecordDomain, row, func(v *privateGroupChallengeRecord) { v.RecordID = "" })
		groups[i] = row
	}
	alternativeByID := map[string]privateGroupingAlternative{}
	for _, alt := range predecessor.wire.Alternatives {
		alternativeByID[alt.AlternativeID] = alt
	}
	alternatives := make([]privateAlternativeChallengeRecord, len(in.AlternativeChallenges))
	seenAlternatives := map[string]bool{}
	for i, r := range in.AlternativeChallenges {
		_, ok := alternativeByID[r.TargetAlternativeID]
		validRefs := r.ProposedGroupRefs == nil
		if r.Verdict == privateChallengeRevise && r.ProposedGroupRefs != nil {
			refs := *r.ProposedGroupRefs
			validRefs = len(refs) == 2 && refs[0] < refs[1] && semantic[refs[0]] && semantic[refs[1]]
		}
		if !ok || seenAlternatives[r.TargetAlternativeID] || (i > 0 && in.AlternativeChallenges[i-1].TargetAlternativeID >= r.TargetAlternativeID) || !privateChallengeVerdict(r.Verdict, false) || !validRefs {
			return "", errors.New("private group challenge alternative invalid")
		}
		seenAlternatives[r.TargetAlternativeID] = true
		var refs *[]string
		if r.ProposedGroupRefs != nil {
			copyRefs := append([]string{}, (*r.ProposedGroupRefs)...)
			refs = &copyRefs
		}
		row := privateAlternativeChallengeRecord{"", r.TargetAlternativeID, r.Verdict, refs}
		row.RecordID = identityJSON(privateAlternativeChallengeDomain, row, func(v *privateAlternativeChallengeRecord) { v.RecordID = "" })
		alternatives[i] = row
	}
	w := privateGroupChallengeArtifactWire{SchemaVersion: privateGroupChallengeArtifactSchema, Completeness: CompletenessUnknown, FeatureIdentity: "UNRESOLVED", AdjudicationStatus: "NOT_PERFORMED", ChallengeProvenance: "CALLER_ASSERTED_DISTINCT_PROCESS_REFS", ChallengeAuthentication: "UNAUTHENTICATED", ChallengeStatus: "CALLER_ASSERTED_CHALLENGE_RETAINED", IndependenceStatus: "NOT_ESTABLISHED", ProducerProcessRef: in.ProducerProcessRef, ChallengerProcessRef: in.ChallengerProcessRef, RecordRef: cloneStringPtr(in.RecordRef), GroupingSelector: groupingSelector, GroupingDigest: groupingSelector, GroupingByteLength: uint64(len(groupingRaw)), GroupingArtifactID: predecessor.ID(), ProfileID: privateGroupChallengeProfileID, ProfileDigest: privateGroupChallengeProfileDigest(), Bounds: b, GroupChallenges: groups, AlternativeChallenges: alternatives, Accounting: privateGroupChallengeAccounting{predecessor.wire.Accounting.CandidateCount, len(groups), len(alternatives), len(groups), len(alternatives), len(groups) + len(alternatives)}}
	w.ArtifactID = identityJSON(privateGroupChallengeArtifactDomain, w, func(v *privateGroupChallengeArtifactWire) { v.ArtifactID = "" })
	artifact := privateGroupChallengeArtifact{w}
	raw, err := artifact.Bytes()
	if err != nil {
		return "", err
	}
	parsed, err := parsePrivateGroupChallengeArtifact(raw)
	if err != nil || parsed.ID() != artifact.ID() {
		return "", errors.New("private group challenge self validation")
	}
	again, err := store.Get(ctx, groupingSelector)
	if err != nil || !bytes.Equal(again, groupingRaw) {
		return "", errors.New("private group challenge predecessor changed")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	selector, err := store.Put(ctx, raw)
	if err != nil {
		return "", err
	}
	if selector != digestOf(raw) {
		return "", errors.New("private group challenge store selector mismatch")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	readback, err := store.Get(ctx, selector)
	if err != nil {
		return "", err
	}
	readArtifact, err := parsePrivateGroupChallengeArtifact(readback)
	if err != nil || !bytes.Equal(readback, raw) || digestOf(readback) != selector || readArtifact.ID() != artifact.ID() || readArtifact.wire.GroupingSelector != groupingSelector || readArtifact.wire.GroupingDigest != groupingSelector || readArtifact.wire.GroupingArtifactID != predecessor.ID() {
		return "", errors.New("private group challenge readback mismatch")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return selector, nil
}
func validateRetainedProposedGroups(groups [][]string, verdict string, b privateGroupChallengeBounds) error {
	if groups == nil {
		return errors.New("private group challenge proposed groups null")
	}
	switch verdict {
	case privateChallengeRevise:
		if len(groups) != 1 {
			return errors.New("private group challenge revise proposal invalid")
		}
	case privateChallengeSplit:
		if len(groups) < 2 {
			return errors.New("private group challenge split proposal invalid")
		}
	default:
		if len(groups) != 0 {
			return errors.New("private group challenge unexpected proposal")
		}
		return nil
	}
	seen, total := map[string]bool{}, 0
	for groupIndex, group := range groups {
		if group == nil || len(group) == 0 || len(group) > b.MaxCandidatesPerGroup || (groupIndex > 0 && groups[groupIndex-1][0] >= group[0]) {
			return errors.New("private group challenge proposal shape")
		}
		for i, id := range group {
			if !validDigest(id) || (i > 0 && group[i-1] >= id) || seen[id] {
				return errors.New("private group challenge proposal canonicality")
			}
			seen[id] = true
			total++
		}
	}
	if total > b.MaxProposalMemberships {
		return errors.New("private group challenge proposal membership limit")
	}
	return nil
}

func cloneStringMatrix(in [][]string) [][]string {
	out := make([][]string, len(in))
	for i := range in {
		out[i] = append([]string{}, in[i]...)
	}
	return out
}
func cloneStringPtr(in *string) *string {
	if in == nil {
		return nil
	}
	v := *in
	return &v
}
