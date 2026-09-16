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
	StructuralContextTool                    = "lsp_trace_v1_structural_context"
	StructuralContextInputID                 = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-structural-context.v1.schema.json"
	StructuralContextResultID                = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.transient-structural-result.v1.schema.json"
	StructuralContextSuccessID               = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-structural-context-result.v1.schema.json"
	StructuralContextDomainErrorID           = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-structural-context-domain-error.v1.schema.json"
	StructuralContextV2Tool                  = "lsp_trace_v2_structural_context"
	StructuralContextV2InputID               = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-structural-context.v2.schema.json"
	StructuralContextUnifiedInputID          = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-structural-context.v3.schema.json"
	StructuralContextV2ResultID              = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.transient-structural-result.v2.schema.json"
	StructuralContextV2SuccessID             = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-structural-context-result.v2.schema.json"
	StructuralContextUnifiedSuccessID        = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-structural-context-result.v3.schema.json"
	StructuralContextV2DomainErrorID         = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-structural-context-domain-error.v2.schema.json"
	SourceProjectionRequestV1ID              = "https://jaresty.github.io/lsp-trace/mcp/schemas/source-projection-request.v1.schema.json"
	SourceProjectionResultV1ID               = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.source-projection.v1.schema.json"
	UnifiedStructuralContextResultID         = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.unified-structural-context-result.v1.schema.json"
	SourceProjectionRequestV2ID              = "https://jaresty.github.io/lsp-trace/mcp/schemas/source-projection-request.v2.schema.json"
	SourceProjectionResultV2ID               = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.source-projection.v2.schema.json"
	StructuralContextProjectionInputID       = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-structural-context.v4.schema.json"
	UnifiedStructuralContextResultV2ID       = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.unified-structural-context-result.v2.schema.json"
	StructuralContextProjectionSuccessID     = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-structural-context-result.v4.schema.json"
	StructuralContextProjectionDomainErrorID = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-structural-context-domain-error.v3.schema.json"
)

func WithStructuralContextV2(m *Manifest) *Manifest {
	c := *m
	c.Schemas = append([]SchemaRegistration{}, m.Schemas...)
	c.Tools = append([]ToolContract{}, m.Tools...)
	c.Schemas = append(c.Schemas,
		SchemaRegistration{ID: StructuralContextV2InputID, Family: "input-structural-context.v2", Layer: "input", Path: "schemas/input-structural-context.v2.schema.json"},
		SchemaRegistration{ID: StructuralContextUnifiedInputID, Family: "input-structural-context.v3", Layer: "input", Path: "schemas/input-structural-context.v3.schema.json"},
		SchemaRegistration{ID: SourceProjectionRequestV1ID, Family: "source-projection-request.v1", Layer: "input", Path: "schemas/source-projection-request.v1.schema.json"},
		SchemaRegistration{ID: StructuralContextV2ResultID, Family: "transient-structural-result.v2", Layer: "artifact", Path: "schemas/lsp-trace.transient-structural-result.v2.schema.json"},
		SchemaRegistration{ID: SourceProjectionResultV1ID, Family: "source-projection.v1", Layer: "artifact", Path: "schemas/lsp-trace.source-projection.v1.schema.json"},
		SchemaRegistration{ID: UnifiedStructuralContextResultID, Family: "unified-structural-context-result.v1", Layer: "artifact", Path: "schemas/lsp-trace.unified-structural-context-result.v1.schema.json"},
		SchemaRegistration{ID: StructuralContextV2SuccessID, Family: "envelope-structural-context-result.v2", Layer: "envelope", Path: "schemas/envelope-structural-context-result.v2.schema.json"},
		SchemaRegistration{ID: StructuralContextUnifiedSuccessID, Family: "envelope-structural-context-result.v3", Layer: "envelope", Path: "schemas/envelope-structural-context-result.v3.schema.json"},
		SchemaRegistration{ID: StructuralContextV2DomainErrorID, Family: "envelope-structural-context-domain-error.v2", Layer: "envelope", Path: "schemas/envelope-structural-context-domain-error.v2.schema.json"},
		SchemaRegistration{ID: SourceProjectionRequestV2ID, Family: "source-projection-request.v2", Layer: "input", Path: "schemas/source-projection-request.v2.schema.json"},
		SchemaRegistration{ID: StructuralContextProjectionInputID, Family: "input-structural-context.v4", Layer: "input", Path: "schemas/input-structural-context.v4.schema.json"},
		SchemaRegistration{ID: SourceProjectionResultV2ID, Family: "source-projection.v2", Layer: "artifact", Path: "schemas/lsp-trace.source-projection.v2.schema.json"},
		SchemaRegistration{ID: UnifiedStructuralContextResultV2ID, Family: "unified-structural-context-result.v2", Layer: "artifact", Path: "schemas/lsp-trace.unified-structural-context-result.v2.schema.json"},
		SchemaRegistration{ID: StructuralContextProjectionSuccessID, Family: "envelope-structural-context-result.v4", Layer: "envelope", Path: "schemas/envelope-structural-context-result.v4.schema.json"},
		SchemaRegistration{ID: StructuralContextProjectionDomainErrorID, Family: "envelope-structural-context-domain-error.v3", Layer: "envelope", Path: "schemas/envelope-structural-context-domain-error.v3.schema.json"},
	)
	c.Tools = append(c.Tools, ToolContract{Name: StructuralContextV2Tool, InputSchemaID: StructuralContextProjectionInputID, EnvelopeSchemaIDs: []string{StructuralContextProjectionSuccessID, StructuralContextProjectionDomainErrorID}, ArtifactSchemaIDs: []string{UnifiedStructuralContextResultV2ID}, Advertised: true, Availability: "ENABLED"})
	return &c
}

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

//go:embed testdata/schemas/input-structural-context.v1.schema.json testdata/schemas/lsp-trace.transient-structural-result.v1.schema.json testdata/schemas/envelope-structural-context-result.v1.schema.json testdata/schemas/envelope-structural-context-domain-error.v1.schema.json testdata/schemas/input-structural-context.v2.schema.json testdata/schemas/input-structural-context.v3.schema.json testdata/schemas/input-structural-context.v4.schema.json testdata/schemas/source-projection-request.v1.schema.json testdata/schemas/source-projection-request.v2.schema.json testdata/schemas/lsp-trace.transient-structural-result.v2.schema.json testdata/schemas/lsp-trace.source-projection.v1.schema.json testdata/schemas/lsp-trace.source-projection.v2.schema.json testdata/schemas/lsp-trace.unified-structural-context-result.v1.schema.json testdata/schemas/lsp-trace.unified-structural-context-result.v2.schema.json testdata/schemas/envelope-structural-context-result.v2.schema.json testdata/schemas/envelope-structural-context-result.v3.schema.json testdata/schemas/envelope-structural-context-result.v4.schema.json testdata/schemas/envelope-structural-context-domain-error.v2.schema.json testdata/schemas/envelope-structural-context-domain-error.v3.schema.json
var structuralContextFiles embed.FS

func structuralContextSchemaPaths() map[string]string {
	return map[string]string{
		StructuralContextInputID:                 "testdata/schemas/input-structural-context.v1.schema.json",
		StructuralContextV2InputID:               "testdata/schemas/input-structural-context.v2.schema.json",
		StructuralContextUnifiedInputID:          "testdata/schemas/input-structural-context.v3.schema.json",
		StructuralContextProjectionInputID:       "testdata/schemas/input-structural-context.v4.schema.json",
		SourceProjectionRequestV1ID:              "testdata/schemas/source-projection-request.v1.schema.json",
		SourceProjectionRequestV2ID:              "testdata/schemas/source-projection-request.v2.schema.json",
		StructuralContextResultID:                "testdata/schemas/lsp-trace.transient-structural-result.v1.schema.json",
		StructuralContextV2ResultID:              "testdata/schemas/lsp-trace.transient-structural-result.v2.schema.json",
		SourceProjectionResultV1ID:               "testdata/schemas/lsp-trace.source-projection.v1.schema.json",
		SourceProjectionResultV2ID:               "testdata/schemas/lsp-trace.source-projection.v2.schema.json",
		UnifiedStructuralContextResultID:         "testdata/schemas/lsp-trace.unified-structural-context-result.v1.schema.json",
		UnifiedStructuralContextResultV2ID:       "testdata/schemas/lsp-trace.unified-structural-context-result.v2.schema.json",
		StructuralContextSuccessID:               "testdata/schemas/envelope-structural-context-result.v1.schema.json",
		StructuralContextV2SuccessID:             "testdata/schemas/envelope-structural-context-result.v2.schema.json",
		StructuralContextUnifiedSuccessID:        "testdata/schemas/envelope-structural-context-result.v3.schema.json",
		StructuralContextProjectionSuccessID:     "testdata/schemas/envelope-structural-context-result.v4.schema.json",
		StructuralContextDomainErrorID:           "testdata/schemas/envelope-structural-context-domain-error.v1.schema.json",
		StructuralContextV2DomainErrorID:         "testdata/schemas/envelope-structural-context-domain-error.v2.schema.json",
		StructuralContextProjectionDomainErrorID: "testdata/schemas/envelope-structural-context-domain-error.v3.schema.json",
	}
}

func StructuralContextSchemaJSON(id string) ([]byte, error) {
	p, ok := structuralContextSchemaPaths()[id]
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
	for id, p := range structuralContextSchemaPaths() {
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

func ValidateStructuralContextV2EnvelopeExclusive(data []byte) error {
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
	for _, id := range []string{StructuralContextV2ResultID, SourceProjectionResultV2ID, UnifiedStructuralContextResultV2ID, StructuralContextProjectionSuccessID, StructuralContextProjectionDomainErrorID} {
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
	for _, id := range []string{StructuralContextProjectionSuccessID, StructuralContextProjectionDomainErrorID} {
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
