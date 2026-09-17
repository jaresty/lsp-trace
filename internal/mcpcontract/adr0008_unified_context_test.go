package mcpcontract

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func TestADR0008UnifiedStructuralContextTargetUnion(t *testing.T) {
	base := `"session_id":"project","generation":1,"down_depth":1,"up_depth":1,"max_nodes":100,"timeout_ms":5000,"request_timeout_ms":1000,"analysis":{"kind":"NEIGHBORHOOD"}`
	for name, raw := range map[string][]byte{
		"symbol":   []byte(`{` + base + `,"symbol":"Target"}`),
		"position": []byte(`{` + base + `,"uri":"file:///workspace/a.go","line":1,"character":2}`),
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateJSON(StructuralContextUnifiedInputID, raw); err != nil {
				t.Fatalf("ASSERT_UNIFIED_CONTEXT_TARGET_ACCEPTED[%s]: %v", name, err)
			}
		})
	}
	for name, raw := range map[string][]byte{
		"mixed":   []byte(`{` + base + `,"symbol":"Target","uri":"file:///workspace/a.go","line":1,"character":2}`),
		"missing": []byte(`{` + base + `}`),
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateJSON(StructuralContextUnifiedInputID, raw); err == nil {
				t.Fatalf("ASSERT_UNIFIED_CONTEXT_TARGET_REJECTED[%s]", name)
			}
		})
	}
}

func TestADR0008UnifiedStructuralContextProjectionComposition(t *testing.T) {
	const (
		requestID = "https://jaresty.github.io/lsp-trace/mcp/schemas/source-projection-request.v2.schema.json"
		resultID  = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.unified-structural-context-result.v2.schema.json"
	)
	inputRaw, err := SchemaJSON(StructuralContextProjectionInputID)
	if err != nil {
		t.Fatalf("ASSERT_UNIFIED_CONTEXT_PROJECTION_INPUT_REGISTERED: %v", err)
	}
	var input map[string]any
	if err := json.Unmarshal(inputRaw, &input); err != nil {
		t.Fatal(err)
	}
	properties := input["properties"].(map[string]any)
	projection, ok := properties["projection"].(map[string]any)
	if !ok || projection["$ref"] != requestID {
		t.Fatalf("ASSERT_UNIFIED_CONTEXT_PROJECTION_REQUEST_REF: %#v", properties["projection"])
	}
	if _, err := SchemaJSON(requestID); err != nil {
		t.Fatalf("ASSERT_SOURCE_PROJECTION_REQUEST_SCHEMA_REGISTERED: %v", err)
	}
	if _, err := SchemaJSON(resultID); err != nil {
		t.Fatalf("ASSERT_UNIFIED_CONTEXT_RESULT_SCHEMA_REGISTERED: %v", err)
	}
	manifest, err := LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	manifest = WithStructuralContextV2(manifest)
	for _, tool := range manifest.Tools {
		if tool.Name == StructuralContextV2Tool {
			if !reflect.DeepEqual(tool.ArtifactSchemaIDs, []string{resultID, UnifiedStructuralContextResultV3ID}) {
				t.Fatalf("ASSERT_UNIFIED_CONTEXT_COMPOSED_RESULT_ONLY: %v", tool.ArtifactSchemaIDs)
			}
			if !reflect.DeepEqual(tool.EnvelopeSchemaIDs, []string{StructuralContextProjectionSuccessID, StructuralContextPagingSuccessID, StructuralContextTraversalDomainErrorID}) {
				t.Fatalf("ASSERT_UNIFIED_CONTEXT_SUCCESS_ENVELOPE_REGISTERED: %v", tool.EnvelopeSchemaIDs)
			}
			if _, err := SchemaJSON(StructuralContextProjectionSuccessID); err != nil {
				t.Fatalf("ASSERT_UNIFIED_CONTEXT_SUCCESS_ENVELOPE_REGISTERED: %v", err)
			}
			return
		}
	}
	t.Fatal("ASSERT_UNIFIED_CONTEXT_MANIFEST_TOOL_PRESENT")
}

func TestADR0008UnifiedStructuralContextPreservesHistoricalInputSchema(t *testing.T) {
	v2Before, err := StructuralContextSchemaJSON(StructuralContextV2InputID)
	if err != nil {
		t.Fatal(err)
	}
	v3Before, err := StructuralContextSchemaJSON(StructuralContextUnifiedInputID)
	if err != nil {
		t.Fatal(err)
	}
	successor, err := StructuralContextSchemaJSON(StructuralContextProjectionInputID)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(v2Before, v3Before) || bytes.Equal(v3Before, successor) {
		t.Fatal("ASSERT_UNIFIED_CONTEXT_NEW_SCHEMA_IDENTITY_DISTINCT")
	}
	manifest, err := LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	manifest = WithStructuralContextV2(manifest)
	var found bool
	for _, tool := range manifest.Tools {
		if tool.Name == StructuralContextV2Tool {
			found = true
			if tool.InputSchemaID != StructuralContextPagingInputID {
				t.Fatalf("ASSERT_UNIFIED_CONTEXT_MANIFEST_USES_V6: %s", tool.InputSchemaID)
			}
		}
	}
	if !found {
		t.Fatal("ASSERT_UNIFIED_CONTEXT_MANIFEST_TOOL_PRESENT")
	}
	v2After, err := StructuralContextSchemaJSON(StructuralContextV2InputID)
	if err != nil || !bytes.Equal(v2Before, v2After) {
		t.Fatal("ASSERT_UNIFIED_CONTEXT_HISTORICAL_V2_BYTES_STABLE")
	}
	v3After, err := StructuralContextSchemaJSON(StructuralContextUnifiedInputID)
	if err != nil || !bytes.Equal(v3Before, v3After) {
		t.Fatal("ASSERT_UNIFIED_CONTEXT_HISTORICAL_V3_BYTES_STABLE")
	}
}
