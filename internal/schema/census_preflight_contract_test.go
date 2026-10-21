package schema

import (
	"encoding/json"
	"testing"
)

func TestNewCensusSchemasRejectAdversarialMutations(t *testing.T) {
	cases := []struct {
		name, family string
		valid        map[string]any
		mutate       func(map[string]any)
	}{
		{"result_unknown_key", FamilyCensusEnumerationPreflight, resultFixture(), func(v map[string]any) { v["path"] = "/private/secret" }},
		{"result_missing_required", FamilyCensusEnumerationPreflight, resultFixture(), func(v map[string]any) { delete(v, "terminal") }},
		{"result_bad_status", FamilyCensusEnumerationPreflight, resultFixture(), func(v map[string]any) { v["terminal"] = "PARTIAL" }},
		{"result_bad_completeness", FamilyCensusEnumerationPreflight, resultFixture(), func(v map[string]any) { v["completeness"] = "COMPLETE" }},
		{"result_privacy_injection", FamilyCensusEnumerationPreflight, resultFixture(), func(v map[string]any) { v["roots"] = []any{"/private/secret"} }},
		{"diagnostic_unknown_key", FamilyCensusEnumerationDiagnostic, diagnosticFixture(), func(v map[string]any) { v["root"] = "/private/secret" }},
		{"diagnostic_missing_required", FamilyCensusEnumerationDiagnostic, diagnosticFixture(), func(v map[string]any) { delete(v, "limit") }},
		{"diagnostic_bad_category", FamilyCensusEnumerationDiagnostic, diagnosticFixture(), func(v map[string]any) { v["category"] = "PATH" }},
		{"diagnostic_bad_observed", FamilyCensusEnumerationDiagnostic, diagnosticFixture(), func(v map[string]any) { v["observed"] = -1 }},
		{"receipt_unknown_key", FamilyCensusRequestReceipt, receiptFixture(), func(v map[string]any) { v["path"] = "/private/secret" }},
		{"receipt_missing_required", FamilyCensusRequestReceipt, receiptFixture(), func(v map[string]any) { delete(v, "fingerprint") }},
		{"receipt_bad_prune_path", FamilyCensusRequestReceipt, receiptFixture(), func(v map[string]any) { v["semantic"].(map[string]any)["pruned_directories"] = []any{"../secret"} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.mutate(tc.valid)
			raw, _ := json.Marshal(tc.valid)
			if _, err := ValidateFor(raw, tc.family, "v1"); tc.family == FamilyCensusRequestReceipt {
				if _, err = ValidateFor(raw, tc.family, "v2"); err == nil {
					t.Fatal("ASSERT_ADVERSARIAL_SCHEMA_REJECTED")
				}
			} else if err == nil {
				t.Fatal("ASSERT_ADVERSARIAL_SCHEMA_REJECTED")
			}
		})
	}
}

func resultFixture() map[string]any {
	return map[string]any{"schema_version": "lsp-trace.census-enumeration-preflight-result.v1", "roots": []any{"."}, "includes": []any{}, "excludes": []any{}, "pruned_directories": []any{"vendor"}, "counts": map[string]any{"entries": 1, "selected": 1, "excluded": 0}, "selected_paths_digest": "sha256:0000000000000000000000000000000000000000000000000000000000000000", "limits": map[string]any{"max_entries": 100000, "max_accepted": 100000, "max_work": 200000, "max_path_bytes": 4096, "max_depth": 64}, "terminal": "COMPLETE", "authority": 0, "completeness": "UNKNOWN", "absence_inference": false}
}
func diagnosticFixture() map[string]any {
	return map[string]any{"schema_version": "lsp-trace.census-enumeration-diagnostic.v1", "category": "MAX_WORK", "observed": 200001, "limit": 200000}
}
func receiptFixture() map[string]any {
	return map[string]any{"schema_version": "lsp-trace.census-request-receipt.v2", "refresh_required": map[string]any{"session_id": "s", "generation": 1}, "semantic": map[string]any{"sources": []any{"."}, "includes": []any{}, "excludes": []any{}, "pruned_directories": []any{"vendor"}, "down_depth": 1, "up_depth": 0, "max_nodes": 10000, "batch_targets": 63, "timeout_ms": 60000, "request_timeout_ms": 30000}, "fingerprint": "sha256:0000000000000000000000000000000000000000000000000000000000000000"}
}
