package mcpcontract

import "strings"

// Additive registration; the historical thirteen-tool Stage 1 manifest stays
// byte-identical. The shared runtime composes this separately versioned input.
const RetainedCallsInputID = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-export-retained-calls.v1.schema.json"
const RetainedCallsV2InputID = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-export-retained-calls.v2.schema.json"
const VerifyRetainedCallsV2InputID = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-verify-retained-calls.v2.schema.json"
const RetainedCallsArtifactID = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.retained-calls.v1.schema.json"
const RetainedCallsV2ArtifactID = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.retained-calls.v2.schema.json"
const RetainedRelationsInputID = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-export-retained-relations.v1.schema.json"
const RetainedRelationsArtifactID = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.retained-relations.v1.schema.json"

func RetainedCallsEnvelopeID(id string) string {
	return strings.Replace(id, "/envelope-", "/envelope-retained-calls-", 1)
}

func RetainedCallsV2EnvelopeID(id string) string {
	return strings.Replace(id, "/envelope-", "/envelope-retained-calls-v2-", 1)
}

func VerifyRetainedCallsV2EnvelopeID(id string) string {
	return strings.Replace(id, "/envelope-", "/envelope-verify-retained-calls-v2-", 1)
}

func WithRetainedCalls(manifest *Manifest) *Manifest {
	copy := *manifest
	copy.Schemas = append(append([]SchemaRegistration{}, manifest.Schemas...),
		SchemaRegistration{ID: RetainedCallsInputID, Family: "input-export-retained-calls.v1", Layer: "input", Path: "schemas/input-export-retained-calls.v1.schema.json"},
		SchemaRegistration{ID: RetainedCallsArtifactID, Family: "lsp-trace.retained-calls.v1", Layer: "artifact", Path: "../../schema/schemas/lsp-trace.retained-calls.v1.schema.json"})
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
	return WithAcquisitionV2(WithBoundedRanking(WithBoundedMetrics(WithBoundedAnalysis(&copy))))
}

// WithRetainedRelations extends the historical V1 export operation in place.
// It preserves tool cardinality and all V1 inputs while making explicit V3
// retained-relations dispatch structurally reachable.
func WithRetainedRelations(manifest *Manifest) *Manifest {
	copy := *manifest
	copy.Schemas = append([]SchemaRegistration{}, manifest.Schemas...)
	copy.Tools = append([]ToolContract{}, manifest.Tools...)
	copy.Schemas = append(copy.Schemas,
		SchemaRegistration{ID: RetainedRelationsInputID, Family: "input-export-retained-relations.v1", Layer: "input", Path: "schemas/input-export-retained-relations.v1.schema.json"},
		SchemaRegistration{ID: RetainedRelationsArtifactID, Family: "lsp-trace.retained-relations.v1", Layer: "artifact", Path: "../../schema/schemas/lsp-trace.retained-relations.v1.schema.json"},
	)
	for i := range copy.Tools {
		if copy.Tools[i].Name == "lsp_trace_v1_export_retained_calls" {
			copy.Tools[i].InputSchemaID = RetainedRelationsInputID
			copy.Tools[i].ArtifactSchemaIDs = append(copy.Tools[i].ArtifactSchemaIDs, RetainedRelationsArtifactID)
			break
		}
	}
	return &copy
}

// WithRetainedCallsV2Export adds the current retained-calls v2 exporter without
// changing the preserved historical twenty-tool WithRetainedCalls composition.
func WithRetainedCallsV2Export(manifest *Manifest) *Manifest {
	copy := *manifest
	copy.Schemas = append([]SchemaRegistration{}, manifest.Schemas...)
	copy.Tools = append([]ToolContract{}, manifest.Tools...)
	for _, tool := range manifest.Tools {
		if tool.Name != "lsp_trace_v1_validate" {
			continue
		}
		envelopes := make([]string, 0, len(tool.EnvelopeSchemaIDs))
		for _, id := range tool.EnvelopeSchemaIDs {
			added := RetainedCallsV2EnvelopeID(id)
			filename := strings.TrimPrefix(added, "https://jaresty.github.io/lsp-trace/mcp/schemas/")
			copy.Schemas = append(copy.Schemas, SchemaRegistration{ID: added, Family: strings.TrimSuffix(filename, ".schema.json"), Layer: "envelope", Path: "schemas/" + filename})
			envelopes = append(envelopes, added)
		}
		copy.Schemas = append(copy.Schemas,
			SchemaRegistration{ID: RetainedCallsV2InputID, Family: "input-export-retained-calls.v2", Layer: "input", Path: "schemas/input-export-retained-calls.v2.schema.json"},
			SchemaRegistration{ID: RetainedCallsV2ArtifactID, Family: "lsp-trace.retained-calls.v2", Layer: "artifact", Path: "../../schema/schemas/lsp-trace.retained-calls.v2.schema.json"})
		copy.Tools = append(copy.Tools, ToolContract{Name: "lsp_trace_v2_export_retained_calls", Aliases: []string{}, InputSchemaID: RetainedCallsV2InputID, EnvelopeSchemaIDs: envelopes, ArtifactSchemaIDs: []string{RetainedCallsV2ArtifactID}, Advertised: true, Availability: "ENABLED"})
		break
	}
	return &copy
}

// WithRetainedCallsV2Verifier adds the current retained-calls verifier without
// changing the preserved historical twenty-tool WithRetainedCalls composition.
func WithRetainedCallsV2Verifier(manifest *Manifest) *Manifest {
	copy := *manifest
	copy.Schemas = append([]SchemaRegistration{}, manifest.Schemas...)
	copy.Tools = append([]ToolContract{}, manifest.Tools...)
	for _, tool := range manifest.Tools {
		if tool.Name != "lsp_trace_v1_validate" {
			continue
		}
		envelopes := make([]string, 0, len(tool.EnvelopeSchemaIDs))
		for _, id := range tool.EnvelopeSchemaIDs {
			added := VerifyRetainedCallsV2EnvelopeID(id)
			filename := strings.TrimPrefix(added, "https://jaresty.github.io/lsp-trace/mcp/schemas/")
			copy.Schemas = append(copy.Schemas, SchemaRegistration{ID: added, Family: strings.TrimSuffix(filename, ".schema.json"), Layer: "envelope", Path: "schemas/" + filename})
			envelopes = append(envelopes, added)
		}
		copy.Schemas = append(copy.Schemas, SchemaRegistration{ID: VerifyRetainedCallsV2InputID, Family: "input-verify-retained-calls.v2", Layer: "input", Path: "schemas/input-verify-retained-calls.v2.schema.json"})
		copy.Tools = append(copy.Tools, ToolContract{Name: "lsp_trace_v2_verify_retained_calls", Aliases: []string{}, InputSchemaID: VerifyRetainedCallsV2InputID, EnvelopeSchemaIDs: envelopes, ArtifactSchemaIDs: []string{RetainedCallsV2ArtifactID}, Advertised: true, Availability: "ENABLED"})
		break
	}
	return &copy
}
