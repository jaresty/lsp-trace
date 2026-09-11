package mcpcontract

import (
	"encoding/json"
	"strings"
)

const (
	ProgramCLeidenTool               = "lsp_trace_v1_program_c_leiden"
	ProgramCLeidenInputID            = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-program-c-leiden.v1.schema.json"
	ProgramCLeidenArtifactID         = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.community-presentation.v1.schema.json"
	ProgramCLeidenArtifactEnvelopeID = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-program-c-leiden-artifact.v1.schema.json"
	ProgramCLeidenDomainEnvelopeID   = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-program-c-leiden-domain-error.v1.schema.json"
)

func ProgramCLeidenEnvelopeID(id string) string {
	switch id {
	case "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-artifact.v1.schema.json":
		return ProgramCLeidenArtifactEnvelopeID
	case "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-domain-error.v1.schema.json":
		return ProgramCLeidenDomainEnvelopeID
	default:
		return id
	}
}

// WithProgramCLeiden appends the certified, transport-neutral Program C Leiden
// presentation without changing any existing operation contract.
func WithProgramCLeiden(manifest *Manifest) *Manifest {
	copy := *manifest
	copy.Schemas = append([]SchemaRegistration{}, manifest.Schemas...)
	copy.Schemas = append(copy.Schemas,
		SchemaRegistration{ID: ProgramCLeidenInputID, Family: "input-program-c-leiden.v1", Layer: "input", Path: "schemas/input-program-c-leiden.v1.schema.json"},
		SchemaRegistration{ID: ProgramCLeidenArtifactID, Family: "lsp-trace.community-presentation.v1", Layer: "artifact", Path: "../../schema/schemas/lsp-trace.community-presentation.v1.schema.json"},
		SchemaRegistration{ID: ProgramCLeidenArtifactEnvelopeID, Family: "envelope-program-c-leiden-artifact.v1", Layer: "envelope", Path: "schemas/envelope-program-c-leiden-artifact.v1.schema.json"},
		SchemaRegistration{ID: ProgramCLeidenDomainEnvelopeID, Family: "envelope-program-c-leiden-domain-error.v1", Layer: "envelope", Path: "schemas/envelope-program-c-leiden-domain-error.v1.schema.json"},
	)
	copy.Tools = append([]ToolContract{}, manifest.Tools...)
	copy.Tools = append(copy.Tools, ToolContract{Name: ProgramCLeidenTool, Aliases: []string{}, InputSchemaID: ProgramCLeidenInputID, EnvelopeSchemaIDs: []string{ProgramCLeidenArtifactEnvelopeID, ProgramCLeidenDomainEnvelopeID}, ArtifactSchemaIDs: []string{ProgramCLeidenArtifactID}, Advertised: true, Availability: "ENABLED"})
	return &copy
}

func programCLeidenSchema(name string) ([]byte, bool, error) {
	switch name {
	case "testdata/schemas/input-program-c-leiden.v1.schema.json":
		return []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://jaresty.github.io/lsp-trace/mcp/schemas/input-program-c-leiden.v1.schema.json","type":"object","additionalProperties":false,"required":["input","seed","pagerank_top_k","hub_top_k"],"properties":{"input":{"type":"string","minLength":1,"maxLength":1048576},"seed":{"type":"integer","minimum":0},"pagerank_top_k":{"type":"integer","minimum":1,"maximum":10000},"hub_top_k":{"type":"integer","minimum":1,"maximum":10000}}}`), true, nil
	case "testdata/schemas/envelope-program-c-leiden-artifact.v1.schema.json", "testdata/schemas/envelope-program-c-leiden-domain-error.v1.schema.json":
		kind := "artifact"
		id := ProgramCLeidenArtifactEnvelopeID
		if strings.Contains(name, "domain-error") {
			kind = "domain-error"
			id = ProgramCLeidenDomainEnvelopeID
		}
		raw, err := contractFiles.ReadFile("testdata/schemas/envelope-" + kind + ".v1.schema.json")
		if err != nil {
			return nil, true, err
		}
		var schema map[string]any
		if err := json.Unmarshal(raw, &schema); err != nil {
			return nil, true, err
		}
		schema["$id"] = id
		properties := schema["properties"].(map[string]any)
		properties["envelope_schema_id"] = map[string]any{"const": id}
		properties["tool"] = map[string]any{"const": ProgramCLeidenTool}
		if _, ok := properties["artifact_schema_id"]; ok {
			properties["artifact_schema_id"] = map[string]any{"const": ProgramCLeidenArtifactID}
		}
		encoded, err := json.Marshal(schema)
		return encoded, true, err
	default:
		return nil, false, nil
	}
}
