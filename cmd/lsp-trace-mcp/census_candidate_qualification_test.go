package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/mcp"
	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/publication"
)

const candidate34TransportResidual = "operation-34 JSON-RPC direct/canonical transport and real-gopls COMPLETE remain unqualified until registration and a successful production acquisition"

// TestPrivateCensusCandidateBinaryQualification is deliberately below the MCP
// transport boundary. It combines the strongest currently executable seams
// without registering operation 34: a fresh production-managed gopls session
// reaches the private runtime/binding's sanitized domain boundary, while the
// production completion and envelope path proves terminal publication behavior.
func TestPrivateCensusCandidateBinaryQualification(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("candidate qualification requires the production LocalDarwinSupervisor")
	}
	if _, failure := newPrivateCensusMCPBinding(nil).Execute(context.Background(), operation.Request{Name: operation.Verify}); failure == nil || failure.Code != operation.FailureInvalidInput {
		t.Fatalf("ASSERT_CANDIDATE34_OPERATION_NAME_PRE_RUNTIME: failure=%+v", failure)
	}
	candidateBinary := buildMCPBinary(t)
	if !filepath.IsAbs(candidateBinary) {
		t.Fatalf("ASSERT_CANDIDATE34_PRODUCTION_BINARY_BUILT: %q", candidateBinary)
	}
	t.Logf("candidate binary built: %s", candidateBinary)

	workspace := t.TempDir()
	for name, body := range map[string]string{
		"go.mod":  "module example.com/candidate34\n\ngo 1.22\n",
		"main.go": "package fixture\n\nfunc Target() {}\n",
	} {
		if err := os.WriteFile(filepath.Join(workspace, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	gopls, err := exec.LookPath("gopls")
	if err != nil || !filepath.IsAbs(gopls) {
		t.Fatalf("ASSERT_CANDIDATE34_INSTALLED_GOPLS_REQUIRED: path=%q err=%v", gopls, err)
	}
	version, err := exec.Command(gopls, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("ASSERT_CANDIDATE34_GOPLS_VERSION: %v %s", err, version)
	}
	t.Logf("candidate server: %s", strings.TrimSpace(string(version)))

	publicationPath := t.TempDir()
	root, err := publication.OpenRoot(publicationPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	server, manager, err := newServerRuntime(true, root)
	if err != nil {
		t.Fatal(err)
	}
	config := bootstrapConfig{Version: 1, Processes: []bootstrapProcessConfig{{
		Alias:     "candidate-34",
		Profile:   bootstrapProfileIdentity{TrustDomain: "candidate-34", Workspace: workspace, Profile: "gopls", EnvironmentReference: "qualification"},
		Execution: managedExecutionAuthority{Path: gopls, Directory: workspace},
	}}}
	sessions, err := startBootstrap(context.Background(), manager, config, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stopBootstrap(context.Background(), manager, sessions) })
	if len(sessions) != 1 {
		t.Fatalf("ASSERT_CANDIDATE34_FRESH_MANAGED_SESSION: sessions=%+v", sessions)
	}
	contextTool, registered := server.Registry.Resolve("lsp_trace_v1_structural_context")
	if !registered || contextTool.ExecutorFamily != mcp.StructuralContextExecutorFamily || server.Executors[mcp.StructuralContextExecutorFamily] == nil {
		t.Fatalf("ASSERT_CANDIDATE35_STRUCTURAL_CONTEXT_REGISTERED_AND_BOUND: tool=%+v registered=%t", contextTool, registered)
	}

	binding := newPrivateCensusMCPBinding(newCensusRuntime(newHostSelectorRuntime(manager, sessions)))
	input, _ := json.Marshal(map[string]any{
		"session_id": sessions[0].Alias, "generation": sessions[0].Generation,
		"sources": []string{"."}, "down_depth": 0, "up_depth": 0,
		"max_nodes": 10, "timeout_ms": 2000, "request_timeout_ms": 1000,
	})
	realProcess, err := binding.callDirect(context.Background(), operation.Request{
		Name: operation.Census, RequestID: "candidate-real-gopls-34", Input: input, PublicationRoot: root,
	})
	if err != nil {
		t.Fatal(err)
	}
	var realEnvelope censusMCPEnvelope
	if err := json.Unmarshal(realProcess.Structured, &realEnvelope); err != nil {
		t.Fatal(err)
	}
	if realEnvelope.Outcome != "DOMAIN_ERROR" || realEnvelope.OperationStatus != "FAILED" || !realProcess.IsError || realEnvelope.RequestID != "candidate-real-gopls-34" || realEnvelope.EnvelopeSchemaID != mcpcontract.FutureCensusDomainErrorID {
		t.Fatalf("ASSERT_CANDIDATE34_REAL_GOPLS_FAILS_CLOSED: envelope=%+v raw=%s", realEnvelope, realProcess.Structured)
	}
	if strings.Contains(string(realProcess.Structured), publicationPath) || strings.Contains(string(realProcess.Structured), workspace) || strings.Contains(string(realProcess.Structured), gopls) {
		t.Fatalf("ASSERT_CANDIDATE34_REAL_GOPLS_DOMAIN_PRIVACY: %s", realProcess.Structured)
	}
	if entries, err := os.ReadDir(publicationPath); err != nil || len(entries) != 0 {
		t.Fatalf("ASSERT_CANDIDATE34_REAL_GOPLS_PRECOMMIT_ZERO_PUBLICATION: entries=%v err=%v", entries, err)
	}

	for _, tc := range []struct {
		name, requestID, outcome string
		degraded                 bool
	}{
		{name: "complete", requestID: "candidate-complete-34", outcome: "COMPLETE"},
		{name: "committed-degraded", requestID: "candidate-degraded-34", outcome: "COMMITTED_DEGRADED", degraded: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			projection, receipt := validMCPCompletionProjection(), validMCPCompletionReceipt()
			if tc.degraded {
				receipt.CloseStatus = "COMMITTED_CLOSE_FAILED"
			}
			publications := 0
			publisher := completionPublisher(receipt, func() { publications++ })
			verifier := &censusVerifierStub{manifest: captureset.Manifest{Constituents: []captureset.Constituent{{ImmutableSelector: "graphs/v5/a.json"}}}}
			completion := completeCensusProjectionWith(context.Background(), projection, publisher, verifier)
			got, err := projectPrivateCensusMCP(tc.requestID, completion)
			if err != nil {
				t.Fatal(err)
			}
			if publications != 1 {
				t.Fatalf("ASSERT_CANDIDATE34_ZERO_DUPLICATE_PUBLICATION: publications=%d", publications)
			}
			if err := mcpcontract.ValidateFutureCensusEnvelopeExclusive(got.Structured); err != nil {
				t.Fatalf("ASSERT_CANDIDATE34_EXACT_ENVELOPE_SCHEMA: %v", err)
			}
			var envelope map[string]any
			if err := json.Unmarshal(got.Structured, &envelope); err != nil {
				t.Fatal(err)
			}
			if envelope["outcome"] != tc.outcome || envelope["operation_status"] != "SUCCEEDED" || envelope["request_id"] != tc.requestID || envelope["envelope_schema_id"] != mcpcontract.FutureCensusSuccessID || got.IsError || !bytes.Equal(got.Text, got.Structured) {
				t.Fatalf("ASSERT_CANDIDATE34_TERMINAL_ENVELOPE: %v", envelope)
			}
			result := envelope["result"].(map[string]any)
			if tc.degraded {
				if result["stage"] != "committed-degradation" || result["status"] != "SUCCEEDED_DEGRADED" || result["code"] != "COMMITTED_DEGRADED" {
					t.Fatalf("ASSERT_CANDIDATE34_COMMITTED_DEGRADED: %v", result)
				}
				return
			}
			if result["authority"] != float64(0) || result["source_graph_complete"] != "UNKNOWN" {
				t.Fatalf("ASSERT_CANDIDATE34_ZERO_AUTHORITY_UNKNOWN_SOURCE_GRAPH: %v", result)
			}
			publicationEvidence := result["publication"].(map[string]any)
			if publicationEvidence["selector"] != receipt.Selector || publicationEvidence["digest"] != receipt.ArtifactSHA256 || publicationEvidence["byte_length"] != float64(receipt.ByteLength) || publicationEvidence["verification_status"] != "VERIFIED" {
				t.Fatalf("ASSERT_CANDIDATE34_EXACT_PUBLICATION_DESCRIPTOR_VERIFICATION: got=%v receipt=%+v", publicationEvidence, receipt)
			}
			if raw := string(got.Structured); strings.Contains(raw, publicationPath) || strings.Contains(raw, workspace) || strings.Contains(raw, gopls) {
				t.Fatalf("ASSERT_CANDIDATE34_COMPLETE_PRIVACY: %s", raw)
			}
		})
	}

	t.Log("PASS ASSERT_CANDIDATE34_PRE_REGISTRATION_GATE; residual=" + candidate34TransportResidual)
}

func TestCensusOperation34RealProcessQualification(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("candidate qualification requires the production LocalDarwinSupervisor")
	}
	binary := buildMCPBinary(t)
	gopls, err := exec.LookPath("gopls")
	if err != nil || !filepath.IsAbs(gopls) {
		t.Fatalf("ASSERT_CENSUS34_REAL_PROCESS_GOPLS_REQUIRED: path=%q err=%v", gopls, err)
	}
	version, err := exec.Command(gopls, "version").CombinedOutput()
	if err != nil || !strings.Contains(string(version), "v0.23.0") {
		t.Fatalf("ASSERT_CENSUS34_EXACT_GOPLS_VERSION: err=%v version=%q", err, version)
	}
	workspace := t.TempDir()
	for name, body := range map[string]string{
		"go.mod":  "module example.com/census34qualification\n\ngo 1.22\n",
		"main.go": "package fixture\n\nfunc Target() {}\nfunc Caller() { Target() }\n",
	} {
		if err := os.WriteFile(filepath.Join(workspace, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	bootstrap := writeBootstrapJSON(t, map[string]any{
		"version": 1,
		"processes": []any{map[string]any{
			"alias": "census-34", "language_id": "go",
			"profile":   map[string]any{"trust_domain": "census-34-qualification", "workspace": workspace, "profile": "gopls", "environment_reference": "qualification"},
			"execution": map[string]any{"path": gopls, "directory": workspace},
		}},
	})
	args := map[string]any{
		"session_id": "census-34", "generation": 1, "sources": []any{"main.go"},
		"down_depth": 0, "up_depth": 0, "max_nodes": 20,
		"timeout_ms": 60000, "request_timeout_ms": 30000,
	}
	gatewayArgs := map[string]any{}
	for key, value := range args {
		gatewayArgs[key] = value
	}
	gatewayArgs["down_depth"] = 1
	listRequest := map[string]any{"jsonrpc": "2.0", "id": 34, "method": "tools/list", "params": map[string]any{}}

	publicationRoot := t.TempDir()
	if err := os.Chmod(publicationRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	responses := runMCPProcess(t, binary, []string{"--tool-profile", "full", "--bootstrap-config", bootstrap, "--publication-root", publicationRoot}, []map[string]any{
		listRequest,
		callRequest(3401, mcpcontract.CensusTool, args),
		callRequest(3402, "lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": mcpcontract.CensusTool, "arguments": gatewayArgs}}),
	})
	tools := processToolNames(t, responses[0])
	if len(tools) != 37 || tools[8] != mcpcontract.CensusTool || !containsString(tools, mcpcontract.StructuralContextTool) {
		t.Fatalf("ASSERT_CENSUS34_FULL_LIST_PRESERVED_WITH_37_APPEND: count=%d tools=%v", len(tools), tools)
	}
	direct := decodeProcessCall(t, responses[1])
	gateway := decodeProcessCall(t, responses[2])
	if direct.env["request_id"] != "offline-1" || gateway.env["request_id"] != "offline-3" {
		t.Fatalf("ASSERT_CENSUS34_DISTINCT_DIRECT_OUTER_IDS: direct=%v outer=%v", direct.env["request_id"], gateway.env["request_id"])
	}
	delegated, ok := gateway.env["delegated_envelope"].(string)
	if !ok || !strings.Contains(delegated, `"request_id":"offline-2"`) || strings.Contains(delegated, `"request_id":"offline-3"`) || mcpcontract.ValidateEnvelopeExclusive([]byte(delegated)) != nil {
		t.Fatalf("ASSERT_CENSUS34_DISTINCT_DELEGATED_ID_AND_SCHEMA: %q", delegated)
	}
	var delegatedEnvelope map[string]any
	if err := json.Unmarshal([]byte(delegated), &delegatedEnvelope); err != nil {
		t.Fatal(err)
	}
	completeCount := 0
	selectors := map[string]struct{}{}
	for name, envelope := range map[string]map[string]any{"direct": direct.env, "delegated": delegatedEnvelope} {
		raw, err := json.Marshal(envelope)
		if err != nil || mcpcontract.ValidateFutureCensusEnvelopeExclusive(raw) != nil {
			t.Fatalf("ASSERT_CENSUS34_%s_EXACT_SCHEMA: err=%v envelope=%s", name, err, raw)
		}
		if envelope["outcome"] != "COMPLETE" || envelope["operation_status"] != "SUCCEEDED" || envelope["isError"] != false {
			t.Fatalf("ASSERT_CENSUS34_%s_REAL_COMPLETE: %v", name, envelope)
		}
		completeCount++
		result, _ := envelope["result"].(map[string]any)
		if result["authority"] != float64(0) || result["source_graph_complete"] != "UNKNOWN" {
			t.Fatalf("ASSERT_CENSUS34_%s_AUTHORITY_COMPLETENESS: %v", name, result)
		}
		publicationEvidence, _ := result["publication"].(map[string]any)
		selector, _ := publicationEvidence["selector"].(string)
		if selector == "" || publicationEvidence["verification_status"] != "VERIFIED" {
			t.Fatalf("ASSERT_CENSUS34_%s_EXACT_VERIFIED_DESCRIPTOR: %v", name, publicationEvidence)
		}
		if _, duplicate := selectors[selector]; duplicate {
			t.Fatalf("ASSERT_CENSUS34_ONE_IMMUTABLE_PUBLICATION_PER_SUCCESS: duplicate=%q", selector)
		}
		selectors[selector] = struct{}{}
		published, err := os.ReadFile(filepath.Join(publicationRoot, filepath.FromSlash(selector)))
		digest := sha256.Sum256(published)
		if err != nil || len(published) != int(publicationEvidence["byte_length"].(float64)) || "sha256:"+hex.EncodeToString(digest[:]) != publicationEvidence["digest"] {
			t.Fatalf("ASSERT_CENSUS34_%s_DESCRIPTOR_BYTES: err=%v descriptor=%v", name, err, publicationEvidence)
		}
		if raw := string(raw); strings.Contains(raw, workspace) || strings.Contains(raw, publicationRoot) || strings.Contains(raw, gopls) {
			t.Fatalf("ASSERT_CENSUS34_%s_PRIVACY: %s", name, raw)
		}
	}
	if completeCount != 2 || len(selectors) != 2 {
		t.Fatalf("ASSERT_CENSUS34_EXACTLY_ONE_PUBLICATION_PER_COMPLETE: complete=%d selectors=%d", completeCount, len(selectors))
	}

	compactRoot := t.TempDir()
	if err := os.Chmod(compactRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	compact := runMCPProcess(t, binary, []string{"--tool-profile", "compact", "--bootstrap-config", bootstrap, "--publication-root", compactRoot}, []map[string]any{
		listRequest,
		callRequest(3501, mcpcontract.CensusTool, map[string]any{"session_id": "census-34", "generation": 999, "sources": []any{"."}}),
		callRequest(3502, "lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": mcpcontract.CensusTool, "arguments": map[string]any{"session_id": "census-34", "generation": 999, "sources": []any{"."}}}}),
	})
	if names := processToolNames(t, compact[0]); containsString(names, mcpcontract.CensusTool) {
		t.Fatalf("ASSERT_CENSUS34_COMPACT_HIDDEN: %v", names)
	}
	compactDirect := decodeProcessCall(t, compact[1])
	compactGateway := decodeProcessCall(t, compact[2])
	if compactDirect.env["tool"] != mcpcontract.CensusTool || strings.Contains(string(mustJSON(t, compactGateway.env)), "unknown canonical tool") {
		t.Fatalf("ASSERT_CENSUS34_COMPACT_CANONICALLY_CALLABLE: direct=%v gateway=%v", compactDirect.env, compactGateway.env)
	}
	t.Logf("PASS ASSERT_CENSUS34_REAL_PROCESS_QUALIFIED outcome=%v/%v diagnostic=%v/%v complete_publications=%d", direct.env["outcome"], delegatedEnvelope["outcome"], direct.env["error"], delegatedEnvelope["error"], completeCount)
}

func processToolNames(t *testing.T, response map[string]any) []string {
	t.Helper()
	result, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatalf("ASSERT_CENSUS34_TOOLS_LIST_RESULT: %v", response)
	}
	raw, ok := result["tools"].([]any)
	if !ok {
		t.Fatalf("ASSERT_CENSUS34_TOOLS_LIST_SHAPE: %v", result)
	}
	names := make([]string, len(raw))
	for i, item := range raw {
		tool, _ := item.(map[string]any)
		names[i], _ = tool["name"].(string)
	}
	return names
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
