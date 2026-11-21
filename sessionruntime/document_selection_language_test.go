package sessionruntime

import (
	"context"
	"encoding/json"
	"testing"
)

// Uses only supplyChild's in-memory writer: no provider process or network.
func TestDocumentSelectionExplicitLanguageBeatsDefault(t *testing.T) {
	for _, tc := range []struct{ name, request, configured, want string }{
		{"explicit-tsx", "typescriptreact", "typescript", "typescriptreact"},
		{"configured-fallback", "", "typescript", "typescript"},
		{"filename-fallback", "", "", "go"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, req, _, _ := supplyFixture(t, []byte("fixture"))
			m.mu.Lock()
			m.sessions[req.SessionID].languageID = tc.configured
			m.mu.Unlock()
			req.LanguageID = tc.request
			got := m.PrepareDocument(context.Background(), req)
			if got.Failure != "" || got.LanguageID != tc.want || got.Supply == nil {
				t.Fatal(got)
			}
			var wire struct {
				TextDocument struct {
					LanguageID string `json:"languageId"`
				} `json:"textDocument"`
			}
			if e := json.Unmarshal(got.Supply.Params, &wire); e != nil || wire.TextDocument.LanguageID != tc.want || got.Supply.Method != "textDocument/didOpen" {
				t.Fatal(wire, e)
			}
		})
	}
}
