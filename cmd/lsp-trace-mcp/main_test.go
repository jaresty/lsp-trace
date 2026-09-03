package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
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
