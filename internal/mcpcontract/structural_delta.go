package mcpcontract

import (
	"embed"
	"encoding/json"
	"errors"

	"lsp-trace/internal/strictjson"
)

const (
	StructuralDeltaTool          = "lsp_trace_v1_structural_delta"
	StructuralDeltaInputID       = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-structural-delta.v1.schema.json"
	StructuralDeltaResultID      = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.transient-structural-delta.v1.schema.json"
	StructuralDeltaSuccessID     = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-structural-delta-result.v1.schema.json"
	StructuralDeltaDomainErrorID = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-structural-delta-domain-error.v1.schema.json"
)

func WithStructuralDelta(m *Manifest) *Manifest {
	c := *m
	c.Schemas = append([]SchemaRegistration{}, m.Schemas...)
	c.Tools = append([]ToolContract{}, m.Tools...)
	c.Schemas = append(c.Schemas,
		SchemaRegistration{ID: StructuralDeltaInputID, Family: "input-structural-delta.v1", Layer: "input", Path: "schemas/input-structural-delta.v1.schema.json"},
		SchemaRegistration{ID: StructuralDeltaResultID, Family: "transient-structural-delta.v1", Layer: "artifact", Path: "schemas/lsp-trace.transient-structural-delta.v1.schema.json"},
		SchemaRegistration{ID: StructuralDeltaSuccessID, Family: "envelope-structural-delta-result.v1", Layer: "envelope", Path: "schemas/envelope-structural-delta-result.v1.schema.json"},
		SchemaRegistration{ID: StructuralDeltaDomainErrorID, Family: "envelope-structural-delta-domain-error.v1", Layer: "envelope", Path: "schemas/envelope-structural-delta-domain-error.v1.schema.json"})
	c.Tools = append(c.Tools, ToolContract{Name: StructuralDeltaTool, InputSchemaID: StructuralDeltaInputID, EnvelopeSchemaIDs: []string{StructuralDeltaSuccessID, StructuralDeltaDomainErrorID}, ArtifactSchemaIDs: []string{StructuralDeltaResultID}, Advertised: true, Availability: "ENABLED"})
	return &c
}

//go:embed testdata/schemas/*structural-delta*.json
var structuralDeltaFiles embed.FS

func ValidateStructuralDeltaEnvelopeExclusive(raw []byte) error {
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return err
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	id, _ := value["envelope_schema_id"].(string)
	if id != StructuralDeltaSuccessID && id != StructuralDeltaDomainErrorID {
		return errors.New("invalid structural delta envelope schema")
	}
	return ValidateJSON(id, raw)
}

func structuralDeltaSchema(name string) ([]byte, bool, error) {
	paths := map[string]string{StructuralDeltaInputID: "testdata/schemas/input-structural-delta.v1.schema.json", StructuralDeltaResultID: "testdata/schemas/lsp-trace.transient-structural-delta.v1.schema.json", StructuralDeltaSuccessID: "testdata/schemas/envelope-structural-delta-result.v1.schema.json", StructuralDeltaDomainErrorID: "testdata/schemas/envelope-structural-delta-domain-error.v1.schema.json"}
	for id, p := range paths {
		if name == p || name == id {
			b, e := structuralDeltaFiles.ReadFile(p)
			return b, true, e
		}
	}
	return nil, false, nil
}
