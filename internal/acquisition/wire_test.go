package acquisition

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/lsp"
)

func TestWireClientContract(t *testing.T) {
	a := item("a", 0)
	r := request(a)
	r.Limits.MaxMessages = 17
	count := 0
	client := NewWireClient(func(ctx context.Context, w WireRequest) (json.RawMessage, error) {
		count++
		if w.Context != r.Context || w.MaxMessages != 17 || w.MaxBytes > r.Limits.MaxResponseBytes {
			t.Fatal("wire exact generation and budgets")
		}
		if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > r.Limits.RequestTimeout {
			t.Fatal("wire deadline")
		}
		switch w.Method {
		case "textDocument/prepareCallHierarchy":
			return json.Marshal([]lsp.CallHierarchyItem{a})
		default:
			var params struct {
				Item lsp.CallHierarchyItem `json:"item"`
			}
			if e := json.Unmarshal(w.Params, &params); e != nil {
				t.Fatal(e)
			}
			if string(params.Item.Data) != string(a.Data) {
				t.Fatal("prepared opaque data not reused")
			}
			return json.RawMessage("null"), nil
		}
	})
	got, e := Acquire(context.Background(), client, r)
	if e != nil {
		t.Fatal(e)
	}
	if count != 3 || got.Targets[0].Outgoing.Status != SuccessEmpty || got.Targets[0].Incoming.Status != SuccessEmpty {
		t.Fatal("wire null should be empty for both directions")
	}
}
func TestWireRejectsMalformedCoordinates(t *testing.T) {
	for _, name := range []string{"range-missing", "coordinate-missing", "negative", "null-range", "oversized", "flat-symbol", "bad-json", "nested", "valid-symbol-tags"} {
		t.Run(name, func(t *testing.T) {
			a := item("a", 0)
			r := request(a)
			data, _ := json.Marshal([]lsp.CallHierarchyItem{a})
			want := ResolutionFailed
			switch name {
			case "range-missing":
				data = []byte(`[{"name":"a","kind":12,"uri":"file:///a.go"}]`)
			case "coordinate-missing":
				data = []byte(`[{"name":"a","kind":12,"uri":"file:///a.go","range":{"start":{},"end":{}},"selectionRange":{"start":{},"end":{}}}]`)
			case "negative":
				data = []byte(strings.Replace(string(data), `"line":0`, `"line":-1`, 1))
			case "null-range":
				data = []byte(`[{"name":"a","kind":12,"uri":"file:///a.go","range":null,"selectionRange":null}]`)
			case "oversized":
				data = []byte(strings.Repeat(" ", r.Limits.MaxResponseBytes+1))
			case "flat-symbol":
				r.Root.Locator = Locator{URI: a.URI, Symbol: a.Name}
				data = []byte(`[{"name":"a","kind":12,"location":{"uri":"file:///a.go","range":{"start":{"line":0,"character":2},"end":{"line":0,"character":6}}}}]`)
			case "bad-json":
				data = []byte(`[`)
			case "nested":
				data = []byte(strings.Repeat("[", 130) + strings.Repeat("]", 130))
			case "valid-symbol-tags":
				r.Root.Locator = Locator{URI: a.URI, Symbol: a.Name}
				want = Resolved
			}
			client := NewWireClient(func(_ context.Context, w WireRequest) (json.RawMessage, error) {
				if name == "valid-symbol-tags" {
					if w.Method == "textDocument/documentSymbol" {
						return json.Marshal([]wireSymbol{{Name: a.Name, Kind: a.Kind, Tags: []int{1}, Deprecated: true, Range: a.Range, SelectionRange: a.SelectionRange}})
					}
					if w.Method != "textDocument/prepareCallHierarchy" {
						return json.RawMessage("null"), nil
					}
				}
				return data, nil
			})
			got, e := Acquire(context.Background(), client, r)
			if e != nil {
				t.Fatal(e)
			}
			if got.Targets[0].Resolution.Status != want {
				t.Fatalf("wire shape %s: %+v", name, got.Targets[0].Resolution)
			}
		})
	}
}
