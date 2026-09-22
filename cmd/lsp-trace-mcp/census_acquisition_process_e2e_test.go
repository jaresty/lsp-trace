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

func TestRealProcessCensusContinuationStopAfterDescribeRequests(t *testing.T) {
	mcp := buildMCPBinary(t)
	fake := buildBinary(t, "fake-lsp", "./cmd/fake-lsp")
	workspace := t.TempDir()
	if err := os.Chmod(workspace, 0700); err != nil {
		t.Fatal(err)
	}
	fixtures := map[string]string{
		"main.go": "package fixture\n\nfunc Target() {\n\tPeer()\n}\n",
		"peer.go": "package fixture\n\nfunc Peer() {}\n",
	}
	for name, contents := range fixtures {
		if err := os.WriteFile(filepath.Join(workspace, name), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	publication := freshDir(t, "publication")
	continuation := freshDir(t, "continuation")
	managed := freshFile(t, "managed.ndjson")
	trace := freshFile(t, "fake.trace")
	runtimeTrace := freshFile(t, "runtime.trace")
	config := map[string]any{"version": 1, "continuation": map[string]any{"publication_root": continuation, "max_object_bytes": 64 << 20, "capabilities": []string{"STOP_AFTER_DESCRIBE_REQUESTS"}, "managed_preparation_diagnostic_path": managed}, "processes": []any{map[string]any{
		"alias": "census-process", "language_id": "go", "profile": map[string]any{"trust_domain": "census-process-e2e", "workspace": workspace, "profile": "fake-lsp", "environment_reference": "e2e"},
		"execution": map[string]any{"path": fake, "directory": workspace, "environment": []string{"LSP_TRACE_FAKE_LSP_DOCUMENT_SYMBOL=process-target", "LSP_TRACE_FAKE_LSP_TRACE=" + trace}},
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
	cmd := exec.CommandContext(ctx, mcp, "--enable-live-lsp", "--tool-profile", "advanced", "--bootstrap-config", configPath, "--publication-root", publication)
	cmd.Env = append(retentionChildEnvironment(os.Environ()), "LSP_TRACE_GO_RUNTIME_TRACE_PATH="+runtimeTrace, "LSP_TRACE_CENSUS_MAPPING_DIAGNOSTICS=1")
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
	send := func(id int, name string, args map[string]any) map[string]any {
		if err := encoder.Encode(callRequest(id, name, args)); err != nil {
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
	list := send(1, "lsp_session_v1_list", map[string]any{"detail": "full"})
	sessionID, generation := exactReadySession(t, list)
	targetBytes, err := os.ReadFile(filepath.Join(workspace, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(targetBytes); got != fixtures["main.go"] {
		t.Fatalf("ASSERT_TARGET_SOURCE_BYTES: %q", got)
	}
	peerBytes, err := os.ReadFile(filepath.Join(workspace, "peer.go"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(peerBytes); got != fixtures["peer.go"] {
		t.Fatalf("ASSERT_PEER_SOURCE_BYTES: %q", got)
	}
	args := map[string]any{"session_id": sessionID, "generation": generation, "sources": []string{"main.go", "peer.go"}, "down_depth": 1, "up_depth": 0, "max_nodes": 10000, "batch_targets": 1, "timeout_ms": 60000, "request_timeout_ms": 30000, "continuation": map[string]any{"kind": "ADR_0007_FEATURE_CATALOG", "stop_after": "DESCRIBE_REQUESTS"}}
	direct := send(2, "lsp_trace_v1_census", args)
	post := send(3, "lsp_session_v1_list", map[string]any{"detail": "full"})
	_ = in.Close()
	if waitErr := cmd.Wait(); waitErr != nil && ctx.Err() == nil {
		t.Fatalf("ASSERT_PROCESS_CLEANUP: %v stderr=%s", waitErr, stderr.String())
	}
	if direct["error"] != nil {
		t.Logf("RED_TRANSPORT_RESPONSE: %s", mustJSON(t, direct))
		t.Logf("RED_STDERR_QUOTED: %q", stderr.String())
		logProcessE2ERoot(t, "RED_PUBLICATION_ROOT", publication)
		logProcessE2ERoot(t, "RED_FAKE_TRACE", trace)
		logProcessE2ERoot(t, "RED_MANAGED_DIAGNOSTIC", managed)
		logProcessE2ERoot(t, "RED_RUNTIME_TRACE", runtimeTrace)
		logProcessE2ERoot(t, "RED_CONTINUATION_ROOT", continuation)
		t.Fatalf("ASSERT_REAL_PROCESS_CALL_RESULT: response=%v", direct)
	}
	d := decodeProcessCall(t, direct)
	retentionParent := retentionParentFromEnv(t)
	if retentionParent != "" {
		if d.env["outcome"] != "COMMITTED_DEGRADED" {
			t.Fatalf("ASSERT_RETENTION_EXPECTED_DIAGNOSTIC_OUTCOME: outcome=%v", d.env["outcome"])
		}
		selector, err := retentionSelectorFromResponse(direct)
		if err != nil {
			t.Fatalf("ASSERT_RETENTION_SELECTOR_PRESENT: %v", err)
		}
		privateTrace, err := os.ReadFile(runtimeTrace)
		if err != nil {
			t.Fatalf("ASSERT_RETENTION_PRIVATE_TRACE_READ: %v", err)
		}
		if !strings.Contains(string(privateTrace), "CONTINUATION_PRECONDITION_CAUSE subcause=HANDOFF_COMPOSE\nCENSUS_HANDOFF_STAGE stage=COMPOSITION\nCENSUS_HANDOFF_BRANCH branch=NODE_CONFLICT") {
			t.Fatalf("ASSERT_RETENTION_EXPECTED_PRIVATE_NODE_CONFLICT: diagnostic trace branch differs")
		}
		leaf, source := retainCommittedCensusForTest(t, retentionParent, publication, selector)
		verifyRetentionCopy(t, leaf, selector, source)
	}
	if d.env["operation_status"] != "SUCCEEDED" || d.env["outcome"] != "PAUSED" || d.env["isError"] != false {
		t.Logf("RED_PUBLIC_DIAGNOSTIC: %s", mustJSON(t, d.env))
		t.Logf("RED_STDERR_QUOTED: %q", stderr.String())
		logProcessE2ERoot(t, "RED_PUBLICATION_ROOT", publication)
		logProcessE2ERoot(t, "RED_FAKE_TRACE", trace)
		logProcessE2ERoot(t, "RED_MANAGED_DIAGNOSTIC", managed)
		logProcessE2ERoot(t, "RED_RUNTIME_TRACE", runtimeTrace)
		logProcessE2ERoot(t, "RED_CONTINUATION_ROOT", continuation)
		t.Fatalf("ASSERT_DIRECT_PAUSED_V2: %v", d.env)
	}
	result, ok := d.env["result"].(map[string]any)
	if !ok {
		t.Fatalf("ASSERT_CENSUS_RESULT: %v", d.env)
	}
	catalog, ok := result["catalog"].(map[string]any)
	if !ok || catalog["kind"] != "ADR_0007_FEATURE_CATALOG" || catalog["status"] != "PAUSED" || catalog["authority"] != float64(0) || catalog["accepted"] != false || catalog["completeness"] != "UNKNOWN" {
		t.Fatalf("ASSERT_CATALOG_BOUNDARY: %v", result)
	}
	if catalog["request_count"].(float64) < 1 {
		t.Fatalf("ASSERT_REQUEST_COUNT: %v", catalog)
	}
	if _, ok := catalog["checkpoint_selector"].(string); !ok {
		t.Fatalf("ASSERT_CHECKPOINT_SELECTOR: %v", catalog)
	}
	traceBytes, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	traceText := string(traceBytes)
	for _, marker := range []string{"textDocument/documentSymbol", "textDocument/prepareCallHierarchy", "callHierarchy/outgoingCalls"} {
		if !strings.Contains(traceText, marker) {
			t.Fatalf("ASSERT_TRACE_METHOD: missing %q trace=%q", marker, traceText)
		}
	}
	if got := strings.Count(traceText, "textDocument/documentSymbol"); got < 2 {
		t.Fatalf("ASSERT_PROCESS_FIXTURE_CONSTITUENTS: documentSymbol requests=%d trace=%q", got, traceText)
	}
	if got := strings.Count(traceText, "callHierarchy/outgoingCalls"); got < 2 {
		t.Fatalf("ASSERT_PROCESS_BOTH_OUTGOING_REQUESTS: outgoingCalls requests=%d trace=%q", got, traceText)
	}
	if strings.Contains(traceText, "$/cancelRequest") {
		t.Fatalf("ASSERT_NO_SUCCESS_CANCELLATION: %q", traceText)
	}
	postCall := decodeProcessCall(t, post)
	if postCall.env["operation_status"] != "SUCCEEDED" {
		t.Fatalf("ASSERT_POST_SESSION_LIST: %v", postCall.env)
	}
}

func logProcessE2ERoot(t *testing.T, label, root string) {
	t.Helper()
	info, err := os.Stat(root)
	if err != nil {
		t.Logf("%s path=%q stat_error=%v", label, root, err)
		return
	}
	t.Logf("%s path=%q mode=%s", label, root, info.Mode())
	if info.IsDir() {
		_ = filepath.Walk(root, func(path string, entry os.FileInfo, walkErr error) error {
			if walkErr != nil {
				t.Logf("%s entry=%q error=%v", label, path, walkErr)
				return nil
			}
			t.Logf("%s entry=%q mode=%s size=%d", label, path, entry.Mode(), entry.Size())
			return nil
		})
		return
	}
	if b, err := os.ReadFile(root); err == nil {
		t.Logf("%s content=%q", label, b)
	}
}

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
	runtimeTrace := os.Getenv(runtimeTracePathEnv)
	if runtimeTrace == "" {
		runtimeTrace = filepath.Join(diagnosticParent, "runtime.trace")
	}
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
	cmd.Env = append(os.Environ(), runtimeTracePathEnv+"="+runtimeTrace)
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
	response := send(callRequest(2, "lsp_trace_v1_census", args))
	_ = in.Close()
	waitErr := cmd.Wait()
	if waitErr != nil && ctx.Err() == nil {
		t.Fatalf("ASSERT_PROCESS_CLEANUP: %v stderr=%s", waitErr, stderr.String())
	}
	call := decodeProcessCall(t, response)
	failure, ok := call.env["error"].(map[string]any)
	if call.env["tool"] != "lsp_trace_v1_census" || call.env["outcome"] != "DOMAIN_ERROR" || call.env["operation_status"] != "FAILED" || call.env["isError"] != true || !ok || failure["code"] != "ACQUISITION_FAILED" {
		t.Fatalf("ASSERT_PUBLIC_GENERIC_ACQUISITION_FAILED: %v", call.env)
	}
	runtimeTraceBytes, err := os.ReadFile(runtimeTrace)
	if err != nil {
		t.Fatalf("ASSERT_PRIVATE_RUNTIME_TRACE_READ: %v", err)
	}
	runtimeTraceInfo, err := os.Stat(runtimeTrace)
	if err != nil || runtimeTraceInfo.Mode().Perm() != 0600 {
		t.Fatalf("ASSERT_PRIVATE_RUNTIME_TRACE_MODE: info=%v err=%v", runtimeTraceInfo, err)
	}
	var branchTags []string
	for remaining := string(runtimeTraceBytes); ; {
		const marker = "CENSUS_ACQUISITION_BRANCH tag="
		index := strings.Index(remaining, marker)
		if index < 0 {
			break
		}
		remaining = remaining[index+len(marker):]
		end := strings.IndexAny(remaining, "\r\n\x00")
		if end < 0 {
			end = len(remaining)
		}
		tag := remaining[:end]
		if !validCensusAcquisitionBranchTag(tag) {
			t.Fatalf("ASSERT_PRIVATE_RUNTIME_TRACE_FINITE_TAG: %q", tag)
		}
		branchTags = append(branchTags, tag)
		remaining = remaining[end:]
	}
	if len(branchTags) < 2 || branchTags[0] != "BEFORE_ACQUIRE" {
		t.Fatalf("ASSERT_PRIVATE_RUNTIME_TRACE_BEFORE_ACQUIRE: %v", branchTags)
	}
	var coreErrorTags []string
	for _, tag := range branchTags[1:] {
		if strings.HasPrefix(tag, "CORE_ERROR_") {
			coreErrorTags = append(coreErrorTags, tag)
		}
	}
	if len(coreErrorTags) != 1 {
		t.Fatalf("ASSERT_EXACTLY_ONE_CORE_ERROR_PHASE: all=%v core=%v", branchTags, coreErrorTags)
	}
	var batchResultTags []string
	for _, tag := range branchTags {
		if tag == "BATCH_RESULT_INCOMPLETE" || tag == "BATCH_RESULT_IDENTITY_DRIFT" {
			batchResultTags = append(batchResultTags, tag)
		}
	}
	if len(batchResultTags) != 1 {
		t.Fatalf("ASSERT_EXACTLY_ONE_BATCH_RESULT_PREDICATE: all=%v batch_result=%v", branchTags, batchResultTags)
	}
	t.Logf("PRIVATE_BRANCH_TAG_SEQUENCE=%s", strings.Join(branchTags, ","))
	publicEnvelope, err := json.Marshal(call.env)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(publicEnvelope), workspace) || strings.Contains(string(publicEnvelope), diagnostic) || strings.Contains(string(publicEnvelope), "hang-outgoing") {
		t.Fatalf("ASSERT_PUBLIC_NO_PRIVATE_LEAK: %s", publicEnvelope)
	}
	t.Logf("RED_EVIDENCE envelope=%s stderr=%q", publicEnvelope, stderr.String())
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
	if record.Fingerprint == "" || record.Ordinal == nil || *record.Ordinal != 0 || record.Category != "ACQUISITION" || record.OperationCode != "CENSUS_BATCH_INCOMPLETE" || record.OperationCategory != "ACQUISITION" {
		t.Fatalf("ASSERT_TRUTHFUL_PRIVATE_RECORD: %+v", record)
	}
	replayed, err := censusdiagnostic.ReadLast(diagnostic)
	if err != nil {
		t.Fatalf("ASSERT_REPLAYABLE_PRIVATE_RECORD: %v record=%+v", err, record)
	}
	if replayed.Fingerprint != record.Fingerprint || replayed.Category != record.Category || replayed.Ordinal == nil || *replayed.Ordinal != *record.Ordinal || replayed.OperationCode != record.OperationCode || replayed.OperationCategory != record.OperationCategory {
		t.Fatalf("ASSERT_REPLAYED_RECORD_MATCHES: decoded=%+v replayed=%+v", record, replayed)
	}
	if stderr.Len() > 4096 {
		t.Fatalf("ASSERT_BOUNDED_STDERR: %d", stderr.Len())
	}
}
