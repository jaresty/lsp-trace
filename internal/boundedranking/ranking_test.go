package boundedranking

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	tracegraph "lsp-trace/internal/graph"
	"lsp-trace/internal/retainedcalls"
	"math"
	"math/big"
	"math/rand"
	"testing"
)

func table(n int, edges [][2]int) retainedcalls.Tables {
	t := retainedcalls.Tables{}
	for i := 0; i < n; i++ {
		t.Endpoints = append(t.Endpoints, tracegraph.Node{ID: fmt.Sprint(i)})
	}
	for i, e := range edges {
		t.Groups = append(t.Groups, retainedcalls.Group{RelationID: fmt.Sprintf("%04d", i), CallerNodeID: fmt.Sprint(e[0]), CalleeNodeID: fmt.Sprint(e[1])})
	}
	return t
}
func calc(t *testing.T, tab retainedcalls.Tables, p Parameters) Evidence {
	t.Helper()
	e, g, err := prepare([]byte("table-only-not-admitted"), tab, p)
	if err != nil {
		t.Fatal(err)
	}
	run(context.Background(), &e, g, -1)
	return e
}

// Exact rational Gaussian elimination solves (I-alpha*T)x=(1-alpha)*p.
// This test-only dense matrix is independent of the iterative producer and proof.
func rationalOracle(n int, edges [][2]int, seeds []Seed) []float64 {
	if n == 0 {
		return nil
	}
	p := make([]*big.Rat, n)
	total := int64(0)
	for _, s := range seeds {
		total += s.Weight
	}
	for i := range p {
		p[i] = new(big.Rat)
		if len(seeds) == 0 {
			p[i].SetFrac64(1, int64(n))
		} else {
			for _, s := range seeds {
				if s.NodeID == fmt.Sprint(i) {
					p[i].SetFrac64(s.Weight, total)
				}
			}
		}
	}
	deg := make([]int, n)
	for _, e := range edges {
		deg[e[0]]++
	}
	a := big.NewRat(17, 20)
	one := big.NewRat(1, 1)
	m := make([][]*big.Rat, n)
	for i := range m {
		m[i] = make([]*big.Rat, n+1)
		for j := range m[i] {
			m[i][j] = new(big.Rat)
		}
		m[i][i].SetInt64(1)
		m[i][n].Mul(new(big.Rat).Sub(one, a), p[i])
	}
	for u := 0; u < n; u++ {
		if deg[u] == 0 {
			for v := 0; v < n; v++ {
				m[v][u].Sub(m[v][u], new(big.Rat).Mul(a, p[v]))
			}
		}
	}
	for _, e := range edges {
		m[e[1]][e[0]].Sub(m[e[1]][e[0]], new(big.Rat).Quo(a, big.NewRat(int64(deg[e[0]]), 1)))
	}
	for k := 0; k < n; k++ {
		pivot := new(big.Rat).Set(m[k][k])
		for j := k; j <= n; j++ {
			m[k][j].Quo(m[k][j], pivot)
		}
		for i := 0; i < n; i++ {
			if i == k {
				continue
			}
			factor := new(big.Rat).Set(m[i][k])
			for j := k; j <= n; j++ {
				m[i][j].Sub(m[i][j], new(big.Rat).Mul(factor, m[k][j]))
			}
		}
	}
	out := make([]float64, n)
	for i := range out {
		out[i], _ = m[i][n].Float64()
	}
	return out
}
func TestRankingRationalAllThreeNodeGraphs(t *testing.T) {
	for mask := 0; mask < 512; mask++ {
		edges := [][2]int{}
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				if mask&(1<<(i*3+j)) != 0 {
					edges = append(edges, [2]int{i, j})
				}
			}
		}
		for _, personal := range []bool{false, true} {
			p := Defaults("PAGERANK")
			p.Tolerance = 1e-12
			if personal {
				p.Algorithm = "PPR"
				p.Seeds = []Seed{{"0", 2}, {"2", 1}}
			}
			tab := table(3, edges)
			e := calc(t, tab, p)
			if e.Status != "COMPLETE" {
				t.Fatal(mask, e)
			}
			oracle := rationalOracle(3, edges, p.Seeds)
			for i, s := range e.Scores {
				if math.Abs(s.Score-oracle[i]) > p.Tolerance/(1-p.Alpha)+2e-15 {
					t.Fatalf("rational disagreement mask=%d PPR=%v node=%d got=%.17g want=%.17g", mask, personal, i, s.Score, oracle[i])
				}
			}
			if err := prove(e, tab); err != nil {
				t.Fatal(mask, err)
			}
		}
	}
}
func TestRankingFixedVectorsParallelScalingPermutation(t *testing.T) {
	// Rational closed form for 0->1 with dangling 1: (20/57,37/57).
	p := Defaults("PAGERANK")
	p.Tolerance = 1e-12
	e := calc(t, table(2, [][2]int{{0, 1}}), p)
	// Independent Python binary64 scalar recurrence, retained 2026-09-07.
	if e.Iterations != 32 || e.Work != 231 || *e.Residual != 5.457301277544957e-13 || *e.Mass != 1 || e.Scores[0].Score != 0.35087719298264763 || e.Scores[1].Score != 0.6491228070173525 {
		t.Fatal("fixed independent binary64 vector", e)
	}
	for i, want := range []float64{20. / 57, 37. / 57} {
		if math.Abs(e.Scores[i].Score-want) > 1e-12 {
			t.Fatal(e)
		}
	}
	tab := table(4, [][2]int{{0, 1}, {0, 1}, {0, 2}, {1, 1}, {2, 0}})
	p = Defaults("PPR")
	p.Seeds = []Seed{{"0", 2}, {"3", 3}}
	e = calc(t, tab, p)
	want := rationalOracle(4, [][2]int{{0, 1}, {0, 1}, {0, 2}, {1, 1}, {2, 0}}, p.Seeds)
	for i, s := range e.Scores {
		if math.Abs(s.Score-want[i]) > 1e-8 {
			t.Fatal(e, want)
		}
	}
	scaled := p
	scaled.Seeds = []Seed{{"3", 21}, {"0", 14}}
	f := calc(t, tab, scaled)
	if !bytes.Equal(canonical(e.Scores), canonical(f.Scores)) || e.BasisDigest == f.BasisDigest {
		t.Fatal("seed scaling")
	}
	r := rand.New(rand.NewSource(7))
	r.Shuffle(len(tab.Endpoints), func(i, j int) { tab.Endpoints[i], tab.Endpoints[j] = tab.Endpoints[j], tab.Endpoints[i] })
	r.Shuffle(len(tab.Groups), func(i, j int) { tab.Groups[i], tab.Groups[j] = tab.Groups[j], tab.Groups[i] })
	f = calc(t, tab, p)
	if !bytes.Equal(canonical(e), canonical(f)) {
		t.Fatal("permutation")
	}
	for _, n := range []int{0, 1, 3} {
		e = calc(t, table(n, nil), Defaults("PAGERANK"))
		if e.Status != "COMPLETE" || e.Iterations != 0 || len(e.Scores) != n || *e.Residual != 0 {
			t.Fatal("empty/isolate", e)
		}
	}
}
func TestRankingWorkIterationCancel(t *testing.T) {
	tab := table(2, [][2]int{{0, 1}})
	p := Defaults("PAGERANK")
	p.MaxIterations = 1
	e := calc(t, tab, p)
	if e.Reason != "NOT_CONVERGED" || e.Work != 14 || e.Iterations != 1 || *e.ResidualIteration != 1 || len(e.Scores) != 0 {
		t.Fatal(e)
	}
	p.MaxWork = 13
	e = calc(t, tab, p)
	if e.Reason != "LIMIT" || e.Work != 13 || *e.ResidualIteration != 0 || len(e.Ranks) != 0 {
		t.Fatal(e)
	}
	p.MaxWork = 6
	e = calc(t, tab, p)
	if e.Residual != nil || e.Mass != nil {
		t.Fatal(e)
	}
	p = Defaults("PAGERANK")
	for stop := 0; stop < 30; stop++ {
		e, g, err := prepare(nil, tab, p)
		if err != nil {
			t.Fatal(err)
		}
		run(context.Background(), &e, g, stop)
		if e.Reason != "CANCELLED" || e.Work != stop || len(e.Scores) != 0 {
			t.Fatal(stop, e)
		}
	}
	p.MaxWork = 6
	e = calc(t, table(2, nil), p)
	if e.Status != "COMPLETE" || e.Work != 6 {
		t.Fatal("exact budget convergence", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	e, g, _ := prepare(nil, table(0, nil), p)
	run(ctx, &e, g, -1)
	if e.Reason != "CANCELLED" || e.Residual != nil {
		t.Fatal(e)
	}
}

type countedCancellation struct {
	context.Context
	remaining int
}

func (c *countedCancellation) Err() error {
	c.remaining--
	if c.remaining <= 0 {
		return context.Canceled
	}
	return nil
}
func TestRankingMaximumTopologyBudgetIsHonest(t *testing.T) {
	edges := [][2]int{{0, 0}, {0, 0}}
	for i := 0; i < 4095; i++ {
		edges = append(edges, [2]int{i, i + 1}, [2]int{i, i + 1})
	}
	p := Defaults("PPR")
	p.Alpha = .99
	p.Seeds = []Seed{{"0", 1}}
	e := calc(t, table(4096, edges), p)
	if e.Status != "INCOMPLETE" || e.Reason != "LIMIT" || e.Work != 1000000 || len(e.Scores) != 0 || e.Residual == nil {
		t.Fatal("default work is not global convergence", e.Status, e.Reason, e.Work)
	}
}
func TestRankingCancellationBoundaryReplay(t *testing.T) {
	tab := table(2, [][2]int{{0, 1}})
	p := Defaults("PAGERANK")
	for at := 1; at < 90; at++ {
		e, g, _ := prepare(nil, tab, p)
		run(&countedCancellation{context.Background(), at}, &e, g, -1)
		f, _, _ := prepare(nil, tab, p)
		run(context.Background(), &f, g, e.Work)
		if !bytes.Equal(canonical(e), canonical(f)) {
			t.Fatalf("cancellation at %d work=%d: %s != %s", at, e.Work, canonical(e), canonical(f))
		}
	}
}

func TestRankingRejectParameters(t *testing.T) {
	base := Defaults("PAGERANK")
	bad := []Parameters{}
	for _, x := range []float64{0, -1, 1, math.NaN(), math.Inf(1)} {
		p := base
		p.Alpha = x
		bad = append(bad, p)
	}
	for _, x := range []float64{0, 1e-13, .01, math.NaN(), math.Inf(1)} {
		p := base
		p.Tolerance = x
		bad = append(bad, p)
	}
	for _, x := range []int{0, -1, 10001} {
		p := base
		p.MaxIterations = x
		bad = append(bad, p)
	}
	for _, x := range []int{0, -1, 1000001} {
		p := base
		p.MaxWork = x
		bad = append(bad, p)
	}
	for _, s := range [][]Seed{nil, {{"missing", 1}}, {{"0", 0}}, {{"0", 1000001}}, {{"0", 1}, {"0", 2}}} {
		p := Defaults("PPR")
		p.Seeds = s
		bad = append(bad, p)
	}
	for _, p := range bad {
		if _, _, err := normalize(p, []string{"0", "1"}); err == nil {
			t.Fatal(p)
		}
	}
	if _, _, err := normalize(Defaults("PPR"), nil); err == nil {
		t.Fatal("empty PPR")
	}
}
func TestRankingProofRejectCoherentMutations(t *testing.T) {
	tab := table(3, [][2]int{{0, 1}, {1, 1}})
	e := calc(t, tab, Defaults("PAGERANK"))
	for _, mutate := range []func(*Evidence){func(e *Evidence) { e.Scores[0].Score += .01 }, func(e *Evidence) { e.Ranks[0], e.Ranks[1] = e.Ranks[1], e.Ranks[0] }, func(e *Evidence) { *e.Residual = 0 }, func(e *Evidence) { *e.Mass = 2 }} {
		var f Evidence
		_ = json.Unmarshal(canonical(e), &f)
		mutate(&f)
		f.Digest = seal(f)
		if prove(f, tab) == nil {
			t.Fatal("forged proof admitted")
		}
	}
}
