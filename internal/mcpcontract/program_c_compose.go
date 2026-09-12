package mcpcontract

import (
	"encoding/json"
	"strings"
)

const (
	ProgramCComposeTool               = "lsp_trace_v1_program_c_compose"
	ProgramCComposeInputID            = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-program-c-compose.v1.schema.json"
	ProgramCComposeArtifactID         = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.private.program-c-multi-capture-composite.v1.schema.json"
	ProgramCComposeArtifactEnvelopeID = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-program-c-compose-artifact.v1.schema.json"
	ProgramCComposeDomainEnvelopeID   = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-program-c-compose-domain-error.v1.schema.json"
)

func ProgramCComposeEnvelopeID(id string) string {
	switch id {
	case "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-artifact.v1.schema.json":
		return ProgramCComposeArtifactEnvelopeID
	case "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-domain-error.v1.schema.json":
		return ProgramCComposeDomainEnvelopeID
	default:
		return id
	}
}

func WithProgramCCompose(manifest *Manifest) *Manifest {
	copy := *manifest
	copy.Schemas = append([]SchemaRegistration{}, manifest.Schemas...)
	copy.Schemas = append(copy.Schemas,
		SchemaRegistration{ID: ProgramCComposeInputID, Family: "input-program-c-compose.v1", Layer: "input", Path: "schemas/input-program-c-compose.v1.schema.json"},
		SchemaRegistration{ID: ProgramCComposeArtifactID, Family: "lsp-trace.private.program-c-multi-capture-composite.v1", Layer: "artifact", Path: "schemas/lsp-trace.private.program-c-multi-capture-composite.v1.schema.json"},
		SchemaRegistration{ID: ProgramCComposeArtifactEnvelopeID, Family: "envelope-program-c-compose-artifact.v1", Layer: "envelope", Path: "schemas/envelope-program-c-compose-artifact.v1.schema.json"},
		SchemaRegistration{ID: ProgramCComposeDomainEnvelopeID, Family: "envelope-program-c-compose-domain-error.v1", Layer: "envelope", Path: "schemas/envelope-program-c-compose-domain-error.v1.schema.json"},
	)
	copy.Tools = append([]ToolContract{}, manifest.Tools...)
	copy.Tools = append(copy.Tools, ToolContract{Name: ProgramCComposeTool, InputSchemaID: ProgramCComposeInputID, EnvelopeSchemaIDs: []string{ProgramCComposeArtifactEnvelopeID, ProgramCComposeDomainEnvelopeID}, ArtifactSchemaIDs: []string{ProgramCComposeArtifactID}, Advertised: true, Availability: "ENABLED"})
	return &copy
}

func programCComposeSchema(name string) ([]byte, bool, error) {
	switch name {
	case "testdata/schemas/input-program-c-compose.v1.schema.json":
		return []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://jaresty.github.io/lsp-trace/mcp/schemas/input-program-c-compose.v1.schema.json","type":"object","additionalProperties":false,"required":["captures"],"properties":{"captures":{"type":"array","minItems":2,"maxItems":16,"items":{"type":"object","additionalProperties":false,"required":["input","identity","sha256","byte_length","exact_metadata"],"properties":{"input":{"type":"string","minLength":1,"maxLength":268435456},"identity":{"type":"string","minLength":1,"maxLength":4096},"sha256":{"type":"string","pattern":"^sha256:[0-9a-f]{64}$"},"byte_length":{"type":"integer","minimum":1,"maximum":268435456},"exact_metadata":{"type":"object","additionalProperties":false,"required":["workspace_identity","revision_custody","position_encoding","acquisition_semantics","privacy_policy"],"properties":{"workspace_identity":{"type":"string","minLength":1},"revision_custody":{"type":"string","minLength":1},"position_encoding":{"type":"string","minLength":1},"acquisition_semantics":{"type":"string","minLength":1},"privacy_policy":{"type":"string","minLength":1}}}}}}}}`), true, nil
	case "testdata/schemas/lsp-trace.private.program-c-multi-capture-composite.v1.schema.json":
		return []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://jaresty.github.io/lsp-trace/schemas/lsp-trace.private.program-c-multi-capture-composite.v1.schema.json","type":"object","additionalProperties":false,"required":["Version","PolicyVersion","PolicySHA256","CompositeID","OutputSHA256","ClaimCeiling","Constituents","Compatibility","Work","Nodes","Edges","Completeness"],"properties":{"Version":{"const":"lsp-trace.private.program-c-multi-capture-composite.v1"},"PolicyVersion":{"const":"program-c-multi-capture-policy.v1"},"PolicySHA256":{"type":"string"},"CompositeID":{"type":"string"},"OutputSHA256":{"type":"string"},"ClaimCeiling":{"type":"string"},"Constituents":{"type":"array","minItems":2,"maxItems":16},"Compatibility":{"type":"object"},"Work":{"type":"object"},"Nodes":{"type":"array"},"Edges":{"type":"array"},"Completeness":{"type":"object"}}}`), true, nil
	case "testdata/schemas/envelope-program-c-compose-artifact.v1.schema.json", "testdata/schemas/envelope-program-c-compose-domain-error.v1.schema.json":
		kind, id := "artifact", ProgramCComposeArtifactEnvelopeID
		if strings.Contains(name, "domain-error") {
			kind, id = "domain-error", ProgramCComposeDomainEnvelopeID
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
		props := schema["properties"].(map[string]any)
		props["envelope_schema_id"] = map[string]any{"const": id}
		props["tool"] = map[string]any{"const": ProgramCComposeTool}
		if _, ok := props["artifact_schema_id"]; ok {
			props["artifact_schema_id"] = map[string]any{"const": ProgramCComposeArtifactID}
		}
		encoded, err := json.Marshal(schema)
		return encoded, true, err
	default:
		return nil, false, nil
	}
}
