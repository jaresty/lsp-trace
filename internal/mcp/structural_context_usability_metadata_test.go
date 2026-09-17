package mcp

import (
	"strings"
	"testing"

	"lsp-trace/internal/mcpcontract"
)

func TestStructuralContextV2SourceInspectionAndAmbiguityGuidance(t *testing.T) {
	tool, ok := NewRegistry(false).ResolveCanonical(mcpcontract.StructuralContextV2Tool)
	if !ok {
		t.Fatal("ASSERT_STRUCTURAL_CONTEXT_USABILITY_METADATA: missing tool")
	}
	description := completeToolDescription(tool)
	for _, required := range []string{
		"exact-position", "projection mode TARGET", "up_depth=0", "down_depth=0",
		"Expand call relationships separately", "Do not automatically retry traversal failures",
		"AMBIGUOUS_TARGET", "resolve the declaration externally", "zero-based line and character",
		"do not expose candidate, path, or source dumps", "Authority remains zero", "source_graph_complete remains UNKNOWN",
	} {
		if !strings.Contains(description, required) {
			t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_USABILITY_METADATA[%s]: %s", required, description)
		}
	}
	if len(description) > 40<<10 {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_METADATA_COMPACT: %d", len(description))
	}
}
