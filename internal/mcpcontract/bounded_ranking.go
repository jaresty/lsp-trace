package mcpcontract

import "strings"

const BoundedRankingInputID = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-bounded-retained-ranking.v1.schema.json"
const BoundedRankingArtifactID = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.bounded-retained-ranking.v1.schema.json"

func BoundedRankingEnvelopeID(id string) string {
	return strings.Replace(id, "/envelope-", "/envelope-bounded-ranking-", 1)
}
func WithBoundedRanking(manifest *Manifest) *Manifest {
	copy := *manifest
	copy.Schemas = append(append([]SchemaRegistration{}, manifest.Schemas...), SchemaRegistration{ID: BoundedRankingInputID, Family: "input-bounded-retained-ranking.v1", Layer: "input", Path: "schemas/input-bounded-retained-ranking.v1.schema.json"}, SchemaRegistration{ID: BoundedRankingArtifactID, Family: "lsp-trace.bounded-retained-ranking.v1", Layer: "artifact", Path: "../../schema/schemas/lsp-trace.bounded-retained-ranking.v1.schema.json"})
	envelopes := []string{}
	for _, kind := range []string{"artifact", "publication", "compact-publication", "domain-error", "publication-error"} {
		filename := "envelope-bounded-ranking-" + kind + ".v1.schema.json"
		id := "https://jaresty.github.io/lsp-trace/mcp/schemas/" + filename
		copy.Schemas = append(copy.Schemas, SchemaRegistration{ID: id, Family: strings.TrimSuffix(filename, ".schema.json"), Layer: "envelope", Path: "schemas/" + filename})
		envelopes = append(envelopes, id)
	}
	copy.Tools = append(append([]ToolContract{}, manifest.Tools...), ToolContract{Name: "lsp_trace_v1_bounded_retained_ranking", Aliases: []string{"lsp_trace_bounded_retained_ranking"}, InputSchemaID: BoundedRankingInputID, EnvelopeSchemaIDs: envelopes, ArtifactSchemaIDs: []string{BoundedRankingArtifactID}, Advertised: true, Availability: "ENABLED"})
	return &copy
}
