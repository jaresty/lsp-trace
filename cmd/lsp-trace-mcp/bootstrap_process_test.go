package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionBootstrapBlocksStdioUntilHostConfiguredProcessIsReady(t *testing.T) {
	const assertion = "ASSERT_PRODUCTION_BOOTSTRAP_HOST_AUTHORITY_CORRELATED_READY_TWELVE_TOOLS"
	t.Log("ASSERTION: " + assertion)

	mcpBinary := buildMCPBinary(t)
	fakeBinary := buildBinary(t, "fake-lsp", "./cmd/fake-lsp")
	workspace := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "bootstrap.json")
	config := map[string]any{
		"version": 1,
		"processes": []any{map[string]any{
			"profile": map[string]any{
				"trust_domain":          "production-bootstrap-test",
				"workspace":             workspace,
				"profile":               "fake-lsp",
				"environment_reference": "hermetic",
			},
			"execution": map[string]any{
				"path":      fakeBinary,
				"directory": workspace,
			},
		}},
	}
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, raw, 0600); err != nil {
		t.Fatal(err)
	}

	request := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"lsp_session_v1_list","arguments":{}}}` + "\n"
	cmd := exec.Command(mcpBinary, "--bootstrap-config", configPath)
	cmd.Stdin = strings.NewReader(request)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s: process failed: %v stderr=%s", assertion, err, stderr.String())
	}
	var response struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	lines := bytes.Split(bytes.TrimSpace(stdout.Bytes()), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("%s: responses=%d stdout=%q", assertion, len(lines), stdout.String())
	}
	if err := json.Unmarshal(lines[0], &response); err != nil {
		t.Fatalf("%s: invalid tools response: %v stdout=%q", assertion, err, stdout.String())
	}
	if len(response.Result.Tools) != 12 {
		t.Fatalf("%s: advertised=%d", assertion, len(response.Result.Tools))
	}
	if !bytes.Contains(lines[1], []byte(`"State":"READY"`)) || !bytes.Contains(lines[1], []byte(`"Generation":1`)) {
		t.Fatalf("%s: bootstrap session not discoverably READY: %s", assertion, lines[1])
	}
	t.Log("PASS " + assertion)
}
