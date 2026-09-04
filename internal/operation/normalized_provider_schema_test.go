package operation

import (
	"context"
	"encoding/json"
	"testing"
)

func TestGraphV4SchemaRetrievalAndValidationPermission(t *testing.T) {
	got, failure := SchemaGetHandler(context.Background(), Request{Input: json.RawMessage(`{"schema":{"family":"graph","version":"v4"}}`)})
	if failure != nil {
		t.Fatalf("ASSERT_MCP_GRAPH_V4_SCHEMA_GET_EXECUTES: %v", failure.Diagnostics)
	}
	if len(got.Artifact) == 0 {
		t.Fatal("ASSERT_MCP_GRAPH_V4_SCHEMA_GET_EXECUTES: empty artifact")
	}
	artifact := map[string]any{
		"schema_version": "lsp-trace.graph.v4",
		"nodes":          []any{},
		"edges":          []any{},
		"relations":      []any{},
	}
	input, err := json.Marshal(map[string]any{
		"input":  artifact,
		"schema": map[string]any{"family": "graph", "version": "v4"},
	})
	if err != nil {
		t.Fatal(err)
	}
	validated, failure := ValidateHandler(context.Background(), Request{Input: input})
	if failure != nil {
		t.Fatalf("ASSERT_MCP_GRAPH_V4_VALIDATE_EXECUTES: %v", failure.Diagnostics)
	}
	result, ok := validated.Value.(ValidationResult)
	if !ok || result.SchemaVersion != "lsp-trace.graph.v4" {
		t.Fatalf("ASSERT_MCP_GRAPH_V4_VALIDATE_EXECUTES: result=%#v", validated.Value)
	}
}
