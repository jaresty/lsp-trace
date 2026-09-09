package mcpcontract

const (
	PublicAnalyticsV2InputID            = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-bounded-retained-analytics.v2.schema.json"
	PublicAnalyticsV2GraphArtifactID    = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.normative-retained-graph.v1.schema.json"
	PublicAnalyticsV2AnalysisArtifactID = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.local-normative-analysis.v1.schema.json"
	PublicAnalyticsV2MetricsArtifactID  = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.local-normative-metrics.v1.schema.json"
	PublicAnalyticsV2RankingArtifactID  = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.local-normative-ranking.v1.schema.json"
)

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
	} {
		copy.Schemas = append(copy.Schemas, s)
	}
	copy.Tools = append([]ToolContract{}, manifest.Tools...)
	for _, x := range []struct{ name, artifact string }{
		{"lsp_trace_v2_bounded_retained_analysis", PublicAnalyticsV2AnalysisArtifactID},
		{"lsp_trace_v2_bounded_retained_metrics", PublicAnalyticsV2MetricsArtifactID},
		{"lsp_trace_v2_bounded_retained_ranking", PublicAnalyticsV2RankingArtifactID},
	} {
		copy.Tools = append(copy.Tools, ToolContract{Name: x.name, Aliases: []string{}, InputSchemaID: PublicAnalyticsV2InputID, EnvelopeSchemaIDs: []string{}, ArtifactSchemaIDs: []string{x.artifact}, Advertised: true, Availability: "ENABLED"})
	}
	return &copy
}
