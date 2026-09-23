package adr0011methodresult

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"lsp-trace/internal/lspwire"
	"lsp-trace/sessionruntime"
	"strings"
	"testing"
)

func TestOwnerPairCanonicalResultGrammar(t *testing.T) {
	key := lspwire.RequestKey{Generation: 3, ID: 7}
	pair := sessionruntime.OwnedMethodPair{SessionID: "s", Generation: 3, Key: key, Method: "textDocument/definition", Params: []byte(`{"textDocument":{"uri":"file:///a"},"position":{"line":0,"character":0}}`), Write: sessionruntime.RequestWriteObservation{SessionID: "s", Generation: 3, Key: key, Method: "textDocument/definition", FrameBytes: 120, FrameSHA256: rawSHA([]byte("write"))}, Read: sessionruntime.ResponseReadObservation{SessionID: "s", Generation: 3, Key: key, FrameBytes: 50, FrameSHA256: rawSHA([]byte("read"))}}
	location := `{"uri":"file:///a","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}`
	cases := []struct {
		name, method, result string
		valid                bool
	}{
		{"null", "textDocument/definition", `null`, true},
		{"empty references", "textDocument/references", `[]`, true},
		{"repeated locations", "textDocument/references", `[` + location + `,` + location + `]`, true},
		{"candidate limit inclusive", "textDocument/references", `[` + strings.TrimSuffix(strings.Repeat(location+`,`, 1000), `,`) + `]`, true},
		{"scalar", "textDocument/definition", `true`, false},
		{"references object", "textDocument/references", location, false},
		{"malformed location", "textDocument/definition", `{"uri":"file:///a","range":{}}`, false},
		{"duplicate nested keys", "textDocument/definition", `{"uri":"file:///a","range":{"start":{"line":0,"line":1,"character":0},"end":{"line":0,"character":1}}}`, false},
		{"candidate limit", "textDocument/references", `[` + strings.TrimSuffix(strings.Repeat(location+`,`, 1001), `,`) + `]`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pair.Method, pair.Write.Method, pair.Result = tc.method, tc.method, []byte(tc.result)
			if tc.method == "textDocument/references" {
				pair.Params = []byte(`{"textDocument":{"uri":"file:///a"},"position":{"line":0,"character":0},"context":{"includeDeclaration":true}}`)
			} else {
				pair.Params = []byte(`{"textDocument":{"uri":"file:///a"},"position":{"line":0,"character":0}}`)
			}
			expected := OwnerPairExpected{SessionID: pair.SessionID, Generation: pair.Generation, KeyID: key.ID, Method: pair.Method, Params: pair.Params, Result: pair.Result, Write: pair.Write, Read: pair.Read}
			built, buildErr := BuildOwnerPairCanonical(pair)
			if (buildErr == nil) != tc.valid {
				t.Fatalf("build grammar validity = %v, want %v", buildErr == nil, tc.valid)
			}
			if !tc.valid {
				record := ownerPairRecord{OwnerPairCanonicalVersion, "PRIVATE;UNADMITTED;NO_PRODUCER_AUTHENTICATION", expected.SessionID, expected.Generation, expected.KeyID, expected.Method, base64.StdEncoding.EncodeToString(expected.Params), base64.StdEncoding.EncodeToString(expected.Result), expected.Write, expected.Read, nil}
				built, _ = json.Marshal(record)
				built = append(built, '\n')
			}
			verifyErr := VerifyOwnerPairCanonical(built, expected)
			if (verifyErr == nil) != tc.valid {
				t.Fatalf("verify grammar validity = %v, want %v", verifyErr == nil, tc.valid)
			}
		})
	}
}

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
