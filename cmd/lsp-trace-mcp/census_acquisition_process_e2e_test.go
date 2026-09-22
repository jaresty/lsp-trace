package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/censusdiagnostic"
)

func TestRealProcessCensusAcquisitionTimeoutAfterDiscovery(t *testing.T) {
	mcp := buildMCPBinary(t)
	fake := buildBinary(t, "fake-lsp", "./cmd/fake-lsp")
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package fixture\n\nfunc Target() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	publication := t.TempDir()
	if err := os.Chmod(publication, 0700); err != nil {
		t.Fatal(err)
	}
	diagnosticParent := t.TempDir()
	if err := os.Chmod(diagnosticParent, 0700); err != nil {
		t.Fatal(err)
	}
	diagnostic := filepath.Join(diagnosticParent, "acquisition.ndjson")
	trace := filepath.Join(diagnosticParent, "fake.trace")
	config := map[string]any{"version": 1, "processes": []any{map[string]any{
		"alias": "census-process", "language_id": "go",
		"profile":   map[string]any{"trust_domain": "census-process-e2e", "workspace": workspace, "profile": "fake-lsp", "environment_reference": "e2e"},
		"execution": map[string]any{"path": fake, "directory": workspace, "environment": []string{"LSP_TRACE_FAKE_LSP_ACQUISITION=hang-outgoing", "LSP_TRACE_FAKE_LSP_TRACE=" + trace}},
	}}}
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "bootstrap.json")
	if err := os.WriteFile(configPath, raw, 0600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, mcp, "--enable-live-lsp", "--tool-profile", "advanced", "--bootstrap-config", configPath, "--publication-root", publication, "--acquisition-diagnostic-path", diagnostic)
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(out)
	encoder := json.NewEncoder(in)
	send := func(request map[string]any) map[string]any {
		if err := encoder.Encode(request); err != nil {
			t.Fatal(err)
		}
		line, err := reader.ReadBytes('\n')
		if err != nil {
			t.Fatal(err)
		}
		var response map[string]any
		if err := json.Unmarshal(line, &response); err != nil {
			t.Fatal(err)
		}
		return response
	}

	list := send(callRequest(1, "lsp_session_v1_list", map[string]any{"detail": "full"}))
	listRaw, _ := json.Marshal(list)
	if !bytes.Contains(listRaw, []byte(`"State":"READY"`)) || !bytes.Contains(listRaw, []byte(`"Generation":1`)) {
		t.Fatalf("ASSERT_DISCOVERY_READY: %s", listRaw)
	}
	var envelope struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(listRaw, &envelope); err != nil || len(envelope.Result.Content) != 1 {
		t.Fatalf("ASSERT_SESSION_LIST_JSON: %s", listRaw)
	}
	var payload struct {
		Result struct {
			Sessions []struct {
				SessionID  string  `json:"SessionID"`
				Generation float64 `json:"Generation"`
				State      string  `json:"State"`
				Routing    struct {
					Alias string `json:"alias"`
				} `json:"routing"`
			} `json:"Sessions"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(envelope.Result.Content[0].Text), &payload); err != nil {
		t.Fatalf("ASSERT_SESSION_LIST_CONTENT_JSON: %v", err)
	}
	var sessionID string
	var generation float64
	for _, record := range payload.Result.Sessions {
		if record.Routing.Alias == "census-process" {
			if record.State != "READY" {
				t.Fatalf("ASSERT_EXACT_READY_SESSION_STATE: %+v", record)
			}
			sessionID, generation = record.SessionID, record.Generation
			break
		}
	}
	t.Logf("READY_SESSION id=%q generation=%v", sessionID, generation)
	if sessionID == "" || generation != 1 {
		t.Fatalf("ASSERT_EXACT_READY_SESSION: id=%q generation=%v", sessionID, generation)
	}
	args := map[string]any{"session_id": sessionID, "generation": generation, "sources": []string{"main.go"}, "down_depth": 1, "up_depth": 0, "max_nodes": 10, "timeout_ms": 1000, "request_timeout_ms": 100}
	response := send(callRequest(2, "lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": "lsp_trace_v1_census", "arguments": args}}))
	_ = in.Close()
	waitErr := cmd.Wait()
	if waitErr != nil && ctx.Err() == nil {
		t.Fatalf("ASSERT_PROCESS_CLEANUP: %v stderr=%s", waitErr, stderr.String())
	}
	call := decodeProcessCall(t, response)
	delegated, ok := call.env["delegated_envelope"].(string)
	if !ok || !strings.Contains(delegated, `"code":"ACQUISITION_FAILED"`) {
		t.Fatalf("ASSERT_PUBLIC_GENERIC_ACQUISITION_FAILED: %v", call.env)
	}
	if strings.Contains(delegated, workspace) || strings.Contains(delegated, diagnostic) || strings.Contains(delegated, "hang-outgoing") {
		t.Fatalf("ASSERT_PUBLIC_NO_PRIVATE_LEAK: %s", delegated)
	}
	t.Logf("RED_EVIDENCE delegated=%s stderr=%q", delegated, stderr.String())
	traceBytes, _ := os.ReadFile(trace)
	t.Logf("FAKE_LSP_TRACE=%s", traceBytes)
	traceText := string(traceBytes)
	for _, marker := range []string{"textDocument/documentSymbol", "textDocument/prepareCallHierarchy", "callHierarchy/outgoingCalls"} {
		if !strings.Contains(traceText, marker) {
			t.Fatalf("ASSERT_DISCOVERY_REACHED_OUTGOING: missing %q trace=%q", marker, traceText)
		}
	}
	contents, err := os.ReadFile(diagnostic)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("DIAGNOSTIC_CONTENT=%q", contents)
	lines := bytes.Split(bytes.TrimSpace(contents), []byte{'\n'})
	if len(lines) != 1 {
		t.Fatalf("ASSERT_ONE_PRIVATE_RECORD: %q", contents)
	}
	var record censusdiagnostic.Record
	if err := json.Unmarshal(lines[0], &record); err != nil {
		t.Fatalf("ASSERT_PRIVATE_RECORD_JSON: %v contents=%q", err, contents)
	}
	if record.Fingerprint == "" || record.Ordinal == nil || record.OperationCode == "" || record.OperationCategory == "" {
		t.Fatalf("ASSERT_TRUTHFUL_PRIVATE_RECORD: %+v", record)
	}
	if stderr.Len() > 4096 {
		t.Fatalf("ASSERT_BOUNDED_STDERR: %d", stderr.Len())
	}
}
