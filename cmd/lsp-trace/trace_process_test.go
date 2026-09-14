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
	"lsp-trace/internal/presentation"
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
	defaultArgs := append(traceFakeArgs(workspace, "slice-symbol"), "--at", "main.go:1:1", "--down-depth", "0", "--up-depth", "0")
	defaultOut, defaultErr, defaultCode := captureRun(t, append([]string{"trace"}, defaultArgs...))
	if defaultCode != 0 || defaultErr != "" {
		t.Fatalf("ASSERT_TRACE_DEFAULT_PROCESS_COMPLETES: code=%d stderr=%q stdout=%q", defaultCode, defaultErr, defaultOut)
	}
	defaultEnvelope, defaultGraph := decodeTraceV5(t, defaultOut)
	if defaultEnvelope.SeedSpec != nil || defaultGraph.Invocation.Expansion.TopmostSiblings {
		t.Fatalf("ASSERT_TRACE_DEFAULT_SIBLINGS_OFF_NO_SEED_SPEC: code=%d stderr=%q expansion=%+v seed=%v", defaultCode, defaultErr, defaultGraph.Invocation.Expansion, defaultEnvelope.SeedSpec)
	}

	traceArgs := append(traceFakeArgs(workspace, "slice-symbol"), "--at", "main.go:1:1", "--down-depth", "0", "--up-depth", "0", "--timeout", "60s", "--request-timeout", "30s", "--siblings")
	traceOut, traceErr, traceCode := captureRun(t, append([]string{"trace"}, traceArgs...))
	cfg, err := parseTrace(traceArgs)
	if err != nil {
		t.Fatal(err)
	}
	_, seedBytes, err := traceSeeds(cfg)
	if err != nil {
		t.Fatal(err)
	}
	seedPath := filepath.Join(t.TempDir(), "seeds.v2.json")
	if err := os.WriteFile(seedPath, seedBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	sliceArgs := []string{"--workspace", workspace, "--server", os.Args[0], "--server-arg", "-test.run=^TestFakeLanguageServerProcess$", "--server-env", "LSP_TRACE_FAKE_SERVER=1", "--server-env", "LSP_TRACE_FAKE_SCENARIO=slice-symbol", "--language-id", "go", "--seed-file", seedPath}
	sliceOut, sliceErr, sliceCode := captureRun(t, append([]string{"slice", "--production-v5"}, sliceArgs...))
	if traceCode != 0 || sliceCode != 0 {
		t.Fatalf("ASSERT_TRACE_PUBLIC_SLICE_PROCESS_COMPLETES: traceCode=%d sliceCode=%d traceErr=%q sliceErr=%q traceOut=%q sliceOut=%q", traceCode, sliceCode, traceErr, sliceErr, traceOut, sliceOut)
	}
	traceEnvelope, traceGraph := decodeTraceV5(t, traceOut)
	sliceEnvelope, sliceGraph := decodeTraceV5(t, sliceOut)
	if traceCode != sliceCode || traceEnvelope.SeedSpec != nil || sliceEnvelope.SeedSpec != nil || traceEnvelope.GraphV5 != sliceEnvelope.GraphV5 || traceEnvelope.GraphV5SHA256 != sliceEnvelope.GraphV5SHA256 || !reflect.DeepEqual(traceGraph.Edges, sliceGraph.Edges) {
		t.Fatalf("ASSERT_TRACE_PUBLIC_SLICE_V5_GRAPH_DIGEST_CALLS_PARITY_NO_SEED_SPEC: traceCode=%d sliceCode=%d traceErr=%q sliceErr=%q traceDigest=%s sliceDigest=%s traceSeed=%v sliceSeed=%v", traceCode, sliceCode, traceErr, sliceErr, traceEnvelope.GraphV5SHA256, sliceEnvelope.GraphV5SHA256, traceEnvelope.SeedSpec, sliceEnvelope.SeedSpec)
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
	if code != 0 {
		t.Fatalf("ASSERT_TRACE_ONE_MULTI_TARGET_V5_ACQUISITION_SETUP: code=%d stderr=%q stdout=%q", code, stderr, stdout)
	}
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

func TestTraceProcessFormatJSONParityAndTreePresentation(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("managed local-process integration is Darwin-only")
	}
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\nfunc start() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	base := append(traceFakeArgs(workspace, "slice-symbol"), "--at", "main.go:1:1", "--down-depth", "0", "--up-depth", "0")
	bareOut, bareErr, bareCode := captureRun(t, append([]string{"trace"}, base...))
	jsonOut, jsonErr, jsonCode := captureRun(t, append(append([]string{"trace"}, base...), "--format", "json"))
	if bareCode != 0 || jsonCode != 0 || bareErr != "" || jsonErr != "" || bareOut != jsonOut {
		t.Fatalf("ASSERT_TRACE_BARE_EXPLICIT_JSON_BYTE_PARITY: bare=(%d,%q) json=(%d,%q) equal=%v", bareCode, bareErr, jsonCode, jsonErr, bareOut == jsonOut)
	}
	expected, err := presentation.RenderTraceV5([]byte(bareOut), presentation.TraceV5Options{})
	if err != nil {
		t.Fatal(err)
	}
	methodLog := filepath.Join(t.TempDir(), "methods.log")
	treeArgs := append(append([]string{}, base...), "--server-env", "LSP_TRACE_FAKE_METHOD_LOG="+methodLog, "--format", "tree")
	treeOut, treeErr, treeCode := captureRun(t, append([]string{"trace"}, treeArgs...))
	methods, readErr := os.ReadFile(methodLog)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if treeCode != 0 || treeErr != "" || treeOut != expected || !strings.Contains(treeOut, "status: PARTIAL\nnodes: 0") || !strings.Contains(treeOut, "targets: none") || methodCount(methods, "initialize") != 1 || methodCount(methods, "textDocument/prepareCallHierarchy") != 1 {
		t.Fatalf("ASSERT_TRACE_TREE_ACCEPTED_RENDERER_PARTIAL_ZERO_NODE_ONE_ACQUISITION: code=%d stderr=%q tree=%q expected=%q methods=%q", treeCode, treeErr, treeOut, expected, methods)
	}
}

func TestTraceFormatSyntaxAndConflictsRejectBeforeServerLaunch(t *testing.T) {
	workspace := t.TempDir()
	marker := filepath.Join(workspace, "server-started")
	base := []string{"trace", "--workspace", workspace, "--server", "/bin/sh", "--server-arg", "-c", "--server-arg", "touch " + marker, "--at", "main.go:1:1"}
	for _, suffix := range [][]string{{"--format"}, {"--format", "yaml"}, {"--format", "json", "--format", "tree"}, {"--format", "tree", "--pretty"}, {"--format", "tree", "--output", filepath.Join(workspace, "out.json")}} {
		stdout, stderr, code := captureRun(t, append(append([]string{}, base...), suffix...))
		if code != 1 || stdout != "" || stderr == "" {
			t.Fatalf("ASSERT_TRACE_FORMAT_INVALID_PRESTART: suffix=%v code=%d stdout=%q stderr=%q", suffix, code, stdout, stderr)
		}
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatalf("ASSERT_TRACE_FORMAT_INVALID_DID_NOT_LAUNCH_SERVER: suffix=%v stat=%v", suffix, err)
		}
	}
}

func TestTraceTreeAcquisitionFailurePreservesDiagnosticAndExit(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	base := []string{"trace", "--workspace", workspace, "--server", filepath.Join(workspace, "missing-server"), "--at", "main.go:1:1"}
	bareOut, bareErr, bareCode := captureRun(t, base)
	treeOut, treeErr, treeCode := captureRun(t, append(append([]string{}, base...), "--format", "tree"))
	if bareCode == 0 || treeCode != bareCode || bareErr == "" || treeErr != bareErr || bareOut != "" || treeOut != "" {
		t.Fatalf("ASSERT_TRACE_TREE_ACQUISITION_FAILURE_PARITY: bare=(%d,%q,%q) tree=(%d,%q,%q)", bareCode, bareOut, bareErr, treeCode, treeOut, treeErr)
	}
}

func TestTraceFormatParserPreservesServerArgumentValueAndHelp(t *testing.T) {
	workspace := t.TempDir()
	args := []string{"--workspace", workspace, "--server", "server", "--server-arg", "--format", "--at", "main.go:1:1", "--format", "tree"}
	cfg, err := parseTrace(args)
	if err != nil || !reflect.DeepEqual([]string(cfg.args), []string{"--format"}) {
		t.Fatalf("ASSERT_TRACE_FORMAT_SERVER_ARG_NOT_CONSUMED: cfg=%+v err=%v", cfg, err)
	}
	stdout, stderr, code := captureRun(t, []string{"trace", "--help", "--format", "tree"})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "-format value") {
		t.Fatalf("ASSERT_TRACE_FORMAT_HELP_SIDE_EFFECT_FREE: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestTraceHelpFullStreamAdmissionContract(t *testing.T) {
	workspace := t.TempDir()
	marker := filepath.Join(workspace, "server-started")
	base := []string{"--workspace", workspace, "--server", "/bin/sh", "--server-arg", "-c", "--server-arg", "touch " + marker, "--at", "main.go:1:1"}
	valid := []struct {
		name string
		args []string
	}{
		{"help-first", append([]string{"--help"}, base...)},
		{"help-middle", append(append([]string{}, base[:4]...), append([]string{"-h"}, base[4:]...)...)},
		{"help-last", append(append([]string{}, base...), "--help")},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := captureRun(t, append([]string{"trace"}, tc.args...))
			if code != 0 || stderr != "" || !strings.Contains(stdout, "-format value") {
				t.Fatalf("ASSERT_TRACE_HELP_VALID_FULL_STREAM: code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("ASSERT_TRACE_HELP_VALID_NO_LAUNCH: stat=%v", err)
			}
		})
	}

	invalid := []struct {
		name string
		args []string
	}{
		{"unknown-then-help", append(append([]string{}, base...), "--unknown", "--help")},
		{"positional-then-help", append(append([]string{}, base...), "stray", "--help")},
		{"help-then-missing-format", append(append([]string{}, base...), "--help", "--format")},
		{"help-as-invalid-format-value", append(append([]string{}, base...), "--format", "--help")},
		{"invalid-format-then-help", append(append([]string{}, base...), "--format", "yaml", "--help")},
		{"duplicate-format-then-help", append(append([]string{}, base...), "--format", "json", "--format", "tree", "--help")},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := captureRun(t, append([]string{"trace"}, tc.args...))
			if code != 1 || stdout != "" || stderr == "" {
				t.Fatalf("ASSERT_TRACE_HELP_MALFORMED_EMPTY_STDOUT: code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("ASSERT_TRACE_HELP_MALFORMED_NO_LAUNCH: stat=%v", err)
			}
		})
	}
}

func TestTraceHelpTokensAreOpaqueServerArguments(t *testing.T) {
	workspace := t.TempDir()
	for _, value := range []string{"--help", "-h", "--format", "--"} {
		t.Run(value, func(t *testing.T) {
			args := []string{"--workspace", workspace, "--server", "server", "--server-arg", value, "--at", "main.go:1:1"}
			cfg, err := parseTrace(args)
			if err != nil || !reflect.DeepEqual([]string(cfg.args), []string{value}) {
				t.Fatalf("ASSERT_TRACE_SERVER_ARG_OPAQUE: value=%q cfg=%+v err=%v", value, cfg, err)
			}
		})
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
