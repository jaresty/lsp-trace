//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
)

func reviewAdapter(t *testing.T, response func(lspwire.Message) json.RawMessage) (*scopeRuntimeAdapter, *scopeAdapterChild) {
	t.Helper()
	restart, m, id, child := adapterFixture(t)
	a, _, err := newScopeRuntimeAdapter(context.Background(), restart, m, id)
	if err != nil {
		t.Fatal(err)
	}
	child.mu.Lock()
	child.response = response
	child.mu.Unlock()
	return a, child
}

const reviewZeroRange = `{"start":{"line":0,"character":0},"end":{"line":0,"character":0}}`

func reviewItem(uri string) string {
	return fmt.Sprintf(`{"name":"Query","kind":12,"uri":%q,"range":%s,"selectionRange":%s}`, uri, reviewZeroRange, reviewZeroRange)
}
func reviewSymbol(extra string) string {
	return fmt.Sprintf(`{"name":"Query","kind":12,"range":%s,"selectionRange":%s%s}`, reviewZeroRange, reviewZeroRange, extra)
}
func reviewNativeRequest(a *scopeRuntimeAdapter) acquisition.Request {
	zero := uint32(0)
	return acquisition.Request{Mode: acquisition.Incoming, Context: acquisition.AcquisitionContext{ID: "review", SessionID: a.identity.SessionID, Generation: a.identity.Generation, PositionEncoding: "utf-16"}, Root: acquisition.Target{ID: "root", Locator: acquisition.Locator{URI: a.files[0].URI, LanguageID: "go", Line: &zero, Character: &zero}, UpDepth: 1}, Limits: acquisition.Limits{MaxNodes: 8, MaxRequests: 8, MaxEvidenceBytes: 1 << 20, MaxPathWork: 100, Timeout: time.Second, RequestTimeout: time.Second, MaxResponseBytes: scopeAdapterResponse, MaxMessages: 64}}
}
func TestScopeAdapterReviewExternalFlat(t *testing.T) {
	a, child := reviewAdapter(t, func(msg lspwire.Message) json.RawMessage {
		return json.RawMessage(fmt.Sprintf(`[{"name":"Query","kind":12,"location":{"uri":"file:///external.go","range":%s}}]`, reviewZeroRange))
	})
	rows, err := a.DocumentSymbols(context.Background(), lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: a.files[0].URI}})
	if err != nil || len(rows) != 0 {
		t.Fatalf("external flat relabeled as local: %d %v", len(rows), err)
	}
	d, err := a.Discover(context.Background(), a.identity)
	if err != nil || d.Complete || len(d.Targets) != 0 {
		t.Fatal("external nominated", d, err)
	}
	known := false
	for _, o := range d.Observations {
		if o.Known && o.Omitted != nil && *o.Omitted == 4 {
			known = true
		}
	}
	if !known {
		t.Fatal("external omissions not reported", d.Observations)
	}
	if len(semanticMessages(child)) != 5 {
		t.Fatal("external generated preparation")
	}
}
func TestScopeAdapterReviewMalformedTerminal(t *testing.T) {
	for _, kind := range []string{"prep-empty", "missing-coordinate", "null-coordinate", "null-range", "reverse-range", "unknown", "symbol-tags", "symbol-deprecated", "symbol-unknown", "duplicate", "duplicate-native", "native-empty", "native-byte-limit"} {
		t.Run(kind, func(t *testing.T) {
			a, child := reviewAdapter(t, func(msg lspwire.Message) json.RawMessage { return json.RawMessage(`[]`) })
			item := reviewItem(a.files[0].URI)
			raw := "[" + item + "]"
			method := "textDocument/prepareCallHierarchy"
			switch kind {
			case "prep-empty", "native-empty":
				raw = `[{}]`
			case "missing-coordinate":
				raw = strings.Replace(raw, `"line":0,`, "", 1)
			case "null-coordinate":
				raw = strings.Replace(raw, `"line":0`, `"line":null`, 1)
			case "null-range":
				raw = strings.Replace(raw, `"range":`+reviewZeroRange, `"range":null`, 1)
			case "reverse-range":
				raw = strings.Replace(raw, `"start":{"line":0`, `"start":{"line":1`, 1)
			case "unknown":
				raw = strings.Replace(raw, `"kind":12`, `"kind":12,"future":1`, 1)
			case "symbol-tags":
				method = "textDocument/documentSymbol"
				raw = "[" + reviewSymbol(`,"tags":[1]`) + "]"
			case "symbol-deprecated":
				method = "textDocument/documentSymbol"
				raw = "[" + reviewSymbol(`,"deprecated":false`) + "]"
			case "symbol-unknown":
				method = "textDocument/documentSymbol"
				raw = "[" + reviewSymbol(`,"future":1`) + "]"
			case "native-byte-limit":
				raw = strings.Replace(raw, `"kind":12`, `"kind":12,"detail":"`+strings.Repeat("x", 2048)+`"`, 1)
			case "duplicate", "duplicate-native":
				raw = strings.Replace(raw, `"kind":12`, `"kind":12,"kind":12`, 1)
			}
			child.mu.Lock()
			child.response = func(lspwire.Message) json.RawMessage { return json.RawMessage(raw) }
			child.mu.Unlock()
			var err error
			if strings.HasPrefix(kind, "native-") || kind == "duplicate-native" {
				r := reviewNativeRequest(a)
				if kind == "native-byte-limit" {
					r.Limits.MaxResponseBytes = 1024
				}
				_, err = acquisition.Acquire(context.Background(), a.nativeClient(), r)
				// Provider failures are retained result data; adapter must still be terminal.
			} else if method == "textDocument/documentSymbol" {
				_, err = a.DocumentSymbols(context.Background(), lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: a.files[0].URI}})
			} else {
				_, err = a.PrepareCallHierarchy(context.Background(), lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: a.files[0].URI}})
			}
			n := len(semanticMessages(child))
			if n != 1 {
				t.Fatalf("fixture must consume exactly one provider response, got %d err=%v", n, err)
			}
			if !strings.Contains(kind, "native") && err == nil {
				t.Error("malformed response accepted")
			}
			child.mu.Lock()
			child.response = func(lspwire.Message) json.RawMessage { return json.RawMessage(`[]`) }
			child.mu.Unlock()
			_, nextErr := a.DocumentSymbols(context.Background(), lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: a.files[0].URI}})
			if nextErr == nil || len(semanticMessages(child)) != n {
				t.Fatalf("post-response error did not latch: before=%d after=%d err=%v", n, len(semanticMessages(child)), nextErr)
			}
		})
	}
}
func TestScopeAdapterReviewValidSymbolsAndEmpty(t *testing.T) {
	for _, kind := range []string{"hierarchical", "flat", "null", "empty"} {
		t.Run(kind, func(t *testing.T) {
			a, child := reviewAdapter(t, nil)
			raw := "[" + reviewSymbol("") + "]"
			switch kind {
			case "flat":
				raw = fmt.Sprintf(`[{"name":"Query","kind":12,"containerName":"pkg","location":{"uri":%q,"range":%s}}]`, a.files[0].URI, reviewZeroRange)
			case "null":
				raw = "null"
			case "empty":
				raw = "[]"
			}
			child.mu.Lock()
			child.response = func(lspwire.Message) json.RawMessage { return json.RawMessage(raw) }
			child.mu.Unlock()
			symbols, err := a.DocumentSymbols(context.Background(), lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: a.files[0].URI}})
			if err != nil {
				t.Fatal(err)
			}
			if kind == "flat" && (len(symbols) != 1 || !symbols[0].Flat || symbols[0].ContainerName != "pkg") {
				t.Fatal(symbols)
			}
			if kind == "hierarchical" && (len(symbols) != 1 || symbols[0].Flat || symbols[0].Range.Start != (lsp.Position{})) {
				t.Fatal(symbols)
			}
			if kind == "null" && symbols != nil {
				t.Fatal("null lost")
			}
			if kind == "empty" && symbols == nil {
				t.Fatal("empty lost")
			}
		})
	}
}
