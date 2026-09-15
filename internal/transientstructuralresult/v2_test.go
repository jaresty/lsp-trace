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
	value["source_body"] = "package src"
	bad, _ := json.Marshal(value)
	if mcpcontract.ValidateJSON(mcpcontract.StructuralContextV2ResultID, bad) == nil {
		t.Fatal("ASSERT_STRUCTURAL_CONTEXT_V2_SOURCE_BODY_REJECTED")
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
