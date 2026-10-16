package mcpcontract

import "embed"

const (
	CallHierarchyCompositionTool       = "lsp_trace_v1_call_hierarchy_composition"
	CallHierarchyCompositionInputID    = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-call-hierarchy-composition.v1.schema.json"
	CallHierarchyCompositionResultID   = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.experimental.call-hierarchy-composition.v1.schema.json"
	CallHierarchyCompositionEnvelopeID = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-call-hierarchy-composition-result.v1.schema.json"
)

func WithCallHierarchyComposition(m *Manifest) *Manifest {
	c := *m
	c.Schemas = append([]SchemaRegistration{}, m.Schemas...)
	c.Tools = append([]ToolContract{}, m.Tools...)
	c.Schemas = append(c.Schemas,
		SchemaRegistration{ID: CallHierarchyCompositionInputID, Family: "input-call-hierarchy-composition.v1", Layer: "input", Path: "schemas/input-call-hierarchy-composition.v1.schema.json"},
		SchemaRegistration{ID: CallHierarchyCompositionResultID, Family: "experimental-call-hierarchy-composition.v1", Layer: "artifact", Path: "schemas/lsp-trace.experimental.call-hierarchy-composition.v1.schema.json"},
		SchemaRegistration{ID: CallHierarchyCompositionEnvelopeID, Family: "envelope-call-hierarchy-composition-result.v1", Layer: "envelope", Path: "schemas/envelope-call-hierarchy-composition-result.v1.schema.json"})
	c.Tools = append(c.Tools, ToolContract{Name: CallHierarchyCompositionTool, InputSchemaID: CallHierarchyCompositionInputID, EnvelopeSchemaIDs: []string{CallHierarchyCompositionEnvelopeID}, ArtifactSchemaIDs: []string{CallHierarchyCompositionResultID}, Advertised: true, Availability: "ENABLED"})
	return &c
}

//go:embed testdata/schemas/*call-hierarchy-composition*.json
var callHierarchyCompositionFiles embed.FS

func callHierarchyCompositionSchema(name string) ([]byte, bool, error) {
	paths := map[string]string{
		CallHierarchyCompositionInputID:    "testdata/schemas/input-call-hierarchy-composition.v1.schema.json",
		CallHierarchyCompositionResultID:   "testdata/schemas/lsp-trace.experimental.call-hierarchy-composition.v1.schema.json",
		CallHierarchyCompositionEnvelopeID: "testdata/schemas/envelope-call-hierarchy-composition-result.v1.schema.json",
	}
	for id, p := range paths {
		if name == id || name == p {
			b, err := callHierarchyCompositionFiles.ReadFile(p)
			return b, true, err
		}
	}
	return nil, false, nil
}
