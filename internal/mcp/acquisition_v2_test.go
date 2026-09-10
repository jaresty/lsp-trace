package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/internal/mcpcontract"
)

func TestAcquisitionV2CapabilitiesAndClosedSchemas(t *testing.T) {
	registry := NewRegistry(false)
	for _, name := range []string{"lsp_trace_v2_slice", "lsp_trace_v2_incoming", "lsp_trace_v2_verify"} {
		tool, ok := registry.Resolve(name)
		if !ok || tool.Description == "" || len(tool.Aliases) != 0 || len(tool.ArtifactSchemaIDs) != 1 || tool.ArtifactSchemaIDs[0] != mcpcontract.GraphProvenanceV2ArtifactID {
			t.Fatalf("ASSERT_V2_EXPLICIT_REGISTRATION: %s %+v", name, tool)
		}
		if strings.HasSuffix(name, "_slice") || strings.HasSuffix(name, "_incoming") {
			if !strings.Contains(tool.Description, "V2 output is DEPRECATED") || !strings.Contains(tool.Description, "output_version=lsp-trace.graph-provenance.v5") {
				t.Fatalf("ASSERT_V2_OUTPUT_DEPRECATION_AND_V5_ROUTE_METADATA: %s %q", name, tool.Description)
			}
		}
	}
	for _, name := range []string{"lsp_trace_v3_slice", "lsp_trace_v3_incoming"} {
		tool, ok := registry.Resolve(name)
		if !ok || !strings.Contains(tool.Description, "V3 output is DEPRECATED") || !strings.Contains(tool.Description, "output_version=lsp-trace.graph-provenance.v5") {
			t.Fatalf("ASSERT_V3_OUTPUT_DEPRECATION_AND_V5_ROUTE_METADATA: %s %+v", name, tool)
		}
	}
	matrix, ok := registry.Capabilities()["acquisition_v2"].(map[string]any)
	if !ok || matrix["default_acquisition_version"] != "v1" || matrix["authority"] != "EXACT_HOST_SESSION_GENERATION_WORKSPACE" || matrix["analyzed_source"] != "UNVERIFIED" {
		t.Fatalf("ASSERT_V2_FACTUAL_LIMITS_RETAINED: %v", matrix)
	}
	for _, stale := range []string{"source_implementation", "deployed_availability", "public_analysis"} {
		if _, present := matrix[stale]; present {
			t.Fatalf("ASSERT_V2_STALE_CAPABILITY_FIELD_REMOVED[%s]: %v", stale, matrix)
		}
	}
	valid := map[string]any{"session_id": "s", "generation": 1, "seed_manifest": map[string]any{"schema_version": "lsp-trace.seed-manifest.v2", "coordinate_convention": "zero-based-session", "root": map[string]any{"id": "root", "locator": map[string]any{"uri": "file:///fixture/a.go", "line": 0, "character": 0}}, "required_targets": []any{}}}
	raw, _ := json.Marshal(valid)
	if err := mcpcontract.ValidateJSON(mcpcontract.AcquisitionV2InputID, raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"workspace", "seed_file", "uri", "symbol", "acquisition_version", "max_nodes"} {
		bad := cloneMap(valid)
		bad[key] = "caller-choice"
		raw, _ := json.Marshal(bad)
		if mcpcontract.ValidateJSON(mcpcontract.AcquisitionV2InputID, raw) == nil {
			t.Fatalf("ASSERT_V2_UNKNOWN_SELECTOR_REJECTED: %s", key)
		}
	}
	for _, bad := range []any{"/tmp/manifest.json", nil} {
		in := cloneMap(valid)
		in["seed_manifest"] = bad
		raw, _ := json.Marshal(in)
		if mcpcontract.ValidateJSON(mcpcontract.AcquisitionV2InputID, raw) == nil {
			t.Fatal("ASSERT_V2_STRUCTURED_NOT_PATH")
		}
	}
}
