package mcpcontract

import "strings"

// Additive registration; the historical thirteen-tool Stage 1 manifest stays
// byte-identical. The shared runtime composes this separately versioned input.
const RetainedCallsInputID = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-export-retained-calls.v1.schema.json"
const RetainedCallsArtifactID = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.retained-calls.v1.schema.json"

func RetainedCallsEnvelopeID(id string) string {
	return strings.Replace(id, "/envelope-", "/envelope-retained-calls-", 1)
}

func WithRetainedCalls(manifest *Manifest) *Manifest {
	copy := *manifest
	copy.Schemas = append(append([]SchemaRegistration{}, manifest.Schemas...), SchemaRegistration{ID: RetainedCallsInputID, Family: "input-export-retained-calls.v1", Layer: "input", Path: "schemas/input-export-retained-calls.v1.schema.json"}, SchemaRegistration{ID: RetainedCallsArtifactID, Family: "lsp-trace.retained-calls.v1", Layer: "artifact", Path: "../../schema/schemas/lsp-trace.retained-calls.v1.schema.json"})
	copy.Tools = append([]ToolContract{}, manifest.Tools...)
	for _, tool := range manifest.Tools {
		if tool.Name == "lsp_trace_v1_validate" {
			envelopes := make([]string, 0, len(tool.EnvelopeSchemaIDs))
			for _, id := range tool.EnvelopeSchemaIDs {
				added := RetainedCallsEnvelopeID(id)
				filename := strings.TrimPrefix(added, "https://jaresty.github.io/lsp-trace/mcp/schemas/")
				copy.Schemas = append(copy.Schemas, SchemaRegistration{ID: added, Family: strings.TrimSuffix(filename, ".schema.json"), Layer: "envelope", Path: "schemas/" + filename})
				envelopes = append(envelopes, added)
			}
			copy.Tools = append(copy.Tools, ToolContract{Name: "lsp_trace_v1_export_retained_calls", Aliases: []string{"lsp_trace_export_retained_calls"}, InputSchemaID: RetainedCallsInputID, EnvelopeSchemaIDs: envelopes, ArtifactSchemaIDs: []string{RetainedCallsArtifactID}, Advertised: true, Availability: "ENABLED"})
		}
	}
	return &copy
}
