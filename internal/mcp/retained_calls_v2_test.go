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

func TestRetainedCallsV2VerifyRegistrationAndCurrentCount(t *testing.T) {
	const registered = "ASSERT_MCP_RETAINED_CALLS_V2_VERIFY_REGISTERED"
	const cardinality = "ASSERT_MCP_CURRENT_CANONICAL_TOOL_COUNT_28"
	r := NewRegistryWithPublication(false, true)
	tool, ok := r.Resolve("lsp_trace_v2_verify_retained_calls")
	if !ok {
		t.Fatal(registered + ": canonical tool absent")
	}
	if tool.Availability != Enabled || tool.ExecutorFamily != OfflineExecutorFamily || len(tool.Aliases) != 0 {
		t.Fatalf("%s: tool=%+v", registered, tool)
	}
	if got := len(r.Advertised()); got != 28 {
		t.Fatalf("%s: got %d", cardinality, got)
	}
	if tool.InputSchemaID != mcpcontract.VerifyRetainedCallsV2InputID || len(tool.ArtifactSchemaIDs) != 1 || tool.ArtifactSchemaIDs[0] != mcpcontract.RetainedCallsV2ArtifactID {
		t.Fatalf("%s: contracts=%+v", registered, tool)
	}
}

func TestRetainedCallsV2VerifyInputContractIsClosedAndRequiresSelector(t *testing.T) {
	const assertion = "ASSERT_MCP_RETAINED_CALLS_V2_VERIFY_CLOSED_INPUT"
	valid := []byte(`{"input":"retained-v2.json","schema":{"family":"retained-calls","version":"v2"}}`)
	if err := mcpcontract.ValidateJSON(mcpcontract.VerifyRetainedCallsV2InputID, valid); err != nil {
		t.Fatalf("%s: valid contract rejected: %v", assertion, err)
	}
	for _, request := range [][]byte{
		[]byte(`{"schema":{"family":"retained-calls","version":"v2"}}`),
		[]byte(`{"input":"retained-v2.json","schema":{"family":"graph-provenance","version":"v2"}}`),
		[]byte(`{"input":"retained-v2.json","schema":{"family":"retained-calls","version":"v2"},"target":"server-path"}`),
	} {
		if err := mcpcontract.ValidateJSON(mcpcontract.VerifyRetainedCallsV2InputID, request); err == nil {
			t.Fatalf("%s: admitted %s", assertion, request)
		}
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
