package boundedanalysis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/retainedcalls"
)

func topology(nodes []string, edges []Edge, p Parameters) Evidence {
	if p.MaxWork == 0 {
		p.MaxWork = MaxWork
	}
	return Evidence{SchemaVersion: Version, Policy: Policy, Scope: Scope, Parameters: p, Nodes: nodes, Edges: edges, Status: "COMPLETE", Path: Path{[]string{}, []string{}, [][]string{}}, Components: []Component{}}
}
func TestAllThreeNodeDirectedTopologiesIndependentFloydOracle(t *testing.T) {
	// All 512 directed three-node graphs, including empty, isolates and self-loops.
	// Independent all-pairs Floyd-Warshall, not a second BFS/SCC implementation.
	for mask := 0; mask < 512; mask++ {
		nodes := []string{"a", "b", "c"}
		edges := []Edge{}
		var distance [3][3]int
		var weak [3][3]bool
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				weak[i][j] = i == j || mask&(1<<(i*3+j)) != 0 || mask&(1<<(j*3+i)) != 0
			}
		}
		for k := 0; k < 3; k++ {
			for i := 0; i < 3; i++ {
				for j := 0; j < 3; j++ {
					weak[i][j] = weak[i][j] || (weak[i][k] && weak[k][j])
				}
			}
		}
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				distance[i][j] = 99
				if i == j {
					distance[i][j] = 0
				}
				if mask&(1<<(i*3+j)) != 0 {
					edges = append(edges, Edge{GroupID: fmt.Sprintf("g%d%d", i, j), Caller: nodes[i], Callee: nodes[j], Weight: 1, CallsiteState: "UNREPORTED", OccurrenceIDs: []string{}})
					if i != j {
						distance[i][j] = 1
					}
				}
			}
		}
		for k := 0; k < 3; k++ {
			for i := 0; i < 3; i++ {
				for j := 0; j < 3; j++ {
					if d := distance[i][k] + distance[k][j]; d < distance[i][j] {
						distance[i][j] = d
					}
				}
			}
		}
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				e := topology(nodes, edges, Parameters{Operation: "PATH", Start: nodes[i], End: nodes[j]})
				run(context.Background(), &e)
				if distance[i][j] == 99 {
					if e.Status != "NOT_FOUND_IN_RETAINED_GRAPH" {
						t.Fatal("ASSERT_ORACLE_UNREACHABLE", mask, i, j)
					}
				} else if e.Status != "FOUND" || len(e.Path.GroupIDs) != distance[i][j] {
					t.Fatal("ASSERT_ORACLE_SHORTEST", mask, i, j, e)
				}
				if err := prove(e); err != nil {
					t.Fatal("ASSERT_INDEPENDENT_PATH_PROOF", err)
				}
			}
		}
		for _, mode := range []string{"WEAK", "STRONG"} {
			e := topology(nodes, edges, Parameters{Operation: "COMPONENTS", Mode: mode})
			run(context.Background(), &e)
			if err := prove(e); err != nil {
				t.Fatal("ASSERT_PARTITION_PROOF", mask, mode, err)
			}
			membership := map[string]int{}
			for c, part := range e.Components {
				for _, n := range part.Members {
					membership[n] = c
				}
			}
			{
				for i := 0; i < 3; i++ {
					for j := 0; j < 3; j++ {
						want := distance[i][j] < 99 && distance[j][i] < 99
						if mode == "WEAK" {
							want = weak[i][j]
						}
						if (membership[nodes[i]] == membership[nodes[j]]) != want {
							t.Fatal("ASSERT_ORACLE_MUTUAL_REACHABILITY", mask, i, j)
						}
					}
				}
			}
		}
	}
}
func TestParallelGroupsAndLexicalTie(t *testing.T) {
	edges := []Edge{{GroupID: "z", Caller: "a", Callee: "b", Weight: 1, OccurrenceIDs: []string{"site1", "site2"}}, {GroupID: "a", Caller: "a", Callee: "b", Weight: 1, OccurrenceIDs: []string{}}}
	e := topology([]string{"a", "b", "isolate"}, edges, Parameters{Operation: "PATH", Start: "a", End: "b"})
	run(context.Background(), &e)
	if e.Path.GroupIDs[0] != "a" || len(e.Path.OccurrenceIDs[0]) != 0 || len(e.Edges) != 2 {
		t.Fatal("ASSERT_PARALLEL_UNIT_GROUP_LEXICAL_WITNESS", e)
	}
}
func TestEmptyLimitsAndCancellation(t *testing.T) {
	for _, op := range []Parameters{{Operation: "PROJECT"}, {Operation: "COMPONENTS", Mode: "WEAK"}, {Operation: "COMPONENTS", Mode: "STRONG"}} {
		e := topology([]string{}, []Edge{}, op)
		run(context.Background(), &e)
		if e.Status != "COMPLETE" || len(e.Components) != 0 {
			t.Fatal("ASSERT_EMPTY_GRAPH", e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, op := range []Parameters{{Operation: "PROJECT"}, {Operation: "PATH", Start: "a", End: "a"}, {Operation: "COMPONENTS", Mode: "WEAK"}, {Operation: "COMPONENTS", Mode: "STRONG"}} {
		e := topology([]string{"a"}, []Edge{}, op)
		run(ctx, &e)
		if e.Status != "INCOMPLETE" || e.Reason != "CANCELLED" {
			t.Fatal("ASSERT_CANCEL_NEVER_NOT_FOUND", e)
		}
	}
	for _, op := range []Parameters{{Operation: "PROJECT"}, {Operation: "PATH", Start: "a", End: "b"}, {Operation: "COMPONENTS", Mode: "WEAK"}, {Operation: "COMPONENTS", Mode: "STRONG"}} {
		op.MaxWork = 1
		e := topology([]string{"a", "b"}, []Edge{{GroupID: "g", Caller: "a", Callee: "b", Weight: 1}}, op)
		run(context.Background(), &e)
		if e.Status != "INCOMPLETE" || e.Reason != "LIMIT" || len(e.Path.Nodes) != 0 || len(e.Components) != 0 {
			t.Fatal("ASSERT_LIMIT_NO_PARTIAL_COMPLETENESS", e)
		}
	}
}

// Fixed absent paths: no checkout or source file is required by offline analysis.
func retainedFixture(t *testing.T) ([]byte, []string) {
	t.Helper()
	nodes := []graph.Node{}
	ids := []string{}
	for _, name := range []string{"a", "b", "c", "d", "isolate"} {
		uri := "file:///__lsp_trace_bounded_absent__/" + name + ".go"
		n := graph.NewNode(graph.Item{Name: name, Kind: 12, URI: uri, Range: graph.Range{End: graph.Position{Line: 10}}, SelectionRange: graph.Range{End: graph.Position{Character: 1}}})
		nodes = append(nodes, n)
		ids = append(ids, n.ID)
	}
	r := graph.Result{SchemaVersion: graph.SchemaVersionV3, Nodes: nodes, Targets: ids[:1], Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
	for i, p := range [][2]int{{0, 1}, {1, 2}, {0, 2}, {2, 3}, {3, 2}} {
		sites := []graph.Range{}
		if i == 0 {
			sites = []graph.Range{{Start: graph.Position{Line: 1}, End: graph.Position{Line: 1, Character: 1}}, {Start: graph.Position{Line: 2}, End: graph.Position{Line: 2, Character: 1}}}
		}
		r.Edges = graph.MergeEdge(r.Edges, graph.Edge{CallerNodeID: ids[p[0]], CalleeNodeID: ids[p[1]], CallSites: sites})
	}
	seed := nodes[0].URI
	r.Invocation.Target = graph.Target{URI: seed}
	rels := []string{}
	for _, edge := range r.Edges {
		rels = append(rels, edge.RelationID)
	}
	r.Invocation.Seeds = []graph.InvocationSeed{{Label: "start", At: seed + ":0:0", ResolvedURI: seed}}
	r.Seeds = []graph.SeedResult{{Label: "start", Requested: r.Invocation.Target, PreparedTargetIDs: ids[:1], ReachedNodeIDs: ids, ReachedRelationIDs: rels, ReachedEdges: r.Edges}}
	r.Slice = &graph.SliceEvidence{StartMode: "at", SourceURI: seed, DownDepth: 1, UpDepth: 1, StartingNodeIDs: ids[:1], Layers: []graph.SliceLayer{{Depth: 0, NodeIDs: ids[:1]}, {Depth: 1, NodeIDs: ids[1:]}}, FrontierNodeIDs: ids[1:], UpwardStartNodeIDs: ids[1:], OutgoingRelationIDs: rels}
	original := append([]string{}, ids...)
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	input, err := graphprovenance.Capture(context.Background(), raw, "/__lsp_trace_bounded_absent__", seed, "fixed-test-session", 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = retainedcalls.Export(input)
	if err != nil {
		t.Fatal(err)
	}
	return raw, original
}
func evidence(t *testing.T, raw []byte, p Parameters) Evidence {
	t.Helper()
	out, err := Analyze(context.Background(), raw, p)
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
func TestCoherentlyResealedMutationsReject(t *testing.T) {
	raw, ids := retainedFixture(t)
	for name, mutate := range map[string]func(*Evidence){
		"lost-node": func(e *Evidence) { e.Nodes = e.Nodes[1:] },
		"lost-edge": func(e *Evidence) { e.Edges = e.Edges[1:] },
		"weight":    func(e *Evidence) { e.Edges[0].Weight = 2 },
		"policy":    func(e *Evidence) { e.Policy = "projection.v1" },
		"scope":     func(e *Evidence) { e.Scope = "AUTHENTICATED" },
		"direction": func(e *Evidence) { e.Edges[0].Caller, e.Edges[0].Callee = e.Edges[0].Callee, e.Edges[0].Caller },
		"witness": func(e *Evidence) {
			for i := range e.Edges {
				if len(e.Edges[i].OccurrenceIDs) > 0 {
					e.Edges[i].OccurrenceIDs = nil
					return
				}
			}
		},
		"lost-frontier": func(e *Evidence) {
			var input retainedcalls.Evidence
			_ = json.Unmarshal(e.InputBytes, &input)
			var bundle map[string]json.RawMessage
			_ = json.Unmarshal(input.Tables.Contexts[0].Bundle, &bundle)
			delete(bundle, "slice")
			input.Tables.Contexts[0].Bundle, _ = json.Marshal(bundle)
			e.InputBytes, _ = json.Marshal(input)
			e.BasisDigest = basis(e.InputBytes, e.Parameters)
		},
	} {
		t.Run(name, func(t *testing.T) {
			e := evidence(t, raw, Parameters{Operation: "PROJECT"})
			mutate(&e)
			e.Digest = seal(e)
			out, _ := json.Marshal(e)
			if _, err := ValidateFor(out, Family, "v1"); err == nil {
				t.Fatal("ASSERT_RESEALED_MUTATION_REJECT", name)
			}
		})
	}
	for _, kind := range []string{"witness", "nonshortest", "not-found", "missing-end"} {
		t.Run(kind, func(t *testing.T) {
			e := evidence(t, raw, Parameters{Operation: "PATH", Start: ids[0], End: ids[2]})
			switch kind {
			case "witness":
				e.Path.OccurrenceIDs[0] = []string{"invented"}
			case "not-found":
				e.Status = "NOT_FOUND_IN_RETAINED_GRAPH"
			case "missing-end":
				e.Path.Nodes = e.Path.Nodes[:1]
			case "nonshortest":
				out, _ := indexes(e)
				e.Path = Path{Nodes: []string{ids[0], ids[1], ids[2]}, GroupIDs: []string{}, OccurrenceIDs: [][]string{}}
				for _, pair := range [][2]string{{ids[0], ids[1]}, {ids[1], ids[2]}} {
					for _, edge := range out[pair[0]] {
						if edge.Callee == pair[1] {
							e.Path.GroupIDs = append(e.Path.GroupIDs, edge.GroupID)
							e.Path.OccurrenceIDs = append(e.Path.OccurrenceIDs, edge.OccurrenceIDs)
						}
					}
				}
			}
			e.Digest = seal(e)
			out, _ := json.Marshal(e)
			if _, err := ValidateFor(out, Family, "v1"); err == nil {
				t.Fatal("ASSERT_PATH_PROOF_REJECT", kind)
			}
		})
	}
	for _, mode := range []string{"WEAK", "STRONG"} {
		e := evidence(t, raw, Parameters{Operation: "COMPONENTS", Mode: mode})
		e.Components = e.Components[:1]
		e.Digest = seal(e)
		out, _ := json.Marshal(e)
		if _, err := ValidateFor(out, Family, "v1"); err == nil {
			t.Fatal("ASSERT_PARTITION_LOSS_REJECT", mode)
		}
	}
}
func TestStrictInputIdentityAndLimits(t *testing.T) {
	raw, ids := retainedFixture(t)
	p := Parameters{Operation: "PROJECT"}
	base := evidence(t, raw, p)
	formatted := evidence(t, append(append([]byte{}, raw...), '\n'), p)
	if base.BasisDigest == formatted.BasisDigest || base.Digest == formatted.Digest || !bytes.Equal(canonical(base.Edges), canonical(formatted.Edges)) {
		t.Fatal("ASSERT_EXACT_FORMAT_BASIS")
	}
	for _, p := range []Parameters{{Operation: "PATH", Start: "missing", End: ids[0]}, {Operation: "PATH", Start: ids[0], End: "missing"}, {Operation: "PATH", Start: "missing", End: "missing"}, {Operation: "COMPONENTS"}, {Operation: "PROJECT", MaxWork: -1}, {Operation: "PROJECT", MaxWork: MaxWork + 1}} {
		if _, err := Analyze(context.Background(), raw, p); err == nil {
			t.Fatal("ASSERT_PARAMETERS_MISSING_IDS", p)
		}
	}
	encoded, _ := json.Marshal(base)
	for _, bad := range [][]byte{bytes.Replace(encoded, []byte(`"policy":`), []byte(`"Policy":`), 1), bytes.Replace(encoded, []byte(`"weight":1`), []byte(`"weight":1,"weight":1`), 1), bytes.Replace(encoded, []byte(`"weight":1`), []byte(`"Weight":1`), 1), bytes.Replace(encoded, []byte(`"scope":`), []byte(`"extra":0,"scope":`), 1)} {
		if _, err := ValidateFor(bad, Family, "v1"); err == nil {
			t.Fatal("ASSERT_RECURSIVE_EXACT_MEMBERS")
		}
	}
	if err := Preflight(bytes.Repeat([]byte("["), 65), MaxBytes); err == nil {
		t.Fatal("ASSERT_PREDECODE_DEPTH")
	}
	if _, err := Analyze(context.Background(), bytes.Repeat([]byte(" "), MaxInputBytes+1), p); err == nil {
		t.Fatal("ASSERT_PREDECODE_BYTES")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out, err := Analyze(ctx, raw, p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ValidateFor(out, Family, "v1"); err != nil {
		t.Fatal("ASSERT_CANCEL_VALIDATES_OFFLINE", err)
	}
}
