package graphprovenance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/lsp"
)

func TestV2ResponseRangeContext(t *testing.T) {
	root := t.TempDir()
	aURI := (&url.URL{Scheme: "file", Path: filepath.Join(root, "a.go")}).String()
	bURI := (&url.URL{Scheme: "file", Path: filepath.Join(root, "b.go")}).String()
	span := lsp.Range{End: lsp.Position{Character: 1}}
	a := lsp.CallHierarchyItem{Name: "A", Kind: 12, URI: aURI, Range: span, SelectionRange: span}
	b := lsp.CallHierarchyItem{Name: "B", Kind: 12, URI: bURI, Range: span, SelectionRange: span}
	client := acquisition.NewWireClient(func(_ context.Context, q acquisition.WireRequest) (json.RawMessage, error) {
		switch q.Method {
		case "textDocument/documentSymbol":
			return json.Marshal([]lsp.DocumentSymbol{{Name: "A", Kind: 12, Range: span, SelectionRange: span}})
		case "textDocument/prepareCallHierarchy":
			var p struct {
				TextDocument struct {
					URI string `json:"uri"`
				} `json:"textDocument"`
			}
			_ = json.Unmarshal(q.Params, &p)
			if p.TextDocument.URI == aURI {
				return json.Marshal([]lsp.CallHierarchyItem{a})
			}
			return json.Marshal([]lsp.CallHierarchyItem{b})
		case "callHierarchy/outgoingCalls", "callHierarchy/incomingCalls":
			var p struct {
				Item lsp.CallHierarchyItem `json:"item"`
			}
			_ = json.Unmarshal(q.Params, &p)
			if q.Method == "callHierarchy/outgoingCalls" && p.Item.URI == aURI {
				return json.Marshal([]any{map[string]any{"to": b, "fromRanges": []lsp.Range{span}}})
			}
			if q.Method == "callHierarchy/incomingCalls" && p.Item.URI == bURI {
				return json.Marshal([]any{map[string]any{"from": a, "fromRanges": []lsp.Range{span}}})
			}
		}
		return json.RawMessage(`[]`), nil
	})
	zero := uint32(0)
	request := acquisition.Request{Mode: acquisition.Slice, Context: acquisition.AcquisitionContext{ID: "ranges", SessionID: "session", Generation: 1, PositionEncoding: "utf-16"}, Root: acquisition.Target{ID: "root", Locator: acquisition.Locator{URI: aURI, Symbol: "A"}, DownDepth: 1, UpDepth: 1}, RequiredTargets: []acquisition.Target{{ID: "b", Locator: acquisition.Locator{URI: bURI, Line: &zero, Character: &zero}, DownDepth: 1, UpDepth: 1}}, Limits: acquisition.Limits{MaxNodes: 100, MaxRequests: 100, MaxEvidenceBytes: 1 << 20, MaxPathWork: 1000, Timeout: time.Second, RequestTimeout: time.Second, MaxResponseBytes: 1 << 20, MaxMessages: 64}}
	result, err := acquisition.Acquire(context.Background(), client, request)
	if err != nil {
		t.Fatal(err)
	}
	bindings, err := CensusV2(result)
	if err != nil {
		t.Fatal(err)
	}
	byPointer := map[string]Binding{}
	for _, binding := range bindings {
		byPointer[binding.Pointer] = binding
	}
	checked := map[string]bool{}
	for i, record := range result.Requests {
		pointer := ""
		switch record.Method {
		case "textDocument/documentSymbol":
			pointer = fmt.Sprintf("/acquisition/requests/%d/response/0/range", i)
		case "callHierarchy/outgoingCalls", "callHierarchy/incomingCalls":
			var rows []json.RawMessage
			_ = json.Unmarshal(record.Response, &rows)
			if len(rows) == 0 {
				continue
			}
			pointer = fmt.Sprintf("/acquisition/requests/%d/response/0/fromRanges/0", i)
		default:
			continue
		}
		got, ok := byPointer[pointer]
		if !ok || got.URI != aURI || got.Attribution != "SOURCE" {
			t.Fatalf("ASSERT_V2_RESPONSE_RANGE_CONTEXT: %s: %+v; retained response=%s", record.Method, got, record.Response)
		}
		checked[record.Method] = true
	}
	if len(checked) != 3 {
		t.Fatal("fixture did not cover document symbols and both call directions")
	}
	if len(result.EdgeObservations) == 0 {
		t.Fatal("missing edge observation fixture")
	}
	for i, observation := range result.EdgeObservations {
		for j := range observation.CallSites {
			binding := byPointer[fmt.Sprintf("/acquisition/edge_observations/%d/call_sites/%d", i, j)]
			if binding.URI != aURI || binding.Attribution != "SOURCE" {
				t.Fatal("ASSERT_V2_EDGE_OBSERVATION_RANGE_CONTEXT", binding)
			}
		}
	}
	raw, err := CaptureV2(context.Background(), result, root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateFor(raw, Family, "v2"); err != nil {
		t.Fatal(err)
	}
	t.Log("ASSERT_V2_RESPONSE_RANGE_CONTEXT: PASS; ASSERT_V2_EDGE_OBSERVATION_RANGE_CONTEXT: PASS")
}
