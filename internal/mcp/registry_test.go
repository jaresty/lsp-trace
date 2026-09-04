package mcp

import (
	"strings"
	"testing"
)

func TestRegistrySnapshotsAreImmutable(t *testing.T) {
	const assertion = "registry snapshots cannot mutate canonical aliases schemas or availability"
	t.Log("ASSERTION: " + assertion)

	r := NewRegistry(false)
	tools := r.Tools()
	var snapshot *Tool
	for i := range tools {
		if tools[i].Name == "lsp_trace_v1_inspect" {
			snapshot = &tools[i]
			break
		}
	}
	if snapshot == nil {
		t.Fatal("inspect tool missing")
	}
	snapshot.Aliases[0] = "mutated_alias"
	snapshot.EnvelopeSchemaIDs[0] = "mutated-envelope"
	snapshot.ArtifactSchemaIDs[0] = "mutated-artifact"
	snapshot.InputSchema["type"] = "array"
	snapshot.Availability = RuntimeDisabled

	resolved, ok := r.Resolve(snapshot.Name)
	if !ok {
		t.Fatalf("%s: canonical tool disappeared", assertion)
	}
	if resolved.Aliases[0] == "mutated_alias" || resolved.EnvelopeSchemaIDs[0] == "mutated-envelope" ||
		resolved.ArtifactSchemaIDs[0] == "mutated-artifact" || resolved.InputSchema["type"] == "array" ||
		resolved.Availability == RuntimeDisabled {
		t.Fatalf("%s: mutation escaped snapshot: %#v", assertion, resolved)
	}
}

func TestComputedRoutingIsFrozenAndAliasesStayHidden(t *testing.T) {
	const assertion = "routing metadata is computed once while canonical and alias resolution stays unambiguous and aliases stay hidden"
	t.Log("ASSERTION: " + assertion)

	enabled := true
	calls := 0
	r := NewRegistryWithRouting(false, Routing{
		Availability: func(tool Tool) Availability {
			calls++
			if tool.Name == "lsp_trace_v1_inspect" && !enabled {
				return RuntimeDisabled
			}
			return tool.Availability
		},
		Aliases: func(tool Tool) []string {
			if tool.Name == "lsp_trace_v1_inspect" {
				return []string{"private_inspect_alias"}
			}
			return tool.Aliases
		},
	})
	if calls != len(r.Tools()) {
		t.Fatalf("%s: availability computed %d times", assertion, calls)
	}
	enabled = false
	resolved, ok := r.Resolve("private_inspect_alias")
	if !ok || resolved.Name != "lsp_trace_v1_inspect" || resolved.Availability != Enabled {
		t.Fatalf("%s: resolved=%#v ok=%v", assertion, resolved, ok)
	}
	for _, tool := range r.Advertised() {
		if tool.Name == "private_inspect_alias" {
			t.Fatalf("%s: alias advertised", assertion)
		}
	}
	if calls != len(r.Tools()) {
		t.Fatalf("%s: availability callback retained after construction", assertion)
	}
}

func TestRegistryRejectsAmbiguousNames(t *testing.T) {
	const assertion = "canonical and alias collisions are rejected during construction"
	t.Log("ASSERTION: " + assertion)
	defer func() {
		if recover() == nil {
			t.Fatalf("%s: collision accepted", assertion)
		}
	}()
	NewRegistryWithRouting(false, Routing{Aliases: func(tool Tool) []string {
		if tool.Name == "lsp_trace_v1_inspect" {
			return []string{"lsp_trace_v1_filter"}
		}
		return tool.Aliases
	}})
}

func TestLifecycleExecutorFamilyIsEnabledAndAdvertisedByDefault(t *testing.T) {
	const assertion = "ASSERT_ALWAYS_LOCAL_TWELVE_TOOL_ORDER"
	t.Log("ASSERTION: " + assertion)
	r := NewRegistry(false)
	if got := len(r.Advertised()); got != 12 {
		t.Fatalf("%s: advertised=%d", assertion, got)
	}
	for _, canonical := range []string{"lsp_session_v1_list", "lsp_session_v1_status", "lsp_session_v1_stop", "lsp_session_v1_restart"} {
		tool, ok := r.Resolve(canonical)
		if !ok || tool.Availability != Enabled || tool.ExecutorFamily != LifecycleExecutorFamily {
			t.Fatalf("%s[%s]: tool=%+v ok=%v", assertion, canonical, tool, ok)
		}
		if tool.Description == "" {
			t.Fatalf("ASSERT_ALWAYS_LOCAL_LIFECYCLE_DESCRIPTION[%s]: empty", canonical)
		}
		alias, ok := r.Resolve(tool.Aliases[0])
		if !ok || alias.Name != canonical || alias.Availability != Enabled || alias.ExecutorFamily != LifecycleExecutorFamily {
			t.Fatalf("%s[%s alias]: tool=%+v ok=%v", assertion, canonical, alias, ok)
		}
	}
	want := []string{"lsp_session_v1_list", "lsp_session_v1_restart", "lsp_session_v1_status", "lsp_session_v1_stop", "lsp_trace_v1_capabilities", "lsp_trace_v1_filter", "lsp_trace_v1_incoming", "lsp_trace_v1_inspect", "lsp_trace_v1_schema_get", "lsp_trace_v1_slice", "lsp_trace_v1_validate", "lsp_trace_v1_verify"}
	for i, tool := range r.Advertised() {
		if tool.Name != want[i] {
			t.Fatalf("%s: tool[%d]=%q want %q", assertion, i, tool.Name, want[i])
		}
	}
	if incoming, ok := r.Resolve("lsp_trace_v1_incoming"); !ok || incoming.Availability != Enabled {
		t.Fatalf("ASSERT_INCOMING_CALLABLE_SLICE_RESERVED: incoming=%+v found=%v", incoming, ok)
	}
	if slice, ok := r.Resolve("lsp_trace_v1_slice"); !ok || slice.Availability != Enabled || slice.ExecutorFamily != ExecutorFamily("slice") {
		t.Fatalf("ASSERT_SLICE_CALLABLE_CANONICAL_ALIAS: slice=%+v found=%v", slice, ok)
	} else if alias, aliasOK := r.Resolve("lsp_trace_slice"); !aliasOK || alias.Name != slice.Name || alias.Availability != Enabled || alias.ExecutorFamily != ExecutorFamily("slice") {
		t.Fatalf("ASSERT_SLICE_CALLABLE_CANONICAL_ALIAS: alias=%+v found=%v", alias, aliasOK)
	}
	advertised := r.Advertised()
	for i := 1; i < len(advertised); i++ {
		if advertised[i-1].Name > advertised[i].Name {
			t.Fatalf("%s: order=%v", assertion, advertised)
		}
	}
	t.Log("PASS " + assertion)
}

func TestLifecycleSchemasPermitGenerationInference(t *testing.T) {
	const assertion = "ASSERT_LIFECYCLE_SCHEMA_GENERATION_OPTIONAL_SESSION_REQUIRED"
	t.Log("ASSERTION: " + assertion)
	r := NewRegistry(false)
	for _, name := range []string{"lsp_session_v1_status", "lsp_session_v1_stop", "lsp_session_v1_restart"} {
		tool, ok := r.Resolve(name)
		if !ok {
			t.Fatalf("%s: missing %s", assertion, name)
		}
		required, _ := tool.InputSchema["required"].([]any)
		hasSession, hasGeneration := false, false
		for _, field := range required {
			hasSession = hasSession || field == "session_id"
			hasGeneration = hasGeneration || field == "generation"
		}
		if !hasSession || hasGeneration {
			t.Fatalf("%s: %s required=%v", assertion, name, required)
		}
	}
	t.Log("PASS " + assertion)
}

func TestTraversalSchemaSelectorContracts(t *testing.T) {
	const assertion = "ASSERT_TRAVERSAL_SCHEMA_OPTIONAL_GENERATION_EXCLUSIVE_SYMBOL_TARGET"
	tool, ok := NewRegistry(false).Resolve("lsp_trace_v1_incoming")
	if !ok {
		t.Fatal(assertion + ": tool missing")
	}
	required, _ := tool.InputSchema["required"].([]any)
	for _, field := range required {
		if field == "generation" || field == "line" || field == "character" {
			t.Fatalf("%s: required=%v", assertion, required)
		}
	}
	properties, _ := tool.InputSchema["properties"].(map[string]any)
	if _, ok := properties["symbol"]; !ok || tool.InputSchema["oneOf"] == nil || len(NewRegistry(false).Advertised()) != 12 {
		t.Fatalf("%s: schema=%v advertised=%d", assertion, tool.InputSchema, len(NewRegistry(false).Advertised()))
	}
}

func TestRegistryContract(t *testing.T) {
	const (
		canonicalAssertion = "registry has twelve canonical entries with eleven always-local enabled tools and one reserved slice entry"
		aliasAssertion     = "each exact alias resolves to its canonical identity and aliases are not advertised"
		unknownAssertion   = "unknown and unsupported versioned names are not recognized"
	)
	t.Log("ASSERTION: " + canonicalAssertion)
	t.Log("ASSERTION: " + aliasAssertion)
	t.Log("ASSERTION: " + unknownAssertion)

	for _, enabledFlag := range []bool{false, true} {
		r := NewRegistry(enabledFlag)
		if got := len(r.Tools()); got != 12 {
			t.Errorf("%s: got %d entries", canonicalAssertion, got)
		}
		if got := len(r.Advertised()); got != 12 {
			t.Errorf("%s: got %d advertised entries", canonicalAssertion, got)
		}
		expected := map[string]string{
			"lsp_trace_v1_inspect": "lsp_trace_inspect", "lsp_trace_v1_filter": "lsp_trace_filter",
			"lsp_trace_v1_validate": "lsp_trace_validate", "lsp_trace_v1_verify": "lsp_trace_verify",
			"lsp_trace_v1_schema_get": "lsp_trace_schema_get", "lsp_trace_v1_capabilities": "lsp_trace_capabilities",
			"lsp_session_v1_list": "lsp_session_list", "lsp_session_v1_status": "lsp_session_status",
			"lsp_session_v1_restart": "lsp_session_restart", "lsp_session_v1_stop": "lsp_session_stop",
			"lsp_trace_v1_incoming": "lsp_trace_incoming", "lsp_trace_v1_slice": "lsp_trace_slice",
		}
		for canonical, alias := range expected {
			for _, name := range []string{canonical, alias} {
				resolved, ok := r.Resolve(name)
				if !ok || resolved.Name != canonical {
					t.Errorf("%s: %s did not resolve to %s", aliasAssertion, name, canonical)
				}
			}
		}
		for _, tool := range r.Advertised() {
			if tool.Availability != Enabled {
				t.Errorf("%s: advertised %s with %s", canonicalAssertion, tool.Name, tool.Availability)
			}
			if !strings.HasPrefix(tool.InputSchemaID, "https://jaresty.github.io/lsp-trace/mcp/schemas/") {
				t.Errorf("%s: %s uses non-manifest schema identity %q", canonicalAssertion, tool.Name, tool.InputSchemaID)
			}
		}
	}

	r := NewRegistry(false)
	for _, name := range []string{"lsp_trace_v2_inspect", "lsp_trace_nope", "unknown"} {
		if _, ok := r.Resolve(name); ok {
			t.Errorf("%s: recognized %s", unknownAssertion, name)
		}
	}
}
