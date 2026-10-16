package mcpcontract

import (
	"encoding/json"
	"testing"
)

func callCompositionSchemaArtifact() map[string]any {
	return map[string]any{
		"schema_version": "lsp-trace.experimental.call-hierarchy-composition.v1", "experimental_version": "1",
		"status": "REFUSED", "phase": "PREPARATION", "code": "X", "incoming": []any{}, "outgoing": []any{},
		"incoming_null": false, "outgoing_null": false, "completeness": "UNKNOWN", "feature_identity": "UNRESOLVED",
		"authority": 0, "session_poisoned": false, "retry_scope": "NONE",
		"receipts": map[string]any{"incoming": map[string]any{"state": "UNAVAILABLE"}, "outgoing": map[string]any{"state": "UNAVAILABLE"}},
	}
}

func callCompositionPosition() map[string]any {
	return map[string]any{"line": 1, "character": 2}
}

func callCompositionRange() map[string]any {
	return map[string]any{"start": callCompositionPosition(), "end": callCompositionPosition()}
}

func callCompositionItem(data any) map[string]any {
	return map[string]any{
		"name": "callee", "kind": 12, "tags": []any{1}, "detail": "provider detail", "uri": "file:///workspace/main.go",
		"range": callCompositionRange(), "selectionRange": callCompositionRange(), "data": data,
	}
}

func assertCallCompositionRejected(t *testing.T, name string, mutate func(map[string]any)) {
	t.Helper()
	artifact := callCompositionSchemaArtifact()
	artifact["incoming"] = []any{map[string]any{"from": callCompositionItem(map[string]any{"provider": []any{1, true, nil}}), "fromRanges": []any{callCompositionRange()}}}
	artifact["outgoing"] = []any{map[string]any{"to": callCompositionItem(nil), "fromRanges": []any{callCompositionRange()}}}
	mutate(artifact)
	b, err := json.Marshal(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateJSON(CallHierarchyCompositionResultID, b); err == nil {
		t.Fatalf("artifact accepted nested injection %s", name)
	}
}

func TestCallHierarchyCompositionClosedEnvelopeAndReceiptUnion(t *testing.T) {
	artifact := callCompositionSchemaArtifact()
	raw, _ := json.Marshal(artifact)
	if err := ValidateJSON(CallHierarchyCompositionResultID, raw); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"raw", "prepared", "generation", "arbitrary"} {
		bad := callCompositionSchemaArtifact()
		bad[field] = true
		b, _ := json.Marshal(bad)
		if err := ValidateJSON(CallHierarchyCompositionResultID, b); err == nil {
			t.Fatalf("artifact accepted %s", field)
		}
	}
	envelope := map[string]any{"envelope_version": "1", "envelope_schema_id": CallHierarchyCompositionEnvelopeID, "tool": CallHierarchyCompositionTool, "request_id": "r", "outcome": "COMPLETE", "operation_status": "SUCCEEDED", "isError": false, "result": artifact}
	for _, field := range []string{"raw", "prepared", "generation", "arbitrary"} {
		bad := make(map[string]any, len(envelope)+1)
		for k, v := range envelope {
			bad[k] = v
		}
		bad[field] = true
		b, _ := json.Marshal(bad)
		if err := ValidateJSON(CallHierarchyCompositionEnvelopeID, b); err == nil {
			t.Fatalf("envelope accepted %s", field)
		}
	}
	badReceipt := callCompositionSchemaArtifact()
	badReceipt["receipts"].(map[string]any)["incoming"] = map[string]any{"state": "UNAVAILABLE", "request_sha256": "0000000000000000000000000000000000000000000000000000000000000000"}
	b, _ := json.Marshal(badReceipt)
	if err := ValidateJSON(CallHierarchyCompositionResultID, b); err == nil {
		t.Fatal("unavailable receipt accepted synthetic digest")
	}
}

func TestCallHierarchyCompositionClosedTypedRelationGraph(t *testing.T) {
	authentic := callCompositionSchemaArtifact()
	authentic["incoming"] = []any{map[string]any{"from": callCompositionItem(map[string]any{
		"provider": map[string]any{"nested": []any{1, "two", true, nil}},
		"raw":      []any{"provider-owned", 3},
	}), "fromRanges": []any{callCompositionRange()}}}
	authentic["outgoing"] = []any{map[string]any{"to": callCompositionItem(nil), "fromRanges": nil}}
	b, err := json.Marshal(authentic)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateJSON(CallHierarchyCompositionResultID, b); err != nil {
		t.Fatalf("authentic semantic artifact rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"incoming relation raw", func(a map[string]any) { a["incoming"].([]any)[0].(map[string]any)["raw"] = true }},
		{"outgoing relation prepared item", func(a map[string]any) {
			a["outgoing"].([]any)[0].(map[string]any)["prepared"] = map[string]any{"token": "private"}
		}},
		{"item generation", func(a map[string]any) {
			a["incoming"].([]any)[0].(map[string]any)["from"].(map[string]any)["generation"] = 7
		}},
		{"item frame bytes", func(a map[string]any) {
			a["outgoing"].([]any)[0].(map[string]any)["to"].(map[string]any)["frame_bytes"] = "private"
		}},
		{"item receipt wrapper", func(a map[string]any) {
			a["incoming"].([]any)[0].(map[string]any)["from"].(map[string]any)["receipt"] = map[string]any{"request_id": 9}
		}},
		{"range unknown", func(a map[string]any) {
			a["incoming"].([]any)[0].(map[string]any)["fromRanges"].([]any)[0].(map[string]any)["raw"] = true
		}},
		{"position unknown", func(a map[string]any) {
			a["outgoing"].([]any)[0].(map[string]any)["to"].(map[string]any)["range"].(map[string]any)["start"].(map[string]any)["generation"] = 1
		}},
		{"receipt unknown", func(a map[string]any) { a["receipts"].(map[string]any)["incoming"].(map[string]any)["raw"] = true }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { assertCallCompositionRejected(t, tt.name, tt.mutate) })
	}

	authenticated := callCompositionSchemaArtifact()
	authenticated["receipts"].(map[string]any)["incoming"] = map[string]any{
		"state": "AUTHENTICATED", "method": "callHierarchy/incomingCalls", "request_id": 1,
		"request_length": 1, "response_length": 1, "result_length": 0,
		"request_sha256": string(make([]byte, 64)), "response_sha256": string(make([]byte, 64)), "result_sha256": string(make([]byte, 64)),
		"observations": []any{map[string]any{"method": "callHierarchy/incomingCalls", "ordinal": 0, "length": 0, "sha256": string(make([]byte, 64)), "component": "element", "selected": true, "request_id": 1, "failure": ""}},
	}
	assertCallCompositionRejected(t, "observation unknown", func(a map[string]any) {
		a["receipts"] = authenticated["receipts"]
		a["receipts"].(map[string]any)["incoming"].(map[string]any)["observations"].([]any)[0].(map[string]any)["raw"] = true
	})
}
