package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRunStdioOnly(t *testing.T) {
	const assertion = "binary serves newline-delimited MCP on stdio and accepts only the enable-live-lsp startup flag"
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
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"--listen", ":0"}, strings.NewReader(""), &stdout, &stderr); code == 0 {
		t.Errorf("%s: network-style flag was accepted", assertion)
	}
}
