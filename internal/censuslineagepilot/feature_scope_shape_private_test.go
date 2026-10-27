//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
)

func TestScopeAdapterReviewMixedFlatOmission(t *testing.T) {
	a, child := reviewAdapter(t, nil)
	contextURI := a.guard.MembersPrivate()[5].URI
	raw := json.RawMessage(fmt.Sprintf(`[{"name":"Local","kind":12,"location":{"uri":%q,"range":%s}},{"name":"Context","kind":12,"location":{"uri":%q,"range":%s}}]`, a.files[0].URI, reviewZeroRange, contextURI, reviewZeroRange))
	child.mu.Lock()
	child.response = func(lspwire.Message) json.RawMessage { return raw }
	child.mu.Unlock()
	rows, err := a.DocumentSymbols(context.Background(), lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: a.files[0].URI}})
	if err != nil || len(rows) != 1 || rows[0].Name != "Local" || rows[0].LocationURI != a.files[0].URI {
		t.Fatal("local/context identity", rows, err)
	}
	if (scopeBindingObservation{a}).symbolOmissions() != 1 {
		t.Fatal("omission observation missing")
	}
	if len(semanticMessages(child)) != 1 {
		t.Fatal("context prepared")
	}
}
func TestScopeAdapterReviewTypedCallRefusals(t *testing.T) {
	for _, kind := range []string{"incoming-wrong-reference", "outgoing-wrong-reference", "null-from-ranges", "missing-from-ranges", "null-item", "missing-site-coordinate", "native-incoming"} {
		t.Run(kind, func(t *testing.T) {
			a, child := reviewAdapter(t, nil)
			raw := fmt.Sprintf(`[{"from":%s,"fromRanges":[]}]`, reviewItem("file:///external.go"))
			incoming := true
			switch kind {
			case "incoming-wrong-reference":
				raw = fmt.Sprintf(`[{"to":%s,"fromRanges":[]}]`, reviewItem("file:///external.go"))
			case "outgoing-wrong-reference":
				incoming = false
			case "null-from-ranges":
				raw = fmt.Sprintf(`[{"from":%s,"fromRanges":null}]`, reviewItem("file:///external.go"))
			case "missing-from-ranges":
				raw = fmt.Sprintf(`[{"from":%s}]`, reviewItem("file:///external.go"))
			case "null-item":
				raw = `[{"from":null,"fromRanges":[]}]`
			case "missing-site-coordinate":
				raw = fmt.Sprintf(`[{"from":%s,"fromRanges":[{"start":{"line":0},"end":{"line":0,"character":0}}]}]`, reviewItem("file:///external.go"))
			case "native-incoming":
				raw = `[{"from":{},"fromRanges":[]}]`
			}
			child.mu.Lock()
			child.response = func(msg lspwire.Message) json.RawMessage {
				if msg.Method == "textDocument/prepareCallHierarchy" {
					return json.RawMessage("[" + reviewItem(a.files[0].URI) + "]")
				}
				return json.RawMessage(raw)
			}
			child.mu.Unlock()
			var err error
			if kind == "native-incoming" {
				_, err = acquisition.Acquire(context.Background(), a.nativeClient(), reviewNativeRequest(a))
			} else if incoming {
				_, _, err = a.IncomingCalls(context.Background(), scopeAdapterItem(a.files[0].URI))
			} else {
				_, _, err = a.OutgoingCalls(context.Background(), scopeAdapterItem(a.files[0].URI))
			}
			n := len(semanticMessages(child))
			want := 1
			if kind == "native-incoming" {
				want = 2
			} else if err == nil {
				t.Error("malformed typed call accepted")
			}
			if n != want {
				t.Fatal("unexpected response count", n, want)
			}
			_, err = a.DocumentSymbols(context.Background(), lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: a.files[0].URI}})
			if err == nil || len(semanticMessages(child)) != n {
				t.Fatal("typed call failure not terminal")
			}
		})
	}
}
func TestScopeAdapterReviewHarmlessPrewriteRefusal(t *testing.T) {
	a, child := reviewAdapter(t, func(lspwire.Message) json.RawMessage { return json.RawMessage(`[]`) })
	// Native wire methods without a coordinator context refuse before callback.
	if _, err := a.nativeClient().DocumentSymbols(context.Background(), lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: a.files[0].URI}}); err == nil {
		t.Fatal("missing native context")
	}
	if len(semanticMessages(child)) != 0 {
		t.Fatal("prewrite refusal consumed response")
	}
	if _, err := a.DocumentSymbols(context.Background(), lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: a.files[0].URI}}); err != nil || len(semanticMessages(child)) != 1 {
		t.Fatal("harmless prewrite refusal latched", err)
	}
}
