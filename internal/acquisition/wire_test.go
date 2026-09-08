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
	for _, name := range []string{"range-missing", "coordinate-missing", "negative", "null-range", "oversized", "bad-json", "nested", "valid-symbol-tags"} {
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

func TestWireDocumentSymbolUnion(t *testing.T) {
	a := item("a", 0)
	a.SelectionRange = a.Range
	flat := func(name, uri string) map[string]any {
		return map[string]any{"name": name, "kind": a.Kind, "location": map[string]any{"uri": uri, "range": a.Range}}
	}
	hierarchical := func(name string) map[string]any {
		return map[string]any{"name": name, "kind": a.Kind, "range": a.Range, "selectionRange": a.SelectionRange}
	}
	for _, tc := range []struct {
		name       string
		symbols    []map[string]any
		wantStatus ResolutionStatus
		wantCalls  int
	}{
		{name: "same-uri", symbols: []map[string]any{flat(a.Name, a.URI)}, wantStatus: Resolved, wantCalls: 4},
		{name: "foreign-uri", symbols: []map[string]any{flat(a.Name, "file:///other.go")}, wantStatus: Missing, wantCalls: 1},
		{name: "duplicate", symbols: []map[string]any{flat(a.Name, a.URI), flat(a.Name, a.URI)}, wantStatus: Ambiguous, wantCalls: 1},
		{name: "missing", symbols: []map[string]any{flat("other", a.URI)}, wantStatus: Missing, wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := request(a)
			r.Root.Locator = Locator{URI: a.URI, Symbol: a.Name}
			calls := 0
			client := NewWireClient(func(_ context.Context, w WireRequest) (json.RawMessage, error) {
				calls++
				switch w.Method {
				case "textDocument/documentSymbol":
					return json.Marshal(tc.symbols)
				case "textDocument/prepareCallHierarchy":
					return json.Marshal([]lsp.CallHierarchyItem{a})
				default:
					return json.RawMessage("null"), nil
				}
			})
			got, err := Acquire(context.Background(), client, r)
			if err != nil {
				t.Fatal(err)
			}
			if resolution := got.Targets[0].Resolution; resolution.Status != tc.wantStatus {
				t.Fatalf("ASSERT_FLAT_SYMBOL_%s_STATUS: %+v", tc.name, resolution)
			}
			if calls != tc.wantCalls {
				t.Fatalf("ASSERT_FLAT_SYMBOL_%s_SHARED_REQUESTS: got %d want %d", tc.name, calls, tc.wantCalls)
			}
		})
	}

	var malformedReason string
	for _, tc := range []struct {
		name    string
		symbols []map[string]any
	}{
		{name: "mixed-hierarchical-flat", symbols: []map[string]any{hierarchical(a.Name), flat(a.Name, a.URI)}},
		{name: "mixed-flat-hierarchical", symbols: []map[string]any{flat(a.Name, a.URI), hierarchical(a.Name)}},
		{name: "mixed-distinct-names", symbols: []map[string]any{hierarchical(a.Name), flat("other", a.URI)}},
		{name: "mixed-ambiguous-name", symbols: []map[string]any{hierarchical(a.Name), flat(a.Name, a.URI), flat(a.Name, a.URI)}},
		{name: "mixed-foreign-flat", symbols: []map[string]any{hierarchical(a.Name), flat(a.Name, "file:///other.go")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := request(a)
			r.Root.Locator = Locator{URI: a.URI, Symbol: a.Name}
			calls := 0
			client := NewWireClient(func(_ context.Context, _ WireRequest) (json.RawMessage, error) {
				calls++
				return json.Marshal(tc.symbols)
			})
			got, err := Acquire(context.Background(), client, r)
			if err != nil {
				t.Fatal(err)
			}
			resolution := got.Targets[0].Resolution
			if resolution.Status != ResolutionFailed {
				t.Fatalf("ASSERT_DOCUMENT_SYMBOL_MIXED_UNION_CLOSED_%s: %+v", tc.name, resolution)
			}
			if malformedReason == "" {
				malformedReason = resolution.Reason
			} else if resolution.Reason != malformedReason {
				t.Fatalf("ASSERT_DOCUMENT_SYMBOL_MIXED_UNION_SAME_REASON_%s: got %q want %q", tc.name, resolution.Reason, malformedReason)
			}
			if calls != 1 {
				t.Fatalf("ASSERT_DOCUMENT_SYMBOL_MIXED_UNION_SHARED_REQUESTS_%s: got %d want 1", tc.name, calls)
			}
		})
	}
}
