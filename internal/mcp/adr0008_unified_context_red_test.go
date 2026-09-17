package mcp

import (
	"strings"
	"testing"

	"lsp-trace/internal/mcpcontract"
)

const unifiedStructuralContextInputID = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-structural-context.v6.schema.json"

func TestADR0008UnifiedContextDiscoveryRED(t *testing.T) {
	full := NewRegistryWithProfile(false, ToolProfileFull)
	compact := NewRegistryWithProfile(false, ToolProfileCompact)

	if got := len(full.Tools()); got != 41 {
		t.Fatalf("ASSERT_UNIFIED_CONTEXT_41_CANONICAL: got %d", got)
	}
	if got := len(full.Advertised()); got != 41 {
		t.Fatalf("ASSERT_UNIFIED_CONTEXT_41_FULL_ADVERTISED: got %d", got)
	}
	if got := len(compact.Advertised()); got != 12 {
		t.Fatalf("ASSERT_UNIFIED_CONTEXT_12_COMPACT_ADVERTISED: got %d", got)
	}
	for _, legacy := range []string{
		mcpcontract.StructuralContextSymbolTool,
		mcpcontract.StructuralContextSymbolV2Tool,
	} {
		if _, ok := full.ResolveCanonical(legacy); ok {
			t.Fatalf("ASSERT_UNIFIED_CONTEXT_LEGACY_OPERATION_ABSENT[%s]", legacy)
		}
		if _, ok := compact.Resolve(legacy); ok {
			t.Fatalf("ASSERT_UNIFIED_CONTEXT_LEGACY_ROUTE_ABSENT[%s]", legacy)
		}
	}
	if tool, ok := compact.ResolveCanonical(mcpcontract.StructuralContextV2Tool); !ok || tool.Name != mcpcontract.StructuralContextV2Tool {
		t.Fatalf("ASSERT_UNIFIED_CONTEXT_COMPACT_ADVERTISED: tool=%+v ok=%t", tool, ok)
	}
}

func TestADR0008UnifiedContextDescriptionPreservesDesignIntent(t *testing.T) {
	tool, ok := NewRegistryWithProfile(false, ToolProfileFull).ResolveCanonical(mcpcontract.StructuralContextV2Tool)
	if !ok {
		t.Fatal("ASSERT_UNIFIED_CONTEXT_TOOL_PRESENT")
	}
	for _, required := range []string{"Symbol", "position", "document-regex", "READY session", "no locator facts", "server-reported CALLS", "authority 0", "completeness unknown"} {
		if !strings.Contains(tool.Description, required) {
			t.Errorf("ASSERT_UNIFIED_CONTEXT_DESCRIPTION[%s]: %q", required, tool.Description)
		}
	}
}

func TestADR0008UnifiedContextUsesNewImmutableInputSchemaRED(t *testing.T) {
	tool, ok := NewRegistryWithProfile(false, ToolProfileFull).ResolveCanonical(mcpcontract.StructuralContextV2Tool)
	if !ok {
		t.Fatal("ASSERT_UNIFIED_CONTEXT_TOOL_PRESENT")
	}
	if tool.InputSchemaID != unifiedStructuralContextInputID {
		t.Fatalf("ASSERT_UNIFIED_CONTEXT_INPUT_SCHEMA_V6: got %q", tool.InputSchemaID)
	}
}

func TestADR0008UnifiedContextAcceptsSymbolWithoutURIRED(t *testing.T) {
	tool, ok := NewRegistryWithProfile(false, ToolProfileFull).ResolveCanonical(mcpcontract.StructuralContextV2Tool)
	if !ok {
		t.Fatal("ASSERT_UNIFIED_CONTEXT_TOOL_PRESENT")
	}
	if err := validateArguments(tool, unifiedContextBase(map[string]any{"symbol": "Target"})); err != nil {
		t.Fatalf("ASSERT_UNIFIED_CONTEXT_SYMBOL_TARGET_WITHOUT_URI: %v", err)
	}
}

func TestADR0008UnifiedContextRejectsMixedTargetRED(t *testing.T) {
	tool, ok := NewRegistryWithProfile(false, ToolProfileFull).ResolveCanonical(mcpcontract.StructuralContextV2Tool)
	if !ok {
		t.Fatal("ASSERT_UNIFIED_CONTEXT_TOOL_PRESENT")
	}
	mixed := unifiedContextBase(map[string]any{
		"symbol":    "Target",
		"uri":       "file:///workspace/a.go",
		"line":      1,
		"character": 2,
	})
	if err := validateArguments(tool, mixed); err == nil {
		t.Fatal("ASSERT_UNIFIED_CONTEXT_SYMBOL_XOR_POSITION")
	}
}

func unifiedContextBase(target map[string]any) map[string]any {
	request := map[string]any{
		"session_id":         "project",
		"generation":         1,
		"down_depth":         1,
		"up_depth":           1,
		"max_nodes":          100,
		"timeout_ms":         5000,
		"request_timeout_ms": 1000,
		"analysis":           map[string]any{"kind": "NEIGHBORHOOD"},
	}
	for key, value := range target {
		request[key] = value
	}
	return request
}
