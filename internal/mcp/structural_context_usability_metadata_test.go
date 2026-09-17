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
		"exact symbol", "position", "document regex", "READY session",
		"Locators add no facts", "CALLS are server-reported", "Bounded context",
		"authority 0", "completeness unknown",
	} {
		if !strings.Contains(description, required) {
			t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_USABILITY_METADATA[%s]: %s", required, description)
		}
	}
	if len(description) > 40<<10 {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_METADATA_COMPACT: %d", len(description))
	}
}
