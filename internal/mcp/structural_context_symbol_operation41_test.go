package mcp

import (
	"strings"
	"testing"
)

const structuralContextSymbolTool = "lsp_trace_v1_structural_context_symbol"

func TestStructuralContextDescriptionsAdvertiseDesignAnalysisIntents(t *testing.T) {
	const assertion = "ASSERT_STRUCTURAL_CONTEXT_DESCRIPTIONS_ROUTE_DESIGN_ANALYSIS"
	registry := NewRegistry(false)
	for _, name := range []string{"lsp_trace_v1_structural_context", "lsp_trace_v2_structural_context", structuralContextSymbolTool} {
		tool, ok := registry.ResolveCanonical(name)
		if !ok {
			t.Fatalf("%s[%s]: missing", assertion, name)
		}
		for _, required := range []string{"code structure", "architecture", "design dependencies", "impact analysis", "bounded", "CALLS"} {
			if !strings.Contains(tool.Description, required) {
				t.Errorf("%s[%s]: missing %q: %q", assertion, name, required, tool.Description)
			}
		}
	}
}

func TestStructuralContextSymbolOperation41RegistryContract(t *testing.T) {
	full := NewRegistryWithProfile(false, ToolProfileFull)
	compact := NewRegistryWithProfile(false, ToolProfileCompact)
	tool, ok := full.ResolveCanonical(structuralContextSymbolTool)
	if !ok || tool.Name != structuralContextSymbolTool {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_SYMBOL_OPERATION41_REGISTERED_HIDDEN_ALIAS: canonical missing")
	}
	alias, aliasOK := full.Resolve("lsp_trace_structural_context_symbol")
	if !aliasOK || alias.Name != structuralContextSymbolTool {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_SYMBOL_OPERATION41_REGISTERED_HIDDEN_ALIAS: alias missing")
	}
	if len(full.Tools()) != 43 || len(full.Advertised()) != 43 || len(compact.Tools()) != 43 || len(compact.Advertised()) != 13 {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_SYMBOL_COUNTS_AND_GATEWAY_BRANCH: full=%d/%d compact=%d/%d", len(full.Tools()), len(full.Advertised()), len(compact.Tools()), len(compact.Advertised()))
	}
	compactTool, compactOK := compact.ResolveCanonical(structuralContextSymbolTool)
	if !compactOK || compactTool.Name != structuralContextSymbolTool {
		t.Fatal("ASSERT_STRUCTURAL_CONTEXT_SYMBOL_OPERATION41_COMPACT_ADVERTISED_ALIAS_HIDDEN")
	}
	if tool.ExecutorFamily != StructuralContextSymbolExecutorFamily {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_SYMBOL_OPERATION41_REGISTERED_HIDDEN_ALIAS: family=%s", tool.ExecutorFamily)
	}

	valid := map[string]any{"session_id": "s", "generation": float64(1), "symbol": "Target", "down_depth": float64(2), "up_depth": float64(2), "max_nodes": float64(100), "timeout_ms": float64(5000), "request_timeout_ms": float64(1000), "max_messages": float64(64), "max_bytes": float64(4194304), "analysis": map[string]any{"kind": "NEIGHBORHOOD"}}
	if err := validateArguments(tool, valid); err != nil {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_SYMBOL_CLOSED_ANALYSIS_INPUT: valid rejected: %v", err)
	}
	minimal := map[string]any{"session_id": "s", "generation": float64(1), "symbol": "Target", "analysis": map[string]any{"kind": "NEIGHBORHOOD"}}
	if err := validateArguments(tool, minimal); err != nil {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_SYMBOL_MECHANICAL_BOUNDS_DEFAULT: minimal input rejected: %v", err)
	}
	for _, forbidden := range []string{"uri", "line", "character"} {
		bad := cloneMap(valid)
		bad[forbidden] = map[string]any{"uri": "file:///w/a.go", "line": float64(0), "character": float64(0)}[forbidden]
		if validateArguments(tool, bad) == nil {
			t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_SYMBOL_CLOSED_ANALYSIS_INPUT: accepted %s", forbidden)
		}
	}
	impact := cloneMap(valid)
	impact["analysis"] = map[string]any{"kind": "IMPACT", "direction": "OUTGOING", "depth": float64(64)}
	if err := validateArguments(tool, impact); err != nil {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_SYMBOL_CLOSED_ANALYSIS_INPUT: impact rejected: %v", err)
	}

	execute, _ := full.ResolveCanonical("lsp_trace_v1_execute")
	request := execute.InputSchema["properties"].(map[string]any)["request"].(map[string]any)
	found := false
	for _, raw := range request["oneOf"].([]any) {
		properties := raw.(map[string]any)["properties"].(map[string]any)
		if properties["operation"].(map[string]any)["const"] == structuralContextSymbolTool {
			found = true
		}
	}
	if !found {
		t.Fatal("ASSERT_STRUCTURAL_CONTEXT_SYMBOL_COUNTS_AND_GATEWAY_BRANCH: missing execute branch")
	}
}
