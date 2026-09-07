package graphprovenance

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
)

// This oracle enumerates native typed fields, not Census's spelling predicate.
func TestCensusExactNativePointers(t *testing.T) {
	_, raw, _ := fixture(t)
	var full graph.Result
	var extra struct {
		Seeds []graph.SeedResult   `json:"seeds"`
		Slice *graph.SliceEvidence `json:"slice"`
	}
	if err := json.Unmarshal(raw, &full); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &extra); err != nil {
		t.Fatal(err)
	}
	full.Seeds, full.Slice = extra.Seeds, extra.Slice
	full.Invocation.Seeds[0].ContentSHA256 = strings.Repeat("a", 64)
	full.Targets = []string{full.Nodes[1].ID}
	full.Terminals = []graph.Boundary{{NodeID: full.Nodes[1].ID, Reason: graph.ServerReportedNoIncoming}}
	full.Frontier = []graph.Boundary{{NodeID: full.Nodes[1].ID, Reason: graph.MaxDepth}}
	full.Slice.OutgoingTerminalNodeIDs = []string{full.Nodes[1].ID}
	raw, err := json.Marshal(full)
	if err != nil {
		t.Fatal(err)
	}
	var native struct {
		Nodes           []graph.Node              `json:"nodes"`
		Edges           []graph.Edge              `json:"edges"`
		EvidenceReceipt graph.EvidenceReceipt     `json:"evidence_receipt"`
		Memberships     []graph.SeedMembership    `json:"seed_memberships"`
		Locators        []graph.PortableLocator   `json:"portable_locators"`
		Replay          graph.ReplayInputManifest `json:"replay_input_manifest"`
	}
	if err := json.Unmarshal(raw, &native); err != nil {
		t.Fatal(err)
	}
	want := []string{"/diagnostics/0", "/diagnostics/1/node_id", "/diagnostics/1", "/invocation/target", "/invocation/target/uri", "/invocation/seeds/0/at", "/invocation/seeds/0/resolved_uri", "/seeds/0/requested_position", "/seeds/0/requested_position/uri", "/seeds/0/prepared_target_ids/0", "/slice/source_uri", "/slice/starting_node_ids/0", "/slice/layers/0/node_ids/0"}
	want = append(want, "/identity/resolved_seeds/0/at", "/identity/resolved_seeds/0/resolved_uri", "/targets/0", "/terminals/0/node_id", "/frontier/0/node_id", "/slice/outgoing_terminal_node_ids/0")
	for i := range native.Nodes {
		for _, field := range []string{"id", "uri", "range", "selection_range"} {
			want = append(want, fmt.Sprintf("/nodes/%d/%s", i, field))
		}
		want = append(want, fmt.Sprintf("/seeds/0/reached_node_ids/%d", i))
	}
	for i := 0; i < 5; i++ {
		for _, field := range []string{"layers/1/node_ids", "frontier_node_ids", "upward_start_node_ids"} {
			want = append(want, fmt.Sprintf("/slice/%s/%d", field, i))
		}
	}
	for i := range native.Edges {
		for _, field := range []string{"caller_node_id", "callee_node_id", "call_sites/0"} {
			want = append(want, fmt.Sprintf("/edges/%d/%s", i, field))
		}
	}
	for i := range native.EvidenceReceipt.Relations {
		for _, field := range []string{"caller_node_id", "callee_node_id"} {
			want = append(want, fmt.Sprintf("/evidence_receipt/relations/%d/%s", i, field))
		}
	}
	for i, m := range native.Memberships {
		want = append(want, fmt.Sprintf("/seed_memberships/%d/seed_at", i))
		if m.EvidenceKind == "PREPARED_TARGET" || m.EvidenceKind == "REACHED_NODE" {
			want = append(want, fmt.Sprintf("/seed_memberships/%d/endpoint_id", i))
		}
	}
	for i := range native.Locators {
		for _, field := range []string{"node_id", "locator", "provenance/source/node_id", "provenance/source/selection_range"} {
			want = append(want, fmt.Sprintf("/portable_locators/%d/%s", i, field))
		}
	}
	for i, a := range native.Replay.Artifacts {
		if a.Kind == "SOURCE_ARTIFACT" {
			want = append(want, fmt.Sprintf("/replay_input_manifest/artifacts/%d/locator", i))
		}
	}
	got, err := Census(raw)
	if err != nil {
		t.Fatal(err)
	}
	pointers := []string{}
	for _, b := range got {
		pointers = append(pointers, b.Pointer)
	}
	sort.Strings(want)
	if !reflect.DeepEqual(want, pointers) {
		t.Fatalf("exhaustive typed pointer census mismatch\nwant=%q\ngot=%q", want, pointers)
	}
}

func TestSingleAtShapeCaptureAndOffline(t *testing.T) {
	root, raw, seed := fixture(t)
	base := captured(t, root, raw, seed, nil)
	for _, tc := range []struct {
		name   string
		mutate func(*graph.Result)
	}{
		{"multiple", func(g *graph.Result) {
			s := g.Invocation.Seeds[0]
			s.Label = "second"
			g.Invocation.Seeds = append(g.Invocation.Seeds, s)
			r := g.Seeds[0]
			r.Label = "second"
			g.Seeds = append(g.Seeds, r)
		}},
		{"zero", func(g *graph.Result) {
			g.Invocation.Seeds = nil
			g.Seeds = nil
			g.Nodes = nil
			g.Edges = nil
			g.Diagnostics = nil
			g.Slice = &graph.SliceEvidence{StartMode: "at", SourceURI: seed, DownDepth: 1, UpDepth: 1}
		}},
		{"requested-target", func(g *graph.Result) { g.Seeds[0].Requested.Line++ }},
		{"slice-source", func(g *graph.Result) { g.Slice.SourceURI = g.Nodes[1].URI }},
		{"resolved-source", func(g *graph.Result) { g.Invocation.Seeds[0].ResolvedURI = g.Nodes[1].URI }},
		{"at-position", func(g *graph.Result) { g.Invocation.Seeds[0].At = seed + ":99:99" }},
		{"prepared-target", func(g *graph.Result) { g.Seeds[0].PreparedTargetIDs = []string{g.Nodes[1].ID} }},
		{"layer-target", func(g *graph.Result) { g.Slice.Layers[0].NodeIDs = []string{g.Nodes[1].ID} }},
		{"discovery-enabled", func(g *graph.Result) { g.Invocation.Expansion.DispatchFamily = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var g graph.Result
			if err := json.Unmarshal(raw, &g); err != nil {
				t.Fatal(err)
			}
			var n struct {
				Seeds []graph.SeedResult   `json:"seeds"`
				Slice *graph.SliceEvidence `json:"slice"`
			}
			if err := json.Unmarshal(raw, &n); err != nil {
				t.Fatal(err)
			}
			g.Seeds, g.Slice = n.Seeds, n.Slice
			tc.mutate(&g)
			mutated, err := json.Marshal(g)
			if err != nil {
				t.Fatalf("regenerated graph setup: %v", err)
			}
			if err := graph.ValidateSemanticBundle(mutated); err != nil {
				t.Fatalf("fixture must remain valid graph-v3: %v", err)
			}
			if _, err := Capture(context.Background(), mutated, root, seed, "s", 1, nil); err == nil || !strings.Contains(err.Error(), "provenance") {
				t.Fatalf("Capture must reject unsupported shape: %v", err)
			}
			e := base
			e.GraphBytes = mutated
			e.GraphDigest = digest(Version+":graph", mutated)
			encoded, err := json.Marshal(e)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ValidateFor(encoded, Family, "v1"); err == nil || !strings.Contains(err.Error(), "provenance") {
				t.Fatalf("offline shape admission: %v", err)
			}
		})
	}
}
