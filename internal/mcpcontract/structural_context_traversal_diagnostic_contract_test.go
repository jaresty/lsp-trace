package mcpcontract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func structuralContextV4Envelope() map[string]any {
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
	}
}

func validateStructuralContextV4(t *testing.T, value map[string]any, wantValid bool) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	err = ValidateJSON(StructuralContextTraversalDomainErrorID, raw)
	if wantValid && err != nil {
		t.Fatalf("valid V4 envelope rejected: %v\n%s", err, raw)
	}
	if !wantValid && err == nil {
		t.Fatalf("invalid V4 envelope accepted: %s", raw)
	}
}

func cloneStructuralContextV4(t *testing.T, value map[string]any) map[string]any {
	t.Helper()
	raw, _ := json.Marshal(value)
	var cloned map[string]any
	if err := json.Unmarshal(raw, &cloned); err != nil {
		t.Fatal(err)
	}
	return cloned
}

func TestStructuralContextTraversalDomainErrorV4TargetDiagnosticIsClosedAndPrivate(t *testing.T) {
	validateStructuralContextV4(t, structuralContextV4Envelope(), true)
	withDiagnostic := structuralContextV4Envelope()
	withDiagnostic["state"], withDiagnostic["error"] = "AMBIGUOUS_TARGET", map[string]any{"code": "AMBIGUOUS_TARGET"}
	withDiagnostic["target_diagnostic"] = map[string]any{"exact_matches": 2, "total_symbols": 9, "omitted_symbols": 1, "action": "FAIL_AMBIGUOUS"}
	validateStructuralContextV4(t, withDiagnostic, true)

	for _, field := range []string{"symbol", "uri", "path", "source", "line", "character", "selector", "raw_error", "provider", "environment", "privacy_policy_id"} {
		bad := cloneStructuralContextV4(t, withDiagnostic)
		bad["target_diagnostic"].(map[string]any)[field] = "private"
		validateStructuralContextV4(t, bad, false)
	}
	rawTarget := structuralContextV4Envelope()
	rawTarget["target"] = map[string]any{"kind": "SYMBOL", "symbol": "private"}
	validateStructuralContextV4(t, rawTarget, false)
	mixed := cloneStructuralContextV4(t, withDiagnostic)
	mixed["phase"], mixed["state"], mixed["error"] = "TRAVERSAL", "INVALID_SERVER_RESPONSE", map[string]any{"code": "INVALID_SERVER_RESPONSE"}
	mixed["diagnostic"] = map[string]any{"stage": "PREPARE", "method": "textDocument/prepareCallHierarchy"}
	validateStructuralContextV4(t, mixed, false)
}

func TestStructuralContextTraversalDomainErrorV4ResourceDiagnosticIsClosedAndExact(t *testing.T) {
	value := structuralContextV4Envelope()
	value["state"], value["error"] = "RESOURCE_LIMIT", map[string]any{"code": "RESOURCE_LIMIT"}
	value["resource_diagnostic"] = map[string]any{"reason": "REGEX_MAX_DOCUMENT_BYTES", "field": "regex_locator.limits.max_document_bytes", "allowed": 100, "observed": 125, "maximum_allowed": 16777216, "suggested_limit": 125}
	validateStructuralContextV4(t, value, true)
	for name, mutate := range map[string]func(map[string]any){
		"unknown_reason": func(v map[string]any) { v["resource_diagnostic"].(map[string]any)["reason"] = "OTHER" },
		"unknown_field": func(v map[string]any) { v["resource_diagnostic"].(map[string]any)["field"] = "max_nodes" },
		"unknown_property": func(v map[string]any) { v["resource_diagnostic"].(map[string]any)["path"] = "private" },
		"wrong_state": func(v map[string]any) { v["state"], v["error"] = "TIMEOUT", map[string]any{"code": "TIMEOUT"} },
	} {
		t.Run(name, func(t *testing.T) {
			bad := cloneStructuralContextV4(t, value)
			mutate(bad)
			validateStructuralContextV4(t, bad, false)
		})
	}
}

func TestStructuralContextTraversalDomainErrorV4ReasonIsBoundedAndPrivate(t *testing.T) {
	withoutReason := structuralContextV4Envelope()
	validateStructuralContextV4(t, withoutReason, true)

	withReason := structuralContextV4Envelope()
	withReason["reason"] = "NO_REGEX_MATCH"
	validateStructuralContextV4(t, withReason, true)

	for _, reason := range []string{"ARBITRARY", "no_regex_match", "SOURCE_UNAVAILABLE ", "file:///private"} {
		bad := cloneStructuralContextV4(t, withReason)
		bad["reason"] = reason
		validateStructuralContextV4(t, bad, false)
	}
	for _, field := range []string{"raw_error", "uri", "path", "source", "selector", "provider"} {
		bad := cloneStructuralContextV4(t, withReason)
		bad[field] = "private"
		validateStructuralContextV4(t, bad, false)
	}
}

func TestStructuralContextTraversalDomainErrorV4DiagnosticIsClosedAndExact(t *testing.T) {
	cases := []map[string]any{
		{"stage": "PREPARE", "method": "textDocument/prepareCallHierarchy"},
		{"stage": "OUTGOING", "method": "callHierarchy/outgoingCalls", "direction": "OUTGOING"},
		{"stage": "INCOMING", "method": "callHierarchy/incomingCalls", "direction": "INCOMING", "depth": 0},
	}
	for _, diagnostic := range cases {
		value := structuralContextV4Envelope()
		value["phase"], value["state"], value["error"] = "TRAVERSAL", "INVALID_SERVER_RESPONSE", map[string]any{"code": "INVALID_SERVER_RESPONSE"}
		value["diagnostic"] = diagnostic
		validateStructuralContextV4(t, value, true)
	}

	base := structuralContextV4Envelope()
	base["phase"], base["state"], base["error"] = "TRAVERSAL", "INVALID_SERVER_RESPONSE", map[string]any{"code": "INVALID_SERVER_RESPONSE"}
	base["diagnostic"] = map[string]any{"stage": "OUTGOING", "method": "callHierarchy/outgoingCalls", "direction": "OUTGOING"}
	for _, field := range []string{"message", "uri", "path", "source", "selector", "environment", "provider"} {
		bad := cloneStructuralContextV4(t, base)
		bad["diagnostic"].(map[string]any)[field] = "private"
		validateStructuralContextV4(t, bad, false)
	}
	for name, mutate := range map[string]func(map[string]any){
		"prepare_direction": func(v map[string]any) {
			v["diagnostic"] = map[string]any{"stage": "PREPARE", "method": "textDocument/prepareCallHierarchy", "direction": "OUTGOING"}
		},
		"method_mismatch": func(v map[string]any) { v["diagnostic"].(map[string]any)["method"] = "callHierarchy/incomingCalls" },
		"negative_depth":  func(v map[string]any) { v["diagnostic"].(map[string]any)["depth"] = -1 },
		"locator_diagnostic": func(v map[string]any) {
			v["phase"], v["state"], v["error"] = "PREFLIGHT", "TARGET_NOT_FOUND", map[string]any{"code": "TARGET_NOT_FOUND"}
		},
	} {
		t.Run(name, func(t *testing.T) {
			bad := cloneStructuralContextV4(t, base)
			mutate(bad)
			validateStructuralContextV4(t, bad, false)
		})
	}
}

func TestStructuralContextDomainErrorV3BytesImmutable(t *testing.T) {
	raw, err := os.ReadFile("testdata/schemas/envelope-structural-context-domain-error.v3.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	if got := hex.EncodeToString(sum[:]); got != "7f97e5ecccad9671b1aff83110aaa4609937235658bc539734bbc35579551735" {
		t.Fatalf("released V3 bytes changed: %s", got)
	}
}
