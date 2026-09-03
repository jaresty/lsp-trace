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

func TestRegistryContract(t *testing.T) {
	const (
		canonicalAssertion = "registry has twelve canonical entries with six enabled offline and six flag-insensitive reserved entries"
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
		if got := len(r.Advertised()); got != 6 {
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
