package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/publication"
)

func TestGroupedSliceSchemaRequiresClosedLeidenOptions(t *testing.T) {
	base := map[string]any{"session_id": "s", "generation": 1, "seed_manifest": map[string]any{"schema_version": "lsp-trace.seed-manifest.v2", "coordinate_convention": "zero-based-session", "root": map[string]any{"id": "root", "locator": map[string]any{"uri": "file:///fixture/a.go", "line": 0, "character": 0}}, "required_targets": []any{}}, "production_v5": true, "group_by": "leiden", "output_selector": "graph.json"}
	for _, options := range []any{nil, map[string]any{"seed": 1, "pagerank_top_k": 2}, map[string]any{"seed": 1, "pagerank_top_k": 2, "hub_top_k": 2, "extra": true}} {
		candidate := cloneMap(base)
		if options != nil {
			candidate["group_options"] = options
		}
		raw, _ := json.Marshal(candidate)
		if mcpcontract.ValidateJSON(mcpcontract.AcquisitionV2InputID, raw) == nil {
			t.Fatalf("ASSERT_GROUPED_SLICE_CLOSED_REQUIRED_OPTIONS: admitted %s", raw)
		}
	}
	base["group_options"] = map[string]any{"seed": 1, "pagerank_top_k": 2, "hub_top_k": 2}
	raw, _ := json.Marshal(base)
	if err := mcpcontract.ValidateJSON(mcpcontract.AcquisitionV2InputID, raw); err != nil {
		t.Fatalf("ASSERT_GROUPED_SLICE_VALID_SCHEMA: %v", err)
	}
}

func TestGroupedSliceDirectAndCanonicalExecutePayloadParity(t *testing.T) {
	arguments := map[string]any{
		"session_id": "s", "generation": float64(1),
		"seed_manifest": map[string]any{"schema_version": "lsp-trace.seed-manifest.v2", "coordinate_convention": "zero-based-session", "root": map[string]any{"id": "root", "locator": map[string]any{"uri": "file:///fixture/a.go", "line": float64(0), "character": float64(0)}}, "required_targets": []any{}},
		"production_v5": true, "group_by": "leiden", "group_options": map[string]any{"seed": float64(0), "pagerank_top_k": float64(2), "hub_top_k": float64(2)}, "output_selector": "graph.json",
	}
	artifact := []byte(`{"schema_version":"lsp-trace.community-presentation.v1","communities":[]}`)
	call := func(tool string) (string, []operation.Request) {
		root, err := publication.OpenRoot(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { root.Close() })
		executor := &gatewayMatrixExecutor{artifacts: map[operation.Name][]byte{operationName("lsp_trace_v3_slice"): artifact}}
		server := matrixServer(NewRegistry(false), executor)
		server.PublicationRoot = root
		if tool == "lsp_trace_v1_execute" {
			response := runServerMessages(t, server, callMessage(tool, map[string]any{"request": map[string]any{"operation": "lsp_trace_v3_slice", "arguments": arguments}}))[0]
			return decodeEnvelopeForAssertion(t, "ASSERT_GROUPED_SLICE_DIRECT_CANONICAL_EXECUTE_PAYLOAD_PARITY", response)["delegated_envelope"].(string), executor.calls
		}
		response := directMatrixCall(server, tool, arguments)
		if response.Error != nil {
			t.Fatal(response.Error)
		}
		raw, err := json.Marshal(response.Result.(callResult).StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw), executor.calls
	}
	direct, directCalls := call("lsp_trace_v3_slice")
	wrapped, wrappedCalls := call("lsp_trace_v1_execute")
	if wrapped != direct {
		t.Fatalf("ASSERT_GROUPED_SLICE_DIRECT_CANONICAL_EXECUTE_PAYLOAD_PARITY: direct=%s execute=%s", direct, wrapped)
	}
	if len(directCalls) != 1 || len(wrappedCalls) != 1 || string(directCalls[0].Input) != string(wrappedCalls[0].Input) || !strings.Contains(string(directCalls[0].Input), `"seed":0`) {
		t.Fatalf("ASSERT_GROUPED_SLICE_DIRECT_CANONICAL_EXECUTE_INPUT_PARITY_AND_EXPLICIT_ZERO_SEED: direct=%v execute=%v", directCalls, wrappedCalls)
	}
}

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
