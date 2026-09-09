package mcpcontract

import (
	"encoding/json"
	"strings"
)

const AcquisitionV2InputID = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-acquisition.v2.schema.json"
const VerifyV2InputID = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-verify.v2.schema.json"
const GraphProvenanceV2ArtifactID = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.graph-provenance.v2.schema.json"
const GraphProvenanceV3ArtifactID = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.graph-provenance.v3.schema.json"

func AcquisitionV2EnvelopeID(id string) string {
	return strings.Replace(strings.Replace(id, "/envelope-", "/envelope-acquisition-", 1), ".v1.schema.json", ".v2.schema.json", 1)
}

// Additive resources keep the frozen stage-1 manifest and envelopes byte exact.
func WithAcquisitionV2(m *Manifest) *Manifest {
	copy := *m
	copy.Schemas = append([]SchemaRegistration{}, m.Schemas...)
	copy.Tools = append([]ToolContract{}, m.Tools...)
	copy.Schemas = append(copy.Schemas, SchemaRegistration{ID: AcquisitionV2InputID, Family: "input-acquisition.v2", Layer: "input", Path: "schemas/input-acquisition.v2.schema.json"}, SchemaRegistration{ID: GraphProvenanceV2ArtifactID, Family: "lsp-trace.graph-provenance.v2", Layer: "artifact", Path: "../graph-provenance.v2"})
	envelopes := []string{}
	for _, kind := range []string{"artifact", "publication", "compact-publication", "publication-error", "domain-error"} {
		id := AcquisitionV2EnvelopeID("https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-" + kind + ".v1.schema.json")
		name := strings.TrimPrefix(id, "https://jaresty.github.io/lsp-trace/mcp/")
		copy.Schemas = append(copy.Schemas, SchemaRegistration{ID: id, Family: strings.TrimSuffix(strings.TrimPrefix(name, "schemas/"), ".schema.json"), Layer: "envelope", Path: name})
		envelopes = append(envelopes, id)
	}
	for _, name := range []string{"lsp_trace_v2_slice", "lsp_trace_v2_incoming"} {
		copy.Tools = append(copy.Tools, ToolContract{Name: name, Aliases: []string{}, InputSchemaID: AcquisitionV2InputID, EnvelopeSchemaIDs: envelopes, ArtifactSchemaIDs: []string{GraphProvenanceV2ArtifactID}, Advertised: true, Availability: "ENABLED"})
	}
	copy.Schemas = append(copy.Schemas, SchemaRegistration{ID: VerifyV2InputID, Family: "input-verify.v2", Layer: "input", Path: "schemas/input-verify.v2.schema.json"})
	copy.Tools = append(copy.Tools, ToolContract{Name: "lsp_trace_v2_verify", Aliases: []string{}, InputSchemaID: VerifyV2InputID, EnvelopeSchemaIDs: envelopes, ArtifactSchemaIDs: []string{GraphProvenanceV2ArtifactID}, Advertised: true, Availability: "ENABLED"})
	return &copy
}

// WithAcquisitionV3 adds only the current graph-provenance V3 traversal tools,
// preserving every historical manifest composition and cardinality.
func WithAcquisitionV3(m *Manifest) *Manifest {
	copy := *m
	copy.Schemas = append([]SchemaRegistration{}, m.Schemas...)
	copy.Tools = append([]ToolContract{}, m.Tools...)
	copy.Schemas = append(copy.Schemas, SchemaRegistration{ID: GraphProvenanceV3ArtifactID, Family: "lsp-trace.graph-provenance.v3", Layer: "artifact", Path: "../../schema/schemas/lsp-trace.graph-provenance.v3.schema.json"})
	envelopes := make([]string, 0, 5)
	for _, kind := range []string{"artifact", "publication", "compact-publication", "publication-error", "domain-error"} {
		envelopes = append(envelopes, AcquisitionV2EnvelopeID("https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-"+kind+".v1.schema.json"))
	}
	for _, name := range []string{"lsp_trace_v3_slice", "lsp_trace_v3_incoming"} {
		copy.Tools = append(copy.Tools, ToolContract{Name: name, Aliases: []string{}, InputSchemaID: AcquisitionV2InputID, EnvelopeSchemaIDs: envelopes, ArtifactSchemaIDs: []string{GraphProvenanceV3ArtifactID}, Advertised: true, Availability: "ENABLED"})
	}
	return &copy
}

// readContractSchema adds immutable v2 resources without broadening historical
// schemas. The envelope shape is inherited, but identity and tool enums are new.
func readContractSchema(name string) ([]byte, error) {
	if name == "testdata/schemas/input-export-retained-calls.v2.schema.json" {
		return []byte(`{"$id":"https://jaresty.github.io/lsp-trace/mcp/schemas/input-export-retained-calls.v2.schema.json","$schema":"https://json-schema.org/draft/2020-12/schema","additionalProperties":false,"properties":{"detail":{"enum":["compact","full"],"type":"string"},"input":{"minLength":1,"type":"string"},"output_selector":{"minLength":1,"type":"string"}},"required":["input"],"type":"object"}`), nil
	}
	if strings.HasPrefix(name, "testdata/schemas/envelope-retained-calls-v2-") && strings.HasSuffix(name, ".v1.schema.json") {
		old := strings.Replace(name, "envelope-retained-calls-v2-", "envelope-", 1)
		raw, err := contractFiles.ReadFile(old)
		if err != nil {
			return nil, err
		}
		var s map[string]any
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		id := RetainedCallsV2EnvelopeID(s["$id"].(string))
		s["$id"] = id
		p := s["properties"].(map[string]any)
		p["envelope_schema_id"] = map[string]any{"const": id}
		p["tool"] = map[string]any{"const": "lsp_trace_v2_export_retained_calls"}
		return json.Marshal(s)
	}
	if name == "testdata/schemas/input-verify.v2.schema.json" {
		return []byte(`{"$id":"https://jaresty.github.io/lsp-trace/mcp/schemas/input-verify.v2.schema.json","$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"required":["input","schema"],"properties":{"input":{"type":"string","minLength":1},"schema":{"type":"object","additionalProperties":false,"required":["family","version"],"properties":{"family":{"const":"graph-provenance"},"version":{"enum":["v2","lsp-trace.graph-provenance.v2"]}}},"output_selector":{"type":"string","minLength":1},"detail":{"enum":["full","compact"]}}}`), nil
	}
	if name == "testdata/schemas/input-verify-retained-calls.v2.schema.json" {
		return []byte(`{"$id":"https://jaresty.github.io/lsp-trace/mcp/schemas/input-verify-retained-calls.v2.schema.json","$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"required":["input","schema"],"properties":{"input":{"type":"string","minLength":1},"schema":{"type":"object","additionalProperties":false,"required":["family","version"],"properties":{"family":{"const":"retained-calls"},"version":{"enum":["v2","lsp-trace.retained-calls.v2"]}}},"output_selector":{"type":"string","minLength":1},"detail":{"enum":["full","compact"]}}}`), nil
	}
	if strings.HasPrefix(name, "testdata/schemas/envelope-verify-retained-calls-v2-") && strings.HasSuffix(name, ".v1.schema.json") {
		old := strings.Replace(name, "envelope-verify-retained-calls-v2-", "envelope-", 1)
		raw, err := contractFiles.ReadFile(old)
		if err != nil {
			return nil, err
		}
		var s map[string]any
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		id := VerifyRetainedCallsV2EnvelopeID(s["$id"].(string))
		s["$id"] = id
		p := s["properties"].(map[string]any)
		p["envelope_schema_id"] = map[string]any{"const": id}
		p["tool"] = map[string]any{"const": "lsp_trace_v2_verify_retained_calls"}
		return json.Marshal(s)
	}
	if name == "testdata/schemas/input-acquisition.v2.schema.json" {
		return json.Marshal(acquisitionV2InputSchema())
	}
	if strings.HasPrefix(name, "testdata/schemas/envelope-acquisition-") && strings.HasSuffix(name, ".v2.schema.json") {
		old := strings.Replace(strings.Replace(name, "envelope-acquisition-", "envelope-", 1), ".v2.schema.json", ".v1.schema.json", 1)
		raw, err := contractFiles.ReadFile(old)
		if err != nil {
			return nil, err
		}
		var s map[string]any
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		id := AcquisitionV2EnvelopeID(s["$id"].(string))
		s["$id"] = id
		p := s["properties"].(map[string]any)
		p["envelope_schema_id"] = map[string]any{"const": id}
		p["tool"] = map[string]any{"enum": []string{"lsp_trace_v2_slice", "lsp_trace_v2_incoming", "lsp_trace_v2_verify", "lsp_trace_v3_slice", "lsp_trace_v3_incoming"}}
		p["custody_receipt"] = map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []string{"provenance", "authenticated"},
			"properties": map[string]any{
				"provenance":    map[string]any{"enum": []string{"CALLER_ASSERTED_LOCAL", "VERIFIED_HOST"}},
				"authenticated": map[string]any{"type": "boolean"},
			},
		}
		return json.Marshal(s)
	}
	return contractFiles.ReadFile(name)
}
func acquisitionV2InputSchema() map[string]any {
	integer := func(max uint64) map[string]any {
		return map[string]any{"type": "integer", "minimum": 0, "maximum": max}
	}
	object := func(p map[string]any, required ...string) map[string]any {
		if required == nil {
			required = []string{}
		}
		return map[string]any{"type": "object", "additionalProperties": false, "properties": p, "required": required}
	}
	text := func() map[string]any { return map[string]any{"type": "string", "minLength": 1} }
	locator := object(map[string]any{"uri": text(), "symbol": text(), "line": integer(4294967295), "character": integer(4294967295), "language_id": text()}, "uri")
	locator["oneOf"] = []any{map[string]any{"required": []string{"symbol"}, "not": map[string]any{"anyOf": []any{map[string]any{"required": []string{"line"}}, map[string]any{"required": []string{"character"}}}}}, map[string]any{"required": []string{"line", "character"}, "not": map[string]any{"required": []string{"symbol"}}}}
	target := object(map[string]any{"id": text(), "locator": locator, "down_depth": integer(64), "up_depth": integer(64)}, "id", "locator")
	limits := map[string]any{}
	for k, max := range map[string]uint64{"max_nodes": 10000, "max_requests": 100000, "max_evidence_bytes": 64 << 20, "max_path_work": 100000000, "timeout_ms": 60000, "request_timeout_ms": 60000, "max_response_bytes": 16 << 20, "max_messages": 4096} {
		p := integer(max)
		if k == "timeout_ms" || k == "request_timeout_ms" || k == "max_response_bytes" || k == "max_messages" {
			p["minimum"] = 1
		}
		limits[k] = p
	}
	manifest := object(map[string]any{"schema_version": map[string]any{"const": "lsp-trace.seed-manifest.v2"}, "coordinate_convention": map[string]any{"const": "zero-based-session"}, "root": target, "required_targets": map[string]any{"type": "array", "maxItems": 63, "items": target}, "limits": object(limits)}, "schema_version", "coordinate_convention", "root", "required_targets")
	out := object(map[string]any{"session_id": text(), "generation": map[string]any{"type": "integer", "minimum": 1}, "seed_manifest": manifest, "detail": map[string]any{"enum": []string{"full", "compact"}}, "output_selector": text()}, "session_id", "generation", "seed_manifest")
	out["$id"] = AcquisitionV2InputID
	out["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	return out
}
