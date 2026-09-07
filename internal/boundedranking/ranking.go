// Package boundedranking ranks only admitted historical retained CALLS.
package boundedranking

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"lsp-trace/internal/boundedanalysis"
	"lsp-trace/internal/boundedmetrics"
	"lsp-trace/internal/retainedcalls"
	"lsp-trace/internal/schema"
)

const Family = "bounded-retained-ranking"
const Version = "lsp-trace.bounded-retained-ranking.v1"
const Policy = "retained-CALLS-unit-group-ranking/v1"
const MaxInputBytes = boundedanalysis.MaxInputBytes
const MaxBytes = boundedanalysis.MaxBytes

// Policy IDs are part of the basis, not request/runtime metadata.
var policies = []string{"initial-p/v1", "dangling-p/v1", "stationary-L1/v1", "lexical-nodes-group-ID/v1", "Go-binary64-explicit-rounding-sequential/v1", "evaluation-3n+m/v1"}

type Seed struct {
	NodeID string `json:"node_id"`
	Weight int64  `json:"weight"`
}
type Parameters struct {
	Algorithm     string  `json:"algorithm"`
	Alpha         float64 `json:"alpha"`
	Tolerance     float64 `json:"tolerance"`
	MaxIterations int     `json:"max_iterations"`
	MaxWork       int     `json:"max_work"`
	Seeds         []Seed  `json:"seeds"`
}

func Defaults(algorithm string) Parameters {
	return Parameters{algorithm, .85, 1e-9, 1000, 1000000, []Seed{}}
}

type Value struct {
	NodeID string  `json:"node_id"`
	Score  float64 `json:"score"`
}
type Evidence struct {
	SchemaVersion     string     `json:"schema_version"`
	Policy            string     `json:"policy"`
	Policies          []string   `json:"policies"`
	Scope             string     `json:"scope"`
	InputBytes        []byte     `json:"input_bytes"`
	Parameters        Parameters `json:"parameters"`
	BasisDigest       string     `json:"basis_digest"`
	Nodes             []string   `json:"nodes"`
	GroupCount        int        `json:"group_count"`
	Personalization   []Value    `json:"personalization"`
	Status            string     `json:"status"`
	Reason            string     `json:"reason"`
	Iterations        int        `json:"iterations"`
	Work              int        `json:"work"`
	Residual          *float64   `json:"residual"`
	Mass              *float64   `json:"score_mass"`
	ResidualIteration *int       `json:"residual_iteration"`
	Scores            []Value    `json:"scores"`
	Ranks             []string   `json:"ranks"`
	Digest            string     `json:"digest"`
}

func canonical(v any) []byte {
	b, _ := json.Marshal(v)
	var x any
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	_ = d.Decode(&x)
	b, _ = json.Marshal(x)
	return b
}
func hash(domain string, v any) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256(append(append([]byte(domain), 0), canonical(v)...)))
}
func seal(e Evidence) string { e.Digest = ""; return hash(Version+":result", e) }
func finite(x float64) bool  { return !math.IsNaN(x) && !math.IsInf(x, 0) }
func normalize(p Parameters, ids []string) (Parameters, []Value, error) {
	fail := func() (Parameters, []Value, error) {
		return p, nil, fmt.Errorf("invalid ranking parameters or exact seeds")
	}
	if !finite(p.Alpha) || p.Alpha <= 0 || p.Alpha > .99 || !finite(p.Tolerance) || p.Tolerance < 1e-12 || p.Tolerance > 1e-3 || p.MaxIterations < 1 || p.MaxIterations > 10000 || p.MaxWork < 1 || p.MaxWork > 1000000 {
		return fail()
	}
	p.Seeds = append([]Seed{}, p.Seeds...)
	sort.Slice(p.Seeds, func(i, j int) bool { return p.Seeds[i].NodeID < p.Seeds[j].NodeID })
	v := make([]Value, len(ids))
	for i, id := range ids {
		v[i].NodeID = id
	}
	switch p.Algorithm {
	case "PAGERANK":
		if len(p.Seeds) != 0 {
			return fail()
		}
		for i := range v {
			v[i].Score = float64(1 / float64(len(v)))
		}
	case "PPR":
		if len(p.Seeds) == 0 || len(p.Seeds) > len(ids) {
			return fail()
		}
		var total int64
		for i, s := range p.Seeds {
			j := sort.SearchStrings(ids, s.NodeID)
			if s.Weight < 1 || s.Weight > 1000000 || j == len(ids) || ids[j] != s.NodeID || (i > 0 && p.Seeds[i-1].NodeID == s.NodeID) {
				return fail()
			}
			total += s.Weight
		}
		for _, s := range p.Seeds {
			v[sort.SearchStrings(ids, s.NodeID)].Score = float64(float64(s.Weight) / float64(total))
		}
	default:
		return fail()
	}
	return p, v, nil
}

type edge struct{ a, b int }
type topology struct {
	ids    []string
	edges  []edge
	degree []int
}

func graph(t retainedcalls.Tables) topology {
	g := topology{ids: []string{}, edges: []edge{}, degree: make([]int, len(t.Endpoints))}
	for _, n := range t.Endpoints {
		g.ids = append(g.ids, n.ID)
	}
	sort.Strings(g.ids)
	groups := append([]retainedcalls.Group{}, t.Groups...)
	sort.Slice(groups, func(i, j int) bool { return groups[i].RelationID < groups[j].RelationID })
	for _, e := range groups {
		a, b := sort.SearchStrings(g.ids, e.CallerNodeID), sort.SearchStrings(g.ids, e.CalleeNodeID)
		g.edges = append(g.edges, edge{a, b})
		g.degree[a]++
	}
	return g
}
func prepare(raw []byte, t retainedcalls.Tables, p Parameters) (Evidence, topology, error) {
	g := graph(t)
	p, v, err := normalize(p, g.ids)
	if err != nil {
		return Evidence{}, g, err
	}
	e := Evidence{SchemaVersion: Version, Policy: Policy, Policies: append([]string{}, policies...), Scope: boundedanalysis.Scope, InputBytes: append([]byte{}, raw...), Parameters: p, Nodes: g.ids, GroupCount: len(g.edges), Personalization: v, Status: "INCOMPLETE", Scores: []Value{}, Ranks: []string{}}
	e.BasisDigest = hash(Version+":basis", struct {
		Policy     string     `json:"policy"`
		Policies   []string   `json:"policies"`
		Input      []byte     `json:"input_bytes"`
		Parameters Parameters `json:"parameters"`
	}{Policy, policies, raw, p})
	return e, g, nil
}

// Every elementary operation has an explicit binary64 rounding boundary. In
// particular, conversion of products prevents compiler FMA contraction.
func add(a, b float64) float64 { return float64(a + b) }
func mul(a, b float64) float64 { return float64(a * b) }
func sub(a, b float64) float64 { return float64(a - b) }
func div(a, b float64) float64 { return float64(a / b) }
func positiveZero(x float64) float64 {
	if x == 0 {
		return 0
	}
	return x
}

// stop is a validation-only retained cancellation boundary; -1 disables it.
func run(ctx context.Context, e *Evidence, g topology, stop int) {
	// Updating x costs no work. Collapse cancellation at that zero-work boundary
	// to the last evaluated candidate, so cancellation replay is unambiguous.
	defer func() {
		if e.Reason == "CANCELLED" && e.ResidualIteration != nil && e.Work%(3*len(g.ids)+len(g.edges)) == 0 {
			e.Iterations = *e.ResidualIteration
		}
	}()
	cancelled := func() bool { return ctx.Err() != nil || (stop >= 0 && e.Work >= stop) }
	tick := func() bool {
		if cancelled() {
			e.Reason = "CANCELLED"
			return false
		}
		if e.Work == e.Parameters.MaxWork {
			e.Reason = "LIMIT"
			return false
		}
		e.Work++
		return true
	}
	if cancelled() {
		e.Reason = "CANCELLED"
		return
	}
	n := len(g.ids)
	x := make([]float64, n)
	for i := range x {
		x[i] = e.Personalization[i].Score
	}
	if n == 0 {
		z := 0.
		it := 0
		e.Residual = &z
		e.Mass = &z
		e.ResidualIteration = &it
		e.Status = "COMPLETE"
		return
	}
	for {
		y := make([]float64, n)
		dangling := 0.
		for i := range x {
			if !tick() {
				return
			}
			if g.degree[i] == 0 {
				dangling = add(dangling, x[i])
			}
		}
		for _, ed := range g.edges {
			if !tick() {
				return
			}
			y[ed.b] = add(y[ed.b], div(x[ed.a], float64(g.degree[ed.a])))
		}
		for i := range y {
			if !tick() {
				return
			}
			p := e.Personalization[i].Score
			y[i] = positiveZero(add(mul(sub(1, e.Parameters.Alpha), p), mul(e.Parameters.Alpha, add(y[i], mul(dangling, p)))))
		}
		residual, mass := 0., 0.
		for i := range x {
			if !tick() {
				return
			}
			residual = add(residual, math.Abs(sub(y[i], x[i])))
			mass = add(mass, x[i])
		}
		it := e.Iterations
		e.Residual = &residual
		e.Mass = &mass
		e.ResidualIteration = &it
		if cancelled() {
			e.Reason = "CANCELLED"
			return
		}
		if residual <= e.Parameters.Tolerance {
			e.Status = "COMPLETE"
			for i, id := range g.ids {
				e.Scores = append(e.Scores, Value{id, positiveZero(x[i])})
			}
			order := append([]Value{}, e.Scores...)
			sort.Slice(order, func(i, j int) bool {
				if order[i].Score == order[j].Score {
					return order[i].NodeID < order[j].NodeID
				}
				return order[i].Score > order[j].Score
			})
			for _, s := range order {
				e.Ranks = append(e.Ranks, s.NodeID)
			}
			return
		}
		if e.Iterations == e.Parameters.MaxIterations {
			e.Reason = "NOT_CONVERGED"
			return
		}
		x = y
		e.Iterations++
	}
}
func Analyze(ctx context.Context, raw []byte, p Parameters) ([]byte, error) {
	input, err := boundedanalysis.AdmitRetained(raw)
	if err != nil {
		return nil, err
	}
	e, g, err := prepare(raw, input.Tables, p)
	if err != nil {
		return nil, err
	}
	run(ctx, &e, g, -1)
	if ctx.Err() != nil {
		// Canonicalize asynchronous cancellation to the observed work boundary,
		// including cancellation after an empty or converged candidate completed.
		stop := e.Work
		e, g, err = prepare(raw, input.Tables, p)
		if err != nil {
			return nil, err
		}
		run(context.Background(), &e, g, stop)
	}
	e.Digest = seal(e)
	b, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	if len(b)+1 > MaxBytes {
		return nil, fmt.Errorf("ranking output LIMIT")
	}
	return append(b, '\n'), nil
}

// Independent incoming equation: construct destination incidence lists directly
// from admitted tables, not the producer's edge/index/degree kernel.
func prove(e Evidence, t retainedcalls.Tables) error {
	fail := func() error { return fmt.Errorf("independent ranking equation/coverage/rank/mass proof mismatch") }
	if len(e.Scores) != len(t.Endpoints) || len(e.Ranks) != len(t.Endpoints) {
		return fail()
	}
	scores := map[string]float64{}
	degree := map[string]int{}
	incoming := map[string][]retainedcalls.Group{}
	for i, s := range e.Scores {
		if s.NodeID != e.Nodes[i] || !finite(s.Score) || s.Score < 0 || math.Signbit(s.Score) {
			return fail()
		}
		scores[s.NodeID] = s.Score
	}
	for _, g := range t.Groups {
		degree[g.CallerNodeID]++
		incoming[g.CalleeNodeID] = append(incoming[g.CalleeNodeID], g)
	}
	dangling, mass := 0., 0.
	for _, id := range e.Nodes {
		mass = float64(mass + scores[id])
		if degree[id] == 0 {
			dangling = float64(dangling + scores[id])
		}
	}
	residual := 0.
	for i, id := range e.Nodes {
		list := incoming[id]
		sort.Slice(list, func(i, j int) bool { return list[i].RelationID < list[j].RelationID })
		sum := 0.
		for _, g := range list {
			term := float64(scores[g.CallerNodeID] / float64(degree[g.CallerNodeID]))
			sum = float64(sum + term)
		}
		p := e.Personalization[i].Score
		d := float64(dangling * p)
		s := float64(sum + d)
		a := float64(e.Parameters.Alpha * s)
		b := float64(float64(1-e.Parameters.Alpha) * p)
		fx := float64(a + b)
		difference := float64(fx - scores[id])
		residual = float64(residual + math.Abs(difference))
	}
	if e.Residual == nil || e.Mass == nil || !finite(mass) || !finite(residual) || residual != *e.Residual || mass != *e.Mass || residual > e.Parameters.Tolerance {
		return fail()
	}
	n, m := len(e.Nodes), len(t.Groups)
	u := math.Ldexp(1, -53)
	gamma := func(k int) float64 { z := float64(k) * u; return z / (1 - z) }
	delta := gamma(8*n + 4*m + 32)
	bound := (gamma(n+2)+delta)/(1-e.Parameters.Alpha-delta) + gamma(n)
	target := 1.
	if n == 0 {
		target = 0
	}
	if math.Abs(mass-target) > bound {
		return fail()
	}
	ranked := append([]Value{}, e.Scores...)
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Score == ranked[j].Score {
			return ranked[i].NodeID < ranked[j].NodeID
		}
		return ranked[i].Score > ranked[j].Score
	})
	for i, s := range ranked {
		if e.Ranks[i] != s.NodeID {
			return fail()
		}
	}
	return nil
}
func ValidateFor(raw []byte, family, version string) (string, error) {
	if family != Family {
		return boundedmetrics.ValidateFor(raw, family, version)
	}
	if err := boundedanalysis.PreflightArtifact(raw); err != nil {
		return "", err
	}
	if _, err := schema.ValidateStructure(raw, family, version); err != nil {
		return "", err
	}
	var e Evidence
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&e); err != nil {
		return "", err
	}
	if !bytes.Equal(canonical(json.RawMessage(raw)), canonical(e)) {
		return "", fmt.Errorf("exact ranking members required")
	}
	input, err := boundedanalysis.AdmitRetained(e.InputBytes)
	if err != nil {
		return "", err
	}
	expected, g, err := prepare(e.InputBytes, input.Tables, e.Parameters)
	if err != nil {
		return "", err
	}
	// Check basis and personalization BEFORE indexing them in independent proof.
	if !bytes.Equal(canonical(e.Nodes), canonical(expected.Nodes)) || !bytes.Equal(canonical(e.Personalization), canonical(expected.Personalization)) {
		return "", fmt.Errorf("ranking coverage/personalization mismatch")
	}
	if e.Status == "COMPLETE" {
		if err = prove(e, input.Tables); err != nil {
			return "", err
		}
	}
	stop := -1
	if e.Status == "INCOMPLETE" && e.Reason == "CANCELLED" {
		stop = e.Work
	}
	run(context.Background(), &expected, g, stop)
	expected.Digest = seal(expected)
	if !bytes.Equal(canonical(e), canonical(expected)) {
		return "", fmt.Errorf("ranking policy/basis/digest/result/accounting replay mismatch")
	}
	return Version, nil
}
