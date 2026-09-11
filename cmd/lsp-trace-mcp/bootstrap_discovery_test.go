package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicBootstrapExampleParsesProductionContract(t *testing.T) {
	var template bootstrapConfig
	if err := json.Unmarshal(publicBootstrapExample, &template); err != nil {
		t.Fatalf("ASSERT_PUBLIC_BOOTSTRAP_EXAMPLE_JSON: %v", err)
	}
	path := filepath.Join(t.TempDir(), "bootstrap.json")
	if err := os.WriteFile(path, publicBootstrapExample, 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := loadBootstrapConfig(path)
	if err != nil {
		t.Fatalf("ASSERT_PUBLIC_BOOTSTRAP_EXAMPLE_PRODUCTION_PARSE: %v", err)
	}
	if len(config.Processes) != 2 {
		t.Fatalf("ASSERT_PUBLIC_BOOTSTRAP_EXAMPLE_PROCESS_COUNT: %d", len(config.Processes))
	}
	csharp := config.Processes[1]
	want := []string{"--features", "razor-support", "--solution", "Project.sln"}
	if strings.Join(csharp.Execution.Arguments, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("ASSERT_PUBLIC_BOOTSTRAP_EXAMPLE_ARGUMENT_ORDER: %q", csharp.Execution.Arguments)
	}
	for i, process := range config.Processes {
		if len(process.Execution.Environment) != 0 {
			t.Fatalf("ASSERT_PUBLIC_BOOTSTRAP_EXAMPLE_NO_ENVIRONMENT_VALUES[%d]: %q", i, process.Execution.Environment)
		}
		templateProcess := template.Processes[i]
		for field, value := range map[string]string{
			"profile.workspace":   templateProcess.Profile.Workspace,
			"execution.path":      templateProcess.Execution.Path,
			"execution.directory": templateProcess.Execution.Directory,
		} {
			if filepath.IsAbs(value) || filepath.VolumeName(value) != "" || strings.Contains(value, "\\") || (len(value) >= 2 && value[1] == ':') {
				t.Fatalf("ASSERT_PUBLIC_BOOTSTRAP_EXAMPLE_PORTABLE_RELATIVE_PATH[%d/%s]: %q", i, field, value)
			}
		}
	}
}

func TestPrintBootstrapExampleExactBytesAndNoStartup(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--print-bootstrap-example"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("ASSERT_PRINT_BOOTSTRAP_EXAMPLE_SUCCEEDS: code=%d stderr=%q", code, stderr.String())
	}
	if !bytes.Equal(stdout.Bytes(), publicBootstrapExample) {
		t.Fatalf("ASSERT_PRINT_BOOTSTRAP_EXAMPLE_EXACT_EMBEDDED_BYTES")
	}
	if stderr.Len() != 0 {
		t.Fatalf("ASSERT_PRINT_BOOTSTRAP_EXAMPLE_NO_STARTUP_DIAGNOSTICS: %q", stderr.String())
	}
}

func TestPrintBootstrapExampleRejectsOperationalOptions(t *testing.T) {
	for _, args := range [][]string{
		{"--print-bootstrap-example", "--enable-live-lsp"},
		{"--print-bootstrap-example", "--bootstrap-config", "/tmp/bootstrap.json"},
		{"--print-bootstrap-example", "--publication-root", "/tmp/publication"},
		{"--print-bootstrap-example", "--tool-profile", "compact"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(args, strings.NewReader(""), &stdout, &stderr); code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "cannot be combined with operational options") {
			t.Fatalf("ASSERT_PRINT_BOOTSTRAP_EXAMPLE_EXCLUSIVE[%v]: code=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
		}
	}
}

func TestNoBootstrapGuidanceIsStderrOnlyAndPublicSafe(t *testing.T) {
	var stdout, stderr bytes.Buffer
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lsp_session_v1_list","arguments":{}}}` + "\n"
	if code := run([]string{"--tool-profile", "compact"}, strings.NewReader(input), &stdout, &stderr); code != 0 {
		t.Fatalf("ASSERT_NO_BOOTSTRAP_OFFLINE_OPERATION_CONTINUES: code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "No managed LSP sessions are provisioned. Configure --bootstrap-config; use --print-bootstrap-example for the public template.") {
		t.Fatalf("ASSERT_NO_BOOTSTRAP_ACTIONABLE_GUIDANCE: %q", stderr.String())
	}
	if strings.Contains(stdout.String(), "No managed LSP sessions") || !strings.Contains(stdout.String(), `"operation_status":"SUCCEEDED"`) {
		t.Fatalf("ASSERT_NO_BOOTSTRAP_GUIDANCE_NOT_MCP_STDOUT: %q", stdout.String())
	}
}
