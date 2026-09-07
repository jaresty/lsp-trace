package boundedmetrics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"lsp-trace/internal/boundedanalysis"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/retainedcalls"
	"lsp-trace/internal/schema"
	"math/rand"
	"strings"
	"testing"
)

// Synthetic table-only algorithm fixtures deliberately do not claim admission.
func table(n int, pairs [][2]int) retainedcalls.Tables {
	t := retainedcalls.Tables{Endpoints: []graph.Node{}, Groups: []retainedcalls.Group{}, Occurrences: []retainedcalls.Occurrence{}}
	for i := 0; i < n; i++ {
		t.Endpoints = append(t.Endpoints, graph.Node{ID: fmt.Sprint(i)})
	}
	for i, p := range pairs {
		t.Groups = append(t.Groups, retainedcalls.Group{RelationID: fmt.Sprint(i), CallerNodeID: fmt.Sprint(p[0]), CalleeNodeID: fmt.Sprint(p[1]), CallsiteState: "UNREPORTED", OccurrenceIDs: []string{}})
	}
	return t
}
func TestMetricsAllThreeNodeTopologies(t *testing.T) {
	// Independent adjacency-matrix oracle, including every self-loop combination.
	for mask := 0; mask < 512; mask++ {
		pairs := [][2]int{}
		var matrix [3][3]int
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				if mask&(1<<(i*3+j)) != 0 {
					pairs = append(pairs, [2]int{i, j})
					matrix[i][j] = 1
				}
			}
		}
		tab := table(3, pairs)
		e, err := compute(context.Background(), []byte("fixed"), tab)
		if err != nil {
			t.Fatal(err)
		}
		loops, q := 0, 0
		for i := 0; i < 3; i++ {
			in, out := 0, 0
			for j := 0; j < 3; j++ {
				in += matrix[j][i]
				out += matrix[i][j]
				if i != j {
					q += matrix[i][j]
				}
			}
			loops += matrix[i][i]
			if e.Nodes[i].InGroupDegree != in || e.Nodes[i].OutGroupDegree != out || e.Nodes[i].InDistinctNeighbors != in || e.Nodes[i].OutDistinctNeighbors != out {
				t.Fatal("ASSERT_MATRIX_DEGREES", mask, e)
			}
		}
		if e.SelfLoopGroupCount != loops || e.DistinctNonloopPairCount != q || e.Density.Value == nil || *e.Density.Value != (Rational{q, 6}) {
			t.Fatal("ASSERT_MATRIX_DENSITY", mask, e)
		}
		if err = prove(e, tab); err != nil {
			t.Fatal("ASSERT_INDEPENDENT_TABLE_PROOF", mask, err)
		}
	}
}
func TestMetricsEmptyIsolateLoopsParallelTwoSitesDense(t *testing.T) {
	fixtures := []struct {
		name string
		n    int
		p    [][2]int
	}{{"empty", 0, nil}, {"isolate", 1, nil}, {"zero-density", 2, nil}, {"loop", 1, [][2]int{{0, 0}}}, {"parallel-two-sites-unreported-isolate", 3, [][2]int{{0, 1}, {0, 1}, {1, 1}}}, {"dense", 4, nil}}
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			if f.name == "dense" {
				for i := 0; i < f.n; i++ {
					for j := 0; j < f.n; j++ {
						f.p = append(f.p, [2]int{i, j})
					}
				}
			}
			tab := table(f.n, f.p)
			if f.name == "parallel-two-sites-unreported-isolate" {
				tab.Groups[0].OccurrenceIDs = []string{"o1", "o2"}
				tab.Groups[0].CallsiteState = "RETAINED_DISTINCT_RANGES"
				tab.Occurrences = []retainedcalls.Occurrence{{ID: "o1"}, {ID: "o2"}}
			}
			e, err := compute(context.Background(), []byte("fixed"), tab)
			if err != nil {
				t.Fatal(err)
			}
			if err = prove(e, tab); err != nil {
				t.Fatal(err)
			}
			if f.n < 2 {
				if e.Density.Value != nil || e.Density.Status != "UNDEFINED_DENOMINATOR" {
					t.Fatal("ASSERT_DENSITY_UNDEFINED", e)
				}
			} else if e.Density.Value == nil {
				t.Fatal("ASSERT_DENSITY_DEFINED")
			}
			if f.name == "zero-density" && *e.Density.Value != (Rational{0, 2}) {
				t.Fatal("ASSERT_ZERO_NOT_UNDEFINED")
			}
			if f.name == "parallel-two-sites-unreported-isolate" {
				if e.ReportedOccurrenceCount != 2 || e.UnreportedGroupCount != 2 || e.GroupCount != 3 || e.Nodes[0].OutGroupDegree != 2 || e.Nodes[0].OutDistinctNeighbors != 1 || e.Nodes[1].InGroupDegree != 3 || e.Nodes[1].InDistinctNeighbors != 2 || e.Nodes[2] != (Node{ID: "2"}) {
					t.Fatal("ASSERT_PARALLEL_LOOP_OCCURRENCES", e)
				}
			}
			for _, h := range [][]Bin{e.InDegreeHistogram, e.OutDegreeHistogram} {
				count, sum := 0, 0
				for _, b := range h {
					count += b.NodeCount
					sum += b.Degree * b.NodeCount
				}
				if count != f.n || sum != len(f.p) {
					t.Fatal("ASSERT_HISTOGRAM_IDENTITIES", h)
				}
			}
			for trial := 0; trial < 12; trial++ {
				rng := rand.New(rand.NewSource(int64(trial)))
				rng.Shuffle(len(tab.Endpoints), func(i, j int) { tab.Endpoints[i], tab.Endpoints[j] = tab.Endpoints[j], tab.Endpoints[i] })
				rng.Shuffle(len(tab.Groups), func(i, j int) { tab.Groups[i], tab.Groups[j] = tab.Groups[j], tab.Groups[i] })
				next, err := compute(context.Background(), []byte("fixed"), tab)
				if err != nil || !bytes.Equal(canonical(e), canonical(next)) {
					t.Fatal("ASSERT_PERMUTATION_DETERMINISM", err)
				}
			}
		})
	}
}

// Admitted fixture uses historical graph/provenance/export producers, with fixed
// absent source paths. It is intentionally distinct from the matrix/table oracle.
func fixture(t *testing.T, n int, pairs [][2]int) []byte {
	t.Helper()
	r := graph.Result{SchemaVersion: graph.SchemaVersionV3, Nodes: []graph.Node{}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("N%d", i)
		uri := "file:///__lsp_trace_metrics_absent__/" + name + ".go"
		r.Nodes = append(r.Nodes, graph.NewNode(graph.Item{Name: name, URI: uri, Kind: 12, Range: graph.Range{End: graph.Position{Line: 10}}, SelectionRange: graph.Range{End: graph.Position{Character: 1}}}))
	}
	for i, p := range pairs {
		sites := []graph.Range{}
		if i == 0 {
			sites = []graph.Range{{Start: graph.Position{Line: 1}, End: graph.Position{Line: 1, Character: 1}}, {Start: graph.Position{Line: 2}, End: graph.Position{Line: 2, Character: 1}}}
		}
		r.Edges = graph.MergeEdge(r.Edges, graph.Edge{CallerNodeID: r.Nodes[p[0]].ID, CalleeNodeID: r.Nodes[p[1]].ID, CallSites: sites})
	}
	seed := "file:///__lsp_trace_metrics_absent__/N0.go"
	ids, rels, start := []string{}, []string{}, []string{}
	for _, node := range r.Nodes {
		ids = append(ids, node.ID)
	}
	for _, edge := range r.Edges {
		rels = append(rels, edge.RelationID)
	}
	if n > 0 {
		start = ids[:1]
	}
	r.Targets = start
	r.Invocation.Target = graph.Target{URI: seed}
	r.Invocation.Seeds = []graph.InvocationSeed{{Label: "start", At: seed + ":0:0", ResolvedURI: seed}}
	r.Seeds = []graph.SeedResult{{Label: "start", Requested: r.Invocation.Target, PreparedTargetIDs: start, ReachedNodeIDs: ids, ReachedRelationIDs: rels, ReachedEdges: r.Edges}}
	layers := []graph.SliceLayer{}
	if n > 0 {
		layers = append(layers, graph.SliceLayer{Depth: 0, NodeIDs: start})
	}
	rest := []string{}
	if n > 1 {
		rest = ids[1:]
		layers = append(layers, graph.SliceLayer{Depth: 1, NodeIDs: rest})
	}
	r.Slice = &graph.SliceEvidence{StartMode: "at", SourceURI: seed, DownDepth: 1, UpDepth: 1, StartingNodeIDs: start, Layers: layers, FrontierNodeIDs: rest, UpwardStartNodeIDs: rest, OutgoingRelationIDs: rels}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	p, err := graphprovenance.Capture(context.Background(), raw, "/__lsp_trace_metrics_absent__", seed, "metrics-fixed", 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = retainedcalls.Export(p)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func admitted(t *testing.T, raw []byte) Evidence {
	t.Helper()
	out, err := Analyze(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ValidateFor(out, Family, "v1"); err != nil {
		t.Fatal(err)
	}
	var e Evidence
	if err = json.Unmarshal(out, &e); err != nil {
		t.Fatal(err)
	}
	return e
}
func TestMetricsAdmittedFixturesAndResealedMutations(t *testing.T) {
	for _, n := range []int{0, 1, 2} {
		raw := fixture(t, n, nil)
		e := admitted(t, raw)
		if e.NodeCount != n || e.GroupCount != 0 {
			t.Fatal("ASSERT_ADMITTED_EMPTY_ISOLATE", e)
		}
	}
	raw := fixture(t, 3, [][2]int{{0, 1}, {1, 1}})
	base := admitted(t, raw)
	for name, mutate := range map[string]func(*Evidence){
		"node-count": func(e *Evidence) { e.NodeCount++ }, "group-count": func(e *Evidence) { e.GroupCount++ }, "reported": func(e *Evidence) { e.ReportedOccurrenceCount++ }, "unreported": func(e *Evidence) { e.UnreportedGroupCount++ }, "loops": func(e *Evidence) { e.SelfLoopGroupCount++ }, "pairs": func(e *Evidence) { e.DistinctNonloopPairCount++ },
		"in-degree": func(e *Evidence) { e.Nodes[0].InGroupDegree++ }, "out-degree": func(e *Evidence) { e.Nodes[0].OutGroupDegree++ }, "in-neighbors": func(e *Evidence) { e.Nodes[0].InDistinctNeighbors++ }, "out-neighbors": func(e *Evidence) { e.Nodes[0].OutDistinctNeighbors++ },
		"isolate": func(e *Evidence) {
			for i, n := range e.Nodes {
				if n.InGroupDegree+n.OutGroupDegree == 0 {
					e.Nodes = append(e.Nodes[:i], e.Nodes[i+1:]...)
					e.NodeCount--
					e.InDegreeHistogram = histogram(e.Nodes, true)
					e.OutDegreeHistogram = histogram(e.Nodes, false)
					e.Density.Value.Denominator = e.NodeCount * (e.NodeCount - 1)
					return
				}
			}
		},
		"id": func(e *Evidence) { e.Nodes[0].ID = "invented" }, "histogram": func(e *Evidence) { e.InDegreeHistogram[0].NodeCount++ }, "density": func(e *Evidence) { e.Density.Value.Numerator++ }, "order": func(e *Evidence) { e.Nodes[0], e.Nodes[1] = e.Nodes[1], e.Nodes[0] }, "basis": func(e *Evidence) { e.BasisDigest = "sha256:" + strings.Repeat("0", 64) }, "policy": func(e *Evidence) { e.Policy = "projection.v1" }, "scope": func(e *Evidence) { e.Scope = "AUTHENTICATED" }, "status": func(e *Evidence) { e.Status = "COMPLETE" },
		"ceilings": func(e *Evidence) {
			var r retainedcalls.Evidence
			_ = json.Unmarshal(e.InputBytes, &r)
			r.Ceilings.DependencyCompleteness = "COMPLETE"
			e.InputBytes, _ = json.Marshal(r)
			e.BasisDigest = basis(e.InputBytes)
		},
	} {
		t.Run(name, func(t *testing.T) {
			var e Evidence
			b, _ := json.Marshal(base)
			_ = json.Unmarshal(b, &e)
			mutate(&e)
			e.Digest = seal(e)
			b, _ = json.Marshal(e)
			if _, err := ValidateFor(b, Family, "v1"); err == nil {
				t.Fatal("ASSERT_COHERENT_RESEAL_REJECT", name)
			}
		})
	}
	encoded, _ := json.Marshal(base)
	if _, err := schema.ValidateFor(encoded, Family, "v1"); err == nil {
		t.Fatal("ASSERT_LOWER_SHAPE_ONLY_FAIL_CLOSED")
	}
	for _, bad := range [][]byte{bytes.Replace(encoded, []byte(`"policy":`), []byte(`"Policy":`), 1), bytes.Replace(encoded, []byte(`"parameters":{}`), []byte(`"parameters":{"max_work":1}`), 1), bytes.Replace(encoded, []byte(`"node_count":3`), []byte(`"node_count":3,"node_count":3`), 1), bytes.Replace(encoded, []byte(`"parameters":{}`), []byte(`"parameters":{},"extra":1`), 1)} {
		if _, err := ValidateFor(bad, Family, "v1"); err == nil {
			t.Fatal("ASSERT_EXACT_CLOSED_DUPLICATE_CASE")
		}
	}
	formatted := admitted(t, append(append([]byte{}, raw...), '\n'))
	if base.BasisDigest == formatted.BasisDigest || base.Digest == formatted.Digest || !bytes.Equal(canonical(base.Nodes), canonical(formatted.Nodes)) {
		t.Fatal("ASSERT_EXACT_BYTES_BASIS")
	}
}

type cancelAfter struct {
	context.Context
	remaining int
}

func (c *cancelAfter) Err() error {
	c.remaining--
	if c.remaining <= 0 {
		return context.Canceled
	}
	return nil
}

func TestMetricsBoundsCancellationAndEncodedPreflight(t *testing.T) {
	raw := fixture(t, 2, [][2]int{{0, 1}})
	base := admitted(t, raw)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if out, err := Analyze(ctx, raw); err == nil || out != nil {
		t.Fatal("ASSERT_CANCEL_NO_PARTIAL_METRICS")
	}
	if out, err := Analyze(context.Background(), bytes.Repeat([]byte(" "), MaxInputBytes+1)); err == nil || out != nil {
		t.Fatal("ASSERT_INPUT_BYTE_BOUND")
	}
	deep := []byte(`{"x":` + strings.Repeat("[", 80) + `{"dup":0,"dup":1}` + strings.Repeat("]", 80) + `}`)
	for _, layer := range []string{"retained", "provenance", "graph"} {
		var r retainedcalls.Evidence
		_ = json.Unmarshal(raw, &r)
		if layer == "graph" {
			var p graphprovenance.Evidence
			_ = json.Unmarshal(r.InputBytes, &p)
			p.GraphBytes = deep
			r.InputBytes, _ = json.Marshal(p)
		} else {
			r.InputBytes = deep
		}
		input, _ := json.Marshal(r)
		if layer == "retained" {
			input = deep
		}
		for _, validate := range []bool{false, true} {
			var err error
			if validate {
				e := base
				e.InputBytes = input
				b, _ := json.Marshal(e)
				_, err = ValidateFor(b, Family, "v1")
			} else {
				_, err = Analyze(context.Background(), input)
			}
			if err == nil || !strings.Contains(err.Error(), "nesting LIMIT") || strings.Contains(err.Error(), "duplicate") {
				t.Fatal("ASSERT_ENCODED_DEPTH_BEFORE_RECURSION", layer, err)
			}
		}
	}
	for _, bad := range [][]byte{bytes.Replace(raw, []byte(`"input_bytes":`), []byte(`"input_bytes":"!","INPUT_BYTES":`), 1), bytes.Replace(raw, []byte(`"input_bytes":`), []byte(`"input_bytes":"!","input_bytes":`), 1)} {
		if _, err := Analyze(context.Background(), bad); err == nil {
			t.Fatal("ASSERT_CARRIER_ALIAS_DUPLICATE")
		}
	}
	// Fixed maximum topology computation and proof are bounded; admission limits
	// are independently exercised by the shared admission package.
	tab := table(MaxNodes, nil)
	for i := 0; i < MaxGroups; i++ {
		tab.Groups = append(tab.Groups, retainedcalls.Group{CallerNodeID: "0", CalleeNodeID: fmt.Sprint(i % MaxNodes), CallsiteState: "UNREPORTED"})
	}
	e, err := compute(context.Background(), []byte("max"), tab)
	if err != nil {
		t.Fatal(err)
	}
	if err = prove(e, tab); err != nil {
		t.Fatal(err)
	}
	for _, after := range []int{1, MaxNodes / 2, MaxNodes + MaxGroups/2} {
		got, err := compute(&cancelAfter{Context: context.Background(), remaining: after}, []byte("max"), tab)
		if err == nil || got.Status != "" || got.Nodes != nil {
			t.Fatal("ASSERT_CANCEL_DURING_WORK_NO_PARTIAL", after)
		}
	}
	if boundedanalysis.MaxNodes != MaxNodes || boundedanalysis.MaxEdges != MaxGroups {
		t.Fatal("ASSERT_FIXED_SHARED_LIMITS")
	}
}
