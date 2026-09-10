package hydratedinspection

import (
	"bytes"
	"encoding/json"
	"sync"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	he "lsp-trace/internal/hydratedevidence"
)

func object(p map[string]any, required ...string) map[string]any {
	if required == nil {
		required = []string{}
	}
	return map[string]any{"type": "object", "additionalProperties": false, "properties": p, "required": required}
}
func str() map[string]any             { return map[string]any{"type": "string"} }
func enum(v ...string) map[string]any { return map[string]any{"type": "string", "enum": v} }
func integer(min, max int) map[string]any {
	return map[string]any{"type": "integer", "minimum": min, "maximum": max}
}
func array(v any) map[string]any {
	return map[string]any{"type": "array", "items": v, "maxItems": 10000}
}
func ref(s string) map[string]any { return map[string]any{"$ref": "#/$defs/" + s} }
func focusProperties() map[string]any {
	id := map[string]any{"type": "string", "maxLength": 1024}
	return map[string]any{"node_ids": array(id), "relation_ids": array(id), "sibling_relation_ids": array(id), "sidecar_record_ids": array(id), "include_bodies": map[string]any{"type": "boolean"}, "whole_file": map[string]any{"type": "boolean"}, "endpoint_context": map[string]any{"type": "boolean"}, "position_encoding": enum("", "utf-8", "utf-16", "utf-32"), "core_policy": policySchema(false)}
}
func policySchema(required bool) map[string]any {
	p := map[string]any{"include_bodies": map[string]any{"const": false}, "allow_caller_boundaries": map[string]any{"const": false}, "max_input_bytes": integer(1, 192<<20), "max_output_bytes": integer(1, 64<<20), "max_body_bytes": integer(0, 16<<20), "max_origins": integer(0, 10000), "max_spans": integer(0, 10000), "max_work": integer(0, 512<<20), "max_page_bytes": integer(4096, 1<<20), "max_pages": integer(1, 10000)}
	req := []string{}
	if required {
		req = []string{"include_bodies", "allow_caller_boundaries", "max_input_bytes", "max_output_bytes", "max_body_bytes", "max_origins", "max_spans", "max_work", "max_page_bytes", "max_pages"}
	}
	return object(p, req...)
}
func InputSchema() []byte {
	p := focusProperties()
	p["input"] = map[string]any{"type": "string", "minLength": 1, "maxLength": MaxArtifactBytes}
	p["sidecars"] = map[string]any{"type": "array", "maxItems": 64, "items": map[string]any{"type": "string", "minLength": 1, "maxLength": MaxArtifactBytes}}
	p["page"] = map[string]any{"type": "boolean"}
	p["cursor"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 2048}
	s := object(p, "input")
	s["$id"] = InputSchemaID
	s["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	s["allOf"] = []any{map[string]any{"if": map[string]any{"required": []string{"cursor"}}, "then": map[string]any{"required": []string{"page"}, "properties": map[string]any{"page": map[string]any{"const": true}}}}}
	raw, _ := json.Marshal(s)
	return raw
}

// OutputSchema is an operation-specific closed contract. It deliberately does not
// claim that schema-only validation proves semantics without original inputs.
func OutputSchema() []byte {
	var core map[string]any
	_ = json.Unmarshal(he.Schema(), &core)
	defs := core["$defs"].(map[string]any)
	fp := focusProperties()
	fp["core_policy"] = policySchema(true)
	// Go's DefaultFocusRequest permits nil input lists; null is not accepted on the
	// public input, but is an exact representation of absent lists in a typed view.
	for _, k := range []string{"node_ids", "relation_ids", "sibling_relation_ids", "sidecar_record_ids"} {
		fp[k] = map[string]any{"oneOf": []any{fp[k], map[string]any{"type": "null"}}}
	}
	defs["FocusRequest"] = object(fp, "node_ids", "relation_ids", "sibling_relation_ids", "sidecar_record_ids", "include_bodies", "whole_file", "endpoint_context", "position_encoding", "core_policy")
	defs["FocusSite"] = object(map[string]any{"role": enum("NODE_RANGE", "CALL_SITE", "CALLER_RANGE", "CALLEE_RANGE", "ORIGIN_DECLARATION_RANGE", "ORIGIN_SELECTION_RANGE", "DECLARATION_DECLARATION_RANGE", "DECLARATION_SELECTION_RANGE", "PREPARED_DECLARATION_RANGE", "PREPARED_SELECTION_RANGE", "ASSERTED_RECORD"), "pointer": str(), "record_id": str(), "node_id": str(), "relation_id": str(), "uri": str(), "status": enum("NO_RETAINED_BINDING", "NON_SOURCE_EXCLUDED", "MAPPED", "NO_BOUND_SOURCE", "NO_RETAINED_RANGE", "INVALID_COORDINATES", "UNKNOWN_OR_AMBIGUOUS_NODE"), "source_ids": array(str()), "origin_ids": array(str())}, "role", "pointer", "record_id", "node_id", "relation_id", "uri", "status", "source_ids", "origin_ids")
	defs["FocusOrigin"] = object(map[string]any{"ordinal": integer(0, 10000), "kind": enum("NODE", "RELATION", "SIBLING_RELATION", "SIDECAR_RECORD"), "requested_id": str(), "status": enum("UNKNOWN_ID", "AMBIGUOUS_ID", "MAPPED", "NO_CALL_SITES", "UNSUPPORTED_RECORD_TYPE"), "caller_node_id": str(), "callee_node_id": str(), "call_site_count": integer(0, 100000), "sites": array(ref("FocusSite"))}, "ordinal", "kind", "requested_id", "status", "caller_node_id", "callee_node_id", "call_site_count", "sites")
	defs["FocusManifest"] = object(map[string]any{"input_digests": array(str()), "selection_digest": str(), "duplicate_policy": map[string]any{"const": "PRESERVE_OCCURRENCES"}, "receipt_policy": map[string]any{"const": "ALL_EXACT_BOUND_RECEIPTS"}, "node_policy": map[string]any{"const": "RETAINED_NODE_RANGE"}, "excluded_non_source_records": integer(0, 100000), "origins": array(ref("FocusOrigin")), "digest": str()}, "input_digests", "selection_digest", "duplicate_policy", "receipt_policy", "node_policy", "excluded_non_source_records", "origins", "digest")
	s := object(map[string]any{"$id": map[string]any{"const": SchemaID}, "schema_version": map[string]any{"const": Version}, "focus_request": ref("FocusRequest"), "manifest": ref("FocusManifest"), "request": ref("Request"), "delivery": enum("FULL", "PAGE"), "bundle": ref("Bundle"), "page": ref("Page"), "next_cursor": map[string]any{"type": "string", "maxLength": 2048}}, "$id", "schema_version", "focus_request", "manifest", "request", "delivery", "next_cursor")
	s["$id"] = SchemaID
	s["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	s["$defs"] = defs
	s["oneOf"] = []any{map[string]any{"required": []string{"bundle"}, "not": map[string]any{"required": []string{"page"}}, "properties": map[string]any{"delivery": map[string]any{"const": "FULL"}, "next_cursor": map[string]any{"const": ""}}}, map[string]any{"required": []string{"page"}, "not": map[string]any{"required": []string{"bundle"}}, "properties": map[string]any{"delivery": map[string]any{"const": "PAGE"}}}}
	raw, _ := json.Marshal(s)
	return raw
}

var inputOnce, outputOnce sync.Once
var inputCompiled, outputCompiled *jsonschema.Schema
var inputErr, outputErr error

func compile(raw []byte, id string) (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	if err = c.AddResource(id, doc); err != nil {
		return nil, err
	}
	return c.Compile(id)
}
func ValidateInputJSON(raw []byte) error {
	if err := preflight(raw, MaxRequestBytes); err != nil {
		return err
	}
	inputOnce.Do(func() { inputCompiled, inputErr = compile(InputSchema(), InputSchemaID) })
	if inputErr != nil {
		return inputErr
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	return inputCompiled.Validate(doc)
}
func ValidateOutputJSON(raw []byte) error {
	if err := preflight(raw, MaxResponseBytes); err != nil {
		return err
	}
	outputOnce.Do(func() { outputCompiled, outputErr = compile(OutputSchema(), SchemaID) })
	if outputErr != nil {
		return outputErr
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	return outputCompiled.Validate(doc)
}
