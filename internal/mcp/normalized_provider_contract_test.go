package mcp

import "testing"

const graphV4SchemaID = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.graph.v4.schema.json"

func TestNormalizedProviderRequestContract(t *testing.T) {
	registry := NewRegistry(false)
	legacy := map[string]any{
		"session_id": "session", "uri": "file:///workspace/main.go", "symbol": "Target",
	}
	for _, name := range []string{"lsp_trace_v1_incoming", "lsp_trace_v1_slice"} {
		tool, ok := registry.Resolve(name)
		if !ok {
			t.Fatalf("ASSERT_MCP_LEGACY_REQUESTS_REMAIN_VALID: missing %s", name)
		}
		request := cloneMap(legacy)
		if name == "lsp_trace_v1_slice" {
			request["start_mode"] = "at"
		}
		if err := validateArguments(tool, request); err != nil {
			t.Fatalf("ASSERT_MCP_LEGACY_REQUESTS_REMAIN_VALID: %s: %v", name, err)
		}

		extended := cloneMap(request)
		extended["relations"] = []any{"CALLS", "BINDS_ARGUMENT", "PASSES_CALLBACK", "INVOKES_TASK", "TRIGGERS_RELOAD", "UPDATES_STATE", "RENDERS_FROM"}
		extended["adapters"] = "auto"
		extended["providers"] = []any{"ember-template-relations@1"}
		extended["workspace_revision"] = map[string]any{"kind": "git", "commit": "326718ae733cb26097bd30246276cecd371a4e79", "custody": "CALLER_ASSERTED"}
		extended["fail_on_unknown_revision"] = true
		if err := validateArguments(tool, extended); err != nil {
			t.Fatalf("ASSERT_MCP_NORMALIZED_RELATION_REQUEST_FIELDS: %s: %v", name, err)
		}

		properties, _ := tool.InputSchema["properties"].(map[string]any)
		for _, forbidden := range []string{"provider_command", "provider_path", "provider_arguments", "provider_directory", "provider_environment"} {
			if _, ok := properties[forbidden]; ok {
				t.Fatalf("ASSERT_MCP_REQUEST_HAS_NO_EXECUTION_AUTHORITY: %s exposes %s", name, forbidden)
			}
		}
	}
}

func TestNormalizedProviderCapabilitiesAndPublication(t *testing.T) {
	registry := NewRegistry(false)
	tools := registry.Tools()
	if len(tools) != 17 || len(registry.Advertised()) != 17 {
		t.Fatalf("ASSERT_MCP_CANONICAL_TOOL_COUNT_UNCHANGED: tools=%d advertised=%d", len(tools), len(registry.Advertised()))
	}
	capabilities := registry.Capabilities()
	relations, ok := capabilities["normalized_relations"].(map[string]any)
	if !ok || relations["default_when_omitted"] != "CALLS_ONLY" || relations["provider_authority"] != "HOST_PROVISIONED_ONLY" {
		t.Fatalf("ASSERT_MCP_NORMALIZED_RELATION_REQUEST_FIELDS: normalized_relations=%v", capabilities["normalized_relations"])
	}
	for _, name := range []string{"lsp_trace_v1_incoming", "lsp_trace_v1_slice"} {
		tool, _ := registry.Resolve(name)
		if !containsString(tool.ArtifactSchemaIDs, graphV4SchemaID) {
			t.Fatalf("ASSERT_MCP_GRAPH_V4_ARTIFACT_PUBLICATION: %s schemas=%v", name, tool.ArtifactSchemaIDs)
		}
	}
	for _, name := range []string{"lsp_trace_v1_schema_get", "lsp_trace_v1_validate"} {
		tool, _ := registry.Resolve(name)
		if !containsString(tool.ArtifactSchemaIDs, graphV4SchemaID) {
			t.Fatalf("ASSERT_MCP_GRAPH_V4_SCHEMA_PERMISSION: %s schemas=%v", name, tool.ArtifactSchemaIDs)
		}
	}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
