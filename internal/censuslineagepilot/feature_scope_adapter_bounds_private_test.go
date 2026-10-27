//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"encoding/json"
	"testing"

	"lsp-trace/internal/directionalraw"
	"lsp-trace/internal/lsp"
)

func TestScopeAdapterPrivateBoundsAndAliases(t *testing.T) {
	for _, kind := range []string{"params", "requests", "capture", "alias", "directional"} {
		t.Run(kind, func(t *testing.T) {
			restart, m, id, child := adapterFixture(t)
			a, _, err := newScopeRuntimeAdapter(context.Background(), restart, m, id)
			if err != nil {
				t.Fatal(err)
			}
			params, _ := json.Marshal(lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: a.files[0].URI}})
			switch kind {
			case "params":
				params = make([]byte, scopeAdapterParams+1)
			case "requests":
				a.requests = scopeAdapterRequests
			case "capture":
				a.captured = scopeAdapterCapture
			case "alias":
				params = json.RawMessage(`{"TextDocument":{"uri":"` + a.files[0].URI + `"}}`)
			}
			before := len(child.snapshot())
			ctx := context.Background()
			method := "textDocument/documentSymbol"
			if kind == "directional" {
				ctx = directionalraw.NewReservation("private-fake").Context(ctx)
				method = "textDocument/prepareCallHierarchy"
			}
			if _, err = a.roundTrip(ctx, method, params); err == nil {
				t.Fatal("bound/alias accepted")
			}
			if len(child.snapshot()) != before || len(semanticMessages(child)) != 0 {
				t.Fatal("preflight wrote")
			}
		})
	}
}
