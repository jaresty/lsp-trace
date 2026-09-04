package mcpcontract

import (
	"encoding/json"
	"testing"
)

const contractGraphV4SchemaID = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.graph.v4.schema.json"

func TestNormalizedProviderManifestAndRequestSchemas(t *testing.T) {
	manifest, err := LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Tools) != 13 {
		t.Fatalf("ASSERT_MCP_CANONICAL_TOOL_COUNT_UNCHANGED: got %d", len(manifest.Tools))
	}
	extended := []byte(`{"session_id":"session","generation":1,"uri":"file:///workspace/main.go","line":0,"character":0,"relations":["CALLS","BINDS_ARGUMENT"],"adapters":"auto","providers":["ember-template-relations@1"],"workspace_revision":{"kind":"git","commit":"326718ae733cb26097bd30246276cecd371a4e79","custody":"CALLER_ASSERTED"},"fail_on_unknown_revision":true}`)
	for _, name := range []string{"lsp_trace_v1_incoming", "lsp_trace_v1_slice"} {
		tool := findTool(manifest, name)
		if tool == nil {
			t.Fatalf("ASSERT_MCP_NORMALIZED_RELATION_STATIC_SCHEMA: missing %s", name)
		}
		request := extended
		if name == "lsp_trace_v1_slice" {
			var value map[string]any
			if err := json.Unmarshal(extended, &value); err != nil {
				t.Fatal(err)
			}
			value["start_mode"] = "at"
			request, _ = json.Marshal(value)
		}
		if err := ValidateJSON(tool.InputSchemaID, request); err != nil {
			t.Fatalf("ASSERT_MCP_NORMALIZED_RELATION_STATIC_SCHEMA: %s: %v", name, err)
		}
		if !contractContains(tool.ArtifactSchemaIDs, contractGraphV4SchemaID) {
			t.Fatalf("ASSERT_MCP_GRAPH_V4_MANIFEST_PUBLICATION: %s schemas=%v", name, tool.ArtifactSchemaIDs)
		}
	}
	for _, name := range []string{"lsp_trace_v1_schema_get", "lsp_trace_v1_validate"} {
		tool := findTool(manifest, name)
		if !contractContains(tool.ArtifactSchemaIDs, contractGraphV4SchemaID) {
			t.Fatalf("ASSERT_MCP_GRAPH_V4_MANIFEST_PERMISSION: %s schemas=%v", name, tool.ArtifactSchemaIDs)
		}
	}
}

func contractContains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
