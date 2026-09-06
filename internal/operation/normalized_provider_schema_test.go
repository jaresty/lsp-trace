package operation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestGraphV4OptionalProviderDiagnosticsSchema(t *testing.T) {
	diagnostic := map[string]any{"phase": "provider", "method": "PASSES_CALLBACK", "message": "BLOCKED: unresolved identity"}
	provenance := map[string]any{"provider_id": "test@1", "provider": map[string]any{}, "protocol": map[string]any{}, "adapter": map[string]any{}, "coverage": map[string]any{"Status": "PARTIAL"}, "bounds": map[string]any{}, "custody": map[string]any{}, "observation_ids": []string{"observation"}, "logical_digest": "sha256:" + strings.Repeat("a", 64), "receipt": map[string]any{}}
	artifact := map[string]any{"schema_version": "lsp-trace.graph.v4", "nodes": []any{}, "edges": []any{}, "relations": []any{}, "provenance": provenance}
	validate := func() *Failure {
		input, err := json.Marshal(map[string]any{"input": artifact, "schema": map[string]any{"family": "graph", "version": "v4"}})
		if err != nil {
			t.Fatal(err)
		}
		_, failure := ValidateHandler(context.Background(), Request{Input: input})
		return failure
	}
	if failure := validate(); failure != nil {
		t.Fatalf("historical omission rejected: %+v", failure)
	}
	provenance["diagnostics"] = []any{diagnostic}
	if failure := validate(); failure != nil {
		t.Fatalf("additive diagnostic rejected: %+v", failure)
	}
	diagnostic["unexpected"] = true
	if failure := validate(); failure == nil {
		t.Fatal("unknown diagnostic field accepted")
	}
}

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
