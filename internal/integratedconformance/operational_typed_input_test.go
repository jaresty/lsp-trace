package integratedconformance

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	executionruntime "lsp-trace/internal/execution"
	"lsp-trace/internal/operation"
)

func TestOperationalLegacyMissingRoot(t *testing.T) {
	_, failure := executionruntime.NewProductionExecutor().Execute(context.Background(), operation.Request{Name: operation.CustodyExecute, RequestID: "offline-1", Input: []byte(`{"source":"legacy"}`)})
	if failure == nil || len(failure.Diagnostics) != 1 || failure.Diagnostics[0] != "root and source are required" {
		t.Fatal("ASSERT_OPERATIONAL_LEGACY_FAILURE: historical missing-root diagnostic changed")
	}
}

func TestOperationalTypedInput(t *testing.T) {
	request := operationalRequest(t)
	raw, _ := json.Marshal(request["operational"])
	var op executionruntime.OperationalInput
	if err := json.Unmarshal(raw, &op); err != nil {
		t.Fatal(err)
	}
	typed := executionruntime.ProductionInput{Root: request["root"].(string), Operational: &op}
	input, _ := json.Marshal(typed)
	if _, failure := executionruntime.NewProductionExecutor().Execute(context.Background(), operation.Request{Name: operation.CustodyExecute, RequestID: "offline-1", Input: input}); failure != nil {
		t.Fatalf("ASSERT_OPERATIONAL_TYPED_INPUT: public typed input rejected: %v", failure.Diagnostics)
	}
	legacy, _ := json.Marshal(executionruntime.ProductionInput{Root: "output", Source: "legacy"})
	if !bytes.Equal(legacy, []byte(`{"root":"output","source":"legacy"}`)) {
		t.Fatalf("ASSERT_OPERATIONAL_LEGACY_INPUT_BYTES: %s", legacy)
	}
}
