package mcpcontract

import (
	"embed"
	"encoding/json"
	"errors"

	"lsp-trace/internal/strictjson"
)

const (
	ContextSymbolChurnCaptureTool          = "lsp_trace_v1_context_symbol_churn_capture"
	ContextSymbolChurnCaptureInputID       = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-context-symbol-churn-capture.v1.schema.json"
	ContextSymbolChurnCaptureSuccessID     = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-context-symbol-churn-capture-result.v1.schema.json"
	ContextSymbolChurnCaptureDomainErrorID = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-context-symbol-churn-capture-domain-error.v1.schema.json"
)

func WithContextSymbolChurnCapture(m *Manifest) *Manifest {
	c := *m
	c.Schemas = append([]SchemaRegistration{}, m.Schemas...)
	c.Tools = append([]ToolContract{}, m.Tools...)
	c.Schemas = append(c.Schemas,
		SchemaRegistration{ID: ContextSymbolChurnCaptureInputID, Family: "input-context-symbol-churn-capture.v1", Layer: "input", Path: "schemas/input-context-symbol-churn-capture.v1.schema.json"},
		SchemaRegistration{ID: ContextSymbolChurnCaptureSuccessID, Family: "envelope-context-symbol-churn-capture-result.v1", Layer: "envelope", Path: "schemas/envelope-context-symbol-churn-capture-result.v1.schema.json"},
		SchemaRegistration{ID: ContextSymbolChurnCaptureDomainErrorID, Family: "envelope-context-symbol-churn-capture-domain-error.v1", Layer: "envelope", Path: "schemas/envelope-context-symbol-churn-capture-domain-error.v1.schema.json"})
	c.Tools = append(c.Tools, ToolContract{Name: ContextSymbolChurnCaptureTool, InputSchemaID: ContextSymbolChurnCaptureInputID, EnvelopeSchemaIDs: []string{ContextSymbolChurnCaptureSuccessID, ContextSymbolChurnCaptureDomainErrorID}, ArtifactSchemaIDs: []string{ContextSymbolChurnResultID}, Advertised: true, Availability: "ENABLED"})
	return &c
}

//go:embed testdata/schemas/*context-symbol-churn-capture*.json
var contextSymbolChurnCaptureFiles embed.FS

func contextSymbolChurnCaptureSchema(name string) ([]byte, bool, error) {
	paths := map[string]string{ContextSymbolChurnCaptureInputID: "testdata/schemas/input-context-symbol-churn-capture.v1.schema.json", ContextSymbolChurnCaptureSuccessID: "testdata/schemas/envelope-context-symbol-churn-capture-result.v1.schema.json", ContextSymbolChurnCaptureDomainErrorID: "testdata/schemas/envelope-context-symbol-churn-capture-domain-error.v1.schema.json"}
	for id, path := range paths {
		if name == id || name == path {
			b, err := contextSymbolChurnCaptureFiles.ReadFile(path)
			return b, true, err
		}
	}
	return nil, false, nil
}

func ValidateContextSymbolChurnCaptureEnvelopeExclusive(raw []byte) error {
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return err
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	id, _ := value["envelope_schema_id"].(string)
	if id != ContextSymbolChurnCaptureSuccessID && id != ContextSymbolChurnCaptureDomainErrorID {
		return errors.New("invalid context symbol churn capture envelope schema")
	}
	return ValidateJSON(id, raw)
}
