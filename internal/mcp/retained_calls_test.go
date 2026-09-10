package mcp

import (
	"reflect"
	"testing"

	"lsp-trace/internal/mcpcontract"
)

func TestRetainedCallsExportRegistration(t *testing.T) {
	r := NewRegistryWithPublication(false, true)
	tool, ok := r.Resolve("lsp_trace_v1_export_retained_calls")
	if !ok || tool.Availability != Enabled || tool.ExecutorFamily != OfflineExecutorFamily {
		t.Fatal("ASSERT_EXPLICIT_OFFLINE_RETAINED_CALLS_EXPORT: enabled shared offline export is missing")
	}
	want := []string{mcpcontract.RetainedCallsArtifactID, mcpcontract.RetainedRelationsArtifactID}
	if !reflect.DeepEqual(tool.ArtifactSchemaIDs, want) {
		t.Fatalf("ASSERT_RETAINED_CALLS_DISTINCT_FAMILY: got=%v want=%v", tool.ArtifactSchemaIDs, want)
	}
}
