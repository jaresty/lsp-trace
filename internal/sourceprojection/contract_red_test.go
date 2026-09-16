package sourceprojection

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const (
	projectionRequestSchemaID = "https://jaresty.github.io/lsp-trace/mcp/schemas/source-projection-request.v1.schema.json"
	projectionResultSchemaID  = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.source-projection.v1.schema.json"
)

func TestSourceProjectionRequestV1Contract(t *testing.T) {
	schema := readProjectionSchema(t, filepath.Join("..", "mcpcontract", "testdata", "schemas", "source-projection-request.v1.schema.json"))
	assertSchemaID(t, "ASSERT_SOURCE_PROJECTION_REQUEST_V1_DISTINCT_ID", schema, projectionRequestSchemaID)
	properties := objectField(t, schema, "properties")
	for _, name := range []string{"mode", "body", "include_relation_occurrences", "include_ancillary", "limits", "privacy_policy_id"} {
		if _, ok := properties[name]; !ok {
			t.Fatalf("ASSERT_SOURCE_PROJECTION_REQUEST_V1_CLOSED_OPTIONS: missing %s", name)
		}
	}
	if schema["additionalProperties"] != false {
		t.Fatal("ASSERT_SOURCE_PROJECTION_REQUEST_V1_CLOSED_OPTIONS: root must be closed")
	}
	limits := objectField(t, objectField(t, properties, "limits"), "properties")
	for _, name := range []string{"max_objects", "max_ranges", "max_source_bytes", "max_work", "max_response_bytes"} {
		if _, ok := limits[name]; !ok {
			t.Fatalf("ASSERT_SOURCE_PROJECTION_INDEPENDENT_LIMITS: missing %s", name)
		}
	}
}

func TestSourceProjectionResultV1NeutralityAndAccounting(t *testing.T) {
	schema := readProjectionSchema(t, filepath.Join("..", "schema", "schemas", "lsp-trace.source-projection.v1.schema.json"))
	assertSchemaID(t, "ASSERT_SOURCE_PROJECTION_RESULT_V1_DISTINCT_ID", schema, projectionResultSchemaID)
	properties := objectField(t, schema, "properties")
	assertConst(t, "ASSERT_SOURCE_PROJECTION_AUTHORITY_ZERO", properties, "authority", float64(0))
	assertConst(t, "ASSERT_SOURCE_PROJECTION_COMPLETENESS_UNKNOWN", properties, "source_graph_complete", "UNKNOWN")
	assertConst(t, "ASSERT_SOURCE_PROJECTION_ADDS_ZERO_GRAPH_FACTS", properties, "graph_facts_added", float64(0))
	for _, name := range []string{"custody_mode", "custody_binding", "physical_projection_id", "status", "units", "citations", "emitted_spans", "accounting", "omissions"} {
		if _, ok := properties[name]; !ok {
			t.Fatalf("ASSERT_SOURCE_PROJECTION_RESULT_COMPLETE_SURFACE: missing %s", name)
		}
	}
	accounting := objectField(t, objectField(t, properties, "accounting"), "properties")
	for _, name := range []string{"candidates", "selected", "omitted", "logical_selected_bytes", "unique_emitted_bytes", "evaluated", "terminal", "unevaluated"} {
		if _, ok := accounting[name]; !ok {
			t.Fatalf("ASSERT_SOURCE_PROJECTION_EXACT_ACCOUNTING: missing %s", name)
		}
	}
}

func TestSourceProjectionResultV1CustodyAndRelationProvenance(t *testing.T) {
	schema := readProjectionSchema(t, filepath.Join("..", "schema", "schemas", "lsp-trace.source-projection.v1.schema.json"))
	defs := objectField(t, schema, "$defs")
	binding := objectField(t, defs, "custody_binding")
	branches, ok := binding["oneOf"].([]any)
	if !ok || len(branches) != 2 {
		t.Fatalf("ASSERT_SOURCE_PROJECTION_CUSTODY_EXCLUSIVE: oneOf=%#v", binding["oneOf"])
	}
	unit := objectField(t, defs, "unit")
	unitProperties := objectField(t, unit, "properties")
	assertConst(t, "ASSERT_SOURCE_PROJECTION_RELATIONS_SERVER_ONLY", unitProperties, "relation_provenance", "SERVER_REPORTED")
	omission := objectField(t, defs, "omission")
	required := stringArrayField(t, omission, "required")
	if !contains(required, "cause") || !contains(required, "unit_id") {
		t.Fatalf("ASSERT_SOURCE_PROJECTION_ONE_TERMINAL_OMISSION_CAUSE: required=%v", required)
	}
}

func readProjectionSchema(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ASSERT_SOURCE_PROJECTION_SCHEMA_PRESENT[%s]: %v", path, err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("ASSERT_SOURCE_PROJECTION_SCHEMA_JSON[%s]: %v", path, err)
	}
	return schema
}

func assertSchemaID(t *testing.T, assertion string, schema map[string]any, want string) {
	t.Helper()
	if got := schema["$id"]; got != want {
		t.Fatalf("%s: got %#v want %q", assertion, got, want)
	}
}

func objectField(t *testing.T, value map[string]any, name string) map[string]any {
	t.Helper()
	got, ok := value[name].(map[string]any)
	if !ok {
		t.Fatalf("schema field %s is not an object: %#v", name, value[name])
	}
	return got
}

func stringArrayField(t *testing.T, value map[string]any, name string) []string {
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

func assertConst(t *testing.T, assertion string, properties map[string]any, name string, want any) {
	t.Helper()
	property := objectField(t, properties, name)
	if got := property["const"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("%s: got %#v want %#v", assertion, got, want)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
