package mcpcontract

import (
	"encoding/json"
	"strings"
)

const TraceTool = "lsp_trace_v1_trace"
const TraceInputID = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-trace.v1.schema.json"

func TraceEnvelopeID(id string) string {
	return strings.Replace(id, "/envelope-", "/envelope-trace-", 1)
}

func WithTrace(m *Manifest) *Manifest {
	c := *m
	c.Schemas = append([]SchemaRegistration{}, m.Schemas...)
	c.Tools = append([]ToolContract{}, m.Tools...)
	c.Schemas = append(c.Schemas, SchemaRegistration{ID: TraceInputID, Family: "input-trace.v1", Layer: "input", Path: "schemas/input-trace.v1.schema.json"})
	envelopes := []string{}
	for _, kind := range []string{"artifact", "publication", "compact-publication", "publication-error", "domain-error"} {
		base := "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-" + kind + ".v1.schema.json"
		id := TraceEnvelopeID(base)
		c.Schemas = append(c.Schemas, SchemaRegistration{ID: id, Family: strings.TrimSuffix(strings.TrimPrefix(id, "https://jaresty.github.io/lsp-trace/mcp/schemas/"), ".schema.json"), Layer: "envelope", Path: "schemas/" + strings.TrimPrefix(id, "https://jaresty.github.io/lsp-trace/mcp/schemas/")})
		envelopes = append(envelopes, id)
	}
	c.Tools = append(c.Tools, ToolContract{Name: TraceTool, Aliases: []string{}, InputSchemaID: TraceInputID, EnvelopeSchemaIDs: envelopes, ArtifactSchemaIDs: []string{GraphProvenanceV5ArtifactID}, Advertised: true, Availability: "ENABLED"})
	return &c
}

func traceSchema(name string) ([]byte, bool, error) {
	if name == "testdata/schemas/input-trace.v1.schema.json" {
		raw, err := json.Marshal(traceInputSchema())
		return raw, true, err
	}
	if !strings.HasPrefix(name, "testdata/schemas/envelope-trace-") {
		return nil, false, nil
	}
	old := strings.Replace(name, "envelope-trace-", "envelope-", 1)
	raw, err := contractFiles.ReadFile(old)
	if err != nil {
		return nil, true, err
	}
	var s map[string]any
	if err = json.Unmarshal(raw, &s); err != nil {
		return nil, true, err
	}
	id := TraceEnvelopeID(s["$id"].(string))
	s["$id"] = id
	p := s["properties"].(map[string]any)
	p["envelope_schema_id"] = map[string]any{"const": id}
	p["tool"] = map[string]any{"const": TraceTool}
	if _, ok := p["artifact_schema_id"]; ok {
		p["artifact_schema_id"] = map[string]any{"const": GraphProvenanceV5ArtifactID}
		p["custody_receipt"] = map[string]any{"type": "object"}
	}
	raw, err = json.Marshal(s)
	return raw, true, err
}

func traceInputSchema() map[string]any {
	integer := func(min, max uint64) map[string]any {
		return map[string]any{"type": "integer", "minimum": min, "maximum": max}
	}
	text := func(format ...string) map[string]any {
		m := map[string]any{"type": "string", "minLength": 1}
		if len(format) > 0 {
			m["format"] = format[0]
		}
		return m
	}
	props := map[string]any{"session_id": text(), "generation": integer(1, ^uint64(0)), "uri": text("uri"), "symbol": text(), "line": integer(0, 4294967295), "character": integer(0, 4294967295), "language_id": text(), "down_depth": integer(0, 64), "up_depth": integer(0, 64), "max_nodes": integer(1, 10000), "timeout_ms": integer(1, 60000), "request_timeout_ms": integer(1, 60000), "topmost_siblings": map[string]any{"type": "boolean"}, "detail": map[string]any{"type": "string", "enum": []any{"compact", "full"}}, "output_selector": text()}
	base := map[string]any{"type": "object", "additionalProperties": false, "properties": props, "required": []any{"session_id", "uri"}}
	base["oneOf"] = []any{map[string]any{"required": []any{"symbol"}, "not": map[string]any{"anyOf": []any{map[string]any{"required": []any{"line"}}, map[string]any{"required": []any{"character"}}}}}, map[string]any{"required": []any{"line", "character"}, "not": map[string]any{"required": []any{"symbol"}}}}
	base["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	base["$id"] = TraceInputID
	return base
}
