package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"lsp-trace/internal/graph"

	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/retainedcalls"
)

// Independent small topology: diamond A->{B,C}->D, D->D, D->B,
// disconnected E->F, and isolate G. All sites are unreported except A->B.
func boundedFixture(t *testing.T) ([]byte, []string) {
	t.Helper()
	return boundedFixtureContent(t, []byte("package p\n"))
}

func boundedFixtureContent(t *testing.T, content []byte) ([]byte, []string) {
	t.Helper()
	root := t.TempDir()
	r := graph.Result{SchemaVersion: graph.SchemaVersionV3, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
	ids := []string{}
	for i := 0; i < 7; i++ {
		file := filepath.Join(root, fmt.Sprintf("%d.go", i))
		if err := os.WriteFile(file, content, 0600); err != nil {
			t.Fatal(err)
		}
		uri := (&url.URL{Scheme: "file", Path: file}).String()
		n := graph.NewNode(graph.Item{Name: fmt.Sprintf("N%d", i), Kind: 12, URI: uri, Range: graph.Range{End: graph.Position{Line: 10}}, SelectionRange: graph.Range{End: graph.Position{Character: 1}}})
		r.Nodes = append(r.Nodes, n)
		ids = append(ids, n.ID)
	}
	for i, p := range [][2]int{{0, 1}, {0, 2}, {1, 3}, {2, 3}, {3, 3}, {3, 1}, {4, 5}} {
		sites := []graph.Range{}
		if i == 0 {
			sites = []graph.Range{{Start: graph.Position{Line: 1}, End: graph.Position{Line: 1, Character: 1}}, {Start: graph.Position{Line: 2}, End: graph.Position{Line: 2, Character: 1}}}
		}
		r.Edges = graph.MergeEdge(r.Edges, graph.Edge{CallerNodeID: ids[p[0]], CalleeNodeID: ids[p[1]], CallSites: sites})
	}
	r.Targets = ids[:1]
	seed := r.Nodes[0].URI
	r.Invocation.Target = graph.Target{URI: seed}
	r.Invocation.Seeds = []graph.InvocationSeed{{Label: "start", At: seed + ":0:0", ResolvedURI: seed}}
	rels := []string{}
	for _, e := range r.Edges {
		rels = append(rels, e.RelationID)
	}
	r.Seeds = []graph.SeedResult{{Label: "start", Requested: r.Invocation.Target, PreparedTargetIDs: ids[:1], ReachedNodeIDs: ids, ReachedRelationIDs: rels, ReachedEdges: r.Edges}}
	r.Slice = &graph.SliceEvidence{StartMode: "at", SourceURI: seed, DownDepth: 1, UpDepth: 1, StartingNodeIDs: ids[:1], Layers: []graph.SliceLayer{{Depth: 0, NodeIDs: ids[:1]}, {Depth: 1, NodeIDs: ids[1:]}}, FrontierNodeIDs: ids[1:], UpwardStartNodeIDs: ids[1:], OutgoingRelationIDs: rels}
	originalIDs := append([]string{}, ids...)
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	input, err := graphprovenance.Capture(context.Background(), raw, root, seed, "bounded-fixture", 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = retainedcalls.Export(input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = retainedcalls.ValidateFor(raw, retainedcalls.Family, "v1"); err != nil {
		t.Fatal(err)
	}
	if err = os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	return raw, originalIDs
}

func TestBoundedAnalysisPublicCLIAndMCP(t *testing.T) {
	raw, _ := boundedFixture(t)
	testBoundedRealOffline(t, buildBinary(t, "lsp-trace", "./cmd/lsp-trace"), buildMCPBinary(t), raw)
}

func TestBoundedAnalysisAlgorithmsRealCLI(t *testing.T) {
	raw, ids := boundedFixture(t)
	cli := buildBinary(t, "lsp-trace", "./cmd/lsp-trace")
	file := filepath.Join(t.TempDir(), "retained.json")
	if err := os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	run := func(t *testing.T, args ...string) map[string]any {
		t.Helper()
		out, err := exec.Command(cli, append(append([]string{"bounded-retained-analysis"}, args...), file)...).CombinedOutput()
		if err != nil {
			t.Fatalf("ASSERT_BOUNDED_EXECUTABLE_ALGORITHM: %v: %s", err, out)
		}
		var result map[string]any
		if err = json.Unmarshal(out, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	t.Run("projection", func(t *testing.T) {
		r := run(t, "--operation", "PROJECT")
		if len(r["nodes"].([]any)) != 7 || len(r["edges"].([]any)) != 7 {
			t.Fatal("ASSERT_ALL_NODES_GROUPS_SELF_LOOP_ISOLATE")
		}
		witnesses := 0
		for _, v := range r["edges"].([]any) {
			e := v.(map[string]any)
			if e["weight"] != float64(1) {
				t.Fatal("ASSERT_UNIT_GROUP_NOT_SITES")
			}
			witnesses += len(e["occurrence_ids"].([]any))
		}
		if witnesses != 2 {
			t.Fatal("ASSERT_UNREPORTED_NO_INVENTED_WITNESSES")
		}
	})
	t.Run("path", func(t *testing.T) {
		r := run(t, "--operation", "PATH", "--start", ids[0], "--end", ids[3])
		p := r["path"].(map[string]any)
		mid := ids[1]
		if ids[2] < mid {
			mid = ids[2]
		}
		want, _ := json.Marshal([]string{ids[0], mid, ids[3]})
		got, _ := json.Marshal(p["nodes"])
		if r["status"] != "FOUND" || !bytes.Equal(want, got) || len(p["group_ids"].([]any)) != 2 {
			t.Fatal("ASSERT_DIRECTED_SHORTEST_LEXICAL_DIAMOND", r)
		}
		reverse := run(t, "--operation", "PATH", "--start", ids[3], "--end", ids[0])
		if reverse["status"] != "NOT_FOUND_IN_RETAINED_GRAPH" {
			t.Fatal("ASSERT_REVERSE_NOT_FOUND_RETAINED_SCOPE", reverse)
		}
		zero := run(t, "--operation", "PATH", "--start", ids[6], "--end", ids[6])
		if zero["status"] != "FOUND" || len(zero["path"].(map[string]any)["group_ids"].([]any)) != 0 {
			t.Fatal("ASSERT_PRESENT_ISOLATE_ZERO_HOP")
		}
		limit := run(t, "--operation", "PATH", "--start", ids[0], "--end", ids[3], "--max-work", "1")
		if limit["status"] != "INCOMPLETE" || limit["reason"] != "LIMIT" {
			t.Fatal("ASSERT_LIMIT_NEVER_NOT_FOUND", limit)
		}
	})
	t.Run("components", func(t *testing.T) {
		for mode, count := range map[string]int{"WEAK": 3, "STRONG": 6} {
			r := run(t, "--operation", "COMPONENTS", "--mode", mode)
			if r["status"] != "COMPLETE" || len(r["components"].([]any)) != count {
				t.Fatal("ASSERT_EXPLICIT_WEAK_STRONG_PARTITION", mode, r)
			}
		}
	})
}
