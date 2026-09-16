package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/vcssymbolsidecar"
)

const (
	contextSymbolChurnSuccessV4        = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-context-symbol-churn-result.v4.schema.json"
	contextSymbolChurnCaptureSuccessV3 = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-context-symbol-churn-capture-result.v3.schema.json"
)

type metricArtifactExecutor struct {
	artifact []byte
	calls    []operation.Request
}

func (e *metricArtifactExecutor) Execute(_ context.Context, req operation.Request) (operation.Result, *operation.Failure) {
	e.calls = append(e.calls, req)
	return operation.Result{Artifact: append([]byte(nil), e.artifact...)}, nil
}

type decodedMetricPresentation struct {
	Total     int                             `json:"total"`
	Returned  int                             `json:"returned"`
	Truncated bool                            `json:"truncated"`
	Items     []vcssymbolsidecar.SymbolMetric `json:"items"`
}

type decodedMetricEnvelope struct {
	EnvelopeVersion  string          `json:"envelope_version"`
	EnvelopeSchemaID string          `json:"envelope_schema_id"`
	Result           json.RawMessage `json:"result"`
	Summary          struct {
		Historical decodedMetricPresentation `json:"historical_symbol_metrics"`
		Current    decodedMetricPresentation `json:"current_symbol_metrics"`
	} `json:"summary"`
}

func metricPositionLess(a, b vcssymbolsidecar.Position) bool {
	return a.Line < b.Line || (a.Line == b.Line && a.Character < b.Character)
}

func artifactMetricLess(a, b vcssymbolsidecar.SymbolMetric) bool {
	if a.Path != b.Path {
		return a.Path < b.Path
	}
	if a.Range.Start != b.Range.Start {
		return metricPositionLess(a.Range.Start, b.Range.Start)
	}
	if a.Range.End != b.Range.End {
		return metricPositionLess(a.Range.End, b.Range.End)
	}
	if a.Name != b.Name {
		return a.Name < b.Name
	}
	return a.Kind < b.Kind
}

func metricSpan(r vcssymbolsidecar.Range) int {
	span := r.End.Line - r.Start.Line
	if r.End.Character > 0 {
		span++
	}
	return span
}

func metricRow(path, name string, kind int, r vcssymbolsidecar.Range, changed int) vcssymbolsidecar.SymbolMetric {
	span := metricSpan(r)
	return vcssymbolsidecar.SymbolMetric{Path: path, Name: name, Kind: kind, Range: r, ChangedLineCount: changed, SymbolSpanLineCount: span, ChurnDensityBasisPoints: changed * 10000 / span}
}

func metricLines(side string, metrics []vcssymbolsidecar.SymbolMetric) []vcssymbolsidecar.LineAttribution {
	lines := make([]vcssymbolsidecar.LineAttribution, 0)
	for _, metric := range metrics {
		for i := 0; i < metric.ChangedLineCount; i++ {
			r := metric.Range
			lines = append(lines, vcssymbolsidecar.LineAttribution{Side: side, Path: metric.Path, Line: metric.Range.Start.Line + i + 1, Outcome: "ATTRIBUTED", SymbolName: metric.Name, SymbolKind: metric.Kind, SymbolRange: &r})
		}
	}
	return lines
}

func metricArtifact(t *testing.T, historical, current []vcssymbolsidecar.SymbolMetric, reverseLines bool) []byte {
	t.Helper()
	historical = append([]vcssymbolsidecar.SymbolMetric{}, historical...)
	current = append([]vcssymbolsidecar.SymbolMetric{}, current...)
	sort.SliceStable(historical, func(i, j int) bool { return artifactMetricLess(historical[i], historical[j]) })
	sort.SliceStable(current, func(i, j int) bool { return artifactMetricLess(current[i], current[j]) })
	lines := append(metricLines("OLD", historical), metricLines("NEW", current)...)
	if reverseLines {
		for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
			lines[i], lines[j] = lines[j], lines[i]
		}
	}
	result := vcssymbolsidecar.ResultV3{
		Result: vcssymbolsidecar.Result{
			SchemaVersion: "lsp-trace.vcs-symbol-churn-sidecar.v3", GraphArtifactDigest: "sha256:" + strings.Repeat("a", 64),
			FromRevision: strings.Repeat("b", 40), ToRevision: strings.Repeat("c", 40),
			OldAcquisition: []vcssymbolsidecar.FileOutcome{{Path: "fixture.go", Status: "EMPTY", Symbols: []vcssymbolsidecar.Symbol{}}},
			NewAcquisition: []vcssymbolsidecar.FileOutcome{{Path: "fixture.go", Status: "EMPTY", Symbols: []vcssymbolsidecar.Symbol{}}},
			Authority:      0, SourceGraphComplete: "UNKNOWN", Attribution: "HISTORICAL_SYMBOL_RANGE", CrossRevisionIdentity: "NOT_EVALUATED",
			LineCount: len(lines), AttributedLineCount: len(lines), Lines: lines,
		},
		HistoricalSymbolMetrics: historical,
		CurrentSymbolMetrics:    current,
	}
	if err := vcssymbolsidecar.ValidateV3(result); err != nil {
		t.Fatalf("invalid test fixture: %v", err)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func invokeMetricEnvelope(t *testing.T, tool string, family ExecutorFamily, artifact []byte, gateway bool) ([]byte, int) {
	t.Helper()
	executor := &metricArtifactExecutor{artifact: artifact}
	s := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull), Executors: map[ExecutorFamily]Executor{family: executor}}
	var args map[string]any
	if tool == mcpcontract.ContextSymbolChurnTool {
		args = map[string]any{"input": "{}", "workspace": "/tmp/repo", "from_revision": "HEAD~1", "to_revision": "HEAD", "profile": "go", "language_id": "go"}
	} else {
		args = map[string]any{"session_id": "project", "generation": 1, "uri": "file:///tmp/repo/a.go", "symbol": "A", "down_depth": 1, "up_depth": 0, "max_nodes": 10, "timeout_ms": 1000, "request_timeout_ms": 500, "analysis": map[string]any{"kind": "NEIGHBORHOOD"}, "from_revision": "HEAD~1", "to_revision": "HEAD", "profile": "go", "language_id": "go"}
	}
	name := tool
	callArgs := args
	if gateway {
		name = "lsp_trace_v1_execute"
		callArgs = map[string]any{"request": map[string]any{"operation": tool, "arguments": args}}
	}
	got := s.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, name, callArgs))
	return symbolChurnEnvelopeBytes(t, got, gateway), len(executor.calls)
}

func decodeMetricEnvelope(t *testing.T, raw []byte) decodedMetricEnvelope {
	t.Helper()
	var got decodedMetricEnvelope
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func countMetrics(side string, n int) []vcssymbolsidecar.SymbolMetric {
	metrics := make([]vcssymbolsidecar.SymbolMetric, n)
	for i := range metrics {
		start := i * 3
		metrics[i] = metricRow("fixture.go", fmt.Sprintf("%s-%02d", side, i), 12, vcssymbolsidecar.Range{Start: vcssymbolsidecar.Position{Line: start}, End: vcssymbolsidecar.Position{Line: start + 1}}, 1)
	}
	return metrics
}

func TestContextSymbolChurnV3MetricPresentationExactRowsOneLineAndEmpty(t *testing.T) {
	oneLine := metricRow("one.go", "single", 12, vcssymbolsidecar.Range{Start: vcssymbolsidecar.Position{Line: 7, Character: 2}, End: vcssymbolsidecar.Position{Line: 7, Character: 9}}, 1)
	got := contextSymbolMetricSummary([]vcssymbolsidecar.SymbolMetric{oneLine})
	items, ok := got["items"].([]vcssymbolsidecar.SymbolMetric)
	if !ok || !reflect.DeepEqual(items, []vcssymbolsidecar.SymbolMetric{oneLine}) || oneLine.ChurnDensityBasisPoints != 10000 {
		t.Fatalf("ASSERT_SYMBOL_CHURN_METRIC_SUMMARY_EXACT_ROWS_NO_MINIMUM_SPAN: items=%+v metric=%+v", items, oneLine)
	}
	empty := contextSymbolMetricSummary([]vcssymbolsidecar.SymbolMetric{})
	emptyItems, ok := empty["items"].([]vcssymbolsidecar.SymbolMetric)
	if !ok || emptyItems == nil || len(emptyItems) != 0 || empty["total"] != 0 || empty["returned"] != 0 || empty["truncated"] != false {
		t.Fatalf("ASSERT_SYMBOL_CHURN_METRIC_SUMMARY_EMPTY_SIDE: summary=%+v", empty)
	}
	t.Log("PASS ASSERT_SYMBOL_CHURN_METRIC_SUMMARY_EXACT_ROWS_NO_MINIMUM_SPAN")
	t.Log("PASS ASSERT_SYMBOL_CHURN_METRIC_SUMMARY_EMPTY_SIDE")
}

func TestContextSymbolChurnV3MetricPresentationBoundsAndIndependentSides(t *testing.T) {
	for _, tc := range []struct {
		name                string
		historical, current int
	}{
		{name: "fewer-than-ten", historical: 3, current: 0},
		{name: "exactly-ten", historical: 10, current: 10},
		{name: "more-than-ten-independent", historical: 11, current: 12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			artifact := metricArtifact(t, countMetrics("historical", tc.historical), countMetrics("current", tc.current), false)
			raw, _ := invokeMetricEnvelope(t, mcpcontract.ContextSymbolChurnTool, ContextSymbolChurnExecutorFamily, artifact, false)
			got := decodeMetricEnvelope(t, raw)
			for name, pair := range map[string]struct {
				got  decodedMetricPresentation
				want int
			}{"historical": {got.Summary.Historical, tc.historical}, "current": {got.Summary.Current, tc.current}} {
				returned := pair.want
				if returned > 10 {
					returned = 10
				}
				if pair.got.Total != pair.want || pair.got.Returned != returned || pair.got.Truncated != (pair.want > 10) || len(pair.got.Items) != returned {
					t.Fatalf("ASSERT_SYMBOL_CHURN_METRIC_SUMMARY_BOUNDS_AND_INDEPENDENT_SIDES[%s/%s]: got=%+v want_total=%d", tc.name, name, pair.got, pair.want)
				}
			}
			t.Log("PASS ASSERT_SYMBOL_CHURN_METRIC_SUMMARY_BOUNDS_AND_INDEPENDENT_SIDES")
		})
	}
}

func TestContextSymbolChurnV3MetricPresentationRankingEveryTieBreaker(t *testing.T) {
	r := func(sl, sc, el, ec int) vcssymbolsidecar.Range {
		return vcssymbolsidecar.Range{Start: vcssymbolsidecar.Position{Line: sl, Character: sc}, End: vcssymbolsidecar.Position{Line: el, Character: ec}}
	}
	for _, tc := range []struct {
		name, first string
		metrics     []vcssymbolsidecar.SymbolMetric
	}{
		{name: "changed-count-before-density", first: "changed-two", metrics: []vcssymbolsidecar.SymbolMetric{metricRow("a.go", "dense-one", 12, r(20, 0, 20, 1), 1), metricRow("z.go", "changed-two", 12, r(0, 0, 10, 0), 2)}},
		{name: "density", first: "denser", metrics: []vcssymbolsidecar.SymbolMetric{metricRow("a.go", "sparser", 12, r(0, 0, 4, 0), 1), metricRow("z.go", "denser", 12, r(0, 0, 2, 0), 1)}},
		{name: "path", first: "path-a", metrics: []vcssymbolsidecar.SymbolMetric{metricRow("b.go", "path-b", 12, r(0, 0, 1, 0), 1), metricRow("a.go", "path-a", 12, r(0, 0, 1, 0), 1)}},
		{name: "start-line", first: "start-line-one", metrics: []vcssymbolsidecar.SymbolMetric{metricRow("a.go", "start-line-two", 12, r(2, 0, 2, 1), 1), metricRow("a.go", "start-line-one", 12, r(1, 0, 1, 1), 1)}},
		{name: "start-character", first: "start-char-zero", metrics: []vcssymbolsidecar.SymbolMetric{metricRow("a.go", "start-char-one", 12, r(1, 1, 1, 5), 1), metricRow("a.go", "start-char-zero", 12, r(1, 0, 1, 5), 1)}},
		{name: "end-line", first: "end-line-one", metrics: []vcssymbolsidecar.SymbolMetric{metricRow("a.go", "end-line-two", 12, r(0, 0, 2, 0), 1), metricRow("a.go", "end-line-one", 12, r(0, 0, 1, 1), 1)}},
		{name: "end-character", first: "end-char-one", metrics: []vcssymbolsidecar.SymbolMetric{metricRow("a.go", "end-char-two", 12, r(1, 0, 1, 2), 1), metricRow("a.go", "end-char-one", 12, r(1, 0, 1, 1), 1)}},
		{name: "name", first: "alpha", metrics: []vcssymbolsidecar.SymbolMetric{metricRow("a.go", "beta", 12, r(1, 0, 1, 1), 1), metricRow("a.go", "alpha", 12, r(1, 0, 1, 1), 1)}},
		{name: "kind", first: "same", metrics: []vcssymbolsidecar.SymbolMetric{metricRow("a.go", "same", 13, r(1, 0, 1, 1), 1), metricRow("a.go", "same", 12, r(1, 0, 1, 1), 1)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			artifact := metricArtifact(t, nil, tc.metrics, false)
			raw, _ := invokeMetricEnvelope(t, mcpcontract.ContextSymbolChurnTool, ContextSymbolChurnExecutorFamily, artifact, false)
			items := decodeMetricEnvelope(t, raw).Summary.Current.Items
			if len(items) != 2 || items[0].Name != tc.first || (tc.name == "kind" && items[0].Kind != 12) {
				t.Fatalf("ASSERT_SYMBOL_CHURN_METRIC_SUMMARY_TOTAL_ORDER[%s]: items=%+v want_first=%s", tc.name, items, tc.first)
			}
			t.Log("PASS ASSERT_SYMBOL_CHURN_METRIC_SUMMARY_TOTAL_ORDER[" + tc.name + "]")
		})
	}
}

func TestContextSymbolChurnV3MetricPresentationPermutationAndArtifactCompleteness(t *testing.T) {
	historical := countMetrics("historical", 11)
	current := countMetrics("current", 12)
	firstArtifact := metricArtifact(t, historical, current, false)
	secondArtifact := metricArtifact(t, historical, current, true)
	firstRaw, _ := invokeMetricEnvelope(t, mcpcontract.ContextSymbolChurnTool, ContextSymbolChurnExecutorFamily, firstArtifact, false)
	secondRaw, _ := invokeMetricEnvelope(t, mcpcontract.ContextSymbolChurnTool, ContextSymbolChurnExecutorFamily, secondArtifact, false)
	first := decodeMetricEnvelope(t, firstRaw)
	second := decodeMetricEnvelope(t, secondRaw)
	var sourceArtifact, returnedArtifact any
	if err := json.Unmarshal(firstArtifact, &sourceArtifact); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(first.Result, &returnedArtifact); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sourceArtifact, returnedArtifact) {
		t.Fatalf("ASSERT_SYMBOL_CHURN_METRIC_SUMMARY_PRESERVES_COMPLETE_NATIVE_ARTIFACT: source=%+v returned=%+v", sourceArtifact, returnedArtifact)
	}
	if !reflect.DeepEqual(first.Summary, second.Summary) {
		t.Fatalf("ASSERT_SYMBOL_CHURN_METRIC_SUMMARY_INPUT_PERMUTATION_DETERMINISM: first=%+v second=%+v", first.Summary, second.Summary)
	}
	var complete struct {
		Historical []vcssymbolsidecar.SymbolMetric    `json:"historical_symbol_metrics"`
		Current    []vcssymbolsidecar.SymbolMetric    `json:"current_symbol_metrics"`
		Lines      []vcssymbolsidecar.LineAttribution `json:"lines"`
	}
	if err := json.Unmarshal(first.Result, &complete); err != nil {
		t.Fatal(err)
	}
	if len(complete.Historical) != 11 || len(complete.Current) != 12 || len(complete.Lines) != 23 || first.Summary.Historical.Returned != 10 || first.Summary.Current.Returned != 10 {
		t.Fatalf("ASSERT_SYMBOL_CHURN_METRIC_SUMMARY_NEVER_TRUNCATES_ARTIFACT: historical=%d current=%d lines=%d summary=%+v", len(complete.Historical), len(complete.Current), len(complete.Lines), first.Summary)
	}
	t.Log("PASS ASSERT_SYMBOL_CHURN_METRIC_SUMMARY_PRESERVES_COMPLETE_NATIVE_ARTIFACT")
	t.Log("PASS ASSERT_SYMBOL_CHURN_METRIC_SUMMARY_INPUT_PERMUTATION_DETERMINISM")
	t.Log("PASS ASSERT_SYMBOL_CHURN_METRIC_SUMMARY_NEVER_TRUNCATES_ARTIFACT")
}

func TestContextSymbolChurnV3MetricPresentationOperationParity(t *testing.T) {
	artifact := metricArtifact(t, countMetrics("historical", 4), countMetrics("current", 11), false)
	op39Direct, calls39Direct := invokeMetricEnvelope(t, mcpcontract.ContextSymbolChurnTool, ContextSymbolChurnExecutorFamily, artifact, false)
	op39Gateway, calls39Gateway := invokeMetricEnvelope(t, mcpcontract.ContextSymbolChurnTool, ContextSymbolChurnExecutorFamily, artifact, true)
	op40Direct, calls40Direct := invokeMetricEnvelope(t, mcpcontract.ContextSymbolChurnCaptureTool, ContextSymbolChurnCaptureExecutorFamily, artifact, false)
	got39Direct := decodeMetricEnvelope(t, op39Direct)
	got39Gateway := decodeMetricEnvelope(t, op39Gateway)
	got40Direct := decodeMetricEnvelope(t, op40Direct)
	if got39Direct.EnvelopeSchemaID != contextSymbolChurnSuccessV4 || got39Direct.EnvelopeVersion != "4" || got40Direct.EnvelopeSchemaID != contextSymbolChurnCaptureSuccessV3 || got40Direct.EnvelopeVersion != "4" {
		t.Fatalf("ASSERT_SYMBOL_CHURN_METRIC_SUMMARY_NEW_ENVELOPE_IDS: op39=%s/%s op40=%s/%s", got39Direct.EnvelopeVersion, got39Direct.EnvelopeSchemaID, got40Direct.EnvelopeVersion, got40Direct.EnvelopeSchemaID)
	}
	if !reflect.DeepEqual(got39Direct.Summary, got39Gateway.Summary) || !reflect.DeepEqual(got39Direct.Summary, got40Direct.Summary) || calls39Direct != 1 || calls39Gateway != 1 || calls40Direct != 1 {
		t.Fatalf("ASSERT_SYMBOL_CHURN_METRIC_SUMMARY_DIRECT_GATEWAY_CAPTURE_PARITY: calls=%d/%d/%d summaries=%+v/%+v/%+v", calls39Direct, calls39Gateway, calls40Direct, got39Direct.Summary, got39Gateway.Summary, got40Direct.Summary)
	}
	t.Log("PASS ASSERT_SYMBOL_CHURN_METRIC_SUMMARY_NEW_ENVELOPE_IDS")
	t.Log("PASS ASSERT_SYMBOL_CHURN_METRIC_SUMMARY_DIRECT_GATEWAY_CAPTURE_PARITY")
}

func TestContextSymbolChurnMetricPresentationSchemasAppendOnlyAndPredecessorsImmutable(t *testing.T) {
	full := NewRegistryWithProfile(false, ToolProfileFull)
	compact := NewRegistryWithProfile(false, ToolProfileCompact)
	for toolName, expected := range map[string]string{mcpcontract.ContextSymbolChurnTool: contextSymbolChurnSuccessV4, mcpcontract.ContextSymbolChurnCaptureTool: contextSymbolChurnCaptureSuccessV3} {
		tool, ok := full.ResolveCanonical(toolName)
		if !ok || !strings.Contains(strings.Join(tool.EnvelopeSchemaIDs, "\n"), expected) {
			t.Fatalf("ASSERT_SYMBOL_CHURN_METRIC_SUMMARY_SCHEMAS_REGISTERED_APPEND_ONLY[%s]: ok=%t schemas=%v", toolName, ok, tool.EnvelopeSchemaIDs)
		}
	}
	if len(full.Tools()) != 43 || len(full.Advertised()) != 43 || len(compact.Tools()) != 43 || len(compact.Advertised()) != 13 {
		t.Fatalf("ASSERT_SYMBOL_CHURN_METRIC_SUMMARY_TOOL_COUNTS_43_13: full=%d/%d compact=%d/%d", len(full.Tools()), len(full.Advertised()), len(compact.Tools()), len(compact.Advertised()))
	}
	for path, expected := range map[string]string{
		"../mcpcontract/testdata/schemas/input-context-symbol-churn.v1.schema.json":                         "3b6aeb405b9847a274a002c641f7073cd7e815cf0b5a713a0c432e5eb7e9a2af",
		"../mcpcontract/testdata/schemas/envelope-context-symbol-churn-domain-error.v1.schema.json":         "4312c374efac603e8ecfab56c301362452824c604c7a44940a31819891d946db",
		"../mcpcontract/testdata/schemas/input-context-symbol-churn-capture.v1.schema.json":                 "46c5f5b997211649cd0f281aa97536fe1050093f942714449f3d137b568aa4a6",
		"../mcpcontract/testdata/schemas/envelope-context-symbol-churn-capture-domain-error.v1.schema.json": "067d11f03f04766b2a67b7c7e0020f9061c03fdb48672fec195a27818ca6c2fc",
		"../mcpcontract/testdata/schemas/envelope-context-symbol-churn-result.v1.schema.json":               "b07a52484f4ac251cbff95d5c6f8c7238c5e8aefb937a5c338edc3ec0d6021bc",
		"../mcpcontract/testdata/schemas/envelope-context-symbol-churn-result.v2.schema.json":               "2570e4a02948352154bebd2b860bdcf48e67c204be123cb8b7d03a06fcae35de",
		"../mcpcontract/testdata/schemas/envelope-context-symbol-churn-result.v3.schema.json":               "d1f51729203d25648df9bf5d8f618047dc494a146a3a7fee557e54270fc2e6b4",
		"../mcpcontract/testdata/schemas/envelope-context-symbol-churn-capture-result.v1.schema.json":       "5e8ea25c4eb377c4faa5362aa4612b8bba8d578d62b95d92cddd8591a9e9431b",
		"../mcpcontract/testdata/schemas/envelope-context-symbol-churn-capture-result.v2.schema.json":       "77edf10e86e314c9ca1d454d2989e70eadea01828799d746af2f5a6aa40d645a",
		"../mcpcontract/testdata/schemas/lsp-trace.vcs-symbol-churn-sidecar.v3.schema.json":                 "adeec32a7b5265cd2254f1e17d3bf1d0ea08b36dba52b0c3f8cfb2284d4972cd",
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		if got := hex.EncodeToString(sum[:]); got != expected {
			t.Fatalf("ASSERT_SYMBOL_CHURN_METRIC_SUMMARY_PREDECESSOR_SCHEMA_BYTES_IMMUTABLE[%s]: got=%s want=%s", path, got, expected)
		}
	}
	t.Log("PASS ASSERT_SYMBOL_CHURN_METRIC_SUMMARY_SCHEMAS_REGISTERED_APPEND_ONLY")
	t.Log("PASS ASSERT_SYMBOL_CHURN_METRIC_SUMMARY_TOOL_COUNTS_43_13")
	t.Log("PASS ASSERT_SYMBOL_CHURN_METRIC_SUMMARY_PREDECESSOR_SCHEMA_BYTES_IMMUTABLE")
	t.Log("PASS ASSERT_SYMBOL_CHURN_METRIC_SUMMARY_INPUT_AND_TYPED_FAILURE_SCHEMA_BYTES_UNCHANGED")
}
