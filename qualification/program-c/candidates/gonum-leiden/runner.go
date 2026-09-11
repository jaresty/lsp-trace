// Package gonumleiden is a private Program C qualification-only implementation.
package gonumleiden

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
	CandidateRevision    = "69ca49f456a7a38cf370131834a2178d9aae17fe"
	CandidateVersion     = "v0.17.1-0.20260426204603-69ca49f456a7"
	GonumModuleSum       = "h1:V43GU8qUQ/EYbEPGzTP0PANTR6RiJgEJdU/vgsFhQiU="
	GonumGoModSum        = "h1:El3tOrEuMpv2UdMrbNlKEh9vd86bmQ6vqIcDwxEOc1E="
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
	SourceDigest string   `json:"source_digest"`
	Nodes        []string `json:"nodes"`
	Edges        []Edge   `json:"edges"`
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

func Run(req Request) (out Output, err error) {
	defer func() {
		if v := recover(); v != nil {
			err = Fail("PANIC", fmt.Sprint(v))
		}
	}()
	f := req.Fixture
	if f.SourceDigest != RetainedExportSHA256 {
		return out, Fail("INPUT_DIGEST_MISMATCH", f.SourceDigest)
	}
	if len(f.Nodes) == 0 {
		return out, Fail("DERIVATION_REJECTED", "no nodes")
	}
	if len(f.Nodes) > MaxNodes || len(f.Edges) > MaxEdges {
		return out, Fail("CAP_BREACH", "fixture cardinality")
	}
	nodes := append([]string(nil), f.Nodes...)
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
	order := append([]string(nil), nodes...)
	if req.ReverseInsertion {
		for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
			order[i], order[j] = order[j], order[i]
		}
	}
	g := simple.NewWeightedDirectedGraph(0, 0)
	for _, n := range order {
		g.AddNode(simple.Node(ids[n]))
	}
	seen := map[[2]int64]bool{}
	for _, e := range f.Edges {
		a, aok := ids[e.From]
		b, bok := ids[e.To]
		if !aok || !bok || e.Occurrences <= 0 || seen[[2]int64{a, b}] {
			return out, Fail("DERIVATION_REJECTED", "invalid or duplicate directed adjacency")
		}
		seen[[2]int64{a, b}] = true
		g.SetWeightedEdge(g.NewWeightedEdge(g.Node(a), g.Node(b), float64(e.Occurrences)))
	}
	cs := community.Leiden(g, 1, rand.NewPCG(req.Seed, req.Seed^0x9e3779b97f4a7c15)).Communities()
	out.Communities = make([][]string, len(cs))
	for i, c := range cs {
		for _, n := range c {
			out.Communities[i] = append(out.Communities[i], nodes[n.ID()-1])
		}
		sort.Strings(out.Communities[i])
	}
	sort.Slice(out.Communities, func(i, j int) bool {
		a, b := out.Communities[i], out.Communities[j]
		for k := 0; k < len(a) && k < len(b); k++ {
			if a[k] != b[k] {
				return a[k] < b[k]
			}
		}
		return len(a) < len(b)
	})
	raw, _ := json.Marshal(out.Communities)
	s := sha256.Sum256(raw)
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
