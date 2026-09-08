package retainedcalls

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/lsp"
)

// Initial RED used the existing present-but-wrong V1 exporter, not an undefined API.
var exportV2UnderTest = ExportV2

func fixtureV2(t *testing.T, mode acquisition.Mode, variant string) ([]byte, acquisition.Result) {
	t.Helper()
	root := t.TempDir()
	uri := func(name string) string {
		return (&url.URL{Scheme: "file", Path: filepath.Join(root, name+".go")}).String()
	}
	for _, name := range []string{"a", "b", "c", "d"} {
		if err := os.WriteFile(filepath.Join(root, name+".go"), []byte("package p\n// exact \r\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	item := func(name string) lsp.CallHierarchyItem {
		return lsp.CallHierarchyItem{Name: name, Kind: 12, URI: uri(name), Range: lsp.Range{End: lsp.Position{Line: 9}}, SelectionRange: lsp.Range{End: lsp.Position{Character: 1}}, Data: json.RawMessage(`{"large":9007199254740993,"exponent":1e+02}`)}
	}
	line, char := uint32(0), uint32(0)
	target := func(id, name string) acquisition.Target {
		return acquisition.Target{ID: id, Locator: acquisition.Locator{URI: uri(name), Line: &line, Character: &char}, DownDepth: 3, UpDepth: 3}
	}
	r := acquisition.Request{Mode: mode, Context: acquisition.AcquisitionContext{ID: "context", SessionID: "session", Generation: 9007199254740993, PositionEncoding: "utf-16"}, Root: target("root", "a"), RequiredTargets: []acquisition.Target{target("required", "c"), target("alias", "a"), target("disconnected", "d")}, Limits: acquisition.Limits{MaxNodes: 10000, MaxRequests: 100, MaxEvidenceBytes: 1 << 20, MaxPathWork: 10000, Timeout: time.Second, RequestTimeout: time.Second, MaxResponseBytes: 1 << 20, MaxMessages: 64}}
	if variant == "supplies" {
		other := uint32(1)
		r.RequiredTargets[1].Locator.Character = &other
	}
	if variant == "budget" {
		r.Limits.MaxEvidenceBytes = 1
	}
	if variant == "unadmitted" {
		r.Limits.MaxNodes = 1
	}
	if variant == "ambiguous" {
		r.Root.Locator.Symbol = "same"
		r.Root.Locator.Line = nil
		r.Root.Locator.Character = nil
	}
	var client acquisition.Client = acquisition.NewWireClient(func(_ context.Context, q acquisition.WireRequest) (json.RawMessage, error) {
		if q.Method == "textDocument/documentSymbol" && variant == "ambiguous" {
			s := lsp.DocumentSymbol{Name: "same", Kind: 12, Range: item("a").Range, SelectionRange: item("a").SelectionRange}
			return json.Marshal([]lsp.DocumentSymbol{s, s})
		}
		if q.Method == "textDocument/prepareCallHierarchy" {
			if variant == "failed" {
				return nil, errors.New("fixture root failure")
			}
			var p lsp.PrepareCallHierarchyParams
			if err := json.Unmarshal(q.Params, &p); err != nil {
				return nil, err
			}
			if variant == "missing" {
				return json.RawMessage(`[]`), nil
			}
			name := "d"
			for _, n := range []string{"a", "b", "c"} {
				if p.TextDocument.URI == uri(n) {
					name = n
				}
			}
			return json.Marshal([]lsp.CallHierarchyItem{item(name)})
		}
		var p struct {
			Item lsp.CallHierarchyItem `json:"item"`
		}
		_ = json.Unmarshal(q.Params, &p)
		sites := []lsp.Range{{Start: lsp.Position{Line: 1}, End: lsp.Position{Line: 1, Character: 1}}, {Start: lsp.Position{Line: 2}, End: lsp.Position{Line: 2, Character: 1}}}
		if variant == "zero" {
			sites = []lsp.Range{}
		}
		if variant == "empty" {
			return json.RawMessage(`[]`), nil
		}
		if q.Method == "callHierarchy/outgoingCalls" {
			next := ""
			if p.Item.Name == "a" {
				next = "b"
			}
			if p.Item.Name == "b" {
				next = "c"
			}
			if mode == acquisition.Incoming {
				next = ""
				if p.Item.Name == "c" {
					next = "b"
				}
				if p.Item.Name == "b" {
					next = "a"
				}
			}
			if next != "" {
				rows := []lsp.CallHierarchyOutgoingCall{{To: item(next), FromRanges: sites}}
				if variant == "partial" {
					bad := item(next)
					bad.SelectionRange.End.Line = 100
					rows = append(rows, lsp.CallHierarchyOutgoingCall{To: bad, FromRanges: sites})
				}
				return json.Marshal(rows)
			}
		}
		if q.Method == "callHierarchy/incomingCalls" {
			prev := ""
			if p.Item.Name == "c" {
				prev = "b"
			}
			if p.Item.Name == "b" {
				prev = "a"
			}
			if mode == acquisition.Incoming {
				prev = ""
				if p.Item.Name == "a" {
					prev = "b"
				}
				if p.Item.Name == "b" {
					prev = "c"
				}
			}
			if prev != "" {
				return json.Marshal([]lsp.CallHierarchyIncomingCall{{From: item(prev), FromRanges: sites}})
			}
		}
		return json.RawMessage(`[]`), nil
	})
	if variant == "supplies" {
		client = &supplyClientV2{Client: client, versions: map[string]int{}}
	}
	result, err := acquisition.Acquire(context.Background(), client, r)
	if err != nil {
		t.Fatal(err)
	}
	input, err := graphprovenance.CaptureV2(context.Background(), result, root)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	return input, result
}

func TestRetainedV2AdmissionVertical(t *testing.T) {
	input, _ := fixtureV2(t, acquisition.Slice, "")
	if _, err := graphprovenance.ValidateFor(input, graphprovenance.Family, "v2"); err != nil {
		t.Fatal(err)
	}
	raw, err := exportV2UnderTest(input)
	if err != nil {
		t.Fatalf("ASSERT_RETAINED_V2_VERTICAL: export admitted source-free V2: %v", err)
	}
	if !bytes.Contains(raw, []byte(`"schema_version":"lsp-trace.retained-calls.v2"`)) {
		t.Fatal("ASSERT_RETAINED_V2_VERTICAL: wrong version")
	}
	t.Log("ASSERT_RETAINED_V2_VERTICAL: PASS")
}
