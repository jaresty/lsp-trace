package programc

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"sort"

	"lsp-trace/internal/schema"
)

const InstabilityVersion = "lsp-trace.community-instability.v1"
const InstabilityPolicyVersion = "program-c-a-08-instability/v1"
const InstabilityClaimCeiling = "STABLE means only that the declared metrics met the declared thresholds for the exact bound graph, projection, implementation, parameters, seeds, and completed runs. It is label-independent, structural, and not a universal stability or semantic-community claim."
const InstabilityStructuralCeiling = "Structural output does not establish feature identity, ownership, architecture, runtime execution, whole-source completeness, producer authentication, permission, production authority, or stability beyond the exact compared runs."

const instabilityMaxRuns = 300
const instabilityMaxPairs = 44850
const instabilityMaxMatchingWork = 1000000

type InstabilityPolicySpec struct {
	Version         string `json:"version"`
	Matching        string `json:"matching"`
	TieBreak        string `json:"tie_break"`
	PairOrder       string `json:"pair_order"`
	MissingNodes    string `json:"missing_nodes"`
	Arithmetic      string `json:"arithmetic"`
	MaxRuns         int    `json:"max_runs"`
	MaxPairs        int    `json:"max_pairs"`
	MaxMatchingWork int    `json:"max_matching_work"`
}

var InstabilityPolicy = InstabilityPolicySpec{
	Version:      InstabilityPolicyVersion,
	Matching:     "maximum-total-exact-jaccard-one-to-one-with-explicit-unmatched-sentinels/v1",
	TieBreak:     "lexicographic-left-member-set-then-right-member-set/v1",
	PairOrder:    "descending-score-then-left-member-set-then-right-member-set/v1",
	MissingNodes: "explicit-singleton-missing-side-bins/v1",
	Arithmetic:   "Go-binary64-log2-canonical-node-order/v1",
	MaxRuns:      instabilityMaxRuns, MaxPairs: instabilityMaxPairs, MaxMatchingWork: instabilityMaxMatchingWork,
}

func InstabilityPolicyBytes() []byte { return canonicalInstability(InstabilityPolicy) }
func InstabilityPolicyDigest() string {
	return instabilityHash(InstabilityVersion+":policy", InstabilityPolicy)
}

type InstabilityIdentity struct {
	AdmittedGraphSHA256       string `json:"admitted_graph_sha256"`
	ProjectionSHA256          string `json:"projection_sha256"`
	ProjectionPolicyID        string `json:"projection_policy_id"`
	AlgorithmName             string `json:"algorithm_name"`
	AlgorithmVersion          string `json:"algorithm_version"`
	ParametersCanonicalSHA256 string `json:"parameters_canonical_sha256"`
	ResourcePolicySHA256      string `json:"resource_policy_sha256"`
}

type InstabilityThresholds struct {
	NodeReassignmentRateMax       float64 `json:"node_reassignment_rate_max"`
	UnmatchedCommunityRateMax     float64 `json:"unmatched_community_rate_max"`
	VariationOfInformationBitsMax float64 `json:"variation_of_information_bits_max"`
	PairwiseJaccardMinimumMin     float64 `json:"pairwise_jaccard_minimum_min"`
}

var exactInstabilityThresholds = InstabilityThresholds{0.05, 0.05, 0.10, 0.80}

type InstabilityRun struct {
	RunID             string
	Seed              uint64
	Status            string // COMPLETE, FAILED, or INCOMPLETE.
	Community         Outcome
	CommunityArtifact []byte
	BoundaryArtifact  []byte
	BoundaryRequest   BoundaryRequest
}

type InstabilityRequest struct {
	Identity      InstabilityIdentity
	DeclaredSeeds []uint64
	Runs          []InstabilityRun
}

type InstabilityAccounting struct {
	DeclaredSeedCount  int `json:"declared_seed_count"`
	DeclaredRunCount   int `json:"declared_run_count"`
	CompletedRunCount  int `json:"completed_run_count"`
	FailedRunCount     int `json:"failed_run_count"`
	IncompleteRunCount int `json:"incomplete_run_count"`
	ExpectedPairCount  int `json:"expected_pair_count"`
	CompletedPairCount int `json:"completed_pair_count"`
	BlockedPairCount   int `json:"blocked_pair_count"`
}

type InstabilityPair struct {
	LeftRunID                             string  `json:"left_run_id"`
	RightRunID                            string  `json:"right_run_id"`
	LeftCommunityArtifactSHA256           string  `json:"left_community_artifact_sha256,omitempty"`
	RightCommunityArtifactSHA256          string  `json:"right_community_artifact_sha256,omitempty"`
	Outcome                               string  `json:"outcome"`
	NodeReassignmentNumerator             int     `json:"node_reassignment_numerator"`
	NodeReassignmentDenominator           int     `json:"node_reassignment_denominator"`
	UnmatchedCommunityNumerator           int     `json:"unmatched_community_numerator"`
	UnmatchedCommunityDenominator         int     `json:"unmatched_community_denominator"`
	VariationOfInformationBits            float64 `json:"variation_of_information_bits"`
	VariationOfInformationNodeDenominator int     `json:"variation_of_information_node_denominator"`
	PairwiseJaccardMinimum                float64 `json:"pairwise_jaccard_minimum"`
	PairwiseJaccardMedian                 float64 `json:"pairwise_jaccard_median"`
	PairwiseJaccardMean                   float64 `json:"pairwise_jaccard_mean"`
	MatchedPairDenominator                int     `json:"matched_pair_denominator"`
	Reason                                string  `json:"reason,omitempty"`
}

type InstabilityArtifact struct {
	SchemaVersion          string                `json:"schema_version"`
	ComparisonIdentity     InstabilityIdentity   `json:"comparison_identity"`
	Outcome                string                `json:"outcome"`
	Thresholds             InstabilityThresholds `json:"thresholds"`
	PolicyVersion          string                `json:"policy_version"`
	Policy                 InstabilityPolicySpec `json:"policy"`
	PolicySHA256           string                `json:"policy_sha256"`
	RunAccounting          InstabilityAccounting `json:"run_accounting"`
	PairwiseComparisons    []InstabilityPair     `json:"pairwise_comparisons"`
	ClaimCeiling           string                `json:"claim_ceiling"`
	StructuralClaimCeiling string                `json:"structural_claim_ceiling"`
}

func canonicalInstability(v any) []byte {
	b, _ := json.Marshal(v)
	var x any
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	_ = d.Decode(&x)
	b, _ = json.Marshal(x)
	return b
}
func instabilityHash(domain string, v any) string {
	b := canonicalInstability(v)
	s := sha256.Sum256(append(append([]byte(domain), 0), b...))
	return "sha256:" + hex.EncodeToString(s[:])
}
func rawSHA(raw []byte) string { s := sha256.Sum256(raw); return "sha256:" + hex.EncodeToString(s[:]) }

func ComputeInstability(r InstabilityRequest) (InstabilityArtifact, error) {
	a := InstabilityArtifact{SchemaVersion: InstabilityVersion, ComparisonIdentity: r.Identity, Outcome: "STABLE", Thresholds: exactInstabilityThresholds, PolicyVersion: InstabilityPolicyVersion, Policy: InstabilityPolicy, PolicySHA256: InstabilityPolicyDigest(), PairwiseComparisons: []InstabilityPair{}, ClaimCeiling: InstabilityClaimCeiling, StructuralClaimCeiling: InstabilityStructuralCeiling}
	if err := validateInstabilityRequest(r); err != nil {
		return InstabilityArtifact{}, err
	}
	runs := append([]InstabilityRun(nil), r.Runs...)
	sort.Slice(runs, func(i, j int) bool { return runs[i].RunID < runs[j].RunID })
	a.RunAccounting.DeclaredSeedCount = len(r.DeclaredSeeds)
	a.RunAccounting.DeclaredRunCount = len(runs)
	a.RunAccounting.ExpectedPairCount = len(runs) * (len(runs) - 1) / 2
	for _, run := range runs {
		switch run.Status {
		case "COMPLETE":
			a.RunAccounting.CompletedRunCount++
		case "FAILED":
			a.RunAccounting.FailedRunCount++
		case "INCOMPLETE":
			a.RunAccounting.IncompleteRunCount++
		}
	}
	for i := 0; i < len(runs); i++ {
		for j := i + 1; j < len(runs); j++ {
			p := InstabilityPair{LeftRunID: runs[i].RunID, RightRunID: runs[j].RunID}
			if runs[i].Status != "COMPLETE" || runs[j].Status != "COMPLETE" {
				p.Outcome = "BLOCKED_INCOMPLETE_RUN"
				p.Reason = "incident run incomplete"
				if runs[i].Status == "FAILED" || runs[j].Status == "FAILED" {
					p.Outcome = "BLOCKED_FAILED_RUN"
					p.Reason = "incident run failed"
				}
				a.RunAccounting.BlockedPairCount++
				a.PairwiseComparisons = append(a.PairwiseComparisons, p)
				continue
			}
			var err error
			p, err = comparePartitions(runs[i], runs[j])
			if err != nil {
				return InstabilityArtifact{}, err
			}
			a.RunAccounting.CompletedPairCount++
			if p.Outcome == "UNSTABLE" {
				a.Outcome = "UNSTABLE"
			}
			a.PairwiseComparisons = append(a.PairwiseComparisons, p)
		}
	}
	if a.RunAccounting.BlockedPairCount > 0 {
		a.Outcome = "INCOMPLETE"
	}
	return a, nil
}

func validateInstabilityRequest(r InstabilityRequest) error {
	if len(r.DeclaredSeeds) < 2 || len(r.DeclaredSeeds) > 100 {
		return fmt.Errorf("declared seed count outside policy")
	}
	seeds := map[uint64]bool{}
	for _, s := range r.DeclaredSeeds {
		if seeds[s] {
			return fmt.Errorf("duplicate seed")
		}
		seeds[s] = true
	}
	if len(r.Runs) < 6 || len(r.Runs) > instabilityMaxRuns || len(r.Runs)*(len(r.Runs)-1)/2 > instabilityMaxPairs {
		return fmt.Errorf("run inventory outside policy")
	}
	ids := map[string]bool{}
	counts := map[uint64]int{}
	var campaignSource *SourceBinding
	for _, run := range r.Runs {
		if run.RunID == "" || ids[run.RunID] {
			return fmt.Errorf("invalid run id")
		}
		ids[run.RunID] = true
		if !seeds[run.Seed] {
			return fmt.Errorf("foreign seed")
		}
		counts[run.Seed]++
		if counts[run.Seed] > 3 {
			return fmt.Errorf("runs per seed exceeds policy")
		}
		if run.Status != "COMPLETE" && run.Status != "FAILED" && run.Status != "INCOMPLETE" {
			return fmt.Errorf("invalid run status")
		}
		if run.Status == "COMPLETE" {
			if err := validateAcceptedRun(run, r.Identity); err != nil {
				return fmt.Errorf("run %s: %w", run.RunID, err)
			}
			if !reflect.DeepEqual(run.Community.Source, run.Community.Projection.Source) {
				return fmt.Errorf("run %s: outcome/projection source binding mismatch", run.RunID)
			}
			if campaignSource == nil {
				source := run.Community.Source
				campaignSource = &source
			} else if !reflect.DeepEqual(*campaignSource, run.Community.Source) {
				return fmt.Errorf("run %s: cross-run source binding mismatch", run.RunID)
			}
		}
	}
	for s := range seeds {
		if counts[s] != 3 {
			return fmt.Errorf("seed %d must declare exactly three runs", s)
		}
	}
	return nil
}

func validateAcceptedRun(run InstabilityRun, id InstabilityIdentity) error {
	o := run.Community
	recomputed, failure := Compute(o.Source.InputBytes(), run.Seed)
	if failure != nil {
		return fmt.Errorf("source input recomputation: %w", failure)
	}
	if !reflect.DeepEqual(recomputed, o) {
		return fmt.Errorf("source input semantic recomputation mismatch")
	}
	if o.Outcome != "COMPLETE" && o.Outcome != "EMPTY" {
		return fmt.Errorf("community outcome not complete")
	}
	if o.Seed != run.Seed || o.ProfileID != id.ProjectionPolicyID || o.ProfileDigest != id.ProjectionSHA256 || o.Algorithm != id.AlgorithmName {
		return fmt.Errorf("community identity mismatch")
	}
	if o.Source.InputSHA256 != id.AdmittedGraphSHA256 || o.Source.SessionID == "" || o.Source.Generation == 0 {
		return fmt.Errorf("source/session/generation mismatch")
	}
	if o.Source.Completeness.Truncated || o.Source.DiagnosticsOmittedRecords > 0 || o.Source.DiagnosticsEvictedRecords > 0 {
		return fmt.Errorf("incomplete source")
	}
	if !validCanonicalCommunities(o.Communities, o.Projection.NodeIdentities) {
		return fmt.Errorf("noncanonical or incomplete partition")
	}
	if err := validateCommunityArtifact(run.CommunityArtifact, o, id); err != nil {
		return err
	}
	if len(run.BoundaryArtifact) == 0 {
		return fmt.Errorf("missing boundary artifact")
	}
	if err := ValidateBoundary(run.BoundaryArtifact, o, run.BoundaryRequest); err != nil {
		return fmt.Errorf("boundary artifact: %w", err)
	}
	return nil
}

type communityWireIdentity struct {
	GraphProvenanceV5SHA256   string `json:"graph_provenance_v5_sha256"`
	AdmittedGraphSHA256       string `json:"admitted_graph_sha256"`
	ProjectionSHA256          string `json:"projection_sha256"`
	ProjectionPolicyID        string `json:"projection_policy_id"`
	AlgorithmName             string `json:"algorithm_name"`
	AlgorithmVersion          string `json:"algorithm_version"`
	ParametersCanonicalSHA256 string `json:"parameters_canonical_sha256"`
	Seed                      uint64 `json:"seed"`
	ResourcePolicySHA256      string `json:"resource_policy_sha256"`
}
type communityWireCommunity struct {
	CommunityID   string   `json:"community_id"`
	MemberNodeIDs []string `json:"member_node_ids"`
}
type communityWireAccounting struct {
	AdmittedNodeCount    int `json:"admitted_node_count"`
	AccountedNodeCount   int `json:"accounted_node_count"`
	UnaccountedNodeCount int `json:"unaccounted_node_count"`
}
type communityWire struct {
	SchemaVersion string                   `json:"schema_version"`
	InputIdentity communityWireIdentity    `json:"input_identity"`
	Outcome       string                   `json:"outcome"`
	Communities   []communityWireCommunity `json:"communities"`
	Accounting    communityWireAccounting  `json:"accounting"`
	ClaimCeiling  string                   `json:"claim_ceiling"`
}

const communityWireCeiling = "Communities are structural observations under the exact bound Graph Provenance V5 input and projection. They do not establish feature, product, ownership, service, organizational, business-boundary, semantic-community, producer-authentication, permission, or production-authority claims."

func expectedCommunityWire(o Outcome, id InstabilityIdentity) communityWire {
	cs := make([]communityWireCommunity, len(o.Communities))
	for i, c := range o.Communities {
		members := append([]string(nil), c.Members...)
		cs[i] = communityWireCommunity{communityID(members), members}
	}
	return communityWire{"lsp-trace.community.v1", communityWireIdentity{o.Source.GraphV5SHA256, id.AdmittedGraphSHA256, id.ProjectionSHA256, id.ProjectionPolicyID, id.AlgorithmName, id.AlgorithmVersion, id.ParametersCanonicalSHA256, o.Seed, id.ResourcePolicySHA256}, o.Outcome, cs, communityWireAccounting{len(o.Projection.NodeIdentities), len(o.Projection.NodeIdentities), 0}, communityWireCeiling}
}
func validateCommunityArtifact(raw []byte, o Outcome, id InstabilityIdentity) error {
	if len(raw) == 0 {
		return fmt.Errorf("missing community artifact")
	}
	if _, err := schema.ValidateFor(raw, schema.FamilyCommunity, "v1"); err != nil {
		return err
	}
	var got communityWire
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&got); err != nil {
		return err
	}
	gb, _ := json.Marshal(got)
	wb, _ := json.Marshal(expectedCommunityWire(o, id))
	if !bytes.Equal(gb, wb) {
		return fmt.Errorf("semantic community validation mismatch")
	}
	return nil
}

type matchedPair struct {
	l, r  []string
	score float64
}

func comparePartitions(l, r InstabilityRun) (InstabilityPair, error) {
	p := InstabilityPair{LeftRunID: l.RunID, RightRunID: r.RunID, LeftCommunityArtifactSHA256: rawSHA(l.CommunityArtifact), RightCommunityArtifactSHA256: rawSHA(r.CommunityArtifact)}
	pairs, err := maximumMatching(l.Community.Communities, r.Community.Communities)
	if err != nil {
		return p, err
	}
	vals := make([]float64, len(pairs))
	unmatched := 0
	for i, x := range pairs {
		vals[i] = x.score
		if x.l == nil || x.r == nil {
			unmatched++
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].score != pairs[j].score {
			return pairs[i].score > pairs[j].score
		}
		if c := compareStrings(pairs[i].l, pairs[j].l); c != 0 {
			return c < 0
		}
		return compareStrings(pairs[i].r, pairs[j].r) < 0
	})
	sort.Float64s(vals)
	p.MatchedPairDenominator = len(vals)
	if len(vals) > 0 {
		p.PairwiseJaccardMinimum = vals[0]
		p.PairwiseJaccardMedian = median(vals)
		for _, v := range vals {
			p.PairwiseJaccardMean += v
		}
		p.PairwiseJaccardMean /= float64(len(vals))
	}
	p.UnmatchedCommunityNumerator = unmatched
	p.UnmatchedCommunityDenominator = len(l.Community.Communities) + len(r.Community.Communities)
	p.NodeReassignmentNumerator, p.NodeReassignmentDenominator = nodeReassignment(pairs)
	p.VariationOfInformationBits, p.VariationOfInformationNodeDenominator = variationInformation(l.Community.Communities, r.Community.Communities)
	p.Outcome = "STABLE"
	if rate(p.NodeReassignmentNumerator, p.NodeReassignmentDenominator) > exactInstabilityThresholds.NodeReassignmentRateMax || rate(p.UnmatchedCommunityNumerator, p.UnmatchedCommunityDenominator) > exactInstabilityThresholds.UnmatchedCommunityRateMax || p.VariationOfInformationBits > exactInstabilityThresholds.VariationOfInformationBitsMax || p.PairwiseJaccardMinimum < exactInstabilityThresholds.PairwiseJaccardMinimumMin {
		p.Outcome = "UNSTABLE"
	}
	return p, nil
}
func rate(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}
func median(v []float64) float64 {
	n := len(v)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return v[n/2]
	}
	return (v[n/2-1] + v[n/2]) / 2
}
func jaccard(a, b []string) float64 {
	i, j, in := 0, 0, 0
	for i < len(a) && j < len(b) {
		if a[i] == b[j] {
			in++
			i++
			j++
		} else if a[i] < b[j] {
			i++
		} else {
			j++
		}
	}
	u := len(a) + len(b) - in
	if u == 0 {
		return 0
	}
	return float64(in) / float64(u)
}

func jaccardRat(a, b []string) *big.Rat {
	i, j, in := 0, 0, 0
	for i < len(a) && j < len(b) {
		if a[i] == b[j] {
			in++
			i++
			j++
		} else if a[i] < b[j] {
			i++
		} else {
			j++
		}
	}
	u := len(a) + len(b) - in
	if u == 0 {
		return new(big.Rat)
	}
	return new(big.Rat).SetFrac64(int64(in), int64(u))
}

func maximumMatching(a, b []Community) ([]matchedPair, error) {
	swap := false
	if len(a) > len(b) {
		a, b = b, a
		swap = true
	}
	n, m := len(a), len(b)
	if m >= 63 {
		return nil, fmt.Errorf("matching policy work limit")
	}
	states := 1 << m
	if states*n > instabilityMaxMatchingWork {
		return nil, fmt.Errorf("matching policy work limit")
	}
	type cell struct {
		ok     bool
		score  *big.Rat
		choice []int
	}
	dp := make([]cell, states)
	dp[0] = cell{ok: true, score: new(big.Rat), choice: []int{}}
	for i := 0; i < n; i++ {
		next := make([]cell, states)
		for mask, c := range dp {
			if !c.ok {
				continue
			}
			for j := 0; j < m; j++ {
				if mask&(1<<j) != 0 {
					continue
				}
				nm := mask | 1<<j
				nc := cell{true, new(big.Rat).Add(c.score, jaccardRat(a[i].Members, b[j].Members)), append(append([]int{}, c.choice...), j)}
				if !next[nm].ok || nc.score.Cmp(next[nm].score) > 0 || (nc.score.Cmp(next[nm].score) == 0 && lexInts(nc.choice, next[nm].choice)) {
					next[nm] = nc
				}
			}
		}
		dp = next
	}
	best := cell{}
	for _, c := range dp {
		if c.ok && (!best.ok || c.score.Cmp(best.score) > 0 || (c.score.Cmp(best.score) == 0 && lexInts(c.choice, best.choice))) {
			best = c
		}
	}
	used := map[int]bool{}
	out := make([]matchedPair, 0, m)
	for i, j := range best.choice {
		used[j] = true
		x := matchedPair{a[i].Members, b[j].Members, jaccard(a[i].Members, b[j].Members)}
		if swap {
			x.l, x.r = x.r, x.l
		}
		out = append(out, x)
	}
	for j := 0; j < m; j++ {
		if !used[j] {
			x := matchedPair{nil, b[j].Members, 0}
			if swap {
				x.l, x.r = x.r, x.l
			}
			out = append(out, x)
		}
	}
	return out, nil
}
func lexInts(a, b []int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}
func communityMap(cs []Community) map[string][]string {
	m := map[string][]string{}
	for _, c := range cs {
		for _, id := range c.Members {
			m[id] = c.Members
		}
	}
	return m
}
func unionNodes(a, b map[string][]string) []string {
	m := map[string]bool{}
	for k := range a {
		m[k] = true
	}
	for k := range b {
		m[k] = true
	}
	v := make([]string, 0, len(m))
	for k := range m {
		v = append(v, k)
	}
	sort.Strings(v)
	return v
}
func nodeReassignment(pairs []matchedPair) (int, int) {
	type matchedSets struct{ left, right []string }
	byNode := map[string]matchedSets{}
	for _, pair := range pairs {
		sets := matchedSets{left: pair.l, right: pair.r}
		for _, id := range pair.l {
			byNode[id] = sets
		}
		for _, id := range pair.r {
			byNode[id] = sets
		}
	}
	n := 0
	for _, sets := range byNode {
		if compareStrings(sets.left, sets.right) != 0 {
			n++
		}
	}
	return n, len(byNode)
}
func variationInformation(a, b []Community) (float64, int) {
	am, bm := communityMap(a), communityMap(b)
	nodes := unionNodes(am, bm)
	n := len(nodes)
	if n == 0 {
		return 0, 0
	}
	ac, bc, joint := map[string]int{}, map[string]int{}, map[string]int{}
	for _, id := range nodes {
		x, ok := am[id]
		if !ok {
			x = []string{id}
		}
		y, ok := bm[id]
		if !ok {
			y = []string{id}
		}
		xs, ys := string(canonicalInstability(x)), string(canonicalInstability(y))
		ac[xs]++
		bc[ys]++
		joint[xs+"\x00"+ys]++
	}
	h := func(m map[string]int) float64 {
		z := 0.0
		for _, c := range m {
			p := float64(c) / float64(n)
			z -= p * math.Log2(p)
		}
		return z
	}
	mi := 0.0
	for key, c := range joint {
		_ = key
		p := float64(c) / float64(n) /* recover marginals by recounting nodes below */
		mi += p * 0
	}
	mi = 0
	for _, id := range nodes {
		x, ok := am[id]
		if !ok {
			x = []string{id}
		}
		y, ok := bm[id]
		if !ok {
			y = []string{id}
		}
		xs, ys := string(canonicalInstability(x)), string(canonicalInstability(y))
		c := joint[xs+"\x00"+ys]
		mi += math.Log2(float64(c*n)/float64(ac[xs]*bc[ys])) / float64(n)
	}
	v := h(ac) + h(bc) - 2*mi
	if v < 0 && v > -1e-12 {
		v = 0
	}
	return v, n
}

func ValidateInstability(raw []byte, r InstabilityRequest) error {
	if _, err := schema.ValidateFor(raw, schema.FamilyCommunityInstability, "v1"); err != nil {
		return err
	}
	var got InstabilityArtifact
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&got); err != nil {
		return err
	}
	want, err := ComputeInstability(r)
	if err != nil {
		return err
	}
	gb, _ := json.Marshal(got)
	wb, _ := json.Marshal(want)
	if !bytes.Equal(gb, wb) {
		return fmt.Errorf("semantic instability validation mismatch")
	}
	return nil
}
