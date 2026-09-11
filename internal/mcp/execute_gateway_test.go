package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/operation"
	"lsp-trace/internal/publication"
)

type gatewayMatrixExecutor struct {
	calls     []operation.Request
	artifacts map[operation.Name][]byte
	failure   *operation.Failure
	value     any
}

func (e *gatewayMatrixExecutor) Execute(_ context.Context, request operation.Request) (operation.Result, *operation.Failure) {
	e.calls = append(e.calls, request)
	if e.failure != nil {
		return operation.Result{}, e.failure
	}
	if strings.HasPrefix(string(request.Name), "session_") || request.Name == operation.Capabilities {
		return operation.Result{Value: e.value}, nil
	}
	result := operation.Result{Artifact: e.artifacts[request.Name]}
	switch request.Name {
	case operation.BoundedRetainedAnalysisV2, operation.BoundedRetainedMetricsV2, operation.BoundedRetainedRankingV2:
		result.LogicalDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	}
	return result, nil
}

type gatewayMatrixCase struct {
	name string
	tool string
	args map[string]any
}

func gatewayMatrixCases() []gatewayMatrixCase {
	manifest := map[string]any{
		"schema_version": "lsp-trace.seed-manifest.v2", "coordinate_convention": "zero-based-session",
		"root":             map[string]any{"id": "root", "locator": map[string]any{"uri": "file:///workspace/main.go", "symbol": "Root"}},
		"required_targets": []any{map[string]any{"id": "target", "locator": map[string]any{"uri": "file:///workspace/main.go", "symbol": "Target"}}},
	}
	return []gatewayMatrixCase{
		{"v1-analysis", "lsp_trace_v1_bounded_retained_analysis", map[string]any{"input": `{}`, "operation": "PROJECT"}},
		{"v1-metrics", "lsp_trace_v1_bounded_retained_metrics", map[string]any{"input": `{}`}},
		{"v1-ranking", "lsp_trace_v1_bounded_retained_ranking", map[string]any{"input": `{}`, "algorithm": "PAGERANK"}},
		{"v1-export", "lsp_trace_v1_export_retained_calls", map[string]any{"input": `{}`}},
		{"v1-filter", "lsp_trace_v1_filter", map[string]any{"input": `{}`, "filter": map[string]any{"compare_seeds": []any{"a", "b"}}}},
		{"v1-inspect", "lsp_trace_v1_inspect", map[string]any{"input": `{}`, "selector": map[string]any{"all_seeds": true}}},
		{"v1-validate", "lsp_trace_v1_validate", map[string]any{"input": `{}`, "schema": map[string]any{"family": "graph", "version": "v3"}}},
		{"v1-verify", "lsp_trace_v1_verify", map[string]any{"input": `{}`}},
		{"v2-analysis", "lsp_trace_v2_bounded_retained_analysis", map[string]any{"input": `{}`, "operation": "ANALYSIS", "filter": []any{"CALLS"}, "max_work": 1}},
		{"v2-metrics", "lsp_trace_v2_bounded_retained_metrics", map[string]any{"input": `{}`, "operation": "METRICS", "filter": []any{"CALLS"}, "max_work": 1}},
		{"v2-ranking", "lsp_trace_v2_bounded_retained_ranking", map[string]any{"input": `{}`, "operation": "RANKING", "filter": []any{"CALLS"}, "max_work": 1}},
		{"v2-export", "lsp_trace_v2_export_retained_calls", map[string]any{"input": `{}`}},
		{"v2-incoming", "lsp_trace_v2_incoming", map[string]any{"session_id": "s", "generation": 1, "seed_manifest": manifest}},
		{"v2-slice", "lsp_trace_v2_slice", map[string]any{"session_id": "s", "generation": 1, "seed_manifest": manifest}},
		{"v2-verify", "lsp_trace_v2_verify", map[string]any{"input": `{}`, "schema": map[string]any{"family": "graph-provenance", "version": "v2"}}},
		{"v2-verify-retained", "lsp_trace_v2_verify_retained_calls", map[string]any{"input": `{}`, "schema": map[string]any{"family": "retained-calls", "version": "v2"}}},
		{"v3-incoming", "lsp_trace_v3_incoming", map[string]any{"session_id": "s", "generation": 1, "seed_manifest": manifest}},
		{"v3-slice", "lsp_trace_v3_slice", map[string]any{"session_id": "s", "generation": 1, "seed_manifest": manifest}},
		{"lifecycle", "lsp_session_v1_list", map[string]any{}},
		{"traversal", "lsp_trace_v1_incoming", map[string]any{"session_id": "s", "uri": "file:///workspace/main.go", "symbol": "Root"}},
		{"v1-slice", "lsp_trace_v1_slice", map[string]any{"session_id": "s", "start_mode": "at", "uri": "file:///workspace/main.go", "symbol": "Root"}},
		{"hydrated", "lsp_trace_v1_inspect_hydrated", map[string]any{"input": `{}`, "node_ids": []any{"n"}}},
	}
}

func matrixServer(registry *Registry, executor *gatewayMatrixExecutor) *Server {
	families := map[ExecutorFamily]Executor{}
	for _, family := range []ExecutorFamily{OfflineExecutorFamily, LifecycleExecutorFamily, IncomingExecutorFamily, SliceExecutorFamily, AcquisitionV2ExecutorFamily} {
		families[family] = executor
	}
	return &Server{Registry: registry, Executor: executor, Executors: families}
}

func matrixExecutor(registry *Registry) *gatewayMatrixExecutor {
	e := &gatewayMatrixExecutor{artifacts: map[operation.Name][]byte{}, value: map[string]any{"matrix": true}}
	for _, tc := range gatewayMatrixCases() {
		tool, ok := registry.ResolveCanonical(tc.tool)
		if ok && len(tool.ArtifactSchemaIDs) > 0 {
			e.artifacts[operationName(tool.Name)] = []byte(`{"$id":"` + tool.ArtifactSchemaIDs[0] + `"}`)
		}
	}
	return e
}

func callMessage(name string, args map[string]any) string {
	params, _ := json.Marshal(map[string]any{"name": name, "arguments": args})
	return `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":` + string(params) + `}` + "\n"
}

func directMatrixCall(server *Server, name string, args map[string]any) response {
	raw, _ := json.Marshal(callParams{Name: name, Arguments: args})
	return server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, raw)
}

func TestExecuteGatewayNestedDirectMatrix(t *testing.T) {
	const assertion = "ASSERT_MCP_EXECUTE_GATEWAY_MATRIX_DIRECT_EQUIVALENCE"
	t.Log("ASSERTION: " + assertion)
	for _, profile := range []ToolProfile{ToolProfileFull, ToolProfileCompact} {
		for _, tc := range gatewayMatrixCases() {
			t.Run(string(profile)+"/"+tc.name, func(t *testing.T) {
				registry := NewRegistryWithPublicationAndProfile(false, false, profile)
				directExecutor := matrixExecutor(registry)
				direct := directMatrixCall(matrixServer(registry, directExecutor), tc.tool, tc.args)
				if direct.Error != nil {
					t.Fatalf("%s direct target validation: %v", assertion, direct.Error)
				}

				gatewayExecutor := matrixExecutor(registry)
				nested := map[string]any{"request": map[string]any{"tool": tc.tool, "arguments": tc.args}}
				wrapped := runServerMessages(t, matrixServer(registry, gatewayExecutor), callMessage("lsp_trace_v1_execute", nested))[0]
				env := decodeEnvelopeForAssertion(t, assertion, wrapped)
				delegated, ok := env["delegated_envelope"].(string)
				if !ok {
					t.Fatalf("%s missing delegated envelope: %v", assertion, env)
				}
				directEnv := direct.Result.(callResult).StructuredContent
				directBytes, _ := json.Marshal(directEnv)
				sum := sha256.Sum256([]byte(delegated))
				if delegated != string(directBytes) || env["delegated_digest"] != "sha256:"+hex.EncodeToString(sum[:]) {
					t.Fatalf("%s exact bytes/digest: delegated=%s direct=%s env=%v", assertion, delegated, directBytes, env)
				}
				if env["requested_tool"] != tc.tool || env["delegated_outcome"] != directEnv.Outcome || env["delegated_is_error"] != directEnv.IsError {
					t.Fatalf("%s outcome/isError: %v delegated=%v", assertion, env, directEnv)
				}
				if len(directExecutor.calls) != 1 || len(gatewayExecutor.calls) != 1 {
					t.Fatalf("%s executor count: direct=%d gateway=%d", assertion, len(directExecutor.calls), len(gatewayExecutor.calls))
				}
				if directExecutor.calls[0].Name != operationName(tc.tool) || gatewayExecutor.calls[0].Name != operationName(tc.tool) || !reflect.DeepEqual(directExecutor.calls[0].Input, gatewayExecutor.calls[0].Input) {
					t.Fatalf("%s selected operation/input: direct=%#v gateway=%#v", assertion, directExecutor.calls[0], gatewayExecutor.calls[0])
				}
			})
		}
	}
}

func TestExecuteGatewayPublicationSelectorParity(t *testing.T) {
	const assertion = "ASSERT_MCP_EXECUTE_GATEWAY_PUBLICATION_SELECTOR_PARITY"
	t.Log("ASSERTION: " + assertion)
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "publication-error"}[existing], func(t *testing.T) {
			call := func(gateway bool) (response, []operation.Request) {
				registry := NewRegistryWithPublication(false, true)
				executor := matrixExecutor(registry)
				rootPath := t.TempDir()
				if existing {
					if err := os.WriteFile(filepath.Join(rootPath, "matrix.json"), []byte("occupied"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				root, err := publication.OpenRoot(rootPath)
				if err != nil {
					t.Fatal(err)
				}
				defer root.Close()
				server := matrixServer(registry, executor)
				server.PublicationRoot = root
				args := map[string]any{"input": `{}`, "output_selector": "matrix.json"}
				if gateway {
					args = map[string]any{"request": map[string]any{"tool": "lsp_trace_v1_export_retained_calls", "arguments": args}}
					return server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, "lsp_trace_v1_execute", args)), executor.calls
				}
				return server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, "lsp_trace_v1_export_retained_calls", args)), executor.calls
			}
			direct, directCalls := call(false)
			wrapped, gatewayCalls := call(true)
			if direct.Error != nil || wrapped.Error != nil || len(directCalls) != 1 || len(gatewayCalls) != 1 {
				t.Fatalf("%s calls/errors: direct=%v nested=%v", assertion, direct, wrapped)
			}
			directBytes, _ := json.Marshal(direct.Result.(callResult).StructuredContent)
			env := wrapped.Result.(callResult).StructuredContent
			if env.DelegatedEnvelope != string(directBytes) || env.DelegatedDigest == "" || (existing && env.DelegatedOutcome != "PUBLICATION_ERROR") || (!existing && env.DelegatedOutcome != "COMPLETE") {
				t.Fatalf("%s existing=%v envelope=%+v direct=%s", assertion, existing, env, directBytes)
			}
		})
	}
}

func mustCallParams(t *testing.T, name string, args map[string]any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(callParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestExecuteGatewayValidationAndErrorParity(t *testing.T) {
	const assertion = "ASSERT_MCP_EXECUTE_GATEWAY_MATRIX_HERMETIC"
	t.Log("ASSERTION: " + assertion)
	registry := NewRegistry(false)
	invalid := map[string]any{"input": `{}`, "algorithm": "UNKNOWN"}
	direct := directMatrixCall(matrixServer(registry, matrixExecutor(registry)), "lsp_trace_v1_bounded_retained_ranking", invalid)
	nested := map[string]any{"request": map[string]any{"tool": "lsp_trace_v1_bounded_retained_ranking", "arguments": invalid}}
	wrapped := runServerMessages(t, matrixServer(registry, matrixExecutor(registry)), callMessage("lsp_trace_v1_execute", nested))[0]
	if wrapped["error"] == nil || direct.Error == nil || wrapped["error"].(map[string]any)["message"] != direct.Error.Message {
		t.Fatalf("%s target validation differs: direct=%v nested=%v", assertion, direct.Error, wrapped)
	}
	for _, failure := range []*operation.Failure{
		{Code: operation.FailureInvalidInput, Diagnostics: []string{"domain"}},
		{Code: "CANCELLED", Diagnostics: []string{"cancel"}},
	} {
		directExecutor := matrixExecutor(registry)
		directExecutor.failure = failure
		direct = directMatrixCall(matrixServer(registry, directExecutor), "lsp_trace_v2_incoming", gatewayMatrixCases()[12].args)
		gatewayExecutor := matrixExecutor(registry)
		gatewayExecutor.failure = failure
		nested = map[string]any{"request": map[string]any{"tool": "lsp_trace_v2_incoming", "arguments": gatewayMatrixCases()[12].args}}
		wrapped = runServerMessages(t, matrixServer(registry, gatewayExecutor), callMessage("lsp_trace_v1_execute", nested))[0]
		env := decodeEnvelopeForAssertion(t, assertion, wrapped)
		directBytes, _ := json.Marshal(direct.Result.(callResult).StructuredContent)
		if env["delegated_envelope"] != string(directBytes) || env["delegated_outcome"] != "DOMAIN_ERROR" || env["delegated_is_error"] != true {
			t.Fatalf("%s domain/cancel parity: %v", assertion, env)
		}
	}
}

func TestExecuteGatewayRejectsAliasUnknownRecursiveAndMalformed(t *testing.T) {
	const assertion = "ASSERT_MCP_EXECUTE_GATEWAY_REJECTS_NONCANONICAL_TARGETS"
	server := &Server{Registry: NewRegistry(false), Executor: &fakeExecutor{}}
	inputs := []string{
		`{"tool":"lsp_trace_capabilities","arguments":{}}`,
		`{"tool":"lsp_trace_v9_missing","arguments":{}}`,
		`{"tool":"lsp_trace_v1_execute","arguments":{"request":{}}}`,
		`{"tool":"lsp_trace_v1_capabilities"}`,
	}
	for i, input := range inputs {
		response := runServerMessages(t, server, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lsp_trace_v1_execute","arguments":{"request":`+input+`}}}`+"\n")[0]
		if response["error"] == nil || !strings.Contains(response["error"].(map[string]any)["message"].(string), "Invalid tool arguments") {
			t.Fatalf("%s[%d]: %v", assertion, i, response)
		}
	}
}

func TestExecuteGatewayNoDepthBypass(t *testing.T) {
	const assertion = "ASSERT_MCP_EXECUTE_GATEWAY_NO_DEPTH_BYPASS"
	server := matrixServer(NewRegistry(false), &gatewayMatrixExecutor{artifacts: map[operation.Name][]byte{}})
	request := map[string]any{"tool": "lsp_trace_v1_execute", "arguments": map[string]any{"request": map[string]any{"tool": "lsp_trace_v1_capabilities", "arguments": map[string]any{}}}}
	response := runServerMessages(t, server, callMessage("lsp_trace_v1_execute", map[string]any{"request": request}))[0]
	if response["error"] == nil || !strings.Contains(response["error"].(map[string]any)["message"].(string), "recursive execute gateway is forbidden") {
		t.Fatalf("%s: %v", assertion, response)
	}
}

func TestExecuteGatewayPreservesLegacyCustodyPath(t *testing.T) {
	const assertion = "ASSERT_MCP_EXECUTE_GATEWAY_LEGACY_CUSTODY_UNCHANGED"
	artifact := []byte(`{"$id":"https://jaresty.github.io/lsp-trace/schemas/lsp-trace.execution.v1.schema.json","execution_version":"1","result":{}}`)
	executor := &executionTransportExecutor{artifact: artifact, digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	server := &Server{Registry: NewRegistry(false), Executor: executor}
	response := runServerMessages(t, server, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lsp_trace_v1_execute","arguments":{"request":{}}}}`+"\n")[0]
	env := decodeEnvelopeForAssertion(t, assertion, response)
	if len(executor.calls) != 1 || env["content"] != string(artifact) || env["delegated_envelope"] != nil {
		t.Fatalf("%s: calls=%v envelope=%v", assertion, executor.calls, env)
	}
}
