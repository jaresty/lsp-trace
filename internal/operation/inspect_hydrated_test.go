package operation_test

import (
	"context"
	"encoding/json"
	"testing"

	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
)

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
