package execution

import (
	"strings"
	"testing"
)

const mcpExecuteRemedy = `invoke MCP tool lsp_trace_v1_execute with { "request": { "operation": ..., "arguments": ... } }`

func TestProductionInputRejectsTopLevelMCPOperationWithRoutingDiagnostic(t *testing.T) {
	var input ProductionInput
	err := decodeProductionInput([]byte(`{"operation":"lsp_trace_v2_structural_context","arguments":{}}`), &input)
	if err == nil || !strings.Contains(err.Error(), mcpExecuteRemedy) {
		t.Fatalf("ASSERT_CLI_EXECUTE_TOP_LEVEL_OPERATION_MISROUTE: err=%v", err)
	}
}

func TestProductionInputRejectsWrappedMCPOperationWithRoutingDiagnostic(t *testing.T) {
	var input ProductionInput
	err := decodeProductionInput([]byte(`{"request":{"operation":"lsp_trace_v2_structural_context","arguments":{}}}`), &input)
	if err == nil || !strings.Contains(err.Error(), mcpExecuteRemedy) {
		t.Fatalf("ASSERT_CLI_EXECUTE_REQUEST_OPERATION_MISROUTE: err=%v", err)
	}
}

func TestProductionInputOrdinaryMalformedRequestRetainsStrictDiagnostic(t *testing.T) {
	var input ProductionInput
	err := decodeProductionInput([]byte(`{"root":"/tmp/out","source":"x","unexpected":true}`), &input)
	const want = `invalid production execution request: json: unknown field "unexpected"`
	if err == nil || err.Error() != want {
		t.Fatalf("ASSERT_CLI_EXECUTE_ORDINARY_MALFORMED_STRICT_COMPATIBILITY: got=%v want=%q", err, want)
	}
}
