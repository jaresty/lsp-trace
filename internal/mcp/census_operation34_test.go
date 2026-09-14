package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
)

type censusProjectedExecutor struct {
	outcome      string
	publications int
	calls        []operation.Request
}

func (e *censusProjectedExecutor) Execute(_ context.Context, request operation.Request) (operation.Result, *operation.Failure) {
	e.calls = append(e.calls, request)
	e.publications++
	result := `{"schema_version":"lsp-trace.census-result.v1","status":"SUCCEEDED","census_id":"c","capture_set_id":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","session_id":"s","generation":1,"target_count":1,"batch_count":1,"file_accounting":{"denominator":1,"processed":1,"excluded":0,"forbidden":0,"unreadable":0,"unsupported":0,"document_symbol_failed":0,"omitted":0,"incomplete":0},"symbol_accounting":{"denominator":1,"prepared":1,"unsupported":0,"preparation_failed":0,"prepare_missing":0,"non_callable":0,"omitted":0,"incomplete":0},"authority":0,"source_graph_complete":"UNKNOWN","native_aggregate_custody":false,"cross_capture_calls":[],"leiden_admissible":false,"publication":{"selector":"census.json","digest":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","byte_length":1,"verification_status":"VERIFIED","directory_sync_status":"COMPLETE","close_status":"COMPLETE"}}`
	if e.outcome == "COMMITTED_DEGRADED" {
		result = `{"schema_version":"lsp-trace.census-diagnostic.v1","status":"SUCCEEDED_DEGRADED","stage":"committed-degradation","code":"COMMITTED_DEGRADED","retry":false}`
	}
	raw := []byte(`{"envelope_version":"1","envelope_schema_id":"` + mcpcontract.CensusSuccessID + `","tool":"` + mcpcontract.CensusTool + `","request_id":"` + request.RequestID + `","outcome":"` + e.outcome + `","operation_status":"SUCCEEDED","isError":false,"result":` + result + `}`)
	return operation.Result{Artifact: raw}, nil
}

func TestCensusOperation34DirectGatewayTerminalIDsAndSinglePublication(t *testing.T) {
	for _, outcome := range []string{"COMPLETE", "COMMITTED_DEGRADED"} {
		t.Run(outcome, func(t *testing.T) {
			executor := &censusProjectedExecutor{outcome: outcome}
			server := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull), Executors: map[ExecutorFamily]Executor{CensusExecutorFamily: executor}}
			args := map[string]any{"session_id": "s", "generation": float64(1), "sources": []any{"."}}
			direct := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.CensusTool, args))
			gateway := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(2)}, mustCallParams(t, "lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": mcpcontract.CensusTool, "arguments": args}}))
			if direct.Error != nil || gateway.Error != nil || executor.publications != 2 || len(executor.calls) != 2 {
				t.Fatalf("ASSERT_CENSUS_%s_DIRECT_GATEWAY_EXACTLY_ONE_PUBLICATION_EACH: direct=%v gateway=%v publications=%d calls=%d", outcome, direct.Error, gateway.Error, executor.publications, len(executor.calls))
			}
			directResult := direct.Result.(callResult)
			gatewayResult := gateway.Result.(callResult)
			if directResult.StructuredContent.RequestID != "offline-1" || executor.calls[0].RequestID != "offline-1" || executor.calls[1].RequestID != "offline-2" || gatewayResult.StructuredContent.RequestID != "offline-3" {
				t.Fatalf("ASSERT_CENSUS_DISTINCT_DIRECT_DELEGATED_EXECUTE_IDS: direct=%s calls=%s/%s outer=%s", directResult.StructuredContent.RequestID, executor.calls[0].RequestID, executor.calls[1].RequestID, gatewayResult.StructuredContent.RequestID)
			}
			delegated := gatewayResult.StructuredContent.DelegatedEnvelope
			if !strings.Contains(delegated, `"request_id":"offline-2"`) || strings.Contains(delegated, `"request_id":"offline-3"`) {
				t.Fatalf("ASSERT_CENSUS_DELEGATED_ID_PRESERVED: %s", delegated)
			}
			if err := mcpcontract.ValidateEnvelopeExclusive([]byte(delegated)); err != nil {
				t.Fatalf("ASSERT_CENSUS_DELEGATED_SCHEMA_EXCLUSIVE: %v", err)
			}
		})
	}
}

func TestEnabledToolRequiresOperationSpecificDescriptionBeforeSuffixComposition(t *testing.T) {
	valid := Tool{Name: mcpcontract.CensusTool, Availability: Enabled, Description: "accountable census"}
	requireOperationSpecificDescription(valid)

	defer func() {
		if recovered := recover(); recovered == nil || !strings.Contains(recovered.(string), "has no operation-specific description") {
			t.Fatalf("ASSERT_ENABLED_TOOL_OPERATION_SPECIFIC_DESCRIPTION_REQUIRED: panic=%v", recovered)
		}
	}()
	mutated := valid
	mutated.Description = "  "
	requireOperationSpecificDescription(mutated)
}

func TestCensusOperation34CanonicalSchemaRejectsCallerAuthorityAndCompletenessExpansion(t *testing.T) {
	registry := NewRegistryWithProfile(false, ToolProfileFull)
	tool, ok := registry.ResolveCanonical(mcpcontract.CensusTool)
	if !ok || tool.InputSchemaID != mcpcontract.CensusInputID || len(tool.EnvelopeSchemaIDs) != 2 || len(tool.ArtifactSchemaIDs) != 1 || tool.ArtifactSchemaIDs[0] != mcpcontract.CensusResultID {
		t.Fatalf("ASSERT_CENSUS_CANONICAL_SCHEMA_EXCLUSIVITY: %+v", tool)
	}
	valid := map[string]any{"session_id": "s", "generation": float64(1), "sources": []any{"."}}
	for _, field := range []string{"output_selector", "publication_root", "authority", "source_graph_complete"} {
		bad := map[string]any{}
		for key, value := range valid {
			bad[key] = value
		}
		bad[field] = "caller"
		if validateArguments(tool, bad) == nil {
			t.Fatalf("ASSERT_CENSUS_CALLER_FIELD_REJECTED_%s", field)
		}
	}
	result := map[string]any{}
	if err := json.Unmarshal([]byte(`{"schema_version":"lsp-trace.census-result.v1","status":"SUCCEEDED","census_id":"c","capture_set_id":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","session_id":"s","generation":1,"target_count":1,"batch_count":1,"file_accounting":{"denominator":1,"processed":1,"excluded":0,"forbidden":0,"unreadable":0,"unsupported":0,"document_symbol_failed":0,"omitted":0,"incomplete":0},"symbol_accounting":{"denominator":1,"prepared":1,"unsupported":0,"preparation_failed":0,"prepare_missing":0,"non_callable":0,"omitted":0,"incomplete":0},"authority":1,"source_graph_complete":"COMPLETE","native_aggregate_custody":false,"cross_capture_calls":[],"leiden_admissible":false,"publication":{"selector":"census.json","digest":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","byte_length":1,"verification_status":"VERIFIED","directory_sync_status":"COMPLETE","close_status":"COMPLETE"}}`), &result); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(result)
	if mcpcontract.ValidateJSON(mcpcontract.CensusResultID, raw) == nil {
		t.Fatal("ASSERT_CENSUS_AUTHORITY_COMPLETENESS_CEILINGS")
	}
}
