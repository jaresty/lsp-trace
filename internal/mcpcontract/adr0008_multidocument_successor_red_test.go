package mcpcontract

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestADR0008SuccessorSchemaIdentitiesRegistered(t *testing.T) {
	ids := []string{
		SourceProjectionRequestV2ID,
		SourceProjectionResultV2ID,
		StructuralContextProjectionInputID,
		UnifiedStructuralContextResultV2ID,
		StructuralContextProjectionSuccessID,
		StructuralContextProjectionDomainErrorID,
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Fatalf("ASSERT_ADR0008_SUCCESSOR_SCHEMA_IDENTITIES_REGISTERED: duplicate %s", id)
		}
		seen[id] = true
		raw, err := SchemaJSON(id)
		if err != nil {
			t.Fatalf("ASSERT_ADR0008_SUCCESSOR_SCHEMA_IDENTITIES_REGISTERED[%s]: %v", id, err)
		}
		var schema map[string]any
		if err := json.Unmarshal(raw, &schema); err != nil || schema["$id"] != id {
			t.Fatalf("ASSERT_ADR0008_SUCCESSOR_SCHEMA_IDENTITIES_REGISTERED[%s]: id=%v err=%v", id, schema["$id"], err)
		}
	}
}

func TestADR0008ProjectionRequestV2DeclaresIndependentDocumentLimits(t *testing.T) {
	schema := successorSchema(t, SourceProjectionRequestV2ID)
	properties := successorObject(t, schema, "properties")
	limits := successorObject(t, successorObject(t, properties, "limits"), "properties")
	for _, name := range []string{
		"max_objects",
		"max_ranges",
		"max_source_bytes",
		"max_work",
		"max_response_bytes",
		"max_additional_documents",
		"max_document_requests",
		"max_document_bytes",
		"max_total_document_bytes",
		"max_document_messages",
		"max_document_acquisition_work",
		"max_display_resolution_work",
	} {
		if _, ok := limits[name]; !ok {
			t.Fatalf("ASSERT_MULTI_DOCUMENT_INDEPENDENT_LIMITS: missing %s", name)
		}
	}
}

func TestADR0008ProjectionResultV2SeparatesEvidenceItemSelectionAndDisplayRanges(t *testing.T) {
	schema := successorSchema(t, SourceProjectionResultV2ID)
	properties := successorObject(t, schema, "properties")
	successorAssertConst(t, properties, "authority", float64(0), "ASSERT_SUCCESSOR_AUTHORITY_ZERO_GRAPH_NEUTRAL")
	successorAssertConst(t, properties, "source_graph_complete", "UNKNOWN", "ASSERT_SUCCESSOR_AUTHORITY_ZERO_GRAPH_NEUTRAL")
	successorAssertConst(t, properties, "graph_facts_added", float64(0), "ASSERT_SUCCESSOR_AUTHORITY_ZERO_GRAPH_NEUTRAL")

	defs := successorObject(t, schema, "$defs")
	unit := successorObject(t, defs, "unit")
	unitProperties := successorObject(t, unit, "properties")
	for _, name := range []string{"evidence_range", "item_range", "selection_range", "display_range", "display_provenance"} {
		if _, ok := unitProperties[name]; !ok {
			t.Fatalf("ASSERT_EVIDENCE_ITEM_DISPLAY_RANGES_DISTINCT_AND_PROVENANCED: missing %s", name)
		}
	}
	required := successorStrings(t, unit, "required")
	if !successorContains(required, "evidence_range") || !successorContains(required, "display_range") {
		t.Fatalf("ASSERT_FULL_DEFINITION_DISPLAY_RANGE_REQUIRED: required=%v", required)
	}
	allOf, ok := unit["allOf"].([]any)
	if !ok || len(allOf) == 0 {
		t.Fatal("ASSERT_EVIDENCE_ITEM_DISPLAY_RANGES_DISTINCT_AND_PROVENANCED: display provenance conditional absent")
	}
}

func TestADR0008ProjectionResultV2DeclaresCanonicalDocumentAccountingWithoutRawSupplies(t *testing.T) {
	schema := successorSchema(t, SourceProjectionResultV2ID)
	properties := successorObject(t, schema, "properties")
	accounting := successorObject(t, successorObject(t, properties, "document_accounting"), "properties")
	for _, name := range []string{"candidates", "selected", "acquired", "unavailable", "withheld", "limit_omitted", "total_acquired_bytes"} {
		if _, ok := accounting[name]; !ok {
			t.Fatalf("ASSERT_MULTI_DOCUMENT_LIMITS_ATOMIC_AND_RECONCILED: missing %s", name)
		}
	}
	selection := successorObject(t, successorObject(t, properties, "document_selection"), "properties")
	successorAssertConst(t, selection, "ordering", "TARGET_FIRST_THEN_URI_LEXICOGRAPHIC", "ASSERT_LIVE_DOCUMENT_SELECTION_TARGET_FIRST_CANONICAL")

	forbidden := map[string]bool{"source_supply": true, "document_supply": true, "raw_document": true, "document_body": true, "complete_document": true}
	var walk func(any)
	walk = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			for key, child := range value {
				if forbidden[key] {
					t.Fatalf("ASSERT_LIVE_DOCUMENT_SUPPLIES_EPHEMERAL_NO_DURABLE_WRITE: serializable field %s", key)
				}
				walk(child)
			}
		case []any:
			for _, child := range value {
				walk(child)
			}
		}
	}
	walk(schema)
}

func TestADR0008SuccessorReferenceClosureAndOperation36Activation(t *testing.T) {
	manifest, err := LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	manifest = WithStructuralContextV2(manifest)
	var tool *ToolContract
	for i := range manifest.Tools {
		if manifest.Tools[i].Name == StructuralContextV2Tool {
			tool = &manifest.Tools[i]
			break
		}
	}
	if tool == nil {
		t.Fatal("ASSERT_SUCCESSOR_REFERENCE_CLOSURE_WITHOUT_RUNTIME_SWITCH: tool absent")
	}
	if tool.InputSchemaID != StructuralContextProjectionInputID ||
		!reflect.DeepEqual(tool.ArtifactSchemaIDs, []string{UnifiedStructuralContextResultV2ID}) ||
		!reflect.DeepEqual(tool.EnvelopeSchemaIDs, []string{StructuralContextProjectionSuccessID, StructuralContextProjectionDomainErrorID}) {
		t.Fatalf("ASSERT_OPERATION36_V4_V2_ACTIVATED: %+v", tool)
	}
	compiler, _, err := registeredCompiler(manifest)
	if err != nil {
		t.Fatalf("ASSERT_SUCCESSOR_REFERENCE_CLOSURE_WITHOUT_RUNTIME_SWITCH[compiler]: %v", err)
	}
	for _, id := range []string{
		StructuralContextProjectionInputID,
		UnifiedStructuralContextResultV2ID,
		StructuralContextProjectionSuccessID,
		StructuralContextProjectionDomainErrorID,
	} {
		if _, err := SchemaJSON(id); err != nil {
			t.Fatalf("ASSERT_SUCCESSOR_REFERENCE_CLOSURE_WITHOUT_RUNTIME_SWITCH[%s]: %v", id, err)
		}
		if _, err := compiler.Compile(id); err != nil {
			t.Fatalf("ASSERT_SUCCESSOR_REFERENCE_CLOSURE_WITHOUT_RUNTIME_SWITCH[%s]: unresolved reference: %v", id, err)
		}
	}
	validInput := []byte(`{"session_id":"project","generation":1,"symbol":"Target","down_depth":1,"up_depth":1,"max_nodes":10,"timeout_ms":1000,"request_timeout_ms":1000,"analysis":{"kind":"NEIGHBORHOOD"},"projection":{"mode":"TARGET","body":"OMIT","include_relation_occurrences":false,"include_ancillary":false,"display_range_policy":"FULL_DEFINITION","limits":{"max_objects":1,"max_ranges":1,"max_source_bytes":0,"max_work":1,"max_response_bytes":4096,"max_additional_documents":0,"max_document_requests":1,"max_document_bytes":1024,"max_total_document_bytes":1024,"max_document_messages":1,"max_document_acquisition_work":1,"max_display_resolution_work":1},"privacy_policy_id":"public"}}`)
	if err := ValidateJSON(StructuralContextProjectionInputID, validInput); err != nil {
		t.Fatalf("ASSERT_SUCCESSOR_REFERENCE_CLOSURE_WITHOUT_RUNTIME_SWITCH[input]: %v", err)
	}
}

func successorSchema(t *testing.T, id string) map[string]any {
	t.Helper()
	raw, err := SchemaJSON(id)
	if err != nil {
		t.Fatalf("schema %s: %v", id, err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("decode %s: %v", id, err)
	}
	return schema
}

func successorObject(t *testing.T, value map[string]any, name string) map[string]any {
	t.Helper()
	got, ok := value[name].(map[string]any)
	if !ok {
		t.Fatalf("schema field %s is not an object: %#v", name, value[name])
	}
	return got
}

func successorStrings(t *testing.T, value map[string]any, name string) []string {
	t.Helper()
	raw, ok := value[name].([]any)
	if !ok {
		t.Fatalf("schema field %s is not an array: %#v", name, value[name])
	}
	out := make([]string, len(raw))
	for i := range raw {
		out[i], ok = raw[i].(string)
		if !ok {
			t.Fatalf("schema field %s[%d] is not a string", name, i)
		}
	}
	return out
}

func successorAssertConst(t *testing.T, properties map[string]any, name string, want any, assertion string) {
	t.Helper()
	property := successorObject(t, properties, name)
	if got := property["const"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("%s[%s]: got %#v want %#v", assertion, name, got, want)
	}
}

func successorContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
