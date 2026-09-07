package main

import (
	"bytes"
	"encoding/json"
	"lsp-trace/internal/boundedanalysis"
	"lsp-trace/internal/retainedcalls"
	"lsp-trace/internal/verification"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Called both with the hermetic topology and the installed-gopls six-file
// five-group ten-occurrence specimen, only after source deletion.
func testBoundedRealOffline(t *testing.T, cli, mcp string, raw []byte) {
	t.Helper()
	var retained retainedcalls.Evidence
	if err := json.Unmarshal(raw, &retained); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	input := filepath.Join(dir, "retained.json")
	if err := os.WriteFile(input, raw, 0600); err != nil {
		t.Fatal(err)
	}
	g := retained.Tables.Groups[0]
	modes := []struct {
		name   string
		args   []string
		params map[string]any
	}{
		{"project", []string{"--operation", "PROJECT"}, map[string]any{"operation": "PROJECT"}},
		{"path", []string{"--operation", "PATH", "--start", g.CallerNodeID, "--end", g.CalleeNodeID}, map[string]any{"operation": "PATH", "start": g.CallerNodeID, "end": g.CalleeNodeID}},
		{"weak", []string{"--operation", "COMPONENTS", "--mode", "WEAK"}, map[string]any{"operation": "COMPONENTS", "mode": "WEAK"}},
		{"strong", []string{"--operation", "COMPONENTS", "--mode", "STRONG"}, map[string]any{"operation": "COMPONENTS", "mode": "STRONG"}},
	}
	for _, mode := range modes {
		t.Run("public-"+mode.name, func(t *testing.T) {
			args := append([]string{"bounded-retained-analysis"}, mode.args...)
			artifact := runCLIProcess(t, cli, append(args, input)...)
			if _, err := boundedanalysis.ValidateFor(artifact, boundedanalysis.Family, "v1"); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(dir, mode.name+".json")
			if err := os.WriteFile(file, artifact, 0600); err != nil {
				t.Fatal(err)
			}
			runCLIProcess(t, cli, "validate", "--family", boundedanalysis.Family, "--version", "v1", file)
			selector := filepath.Join(dir, mode.name+"-selector.json")
			runCLIProcess(t, cli, append(append(args, "--output", selector), input)...)
			selected, err := os.ReadFile(selector)
			if err != nil {
				t.Fatal(err)
			}
			s, err := verification.DecodeSelector(selected)
			if err != nil {
				t.Fatal(err)
			}
			published := filepath.Join(dir, s.Generation, "artifact.json")
			b, err := os.ReadFile(published)
			if err != nil || !bytes.Equal(b, artifact) {
				t.Fatal("ASSERT_BOUNDED_CLI_PUBLICATION_EXACT")
			}
			runCLIProcess(t, cli, "verify", "--family", boundedanalysis.Family, "--version", "v1", selector)
			if _, err = exec.Command(cli, "verify", selector).CombinedOutput(); err == nil {
				t.Fatal("ASSERT_BOUNDED_HISTORICAL_VERIFY_DEFAULT")
			}
			if _, err = exec.Command(cli, append(append(args, "--output", selector), input)...).CombinedOutput(); err == nil {
				t.Fatal("ASSERT_BOUNDED_CLI_NO_OVERWRITE")
			}
			if err = os.WriteFile(published, append(b, ' '), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err = exec.Command(cli, "verify", "--family", boundedanalysis.Family, "--version", "v1", selector).CombinedOutput(); err == nil {
				t.Fatal("ASSERT_BOUNDED_VERIFY_TAMPER")
			}
			params := mode.params
			params["input"] = string(raw)
			publishedParams := map[string]any{}
			for k, v := range params {
				publishedParams[k] = v
			}
			publishedParams["output_selector"] = mode.name + ".json"
			compact := map[string]any{}
			for k, v := range params {
				compact[k] = v
			}
			compact["output_selector"] = mode.name + "-compact.json"
			compact["detail"] = "compact"
			root := t.TempDir()
			calls := runMCPProcess(t, mcp, []string{"--publication-root", root}, []map[string]any{
				callRequest(1, "lsp_trace_v1_bounded_retained_analysis", params),
				callRequest(2, "lsp_trace_v1_validate", map[string]any{"input": string(artifact), "schema": map[string]any{"family": boundedanalysis.Family, "version": "v1"}}),
				callRequest(3, "lsp_trace_v1_bounded_retained_analysis", publishedParams),
				callRequest(4, "lsp_trace_v1_bounded_retained_analysis", publishedParams),
				callRequest(5, "lsp_trace_v1_bounded_retained_analysis", compact),
			})
			for _, i := range []int{0, 1, 2, 4} {
				if env := decodeProcessCall(t, calls[i]).env; env["operation_status"] != "SUCCEEDED" {
					t.Fatal("ASSERT_BOUNDED_MCP_SUCCESS", i, env)
				}
			}
			if !bytes.Equal(inlineArtifactBytes(t, decodeProcessCall(t, calls[0]).env), artifact) || !bytes.Equal(inlineArtifactBytes(t, decodeProcessCall(t, calls[1]).env), artifact) {
				t.Fatal("ASSERT_BOUNDED_CLI_MCP_EXACT_PARITY")
			}
			if env := decodeProcessCall(t, calls[3]).env; env["outcome"] != "PUBLICATION_ERROR" {
				t.Fatal("ASSERT_BOUNDED_MCP_NO_OVERWRITE", env)
			}
			if env := decodeProcessCall(t, calls[4]).env; env["summary"] == nil || env["content"] != nil {
				t.Fatal("ASSERT_BOUNDED_COMPACT", env)
			}
			b, err = os.ReadFile(filepath.Join(root, mode.name+".json"))
			if err != nil || !bytes.Equal(b, artifact) {
				t.Fatal("ASSERT_BOUNDED_MCP_PUBLICATION_EXACT", err)
			}
		})
	}
	schema := runCLIProcess(t, cli, "schema", "get", "--family", boundedanalysis.Family, "--version", "v1")
	root := t.TempDir()
	large := append(append([]byte{}, raw...), bytes.Repeat([]byte(" "), 800000)...)
	responses := runMCPProcess(t, mcp, []string{"--publication-root", root}, []map[string]any{
		callRequest(1, "lsp_trace_v1_schema_get", map[string]any{"schema": map[string]any{"family": boundedanalysis.Family, "version": "v1"}}),
		callRequest(2, "lsp_trace_v1_bounded_retained_analysis", map[string]any{"input": "{}", "operation": "PROJECT"}),
		callRequest(3, "lsp_trace_v1_bounded_retained_analysis", map[string]any{"input": string(large), "operation": "PROJECT"}),
		callRequest(4, "lsp_trace_v1_bounded_retained_analysis", map[string]any{"input": string(large), "operation": "PROJECT", "output_selector": "large.json"}),
	})
	if !bytes.Equal(inlineArtifactBytes(t, decodeProcessCall(t, responses[0]).env), schema) {
		t.Fatal("ASSERT_BOUNDED_SCHEMA_PARITY")
	}
	if env := decodeProcessCall(t, responses[1]).env; env["code"] != "INPUT_INVALID" {
		t.Fatal("ASSERT_BOUNDED_INPUT_ERROR", env)
	}
	if env := decodeProcessCall(t, responses[2]).env; env["code"] != "OUTPUT_REQUIRES_SELECTOR" {
		t.Fatal("ASSERT_BOUNDED_LARGE_REQUIRES_SELECTOR", env)
	}
	if env := decodeProcessCall(t, responses[3]).env; env["operation_status"] != "SUCCEEDED" {
		t.Fatal("ASSERT_BOUNDED_LARGE_PUBLICATION", env)
	}
	published, err := os.ReadFile(filepath.Join(root, "large.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(published) <= 1048576 {
		t.Fatal("ASSERT_BOUNDED_LARGE_FIXTURE")
	}
	if _, err = boundedanalysis.ValidateFor(published, boundedanalysis.Family, "v1"); err != nil {
		t.Fatal("ASSERT_BOUNDED_LARGE_SEMANTICS", err)
	}
}
