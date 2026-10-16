package mcp

import (
	"encoding/json"
	"reflect"
	"testing"

	"lsp-trace/internal/mcpcontract"
)

func TestExperimentalCallHierarchyCompositionRegistryAndSchema(t *testing.T) {
	advanced := NewRegistryWithProfile(false, ToolProfileExperimental)
	tool, ok := advanced.ResolveCanonical(mcpcontract.CallHierarchyCompositionTool)
	if !ok || tool.ExecutorFamily != CallHierarchyCompositionExecutorFamily || tool.InputSchemaID != mcpcontract.CallHierarchyCompositionInputID {
		t.Fatalf("tool=%+v ok=%t", tool, ok)
	}
	for _, profile := range []ToolProfile{ToolProfileDefault, ToolProfileAdvanced, ToolProfileCompact, ToolProfileFull} {
		registry := NewRegistryWithProfile(false, profile)
		for _, got := range registry.Advertised() {
			if got.Name == mcpcontract.CallHierarchyCompositionTool {
				t.Fatalf("tool leaked to %s", profile)
			}
		}
		if _, ok := registry.Resolve(mcpcontract.CallHierarchyCompositionTool); ok {
			t.Fatalf("alias dispatch leaked to %s", profile)
		}
		if _, ok := registry.ResolveCanonical(mcpcontract.CallHierarchyCompositionTool); ok {
			t.Fatalf("canonical dispatch leaked to %s", profile)
		}
	}
	found := 0
	for _, got := range advanced.Advertised() {
		if got.Name == mcpcontract.CallHierarchyCompositionTool {
			found++
		}
	}
	if found != 1 {
		t.Fatalf("advanced count=%d", found)
	}
	props := tool.InputSchema["properties"].(map[string]any)
	if tool.InputSchema["additionalProperties"] != false || props["generation"] != nil || props["raw"] != nil || props["experimental_version"].(map[string]any)["const"] != "1" {
		t.Fatalf("schema=%#v", tool.InputSchema)
	}
	valid := []map[string]any{{"experimental_version": "1", "session_id": "s", "uri": "file:///w/a.go", "language_id": "go", "symbol": "A"}, {"experimental_version": "1", "session_id": "s", "uri": "file:///w/a.go", "language_id": "go", "line": float64(0), "character": float64(0)}}
	for _, v := range valid {
		b, _ := json.Marshal(v)
		if err := mcpcontract.ValidateJSON(mcpcontract.CallHierarchyCompositionInputID, b); err != nil {
			t.Fatal(err)
		}
	}
	invalid := []map[string]any{{"experimental_version": "1", "session_id": "s", "uri": "file:///w/a.go", "language_id": "go"}, {"experimental_version": "1", "session_id": "s", "uri": "file:///w/a.go", "language_id": "go", "symbol": "A", "line": float64(0), "character": float64(0)}, {"experimental_version": "1", "session_id": "s", "uri": "file:///w/a.go", "language_id": "go", "symbol": "A", "generation": float64(1)}, {"experimental_version": "2", "session_id": "s", "uri": "file:///w/a.go", "language_id": "go", "symbol": "A"}, {"experimental_version": "1", "session_id": "s", "uri": "file:///w/a.go", "language_id": "go", "symbol": "A", "max_bytes": float64(33554433)}}
	for _, v := range invalid {
		b, _ := json.Marshal(v)
		if err := mcpcontract.ValidateJSON(mcpcontract.CallHierarchyCompositionInputID, b); err == nil {
			t.Fatalf("accepted %#v", v)
		}
	}
	if !reflect.DeepEqual(tool.ArtifactSchemaIDs, []string{mcpcontract.CallHierarchyCompositionResultID}) {
		t.Fatal(tool.ArtifactSchemaIDs)
	}
}
