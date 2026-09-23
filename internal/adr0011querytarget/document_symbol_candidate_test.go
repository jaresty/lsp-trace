package adr0011querytarget

import (
	"errors"
	"strings"
	"testing"
)

const hierarchical = `[{
 "name":"Parent","kind":5,
 "range":{"start":{"line":0,"character":0},"end":{"line":8,"character":0}},
 "selectionRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":6}},
 "children":[{
  "name":"Child","kind":12,
  "range":{"start":{"line":2,"character":0},"end":{"line":5,"character":0}},
  "selectionRange":{"start":{"line":2,"character":2},"end":{"line":2,"character":7}}
 }]
}]`

func candidateQuery() Query {
	return Query{OccurrenceID: "declared-query", URI: "file:///w/a.go", Encoding: "utf-16", DocumentVersion: "4", SourceDigest: "sha256:" + strings.Repeat("a", 64), SessionID: "project", Generation: 1, Line: 2, Character: 3}
}

func TestDocumentSymbolCandidateUsesUniqueSelectionNotContainingDisplay(t *testing.T) {
	q := candidateQuery()
	got, err := SelectDocumentSymbolCandidate(q, []byte(hierarchical))
	if err != nil || got.SymbolName != "Child" || got.SymbolKind != 12 || got.QueryOccurrenceID != q.OccurrenceID || got.QueryURI != q.URI || got.QueryLine != q.Line || got.SelectionRange.Start != (Position{2, 2}) || got.DisplayRange.Start != (Position{2, 0}) || got.ID == "" || got.SymbolID == "" {
		t.Fatalf("exact child selection lost: %+v %v", got, err)
	}
	// The parent's display contains the query but its selection does not.
	parent := q
	parent.Line, parent.Character = 0, 2
	selected, err := SelectDocumentSymbolCandidate(parent, []byte(hierarchical))
	if err != nil || selected.SymbolName != "Parent" || selected.SymbolID == got.SymbolID {
		t.Fatalf("different selection did not choose distinct symbol: %+v %v", selected, err)
	}
	version := q
	version.DocumentVersion = "5"
	changed, err := SelectDocumentSymbolCandidate(version, []byte(hierarchical))
	if err != nil || changed.SymbolID == got.SymbolID {
		t.Fatalf("changed source version reused candidate identity: %+v %v", changed, err)
	}
	// A caller-controlled source digest can be changed consistently: this
	// function does not authenticate the document or its producer.
	forged := q
	forged.SourceDigest = "sha256:" + strings.Repeat("b", 64)
	if _, err := SelectDocumentSymbolCandidate(forged, []byte(hierarchical)); err != nil {
		t.Fatalf("candidate ceiling control unexpectedly authenticated source: %v", err)
	}
}

func TestDocumentSymbolCandidateRejectsAmbiguityMalformedAndLimits(t *testing.T) {
	q := candidateQuery()
	inner := strings.TrimSpace(hierarchical)
	duplicate := "[" + inner[1:len(inner)-1] + "," + inner[1:len(inner)-1] + "]"
	for _, tc := range []struct{ name, raw string }{
		{"no match", `[]`},
		{"null", `null`},
		{"flat only", `[{"name":"Child","kind":12,"location":{"uri":"file:///w/a.go","range":{"start":{"line":2,"character":2},"end":{"line":2,"character":7}}}}]`},
		{"duplicate symbol", duplicate},
		{"missing coordinate", strings.Replace(hierarchical, `"line":2,"character":2`, `"line":2`, 1)},
		{"unknown member", strings.Replace(hierarchical, `"name":"Child"`, `"name":"Child","invented":true`, 1)},
		{"duplicate key", strings.Replace(hierarchical, `"name":"Child"`, `"name":"Child","name":"Other"`, 1)},
		{"selection outside", strings.Replace(hierarchical, `"line":2,"character":2`, `"line":9,"character":2`, 1)},
		{"oversized", strings.Repeat(" ", maxResultBytes+1)},
		{"deep", strings.Repeat("[", maxJSONDepth+1) + "0" + strings.Repeat("]", maxJSONDepth+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SelectDocumentSymbolCandidate(q, []byte(tc.raw))
			if !errors.Is(err, ErrUnresolved) || got.ID != "" {
				t.Fatalf("invalid document-symbol evidence selected: %+v %v", got, err)
			}
		})
	}
}
