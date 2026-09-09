package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"lsp-trace/internal/normativeanalytics"
)

func runAnalyticsCLIError(t *testing.T, binary string, args ...string) string {
	t.Helper()
	cmd := exec.Command(binary, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil {
		t.Fatalf("ASSERT_PUBLIC_ANALYTICS_V2_CLI_FAILURE: args=%q", args)
	}
	if stderr.Len() == 0 {
		t.Fatalf("ASSERT_PUBLIC_ANALYTICS_V2_CLI_DIAGNOSTIC: args=%q", args)
	}
	return stderr.String()
}

func TestPublicAnalyticsV2ProcessErrorMatrix(t *testing.T) {
	mcpBinary := buildMCPBinary(t)
	cliBinary := buildBinary(t, "lsp-trace", "./cmd/lsp-trace")
	valid := []byte(`{"schema_version":"lsp-trace.normative-retained-graph.v1","build_revision":"r1","nodes":["a","b"],"edges":[]}`)
	duplicate := []byte(`{"schema_version":"lsp-trace.normative-retained-graph.v1","schema_version":"lsp-trace.normative-retained-graph.v1","build_revision":"r1","nodes":["a","b"],"edges":[]}`)
	unknown := []byte(`{"schema_version":"lsp-trace.normative-retained-graph.v1","build_revision":"r1","nodes":["a","b"],"edges":[],"unknown":true}`)
	oversized := bytes.Repeat([]byte("x"), normativeanalytics.MaxRetainedBytes+1)

	for _, tc := range []struct{ command, tool, operation, wrong string }{
		{"bounded-retained-analysis-v2", "lsp_trace_v2_bounded_retained_analysis", "ANALYSIS", "METRICS"},
		{"bounded-retained-metrics-v2", "lsp_trace_v2_bounded_retained_metrics", "METRICS", "RANKING"},
		{"bounded-retained-ranking-v2", "lsp_trace_v2_bounded_retained_ranking", "RANKING", "ANALYSIS"},
	} {
		t.Run(tc.operation, func(t *testing.T) {
			root := t.TempDir()
			write := func(name string, data []byte) string {
				path := filepath.Join(root, name)
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				return path
			}
			files := map[string]string{
				"malformed": write("malformed.json", []byte("{")),
				"duplicate": write("duplicate.json", duplicate),
				"unknown":   write("unknown.json", unknown),
				"oversized": write("oversized.json", oversized),
				"valid":     write("valid.json", valid),
			}
			for _, c := range []struct{ name, path string }{{"malformed", files["malformed"]}, {"duplicate", files["duplicate"]}, {"unknown", files["unknown"]}, {"oversized", files["oversized"]}} {
				runAnalyticsCLIError(t, cliBinary, tc.command, "--operation", tc.operation, "--filter", "CALLS", "--max-work", "10000", c.path)
				args := map[string]any{"input": string(mustReadFile(t, c.path)), "operation": tc.operation, "filter": []string{"CALLS"}, "max_work": 10000}
				env := decodeProcessCall(t, runMCPProcess(t, mcpBinary, nil, []map[string]any{callRequest(1, tc.tool, args)})[0]).env
				if env["operation_status"] != "FAILED" || env["code"] != "INPUT_INVALID" {
					t.Fatalf("ASSERT_PUBLIC_ANALYTICS_V2_ERROR_PARITY_%s: %v", c.name, env)
				}
			}

			runAnalyticsCLIError(t, cliBinary, tc.command, "--operation", tc.wrong, "--filter", "CALLS", "--max-work", "10000", files["valid"])
			wrong := map[string]any{"input": string(valid), "operation": tc.wrong, "filter": []string{"CALLS"}, "max_work": 10000}
			if env := decodeProcessCall(t, runMCPProcess(t, mcpBinary, nil, []map[string]any{callRequest(2, tc.tool, wrong)})[0]).env; env["operation_status"] != "FAILED" || env["code"] != "INPUT_INVALID" {
				t.Fatalf("ASSERT_PUBLIC_ANALYTICS_V2_WRONG_OPERATION_PARITY: %v", env)
			}

			runAnalyticsCLIError(t, cliBinary, tc.command, "--operation", tc.operation, "--filter", "CALLS", "--max-work", "10000", "--publication-root", root, "--publication-selector", "missing.json")
			conflict := map[string]any{"input": string(valid), "publication_selector": map[string]any{"selector": "missing.json", "artifact_schema_id": "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.normative-retained-graph.v1.schema.json", "artifact_digest": "sha256:" + string(bytes.Repeat([]byte("0"), 64)), "artifact_byte_length": len(valid)}, "operation": tc.operation, "filter": []string{"CALLS"}, "max_work": 10000}
			response := runMCPProcess(t, mcpBinary, []string{"--publication-root", root}, []map[string]any{callRequest(3, tc.tool, conflict)})[0]
			native, ok := response["error"].(map[string]any)
			if !ok || native["code"] != float64(-32602) {
				t.Fatalf("ASSERT_PUBLIC_ANALYTICS_V2_CARRIER_CONFLICT_NATIVE_PARITY: %v", response)
			}

			occupied := write("occupied.json", []byte("caller-owned"))
			runAnalyticsCLIError(t, cliBinary, tc.command, "--operation", tc.operation, "--filter", "CALLS", "--max-work", "10000", "--output", occupied, files["valid"])
			publish := map[string]any{"input": string(valid), "operation": tc.operation, "filter": []string{"CALLS"}, "max_work": 10000, "output_selector": "occupied.json"}
			if env := decodeProcessCall(t, runMCPProcess(t, mcpBinary, []string{"--publication-root", root}, []map[string]any{callRequest(4, tc.tool, publish)})[0]).env; env["operation_status"] != "FAILED" || env["code"] != "PUBLICATION_FAILED" {
				t.Fatalf("ASSERT_PUBLIC_ANALYTICS_V2_PUBLICATION_COLLISION_PARITY: %v", env)
			}
			if got := mustReadFile(t, occupied); !bytes.Equal(got, []byte("caller-owned")) {
				t.Fatal("ASSERT_PUBLIC_ANALYTICS_V2_NO_OVERWRITE")
			}
			unsafe := map[string]any{"input": string(valid), "operation": tc.operation, "filter": []string{"CALLS"}, "max_work": 10000, "output_selector": "../unsafe.json"}
			if env := decodeProcessCall(t, runMCPProcess(t, mcpBinary, []string{"--publication-root", root}, []map[string]any{callRequest(5, tc.tool, unsafe)})[0]).env; env["operation_status"] != "FAILED" || env["code"] != "OUTPUT_SELECTOR_UNSAFE" {
				t.Fatalf("ASSERT_PUBLIC_ANALYTICS_V2_UNSAFE_SELECTOR_PARITY: %v", env)
			}
		})
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
