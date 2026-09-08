package main

import (
	"bytes"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// Qualification compares executable baseline bytes, not reconstructed expectations.
func TestFR20FrozenV1ProcessParity(t *testing.T) {
	oldCLI, oldMCP := os.Getenv("LSP_TRACE_FR20_BASELINE_CLI"), os.Getenv("LSP_TRACE_FR20_BASELINE_MCP")
	if oldCLI == "" || oldMCP == "" {
		t.Skip("set exact f9981ba baseline binaries for frozen-byte qualification")
	}
	if runtime.GOOS != "darwin" {
		t.Skip("managed fixture requires Darwin")
	}
	cli := buildBinary(t, "lsp-trace", "./cmd/lsp-trace")
	mcp := buildMCPBinary(t)
	fake := buildBinary(t, "fake-lsp", "./cmd/fake-lsp")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("leaf\n\ncaller\n"), 0600); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.Join(root, "main.go")}).String()
	for _, mode := range []string{"slice", "incoming"} {
		t.Run(mode, func(t *testing.T) {
			args := []string{mode, "--workspace", root, "--server", fake, "--at", "main.go:1:1", "--language-id", "go"}
			if mode == "slice" {
				args = append(args, "--down-depth", "1", "--up-depth", "1")
			} else {
				args = append(args, "--max-depth", "1")
			}
			run := func(binary string, args []string) any {
				cmd := exec.Command(binary, args...)
				var out, errout bytes.Buffer
				cmd.Stdout = &out
				cmd.Stderr = &errout
				err := cmd.Run()
				code := 0
				if err != nil {
					exit, ok := err.(*exec.ExitError)
					if !ok {
						t.Fatal(err)
					}
					code = exit.ExitCode()
				}
				if code != 0 && code != 2 {
					t.Fatalf("baseline fixture failed: %d %s", code, errout.String())
				}
				return struct {
					Code     int
					Out, Err string
				}{code, out.String(), errout.String()}
			}
			before := run(oldCLI, args)
			after := run(cli, args)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("ASSERT_V1_FROZEN_CLI_BYTES")
			}
			explicit := append(append([]string{}, args...), "--acquisition-version", "v1")
			if !reflect.DeepEqual(before, run(cli, explicit)) {
				t.Fatal("ASSERT_EXPLICIT_V1_BYTES")
			}
			config := map[string]any{"version": 1, "processes": []any{map[string]any{"alias": "fixture", "language_id": "go", "profile": map[string]any{"trust_domain": "legacy-parity", "workspace": root, "profile": "cli", "environment_reference": "cli"}, "execution": map[string]any{"path": fake, "directory": root, "environment": os.Environ()}}}}
			input := map[string]any{"session_id": "fixture", "generation": 1, "uri": uri, "line": 0, "character": 0}
			if mode == "slice" {
				input["start_mode"] = "at"
			}
			request := []map[string]any{callRequest(1, "lsp_trace_v1_"+mode, input)}
			cfg := writeBootstrapJSON(t, config)
			old := runMCPProcess(t, oldMCP, []string{"--bootstrap-config", cfg}, request)
			now := runMCPProcess(t, mcp, []string{"--bootstrap-config", cfg}, request)
			if !bytes.Equal(inlineArtifactBytes(t, decodeProcessCall(t, old[0]).env), inlineArtifactBytes(t, decodeProcessCall(t, now[0]).env)) {
				t.Fatal("ASSERT_V1_FROZEN_MCP_BYTES")
			}
		})
	}
}
