package mcp

import (
	"encoding/json"
	"testing"

	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/schema"
)

func TestFR23PublicV3Contract(t *testing.T) {
	r := NewRegistry(false)
	if got := len(r.Advertised()); got != 25 {
		t.Fatalf("ASSERT_FR23_V3_REGISTRY_COUNT: got %d want 25", got)
	}
	for _, name := range []string{"lsp_trace_v3_slice", "lsp_trace_v3_incoming"} {
		tool, ok := r.Resolve(name)
		if !ok || tool.Availability != Enabled || len(tool.Aliases) != 0 {
			t.Fatalf("ASSERT_FR23_V3_CANONICAL_TOOL: %s %+v", name, tool)
		}
		if tool.InputSchemaID != mcpcontract.AcquisitionV2InputID {
			t.Fatalf("ASSERT_FR23_V3_INPUT_REUSES_CLOSED_V2_SHAPE: %s", tool.InputSchemaID)
		}
	}
	if _, err := schema.BytesFor(schema.FamilyGraphProvenance, "v3"); err != nil {
		t.Fatalf("ASSERT_FR23_V3_SCHEMA_GET: %v", err)
	}
}

func TestFR23V2InputStillRejectsVersionField(t *testing.T) {
	raw, err := mcpcontract.SchemaJSON(mcpcontract.AcquisitionV2InputID)
	if err != nil {
		t.Fatal(err)
	}
	var s map[string]any
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	properties := s["properties"].(map[string]any)
	if _, widened := properties["provenance_version"]; widened {
		t.Fatal("ASSERT_FR23_V2_INPUT_FROZEN: provenance_version admitted")
	}
}
