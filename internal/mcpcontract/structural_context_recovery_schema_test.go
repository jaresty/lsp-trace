package mcpcontract

import (
	"encoding/json"
	"testing"
)

func structuralContextRecoveryEnvelope(recovery map[string]any) map[string]any {
	return map[string]any{
		"envelope_version":   "1",
		"envelope_schema_id": StructuralContextTraversalDomainErrorID,
		"tool":               StructuralContextV2Tool,
		"request_id":         "sc_0123456789abcdef0123456789abcdef",
		"outcome":            "DOMAIN_ERROR",
		"operation_status":   "FAILED",
		"isError":            true,
		"phase":              "PREFLIGHT",
		"state":              "TARGET_NOT_FOUND",
		"error":              map[string]any{"code": "TARGET_NOT_FOUND"},
		"target_diagnostic": map[string]any{
			"exact_matches":   0,
			"total_symbols":   0,
			"omitted_symbols": 0,
			"action":          "FAIL_ABSENT",
			"recovery":        recovery,
		},
	}
}

func validateStructuralContextRecoveryEnvelope(t *testing.T, recovery map[string]any, wantValid bool) {
	t.Helper()
	raw, err := json.Marshal(structuralContextRecoveryEnvelope(recovery))
	if err != nil {
		t.Fatal(err)
	}
	for name, validate := range map[string]func([]byte) error{
		"direct":  func(data []byte) error { return ValidateJSON(StructuralContextTraversalDomainErrorID, data) },
		"gateway": ValidateStructuralContextV2EnvelopeExclusive,
	} {
		err := validate(raw)
		if wantValid && err != nil {
			t.Fatalf("ASSERT_TARGET_RECOVERY_%s_VALID: %v\n%s", name, err, raw)
		}
		if !wantValid && err == nil {
			t.Fatalf("ASSERT_TARGET_RECOVERY_%s_INVALID accepted %s", name, raw)
		}
	}
}

func structuralContextRecoveryLimits() map[string]any {
	return map[string]any{
		"max_document_bytes": 61440,
		"max_matches":        100,
		"max_pattern_bytes":  4096,
		"max_work":           65536,
	}
}

func structuralContextSourceOnlyFragment() map[string]any {
	return map[string]any{
		"session_id": "s",
		"generation": 1,
		"uri":        "file:///workspace/a.go",
		"line":       2,
		"character":  3,
		"up_depth":   0,
		"down_depth": 0,
		"projection": map[string]any{
			"mode":                         "TARGET",
			"body":                         "INCLUDE",
			"include_relation_occurrences": false,
		},
	}
}

func TestStructuralContextTargetRecoveryEmittedKindsValidateDirectAndGateway(t *testing.T) {
	cases := map[string]map[string]any{
		"position": {
			"kind": "POSITION_LOCATOR", "uri": "file:///workspace/a.go", "line": 2, "character": 3,
		},
		"position_template": {
			"kind": "POSITION_LOCATOR_TEMPLATE", "required_fields": []any{"uri", "line", "character"},
		},
		"regex_template": {
			"kind": "REGEX_LOCATOR_TEMPLATE", "required_pattern": true, "match_index": 0,
			"required_fields": []any{"uri", "pattern", "match_index"}, "limits": structuralContextRecoveryLimits(),
		},
		"regex_template_with_uri": {
			"kind": "REGEX_LOCATOR_TEMPLATE", "uri": "file:///workspace/a.go", "required_pattern": true, "match_index": 0,
			"required_fields": []any{"uri", "pattern", "match_index"}, "limits": structuralContextRecoveryLimits(),
		},
		"unavailable": {
			"kind": "UNAVAILABLE", "unavailable_reason": "NO_INDEPENDENTLY_VALID_LOCATOR",
		},
		"source_only_position": {
			"kind": "SOURCE_ONLY_REQUEST_TEMPLATE", "complete": false,
			"request_fragment": structuralContextSourceOnlyFragment(),
			"omitted_fields":   []any{"projection.privacy_policy_id"},
		},
		"source_only_symbol": {
			"kind": "SOURCE_ONLY_REQUEST_TEMPLATE", "complete": false,
			"request_fragment": map[string]any{
				"session_id": "s", "generation": 1, "uri": "file:///workspace/a.go", "symbol": "Target",
				"up_depth": 0, "down_depth": 0, "projection": map[string]any{"mode": "TARGET", "body": "OMIT", "include_relation_occurrences": false},
			},
			"omitted_fields": []any{"projection.privacy_policy_id"},
		},
		"source_only_regex": {
			"kind": "SOURCE_ONLY_REQUEST_TEMPLATE", "complete": false,
			"request_fragment": map[string]any{
				"session_id": "s", "generation": 1,
				"regex_locator": map[string]any{"uri": "file:///workspace/a.go", "pattern": "Target", "match_index": 0, "capture_group": 0, "limits": structuralContextRecoveryLimits()},
				"up_depth":      0, "down_depth": 0, "projection": map[string]any{"mode": "TARGET", "body": "INCLUDE", "include_relation_occurrences": false},
			},
			"omitted_fields": []any{"projection.privacy_policy_id"},
		},
	}
	for name, recovery := range cases {
		t.Run(name, func(t *testing.T) { validateStructuralContextRecoveryEnvelope(t, recovery, true) })
	}
}

func TestStructuralContextTargetRecoveryRejectsMixedAndSurplusFields(t *testing.T) {
	cases := map[string]map[string]any{
		"unknown_kind":                   {"kind": "OTHER"},
		"position_missing_character":     {"kind": "POSITION_LOCATOR", "uri": "file:///workspace/a.go", "line": 2},
		"position_mixed_template":        {"kind": "POSITION_LOCATOR", "uri": "file:///workspace/a.go", "line": 2, "character": 3, "required_fields": []any{"uri", "line", "character"}},
		"position_template_concrete":     {"kind": "POSITION_LOCATOR_TEMPLATE", "required_fields": []any{"uri", "line", "character"}, "uri": "file:///workspace/a.go"},
		"position_template_wrong_fields": {"kind": "POSITION_LOCATOR_TEMPLATE", "required_fields": []any{"uri", "pattern", "match_index"}},
		"regex_template_missing_limits":  {"kind": "REGEX_LOCATOR_TEMPLATE", "required_pattern": true, "match_index": 0, "required_fields": []any{"uri", "pattern", "match_index"}},
		"regex_template_position_field":  {"kind": "REGEX_LOCATOR_TEMPLATE", "required_pattern": true, "match_index": 0, "required_fields": []any{"uri", "pattern", "match_index"}, "limits": structuralContextRecoveryLimits(), "line": 2},
		"unavailable_locator":            {"kind": "UNAVAILABLE", "unavailable_reason": "NO_INDEPENDENTLY_VALID_LOCATOR", "uri": "file:///workspace/a.go"},
		"source_only_complete":           {"kind": "SOURCE_ONLY_REQUEST_TEMPLATE", "complete": true, "request_fragment": structuralContextSourceOnlyFragment(), "omitted_fields": []any{"projection.privacy_policy_id"}},
		"source_only_mixed_locator": {"kind": "SOURCE_ONLY_REQUEST_TEMPLATE", "complete": false, "request_fragment": map[string]any{
			"session_id": "s", "generation": 1, "uri": "file:///workspace/a.go", "symbol": "Target", "line": 2, "character": 3,
			"up_depth": 0, "down_depth": 0, "projection": map[string]any{"mode": "TARGET", "body": "INCLUDE", "include_relation_occurrences": false},
		}, "omitted_fields": []any{"projection.privacy_policy_id"}},
		"surplus": {"kind": "UNAVAILABLE", "unavailable_reason": "NO_INDEPENDENTLY_VALID_LOCATOR", "extra": true},
	}
	for name, recovery := range cases {
		t.Run(name, func(t *testing.T) { validateStructuralContextRecoveryEnvelope(t, recovery, false) })
	}
}
