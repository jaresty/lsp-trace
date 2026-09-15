package mcpcontract

import (
	"embed"
	"encoding/json"
	"errors"

	"lsp-trace/internal/strictjson"
)

const (
	ContextChurnTool          = "lsp_trace_v1_context_churn"
	ContextChurnInputID       = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-context-churn.v1.schema.json"
	ContextChurnResultID      = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.vcs-churn-sidecar.v1.schema.json"
	ContextChurnSuccessID     = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-context-churn-result.v1.schema.json"
	ContextChurnDomainErrorID = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-context-churn-domain-error.v1.schema.json"
)

func WithContextChurn(m *Manifest) *Manifest {
	c := *m
	c.Schemas = append([]SchemaRegistration{}, m.Schemas...)
	c.Tools = append([]ToolContract{}, m.Tools...)
	c.Schemas = append(c.Schemas,
		SchemaRegistration{ID: ContextChurnInputID, Family: "input-context-churn.v1", Layer: "input", Path: "schemas/input-context-churn.v1.schema.json"},
		SchemaRegistration{ID: ContextChurnResultID, Family: "vcs-churn-sidecar.v1", Layer: "artifact", Path: "schemas/lsp-trace.vcs-churn-sidecar.v1.schema.json"},
		SchemaRegistration{ID: ContextChurnSuccessID, Family: "envelope-context-churn-result.v1", Layer: "envelope", Path: "schemas/envelope-context-churn-result.v1.schema.json"},
		SchemaRegistration{ID: ContextChurnDomainErrorID, Family: "envelope-context-churn-domain-error.v1", Layer: "envelope", Path: "schemas/envelope-context-churn-domain-error.v1.schema.json"})
	c.Tools = append(c.Tools, ToolContract{Name: ContextChurnTool, InputSchemaID: ContextChurnInputID, EnvelopeSchemaIDs: []string{ContextChurnSuccessID, ContextChurnDomainErrorID}, ArtifactSchemaIDs: []string{ContextChurnResultID}, Advertised: true, Availability: "ENABLED"})
	return &c
}

//go:embed testdata/schemas/*context-churn*.json testdata/schemas/lsp-trace.vcs-churn-sidecar.v1.schema.json
var contextChurnFiles embed.FS

func ValidateContextChurnEnvelopeExclusive(raw []byte) error {
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return err
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	id, _ := value["envelope_schema_id"].(string)
	if id != ContextChurnSuccessID && id != ContextChurnDomainErrorID {
		return errors.New("invalid context churn envelope schema")
	}
	return ValidateJSON(id, raw)
}

func contextChurnSchema(name string) ([]byte, bool, error) {
	paths := map[string]string{
		ContextChurnInputID:       "testdata/schemas/input-context-churn.v1.schema.json",
		ContextChurnResultID:      "testdata/schemas/lsp-trace.vcs-churn-sidecar.v1.schema.json",
		ContextChurnSuccessID:     "testdata/schemas/envelope-context-churn-result.v1.schema.json",
		ContextChurnDomainErrorID: "testdata/schemas/envelope-context-churn-domain-error.v1.schema.json",
	}
	for id, p := range paths {
		if name == p || name == id {
			b, err := contextChurnFiles.ReadFile(p)
			return b, true, err
		}
	}
	return nil, false, nil
}
