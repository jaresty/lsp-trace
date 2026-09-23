package adr0011methodresult

import (
	"bytes"
	"lsp-trace/internal/lspwire"
	"lsp-trace/sessionruntime"
	"testing"
)

func TestOwnerPairCanonicalSourceQueryBinding(t *testing.T) {
	key := lspwire.RequestKey{Generation: 3, ID: 7}
	source := &sessionruntime.OwnedDocumentBinding{URI: "file:///a", Version: 2, SHA256: rawSHA([]byte("source"))}
	pair := sessionruntime.OwnedMethodPair{
		SessionID: "s", Generation: 3, Key: key, Method: "textDocument/definition",
		Params: []byte(`{"textDocument":{"uri":"file:///a"},"position":{"line":0,"character":0}}`), Result: []byte(`null`),
		Write: sessionruntime.RequestWriteObservation{SessionID: "s", Generation: 3, Key: key, Method: "textDocument/definition", FrameBytes: 120, FrameSHA256: rawSHA([]byte("write"))},
		Read:  sessionruntime.ResponseReadObservation{SessionID: "s", Generation: 3, Key: key, FrameBytes: 50, FrameSHA256: rawSHA([]byte("read"))}, Source: source,
	}
	expected := OwnerPairExpected{SessionID: pair.SessionID, Generation: pair.Generation, KeyID: key.ID, Method: pair.Method, Params: pair.Params, Result: pair.Result, Write: pair.Write, Read: pair.Read, Source: source}
	encoded, err := BuildOwnerPairCanonical(pair)
	if err != nil {
		t.Fatalf("matching source: %v", err)
	}
	if err := VerifyOwnerPairCanonical(encoded, expected); err != nil {
		t.Fatalf("matching source readback: %v", err)
	}
	wrong := *source
	wrong.URI = "file:///other"
	pair.Source = &wrong
	if _, err := BuildOwnerPairCanonical(pair); err == nil {
		t.Fatal("TestOwnerPairCanonicalSourceQueryBinding: present source URI mismatching validated params accepted")
	}
	expected.Source = &wrong
	if err := VerifyOwnerPairCanonical(encoded, expected); err == nil {
		t.Fatal("independent expected wrong source URI accepted")
	}
	wrong = *source
	wrong.Version++
	expected.Source = &wrong
	if err := VerifyOwnerPairCanonical(encoded, expected); err == nil {
		t.Fatal("independent expected wrong source version accepted")
	}
	wrong = *source
	wrong.SHA256 = rawSHA([]byte("other"))
	expected.Source = &wrong
	if err := VerifyOwnerPairCanonical(encoded, expected); err == nil {
		t.Fatal("independent expected wrong source digest accepted")
	}
	expected.Source = nil
	if err := VerifyOwnerPairCanonical(encoded, expected); err == nil {
		t.Fatal("source presence accepted as absence")
	}
	pair.Source = nil
	withoutSource, err := BuildOwnerPairCanonical(pair)
	if err != nil {
		t.Fatalf("absent source: %v", err)
	}
	if err := VerifyOwnerPairCanonical(withoutSource, expected); err != nil {
		t.Fatalf("absent source readback: %v", err)
	}
}

func TestOwnerPairCanonical(t *testing.T) {
	key := lspwire.RequestKey{Generation: 3, ID: 7}
	pair := sessionruntime.OwnedMethodPair{SessionID: "s", Generation: 3, Key: key, Method: "textDocument/definition", Params: []byte(`{"textDocument":{"uri":"file:///a"},"position":{"line":0,"character":0}}`), Result: []byte(`null`), Write: sessionruntime.RequestWriteObservation{SessionID: "s", Generation: 3, Key: key, Method: "textDocument/definition", FrameBytes: 120, FrameSHA256: rawSHA([]byte("write"))}, Read: sessionruntime.ResponseReadObservation{SessionID: "s", Generation: 3, Key: key, FrameBytes: 50, FrameSHA256: rawSHA([]byte("read"))}}
	expected := OwnerPairExpected{SessionID: pair.SessionID, Generation: pair.Generation, KeyID: pair.Key.ID, Method: pair.Method, Params: pair.Params, Result: pair.Result, Write: pair.Write, Read: pair.Read}
	raw, err := BuildOwnerPairCanonical(pair)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyOwnerPairCanonical(raw, expected); err != nil {
		t.Fatalf("valid pair: %v", err)
	}
	expected.Result = []byte(`[]`)
	if err := VerifyOwnerPairCanonical(raw, expected); err == nil {
		t.Fatal("TestOwnerPairCanonical: independent expected result mismatch accepted")
	}
	expected.Result = pair.Result
	expected.SessionID = "other"
	if VerifyOwnerPairCanonical(raw, expected) == nil {
		t.Fatal("independent session mismatch accepted")
	}
	expected.SessionID = pair.SessionID
	for _, mutation := range [][]byte{
		bytes.Replace(raw, []byte(`"Version":`), []byte(`"Extra":0,"Version":`), 1),
		bytes.Replace(raw, []byte(`"Version":`), []byte(`"Version":"duplicate","Version":`), 1),
		bytes.TrimSuffix(raw, []byte("\n")),
	} {
		if VerifyOwnerPairCanonical(mutation, expected) == nil {
			t.Fatal("malformed canonical encoding accepted")
		}
	}
	source := &sessionruntime.OwnedDocumentBinding{URI: "file:///a", Version: 2, SHA256: rawSHA([]byte("source"))}
	pair.Source = source
	withSource, err := BuildOwnerPairCanonical(pair)
	if err != nil {
		t.Fatal(err)
	}
	expected.Source = source
	if err := VerifyOwnerPairCanonical(withSource, expected); err != nil {
		t.Fatal(err)
	}
	if VerifyOwnerPairCanonical(raw, expected) == nil {
		t.Fatal("source absence accepted as presence")
	}
	pair.Params = bytes.Repeat([]byte(" "), 64<<10+1)
	if _, err := BuildOwnerPairCanonical(pair); err == nil {
		t.Fatal("oversized params accepted")
	}
	pair.Params = expected.Params
	pair.Result = bytes.Repeat([]byte(" "), 1<<20+1)
	if _, err := BuildOwnerPairCanonical(pair); err == nil {
		t.Fatal("oversized result accepted")
	}
}
