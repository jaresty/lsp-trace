package mcpcontract

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStructuralContextV2TargetEnvelopeValidationReportsFirstInvariant(t *testing.T) {
	result := map[string]any{
		"schema_version": "lsp-trace.unified-structural-context-result.v2",
		"structural": map[string]any{
			"schema_version":         "lsp-trace.transient-structural-result.v2",
			"authority":              0,
			"source_graph_complete":  "UNKNOWN",
			"position_encoding":      "utf-16",
			"target_node_id":         "tn_0123456789abcdef0123456789abcdef",
			"nodes":                  []any{},
			"calls":                  []any{},
			"analytics_scope":        "BOUNDED_LOCAL",
			"coupling":               []any{},
			"external_nodes_omitted": 0,
			"external_calls_omitted": 0,
		},
		"projection": map[string]any{
			"schema_version":         "lsp-trace.source-projection.v2",
			"authority":              0,
			"source_graph_complete":  "UNKNOWN",
			"graph_facts_added":      0,
			"custody_mode":           "LIVE",
			"custody_binding":        map[string]any{"custody": "LIVE", "session_id": "s", "generation": 1, "position_encoding": "utf-16"},
			"physical_projection_id": "sha256:" + strings.Repeat("a", 64),
			"request_policy_id":      "sha256:" + strings.Repeat("b", 64),
			"status":                 "COMPLETE",
			"document_selection":     map[string]any{"ordering": "TARGET_FIRST_THEN_URI_LEXICOGRAPHIC", "target_uri": "file:///workspace/a.go", "selected_uris": []any{"file:///workspace/a.go"}},
			"document_bindings":      []any{},
			"document_accounting":    map[string]any{"candidates": 1, "selected": 1, "acquired": 1, "unavailable": 0, "withheld": 0, "limit_omitted": 0, "total_acquired_bytes": 0},
			"units":                  []any{},
			"citations":              []any{},
			"emitted_spans":          []any{},
			"accounting":             map[string]any{"candidates": 1, "selected": 0, "omitted": 1, "logical_selected_bytes": 0, "unique_emitted_bytes": 0, "evaluated": 1, "terminal": 1, "unevaluated": 0},
			"omissions":              []any{},
			"privacy_summary":        map[string]any{"policy_id": "public", "body_requested": true, "body_returned": 0, "body_withheld": 0},
		},
	}
	envelope := map[string]any{
		"envelope_version":   "1",
		"envelope_schema_id": StructuralContextProjectionSuccessID,
		"tool":               StructuralContextV2Tool,
		"request_id":         "offline-1",
		"outcome":            "COMPLETE",
		"operation_status":   "SUCCEEDED",
		"isError":            false,
		"result":             result,
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	err = ValidateStructuralContextV2EnvelopeExclusive(raw)
	if err == nil || !strings.Contains(err.Error(), "request_id") || !strings.Contains(err.Error(), "pattern") {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_TARGET_EMITTED_ENVELOPE_FIRST_INVARIANT: %v", err)
	}
}
