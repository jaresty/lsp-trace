package mcpcontract

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"lsp-trace/internal/strictjson"
)

const (
	StructuralContextSymbolTool          = "lsp_trace_v1_structural_context_symbol"
	StructuralContextSymbolInputID       = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-structural-context-symbol.v1.schema.json"
	StructuralContextSymbolSuccessID     = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-structural-context-symbol-result.v1.schema.json"
	StructuralContextSymbolDomainErrorID = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-structural-context-symbol-domain-error.v1.schema.json"
)

func WithStructuralContextSymbol(m *Manifest) *Manifest {
	c := *m
	c.Schemas = append([]SchemaRegistration{}, m.Schemas...)
	c.Tools = append([]ToolContract{}, m.Tools...)
	c.Schemas = append(c.Schemas,
		SchemaRegistration{ID: StructuralContextSymbolInputID, Family: "input-structural-context-symbol.v1", Layer: "input", Path: "schemas/input-structural-context-symbol.v1.schema.json"},
		SchemaRegistration{ID: StructuralContextSymbolSuccessID, Family: "envelope-structural-context-symbol-result.v1", Layer: "envelope", Path: "schemas/envelope-structural-context-symbol-result.v1.schema.json"},
		SchemaRegistration{ID: StructuralContextSymbolDomainErrorID, Family: "envelope-structural-context-symbol-domain-error.v1", Layer: "envelope", Path: "schemas/envelope-structural-context-symbol-domain-error.v1.schema.json"})
	c.Tools = append(c.Tools, ToolContract{Name: StructuralContextSymbolTool, Aliases: []string{"lsp_trace_structural_context_symbol"}, InputSchemaID: StructuralContextSymbolInputID, EnvelopeSchemaIDs: []string{StructuralContextSymbolSuccessID, StructuralContextSymbolDomainErrorID}, ArtifactSchemaIDs: []string{StructuralContextResultID}, Advertised: true, Availability: "ENABLED"})
	return &c
}

//go:embed testdata/schemas/*structural-context-symbol*.json
var structuralContextSymbolFiles embed.FS

func structuralContextSymbolSchema(name string) ([]byte, bool, error) {
	paths := map[string]string{
		StructuralContextSymbolInputID:       "testdata/schemas/input-structural-context-symbol.v1.schema.json",
		StructuralContextSymbolSuccessID:     "testdata/schemas/envelope-structural-context-symbol-result.v1.schema.json",
		StructuralContextSymbolDomainErrorID: "testdata/schemas/envelope-structural-context-symbol-domain-error.v1.schema.json",
	}
	for id, p := range paths {
		if name == id || name == p {
			raw, err := structuralContextSymbolFiles.ReadFile(p)
			return raw, true, err
		}
	}
	return nil, false, nil
}

func ValidateStructuralContextSymbolEnvelopeExclusive(data []byte) error {
	if err := strictjson.RejectDuplicates(data); err != nil {
		return err
	}
	var value map[string]any
	if json.Unmarshal(data, &value) != nil {
		return errors.New("invalid structural context symbol envelope")
	}
	named, _ := value["envelope_schema_id"].(string)
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	for _, id := range []string{StructuralContextResultID, StructuralContextSymbolSuccessID, StructuralContextSymbolDomainErrorID} {
		var raw []byte
		var err error
		if id == StructuralContextResultID {
			raw, err = StructuralContextSchemaJSON(id)
		} else {
			raw, _, err = structuralContextSymbolSchema(id)
		}
		if err != nil {
			return err
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			return err
		}
		if err = compiler.AddResource(id, doc); err != nil {
			return err
		}
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return err
	}
	matches := 0
	for _, id := range []string{StructuralContextSymbolSuccessID, StructuralContextSymbolDomainErrorID} {
		schema, err := compiler.Compile(id)
		if err != nil {
			return err
		}
		if schema.Validate(doc) == nil {
			matches++
			if id != named {
				return errors.New("envelope schema id mismatch")
			}
		}
	}
	if matches != 1 {
		return errors.New("invalid structural context symbol envelope")
	}
	return nil
}
