// Package gonumlouvain is a private Program C qualification-only implementation.
package gonumlouvain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"sort"

	"gonum.org/v1/gonum/graph/community"
	"gonum.org/v1/gonum/graph/simple"
)

const (
	ProfileName          = "calls-v1"
	CandidateVersion     = "v0.17.0"
	GonumModuleSum       = "h1:XKuoEMdKHiUPRL0QhxePqGI7Mb6r7+elIvbVqMpvmTI="
	RetainedExportSHA256 = "sha256:b7f3da235f18aa0ddd66f6a1184e84ea066e4244940f377c02598e8adec52011"
	MaxNodes             = 10_000
	MaxEdges             = 100_000
	Runs                 = 3
	Permutations         = 1
)

type Failure struct {
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

func (e *Failure) Error() string     { return e.Kind + ": " + e.Detail }
func Fail(kind, detail string) error { return &Failure{Kind: kind, Detail: detail} }
func FailureKind(err error) string {
	var f *Failure
	if errors.As(err, &f) {
		return f.Kind
	}
	return "NONZERO_EXIT"
}

type Edge struct {
	From        string `json:"from"`
	To          string `json:"to"`
	Occurrences int    `json:"occurrences"`
}
type Fixture struct {
	// Legacy retained-export fields remain for predecessor tests and receipts.
	SourceDigest string   `json:"source_digest,omitempty"`
	Nodes        []string `json:"nodes,omitempty"`
	Edges        []Edge   `json:"edges,omitempty"`
	// Gate II successor requests carry exact graph bytes plus their bound counts.
	ID          string `json:"id,omitempty"`
	GraphSHA256 string `json:"graph_sha256,omitempty"`
	GraphBytes  string `json:"graph_bytes,omitempty"`
	NodeCount   int    `json:"node_count,omitempty"`
	EdgeCount   int    `json:"directed_weighted_edge_count,omitempty"`
}
type RawOutput struct {
	Communities [][]string `json:"communities"`
}
type Output struct {
	Communities [][]string `json:"communities"`
	Digest      string     `json:"digest"`
}
type Request struct {
	Fixture          Fixture `json:"fixture"`
	Seed             uint64  `json:"seed"`
	ReverseInsertion bool    `json:"reverse_insertion"`
}

func Run(req Request) (out RawOutput, err error) {
	defer func() {
		if v := recover(); v != nil {
			err = Fail("PANIC", fmt.Sprint(v))
		}
	}()
	f := req.Fixture
	sourceNodes := f.Nodes
	sourceEdges := f.Edges
	if f.GraphBytes != "" {
		graph, graphErr := parseGraphFixture(f)
		if graphErr != nil {
			return out, graphErr
		}
		sourceNodes = graph.Nodes
		sourceEdges = make([]Edge, len(graph.Edges))
		for i, edge := range graph.Edges {
			sourceEdges[i] = Edge{From: edge.From, To: edge.To, Occurrences: edge.Weight}
		}
	} else if f.SourceDigest != RetainedExportSHA256 {
		return out, Fail("INPUT_DIGEST_MISMATCH", f.SourceDigest)
	}
	if len(sourceNodes) == 0 {
		return out, Fail("DERIVATION_REJECTED", "no nodes")
	}
	if len(sourceNodes) > MaxNodes || len(sourceEdges) > MaxEdges {
		return out, Fail("CAP_BREACH", "fixture cardinality")
	}
	nodes := append([]string(nil), sourceNodes...)
	sort.Strings(nodes)
	for i := 1; i < len(nodes); i++ {
		if nodes[i] == nodes[i-1] {
			return out, Fail("DERIVATION_REJECTED", "duplicate node")
		}
	}
	ids := map[string]int64{}
	for i, n := range nodes {
		ids[n] = int64(i + 1)
	}
	order := append([]string(nil), sourceNodes...)
	edges := append([]Edge(nil), sourceEdges...)
	if req.ReverseInsertion {
		for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
			order[i], order[j] = order[j], order[i]
		}
		for i, j := 0, len(edges)-1; i < j; i, j = i+1, j-1 {
			edges[i], edges[j] = edges[j], edges[i]
		}
	}
	g := simple.NewWeightedDirectedGraph(0, 0)
	for _, n := range order {
		g.AddNode(simple.Node(ids[n]))
	}
	seen := map[[2]int64]bool{}
	for _, e := range edges {
		a, aok := ids[e.From]
		b, bok := ids[e.To]
		if !aok || !bok || e.Occurrences <= 0 || seen[[2]int64{a, b}] {
			return out, Fail("DERIVATION_REJECTED", "invalid or duplicate directed adjacency")
		}
		seen[[2]int64{a, b}] = true
		g.SetWeightedEdge(g.NewWeightedEdge(g.Node(a), g.Node(b), float64(e.Occurrences)))
	}
	cs := community.Modularize(g, 1, rand.NewPCG(req.Seed, req.Seed^0x9e3779b97f4a7c15)).Communities()
	out.Communities = make([][]string, len(cs))
	for i, c := range cs {
		for _, n := range c {
			out.Communities[i] = append(out.Communities[i], nodes[n.ID()-1])
		}
	}
	return out, nil
}

func Canonicalize(rawOutput RawOutput, expectedNodes []string) (Output, error) {
	out := Output{Communities: make([][]string, len(rawOutput.Communities))}
	expected := make(map[string]bool, len(expectedNodes))
	for _, node := range expectedNodes {
		if node == "" || expected[node] {
			return Output{}, Fail("NONCANONICAL_OUTPUT", "invalid expected node inventory")
		}
		expected[node] = true
	}
	seen := make(map[string]bool, len(expectedNodes))
	for i, community := range rawOutput.Communities {
		if len(community) == 0 {
			return Output{}, Fail("NONCANONICAL_OUTPUT", "empty community")
		}
		out.Communities[i] = append([]string(nil), community...)
		for _, node := range community {
			if !expected[node] || seen[node] {
				return Output{}, Fail("NONCANONICAL_OUTPUT", "unknown or duplicate community member")
			}
			seen[node] = true
		}
		sort.Strings(out.Communities[i])
	}
	if len(seen) != len(expected) {
		return Output{}, Fail("NONCANONICAL_OUTPUT", "incomplete community partition")
	}
	sort.Slice(out.Communities, func(i, j int) bool { return communityLess(out.Communities[i], out.Communities[j]) })
	canonical, _ := json.Marshal(out.Communities)
	s := sha256.Sum256(canonical)
	out.Digest = "sha256:" + hex.EncodeToString(s[:])
	return out, nil
}

func ValidateCanonical(o Output) error {
	for i, c := range o.Communities {
		if !sort.StringsAreSorted(c) {
			return Fail("NONCANONICAL_OUTPUT", fmt.Sprintf("community %d", i))
		}
		if i > 0 && !communityLess(o.Communities[i-1], c) {
			return Fail("NONCANONICAL_OUTPUT", "community order")
		}
	}
	raw, _ := json.Marshal(o.Communities)
	s := sha256.Sum256(raw)
	if o.Digest != "sha256:"+hex.EncodeToString(s[:]) {
		return Fail("NONCANONICAL_OUTPUT", "digest")
	}
	return nil
}
func communityLess(a, b []string) bool {
	for k := 0; k < len(a) && k < len(b); k++ {
		if a[k] != b[k] {
			return a[k] < b[k]
		}
	}
	return len(a) < len(b)
}
func ValidateSchedule(results []SupervisedResult) error {
	if len(results) != Runs+Permutations {
		return Fail("SCHEDULE_INVALID", fmt.Sprint(len(results)))
	}
	for i, r := range results {
		if err := ValidateCanonical(r.Output); err != nil {
			return err
		}
		if i > 0 && r.Output.Digest != results[0].Output.Digest {
			return Fail("NONDETERMINISM", fmt.Sprintf("run 0 != run %d", i))
		}
	}
	return nil
}
