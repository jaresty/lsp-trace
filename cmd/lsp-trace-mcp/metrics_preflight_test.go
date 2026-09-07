package main

import (
	"bytes"
	"context"
	"encoding/json"
	"lsp-trace/internal/boundedmetrics"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/retainedcalls"
	"lsp-trace/internal/verification"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMetricsPublicEncodedPreflight(t *testing.T) {
	raw, _ := boundedFixture(t)
	cli, mcp := buildBinary(t, "lsp-trace", "./cmd/lsp-trace"), buildMCPBinary(t)
	base, err := boundedmetrics.Analyze(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	deep := []byte(`{"x":` + strings.Repeat("[", 80) + `{"dup":0,"dup":1}` + strings.Repeat("]", 80) + `}`)
	for _, layer := range []string{"provenance", "graph"} {
		t.Run(layer, func(t *testing.T) {
			var r retainedcalls.Evidence
			_ = json.Unmarshal(raw, &r)
			if layer == "graph" {
				var p graphprovenance.Evidence
				_ = json.Unmarshal(r.InputBytes, &p)
				p.GraphBytes = deep
				r.InputBytes, _ = json.Marshal(p)
			} else {
				r.InputBytes = deep
			}
			input, _ := json.Marshal(r)
			var e boundedmetrics.Evidence
			_ = json.Unmarshal(base, &e)
			e.InputBytes = input
			artifact, _ := json.Marshal(e)
			dir := t.TempDir()
			write := func(name string, b []byte) string {
				t.Helper()
				p := filepath.Join(dir, name)
				if err := os.WriteFile(p, b, 0600); err != nil {
					t.Fatal(err)
				}
				return p
			}
			retainedPath, artifactPath := write("retained.json", input), write("metrics.json", artifact)
			if err := os.Mkdir(filepath.Join(dir, "generation"), 0700); err != nil {
				t.Fatal(err)
			}
			write("generation/artifact.json", artifact)
			receipt, err := verification.ReceiptBytes(artifact, verification.DirectoryDurabilityChecked)
			if err != nil {
				t.Fatal(err)
			}
			write("generation/receipt.json", receipt)
			selector := write("selected.json", []byte(`{"generation":"generation"}`))
			for _, args := range [][]string{{"bounded-retained-metrics", retainedPath}, {"validate", "--family", boundedmetrics.Family, "--version", "v1", artifactPath}, {"verify", "--family", boundedmetrics.Family, "--version", "v1", selector}} {
				out, err := exec.Command(cli, args...).CombinedOutput()
				if err == nil || !bytes.Contains(out, []byte("nesting LIMIT")) || bytes.Contains(out, []byte("duplicate")) {
					t.Fatalf("ASSERT_METRICS_PUBLIC_PREFLIGHT %v: %v %s", args, err, out)
				}
			}
			calls := runMCPProcess(t, mcp, nil, []map[string]any{
				callRequest(1, "lsp_trace_v1_bounded_retained_metrics", map[string]any{"input": string(input)}),
				callRequest(2, "lsp_trace_v1_validate", map[string]any{"input": string(artifact), "schema": map[string]any{"family": boundedmetrics.Family, "version": "v1"}}),
				callRequest(3, "lsp_trace_v1_bounded_retained_metrics", map[string]any{"input": json.RawMessage(input)}),
				callRequest(4, "lsp_trace_v1_validate", map[string]any{"input": json.RawMessage(artifact), "schema": map[string]any{"family": boundedmetrics.Family, "version": "v1"}}),
			})
			for _, call := range calls {
				env := decodeProcessCall(t, call).env
				b, _ := json.Marshal(env)
				if env["operation_status"] == "SUCCEEDED" || !bytes.Contains(b, []byte("nesting LIMIT")) || bytes.Contains(b, []byte("duplicate")) {
					t.Fatalf("ASSERT_METRICS_MCP_PREFLIGHT: %s", b)
				}
			}
		})
	}
}
