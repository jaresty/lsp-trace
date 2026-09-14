package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/captureset"
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
	for _, name := range []string{mcpcontract.FutureCensusTool, "lsp_trace_v1_structural_context"} {
		if _, registered := server.Registry.Resolve(name); registered {
			t.Fatalf("ASSERT_CANDIDATE34_34_35_UNREGISTERED: %s", name)
		}
	}

	binding := newPrivateCensusMCPBinding(newCensusRuntime(newHostSelectorRuntime(manager, sessions)))
	input, _ := json.Marshal(map[string]any{
		"session_id": sessions[0].Alias, "generation": sessions[0].Generation,
		"sources": []string{"."}, "down_depth": 0, "up_depth": 0,
		"max_nodes": 10, "timeout_ms": 2000, "request_timeout_ms": 1000,
	})
	realProcess, err := binding.callDirect(context.Background(), operation.Request{
		Name: mcpcontract.FutureCensusTool, RequestID: "candidate-real-gopls-34", Input: input, PublicationRoot: root,
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
