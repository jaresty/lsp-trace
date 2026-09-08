package mcp

import (
	"encoding/json"
	"testing"

	"lsp-trace/internal/mcpcontract"
)

func TestRetainedCallsV2ExportRegistration(t *testing.T) {
	const assertion = "ASSERT_MCP_RETAINED_CALLS_V2_REGISTERED"
	r := NewRegistryWithPublication(false, true)
	tool, ok := r.Resolve("lsp_trace_v2_export_retained_calls")
	if !ok {
		t.Fatal(assertion + ": canonical tool absent")
	}
	if tool.Availability != Enabled || tool.ExecutorFamily != OfflineExecutorFamily {
		t.Fatalf("%s: tool=%+v", assertion, tool)
	}
	if len(tool.Aliases) != 0 {
		t.Fatalf("%s: V2 canonical tool unexpectedly has aliases: %v", assertion, tool.Aliases)
	}
	if len(tool.ArtifactSchemaIDs) != 1 || tool.ArtifactSchemaIDs[0] != "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.retained-calls.v2.schema.json" {
		t.Fatalf("%s: artifact schemas=%v", assertion, tool.ArtifactSchemaIDs)
	}
}

func TestRetainedCallsV2InputContractIsClosedStringOnly(t *testing.T) {
	const assertion = "ASSERT_MCP_RETAINED_CALLS_V2_CLOSED_STRING_INPUT"
	for _, request := range []map[string]any{
		{"input": map[string]any{}},
		{"input": "{}", "version": "v2"},
		{"input": "{}", "path": "server-side.json"},
	} {
		raw, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		if err = mcpcontract.ValidateJSON(mcpcontract.RetainedCallsV2InputID, raw); err == nil {
			t.Fatalf("%s: admitted %s", assertion, raw)
		}
	}
	if err := mcpcontract.ValidateJSON(mcpcontract.RetainedCallsV2InputID, []byte(`{"input":"{}","output_selector":"out.json","detail":"compact"}`)); err != nil {
		t.Fatalf("%s: valid contract rejected: %v", assertion, err)
	}
}

func TestRetainedCallsV1InputContractRejectsVersionWidening(t *testing.T) {
	const assertion = "ASSERT_MCP_RETAINED_CALLS_V1_REJECTS_VERSION_FIELD"
	request, err := json.Marshal(map[string]any{"input": "{}", "version": "v2"})
	if err != nil {
		t.Fatal(err)
	}
	if err = mcpcontract.ValidateJSON(mcpcontract.RetainedCallsInputID, request); err == nil {
		t.Fatal(assertion + ": old tool silently admitted a version field")
	}
}
