package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/mcp"

	"lsp-trace/internal/transientstructural"
	"lsp-trace/internal/transientstructuralresult"
)

func TestParseContextMachineContract(t *testing.T) {
	c, err := parseContext([]string{"--machine", "--workspace", ".", "--server", "gopls", "--at", "main.go:3:5", "--analysis", "impact", "--direction", "incoming", "--analysis-depth", "2", "--up-depth", "2"})
	if err != nil {
		t.Fatal(err)
	}
	if !c.machine || c.line != 2 || c.character != 4 {
		t.Fatalf("machine/coordinates = %v/%d/%d", c.machine, c.line, c.character)
	}
	if c.maxMessages != 64 || c.maxBytes != 4194304 {
		t.Fatalf("defaults = %d/%d", c.maxMessages, c.maxBytes)
	}
	if c.analysis.Kind != transientstructural.AnalysisImpact || c.analysis.Direction != transientstructural.DirectionIncoming || c.analysis.MaxDepth != 2 {
		t.Fatalf("analysis = %#v", c.analysis)
	}
}

func TestParseContextRejectsPrivateContractViolations(t *testing.T) {
	base := []string{"--machine", "--workspace", ".", "--server", "gopls", "--at", "main.go:1:1"}
	for _, extra := range [][]string{{"--max-messages", "4097"}, {"--max-bytes", "16777217"}, {"--timeout", "1s", "--request-timeout", "2s"}, {"--analysis", "impact"}, {"--output-selector", "x"}} {
		if _, err := parseContext(append(append([]string{}, base...), extra...)); err == nil {
			t.Fatalf("accepted %v", extra)
		}
	}
	if _, err := parseContext(base[1:]); err == nil {
		t.Fatal("accepted invocation without --machine")
	}
}

func TestContextMachinePrivateProcessQualification(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	before := len(mcp.NewRegistry(false).Tools())
	args := []string{"context", "--machine", "--workspace", workspace, "--server", os.Args[0], "--server-arg", "-test.run=^TestFakeLanguageServerProcess$", "--server-env", "LSP_TRACE_FAKE_SERVER=1", "--server-env", "LSP_TRACE_FAKE_SCENARIO=slice", "--at", "main.go:1:1", "--down-depth", "1", "--up-depth", "1", "--request-timeout", "500ms", "--timeout", "2s"}
	stdout, stderr, code := captureRun(t, args)
	var got transientstructuralresult.Result
	if err := json.Unmarshal([]byte(stdout), &got); code != 0 || err != nil {
		t.Fatalf("ASSERT_CONTEXT_PRIVATE_PROCESS: code=%d decode=%v stderr=%q stdout=%q", code, err, stderr, stdout)
	}
	if got.State != transientstructuralresult.Complete && got.State != transientstructuralresult.Empty {
		t.Fatalf("ASSERT_CONTEXT_CLOSED_SUCCESS: %s", got.State)
	}
	if len(mcp.NewRegistry(false).Tools()) != before {
		t.Fatal("ASSERT_CONTEXT_REGISTRY_CARDINALITY_UNCHANGED")
	}
	for _, tool := range mcp.NewRegistry(false).Tools() {
		if strings.Contains(tool.Name, "structural_context") {
			t.Fatal("ASSERT_CONTEXT_OPERATION_35_ABSENT")
		}
	}
}

func TestNormalizeContextResultUsesSharedBoundary(t *testing.T) {
	in := transientstructural.Result{State: transientstructural.StateComplete, Qualification: transientstructural.Qualification{SessionID: "ts_0123456789abcdef0123456789abcdef", Generation: 7, PositionEncoding: "utf-16"}, TargetID: strings.Repeat("a", 64), Accounting: transientstructural.Accounting{}, Analysis: transientstructural.AnalysisResult{Kind: transientstructural.AnalysisNeighborhood, Nodes: []transientstructural.NodeFact{{ID: strings.Repeat("a", 64)}}}}
	q := transientstructural.Request{Generation: 7, DownDepth: 2, UpDepth: 2, MaxNodes: 100, TimeoutMS: 5000, RequestTimeoutMS: 1000, MaxMessages: 64, MaxBytes: 4194304, Analysis: transientstructural.AnalysisRequest{Kind: transientstructural.AnalysisNeighborhood}}
	got, err := normalizeContextResult(in, q, "ts_0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	if err := got.Validate(); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(got)
	if !strings.Contains(string(raw), `"node_id":"tn_`) {
		t.Fatalf("missing private node identity: %s", raw)
	}
	if strings.Contains(string(raw), "output_selector") || strings.Contains(string(raw), "custody") || strings.Contains(string(raw), "source_supply") {
		t.Fatalf("forbidden field: %s", raw)
	}
	other, err := normalizeContextResult(in, q, "ts_abcdef0123456789abcdef0123456789")
	if err != nil {
		t.Fatal(err)
	}
	if other.TargetNodeID == got.TargetNodeID {
		t.Fatal("ASSERT_CONTEXT_MANAGER_SCOPED_IDENTITY")
	}
	var _ transientstructuralresult.Result = got
}

func TestNormalizeContextResultOmissionsAndPrivateFailures(t *testing.T) {
	in := transientstructural.Result{
		State:         transientstructural.StateEmpty,
		Qualification: transientstructural.Qualification{Generation: 7, PositionEncoding: "utf-16"},
		TargetID:      "private-root",
		Accounting: transientstructural.Accounting{
			Nodes:     transientstructural.AdmissionAccounting{Observed: 1, Admitted: 1},
			Frontier:  transientstructural.FrontierAccounting{Observed: 1, Unexpanded: 1},
			Omissions: []transientstructural.OmissionCount{{Reason: transientstructural.OmissionDepthBound, Count: 1}},
		},
		Analysis: transientstructural.AnalysisResult{Kind: transientstructural.AnalysisNeighborhood, Nodes: []transientstructural.NodeFact{{ID: "private-root"}}},
	}
	q := transientstructural.Request{Generation: 7, DownDepth: 0, UpDepth: 0, MaxNodes: 100, TimeoutMS: 5000, RequestTimeoutMS: 1000, MaxMessages: 64, MaxBytes: 4194304, Analysis: transientstructural.AnalysisRequest{Kind: transientstructural.AnalysisNeighborhood}}
	got, err := normalizeContextResult(in, q, "ts_0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	if got.Accounting.FrontierOmissionReasons[transientstructuralresult.DepthBound] != 1 {
		t.Fatalf("ASSERT_CONTEXT_OMISSION_REASON_TRANSLATION: %+v", got.Accounting.FrontierOmissionReasons)
	}
	in.TargetID = "secret-target-not-in-nodes"
	_, err = normalizeContextResult(in, q, "ts_0123456789abcdef0123456789abcdef")
	if err == nil || strings.Contains(err.Error(), in.TargetID) || strings.Contains(err.Error(), "ts_") {
		t.Fatalf("ASSERT_CONTEXT_PRIVATE_NORMALIZATION_FAILURE: %q", err)
	}
}
