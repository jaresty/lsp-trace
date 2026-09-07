// Package boundedmetrics measures only admitted historical retained CALLS.
// Completion describes computation over the retained graph, not source coverage.
package boundedmetrics

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"lsp-trace/internal/boundedanalysis"
	"lsp-trace/internal/retainedcalls"
	"lsp-trace/internal/schema"
)

const Family = "bounded-retained-metrics"
const Version = "lsp-trace.bounded-retained-metrics.v1"
const Policy = "retained-CALLS-unit-group-structural-metrics/v1"
const Scope = boundedanalysis.Scope
const Completion = "COMPUTED_OVER_RETAINED_GRAPH"
const DensityPolicy = "DIRECTED_DISTINCT_NONLOOP_PAIRS"
const MaxInputBytes = boundedanalysis.MaxInputBytes
const MaxBytes = boundedanalysis.MaxBytes
const MaxNodes = boundedanalysis.MaxNodes
const MaxGroups = boundedanalysis.MaxEdges

type Parameters struct{}
type Node struct {
	ID                   string `json:"id"`
	InGroupDegree        int    `json:"in_group_degree"`
	OutGroupDegree       int    `json:"out_group_degree"`
	InDistinctNeighbors  int    `json:"in_distinct_neighbors"`
	OutDistinctNeighbors int    `json:"out_distinct_neighbors"`
}
type Bin struct {
	Degree    int `json:"degree"`
	NodeCount int `json:"node_count"`
}
type Rational struct {
	Numerator   int `json:"numerator"`
	Denominator int `json:"denominator"`
}
type Density struct {
	Policy string    `json:"policy"`
	Status string    `json:"status"`
	Value  *Rational `json:"value"`
}
type Evidence struct {
	SchemaVersion            string     `json:"schema_version"`
	Policy                   string     `json:"policy"`
	Scope                    string     `json:"scope"`
	InputBytes               []byte     `json:"input_bytes"`
	Parameters               Parameters `json:"parameters"`
	BasisDigest              string     `json:"basis_digest"`
	Status                   string     `json:"status"`
	NodeCount                int        `json:"node_count"`
	GroupCount               int        `json:"group_count"`
	ReportedOccurrenceCount  int        `json:"reported_occurrence_count"`
	UnreportedGroupCount     int        `json:"unreported_group_count"`
	SelfLoopGroupCount       int        `json:"self_loop_group_count"`
	DistinctNonloopPairCount int        `json:"distinct_nonloop_pair_count"`
	Nodes                    []Node     `json:"nodes"`
	InDegreeHistogram        []Bin      `json:"in_degree_histogram"`
	OutDegreeHistogram       []Bin      `json:"out_degree_histogram"`
	Density                  Density    `json:"density"`
	Digest                   string     `json:"digest"`
}

func canonical(v any) []byte {
	raw, _ := json.Marshal(v)
	var x any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	_ = d.Decode(&x)
	raw, _ = json.Marshal(x)
	return raw
}
func hash(domain string, raw []byte) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256(append(append([]byte(domain), 0), raw...)))
}
func basis(raw []byte) string {
	return hash(Version+":basis", canonical(struct {
		Policy     string     `json:"policy"`
		Input      []byte     `json:"input_bytes"`
		Parameters Parameters `json:"parameters"`
	}{Policy, raw, Parameters{}}))
}
func seal(e Evidence) string { e.Digest = ""; return hash(Version+":result", canonical(e)) }
func histogram(nodes []Node, incoming bool) []Bin {
	counts := map[int]int{}
	for _, n := range nodes {
		d := n.OutGroupDegree
		if incoming {
			d = n.InGroupDegree
		}
		counts[d]++
	}
	bins := make([]Bin, 0, len(counts))
	for d, n := range counts {
		bins = append(bins, Bin{d, n})
	}
	sort.Slice(bins, func(i, j int) bool { return bins[i].Degree < bins[j].Degree })
	return bins
}
func compute(ctx context.Context, raw []byte, t retainedcalls.Tables) (Evidence, error) {
	e := Evidence{SchemaVersion: Version, Policy: Policy, Scope: Scope, InputBytes: append([]byte{}, raw...), BasisDigest: basis(raw), Status: Completion, NodeCount: len(t.Endpoints), GroupCount: len(t.Groups), Nodes: []Node{}}
	index := map[string]int{}
	for _, n := range t.Endpoints {
		if err := ctx.Err(); err != nil {
			return Evidence{}, err
		}
		e.Nodes = append(e.Nodes, Node{ID: n.ID})
	}
	sort.Slice(e.Nodes, func(i, j int) bool { return e.Nodes[i].ID < e.Nodes[j].ID })
	ins, outs := make([]map[string]bool, len(e.Nodes)), make([]map[string]bool, len(e.Nodes))
	for i, n := range e.Nodes {
		index[n.ID] = i
		ins[i] = map[string]bool{}
		outs[i] = map[string]bool{}
	}
	pairs := map[[2]string]bool{}
	for _, g := range t.Groups {
		if err := ctx.Err(); err != nil {
			return Evidence{}, err
		}
		a, b := index[g.CallerNodeID], index[g.CalleeNodeID]
		e.Nodes[a].OutGroupDegree++
		e.Nodes[b].InGroupDegree++
		outs[a][g.CalleeNodeID] = true
		ins[b][g.CallerNodeID] = true
		e.ReportedOccurrenceCount += len(g.OccurrenceIDs)
		if g.CallsiteState == "UNREPORTED" {
			e.UnreportedGroupCount++
		}
		if a == b {
			e.SelfLoopGroupCount++
		} else {
			pairs[[2]string{g.CallerNodeID, g.CalleeNodeID}] = true
		}
	}
	for i := range e.Nodes {
		e.Nodes[i].InDistinctNeighbors = len(ins[i])
		e.Nodes[i].OutDistinctNeighbors = len(outs[i])
	}
	e.DistinctNonloopPairCount = len(pairs)
	e.InDegreeHistogram = histogram(e.Nodes, true)
	e.OutDegreeHistogram = histogram(e.Nodes, false)
	e.Density = Density{Policy: DensityPolicy, Status: "UNDEFINED_DENOMINATOR"}
	if e.NodeCount >= 2 {
		e.Density.Status = "DEFINED"
		e.Density.Value = &Rational{len(pairs), e.NodeCount * (e.NodeCount - 1)}
	}
	if err := ctx.Err(); err != nil {
		return Evidence{}, err
	}
	return e, nil
}

// Analyze returns no artifact on cancellation or limit failure. There are no
// configurable traversal/work parameters: the fixed n/m bounds bound all work.
func Analyze(ctx context.Context, raw []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	input, err := boundedanalysis.AdmitRetained(raw)
	if err != nil {
		return nil, err
	}
	e, err := compute(ctx, raw, input.Tables)
	if err != nil {
		return nil, err
	}
	e.Digest = seal(e)
	out, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	if len(out)+1 > MaxBytes {
		return nil, errors.New("bounded metrics output LIMIT")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}
func ValidateFor(raw []byte, family, version string) (string, error) {
	if family != Family {
		return boundedanalysis.ValidateFor(raw, family, version)
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
		return "", errors.New("exact metrics members required recursively")
	}
	input, err := boundedanalysis.AdmitRetained(e.InputBytes)
	if err != nil {
		return "", err
	}
	if err = prove(e, input.Tables); err != nil {
		return "", err
	}
	expected, err := compute(context.Background(), e.InputBytes, input.Tables)
	if err != nil {
		return "", err
	}
	expected.Digest = seal(expected)
	if !bytes.Equal(canonical(e), canonical(expected)) {
		return "", errors.New("metrics policy/basis/digest/order/status replay mismatch")
	}
	return Version, nil
}

// prove independently derives incidence runs from retained tables, not producer
// node accumulators. Sorted lists (rather than hash neighbor sets) prove degrees,
// neighbors, ordered-pair density and histograms. The occurrence table is the
// independent denominator; admission already proves each witness's unique join.
func prove(e Evidence, t retainedcalls.Tables) error {
	fail := func() error { return errors.New("independent retained-table metrics proof mismatch") }
	if e.NodeCount != len(t.Endpoints) || e.GroupCount != len(t.Groups) || e.ReportedOccurrenceCount != len(t.Occurrences) || len(e.Nodes) != len(t.Endpoints) {
		return fail()
	}
	type incidence struct{ node, neighbor string }
	in, out := []incidence{}, []incidence{}
	pairs := [][2]string{}
	loops, unreported, witnesses := 0, 0, 0
	for _, g := range t.Groups {
		in = append(in, incidence{g.CalleeNodeID, g.CallerNodeID})
		out = append(out, incidence{g.CallerNodeID, g.CalleeNodeID})
		witnesses += len(g.OccurrenceIDs)
		if len(g.OccurrenceIDs) == 0 {
			unreported++
		}
		if g.CallerNodeID == g.CalleeNodeID {
			loops++
		} else {
			pairs = append(pairs, [2]string{g.CallerNodeID, g.CalleeNodeID})
		}
	}
	if witnesses != len(t.Occurrences) || e.SelfLoopGroupCount != loops || e.UnreportedGroupCount != unreported {
		return fail()
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i][0] != pairs[j][0] {
			return pairs[i][0] < pairs[j][0]
		}
		return pairs[i][1] < pairs[j][1]
	})
	q := 0
	for i, p := range pairs {
		if i == 0 || p != pairs[i-1] {
			q++
		}
	}
	if e.DistinctNonloopPairCount != q || e.Density.Policy != DensityPolicy {
		return fail()
	}
	if len(t.Endpoints) < 2 {
		if e.Density.Status != "UNDEFINED_DENOMINATOR" || e.Density.Value != nil {
			return fail()
		}
	} else if e.Density.Status != "DEFINED" || e.Density.Value == nil || *e.Density.Value != (Rational{q, len(t.Endpoints) * (len(t.Endpoints) - 1)}) {
		return fail()
	}
	ids := []string{}
	for _, n := range t.Endpoints {
		ids = append(ids, n.ID)
	}
	sort.Strings(ids)
	for i, id := range ids {
		if e.Nodes[i].ID != id {
			return fail()
		}
	}
	for pass, list := range [][]incidence{in, out} {
		sort.Slice(list, func(i, j int) bool {
			if list[i].node != list[j].node {
				return list[i].node < list[j].node
			}
			return list[i].neighbor < list[j].neighbor
		})
		degrees := make([]int, len(ids))
		h, total := 0, 0
		for i, id := range ids {
			start, neighbors := h, 0
			for h < len(list) && list[h].node == id {
				if h == start || list[h].neighbor != list[h-1].neighbor {
					neighbors++
				}
				h++
			}
			degree := h - start
			degrees[i] = degree
			total += degree
			gotD, gotN := e.Nodes[i].InGroupDegree, e.Nodes[i].InDistinctNeighbors
			if pass == 1 {
				gotD, gotN = e.Nodes[i].OutGroupDegree, e.Nodes[i].OutDistinctNeighbors
			}
			if gotD != degree || gotN != neighbors {
				return fail()
			}
		}
		if h != len(list) || total != len(t.Groups) {
			return fail()
		}
		sort.Ints(degrees)
		bins := []Bin{}
		for _, d := range degrees {
			if len(bins) == 0 || bins[len(bins)-1].Degree != d {
				bins = append(bins, Bin{Degree: d})
			}
			bins[len(bins)-1].NodeCount++
		}
		got := e.InDegreeHistogram
		if pass == 1 {
			got = e.OutDegreeHistogram
		}
		if !bytes.Equal(canonical(got), canonical(bins)) {
			return fail()
		}
	}
	return nil
}
