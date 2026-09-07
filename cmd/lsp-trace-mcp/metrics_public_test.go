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

// Runs on hermetic and installed-gopls retained evidence, after source deletion.
func testMetricsRealOffline(t *testing.T, cli, mcp string, raw []byte) {
	t.Helper()
	dir := t.TempDir()
	input := filepath.Join(dir, "retained.json")
	if err := os.WriteFile(input, raw, 0600); err != nil {
		t.Fatal(err)
	}
	artifact := runCLIProcess(t, cli, "bounded-retained-metrics", input)
	if _, err := boundedmetrics.ValidateFor(artifact, boundedmetrics.Family, "v1"); err != nil {
		t.Fatal(err)
	}
	var e boundedmetrics.Evidence
	_ = json.Unmarshal(artifact, &e)
	var r retainedcalls.Evidence
	_ = json.Unmarshal(raw, &r)
	if e.NodeCount != len(r.Tables.Endpoints) || e.GroupCount != len(r.Tables.Groups) || e.ReportedOccurrenceCount != len(r.Tables.Occurrences) {
		t.Fatal("ASSERT_METRICS_RETAINED_TOTALS", e)
	}
	if e.NodeCount == 6 && e.GroupCount == 5 {
		in, out := 0, 0
		for _, n := range e.Nodes {
			in += n.InGroupDegree
			out += n.OutGroupDegree
		}
		if e.ReportedOccurrenceCount != 10 || in != 5 || out != 5 || e.DistinctNonloopPairCount != 5 || e.SelfLoopGroupCount != 0 || e.UnreportedGroupCount != 0 || e.Density.Value == nil || *e.Density.Value != (boundedmetrics.Rational{Numerator: 5, Denominator: 30}) {
			t.Fatal("ASSERT_GOPLS_METRICS_SIX_FIVE_TEN_DENSITY", e)
		}
		t.Log("PASS ASSERT_GOPLS_METRICS_SIX_FIVE_TEN_DENSITY: n=6 m=5 reported=10 sum_in=sum_out=5 q=5 density=5/30 loops=0 unreported=0; after source deletion")
	}
	file := filepath.Join(dir, "metrics.json")
	if err := os.WriteFile(file, artifact, 0600); err != nil {
		t.Fatal(err)
	}
	runCLIProcess(t, cli, "validate", "--family", boundedmetrics.Family, "--version", "v1", file)
	selector := filepath.Join(dir, "selector.json")
	runCLIProcess(t, cli, "bounded-retained-metrics", "--output", selector, input)
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
		t.Fatal("ASSERT_METRICS_CLI_PUBLICATION_BYTES", err)
	}
	runCLIProcess(t, cli, "verify", "--family", boundedmetrics.Family, "--version", "v1", selector)
	for _, args := range [][]string{{"verify", selector}, {"bounded-retained-metrics", "--output", selector, input}} {
		if _, err := exec.Command(cli, args...).CombinedOutput(); err == nil {
			t.Fatal("ASSERT_METRICS_DEFAULT_OR_OVERWRITE_REJECT", args)
		}
	}
	if err = os.WriteFile(published, append(b, ' '), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = exec.Command(cli, "verify", "--family", boundedmetrics.Family, "--version", "v1", selector).CombinedOutput(); err == nil {
		t.Fatal("ASSERT_METRICS_CUSTODY_TAMPER_REJECT")
	}
	root := t.TempDir()
	schema := runCLIProcess(t, cli, "schema", "get", "--family", boundedmetrics.Family, "--version", "v1")
	large := append(append([]byte{}, raw...), bytes.Repeat([]byte(" "), 800000)...)
	calls := runMCPProcess(t, mcp, []string{"--publication-root", root}, []map[string]any{
		callRequest(1, "lsp_trace_v1_bounded_retained_metrics", map[string]any{"input": string(raw)}),
		callRequest(2, "lsp_trace_bounded_retained_metrics", map[string]any{"input": string(raw)}),
		callRequest(3, "lsp_trace_v1_validate", map[string]any{"input": string(artifact), "schema": map[string]any{"family": boundedmetrics.Family, "version": "v1"}}),
		callRequest(4, "lsp_trace_v1_bounded_retained_metrics", map[string]any{"input": string(raw), "output_selector": "metrics.json"}),
		callRequest(5, "lsp_trace_v1_bounded_retained_metrics", map[string]any{"input": string(raw), "output_selector": "metrics.json"}),
		callRequest(6, "lsp_trace_v1_bounded_retained_metrics", map[string]any{"input": string(raw), "output_selector": "compact.json", "detail": "compact"}),
		callRequest(7, "lsp_trace_v1_schema_get", map[string]any{"schema": map[string]any{"family": boundedmetrics.Family, "version": "v1"}}),
		callRequest(8, "lsp_trace_v1_bounded_retained_metrics", map[string]any{"input": "{}"}),
		callRequest(9, "lsp_trace_v1_bounded_retained_metrics", map[string]any{"input": string(large)}),
		callRequest(10, "lsp_trace_v1_bounded_retained_metrics", map[string]any{"input": string(large), "output_selector": "large.json"}),
		callRequest(11, "lsp_trace_v1_bounded_retained_metrics", map[string]any{"input": string(raw), "detail": "compact"}),
		callRequest(12, "lsp_trace_v1_bounded_retained_metrics", map[string]any{"input": string(raw), "output_selector": "../escape.json"}),
	})
	for _, i := range []int{0, 1, 2} {
		if !bytes.Equal(inlineArtifactBytes(t, decodeProcessCall(t, calls[i]).env), artifact) {
			t.Fatal("ASSERT_METRICS_CLI_MCP_EXACT", i)
		}
	}
	if !bytes.Equal(inlineArtifactBytes(t, decodeProcessCall(t, calls[6]).env), schema) {
		t.Fatal("ASSERT_METRICS_SCHEMA_PARITY")
	}
	for i, code := range map[int]string{4: "PUBLICATION_FAILED", 7: "INPUT_INVALID", 8: "OUTPUT_REQUIRES_SELECTOR", 10: "OUTPUT_REQUIRES_SELECTOR", 11: "OUTPUT_SELECTOR_UNSAFE"} {
		if e := decodeProcessCall(t, calls[i]).env; e["code"] != code {
			t.Fatal("ASSERT_METRICS_PUBLIC_ERROR", i, e)
		}
	}
	if e := decodeProcessCall(t, calls[5]).env; e["summary"] == nil || e["content"] != nil {
		t.Fatal("ASSERT_METRICS_COMPACT", e)
	}
	for _, name := range []string{"metrics.json", "compact.json", "large.json"} {
		b, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = boundedmetrics.ValidateFor(b, boundedmetrics.Family, "v1"); err != nil {
			t.Fatal(err)
		}
		if name != "large.json" && !bytes.Equal(b, artifact) {
			t.Fatal("ASSERT_METRICS_PUBLISHED_BYTES")
		}
		if name == "large.json" && len(b) <= 1048576 {
			t.Fatal("ASSERT_METRICS_OVERSIZED_FIXTURE")
		}
	}
}
func TestMetricsPublicCLIAndMCP(t *testing.T) {
	raw, _ := boundedFixture(t)
	testMetricsRealOffline(t, buildBinary(t, "lsp-trace", "./cmd/lsp-trace"), buildMCPBinary(t), raw)
}
func TestMetricsMCPObjectInputExactNumbers(t *testing.T) {
	raw, _ := boundedFixture(t)
	var retained retainedcalls.Evidence
	_ = json.Unmarshal(raw, &retained)
	var provenance graphprovenance.Evidence
	_ = json.Unmarshal(retained.InputBytes, &provenance)
	provenance.Generation = 9007199254740993
	input, _ := json.Marshal(provenance)
	raw, err := retainedcalls.Export(input)
	if err != nil {
		t.Fatal(err)
	}
	var compact bytes.Buffer
	if err = json.Compact(&compact, raw); err != nil {
		t.Fatal(err)
	}
	raw = compact.Bytes()
	want, err := boundedmetrics.Analyze(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	calls := runMCPProcess(t, buildMCPBinary(t), nil, []map[string]any{
		callRequest(1, "lsp_trace_v1_bounded_retained_metrics", map[string]any{"input": json.RawMessage(raw)}),
		callRequest(2, "lsp_trace_v1_validate", map[string]any{"input": json.RawMessage(bytes.TrimSpace(want)), "schema": map[string]any{"family": boundedmetrics.Family, "version": "v1"}}),
	})
	if !bytes.Equal(inlineArtifactBytes(t, decodeProcessCall(t, calls[0]).env), want) {
		t.Fatal("ASSERT_METRICS_RAW_LARGE_NUMBER")
	}
	if !bytes.Equal(inlineArtifactBytes(t, decodeProcessCall(t, calls[1]).env), bytes.TrimSpace(want)) {
		t.Fatal("ASSERT_METRICS_VALIDATE_RAW_BYTES")
	}
}
func TestMetricsOpaqueSourceContent(t *testing.T) {
	raw, _ := boundedFixtureContent(t, []byte(strings.Repeat("[", 80)+`{"dup":0,"dup":1}`+strings.Repeat("]", 80)))
	testMetricsRealOffline(t, buildBinary(t, "lsp-trace", "./cmd/lsp-trace"), buildMCPBinary(t), raw)
}
