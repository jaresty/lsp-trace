package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
)

func traceFakeArgs(workspace, scenario string) []string {
	return []string{"--workspace", workspace, "--server", os.Args[0], "--server-arg", "-test.run=^TestFakeLanguageServerProcess$", "--server-env", "LSP_TRACE_FAKE_SERVER=1", "--server-env", "LSP_TRACE_FAKE_SCENARIO=" + scenario, "--language-id", "go", "--request-timeout", "1s", "--timeout", "5s"}
}

func decodeTraceV5(t *testing.T, raw string) (graphprovenance.EvidenceV5, graph.Result) {
	t.Helper()
	var envelope graphprovenance.EvidenceV5
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatalf("decode envelope: %v: %s", err, raw)
	}
	native, err := base64.StdEncoding.DecodeString(envelope.GraphV5)
	if err != nil {
		t.Fatal(err)
	}
	var result graph.Result
	if err := json.Unmarshal(native, &result); err != nil {
		t.Fatal(err)
	}
	return envelope, result
}

func TestTraceProcessV5ParityAndNoSeedSpecCustody(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("managed local-process integration is Darwin-only")
	}
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\nfunc start() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	traceArgs := append(traceFakeArgs(workspace, "slice-symbol"), "--at", "main.go:1:1", "--down-depth", "0", "--up-depth", "0")
	traceOut, traceErr, traceCode := captureRun(t, append([]string{"trace"}, traceArgs...))
	cfg, err := parseTrace(traceArgs)
	if err != nil {
		t.Fatal(err)
	}
	seeds, _, err := traceSeeds(cfg)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := traceManifest(cfg, seeds)
	if err != nil {
		t.Fatal(err)
	}
	var sliceOut, sliceErr strings.Builder
	acquisitionArgs := traceFakeArgs(workspace, "slice-symbol")
	forward := append(acquisitionArgs[:12:12], "--output-version", graphprovenance.VersionV5)
	sliceCode := runTraceAcquisition("slice", "v3", forward, &sliceOut, &sliceErr, traceAcquisitionInput{manifest: manifest, seeds: seeds})
	if traceCode != 0 || sliceCode != 0 {
		t.Fatalf("ASSERT_TRACE_PROCESS_COMPLETES: traceCode=%d sliceCode=%d traceErr=%q sliceErr=%q traceOut=%q sliceOut=%q", traceCode, sliceCode, traceErr, sliceErr.String(), traceOut, sliceOut.String())
	}
	traceEnvelope, traceGraph := decodeTraceV5(t, traceOut)
	sliceEnvelope, sliceGraph := decodeTraceV5(t, sliceOut.String())
	if traceCode != sliceCode || traceEnvelope.SeedSpec != nil || sliceEnvelope.SeedSpec != nil || traceEnvelope.GraphV5 != sliceEnvelope.GraphV5 || traceEnvelope.GraphV5SHA256 != sliceEnvelope.GraphV5SHA256 || !reflect.DeepEqual(traceGraph.Edges, sliceGraph.Edges) {
		t.Fatalf("ASSERT_TRACE_PROCESS_V5_GRAPH_DIGEST_CALLS_PARITY_NO_SEED_SPEC: traceCode=%d sliceCode=%d traceErr=%q sliceErr=%q traceDigest=%s sliceDigest=%s traceSeed=%v sliceSeed=%v", traceCode, sliceCode, traceErr, sliceErr.String(), traceEnvelope.GraphV5SHA256, sliceEnvelope.GraphV5SHA256, traceEnvelope.SeedSpec, sliceEnvelope.SeedSpec)
	}
	if traceGraph.Invocation.Expansion.TopmostSiblings {
		t.Fatalf("ASSERT_TRACE_DEFAULT_SIBLINGS_OFF: %+v", traceGraph.Invocation.Expansion)
	}
}

func TestTraceProcessRepeatedAtUsesOneMultiTargetV5Acquisition(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("managed local-process integration is Darwin-only")
	}
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\nfunc first() {}\nfunc second() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	methodLog := filepath.Join(t.TempDir(), "methods.log")
	args := append(traceFakeArgs(workspace, "slice-symbol"), "--server-env", "LSP_TRACE_FAKE_METHOD_LOG="+methodLog, "--at", "main.go:1:1", "--at", "main.go:2:1", "--down-depth", "0", "--up-depth", "0")
	stdout, stderr, code := captureRun(t, append([]string{"trace"}, args...))
	envelope, _ := decodeTraceV5(t, stdout)
	native, err := base64.StdEncoding.DecodeString(envelope.GraphV5)
	if err != nil {
		t.Fatal(err)
	}
	var nativeShape struct {
		Seeds []json.RawMessage `json:"seeds"`
	}
	if err := json.Unmarshal(native, &nativeShape); err != nil {
		t.Fatal(err)
	}
	methods, err := os.ReadFile(methodLog)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 || len(nativeShape.Seeds) != 2 || methodCount(methods, "textDocument/prepareCallHierarchy") != 2 || methodCount(methods, "initialize") != 1 {
		t.Fatalf("ASSERT_TRACE_ONE_MULTI_TARGET_V5_ACQUISITION: code=%d stderr=%q seeds=%d methods=%q", code, stderr, len(nativeShape.Seeds), methods)
	}
}

func TestTraceProcessExactSymbolFailuresBeforePrepare(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("managed local-process integration is Darwin-only")
	}
	for _, tc := range []struct {
		scenario, symbol string
		total, omitted   int
	}{{"slice-trace-missing", "missing", 0, 0}, {"slice-trace-ambiguous", "duplicate", 10, 2}} {
		t.Run(tc.scenario, func(t *testing.T) {
			workspace := t.TempDir()
			if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			methodLog := filepath.Join(t.TempDir(), "methods.log")
			args := append(traceFakeArgs(workspace, tc.scenario), "--server-env", "LSP_TRACE_FAKE_METHOD_LOG="+methodLog, "--file", "main.go", "--symbol", tc.symbol)
			stdout, stderr, code := captureRun(t, append([]string{"trace"}, args...))
			methods, err := os.ReadFile(methodLog)
			if err != nil {
				t.Fatal(err)
			}
			if code != 1 || stdout != "" || strings.Contains(string(methods), "textDocument/prepareCallHierarchy") || strings.Contains(string(methods), "callHierarchy/") || !strings.Contains(stderr, "total="+itoa(tc.total)+" omitted="+itoa(tc.omitted)) {
				t.Fatalf("ASSERT_TRACE_SYMBOL_FAILURE_BEFORE_PREPARE: code=%d stdout=%q stderr=%q methods=%q", code, stdout, stderr, methods)
			}
			if tc.total > 1 {
				if strings.Count(stderr, "candidate:") != 8 || !strings.Contains(stderr, "use --at PATH:LINE:COLUMN") || strings.Index(stderr, "main.go:1:1") > strings.Index(stderr, "main.go:2:2") {
					t.Fatalf("ASSERT_TRACE_AMBIGUITY_BOUNDED_SORTED_REMEDIATION: %q", stderr)
				}
			}
		})
	}
}

func methodCount(raw []byte, method string) int {
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line == method {
			count++
		}
	}
	return count
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }
