package mcp

import (
	"encoding/json"
	"testing"
)

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
	if !ok || !aliasOK || alias.Name != executionToolName || canonical.Availability != Enabled || canonical.Description == "" || canonical.InputSchema["additionalProperties"] != false || len(branches) != 40 {
		t.Fatalf("%s: canonical=%+v alias=%+v branches=%d", assertion, canonical, alias, len(branches))
	}
	if _, legacy := properties["output_selector"]; legacy {
		t.Fatalf("%s: execute outer schema retains legacy output_selector", assertion)
	}
}

func TestExecutionToolUsesBoundedPresentationSchema(t *testing.T) {
	registry := NewRegistryWithProfile(false, ToolProfileCompact)
	tool, ok := registry.Resolve(executionToolName)
	if !ok {
		t.Fatal("ASSERT_EXECUTE_PRESENTATION_TOOL: missing")
	}
	canonicalRequest := tool.InputSchema["properties"].(map[string]any)["request"].(map[string]any)
	if branches, _ := canonicalRequest["oneOf"].([]any); len(branches) != 40 {
		t.Fatalf("ASSERT_EXECUTE_CANONICAL_SCHEMA_PRESERVED: branches=%d", len(branches))
	}
	presentation := tool.PresentationInputSchema
	properties, _ := presentation["properties"].(map[string]any)
	request, _ := properties["request"].(map[string]any)
	requestProperties, _ := request["properties"].(map[string]any)
	operation, _ := requestProperties["operation"].(map[string]any)
	arguments, _ := requestProperties["arguments"].(map[string]any)
	if presentation["additionalProperties"] != false || request["additionalProperties"] != false || operation["type"] != "string" || arguments["type"] != "object" || request["oneOf"] != nil {
		t.Fatalf("ASSERT_EXECUTE_PRESENTATION_BOUNDED_CLOSED: %#v", presentation)
	}
}

func TestCompactAdvertisementMetadataBound(t *testing.T) {
	registry := NewRegistryWithProfile(false, ToolProfileCompact)
	advertised := registry.Advertised()
	if len(advertised) != 12 {
		t.Fatalf("ASSERT_COMPACT_TOOL_COUNT_STABLE: got=%d", len(advertised))
	}
	wire := make([]map[string]any, 0, len(advertised))
	for _, tool := range advertised {
		schema := tool.InputSchema
		if tool.PresentationInputSchema != nil {
			schema = tool.PresentationInputSchema
		}
		wire = append(wire, map[string]any{"name": tool.Name, "description": tool.Description, "inputSchema": schema})
	}
	encoded, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > 40*1024 {
		t.Fatalf("ASSERT_COMPACT_ADVERTISEMENT_UNDER_40_KIB: bytes=%d", len(encoded))
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
