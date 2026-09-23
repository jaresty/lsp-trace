package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"
)

func documentSymbolOwnedRequest(s StartResult, source OwnedDocumentBinding) RoundTripRequest {
	params, _ := json.Marshal(struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}{TextDocument: struct {
		URI string `json:"uri"`
	}{URI: source.URI}})
	return RoundTripRequest{SessionID: s.SessionID, Generation: s.Generation, Method: "textDocument/documentSymbol", Params: params, Deadline: time.Now().Add(time.Second), MaxMessages: 1, MaxBytes: 4096, CaptureOwnedMethodPair: true, ExpectedOwnedDocument: &source}
}

func TestDocumentSymbolManagedPair(t *testing.T) {
	m, s, child, source := ownedDocumentFixture(t, true)
	defer m.Shutdown(context.Background())
	req := documentSymbolOwnedRequest(s, source)
	got := m.RoundTrip(context.Background(), req)
	pair, ok := got.CompletedOwnedMethodPair()
	if got.Failure != "" || !ok || pair.Source == nil || *pair.Source != source || pair.Method != req.Method || pair.SessionID != req.SessionID || pair.Generation != req.Generation || pair.Key != got.Key || pair.Key.ID == 0 || !bytes.Equal(pair.Params, req.Params) || !bytes.Equal(pair.Result, got.Result) || pair.Write.Key != pair.Key || pair.Read.Key != pair.Key || pair.Write.Method != req.Method {
		t.Fatalf("ASSERT_DOC_SYMBOL_MANAGED_PAIR: failure=%s ok=%v pair=%+v", got.Failure, ok, pair)
	}
	requests, _ := child.snapshot()
	found := false
	for _, r := range requests {
		if r.Method == req.Method {
			found = bytes.Equal(r.Params, req.Params)
		}
	}
	if !found {
		t.Fatal("ASSERT_DOC_SYMBOL_MANAGED_PAIR: no matching managed wire request")
	}
	req.Params[0] = 'x'
	pair.Result[0] = 'x'
	pair.Source.URI = "file:///wrong"
	again, present := got.CompletedOwnedMethodPair()
	if !present || again.Source.URI != source.URI || !bytes.Equal(again.Params, documentSymbolOwnedRequest(s, source).Params) || !bytes.Equal(again.Result, got.Result) {
		t.Fatal("ASSERT_DOC_SYMBOL_MANAGED_PAIR: copied fields not independent")
	}
}

func TestDocumentSymbolSourceMismatch(t *testing.T) {
	for _, name := range []string{"uri", "version", "digest"} {
		t.Run(name, func(t *testing.T) {
			m, s, child, source := ownedDocumentFixture(t, true)
			defer m.Shutdown(context.Background())
			req := documentSymbolOwnedRequest(s, source)
			switch name {
			case "uri":
				req.ExpectedOwnedDocument.URI = "file:///wrong.go"
			case "version":
				req.ExpectedOwnedDocument.Version++
			case "digest":
				req.ExpectedOwnedDocument.SHA256 = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
			}
			got := m.RoundTrip(context.Background(), req)
			requests, _ := child.snapshot()
			for _, r := range requests {
				if r.Method == req.Method {
					t.Fatal("ASSERT_DOC_SYMBOL_SOURCE_MISMATCH: method written")
				}
			}
			if got.Failure != DocumentSupplyUnavailable || got.Key.ID != 0 {
				t.Fatalf("ASSERT_DOC_SYMBOL_SOURCE_MISMATCH: %+v", got)
			}
		})
	}
}
