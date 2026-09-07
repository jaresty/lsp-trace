package mcp

import "testing"

func TestRetainedCallsExportRegistration(t *testing.T) {
	r := NewRegistryWithPublication(false, true)
	tool, ok := r.Resolve("lsp_trace_v1_export_retained_calls")
	if !ok || tool.Availability != Enabled || tool.ExecutorFamily != OfflineExecutorFamily {
		t.Fatal("ASSERT_EXPLICIT_OFFLINE_RETAINED_CALLS_EXPORT: enabled shared offline export is missing")
	}
	if len(tool.ArtifactSchemaIDs) != 1 || tool.ArtifactSchemaIDs[0] != "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.retained-calls.v1.schema.json" {
		t.Fatal("ASSERT_RETAINED_CALLS_DISTINCT_FAMILY")
	}
}
