package main

import (
	"bytes"
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

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

func TestAlwaysLocalIncomingManagedFakeLSPEndToEnd(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("LOCAL_DARWIN_SUPERVISION_ONLY")
	}
	server, manager, err := newServerRuntime(false)
	if err != nil {
		t.Fatal(err)
	}
	fake := buildBinary(t, "fake-lsp", "./cmd/fake-lsp")
	validated, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "incoming-e2e", Workspace: t.TempDir(), Profile: "fake-lsp", EnvironmentReference: "hermetic"})
	if err != nil {
		t.Fatal(err)
	}
	started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(validated), Process: managedprocess.Spec{Path: fake, Dir: t.TempDir()}})
	pending := manager.BeginReadiness(context.Background(), started.SessionID, started.Generation, time.Now().Add(5*time.Second))
	ready, found := manager.WaitReadiness(context.Background(), pending.ID)
	if !found || ready.State != sessionruntime.ReadinessReady || !ready.Metadata.CallHierarchySupport || ready.Metadata.PositionEncoding != "utf-16" {
		t.Fatalf("ASSERT_INCOMING_RETAINED_INITIALIZE_EVIDENCE: start=%+v ready=%+v", started, ready)
	}
	var stdout bytes.Buffer
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"lsp_trace_v1_incoming","arguments":{"session_id":"` + started.SessionID + `","generation":1,"uri":"file:///fixture/main.go","line":0,"character":0,"max_depth":4,"max_nodes":20,"timeout_ms":5000,"request_timeout_ms":1000}}}` + "\n"
	if err := server.Serve(strings.NewReader(input), &stdout); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("ASSERT_INCOMING_MCP_STDIO_CALLABLE: %q", stdout.String())
	}
	var listed struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &listed); err != nil || len(listed.Result.Tools) != 11 {
		t.Fatalf("ASSERT_ALWAYS_LOCAL_ELEVEN_TOOL_ORDER: response=%s err=%v", lines[0], err)
	}
	for i := 1; i < len(listed.Result.Tools); i++ {
		if listed.Result.Tools[i-1].Name > listed.Result.Tools[i].Name {
			t.Fatalf("ASSERT_ALWAYS_LOCAL_ELEVEN_TOOL_ORDER: tools=%v", listed.Result.Tools)
		}
	}
	if !strings.Contains(lines[1], `"tool":"lsp_trace_v1_incoming"`) || !strings.Contains(lines[1], `\"traversal_complete\":true`) || !strings.Contains(lines[1], `\"edge_count\":1`) {
		t.Fatalf("ASSERT_INCOMING_MCP_STDIO_CALLABLE: %s", lines[1])
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
