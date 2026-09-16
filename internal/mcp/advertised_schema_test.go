package mcp

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"lsp-trace/internal/mcpcontract"
)

func TestStructuralContextV2AdvertisementIsSelfContained(t *testing.T) {
	canonicalBefore, err := mcpcontract.SchemaJSON(mcpcontract.StructuralContextProjectionInputID)
	if err != nil {
		t.Fatal(err)
	}

	registry := NewRegistryWithProfile(true, ToolProfileCompact)
	tool, ok := registry.Resolve(mcpcontract.StructuralContextV2Tool)
	if !ok {
		t.Fatal("ASSERT_SELF_CONTAINED_ADVERTISEMENT: operation 36 missing")
	}
	projection, ok := tool.InputSchema["properties"].(map[string]any)["projection"].(map[string]any)
	if !ok {
		t.Fatalf("ASSERT_SELF_CONTAINED_ADVERTISEMENT: projection=%#v", tool.InputSchema["properties"])
	}
	if _, external := projection["$ref"]; external {
		t.Fatalf("ASSERT_SELF_CONTAINED_ADVERTISEMENT: external ref retained: %#v", projection)
	}
	if projection["type"] != "object" || projection["additionalProperties"] != false {
		t.Fatalf("ASSERT_SELF_CONTAINED_ADVERTISEMENT: projection=%#v", projection)
	}
	properties, ok := projection["properties"].(map[string]any)
	if !ok || properties["display_range_policy"] == nil || properties["limits"] == nil {
		t.Fatalf("ASSERT_SELF_CONTAINED_ADVERTISEMENT: projection properties=%#v", properties)
	}

	canonicalAfter, err := mcpcontract.SchemaJSON(mcpcontract.StructuralContextProjectionInputID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(canonicalBefore, canonicalAfter) {
		t.Fatal("ASSERT_CANONICAL_SCHEMA_BYTES_UNCHANGED")
	}
	var canonical map[string]any
	if err := json.Unmarshal(canonicalAfter, &canonical); err != nil {
		t.Fatal(err)
	}
	canonicalProjection := canonical["properties"].(map[string]any)["projection"]
	if !reflect.DeepEqual(canonicalProjection, map[string]any{"$ref": mcpcontract.SourceProjectionRequestV2ID}) {
		t.Fatalf("ASSERT_CANONICAL_EXTERNAL_ID_PRESERVED: %#v", canonicalProjection)
	}
}

func TestAdvertisementExpansionRejectsUnknownExternalReference(t *testing.T) {
	_, err := selfContainedAdvertisementSchema(map[string]any{"type": "object", "properties": map[string]any{"x": map[string]any{"$ref": "https://jaresty.github.io/lsp-trace/mcp/schemas/unknown.schema.json"}}})
	if err == nil {
		t.Fatal("ASSERT_ADVERTISEMENT_UNKNOWN_REFERENCE_FAILS_CLOSED")
	}
}
