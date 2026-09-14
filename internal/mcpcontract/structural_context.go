package mcpcontract

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"strings"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"lsp-trace/internal/strictjson"
)

const (
	StructuralContextTool          = "lsp_trace_v1_structural_context"
	StructuralContextInputID       = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-structural-context.v1.schema.json"
	StructuralContextResultID      = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.transient-structural-result.v1.schema.json"
	StructuralContextSuccessID     = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-structural-context-result.v1.schema.json"
	StructuralContextDomainErrorID = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-structural-context-domain-error.v1.schema.json"
)

func WithStructuralContext(m *Manifest) *Manifest {
	c := *m
	c.Schemas = append([]SchemaRegistration{}, m.Schemas...)
	c.Tools = append([]ToolContract{}, m.Tools...)
	c.Schemas = append(c.Schemas,
		SchemaRegistration{ID: StructuralContextInputID, Family: "input-structural-context.v1", Layer: "input", Path: "schemas/input-structural-context.v1.schema.json"},
		SchemaRegistration{ID: StructuralContextResultID, Family: "transient-structural-result.v1", Layer: "artifact", Path: "schemas/lsp-trace.transient-structural-result.v1.schema.json"},
		SchemaRegistration{ID: StructuralContextSuccessID, Family: "envelope-structural-context-result.v1", Layer: "envelope", Path: "schemas/envelope-structural-context-result.v1.schema.json"},
		SchemaRegistration{ID: StructuralContextDomainErrorID, Family: "envelope-structural-context-domain-error.v1", Layer: "envelope", Path: "schemas/envelope-structural-context-domain-error.v1.schema.json"},
	)
	c.Tools = append(c.Tools, ToolContract{Name: StructuralContextTool, Aliases: []string{}, InputSchemaID: StructuralContextInputID, EnvelopeSchemaIDs: []string{StructuralContextSuccessID, StructuralContextDomainErrorID}, ArtifactSchemaIDs: []string{StructuralContextResultID}, Advertised: true, Availability: "ENABLED"})
	return &c
}

//go:embed testdata/schemas/input-structural-context.v1.schema.json testdata/schemas/lsp-trace.transient-structural-result.v1.schema.json testdata/schemas/envelope-structural-context-result.v1.schema.json testdata/schemas/envelope-structural-context-domain-error.v1.schema.json
var structuralContextFiles embed.FS

func StructuralContextSchemaJSON(id string) ([]byte, error) {
	paths := map[string]string{StructuralContextInputID: "testdata/schemas/input-structural-context.v1.schema.json", StructuralContextResultID: "testdata/schemas/lsp-trace.transient-structural-result.v1.schema.json", StructuralContextSuccessID: "testdata/schemas/envelope-structural-context-result.v1.schema.json", StructuralContextDomainErrorID: "testdata/schemas/envelope-structural-context-domain-error.v1.schema.json"}
	p, ok := paths[id]
	if !ok {
		return nil, errors.New("unknown structural context schema")
	}
	raw, err := structuralContextFiles.ReadFile(p)
	return append([]byte(nil), raw...), err
}

func structuralContextSchema(name string) ([]byte, bool, error) {
	if !strings.HasPrefix(name, "testdata/schemas/") {
		return nil, false, nil
	}
	for id, p := range map[string]string{StructuralContextInputID: "testdata/schemas/input-structural-context.v1.schema.json", StructuralContextResultID: "testdata/schemas/lsp-trace.transient-structural-result.v1.schema.json", StructuralContextSuccessID: "testdata/schemas/envelope-structural-context-result.v1.schema.json", StructuralContextDomainErrorID: "testdata/schemas/envelope-structural-context-domain-error.v1.schema.json"} {
		if name == p {
			r, e := StructuralContextSchemaJSON(id)
			return r, true, e
		}
	}
	return nil, false, nil
}

func ValidateStructuralContextEnvelopeExclusive(data []byte) error {
	if err := strictjson.RejectDuplicates(data); err != nil {
		return errFutureShape
	}
	var value map[string]any
	if json.Unmarshal(data, &value) != nil {
		return errFutureShape
	}
	named, _ := value["envelope_schema_id"].(string)
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	for _, id := range []string{StructuralContextResultID, StructuralContextSuccessID, StructuralContextDomainErrorID} {
		raw, _ := StructuralContextSchemaJSON(id)
		doc, e := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if e != nil {
			return e
		}
		if e = compiler.AddResource(id, doc); e != nil {
			return e
		}
	}
	doc, e := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if e != nil {
		return errFutureShape
	}
	matches := []string{}
	for _, id := range []string{StructuralContextSuccessID, StructuralContextDomainErrorID} {
		s, e := compiler.Compile(id)
		if e != nil {
			return e
		}
		if s.Validate(doc) == nil {
			matches = append(matches, id)
		}
	}
	if len(matches) != 1 || matches[0] != named {
		return errFutureShape
	}
	return nil
}

func ValidateStructuralContextRequestV1(v map[string]any) error { return validateFutureInput(v) }
func ValidateStructuralContextSemanticsV1(input, result map[string]any) error {
	return ValidateFutureStructuralSemanticsV1(input, result)
}
