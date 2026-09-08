package mcpcontract

import (
	"bytes"
	"encoding/json"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/schema"
	"reflect"
	"testing"
)

func TestMetricsAdditiveContractAndClosedInputs(t *testing.T) {
	historical, err := LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(historical)
	runtime := WithRetainedCalls(historical)
	after, _ := json.Marshal(historical)
	if !bytes.Equal(before, after) || len(historical.Tools) != 13 || len(runtime.Tools) != 20 {
		t.Fatalf("ASSERT_RETAINED_CALLS_HISTORICAL_20_IMMUTABLE: historical=%d retained=%d", len(historical.Tools), len(runtime.Tools))
	}
	for i, tool := range historical.Tools {
		if !reflect.DeepEqual(tool, runtime.Tools[i]) {
			t.Fatal("ASSERT_METRICS_HISTORICAL_TOOL_UNCHANGED", tool.Name)
		}
	}
	oldAnalysis := WithBoundedAnalysis(historical).Tools[len(historical.Tools)]
	for _, tool := range runtime.Tools {
		if tool.Name == oldAnalysis.Name && !reflect.DeepEqual(tool, oldAnalysis) {
			t.Fatal("ASSERT_METRICS_OLD_ANALYSIS_CONTRACT_UNCHANGED")
		}
	}
	for _, valid := range []string{`{"input":{}}`, `{"input":"{}"}`, `{"input":"{}","detail":"compact","output_selector":"a.json"}`} {
		if err := ValidateJSON(BoundedMetricsInputID, []byte(valid)); err != nil {
			t.Fatal(err)
		}
	}
	for _, invalid := range []string{`{}`, `{"Input":"{}"}`, `{"input":"{}","operation":"METRICS"}`, `{"input":"{}","max_work":1}`, `{"input":"{}","parameters":{}}`, `{"input":null}`, `{"input":"{}","output_selector":""}`, `{"input":"{}","detail":"other"}`} {
		if err := ValidateJSON(BoundedMetricsInputID, []byte(invalid)); err == nil {
			t.Fatal("ASSERT_METRICS_CLOSED_INPUT", invalid)
		}
	}
	if err := ValidateJSON(BoundedAnalysisInputID, []byte(`{"input":"{}","operation":"METRICS"}`)); err == nil {
		t.Fatal("ASSERT_OLD_ANALYSIS_ENUM_UNCHANGED")
	}
	if _, err := schema.BytesFor("bounded-retained-metrics", "v1"); err != nil {
		t.Fatal(err)
	}
	v, err := NewOperationInputValidator()
	if err != nil {
		t.Fatal(err)
	}
	if err = v.ValidateOperationInput(operation.BoundedRetainedMetrics, json.RawMessage(`{"input":{},"input":{}}`)); err == nil {
		t.Fatal("ASSERT_METRICS_OPERATION_DUPLICATE_REJECT")
	}
}
