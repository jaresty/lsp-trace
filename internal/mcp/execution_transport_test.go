package mcp

import "testing"

const (
	executionToolName  = "lsp_trace_v1_execute"
	executionToolAlias = "lsp_trace_execute"
)

func TestExecutionToolRegistryDiscoveryAndClosedSchema(t *testing.T) {
	const assertion = "ASSERT_MCP_EXECUTION_REGISTRY_DISCOVERY_AND_CLOSED_SCHEMA"
	registry := NewRegistryWithPublication(false, true)
	canonical, ok := registry.Resolve(executionToolName)
	alias, aliasOK := registry.Resolve(executionToolAlias)
	properties, _ := canonical.InputSchema["properties"].(map[string]any)
	request, _ := properties["request"].(map[string]any)
	branches, _ := request["oneOf"].([]any)
	if !ok || !aliasOK || alias.Name != executionToolName || canonical.Availability != Enabled || canonical.Description == "" || canonical.InputSchema["additionalProperties"] != false || len(branches) != 36 {
		t.Fatalf("%s: canonical=%+v alias=%+v branches=%d", assertion, canonical, alias, len(branches))
	}
	if _, legacy := properties["output_selector"]; legacy {
		t.Fatalf("%s: execute outer schema retains legacy output_selector", assertion)
	}
}

func TestExecutionAliasUsesCanonicalOperationRequest(t *testing.T) {
	registry := NewRegistry(false)
	executor := matrixExecutor(registry)
	response := runServerMessages(t, matrixServer(registry, executor), callMessage(executionToolAlias, map[string]any{"request": map[string]any{"operation": "lsp_trace_v1_capabilities", "arguments": map[string]any{}}}))[0]
	if response["error"] != nil || len(executor.calls) != 1 {
		t.Fatalf("ASSERT_EXECUTE_ALIAS_CANONICAL_REQUEST: response=%v calls=%v", response, executor.calls)
	}
}
