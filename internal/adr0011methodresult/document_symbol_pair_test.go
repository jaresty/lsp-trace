package adr0011methodresult

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/internal/lspwire"
	"lsp-trace/sessionruntime"
)

func documentSymbolPairFixture() (sessionruntime.OwnedMethodPair, DocumentSymbolPairExpected) {
	key := lspwire.RequestKey{Generation: 3, ID: 7}
	pair := sessionruntime.OwnedMethodPair{SessionID: "s", Generation: 3, Key: key, Method: "textDocument/documentSymbol", Params: []byte(`{"textDocument":{"uri":"file:///a"}}`), Result: []byte(`[]`), Source: &sessionruntime.OwnedDocumentBinding{URI: "file:///a", Version: 2, SHA256: rawSHA([]byte("source"))}, Write: sessionruntime.RequestWriteObservation{SessionID: "s", Generation: 3, Key: key, Method: "textDocument/documentSymbol", FrameBytes: 120, FrameSHA256: rawSHA([]byte("write"))}, Read: sessionruntime.ResponseReadObservation{SessionID: "s", Generation: 3, Key: key, FrameBytes: 50, FrameSHA256: rawSHA([]byte("read"))}}
	return pair, DocumentSymbolPairExpected{SessionID: pair.SessionID, Generation: pair.Generation, KeyID: key.ID, Method: pair.Method, Params: pair.Params, Result: pair.Result, Write: pair.Write, Read: pair.Read, Source: pair.Source}
}
func TestDocumentSymbolPairCanonicalPrivate(t *testing.T) {
	pair, want := documentSymbolPairFixture()
	raw, err := BuildDocumentSymbolPairCanonical(pair)
	if err != nil {
		t.Fatalf("ASSERT_DOC_PAIR_BUILD_EMPTY_RESULT: %v", err)
	}
	if err = VerifyDocumentSymbolPairCanonical(raw, want); err != nil {
		t.Fatalf("ASSERT_DOC_PAIR_ROUNDTRIP: %v", err)
	}
	if !bytes.HasSuffix(raw, []byte{'\n'}) || bytes.Contains(raw, []byte("PRODUCER_AUTHENTICATED")) {
		t.Fatal("ASSERT_DOC_PAIR_PRIVATE_CANONICAL")
	}
	cases := []struct {
		name   string
		mutate func(*DocumentSymbolPairExpected)
	}{
		{"source", func(e *DocumentSymbolPairExpected) { s := *e.Source; s.Version++; e.Source = &s }},
		{"key", func(e *DocumentSymbolPairExpected) { e.KeyID++ }},
		{"result", func(e *DocumentSymbolPairExpected) { e.Result = []byte(`[{}]`) }},
		{"write", func(e *DocumentSymbolPairExpected) { e.Write.FrameBytes++ }},
		{"read", func(e *DocumentSymbolPairExpected) { e.Read.FrameBytes++ }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			other := want
			tc.mutate(&other)
			if VerifyDocumentSymbolPairCanonical(raw, other) == nil {
				t.Fatalf("ASSERT_DOC_PAIR_INDEPENDENT_%s: accepted", tc.name)
			}
		})
	}
	for _, suffix := range []string{` {}`, `{"unknown":1}`} {
		if VerifyDocumentSymbolPairCanonical(append(bytes.Clone(raw), suffix...), want) == nil {
			t.Fatalf("ASSERT_DOC_PAIR_TRAILING: %q", suffix)
		}
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	fields["unknown"] = json.RawMessage(`1`)
	unknown, _ := json.Marshal(fields)
	if VerifyDocumentSymbolPairCanonical(append(unknown, '\n'), want) == nil {
		t.Fatal("ASSERT_DOC_PAIR_UNKNOWN")
	}
	duplicate := bytes.Replace(raw, []byte(`"Version":`), []byte(`"Version":"duplicate","Version":`), 1)
	if VerifyDocumentSymbolPairCanonical(duplicate, want) == nil {
		t.Fatal("ASSERT_DOC_PAIR_DUPLICATE")
	}
	pair.Result = []byte(strings.Replace(`[{"name":"A","kind":12,"range":{"start":{"line":0,"character":0},"end":{"line":1,"character":0}},"selectionRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}},"children":[]}]`, `"children":[]`, `"children":[{"name":"Bad","kind":0}]`, 1))
	if _, err = BuildDocumentSymbolPairCanonical(pair); err == nil {
		t.Fatal("ASSERT_DOC_PAIR_INVALID_LATER_CHILD")
	}
	pair, want = documentSymbolPairFixture()
	pair.Source = nil
	if _, err = BuildDocumentSymbolPairCanonical(pair); err == nil {
		t.Fatal("ASSERT_DOC_PAIR_SOURCE_REQUIRED")
	}
	_ = want
}
