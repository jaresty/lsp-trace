package sessionruntime

import (
	"encoding/json"
	qt "lsp-trace/internal/adr0011querytarget"
	"strings"
	"testing"
)

// Pure selector tests are not runtime custody issuance. Neither these DTOs nor
// their successful selection can mint a PrivateGroupingSourceEvidence.
func TestPrivateDefinitionOccurrenceGeneric(t *testing.T) {
	symbols := []byte(`[{"name":"member","kind":7,"range":{"start":{"line":2,"character":0},"end":{"line":6,"character":0}},"selectionRange":{"start":{"line":2,"character":1},"end":{"line":2,"character":7}}}]`)
	for _, uri := range []string{"file:///w/object.cs", "file:///w/object.ex", "file:///w/data", "untitled:buffer"} {
		q := qt.Query{OccurrenceID: "actual-query", URI: uri, Encoding: "utf-16", DocumentVersion: "1", SourceDigest: "sha256:" + strings.Repeat("a", 64), SessionID: "session", Generation: 1, Line: 4, Character: 9}
		params, _ := json.Marshal(map[string]any{"textDocument": map[string]string{"uri": uri}, "position": map[string]uint32{"line": q.Line, "character": q.Character}})
		for _, result := range []string{
			`[{"uri":"file:///w/target.data","range":{"start":{"line":8,"character":3},"end":{"line":8,"character":8}}}]`,
			`[{"targetUri":"file:///w/target.data","targetRange":{"start":{"line":8,"character":0},"end":{"line":10,"character":0}},"targetSelectionRange":{"start":{"line":8,"character":3},"end":{"line":8,"character":8}}}]`,
		} {
			p := OwnedMethodPair{Method: "textDocument/definition", SessionID: q.SessionID, Generation: q.Generation, Source: &OwnedDocumentBinding{URI: uri, Version: 1, SHA256: q.SourceDigest}, Params: params, Result: []byte(result)}
			c, e := SelectPrivateDefinitionOccurrenceV1(q, symbols, p)
			if e != nil || c.SymbolKind != 7 || c.QueryURI != uri || c.QueryLine != 4 || c.QueryCharacter != 9 {
				t.Fatal("ASSERT_GENERIC_DEFINITION", uri, c, e)
			}
			if _, e := SelectPrivateDefinitionOccurrenceV1(q, symbols, OwnedMethodPair{}); e == nil {
				t.Fatal("missing method admitted")
			}
		}
	}
	t.Log("ASSERT_GENERIC_DEFINITION: PASS; property container, Location/LocationLink, four URI spellings")
}
