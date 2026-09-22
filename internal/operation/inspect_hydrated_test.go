package operation_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
)

func TestInspectHydratedUnsupportedEvidenceDiagnosticIsActionableAndPrivate(t *testing.T) {
	input := json.RawMessage(`{"input":"{}","node_ids":["missing"]}`)
	_, failure := operation.InspectHydratedHandler(context.Background(), operation.Request{Input: input})
	if failure == nil || failure.Code != "INPUT_INVALID" {
		t.Fatalf("ASSERT_INSPECT_HYDRATED_UNSUPPORTED_EVIDENCE_CODE: %+v", failure)
	}
	text := failure.Error()
	for _, required := range []string{"input.schema_version", "SUPPORTED_EVIDENCE_FAMILY_VERSION", "observed=missing", "lsp_trace_v1_schema_get", "correct input.schema_version and resubmit", `{"schema_version":"lsp-trace.graph-provenance.v1"}`} {
		if !strings.Contains(text, required) {
			t.Fatalf("ASSERT_INSPECT_HYDRATED_ACTIONABLE_%s: %s", strings.ToUpper(strings.ReplaceAll(required, " ", "_")), text)
		}
	}
	for _, forbidden := range []string{"source_body", "source_bytes", "file:///", "/Users/"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("ASSERT_INSPECT_HYDRATED_PRIVATE_%s: %s", strings.ToUpper(forbidden), text)
		}
	}

	hostile := json.RawMessage(`{"input":"{\"schema_version\":\"file:///Users/private/secret.go\"}","node_ids":["missing"]}`)
	_, failure = operation.InspectHydratedHandler(context.Background(), operation.Request{Input: hostile})
	if failure == nil || !strings.Contains(failure.Error(), "observed=unrecognized") || strings.Contains(failure.Error(), "private/secret") {
		t.Fatalf("ASSERT_INSPECT_HYDRATED_UNRECOGNIZED_VERSION_PRIVATE: %+v", failure)
	}
}

func TestHydratedOperationPreflight(t *testing.T) {
	validator, err := mcpcontract.NewOperationInputValidator()
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	executor := operation.NewOffline(validator, map[operation.Name]operation.Handler{operation.InspectHydrated: func(context.Context, operation.Request) (operation.Result, *operation.Failure) {
		calls++
		return operation.Result{}, nil
	}})
	for _, input := range []string{`{"input":"not parsed","include_bodies":"true"}`, `{"input":"not parsed","core_policy":{"max_work":1.5}}`, `{"input":"not parsed","output_selector":"unsafe"}`, `{"input":"not parsed","node_ids":null}`} {
		_, failure := executor.Execute(context.Background(), operation.Request{Name: operation.InspectHydrated, Input: json.RawMessage(input)})
		if failure == nil || failure.Code != operation.FailureInvalidInput || calls != 0 {
			t.Fatalf("PUBLIC_OPERATION_PREFLIGHT FAIL: %v calls=%d", failure, calls)
		}
		_, failure = operation.InspectHydratedHandler(context.Background(), operation.Request{Input: json.RawMessage(input)})
		if failure == nil || failure.Code != operation.FailureInvalidInput {
			t.Fatal("PUBLIC_OPERATION_PREFLIGHT FAIL: direct CLI handler")
		}
	}
	t.Log("PUBLIC_OPERATION_PREFLIGHT PASS: structural failure before shared semantic dispatch")
}
