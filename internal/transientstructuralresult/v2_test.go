package transientstructuralresult

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/transientstructural"
)

func fileURI(p string) string { return (&url.URL{Scheme: "file", Path: p}).String() }
func TestProjectV2FakeServerLiveShape(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "src", "a.go")
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("package src"), 0644); err != nil {
		t.Fatal(err)
	}
	r := transientstructural.Result{TargetID: "raw-a", Qualification: transientstructural.Qualification{PositionEncoding: "utf-16"}, Analysis: transientstructural.AnalysisResult{Nodes: []transientstructural.NodeFact{{ID: "raw-a", Name: "A", Kind: 12, URI: fileURI(file), Range: graph.Range{End: graph.Position{Character: 1}}}}, Occurrences: []transientstructural.OccurrenceFact{{CallerID: "raw-a", CalleeID: "raw-a", URI: fileURI(file), Range: graph.Range{Start: graph.Position{Line: 2}, End: graph.Position{Line: 2, Character: 3}}}}}}
	got, err := ProjectV2(r, transientstructural.Request{Generation: 1}, "ts_0123456789abcdef0123456789abcdef", root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Authority != 0 || got.SourceGraphComplete != "UNKNOWN" || got.Nodes[0].Path != "src/a.go" || got.Nodes[0].Name != "A" || got.Nodes[0].Kind != 12 || got.Calls[0].Path != "src/a.go" {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_V2_FAKE_SERVER_LIVE_SHAPE: %+v", got)
	}
	raw, _ := json.Marshal(got)
	if err := mcpcontract.ValidateJSON(mcpcontract.StructuralContextV2ResultID, raw); err != nil {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_V2_RESULT_SCHEMA: %v", err)
	}
	var value map[string]any
	_ = json.Unmarshal(raw, &value)
	if value["analytics_scope"] != "BOUNDED_LOCAL" || value["coupling"] == nil {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_V2_SHARED_COUPLING_PRESENT: %s", raw)
	}
	for _, field := range []string{"strong_components", "weak_projection", "weak_bridges", "articulation_points", "pagerank", "hits"} {
		if value[field] == nil {
			t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_V2_ANALYTICS_PRESENT_%s: %s", field, raw)
		}
	}
	if value["pagerank_damping"] != 0.85 || value["analytics_tolerance"] != 1e-12 {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_V2_FROZEN_RANKING_POLICY: %s", raw)
	}
	value["source_body"] = "package src"
	bad, _ := json.Marshal(value)
	if mcpcontract.ValidateJSON(mcpcontract.StructuralContextV2ResultID, bad) == nil {
		t.Fatal("ASSERT_STRUCTURAL_CONTEXT_V2_SOURCE_BODY_REJECTED")
	}
}
func TestProjectV2RelationalValidationRejectsForeignAnalyticsNode(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "a.go")
	if err := os.WriteFile(file, []byte("package a"), 0644); err != nil {
		t.Fatal(err)
	}
	in := transientstructural.Result{TargetID: "a", Qualification: transientstructural.Qualification{PositionEncoding: "utf-16"}, Analysis: transientstructural.AnalysisResult{Nodes: []transientstructural.NodeFact{{ID: "a", Name: "A", Kind: 12, URI: fileURI(file)}}}}
	got, err := ProjectV2(in, transientstructural.Request{Generation: 1}, "ts_0123456789abcdef0123456789abcdef", root)
	if err != nil {
		t.Fatal(err)
	}
	got.PageRank[0].NodeID = "tn_ffffffffffffffffffffffffffffffff"
	if ValidateV2(got) == nil {
		t.Fatal("ASSERT_STRUCTURAL_CONTEXT_V2_RELATIONAL_FOREIGN_NODE_REJECTED")
	}
}

func TestProjectV2OmitsAndAccountsExternalDependencies(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "main.go")
	outside := filepath.Join(t.TempDir(), "stdlib.go")
	for _, p := range []string{inside, outside} {
		if err := os.WriteFile(p, []byte("package p"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	in := transientstructural.Result{TargetID: "root", Qualification: transientstructural.Qualification{PositionEncoding: "utf-16"}, Analysis: transientstructural.AnalysisResult{
		Nodes:       []transientstructural.NodeFact{{ID: "root", Name: "main", Kind: 12, URI: fileURI(inside)}, {ID: "external", Name: "Println", Kind: 12, URI: fileURI(outside)}},
		Occurrences: []transientstructural.OccurrenceFact{{CallerID: "root", CalleeID: "external", URI: fileURI(inside)}},
	}}
	got, err := ProjectV2(in, transientstructural.Request{Generation: 1}, "ts_0123456789abcdef0123456789abcdef", root)
	if err != nil {
		t.Fatalf("ASSERT_V2_EXTERNAL_DEPENDENCIES_FILTERED_NOT_FATAL: %v", err)
	}
	raw, _ := json.Marshal(got)
	var value map[string]any
	_ = json.Unmarshal(raw, &value)
	if len(got.Nodes) != 1 || len(got.Calls) != 0 || value["external_nodes_omitted"] != float64(1) || value["external_calls_omitted"] != float64(1) {
		t.Fatalf("ASSERT_V2_EXTERNAL_DEPENDENCIES_ACCOUNTED: %s", raw)
	}
}

func TestProjectV2RejectsURIComponentsOutsideFileIdentity(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "a.go")
	if err := os.WriteFile(file, []byte("package a"), 0644); err != nil {
		t.Fatal(err)
	}
	for name, raw := range map[string]string{
		"query":    fileURI(file) + "?token=secret",
		"fragment": fileURI(file) + "#symbol",
		"userinfo": "file://user@" + file,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := relativeURI(root, raw); err == nil {
				t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_V2_URI_COMPONENT_REJECTED_%s", name)
			}
		})
	}
}

func TestProjectV2RejectsAbsoluteAndEscapingPaths(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "x.go")
	_ = os.WriteFile(outside, []byte("x"), 0644)
	if _, err := relativeURI(root, fileURI(outside)); err == nil {
		t.Fatal("ASSERT_STRUCTURAL_CONTEXT_V2_PATH_ESCAPE_REJECTED")
	}
	raw := []byte(`{"schema_version":"lsp-trace.transient-structural-result.v2","authority":0,"source_graph_complete":"UNKNOWN","position_encoding":"utf-16","target_node_id":"tn_0123456789abcdef0123456789abcdef","nodes":[{"node_id":"tn_0123456789abcdef0123456789abcdef","name":"A","kind":12,"path":"/abs/a.go","declaration_range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}],"calls":[]}`)
	if mcpcontract.ValidateJSON(mcpcontract.StructuralContextV2ResultID, raw) == nil {
		t.Fatal("ASSERT_STRUCTURAL_CONTEXT_V2_ABSOLUTE_PATH_REJECTED")
	}
	link := filepath.Join(root, "link.go")
	if err := os.Symlink(outside, link); err == nil {
		if _, err := relativeURI(root, fileURI(link)); err == nil {
			t.Fatal("ASSERT_STRUCTURAL_CONTEXT_V2_SYMLINK_ESCAPE_REJECTED")
		}
	}
}
