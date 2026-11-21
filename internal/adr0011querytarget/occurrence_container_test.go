package adr0011querytarget

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOccurrenceContainerLanguageNeutral(t *testing.T) {
	for _, uri := range []string{"file:///workspace/one.cs", "file:///workspace/two.ex", "file:///workspace/no-extension", "untitled:buffer"} {
		for _, kind := range []int{5, 7, 12, 13, 26} {
			q := candidateQuery()
			q.URI = uri
			q.Line = 3
			q.Character = 4
			raw := []byte(strings.Replace(hierarchical, `"kind":12`, `"kind":`+stringKind(kind), 1))
			got, err := SelectOccurrenceContainerV1(q, raw)
			if err != nil || got.SymbolName != "Child" || got.SymbolKind != kind || got.QueryLine != q.Line || got.QueryCharacter != q.Character || got.Version != OccurrenceCandidateVersionV1 {
				t.Fatalf("ASSERT_GENERIC_OCCURRENCE: %s kind=%d %+v %v", uri, kind, got, err)
			}
			if _, err := SelectDocumentSymbolCandidateV1(q, raw); err == nil {
				t.Fatal("ASSERT_DECLARATION_SELECTOR_UNCHANGED")
			}
			declaration := q
			declaration.Line = 2
			declaration.Character = 3
			named, err := SelectDocumentSymbolCandidateV1(declaration, raw)
			if err != nil || named.SymbolID != got.SymbolID || DefinitionQueryNodeV1(q, got) == got.SymbolID {
				t.Fatal("ASSERT_OCCURRENCE_CONTAINER_DISTINCT", err)
			}
			moved := q
			moved.Character++
			if DefinitionQueryNodeV1(moved, got) == DefinitionQueryNodeV1(q, got) {
				t.Fatal("ASSERT_OCCURRENCE_IDENTITY_POSITION")
			}
		}
	}
	t.Log("ASSERT_GENERIC_OCCURRENCE: PASS; declaration, occurrence and container identities distinct")
}
func stringKind(n int) string { b, _ := json.Marshal(n); return string(b) }

func TestOccurrenceContainerRefusals(t *testing.T) {
	item := `{"name":"generic","kind":7,"range":{"start":{"line":2,"character":0},"end":{"line":5,"character":0}},"selectionRange":{"start":{"line":2,"character":1},"end":{"line":2,"character":3}}}`
	for _, tc := range []struct {
		name, raw       string
		line, character uint32
	}{
		{"outside", "[" + item + "]", 6, 0},
		{"exclusive-end", "[" + item + "]", 5, 0},
		{"duplicate", "[" + item + "," + item + "]", 3, 4},
		{"overlap", "[" + item + "," + strings.ReplaceAll(strings.ReplaceAll(item, `"line":2`, `"line":3`), `"line":5`, `"line":6`) + "]", 3, 4},
		{"malformed-sibling", "[" + item + `,{"name":"bad","kind":0}]`, 3, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := candidateQuery()
			q.Line = tc.line
			q.Character = tc.character
			if _, err := SelectOccurrenceContainerV1(q, []byte(tc.raw)); err == nil {
				t.Fatal("ASSERT_GENERIC_OCCURRENCE_REFUSAL", tc.name)
			}
			t.Log("ASSERT_GENERIC_OCCURRENCE_REFUSAL: PASS", tc.name)
		})
	}
}
