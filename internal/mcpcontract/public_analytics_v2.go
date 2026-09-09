package mcpcontract

const (
	PublicAnalyticsV2InputID                    = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-bounded-retained-analytics.v2.schema.json"
	PublicAnalyticsV2GraphArtifactID            = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.normative-retained-graph.v1.schema.json"
	PublicAnalyticsV2AnalysisArtifactID         = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.local-normative-analysis.v1.schema.json"
	PublicAnalyticsV2MetricsArtifactID          = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.local-normative-metrics.v1.schema.json"
	PublicAnalyticsV2RankingArtifactID          = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.local-normative-ranking.v1.schema.json"
	PublicAnalyticsV2ArtifactEnvelopeID         = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-public-analytics-v2-artifact.v1.schema.json"
	PublicAnalyticsV2PublicationEnvelopeID      = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-public-analytics-v2-publication.v1.schema.json"
	PublicAnalyticsV2CompactEnvelopeID          = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-public-analytics-v2-compact-publication.v1.schema.json"
	PublicAnalyticsV2PublicationErrorEnvelopeID = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-public-analytics-v2-publication-error.v1.schema.json"
	PublicAnalyticsV2DomainEnvelopeID           = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-public-analytics-v2-domain-error.v1.schema.json"
)

func PublicAnalyticsV2EnvelopeID(id string) string {
	switch id {
	case "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-artifact.v1.schema.json":
		return PublicAnalyticsV2ArtifactEnvelopeID
	case "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-publication.v1.schema.json":
		return PublicAnalyticsV2PublicationEnvelopeID
	case "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-compact-publication.v1.schema.json":
		return PublicAnalyticsV2CompactEnvelopeID
	case "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-publication-error.v1.schema.json":
		return PublicAnalyticsV2PublicationErrorEnvelopeID
	case "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-domain-error.v1.schema.json":
		return PublicAnalyticsV2DomainEnvelopeID
	default:
		return id
	}
}

// WithPublicAnalyticsV2 is the outer layer-28 composition. It only appends the
// three local, bounded, synthetic analytics routes; every prior contract remains
// byte-for-byte owned by its existing layer.
func WithPublicAnalyticsV2(manifest *Manifest) *Manifest {
	copy := *manifest
	copy.Schemas = append([]SchemaRegistration{}, manifest.Schemas...)
	for _, s := range []SchemaRegistration{
		{ID: PublicAnalyticsV2InputID, Family: "input-bounded-retained-analytics.v2", Layer: "input", Path: "schemas/input-bounded-retained-analytics.v2.schema.json"},
		{ID: PublicAnalyticsV2GraphArtifactID, Family: "lsp-trace.bounded-retained-graph-v2.v2", Layer: "artifact", Path: "../../schema/schemas/lsp-trace.normative-retained-graph.v1.schema.json"},
		{ID: PublicAnalyticsV2AnalysisArtifactID, Family: "lsp-trace.bounded-retained-analysis-v2.v2", Layer: "artifact", Path: "../../schema/schemas/lsp-trace.local-normative-analysis.v1.schema.json"},
		{ID: PublicAnalyticsV2MetricsArtifactID, Family: "lsp-trace.bounded-retained-metrics-v2.v2", Layer: "artifact", Path: "../../schema/schemas/lsp-trace.local-normative-metrics.v1.schema.json"},
		{ID: PublicAnalyticsV2RankingArtifactID, Family: "lsp-trace.bounded-retained-ranking-v2.v2", Layer: "artifact", Path: "../../schema/schemas/lsp-trace.local-normative-ranking.v1.schema.json"},
		{ID: PublicAnalyticsV2ArtifactEnvelopeID, Family: "envelope-public-analytics-v2-artifact.v1", Layer: "envelope", Path: "schemas/envelope-public-analytics-v2-artifact.v1.schema.json"},
		{ID: PublicAnalyticsV2PublicationEnvelopeID, Family: "envelope-public-analytics-v2-publication.v1", Layer: "envelope", Path: "schemas/envelope-public-analytics-v2-publication.v1.schema.json"},
		{ID: PublicAnalyticsV2CompactEnvelopeID, Family: "envelope-public-analytics-v2-compact-publication.v1", Layer: "envelope", Path: "schemas/envelope-public-analytics-v2-compact-publication.v1.schema.json"},
		{ID: PublicAnalyticsV2PublicationErrorEnvelopeID, Family: "envelope-public-analytics-v2-publication-error.v1", Layer: "envelope", Path: "schemas/envelope-public-analytics-v2-publication-error.v1.schema.json"},
		{ID: PublicAnalyticsV2DomainEnvelopeID, Family: "envelope-public-analytics-v2-domain-error.v1", Layer: "envelope", Path: "schemas/envelope-public-analytics-v2-domain-error.v1.schema.json"},
	} {
		copy.Schemas = append(copy.Schemas, s)
	}
	copy.Tools = append([]ToolContract{}, manifest.Tools...)
	for _, x := range []struct{ name, artifact string }{
		{"lsp_trace_v2_bounded_retained_analysis", PublicAnalyticsV2AnalysisArtifactID},
		{"lsp_trace_v2_bounded_retained_metrics", PublicAnalyticsV2MetricsArtifactID},
		{"lsp_trace_v2_bounded_retained_ranking", PublicAnalyticsV2RankingArtifactID},
	} {
		copy.Tools = append(copy.Tools, ToolContract{Name: x.name, Aliases: []string{}, InputSchemaID: PublicAnalyticsV2InputID, EnvelopeSchemaIDs: []string{PublicAnalyticsV2ArtifactEnvelopeID, PublicAnalyticsV2PublicationEnvelopeID, PublicAnalyticsV2CompactEnvelopeID, PublicAnalyticsV2PublicationErrorEnvelopeID, PublicAnalyticsV2DomainEnvelopeID}, ArtifactSchemaIDs: []string{x.artifact}, Advertised: true, Availability: "ENABLED"})
	}
	return &copy
}
