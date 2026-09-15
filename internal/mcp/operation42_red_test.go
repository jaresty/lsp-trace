package mcp

import "testing"

func TestOperation42CanonicalLifecycleRegistration(t *testing.T) {
	const assertion = "ASSERT_OPERATION_42_CANONICAL_LIFECYCLE_REGISTRATION"
	tool, ok := NewRegistry(false).ResolveCanonical("lsp_session_v1_derive_workspace")
	if !ok {
		t.Fatalf("%s: canonical operation missing", assertion)
	}
	if tool.ExecutorFamily != LifecycleExecutorFamily || tool.Availability != Enabled {
		t.Fatalf("%s: tool=%+v", assertion, tool)
	}
	if len(tool.Aliases) != 0 {
		t.Fatalf("ASSERT_OPERATION_42_COUNTS_AND_NO_ALIAS: aliases=%v", tool.Aliases)
	}
}

func TestOperation42ExactClosedInput(t *testing.T) {
	const assertion = "ASSERT_OPERATION_42_EXACT_CLOSED_INPUT"
	tool, ok := NewRegistry(false).ResolveCanonical("lsp_session_v1_derive_workspace")
	if !ok {
		t.Fatalf("%s: canonical operation missing", assertion)
	}
	properties, _ := tool.InputSchema["properties"].(map[string]any)
	if len(properties) != 3 || properties["session_id"] == nil || properties["generation"] == nil || properties["workspace_uri"] == nil {
		t.Fatalf("%s: properties=%v", assertion, properties)
	}
	required, _ := tool.InputSchema["required"].([]any)
	if len(required) != 3 {
		t.Fatalf("%s: required=%v", assertion, required)
	}
	if additional, ok := tool.InputSchema["additionalProperties"].(bool); !ok || additional {
		t.Fatalf("%s: additionalProperties=%v", assertion, tool.InputSchema["additionalProperties"])
	}
}

func TestOperation42Counts(t *testing.T) {
	const assertion = "ASSERT_OPERATION_42_COUNTS_AND_NO_ALIAS"
	if got := len(NewRegistry(false).Tools()); got != 42 {
		t.Fatalf("%s: canonical=%d want=42", assertion, got)
	}
	if got := len(NewRegistryWithProfile(false, ToolProfileCompact).Advertised()); got != 12 {
		t.Fatalf("%s: compact=%d want=12", assertion, got)
	}
	if _, ok := NewRegistry(false).Resolve("lsp_session_derive_workspace"); ok {
		t.Fatalf("%s: unexpected alias resolves", assertion)
	}
}
