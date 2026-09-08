package graphprovenance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/lsp"
	"lsp-trace/sessionruntime"
)

func suppliedV2Fixture(t *testing.T, mode acquisition.Mode, connected, duplicate bool) (acquisition.Result, string) {
	base, root := coordinatorV2Fixture(t, nil)
	req := base.Request
	req.Mode = mode
	one := uint32(1)
	req.RequiredTargets[0].Locator.Line = &one
	req.RequiredTargets[0].ID = "b"
	uri := req.Root.Locator.URI
	item := func(line uint32) lsp.CallHierarchyItem {
		return lsp.CallHierarchyItem{Name: fmt.Sprintf("F%d", line), Kind: 12, URI: uri, Range: lsp.Range{Start: lsp.Position{Line: line}, End: lsp.Position{Line: line, Character: 10}}, SelectionRange: lsp.Range{Start: lsp.Position{Line: line}, End: lsp.Position{Line: line, Character: 1}}}
	}
	wire := acquisition.NewWireClient(func(_ context.Context, w acquisition.WireRequest) (json.RawMessage, error) {
		if w.Method == "textDocument/prepareCallHierarchy" {
			var p lsp.PrepareCallHierarchyParams
			_ = json.Unmarshal(w.Params, &p)
			return json.Marshal([]lsp.CallHierarchyItem{item(p.Position.Line)})
		}
		var p struct {
			Item lsp.CallHierarchyItem `json:"item"`
		}
		_ = json.Unmarshal(w.Params, &p)
		if connected {
			// SLICE asks root -> required; INCOMING asks required -> root.
			caller, callee := uint32(0), uint32(1)
			if mode == acquisition.Incoming {
				caller, callee = callee, caller
			}
			if w.Method == "callHierarchy/outgoingCalls" && p.Item.Range.Start.Line == caller {
				return json.Marshal([]lsp.CallHierarchyOutgoingCall{{To: item(callee), FromRanges: []lsp.Range{item(caller).SelectionRange}}})
			}
			if w.Method == "callHierarchy/incomingCalls" && p.Item.Range.Start.Line == callee {
				return json.Marshal([]lsp.CallHierarchyIncomingCall{{From: item(caller), FromRanges: []lsp.Range{item(caller).SelectionRange}}})
			}
		}
		return json.RawMessage(`[]`), nil
	})
	count := 0
	client := acquisition.WithDocumentSupply(wire, func(_ context.Context, a acquisition.AcquisitionContext, l acquisition.Locator) (acquisition.Supply, error) {
		count++
		version := count
		if duplicate {
			version = 1
		}
		content := []byte(fmt.Sprintf("source version %d", count))
		method := "textDocument/didOpen"
		params := map[string]any{"textDocument": map[string]any{"uri": uri, "languageId": "go", "version": version, "text": string(content)}}
		if version > 1 {
			method = "textDocument/didChange"
			params = map[string]any{"textDocument": map[string]any{"uri": uri, "version": version}, "contentChanges": []any{map[string]any{"text": string(content)}}}
		}
		raw, _ := json.Marshal(params)
		observation, _ := json.Marshal(sessionruntime.DocumentSupply{Classification: Supplied, SessionID: a.SessionID, Generation: a.Generation, URI: l.URI, DocumentVersion: version, Method: method, Content: content, Params: raw})
		return acquisition.Supply{URI: l.URI, LanguageID: "go", Observation: observation}, nil
	})
	result, err := acquisition.Acquire(context.Background(), client, req)
	if err != nil {
		t.Fatal(err)
	}
	return result, root
}

func TestV2ActualSupplyVersionsAndOfflineConnections(t *testing.T) {
	for _, mode := range []acquisition.Mode{acquisition.Slice, acquisition.Incoming} {
		for _, connected := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/%v", mode, connected), func(t *testing.T) {
				r, root := suppliedV2Fixture(t, mode, connected, false)
				expectedStatus := "NOT_FOUND_IN_RETAINED_GRAPH"
				if connected {
					expectedStatus = "FOUND"
				}
				if r.Targets[1].Connection.Status != expectedStatus {
					t.Fatal("independent endpoint orientation oracle")
				}
				if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("later observation"), 0600); err != nil {
					t.Fatal(err)
				}
				raw, err := CaptureV2(context.Background(), r, root)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.RemoveAll(root); err != nil {
					t.Fatal(err)
				}
				var e EvidenceV2
				if err := json.Unmarshal(raw, &e); err != nil {
					t.Fatal(err)
				}
				if len(e.Supplies) != 2 || len(e.Captures) != 1 || e.Supplies[0].Receipt.Supply.Version != 1 || e.Supplies[1].Receipt.Supply.Version != 2 || e.Supplies[0].RequestID == e.Supplies[1].RequestID {
					t.Fatal("ASSERT_V2_ACTUAL_DISTINCT_SUPPLIES")
				}
				if !bytes.Equal(e.Captures[0].Content, []byte("later observation")) || bytes.Equal(e.Captures[0].Content, e.Supplies[1].Receipt.Content) {
					t.Fatal("ASSERT_V2_POST_CAPTURE_DISTINCT")
				}
				if _, err := ValidateFor(raw, Family, "v2"); err != nil {
					t.Fatal(err)
				}
				got, _ := json.Marshal(e.Acquisition)
				want, _ := json.Marshal(r)
				if !bytes.Equal(got, want) {
					t.Fatal("ASSERT_V2_CONNECTED_RESULT_ROUNDTRIP")
				}
				e.Supplies[1].Receipt.Supply.Version = 1
				bad, _ := json.Marshal(e)
				if _, err := ValidateFor(bad, Family, "v2"); err == nil {
					t.Fatal("ASSERT_V2_DUPLICATE_RECEIPT_VERSION_REJECT")
				}
				t.Log("ASSERT_V2_ACTUAL_DISTINCT_SUPPLIES: PASS; ASSERT_V2_POST_CAPTURE_DISTINCT: PASS; ASSERT_V2_CONNECTED_RESULT_ROUNDTRIP: PASS; ASSERT_V2_DUPLICATE_RECEIPT_VERSION_REJECT: PASS")
			})
		}
	}
	r, root := suppliedV2Fixture(t, acquisition.Slice, true, true)
	if _, err := CaptureV2(context.Background(), r, root); err == nil {
		t.Fatal("ASSERT_V2_DUPLICATE_ACTUAL_VERSION_REJECT")
	}
}
