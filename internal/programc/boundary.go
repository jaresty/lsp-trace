package programc

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"lsp-trace/internal/schema"
)

const BoundaryVersion = "lsp-trace.community-boundary.v1"
const BoundaryClaimCeiling = "All measures and witnesses are structural observations under the exact bound projection. They do not establish feature, product, ownership, service, organizational, or business-boundary identity or explanation."
const BoundaryPolicyID = "program-c-a-07-boundary-accounting/v2"
const boundaryPageRankAlpha = .85
const boundaryPageRankTolerance = 1e-9
const boundaryPageRankMaxIterations = 1000
const boundaryPageRankMaxWork = 1000000

type BoundaryPageRankPolicy struct {
	Alpha         float64 `json:"alpha"`
	Tolerance     float64 `json:"tolerance"`
	MaxIterations int     `json:"max_iterations"`
	MaxWork       int     `json:"max_work"`
	Arithmetic    string  `json:"arithmetic"`
	Score         string  `json:"score"`
	Work          string  `json:"work"`
	Convergence   string  `json:"convergence"`
}
type BoundaryPolicySpec struct {
	ID                       string                 `json:"id"`
	Conductance              string                 `json:"conductance"`
	ConductanceZeroOutcome   string                 `json:"conductance_zero_outcome"`
	PageRank                 BoundaryPageRankPolicy `json:"pagerank"`
	PageRankSelection        string                 `json:"pagerank_selection"`
	HubSelection             string                 `json:"hub_selection"`
	HubScore                 string                 `json:"hub_score"`
	WeakCriticalSemantics    string                 `json:"weak_critical_semantics"`
	CrossingWitnessSemantics string                 `json:"crossing_witness_semantics"`
}

var BoundaryPolicy = BoundaryPolicySpec{
	ID:                       BoundaryPolicyID,
	Conductance:              "cut_weight(C)/(min(directed_out_volume(C),directed_out_volume(V\\C)))",
	ConductanceZeroOutcome:   "UNAVAILABLE:zero minimum directed out-volume",
	PageRank:                 BoundaryPageRankPolicy{boundaryPageRankAlpha, boundaryPageRankTolerance, boundaryPageRankMaxIterations, boundaryPageRankMaxWork, "Go-binary64-explicit-rounding-sequential/v1", "positive-binary64-node-score/v1", "evaluation-3n+m/v1", "stationary-L1-inclusive/v1"},
	PageRankSelection:        "required-top-k-inclusive-ties-then-crossing/v1",
	HubSelection:             "required-top-k-inclusive-ties-then-crossing/v1",
	HubScore:                 "weighted-directed-in-plus-out-occurrence-sum/v1",
	WeakCriticalSemantics:    "weak-undirected-occurrence-multigraph-bridge-articulation/v1",
	CrossingWitnessSemantics: "direct-community-pair-minimum-occurrence-identity/v1",
}

func BoundaryPolicyBytes() []byte  { return canonicalBoundary(BoundaryPolicy) }
func BoundaryPolicyDigest() string { return bHash(BoundaryVersion+":policy", BoundaryPolicy) }

var boundaryRankPolicies = []string{"initial-p/v1", "dangling-p/v1", "stationary-L1-inclusive/v1", "lexical-nodes-occurrence-ID/v1", "Go-binary64-explicit-rounding-sequential/v1", "positive-binary64-node-score/v1", "evaluation-3n+m/v1"}

type BoundaryRequest struct {
	PageRankTopK int `json:"pagerank_top_k"`
	HubTopK      int `json:"hub_top_k"`
}
type BoundaryAccounting struct {
	AdmittedOccurrences    int `json:"admitted_relation_occurrence_count"`
	AccountedOccurrences   int `json:"accounted_relation_occurrence_count"`
	UnaccountedOccurrences int `json:"unaccounted_relation_occurrence_count"`
	IntraOccurrences       int `json:"intra_relation_occurrence_count"`
	CrossingOccurrences    int `json:"crossing_relation_occurrence_count"`
	AdmittedNodes          int `json:"admitted_node_count"`
	AccountedNodes         int `json:"accounted_node_count"`
	UnaccountedNodes       int `json:"unaccounted_node_count"`
}
type BoundaryMeasure struct {
	Outcome     string  `json:"outcome"`
	Numerator   float64 `json:"numerator"`
	Denominator float64 `json:"denominator"`
	Value       float64 `json:"value"`
	Reason      string  `json:"reason,omitempty"`
}
type BoundaryCommunity struct {
	CommunityID         string          `json:"community_id"`
	Members             []string        `json:"members"`
	IntraOccurrences    BoundaryMeasure `json:"intra_occurrences"`
	CrossingOccurrences BoundaryMeasure `json:"crossing_occurrences"`
	Conductance         BoundaryMeasure `json:"conductance"`
}
type BoundaryNodeScore struct {
	NodeID string  `json:"node_id"`
	Score  float64 `json:"score"`
}
type CrossingWitness struct {
	CommunityA   string `json:"community_a"`
	CommunityB   string `json:"community_b"`
	OccurrenceID string `json:"occurrence_id"`
	SourceNodeID string `json:"source_node_id"`
	TargetNodeID string `json:"target_node_id"`
}
type BoundaryPageRank struct {
	Status            string   `json:"status"`
	Reason            string   `json:"reason"`
	Iterations        int      `json:"iterations"`
	Work              int      `json:"work"`
	Residual          *float64 `json:"residual"`
	ResidualIteration *int     `json:"residual_iteration"`
}
type BoundaryBindings struct {
	SourceSHA256     string `json:"source_sha256"`
	SessionID        string `json:"session_id"`
	Generation       uint64 `json:"generation"`
	ProfileID        string `json:"profile_id"`
	ProfileSHA256    string `json:"profile_sha256"`
	Algorithm        string `json:"algorithm"`
	AlgorithmVersion string `json:"algorithm_version"`
	PartitionSHA256  string `json:"partition_sha256"`
	PolicySHA256     string `json:"policy_sha256"`
	Seed             uint64 `json:"seed"`
}
type BoundaryArtifact struct {
	SchemaVersion               string              `json:"schema_version"`
	Outcome                     string              `json:"outcome"`
	Bindings                    BoundaryBindings    `json:"bindings"`
	Request                     BoundaryRequest     `json:"request"`
	Policy                      BoundaryPolicySpec  `json:"policy"`
	RankPolicies                []string            `json:"rank_policies"`
	PageRank                    BoundaryPageRank    `json:"pagerank"`
	Accounting                  BoundaryAccounting  `json:"accounting"`
	Communities                 []BoundaryCommunity `json:"communities"`
	HighCentralityCrossingNodes []BoundaryNodeScore `json:"high_centrality_crossing_nodes"`
	HubCrossingNodes            []BoundaryNodeScore `json:"hub_crossing_nodes"`
	Bridges                     []string            `json:"bridges"`
	ArticulationPoints          []string            `json:"articulation_points"`
	CrossingWitnesses           []CrossingWitness   `json:"crossing_witnesses"`
	ClaimCeiling                string              `json:"claim_ceiling"`
	Digest                      string              `json:"digest"`
}

func canonicalBoundary(v any) []byte {
	b, _ := json.Marshal(v)
	var x any
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	_ = d.Decode(&x)
	b, _ = json.Marshal(x)
	return b
}
func bHash(domain string, v any) string {
	b := canonicalBoundary(v)
	s := sha256.Sum256(append(append([]byte(domain), 0), b...))
	return "sha256:" + hex.EncodeToString(s[:])
}
func communityID(m []string) string { return bHash(BoundaryVersion+":community", m) }
func boundarySeal(a BoundaryArtifact) string {
	a.Digest = ""
	return bHash(BoundaryVersion+":result", a)
}
func add64(a, b float64) float64 { return float64(a + b) }
func mul64(a, b float64) float64 { return float64(a * b) }
func sub64(a, b float64) float64 { return float64(a - b) }
func div64(a, b float64) float64 { return float64(a / b) }
func positiveZero64(x float64) float64 {
	if x == 0 {
		return 0
	}
	return x
}

func ComputeBoundary(o Outcome, r BoundaryRequest) (BoundaryArtifact, error) {
	n := len(o.Projection.NodeIdentities)
	if n == 0 || r.PageRankTopK < 1 || r.PageRankTopK > n || r.HubTopK < 1 || r.HubTopK > n {
		return BoundaryArtifact{}, fmt.Errorf("invalid mandatory top_k for %d admitted nodes", n)
	}
	member := map[string]int{}
	for ci, c := range o.Communities {
		if len(c.Members) == 0 {
			return BoundaryArtifact{}, fmt.Errorf("empty community")
		}
		for _, id := range c.Members {
			if _, ok := o.Projection.NodeIDs[id]; !ok {
				return BoundaryArtifact{}, fmt.Errorf("partition foreign key %q", id)
			}
			if _, ok := member[id]; ok {
				return BoundaryArtifact{}, fmt.Errorf("partition duplicate %q", id)
			}
			member[id] = ci
		}
	}
	if len(member) != n {
		return BoundaryArtifact{}, fmt.Errorf("partition does not cover admitted nodes")
	}
	occ := append([]Occurrence{}, o.Projection.Occurrences...)
	sort.Slice(occ, func(i, j int) bool { return occ[i].Identity < occ[j].Identity })
	for i, e := range occ {
		if e.Weight <= 0 || math.IsNaN(e.Weight) || math.IsInf(e.Weight, 0) || e.From < 0 || e.To < 0 || int(e.From) >= n || int(e.To) >= n {
			return BoundaryArtifact{}, fmt.Errorf("invalid occurrence")
		}
		if i > 0 && e.Identity == occ[i-1].Identity {
			return BoundaryArtifact{}, fmt.Errorf("duplicate occurrence identity")
		}
	}
	source := o.Source.InputSHA256
	if source == "" {
		source = o.Projection.Source.InputSHA256
	}
	bindings := BoundaryBindings{source, o.Source.SessionID, o.Source.Generation, o.ProfileID, o.ProfileDigest, o.Algorithm, "gonum-v0.17.1", o.LogicalDigest, BoundaryPolicyDigest(), o.Seed}
	a := BoundaryArtifact{SchemaVersion: BoundaryVersion, Outcome: "COMPLETE", Bindings: bindings, Request: r, Policy: BoundaryPolicy, RankPolicies: append([]string{}, boundaryRankPolicies...), PageRank: BoundaryPageRank{Status: "INCOMPLETE"}, Accounting: BoundaryAccounting{AdmittedOccurrences: len(occ), AccountedOccurrences: len(occ), AdmittedNodes: n, AccountedNodes: n}, ClaimCeiling: BoundaryClaimCeiling, Communities: make([]BoundaryCommunity, len(o.Communities)), Bridges: []string{}, ArticulationPoints: []string{}, CrossingWitnesses: []CrossingWitness{}, HighCentralityCrossingNodes: []BoundaryNodeScore{}, HubCrossingNodes: []BoundaryNodeScore{}}
	if len(occ) == 0 {
		a.Outcome = "EMPTY"
	}
	if o.Source.Completeness.Truncated || o.Source.DiagnosticsOmittedRecords > 0 || o.Source.DiagnosticsEvictedRecords > 0 {
		a.Outcome = "INCOMPLETE"
	}
	crossing := map[int64]bool{}
	pairWitness := map[[2]int]Occurrence{}
	out := make([]float64, n)
	hub := make([]float64, n)
	intra := make([]int, len(o.Communities))
	cross := make([]int, len(o.Communities))
	cut := make([]float64, len(o.Communities))
	for _, e := range occ {
		u, v := o.Projection.NodeIdentities[e.From], o.Projection.NodeIdentities[e.To]
		cu, cv := member[u], member[v]
		out[e.From] = add64(out[e.From], e.Weight)
		hub[e.From] = add64(hub[e.From], e.Weight)
		hub[e.To] = add64(hub[e.To], e.Weight)
		if cu == cv {
			a.Accounting.IntraOccurrences++
			intra[cu]++
		} else {
			a.Accounting.CrossingOccurrences++
			cross[cu]++
			cross[cv]++
			cut[cu] = add64(cut[cu], e.Weight)
			cut[cv] = add64(cut[cv], e.Weight)
			crossing[e.From] = true
			crossing[e.To] = true
			p := [2]int{cu, cv}
			if p[0] > p[1] {
				p[0], p[1] = p[1], p[0]
			}
			if old, ok := pairWitness[p]; !ok || e.Identity < old.Identity {
				pairWitness[p] = e
			}
		}
	}
	totalOut := 0.
	for _, x := range out {
		totalOut = add64(totalOut, x)
	}
	for i, c := range o.Communities {
		m := append([]string{}, c.Members...)
		sort.Strings(m)
		os := 0.
		for _, id := range m {
			os = add64(os, out[o.Projection.NodeIDs[id]])
		}
		den := math.Min(os, float64(totalOut-os))
		bc := BoundaryCommunity{CommunityID: communityID(m), Members: m, IntraOccurrences: BoundaryMeasure{Outcome: "VALUE", Numerator: float64(intra[i]), Denominator: float64(len(occ))}, CrossingOccurrences: BoundaryMeasure{Outcome: "VALUE", Numerator: float64(cross[i]), Denominator: float64(len(occ))}, Conductance: BoundaryMeasure{Outcome: "VALUE", Numerator: cut[i], Denominator: den}}
		if len(occ) == 0 {
			bc.IntraOccurrences.Outcome = "EMPTY"
			bc.CrossingOccurrences.Outcome = "EMPTY"
		}
		if den == 0 {
			bc.Conductance.Outcome = "UNAVAILABLE"
			bc.Conductance.Reason = "zero minimum directed out-volume"
		} else {
			bc.Conductance.Value = div64(cut[i], den)
		}
		a.Communities[i] = bc
	}
	sort.Slice(a.Communities, func(i, j int) bool { return a.Communities[i].CommunityID < a.Communities[j].CommunityID })
	prs, pr := pageRank(n, occ, out)
	a.PageRank = pr
	if pr.Status == "COMPLETE" {
		a.HighCentralityCrossingNodes = topScores(o.Projection.NodeIdentities, prs, crossing, r.PageRankTopK)
	} else {
		a.Outcome = "INCOMPLETE"
	}
	a.HubCrossingNodes = topScores(o.Projection.NodeIdentities, hub, crossing, r.HubTopK)
	a.Bridges, a.ArticulationPoints = weakCritical(o.Projection.NodeIdentities, occ)
	keys := make([][2]int, 0, len(pairWitness))
	for k := range pairWitness {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i][0] != keys[j][0] {
			return keys[i][0] < keys[j][0]
		}
		return keys[i][1] < keys[j][1]
	})
	for _, k := range keys {
		e := pairWitness[k]
		a.CrossingWitnesses = append(a.CrossingWitnesses, CrossingWitness{communityID(o.Communities[k[0]].Members), communityID(o.Communities[k[1]].Members), e.Identity, o.Projection.NodeIdentities[e.From], o.Projection.NodeIdentities[e.To]})
	}
	a.Digest = boundarySeal(a)
	return a, nil
}

// pageRank mirrors internal/boundedranking's binary64 boundaries,
// stationary-L1-inclusive convergence test, and 3n+m work definition. Its
// unexported runner ranks unweighted retained-call groups, so Program C keeps
// this weighted-occurrence adapter bound to the same policy identities.
func pageRank(n int, es []Occurrence, out []float64) ([]float64, BoundaryPageRank) {
	result := BoundaryPageRank{Status: "INCOMPLETE"}
	x := make([]float64, n)
	for i := range x {
		x[i] = div64(1, float64(n))
	}
	tick := func() bool {
		if result.Work == boundaryPageRankMaxWork {
			result.Reason = "LIMIT"
			return false
		}
		result.Work++
		return true
	}
	for {
		y := make([]float64, n)
		dang := 0.
		for i := range x {
			if !tick() {
				return nil, result
			}
			if out[i] == 0 {
				dang = add64(dang, x[i])
			}
		}
		for _, e := range es {
			if !tick() {
				return nil, result
			}
			y[e.To] = add64(y[e.To], mul64(x[e.From], div64(e.Weight, out[e.From])))
		}
		for i := range y {
			if !tick() {
				return nil, result
			}
			p := div64(1, float64(n))
			y[i] = positiveZero64(add64(mul64(sub64(1, boundaryPageRankAlpha), p), mul64(boundaryPageRankAlpha, add64(y[i], mul64(dang, p)))))
		}
		res := 0.
		for i := range x {
			if !tick() {
				return nil, result
			}
			res = add64(res, math.Abs(sub64(y[i], x[i])))
		}
		it := result.Iterations
		result.Residual = &res
		result.ResidualIteration = &it
		if res <= boundaryPageRankTolerance {
			result.Status = "COMPLETE"
			return x, result
		}
		if result.Iterations == boundaryPageRankMaxIterations {
			result.Reason = "NOT_CONVERGED"
			return nil, result
		}
		x = y
		result.Iterations++
	}
}
func topScores(ids []string, s []float64, cross map[int64]bool, k int) []BoundaryNodeScore {
	all := make([]BoundaryNodeScore, len(ids))
	for i, id := range ids {
		all[i] = BoundaryNodeScore{id, s[i]}
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Score != all[j].Score {
			return all[i].Score > all[j].Score
		}
		return all[i].NodeID < all[j].NodeID
	})
	cut := all[k-1].Score
	out := []BoundaryNodeScore{}
	for _, v := range all {
		if v.Score < cut {
			break
		}
		idx := sort.SearchStrings(ids, v.NodeID)
		if cross[int64(idx)] {
			out = append(out, v)
		}
	}
	return out
}
func weakCritical(ids []string, es []Occurrence) ([]string, []string) {
	type ae struct{ to, e int }
	adj := make([][]ae, len(ids))
	for i, x := range es {
		if x.From == x.To {
			continue
		}
		adj[x.From] = append(adj[x.From], ae{int(x.To), i})
		adj[x.To] = append(adj[x.To], ae{int(x.From), i})
	}
	disc := make([]int, len(ids))
	low := make([]int, len(ids))
	time := 0
	arts := map[int]bool{}
	bridges := []string{}
	var dfs func(int, int)
	dfs = func(u, pe int) {
		time++
		disc[u] = time
		low[u] = time
		children := 0
		for _, a := range adj[u] {
			if a.e == pe {
				continue
			}
			if disc[a.to] == 0 {
				children++
				dfs(a.to, a.e)
				if low[a.to] < low[u] {
					low[u] = low[a.to]
				}
				if low[a.to] > disc[u] {
					bridges = append(bridges, es[a.e].Identity)
				}
				if pe >= 0 && low[a.to] >= disc[u] {
					arts[u] = true
				}
			} else if disc[a.to] < low[u] {
				low[u] = disc[a.to]
			}
		}
		if pe < 0 && children > 1 {
			arts[u] = true
		}
	}
	for i := range ids {
		if disc[i] == 0 {
			dfs(i, -1)
		}
	}
	sort.Strings(bridges)
	ap := []string{}
	for i := range arts {
		ap = append(ap, ids[i])
	}
	sort.Strings(ap)
	return bridges, ap
}
func ValidateBoundary(raw []byte, o Outcome, r BoundaryRequest) error {
	if _, err := schema.ValidateFor(raw, schema.FamilyCommunityBoundary, "v1"); err != nil {
		return err
	}
	var got BoundaryArtifact
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&got); err != nil {
		return err
	}
	want, err := ComputeBoundary(o, r)
	if err != nil {
		return err
	}
	gb, _ := json.Marshal(got)
	wb, _ := json.Marshal(want)
	if !bytes.Equal(gb, wb) {
		return fmt.Errorf("semantic boundary validation mismatch")
	}
	return nil
}
