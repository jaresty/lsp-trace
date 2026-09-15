package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/programctestfixture"
)

const programCInstabilityTool = "lsp_trace_v1_program_c_instability"

func TestProgramCInstabilityCanonicalHiddenCompactSurface(t *testing.T) {
	full := NewRegistryWithProfile(false, ToolProfileFull)
	compact := NewRegistryWithProfile(false, ToolProfileCompact)
	tool, ok := full.Resolve(programCInstabilityTool)
	if !ok || tool.Name != programCInstabilityTool {
		t.Fatalf("ASSERT_A08_CANONICAL_MCP_REGISTERED: ok=%t tool=%+v", ok, tool)
	}
	if _, ok := compact.Resolve(programCInstabilityTool); !ok || len(compact.Advertised()) != 12 {
		t.Fatalf("ASSERT_A08_HIDDEN_COMPACT_DISPATCHABLE_EXACT_11: resolved=%t advertised=%d", ok, len(compact.Advertised()))
	}
}

func TestProgramCInstabilityDirectExecution(t *testing.T) {
	validator, err := mcpcontract.NewOperationInputValidator()
	if err != nil {
		t.Fatal(err)
	}
	newServer := func() *Server {
		return &Server{Registry: NewRegistryWithProfile(false, ToolProfileCompact), Executor: operation.NewOffline(validator, map[operation.Name]operation.Handler{operation.ProgramCInstability: operation.ProgramCInstabilityHandler})}
	}
	server := newServer()
	args := map[string]any{"input": string(programctestfixture.ValidV5(t)), "seeds": []any{float64(1), float64(2)}, "algorithm_version": "gonum-v0.17.1", "parameters_sha256": "sha256:" + strings.Repeat("c", 64), "resource_policy_sha256": "sha256:" + strings.Repeat("d", 64)}
	response := directMatrixCall(server, programCInstabilityTool, args)
	if response.Error != nil {
		t.Fatalf("ASSERT_A08_DIRECT_EXECUTION: %v", response.Error)
	}
	directEnvelope, err := json.Marshal(response.Result.(callResult).StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	wrapped := runServerMessages(t, newServer(), callMessage("lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": programCInstabilityTool, "arguments": args}}))[0]
	envelope := decodeEnvelopeForAssertion(t, "ASSERT_A08_CANONICAL_EXECUTE_PARITY", wrapped)
	if envelope["delegated_envelope"] != string(directEnvelope) {
		t.Fatalf("ASSERT_A08_CANONICAL_EXECUTE_PARITY: direct=%s execute=%v", directEnvelope, envelope["delegated_envelope"])
	}
}
