package mcp

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

var compactCanonicalNames = []string{
	"lsp_session_v1_list", "lsp_session_v1_restart", "lsp_session_v1_status", "lsp_session_v1_stop",
	"lsp_trace_v1_capabilities", "lsp_trace_v1_execute", "lsp_trace_v1_incoming",
	"lsp_trace_v1_inspect_hydrated", "lsp_trace_v1_schema_get", "lsp_trace_v1_slice",
}

func toolNames(tools []Tool) []string {
	out := make([]string, len(tools))
	for i := range tools {
		out[i] = tools[i].Name
	}
	return out
}

func TestToolProfilesPreserveFullAndCompactAdvertisement(t *testing.T) {
	for name, registry := range map[string]*Registry{
		"default": NewRegistry(false), "full": NewRegistryWithProfile(false, ToolProfileFull),
	} {
		if got := len(registry.Tools()); got != 28 {
			t.Fatalf("ASSERT_%s_DISPATCHABLE_28: got %d", name, got)
		}
		if got := len(registry.Advertised()); got != 28 {
			t.Fatalf("ASSERT_%s_ADVERTISED_28: got %d", name, got)
		}
	}
	compact := NewRegistryWithProfile(false, ToolProfileCompact)
	if got := toolNames(compact.Advertised()); !reflect.DeepEqual(got, compactCanonicalNames) {
		t.Fatalf("ASSERT_COMPACT_ADVERTISED_EXACT10_LEXICAL: got %v", got)
	}
	if got := len(compact.Tools()); got != 28 {
		t.Fatalf("ASSERT_COMPACT_DISPATCHABLE_28: got %d", got)
	}
	for _, hidden := range []string{"lsp_trace_v1_validate", "lsp_trace_v2_slice", "lsp_trace_v3_incoming"} {
		tool, ok := compact.Resolve(hidden)
		if !ok || tool.Name != hidden {
			t.Fatalf("ASSERT_COMPACT_HIDDEN_DIRECT_RESOLVES[%s]: tool=%+v ok=%v", hidden, tool, ok)
		}
	}
	alias, ok := compact.Resolve("lsp_trace_validate")
	if !ok || alias.Name != "lsp_trace_v1_validate" {
		t.Fatalf("ASSERT_COMPACT_HIDDEN_ALIAS_RESOLVES: tool=%+v ok=%v", alias, ok)
	}
	advertised := compact.Advertised()
	advertised[0].Name = "mutated"
	if got := compact.Advertised()[0].Name; got != compactCanonicalNames[0] {
		t.Fatalf("ASSERT_PROFILE_SNAPSHOT_IMMUTABLE: %q", got)
	}
}

func TestToolProfileRejectsUnknown(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("ASSERT_TOOL_PROFILE_CLOSED: unknown accepted")
		}
	}()
	NewRegistryWithProfile(false, ToolProfile("other"))
}

func TestCompactCapabilitiesAreExactAndDescriptionsSelfContained(t *testing.T) {
	r := NewRegistryWithProfile(false, ToolProfileCompact)
	caps := r.Capabilities()
	if caps["active_tool_profile"] != "compact" || !reflect.DeepEqual(caps["advertised_tool_names"], compactCanonicalNames) {
		t.Fatalf("ASSERT_CAPABILITY_ACTIVE_AND_ADVERTISED_EXACT: %#v", caps)
	}
	if got := caps["dispatchable_tool_names"].([]string); len(got) != 28 {
		t.Fatalf("ASSERT_CAPABILITY_DISPATCHABLE_28: %v", got)
	}
	if got := caps["tools"].([]Tool); len(got) != 10 {
		t.Fatalf("ASSERT_CAPABILITY_TOOLS_MEANS_ADVERTISED: %d", len(got))
	}
	text := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(toString(caps), " ", ""), "\n", ""), "\t", "")))
	for _, stale := range []string{"source_implementation", "deployed_availability", "public_analysis"} {
		if strings.Contains(text, stale) {
			t.Fatalf("ASSERT_CAPABILITY_STALE_FIELD_REMOVED[%s]", stale)
		}
	}
	for _, tool := range r.Tools() {
		for _, phrase := range []string{"operation requiring input matching", "returning a mcp result envelope", "source completeness", "producer authentication", "lsp_trace_v1_execute"} {
			if !strings.Contains(strings.ToLower(tool.Description), phrase) {
				t.Fatalf("ASSERT_DESCRIPTION_SELF_CONTAINED[%s]: missing %q in %q", tool.Name, phrase, tool.Description)
			}
		}
		if strings.Contains(strings.ToLower(tool.Description), "unknown|unknown") {
			t.Fatalf("ASSERT_DESCRIPTION_NO_UNKNOWN_PAIR[%s]", tool.Name)
		}
	}
}

func toString(v any) string { raw, _ := json.Marshal(v); return string(raw) }

func TestPublicAnalyticsPresentationSchemaKeepsCarrierCoupling(t *testing.T) {
	for _, name := range []string{"lsp_trace_v2_bounded_retained_analysis", "lsp_trace_v2_bounded_retained_metrics", "lsp_trace_v2_bounded_retained_ranking"} {
		tool, ok := NewRegistryWithProfile(false, ToolProfileCompact).Resolve(name)
		if !ok {
			t.Fatal(name)
		}
		oneOf, ok := tool.PresentationInputSchema["oneOf"].([]any)
		if !ok || len(oneOf) != 2 {
			t.Fatalf("ASSERT_PRESENTATION_COMPACT_SELECTOR_COUPLING[%s]: %#v", name, tool.PresentationInputSchema)
		}
		for _, raw := range oneOf {
			required := raw.(map[string]any)["required"].([]any)
			if len(required) != 4 {
				t.Fatalf("ASSERT_PRESENTATION_COMPACT_SELECTOR_COUPLING[%s]: required=%v", name, required)
			}
		}
	}
}
