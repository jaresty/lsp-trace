package mcpcontract

import (
	"encoding/json"
	"strings"
)

const (
	ProgramCInstabilityTool               = "lsp_trace_v1_program_c_instability"
	ProgramCInstabilityInputID            = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-program-c-instability.v1.schema.json"
	ProgramCInstabilityArtifactID         = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.community-instability.v1.schema.json"
	ProgramCInstabilityArtifactEnvelopeID = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-program-c-instability-artifact.v1.schema.json"
	ProgramCInstabilityDomainEnvelopeID   = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-program-c-instability-domain-error.v1.schema.json"
)

func ProgramCInstabilityEnvelopeID(id string) string {
	switch id {
	case "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-artifact.v1.schema.json":
		return ProgramCInstabilityArtifactEnvelopeID
	case "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-domain-error.v1.schema.json":
		return ProgramCInstabilityDomainEnvelopeID
	default:
		return id
	}
}

func WithProgramCInstability(manifest *Manifest) *Manifest {
	copy := *manifest
	copy.Schemas = append([]SchemaRegistration{}, manifest.Schemas...)
	copy.Schemas = append(copy.Schemas,
		SchemaRegistration{ID: ProgramCInstabilityInputID, Family: "input-program-c-instability.v1", Layer: "input", Path: "schemas/input-program-c-instability.v1.schema.json"},
		SchemaRegistration{ID: ProgramCInstabilityArtifactID, Family: "lsp-trace.community-instability.v1", Layer: "artifact", Path: "../../schema/schemas/lsp-trace.community-instability.v1.schema.json"},
		SchemaRegistration{ID: ProgramCInstabilityArtifactEnvelopeID, Family: "envelope-program-c-instability-artifact.v1", Layer: "envelope", Path: "schemas/envelope-program-c-instability-artifact.v1.schema.json"},
		SchemaRegistration{ID: ProgramCInstabilityDomainEnvelopeID, Family: "envelope-program-c-instability-domain-error.v1", Layer: "envelope", Path: "schemas/envelope-program-c-instability-domain-error.v1.schema.json"},
	)
	copy.Tools = append([]ToolContract{}, manifest.Tools...)
	copy.Tools = append(copy.Tools, ToolContract{Name: ProgramCInstabilityTool, InputSchemaID: ProgramCInstabilityInputID, EnvelopeSchemaIDs: []string{ProgramCInstabilityArtifactEnvelopeID, ProgramCInstabilityDomainEnvelopeID}, ArtifactSchemaIDs: []string{ProgramCInstabilityArtifactID}, Advertised: true, Availability: "ENABLED"})
	return &copy
}

func programCInstabilitySchema(name string) ([]byte, bool, error) {
	switch name {
	case "testdata/schemas/input-program-c-instability.v1.schema.json":
		return []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://jaresty.github.io/lsp-trace/mcp/schemas/input-program-c-instability.v1.schema.json","type":"object","additionalProperties":false,"required":["input","seeds","algorithm_version","parameters_sha256","resource_policy_sha256"],"properties":{"input":{"type":"string","minLength":1,"maxLength":1048576},"seeds":{"type":"array","minItems":2,"maxItems":100,"uniqueItems":true,"items":{"type":"integer","minimum":0}},"algorithm_version":{"type":"string","minLength":1,"maxLength":256},"parameters_sha256":{"type":"string","pattern":"^sha256:[0-9a-f]{64}$"},"resource_policy_sha256":{"type":"string","pattern":"^sha256:[0-9a-f]{64}$"}}}`), true, nil
	case "testdata/schemas/envelope-program-c-instability-artifact.v1.schema.json", "testdata/schemas/envelope-program-c-instability-domain-error.v1.schema.json":
		kind, id := "artifact", ProgramCInstabilityArtifactEnvelopeID
		if strings.Contains(name, "domain-error") {
			kind, id = "domain-error", ProgramCInstabilityDomainEnvelopeID
		}
		raw, err := contractFiles.ReadFile("testdata/schemas/envelope-" + kind + ".v1.schema.json")
		if err != nil {
			return nil, true, err
		}
		var s map[string]any
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, true, err
		}
		p := s["properties"].(map[string]any)
		s["$id"] = id
		p["envelope_schema_id"] = map[string]any{"const": id}
		p["tool"] = map[string]any{"const": ProgramCInstabilityTool}
		if _, ok := p["artifact_schema_id"]; ok {
			p["artifact_schema_id"] = map[string]any{"const": ProgramCInstabilityArtifactID}
		}
		encoded, err := json.Marshal(s)
		return encoded, true, err
	default:
		return nil, false, nil
	}
}
