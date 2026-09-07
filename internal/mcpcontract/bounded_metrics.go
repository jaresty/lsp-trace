package mcpcontract

import "strings"

const BoundedMetricsInputID = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-bounded-retained-metrics.v1.schema.json"
const BoundedMetricsArtifactID = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.bounded-retained-metrics.v1.schema.json"

func BoundedMetricsEnvelopeID(id string) string {
	return strings.Replace(id, "/envelope-", "/envelope-bounded-metrics-", 1)
}

// WithBoundedMetrics is additive; neither Stage 1 nor retained-call contracts
// are rebound. A new analytical family has its own closed envelope variants.
func WithBoundedMetrics(manifest *Manifest) *Manifest {
	copy := *manifest
	copy.Schemas = append(append([]SchemaRegistration{}, manifest.Schemas...), SchemaRegistration{ID: BoundedMetricsInputID, Family: "input-bounded-retained-metrics.v1", Layer: "input", Path: "schemas/input-bounded-retained-metrics.v1.schema.json"}, SchemaRegistration{ID: BoundedMetricsArtifactID, Family: "lsp-trace.bounded-retained-metrics.v1", Layer: "artifact", Path: "../../schema/schemas/lsp-trace.bounded-retained-metrics.v1.schema.json"})
	envelopes := []string{}
	for _, kind := range []string{"artifact", "publication", "compact-publication", "domain-error", "publication-error"} {
		filename := "envelope-bounded-metrics-" + kind + ".v1.schema.json"
		id := "https://jaresty.github.io/lsp-trace/mcp/schemas/" + filename
		copy.Schemas = append(copy.Schemas, SchemaRegistration{ID: id, Family: strings.TrimSuffix(filename, ".schema.json"), Layer: "envelope", Path: "schemas/" + filename})
		envelopes = append(envelopes, id)
	}
	copy.Tools = append(append([]ToolContract{}, manifest.Tools...), ToolContract{Name: "lsp_trace_v1_bounded_retained_metrics", Aliases: []string{"lsp_trace_bounded_retained_metrics"}, InputSchemaID: BoundedMetricsInputID, EnvelopeSchemaIDs: envelopes, ArtifactSchemaIDs: []string{BoundedMetricsArtifactID}, Advertised: true, Availability: "ENABLED"})
	return &copy
}
