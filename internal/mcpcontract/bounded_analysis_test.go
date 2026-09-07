package mcpcontract

import (
	"lsp-trace/internal/schema"
	"testing"
)

func TestBoundedRetainedAnalysisPublicContract(t *testing.T) {
	t.Run("artifact-schema", func(t *testing.T) {
		if _, err := schema.BytesFor("bounded-retained-analysis", "v1"); err != nil {
			t.Fatalf("ASSERT_BOUNDED_ANALYSIS_ADDITIVE_ARTIFACT_SCHEMA: %v", err)
		}
	})
	t.Run("offline-input", func(t *testing.T) {
		const id = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-bounded-retained-analysis.v1.schema.json"
		if _, err := SchemaJSON(id); err != nil {
			t.Fatalf("ASSERT_BOUNDED_ANALYSIS_ADDITIVE_OFFLINE_INPUT: %v", err)
		}
	})
}
