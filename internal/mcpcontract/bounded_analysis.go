package mcpcontract

import "strings"

const BoundedAnalysisInputID = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-bounded-retained-analysis.v1.schema.json"
const BoundedAnalysisArtifactID = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.bounded-retained-analysis.v1.schema.json"

func BoundedAnalysisEnvelopeID(id string) string {
	return strings.Replace(id, "/envelope-", "/envelope-bounded-analysis-", 1)
}

// WithBoundedAnalysis is additive; neither Stage 1 nor retained-call contracts
// are rebound. A new analytical family has its own closed envelope variants.
func WithBoundedAnalysis(manifest *Manifest) *Manifest {
	copy := *manifest
	copy.Schemas = append(append([]SchemaRegistration{}, manifest.Schemas...), SchemaRegistration{ID: BoundedAnalysisInputID, Family: "input-bounded-retained-analysis.v1", Layer: "input", Path: "schemas/input-bounded-retained-analysis.v1.schema.json"}, SchemaRegistration{ID: BoundedAnalysisArtifactID, Family: "lsp-trace.bounded-retained-analysis.v1", Layer: "artifact", Path: "../../schema/schemas/lsp-trace.bounded-retained-analysis.v1.schema.json"})
	envelopes := []string{}
	for _, kind := range []string{"artifact", "publication", "compact-publication", "domain-error", "publication-error"} {
		filename := "envelope-bounded-analysis-" + kind + ".v1.schema.json"
		id := "https://jaresty.github.io/lsp-trace/mcp/schemas/" + filename
		copy.Schemas = append(copy.Schemas, SchemaRegistration{ID: id, Family: strings.TrimSuffix(filename, ".schema.json"), Layer: "envelope", Path: "schemas/" + filename})
		envelopes = append(envelopes, id)
	}
	copy.Tools = append(append([]ToolContract{}, manifest.Tools...), ToolContract{Name: "lsp_trace_v1_bounded_retained_analysis", Aliases: []string{"lsp_trace_bounded_retained_analysis"}, InputSchemaID: BoundedAnalysisInputID, EnvelopeSchemaIDs: envelopes, ArtifactSchemaIDs: []string{BoundedAnalysisArtifactID}, Advertised: true, Availability: "ENABLED"})
	return &copy
}
