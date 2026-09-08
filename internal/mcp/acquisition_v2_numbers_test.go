package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAcquisitionV2ExactNumbers(t *testing.T) {
	raw := json.RawMessage(`{"name":"lsp_trace_v2_slice","arguments":{"session_id":"s","generation":9007199254740993,"seed_manifest":{}}}`)
	var p callParams
	selected, err := decodeBoundedParams(raw, &p)
	if !selected || err != nil {
		t.Fatalf("ASSERT_V2_NUMBER_PRESERVING_ROUTE: selected=%v err=%v", selected, err)
	}
	encoded, _ := json.Marshal(p.Arguments)
	if !strings.Contains(string(encoded), `"generation":9007199254740993`) {
		t.Fatalf("ASSERT_V2_EXACT_GENERATION: %s", encoded)
	}
	raw = json.RawMessage(`{"name":"lsp_trace_v1_validate","arguments":{"schema":{"family":"graph-provenance","version":"v2"},"input":{"schema_version":"lsp-trace.graph-provenance.v2","opaque":9007199254740993}}}`)
	selected, err = decodeBoundedParams(raw, &p)
	if !selected || err != nil {
		t.Fatalf("ASSERT_V2_OBJECT_ADMISSION_EXACT: selected=%v err=%v", selected, err)
	}
	text, ok := p.Arguments["input"].(string)
	if !ok || !strings.Contains(text, "9007199254740993") {
		t.Fatal("ASSERT_V2_OBJECT_BYTES")
	}
}
