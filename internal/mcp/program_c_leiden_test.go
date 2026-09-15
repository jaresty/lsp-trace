package mcp

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/programctestfixture"
)

func leidenServer(t testing.TB, profile ToolProfile) *Server {
	t.Helper()
	validator, err := mcpcontract.NewOperationInputValidator()
	if err != nil {
		t.Fatal(err)
	}
	return &Server{
		Registry: NewRegistryWithProfile(false, profile),
		Executor: operation.NewOffline(validator, map[operation.Name]operation.Handler{
			operation.ProgramCLeiden: operation.ProgramCLeidenHandler,
		}),
	}
}

func leidenArguments(t testing.TB) map[string]any {
	t.Helper()
	return map[string]any{"input": string(programctestfixture.ValidV5(t)), "seed": float64(19), "pagerank_top_k": float64(2), "hub_top_k": float64(2)}
}

func TestProgramCLeidenRegistrySchemasProfilesAndRequiredTopK(t *testing.T) {
	full := NewRegistryWithProfile(false, ToolProfileFull)
	compact := NewRegistryWithProfile(false, ToolProfileCompact)
	tool, ok := full.Resolve(mcpcontract.ProgramCLeidenTool)
	if !ok || len(full.Tools()) != 40 || len(full.Advertised()) != 40 || len(compact.Tools()) != 40 || len(compact.Advertised()) != 10 {
		t.Fatalf("ASSERT_PROGRAM_C_LEIDEN_32_FULL_10_COMPACT: ok=%t full=%d/%d compact=%d/%d", ok, len(full.Tools()), len(full.Advertised()), len(compact.Tools()), len(compact.Advertised()))
	}
	if _, ok := compact.Resolve(mcpcontract.ProgramCLeidenTool); !ok {
		t.Fatal("ASSERT_PROGRAM_C_LEIDEN_COMPACT_HIDDEN_DISPATCHABLE")
	}
	for _, id := range []string{mcpcontract.ProgramCLeidenInputID, mcpcontract.ProgramCLeidenArtifactID, mcpcontract.ProgramCLeidenArtifactEnvelopeID, mcpcontract.ProgramCLeidenDomainEnvelopeID} {
		if !strings.Contains(strings.Join(append(append([]string{tool.InputSchemaID}, tool.ArtifactSchemaIDs...), tool.EnvelopeSchemaIDs...), "\n"), id) {
			t.Fatalf("ASSERT_PROGRAM_C_LEIDEN_SCHEMA_REGISTERED: %s tool=%+v", id, tool)
		}
	}
	required, _ := tool.InputSchema["required"].([]any)
	if !reflect.DeepEqual(required, []any{"input", "seed", "pagerank_top_k", "hub_top_k"}) {
		t.Fatalf("ASSERT_PROGRAM_C_LEIDEN_TOP_K_REQUIRED_NO_DEFAULT: %v", required)
	}
	args := leidenArguments(t)
	delete(args, "pagerank_top_k")
	if response := directMatrixCall(leidenServer(t, ToolProfileFull), mcpcontract.ProgramCLeidenTool, args); response.Error == nil {
		t.Fatal("ASSERT_PROGRAM_C_LEIDEN_OMITTED_TOP_K_REJECTED")
	}
}

func TestProgramCLeidenDirectExecuteExactEnvelopeAndArtifactParity(t *testing.T) {
	args := leidenArguments(t)
	direct := directMatrixCall(leidenServer(t, ToolProfileFull), mcpcontract.ProgramCLeidenTool, args)
	if direct.Error != nil {
		t.Fatal(direct.Error)
	}
	directEnvelope, err := json.Marshal(direct.Result.(callResult).StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	wrapped := runServerMessages(t, leidenServer(t, ToolProfileCompact), callMessage("lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": mcpcontract.ProgramCLeidenTool, "arguments": args}}))[0]
	envelope := decodeEnvelopeForAssertion(t, "ASSERT_PROGRAM_C_LEIDEN_DIRECT_EXECUTE_PARITY", wrapped)
	if envelope["delegated_envelope"] != string(directEnvelope) {
		t.Fatalf("ASSERT_PROGRAM_C_LEIDEN_DIRECT_EXECUTE_PARITY: direct=%s execute=%v", directEnvelope, envelope["delegated_envelope"])
	}
	if strings.Contains(string(directEnvelope), programctestfixture.OpaqueMarker) || strings.Contains(string(directEnvelope), programctestfixture.SourceBodyMarker) {
		t.Fatal("ASSERT_PROGRAM_C_LEIDEN_NO_SOURCE_BODY_OR_OPAQUE_LEAK")
	}
	for _, want := range []string{"lsp-trace.community-presentation.v1", "Structural communities do not establish", "Cross-seed stability is unassessed"} {
		if !strings.Contains(string(directEnvelope), want) {
			t.Fatalf("ASSERT_PROGRAM_C_LEIDEN_CAVEAT_OR_SCHEMA_MISSING: %s envelope=%s", want, directEnvelope)
		}
	}
}
