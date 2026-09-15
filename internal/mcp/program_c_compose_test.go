package mcp

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/programccompose"
	"lsp-trace/internal/programctestfixture"
)

func composeArguments(t testing.TB) map[string]any {
	t.Helper()
	raw := programctestfixture.ValidV5(t)
	digest := sha256.Sum256(raw)
	capture := func(identity string) map[string]any {
		return map[string]any{
			"input": string(raw), "identity": identity,
			"sha256": fmt.Sprintf("sha256:%x", digest), "byte_length": len(raw),
			"exact_metadata": map[string]any{
				"workspace_identity": "workspace-1", "revision_custody": "CALLER_ASSERTED",
				"position_encoding": "utf-16", "acquisition_semantics": "managed-lsp-v1",
				"privacy_policy": "bundle-custodian-v1",
			},
		}
	}
	return map[string]any{"captures": []any{capture("capture-b"), capture("capture-a")}}
}

func composeServer(t testing.TB, profile ToolProfile) *Server {
	t.Helper()
	validator, err := mcpcontract.NewOperationInputValidator()
	if err != nil {
		t.Fatal(err)
	}
	return &Server{Registry: NewRegistryWithProfile(false, profile), Executor: operation.NewOffline(validator, map[operation.Name]operation.Handler{operation.ProgramCCompose: operation.ProgramCComposeHandler})}
}

func TestProgramCComposeOperation32RegistrySchemasAndCompactProfile(t *testing.T) {
	full, compact := NewRegistryWithProfile(false, ToolProfileFull), NewRegistryWithProfile(false, ToolProfileCompact)
	tool, ok := full.Resolve(mcpcontract.ProgramCComposeTool)
	if !ok || len(full.Tools()) != 42 || len(full.Advertised()) != 42 || len(compact.Tools()) != 42 || len(compact.Advertised()) != 12 {
		t.Fatalf("ASSERT_PROGRAM_C_COMPOSE_32_FULL_11_COMPACT: ok=%t full=%d/%d compact=%d/%d", ok, len(full.Tools()), len(full.Advertised()), len(compact.Tools()), len(compact.Advertised()))
	}
	if _, ok := compact.Resolve(mcpcontract.ProgramCComposeTool); !ok {
		t.Fatal("ASSERT_PROGRAM_C_COMPOSE_COMPACT_HIDDEN_DISPATCHABLE")
	}
	got := append(append([]string{tool.InputSchemaID}, tool.ArtifactSchemaIDs...), tool.EnvelopeSchemaIDs...)
	want := []string{mcpcontract.ProgramCComposeInputID, mcpcontract.ProgramCComposeArtifactID, mcpcontract.ProgramCComposeArtifactEnvelopeID, mcpcontract.ProgramCComposeDomainEnvelopeID}
	for _, id := range want {
		if !strings.Contains(strings.Join(got, "\n"), id) {
			t.Fatalf("ASSERT_PROGRAM_C_COMPOSE_SCHEMA_REGISTERED: %s", id)
		}
	}
	required, _ := tool.InputSchema["required"].([]any)
	if !reflect.DeepEqual(required, []any{"captures"}) {
		t.Fatalf("ASSERT_PROGRAM_C_COMPOSE_CAPTURE_INPUT_REQUIRED: %v", required)
	}
}

func TestProgramCComposeDirectExecuteDeterminismSchemasAndLeidenClosed(t *testing.T) {
	args := composeArguments(t)
	direct := directMatrixCall(composeServer(t, ToolProfileFull), mcpcontract.ProgramCComposeTool, args)
	if direct.Error != nil {
		t.Fatal(direct.Error)
	}
	directEnvelope, _ := json.Marshal(direct.Result.(callResult).StructuredContent)
	wrapped := runServerMessages(t, composeServer(t, ToolProfileCompact), callMessage("lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": mcpcontract.ProgramCComposeTool, "arguments": args}}))[0]
	envelope := decodeEnvelopeForAssertion(t, "ASSERT_PROGRAM_C_COMPOSE_DIRECT_EXECUTE_PARITY", wrapped)
	if envelope["delegated_envelope"] != string(directEnvelope) {
		t.Fatalf("ASSERT_PROGRAM_C_COMPOSE_DIRECT_EXECUTE_PARITY: direct=%s execute=%v", directEnvelope, envelope["delegated_envelope"])
	}
	var directObject map[string]any
	if err := json.Unmarshal(directEnvelope, &directObject); err != nil {
		t.Fatal(err)
	}
	artifact := directObject["content"].(string)
	if _, err := programccompose.Validate([]byte(artifact)); err != nil {
		t.Fatal("ASSERT_PROGRAM_C_COMPOSE_REQUESTED_ARTIFACT_SCHEMA_AND_REPLAY", err)
	}
	leiden := directMatrixCall(leidenServer(t, ToolProfileFull), mcpcontract.ProgramCLeidenTool, map[string]any{"input": artifact, "seed": float64(1), "pagerank_top_k": float64(1), "hub_top_k": float64(1)})
	if leiden.Error != nil || !leiden.Result.(callResult).IsError {
		t.Fatalf("ASSERT_PROGRAM_C_COMPOSITE_LEIDEN_FAILS_CLOSED: response=%+v", leiden)
	}
	second := directMatrixCall(composeServer(t, ToolProfileFull), mcpcontract.ProgramCComposeTool, args)
	secondEnvelope, _ := json.Marshal(second.Result.(callResult).StructuredContent)
	if string(directEnvelope) != string(secondEnvelope) {
		t.Fatal("ASSERT_PROGRAM_C_COMPOSE_DETERMINISTIC_ENVELOPE")
	}
}
