package mcpcontract

import (
	"encoding/json"
	hi "lsp-trace/internal/hydratedinspection"
	"strings"
)

const HydratedTool = "lsp_trace_v1_inspect_hydrated"

func HydratedEnvelopeID(id string) string {
	return strings.Replace(id, "/envelope-", "/envelope-inspect-hydrated-", 1)
}

// WithHydratedInspection composes a separate operation-specific contract without
// changing the frozen thirteen-tool manifest or retained-calls registrations.
func WithHydratedInspection(m *Manifest) *Manifest {
	c := *m
	c.Tools = append([]ToolContract{}, m.Tools...)
	c.Schemas = append([]SchemaRegistration{}, m.Schemas...)
	c.Schemas = append(c.Schemas, SchemaRegistration{ID: hi.InputSchemaID, Family: "input-inspect-hydrated.v1", Layer: "input", Path: "schemas/input-inspect-hydrated.v1.schema.json"}, SchemaRegistration{ID: hi.SchemaID, Family: "output-inspect-hydrated.v1", Layer: "artifact", Path: "schemas/output-inspect-hydrated.v1.schema.json"})
	envelopes := []string{}
	for _, kind := range []string{"artifact", "domain-error"} {
		id := HydratedEnvelopeID("https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-" + kind + ".v1.schema.json")
		name := strings.TrimPrefix(id, "https://jaresty.github.io/lsp-trace/mcp/")
		c.Schemas = append(c.Schemas, SchemaRegistration{ID: id, Family: strings.TrimSuffix(strings.TrimPrefix(name, "schemas/"), ".schema.json"), Layer: "envelope", Path: name})
		envelopes = append(envelopes, id)
	}
	c.Tools = append(c.Tools, ToolContract{Name: HydratedTool, Aliases: []string{}, InputSchemaID: hi.InputSchemaID, EnvelopeSchemaIDs: envelopes, ArtifactSchemaIDs: []string{hi.SchemaID}, Advertised: true, Availability: "ENABLED"})
	return &c
}
func readPublicContractSchema(name string) ([]byte, error) {
	switch name {
	case "testdata/schemas/input-inspect-hydrated.v1.schema.json":
		return hi.InputSchema(), nil
	case "testdata/schemas/output-inspect-hydrated.v1.schema.json":
		return hi.OutputSchema(), nil
	}
	if strings.HasPrefix(name, "testdata/schemas/envelope-inspect-hydrated-") {
		old := strings.Replace(name, "envelope-inspect-hydrated-", "envelope-", 1)
		raw, err := contractFiles.ReadFile(old)
		if err != nil {
			return nil, err
		}
		var s map[string]any
		if err = json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		id := HydratedEnvelopeID(s["$id"].(string))
		s["$id"] = id
		p := s["properties"].(map[string]any)
		p["envelope_schema_id"] = map[string]any{"const": id}
		p["tool"] = map[string]any{"const": HydratedTool}
		if _, ok := p["artifact_schema_id"]; ok {
			p["artifact_schema_id"] = map[string]any{"const": hi.SchemaID}
		}
		return json.Marshal(s)
	}
	return readContractSchema(name)
}
