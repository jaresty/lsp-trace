package mcp

import (
	"strings"
	"testing"
)

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
