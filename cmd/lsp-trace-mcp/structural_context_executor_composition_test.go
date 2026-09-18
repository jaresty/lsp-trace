package main

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/transientstructural"
	"lsp-trace/sessionruntime"
)

type structuralContextDelegateFunc func(context.Context, operation.Request) (operation.Result, *operation.Failure)

func (f structuralContextDelegateFunc) Execute(ctx context.Context, req operation.Request) (operation.Result, *operation.Failure) {
	return f(ctx, req)
}

const validStructuralV2 = `{"schema_version":"lsp-trace.transient-structural-result.v2","authority":0,"source_graph_complete":"UNKNOWN","position_encoding":"utf-16","target_node_id":"tn_0123456789abcdef0123456789abcdef","nodes":[{"node_id":"tn_0123456789abcdef0123456789abcdef","name":"A","kind":12,"path":"src/a.go","declaration_range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}],"calls":[],"analytics_scope":"BOUNDED_LOCAL","strong_components":[],"weak_projection":"DIRECTED_ARCS_COLLAPSED_TO_SIMPLE_UNDIRECTED_PAIRS; SELF_LOOPS_IGNORED; PARALLEL_AND_ANTIPARALLEL_ARCS_COLLAPSED","weak_bridges":[],"articulation_points":[],"pagerank_damping":0.85,"analytics_tolerance":1e-12,"pagerank":[],"hits":[],"coupling":[{"node_id":"tn_0123456789abcdef0123456789abcdef","ca":0,"ce":0,"instability":0}],"external_nodes_omitted":0,"external_calls_omitted":0}`

func projectionWorkspace(t *testing.T, names ...string) (string, map[string]string) {
	t.Helper()
	root := t.TempDir()
	uris := make(map[string]string, len(names))
	for _, name := range names {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte("A\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		uris[name] = (&url.URL{Scheme: "file", Path: path}).String()
	}
	return root, uris
}

func TestUnifiedStructuralContextV2ExecutorComposesStructuralResult(t *testing.T) {
	const structural = validStructuralV2
	for _, tc := range []struct {
		name  string
		input string
	}{
		{name: "position", input: `{"uri":"file:///repo/a.go","line":0,"character":0}`},
		{name: "symbol", input: `{"symbol":"A"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			delegate := structuralContextDelegateFunc(func(_ context.Context, _ operation.Request) (operation.Result, *operation.Failure) {
				return operation.Result{Artifact: []byte(structural)}, nil
			})
			e := &unifiedStructuralContextV2Executor{exact: delegate, symbol: delegate}
			got, failure := e.Execute(context.Background(), operation.Request{Name: operation.Name("structural_context_v2"), Input: json.RawMessage(tc.input)})
			if failure != nil {
				t.Fatalf("ASSERT_UNIFIED_CONTEXT_RUNTIME_COMPOSITION_%s: failure=%v", tc.name, failure)
			}
			if err := mcpcontract.ValidateJSON(mcpcontract.UnifiedStructuralContextResultV2ID, got.Artifact); err != nil {
				t.Fatalf("ASSERT_UNIFIED_CONTEXT_RUNTIME_COMPOSITION_%s: %v\n%s", tc.name, err, got.Artifact)
			}
			var result struct {
				SchemaVersion string          `json:"schema_version"`
				Structural    json.RawMessage `json:"structural"`
				Projection    json.RawMessage `json:"projection"`
			}
			if err := json.Unmarshal(got.Artifact, &result); err != nil {
				t.Fatalf("ASSERT_UNIFIED_CONTEXT_RUNTIME_COMPOSITION_%s: %v", tc.name, err)
			}
			if result.SchemaVersion != "lsp-trace.unified-structural-context-result.v2" || string(result.Structural) != structural || result.Projection != nil {
				t.Fatalf("ASSERT_UNIFIED_CONTEXT_RUNTIME_COMPOSITION_%s: %+v", tc.name, result)
			}
		})
	}
}

func TestUnifiedStructuralContextV2ExecutorBindsLiveProjection(t *testing.T) {
	workspaceRoot, uris := projectionWorkspace(t, "a.go")
	uri := uris["a.go"]
	r := graph.Range{Start: graph.Position{Line: 0, Character: 0}, End: graph.Position{Line: 0, Character: 1}}
	transient := transientstructural.Result{TargetID: "root", Qualification: transientstructural.Qualification{SessionID: "s", Generation: 1, PositionEncoding: "utf-16"}, Analysis: transientstructural.AnalysisResult{Nodes: []transientstructural.NodeFact{{ID: "root", URI: uri, Range: r}}}, SourceSupply: &sessionruntime.DocumentSupply{Classification: "LSP_SUPPLIED", SessionID: "s", Generation: 1, URI: uri, DocumentVersion: 1, Method: "textDocument/didOpen", Content: []byte("A\n")}}
	delegate := structuralContextDelegateFunc(func(_ context.Context, _ operation.Request) (operation.Result, *operation.Failure) {
		return operation.Result{Artifact: []byte(validStructuralV2), Value: structuralContextProjectionInput{Transient: transient, WorkspaceRoot: workspaceRoot}}, nil
	})
	e := &unifiedStructuralContextV2Executor{exact: delegate, symbol: delegate}
	projection := `{"mode":"TARGET","body":"INCLUDE","include_relation_occurrences":false,"include_ancillary":false,"display_range_policy":"EXACT_EVIDENCE","limits":{"max_objects":1,"max_ranges":1,"max_source_bytes":2,"max_work":1,"max_response_bytes":100000,"max_additional_documents":0,"max_document_requests":1,"max_document_bytes":2,"max_total_document_bytes":2,"max_document_messages":1,"max_document_acquisition_work":1,"max_display_resolution_work":1},"privacy_policy_id":"public"}`
	got, failure := e.Execute(context.Background(), operation.Request{Name: operation.Name("structural_context_v2"), Input: json.RawMessage(`{"uri":"file:///repo/a.go","line":0,"character":0,"projection":` + projection + `}`)})
	if failure != nil {
		t.Fatalf("ASSERT_UNIFIED_CONTEXT_LIVE_PROJECTION: %v", failure)
	}
	if err := mcpcontract.ValidateJSON(mcpcontract.UnifiedStructuralContextResultV2ID, got.Artifact); err != nil {
		t.Fatalf("ASSERT_UNIFIED_CONTEXT_LIVE_PROJECTION: %v\n%s", err, got.Artifact)
	}
	var envelope struct {
		Projection json.RawMessage `json:"projection"`
	}
	if json.Unmarshal(got.Artifact, &envelope) != nil || len(envelope.Projection) == 0 {
		t.Fatalf("ASSERT_UNIFIED_CONTEXT_LIVE_PROJECTION: %s", got.Artifact)
	}
}

func TestUnifiedStructuralContextV2ExecutorLiveProjectionHonorsDocumentAndPrivacyBoundaries(t *testing.T) {
	workspaceRoot, uris := projectionWorkspace(t, "a.go", "b.go")
	uri := uris["a.go"]
	r := graph.Range{Start: graph.Position{Line: 0, Character: 0}, End: graph.Position{Line: 0, Character: 1}}
	transient := transientstructural.Result{TargetID: "root", Qualification: transientstructural.Qualification{SessionID: "s", Generation: 1, PositionEncoding: "utf-16"}, Analysis: transientstructural.AnalysisResult{Nodes: []transientstructural.NodeFact{{ID: "root", URI: uri, Range: r}, {ID: "other", URI: uris["b.go"], Range: r}}}, SourceSupply: &sessionruntime.DocumentSupply{Classification: "LSP_SUPPLIED", SessionID: "s", Generation: 1, URI: uri, DocumentVersion: 1, Method: "textDocument/didOpen", Content: []byte("A\n")}}
	delegate := structuralContextDelegateFunc(func(_ context.Context, _ operation.Request) (operation.Result, *operation.Failure) {
		return operation.Result{Artifact: []byte(validStructuralV2), Value: structuralContextProjectionInput{Transient: transient, WorkspaceRoot: workspaceRoot}}, nil
	})
	e := &unifiedStructuralContextV2Executor{exact: delegate, symbol: delegate}
	for _, tc := range []struct {
		name, mode, body  string
		selected, omitted int
		wantBody          bool
		wantFailure       bool
	}{
		{name: "projected-body-unavailable", mode: "PROJECTED", body: "INCLUDE", wantFailure: true},
		{name: "target-metadata", mode: "TARGET", body: "OMIT", selected: 1, omitted: 0, wantBody: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			projection := `{"mode":"` + tc.mode + `","body":"` + tc.body + `","include_relation_occurrences":false,"include_ancillary":false,"display_range_policy":"EXACT_EVIDENCE","limits":{"max_objects":2,"max_ranges":2,"max_source_bytes":2,"max_work":2,"max_response_bytes":100000,"max_additional_documents":0,"max_document_requests":1,"max_document_bytes":2,"max_total_document_bytes":2,"max_document_messages":1,"max_document_acquisition_work":1,"max_display_resolution_work":1},"privacy_policy_id":"public"}`
			got, failure := e.Execute(context.Background(), operation.Request{Name: operation.Name("structural_context_v2"), Input: json.RawMessage(`{"uri":"file:///repo/a.go","line":0,"character":0,"projection":` + projection + `}`)})
			if tc.wantFailure {
				if failure == nil || failure.Code != "SOURCE_PROJECTION_FAILED" || len(got.Artifact) != 0 {
					t.Fatalf("ASSERT_LIVE_PROJECTION_UNAVAILABLE_NO_PARTIAL_RESULT: artifact=%d failure=%v", len(got.Artifact), failure)
				}
				return
			}
			if failure != nil {
				t.Fatalf("ASSERT_LIVE_PROJECTION_BOUNDARY_%s: %v", tc.name, failure)
			}
			var envelope struct {
				Projection struct {
					Units []struct {
						Body string `json:"body"`
					} `json:"units"`
					Accounting struct{ Selected, Omitted int } `json:"accounting"`
					Omissions  []struct {
						Cause string `json:"cause"`
					} `json:"omissions"`
				} `json:"projection"`
			}
			if err := json.Unmarshal(got.Artifact, &envelope); err != nil {
				t.Fatal(err)
			}
			p := envelope.Projection
			if p.Accounting.Selected != tc.selected || p.Accounting.Omitted != tc.omitted || len(p.Units) != tc.selected {
				t.Fatalf("ASSERT_LIVE_PROJECTION_BOUNDARY_%s: %+v", tc.name, p)
			}
			if tc.omitted == 1 && (len(p.Omissions) != 1 || p.Omissions[0].Cause != "SOURCE_UNAVAILABLE") {
				t.Fatalf("ASSERT_LIVE_PROJECTION_UNAVAILABLE_NO_FALLBACK: %+v", p.Omissions)
			}
			hasBody := len(p.Units) == 1 && p.Units[0].Body == "A"
			if hasBody != tc.wantBody {
				t.Fatalf("ASSERT_LIVE_PROJECTION_BODY_OPT_IN_%s: %+v", tc.name, p.Units)
			}
		})
	}
}

func TestUnifiedStructuralContextV2ExecutorBoundsCompleteUnifiedResponse(t *testing.T) {
	workspaceRoot, uris := projectionWorkspace(t, "a.go")
	uri := uris["a.go"]
	r := graph.Range{Start: graph.Position{Line: 0, Character: 0}, End: graph.Position{Line: 0, Character: 1}}
	transient := transientstructural.Result{TargetID: "root", Qualification: transientstructural.Qualification{SessionID: "s", Generation: 1, PositionEncoding: "utf-16"}, Analysis: transientstructural.AnalysisResult{Nodes: []transientstructural.NodeFact{{ID: "root", URI: uri, Range: r}}}, SourceSupply: &sessionruntime.DocumentSupply{Classification: "LSP_SUPPLIED", SessionID: "s", Generation: 1, URI: uri, DocumentVersion: 1, Method: "textDocument/didOpen", Content: []byte("A\n")}}
	delegate := structuralContextDelegateFunc(func(_ context.Context, _ operation.Request) (operation.Result, *operation.Failure) {
		return operation.Result{Artifact: []byte(validStructuralV2), Value: structuralContextProjectionInput{Transient: transient, WorkspaceRoot: workspaceRoot}}, nil
	})
	e := &unifiedStructuralContextV2Executor{exact: delegate, symbol: delegate}
	projection := `{"mode":"TARGET","body":"INCLUDE","include_relation_occurrences":false,"include_ancillary":false,"display_range_policy":"EXACT_EVIDENCE","limits":{"max_objects":1,"max_ranges":1,"max_source_bytes":2,"max_work":1,"max_response_bytes":2000,"max_additional_documents":0,"max_document_requests":1,"max_document_bytes":2,"max_total_document_bytes":2,"max_document_messages":1,"max_document_acquisition_work":1,"max_display_resolution_work":1},"privacy_policy_id":"public"}`
	got, failure := e.Execute(context.Background(), operation.Request{Name: operation.Name("structural_context_v2"), Input: json.RawMessage(`{"uri":"file:///repo/a.go","line":0,"character":0,"projection":` + projection + `}`)})
	if failure == nil || failure.Code != "SOURCE_PROJECTION_FAILED" || len(got.Artifact) != 0 {
		t.Fatalf("ASSERT_UNIFIED_CONTEXT_COMPLETE_RESPONSE_LIMIT: bytes=%d failure=%+v", len(got.Artifact), failure)
	}
}

func TestUnifiedStructuralContextV2ExecutorPagesLiveProjectionV3(t *testing.T) {
	workspaceRoot, uris := projectionWorkspace(t, "a.go")
	uri := uris["a.go"]
	r := graph.Range{Start: graph.Position{Line: 0, Character: 0}, End: graph.Position{Line: 0, Character: 1}}
	transient := transientstructural.Result{TargetID: "root", Qualification: transientstructural.Qualification{SessionID: "s", Generation: 1, PositionEncoding: "utf-16"}, Analysis: transientstructural.AnalysisResult{Nodes: []transientstructural.NodeFact{{ID: "root", URI: uri, Range: r}}}, SourceSupply: &sessionruntime.DocumentSupply{Classification: "LSP_SUPPLIED", SessionID: "s", Generation: 1, URI: uri, DocumentVersion: 1, Method: "textDocument/didOpen", Content: []byte("A\n")}}
	delegate := structuralContextDelegateFunc(func(_ context.Context, _ operation.Request) (operation.Result, *operation.Failure) {
		return operation.Result{Artifact: []byte(validStructuralV2), Value: structuralContextProjectionInput{Transient: transient, WorkspaceRoot: workspaceRoot}}, nil
	})
	e := &unifiedStructuralContextV2Executor{exact: delegate, symbol: delegate}
	projection := map[string]any{
		"mode": "TARGET", "body": "INCLUDE", "include_relation_occurrences": false, "include_ancillary": false,
		"display_range_policy": "FULL_DEFINITION", "privacy_policy_id": "public",
		"limits": map[string]any{"max_objects": 10, "max_ranges": 10, "max_source_bytes": 2, "max_work": 10, "max_response_bytes": 100000, "max_additional_documents": 0, "max_document_requests": 1, "max_document_bytes": 2, "max_total_document_bytes": 2, "max_document_messages": 1, "max_document_acquisition_work": 1, "max_display_resolution_work": 1},
		"paging": map[string]any{"max_page_bytes": 1200, "max_pages": 20, "max_response_bytes": 100000},
	}
	invoke := func() map[string]any {
		t.Helper()
		projectionRaw, _ := json.Marshal(projection)
		input := []byte(`{"uri":"file:///repo/a.go","line":0,"character":0,"projection":` + string(projectionRaw) + `}`)
		got, failure := e.Execute(context.Background(), operation.Request{Name: operation.Name("structural_context_v2"), Input: input})
		if failure != nil {
			t.Fatalf("ASSERT_UNIFIED_CONTEXT_V3_PAGING: %v", failure)
		}
		if err := mcpcontract.ValidateJSON(mcpcontract.UnifiedStructuralContextResultV3ID, got.Artifact); err != nil {
			t.Fatalf("ASSERT_UNIFIED_CONTEXT_V3_PAGING: %v\n%s", err, got.Artifact)
		}
		var envelope map[string]any
		_ = json.Unmarshal(got.Artifact, &envelope)
		return envelope
	}
	first := invoke()
	page := first["projection"].(map[string]any)
	if page["complete"] == true || page["next_cursor"] == nil {
		t.Fatalf("ASSERT_UNIFIED_CONTEXT_V3_FIRST_PAGE: %v", page)
	}
	projection["paging"].(map[string]any)["cursor"] = page["next_cursor"]
	second := invoke()
	secondPage := second["projection"].(map[string]any)
	if secondPage["accounting"].(map[string]any)["pages"].(float64) != 2 {
		t.Fatalf("ASSERT_UNIFIED_CONTEXT_V3_CONTINUATION: %v", secondPage)
	}
}

func TestUnifiedStructuralContextV2ExecutorPreservesDelegateFailure(t *testing.T) {
	want := &operation.Failure{Code: "TARGET_NOT_FOUND", Diagnostics: []string{"bounded"}}
	delegate := structuralContextDelegateFunc(func(_ context.Context, _ operation.Request) (operation.Result, *operation.Failure) {
		return operation.Result{}, want
	})
	e := &unifiedStructuralContextV2Executor{exact: delegate, symbol: delegate}
	_, got := e.Execute(context.Background(), operation.Request{Name: operation.Name("structural_context_v2"), Input: json.RawMessage(`{"uri":"file:///repo/a.go","line":0,"character":0}`)})
	if got != want {
		t.Fatalf("ASSERT_UNIFIED_CONTEXT_RUNTIME_FAILURE_PASSTHROUGH: got=%p want=%p", got, want)
	}
}
