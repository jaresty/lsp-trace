package mcpcontract

import (
	"embed"
	"encoding/json"
	"errors"

	"lsp-trace/internal/strictjson"
)

const (
	ContextSymbolChurnTool          = "lsp_trace_v1_context_symbol_churn"
	ContextSymbolChurnInputID       = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-context-symbol-churn.v1.schema.json"
	ContextSymbolChurnResultID      = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.vcs-symbol-churn-sidecar.v2.schema.json"
	ContextSymbolChurnResultV3ID    = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.vcs-symbol-churn-sidecar.v3.schema.json"
	ContextSymbolChurnSuccessID     = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-context-symbol-churn-result.v1.schema.json"
	ContextSymbolChurnSuccessV2ID   = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-context-symbol-churn-result.v2.schema.json"
	ContextSymbolChurnSuccessV3ID   = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-context-symbol-churn-result.v3.schema.json"
	ContextSymbolChurnDomainErrorID = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-context-symbol-churn-domain-error.v1.schema.json"
)

func WithContextSymbolChurn(m *Manifest) *Manifest {
	c := *m
	c.Schemas = append([]SchemaRegistration{}, m.Schemas...)
	c.Tools = append([]ToolContract{}, m.Tools...)
	c.Schemas = append(c.Schemas,
		SchemaRegistration{ID: ContextSymbolChurnInputID, Family: "input-context-symbol-churn.v1", Layer: "input", Path: "schemas/input-context-symbol-churn.v1.schema.json"},
		SchemaRegistration{ID: ContextSymbolChurnResultID, Family: "vcs-symbol-churn-sidecar.v2", Layer: "artifact", Path: "schemas/lsp-trace.vcs-symbol-churn-sidecar.v2.schema.json"},
		SchemaRegistration{ID: ContextSymbolChurnResultV3ID, Family: "vcs-symbol-churn-sidecar.v3", Layer: "artifact", Path: "schemas/lsp-trace.vcs-symbol-churn-sidecar.v3.schema.json"},
		SchemaRegistration{ID: ContextSymbolChurnSuccessID, Family: "envelope-context-symbol-churn-result.v1", Layer: "envelope", Path: "schemas/envelope-context-symbol-churn-result.v1.schema.json"},
		SchemaRegistration{ID: ContextSymbolChurnSuccessV2ID, Family: "envelope-context-symbol-churn-result.v2", Layer: "envelope", Path: "schemas/envelope-context-symbol-churn-result.v2.schema.json"},
		SchemaRegistration{ID: ContextSymbolChurnSuccessV3ID, Family: "envelope-context-symbol-churn-result.v3", Layer: "envelope", Path: "schemas/envelope-context-symbol-churn-result.v3.schema.json"},
		SchemaRegistration{ID: ContextSymbolChurnDomainErrorID, Family: "envelope-context-symbol-churn-domain-error.v1", Layer: "envelope", Path: "schemas/envelope-context-symbol-churn-domain-error.v1.schema.json"})
	c.Tools = append(c.Tools, ToolContract{Name: ContextSymbolChurnTool, InputSchemaID: ContextSymbolChurnInputID, EnvelopeSchemaIDs: []string{ContextSymbolChurnSuccessID, ContextSymbolChurnSuccessV2ID, ContextSymbolChurnSuccessV3ID, ContextSymbolChurnDomainErrorID}, ArtifactSchemaIDs: []string{ContextSymbolChurnResultID, ContextSymbolChurnResultV3ID}, Advertised: true, Availability: "ENABLED"})
	return &c
}

//go:embed testdata/schemas/*context-symbol-churn*.json testdata/schemas/lsp-trace.vcs-symbol-churn-sidecar.v2.schema.json testdata/schemas/lsp-trace.vcs-symbol-churn-sidecar.v3.schema.json
var contextSymbolChurnFiles embed.FS

func ValidateContextSymbolChurnEnvelopeExclusive(raw []byte) error {
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return err
	}
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		return err
	}
	id, _ := v["envelope_schema_id"].(string)
	if id != ContextSymbolChurnSuccessID && id != ContextSymbolChurnSuccessV2ID && id != ContextSymbolChurnSuccessV3ID && id != ContextSymbolChurnDomainErrorID {
		return errors.New("invalid context symbol churn envelope schema")
	}
	return ValidateJSON(id, raw)
}
func contextSymbolChurnSchema(name string) ([]byte, bool, error) {
	paths := map[string]string{ContextSymbolChurnInputID: "testdata/schemas/input-context-symbol-churn.v1.schema.json", ContextSymbolChurnResultID: "testdata/schemas/lsp-trace.vcs-symbol-churn-sidecar.v2.schema.json", ContextSymbolChurnResultV3ID: "testdata/schemas/lsp-trace.vcs-symbol-churn-sidecar.v3.schema.json", ContextSymbolChurnSuccessID: "testdata/schemas/envelope-context-symbol-churn-result.v1.schema.json", ContextSymbolChurnSuccessV2ID: "testdata/schemas/envelope-context-symbol-churn-result.v2.schema.json", ContextSymbolChurnSuccessV3ID: "testdata/schemas/envelope-context-symbol-churn-result.v3.schema.json", ContextSymbolChurnDomainErrorID: "testdata/schemas/envelope-context-symbol-churn-domain-error.v1.schema.json"}
	for id, p := range paths {
		if name == id || name == p {
			b, e := contextSymbolChurnFiles.ReadFile(p)
			return b, true, e
		}
	}
	return nil, false, nil
}
