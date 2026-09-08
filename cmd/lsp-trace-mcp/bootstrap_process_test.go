package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestProductionBootstrapBlocksStdioUntilHostConfiguredProcessIsReady(t *testing.T) {
	const assertion = "ASSERT_PRODUCTION_BOOTSTRAP_HOST_AUTHORITY_CORRELATED_READY_THIRTEEN_TOOLS"
	t.Log("ASSERTION: " + assertion)

	mcpBinary := buildMCPBinary(t)
	fakeBinary := buildBinary(t, "fake-lsp", "./cmd/fake-lsp")
	workspace := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "bootstrap.json")
	startedMarker := filepath.Join(t.TempDir(), "fake-lsp-started")
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
				"path":        fakeBinary,
				"directory":   workspace,
				"environment": []string{"LSP_TRACE_FAKE_LSP_SCHEDULED=" + startedMarker},
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
	err = cmd.Run()
	if runtime.GOOS != "darwin" {
		if err == nil || !strings.Contains(stderr.String(), "PROCESS_CONTAINMENT_UNAVAILABLE") {
			t.Fatalf("%s: unsupported platform did not fail closed: err=%v stderr=%s", assertion, err, stderr.String())
		}
		if stdout.Len() != 0 {
			t.Fatalf("%s: unsupported platform emitted successful stdio: %q", assertion, stdout.String())
		}
		if _, markerErr := os.Stat(startedMarker); !os.IsNotExist(markerErr) {
			t.Fatalf("%s: unsupported platform started child: marker error=%v", assertion, markerErr)
		}
		t.Log("PASS ASSERT_PRODUCTION_BOOTSTRAP_UNSUPPORTED_PLATFORM_ZERO_EFFECTS")
		return
	}
	if err != nil {
		t.Fatalf("%s: process failed: %v stderr=%s", assertion, err, stderr.String())
	}
	if _, err := os.Stat(startedMarker); err != nil {
		t.Fatalf("%s: supported platform did not start configured child: %v", assertion, err)
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
	if len(response.Result.Tools) != 25 {
		t.Fatalf("%s: advertised=%d", assertion, len(response.Result.Tools))
	}
	if !bytes.Contains(lines[1], []byte(`"State":"READY"`)) || !bytes.Contains(lines[1], []byte(`"Generation":1`)) {
		t.Fatalf("%s: bootstrap session not discoverably READY: %s", assertion, lines[1])
	}
	t.Log("PASS " + assertion)
}
