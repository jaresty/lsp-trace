package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"lsp-trace/incomingops"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/mcp"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
	"lsp-trace/sliceops"
)

func TestHostSelectorCompositionIncludesLifecycle(t *testing.T) {
	server, manager, err := newServerRuntime(false)
	if err != nil {
		t.Fatal(err)
	}
	beforeLifecycle := server.Executors[mcp.LifecycleExecutorFamily]
	beforeIncoming := server.Executors[mcp.IncomingExecutorFamily]
	beforeSlice := server.Executors[mcp.SliceExecutorFamily]

	selected := composeHostSelectorRuntime(server, manager, []bootstrapSession{{Alias: "project", SessionID: "canonical"}})
	if selected.aliases["project"] != "canonical" || server.Executors[mcp.LifecycleExecutorFamily] == beforeLifecycle || server.Executors[mcp.IncomingExecutorFamily] == beforeIncoming || server.Executors[mcp.SliceExecutorFamily] == beforeSlice {
		t.Fatalf("ASSERT_HOST_SELECTOR_COMPOSES_LIFECYCLE_WITHOUT_AUTHORITY_CHANGE: aliases=%v lifecycle_replaced=%v incoming_replaced=%v slice_replaced=%v", selected.aliases, server.Executors[mcp.LifecycleExecutorFamily] != beforeLifecycle, server.Executors[mcp.IncomingExecutorFamily] != beforeIncoming, server.Executors[mcp.SliceExecutorFamily] != beforeSlice)
	}
	if got := server.Registry.Tools(); len(got) != 28 {
		t.Fatalf("ASSERT_HOST_SELECTOR_COMPOSES_LIFECYCLE_WITHOUT_AUTHORITY_CHANGE: tool_count=%d", len(got))
	}
	_, incomingCollector := any(selected).(incomingops.RelationCollector)
	_, sliceCollector := any(selected).(sliceops.RelationCollector)
	if !incomingCollector || !sliceCollector {
		t.Fatalf("ASSERT_PRODUCTION_INCOMING_RELATION_COLLECTOR=%v ASSERT_PRODUCTION_SLICE_RELATION_COLLECTOR=%v", incomingCollector, sliceCollector)
	}
	t.Log("PASS ASSERT_HOST_SELECTOR_COMPOSES_LIFECYCLE_WITHOUT_AUTHORITY_CHANGE")
}

func TestAlwaysLocalLifecycleListDispatch(t *testing.T) {
	const assertion = "ASSERT_ALWAYS_LOCAL_LIFECYCLE_LIST_DISPATCH"
	server, err := newServer(false)
	if err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lsp_session_v1_list","arguments":{}}}` + "\n"
	if err := server.Serve(strings.NewReader(input), &stdout); err != nil {
		t.Fatal(err)
	}
	var response struct {
		Result struct {
			IsError    bool           `json:"isError"`
			Structured map[string]any `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &response); err != nil {
		t.Fatal(err)
	}
	if response.Result.IsError || response.Result.Structured["outcome"] != "COMPLETE" || response.Result.Structured["operation_status"] != "SUCCEEDED" {
		t.Fatalf("%s: response=%s", assertion, stdout.String())
	}
}

func TestAlwaysLocalTraversalManagedFakeLSPEndToEnd(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("LOCAL_DARWIN_SUPERVISION_ONLY")
	}
	server, manager, err := newServerRuntime(false)
	if err != nil {
		t.Fatal(err)
	}
	fake := buildBinary(t, "fake-lsp", "./cmd/fake-lsp")
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	uri := "file://" + filepath.ToSlash(filepath.Join(workspace, "main.go"))
	validated, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "incoming-e2e", Workspace: workspace, Profile: "fake-lsp", EnvironmentReference: "hermetic"})
	if err != nil {
		t.Fatal(err)
	}
	started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(validated), Process: managedprocess.Spec{Path: fake, Dir: workspace}})
	pending := manager.BeginReadiness(context.Background(), started.SessionID, started.Generation, time.Now().Add(5*time.Second))
	ready, found := manager.WaitReadiness(context.Background(), pending.ID)
	if !found || ready.State != sessionruntime.ReadinessReady || !ready.Metadata.CallHierarchySupport || ready.Metadata.PositionEncoding != "utf-16" {
		t.Fatalf("ASSERT_INCOMING_RETAINED_INITIALIZE_EVIDENCE: start=%+v ready=%+v", started, ready)
	}
	var stdout bytes.Buffer
	sliceArgs := `{"session_id":"` + started.SessionID + `","generation":1,"start_mode":"at","uri":"` + uri + `","line":0,"character":0,"down_depth":1,"up_depth":2,"max_nodes":20,"max_messages":64,"max_bytes":4194304,"timeout_ms":5000,"request_timeout_ms":1000}`
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"lsp_trace_v1_slice","arguments":` + sliceArgs + `}}` + "\n" +
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"lsp_trace_slice","arguments":` + sliceArgs + `}}` + "\n"
	if err := server.Serve(strings.NewReader(input), &stdout); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("ASSERT_INCOMING_MCP_STDIO_CALLABLE: %q", stdout.String())
	}
	var listed struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &listed); err != nil || len(listed.Result.Tools) != 28 {
		t.Fatalf("ASSERT_ALWAYS_LOCAL_THIRTEEN_TOOL_ORDER: response=%s err=%v", lines[0], err)
	}
	for i := 1; i < len(listed.Result.Tools); i++ {
		if listed.Result.Tools[i-1].Name > listed.Result.Tools[i].Name {
			t.Fatalf("ASSERT_ALWAYS_LOCAL_THIRTEEN_TOOL_ORDER: tools=%v", listed.Result.Tools)
		}
	}
	for _, line := range lines[1:] {
		if !strings.Contains(line, `"tool":"lsp_trace_v1_slice"`) || !strings.Contains(line, `\"upward_start_node_ids\"`) || !strings.Contains(line, `\"traversal_complete\":true`) {
			t.Fatalf("ASSERT_SLICE_MCP_STDIO_CANONICAL_ALIAS_CALLABLE: %s", line)
		}
	}
}

func TestCompactToolProfileProcessAdvertisementAndHiddenDispatch(t *testing.T) {
	var stdout, stderr bytes.Buffer
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"lsp_trace_v1_validate","arguments":{"input":"{}"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"lsp_trace_v1_execute","arguments":{"request":{"tool":"lsp_trace_v1_validate","arguments":{"input":"{}"}}}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"lsp_trace_v1_capabilities","arguments":{"operation":"lsp_trace_v3_slice"}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"lsp_trace_v1_capabilities","arguments":{"operation":"lsp_trace_v9_missing"}}}`,
	}, "\n") + "\n"
	if code := run([]string{"--tool-profile", "compact"}, strings.NewReader(input), &stdout, &stderr); code != 0 {
		t.Fatalf("ASSERT_COMPACT_PROCESS_RUNS: code=%d stderr=%s", code, stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 5 {
		t.Fatalf("ASSERT_COMPACT_PROCESS_RESPONSES: %q", stdout.String())
	}
	var listed struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &listed); err != nil || len(listed.Result.Tools) != 10 {
		t.Fatalf("ASSERT_COMPACT_PROCESS_ADVERTISES_10: err=%v response=%s", err, lines[0])
	}
	for i := 1; i < len(listed.Result.Tools); i++ {
		if listed.Result.Tools[i-1].Name > listed.Result.Tools[i].Name {
			t.Fatalf("ASSERT_COMPACT_PROCESS_LEXICAL: %+v", listed.Result.Tools)
		}
	}
	if strings.Contains(lines[1], "Unknown tool") || !strings.Contains(lines[1], `"tool":"lsp_trace_v1_validate"`) {
		t.Fatalf("ASSERT_COMPACT_HIDDEN_DIRECT_CACHED_CALL: %s", lines[1])
	}
	if strings.Contains(lines[2], "unknown canonical tool") || !strings.Contains(lines[2], `"requested_tool":"lsp_trace_v1_validate"`) {
		t.Fatalf("ASSERT_COMPACT_HIDDEN_EXECUTE_GATEWAY_CALL: %s", lines[2])
	}
	if !strings.Contains(lines[3], `"name":"lsp_trace_v3_slice"`) || !strings.Contains(lines[3], `"advertised":false`) || !strings.Contains(lines[3], `"graph_v5_production"`) || !strings.Contains(lines[3], `"request.arguments.output_selector"`) {
		t.Fatalf("ASSERT_COMPACT_HIDDEN_OPERATION_DESCRIPTION: %s", lines[3])
	}
	if !strings.Contains(lines[4], `"code":"INPUT_INVALID"`) || !strings.Contains(lines[4], `unknown canonical operation`) {
		t.Fatalf("ASSERT_COMPACT_UNKNOWN_OPERATION_DESCRIPTION: %s", lines[4])
	}
}

func TestToolProfileCLIRejectsUnknownAndDuplicate(t *testing.T) {
	for _, args := range [][]string{{"--tool-profile", "other"}, {"--tool-profile", "compact", "--tool-profile=full"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, strings.NewReader(""), &stdout, &stderr); code != 2 {
			t.Fatalf("ASSERT_TOOL_PROFILE_CLI_STRICT[%v]: code=%d stderr=%s", args, code, stderr.String())
		}
	}
}

func TestRunStdioOnly(t *testing.T) {
	const assertion = "binary serves newline-delimited MCP on stdio with a conspicuous trusted local process warning"
	t.Log("ASSERTION: " + assertion)
	var stdout, stderr bytes.Buffer
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}` + "\n"
	if code := run(nil, strings.NewReader(input), &stdout, &stderr); code != 0 {
		t.Fatalf("%s: code=%d stderr=%s", assertion, code, stderr.String())
	}
	var response map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &response); err != nil || response["result"] == nil {
		t.Errorf("%s: stdout=%q error=%v", assertion, stdout.String(), err)
	}
	warning := stderr.String()
	for _, phrase := range []string{"developer's permissions", "not sandboxed", "local files", "network", "trusted"} {
		if !strings.Contains(warning, phrase) {
			t.Errorf("ASSERT_ALWAYS_LOCAL_TRUST_WARNING: missing %q in %q", phrase, warning)
		}
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"--listen", ":0"}, strings.NewReader(""), &stdout, &stderr); code == 0 {
		t.Errorf("%s: network-style flag was accepted", assertion)
	}
}
