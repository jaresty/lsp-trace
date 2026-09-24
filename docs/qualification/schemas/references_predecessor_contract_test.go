package schemas

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"lsp-trace/internal/adr0011querytarget"
	"lsp-trace/internal/strictjson"
)

// Synthetic illustration of the prospective byte contract, not a producer
// implementation or proof of matched managed write/read observation.
func exactParamsFromWrite(write []byte, expectedMethod string, expectedID int, offset, length int, digest string) bool {
	if strictjson.RejectDuplicates(write) != nil || offset < 0 || length < 1 || length > 65536 || offset > len(write) || length > len(write)-offset {
		return false
	}
	var body struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int             `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	dec := json.NewDecoder(bytes.NewReader(write))
	dec.DisallowUnknownFields()
	if dec.Decode(&body) != nil || body.JSONRPC != "2.0" || body.ID != expectedID || body.Method != expectedMethod || len(body.Params) == 0 {
		return false
	}
	var trailing any
	if dec.Decode(&trailing) != io.EOF {
		return false
	}
	// This fixture has one params token. A production verifier must obtain its
	// exact parser span independently, never search for a matching byte string.
	span := write[offset : offset+length]
	if !bytes.Equal(span, body.Params) || bytes.Count(write, span) != 1 {
		return false
	}
	return digest == fmt.Sprintf("sha256:%x", sha256.Sum256(span))
}
func TestReferencesExactParameterSliceContract(t *testing.T) {
	write := []byte(`{"jsonrpc":"2.0","id":7,"method":"textDocument/references","params": {"textDocument":{"uri":"file:///a.go"}, "position":{"line":1,"character":2},"context":{"includeDeclaration":true}}}`)
	params := []byte(`{"textDocument":{"uri":"file:///a.go"}, "position":{"line":1,"character":2},"context":{"includeDeclaration":true}}`)
	start := bytes.Index(write, params)
	if start < 0 {
		t.Fatal("missing fixture")
	}
	d := fmt.Sprintf("sha256:%x", sha256.Sum256(params))
	valid := func(b []byte, off, length int) bool {
		return exactParamsFromWrite(b, "textDocument/references", 7, off, length, d)
	}
	if !valid(write, start, len(params)) {
		t.Fatal("exact write")
	}
	if valid(write, start+1, len(params)) || valid(write, start, len(params)-1) || valid(write, start, len(params)+1) {
		t.Fatal("shifted params span")
	}
	changed := bytes.Replace(write, []byte(`"line":1`), []byte(`"line":2`), 1)
	if valid(changed, start, len(params)) {
		t.Fatal("substituted original params")
	}
	reserialized := bytes.Replace(write, []byte(`, "position"`), []byte(`,"position"`), 1)
	if valid(reserialized, start, len(params)-1) {
		t.Fatal("semantically equivalent bytes merged")
	}
	if exactParamsFromWrite(write, "textDocument/documentSymbol", 7, start, len(params), d) || exactParamsFromWrite(write, "textDocument/references", 8, start, len(params), d) {
		t.Fatal("unmatched method or wire ID")
	}
}
func TestReferencesTargetResultTieCounterexample(t *testing.T) {
	q := adr0011querytarget.Query{OccurrenceID: "q", URI: "file:///a.go", Encoding: "utf-16", DocumentVersion: "1", SourceDigest: "sha256:" + strings.Repeat("a", 64), SessionID: "s", Generation: 1, Line: 1, Character: 2}
	sym := `{"name":"A","kind":12,"range":{"start":{"line":0,"character":0},"end":{"line":3,"character":0}},"selectionRange":{"start":{"line":1,"character":0},"end":{"line":1,"character":4}}}`
	one := []byte("[" + sym + "]")
	if _, err := adr0011querytarget.SelectDocumentSymbolCandidateV1(q, one); err != nil {
		t.Fatalf("one unique symbol: %v", err)
	}
	tied := []byte("[" + sym + "," + sym + "]")
	if _, err := adr0011querytarget.SelectDocumentSymbolCandidateV1(q, tied); err == nil {
		t.Fatal("tied result selected")
	}
	if sha256.Sum256(one) == sha256.Sum256(tied) {
		t.Fatal("raw result identity did not change")
	}
	// The actual final replayer must independently read the target-result blob,
	// rederive selection and IDs, and reject even a self-consistent forged pair.
}
