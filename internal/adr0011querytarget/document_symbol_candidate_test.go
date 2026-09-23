package adr0011querytarget

import (
	"errors"
	"fmt"
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

func TestDocumentSymbolCandidateV1NestedStrictSelection(t *testing.T) {
	q := candidateQuery()
	raw := strings.Replace(hierarchical, `"line":0,"character":6`, `"line":5,"character":0`, 1)
	if _, err := SelectDocumentSymbolCandidate(q, []byte(raw)); !errors.Is(err, ErrUnresolved) {
		t.Fatalf("v0 nested selection must remain unresolved: %v", err)
	}
	got, err := SelectDocumentSymbolCandidateV1(q, []byte(raw))
	if err != nil || got.SymbolName != "Child" || got.Version != CandidateVersionV1 {
		t.Fatalf("v1 strict nested child: %+v %v", got, err)
	}
}

func TestDocumentSymbolCandidateV1BoundariesAndIdentity(t *testing.T) {
	q := candidateQuery()
	base := strings.Replace(hierarchical, `"line":0,"character":6`, `"line":5,"character":0`, 1)
	child := `{"name":"Other","kind":12,"range":{"start":{"line":2,"character":0},"end":{"line":5,"character":0}},"selectionRange":{"start":{"line":2,"character":2},"end":{"line":2,"character":7}}}`
	cases := []struct {
		name, raw string
		valid     bool
	}{
		{"strict chain", base, true},
		{"equal", strings.Replace(base, `"selectionRange":{"start":{"line":0,"character":0},"end":{"line":5,"character":0}}`, `"selectionRange":{"start":{"line":2,"character":2},"end":{"line":2,"character":7}}`, 1), false},
		{"duplicate", strings.Replace(base, `}]`, `},`+child+`]`, 1), false},
		{"three-level chain", strings.Replace(base, `}]`, `},{"name":"Deep","kind":12,"range":{"start":{"line":2,"character":0},"end":{"line":5,"character":0}},"selectionRange":{"start":{"line":2,"character":3},"end":{"line":2,"character":5}}}]`, 1), true},
		{"incomparable", strings.Replace(base, `}]`, `},{"name":"Overlap","kind":12,"range":{"start":{"line":2,"character":0},"end":{"line":5,"character":0}},"selectionRange":{"start":{"line":2,"character":3},"end":{"line":2,"character":8}}}]`, 1), false},
		{"malformed sibling", strings.Replace(base, `}]`, `},{"name":"Bad","kind":0}]`, 1), false},
		{"disjoint", strings.Replace(base, `}]`, `},{"name":"Far","kind":12,"range":{"start":{"line":3,"character":0},"end":{"line":4,"character":0}},"selectionRange":{"start":{"line":3,"character":0},"end":{"line":3,"character":1}}}]`, 1), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SelectDocumentSymbolCandidateV1(q, []byte(tc.raw))
			if tc.valid && (err != nil || got.SymbolName != map[bool]string{true: "Deep", false: "Child"}[tc.name == "three-level chain"]) || !tc.valid && (!errors.Is(err, ErrUnresolved) || got != (Candidate{})) {
				t.Fatalf("selection: %+v %v", got, err)
			}
		})
	}
	got, err := SelectDocumentSymbolCandidateV1(q, []byte(base))
	if err != nil {
		t.Fatal(err)
	}
	v0, err := SelectDocumentSymbolCandidate(q, []byte(hierarchical))
	if err != nil {
		t.Fatal(err)
	}
	if got.Version == v0.Version || got.ID == v0.ID || got.SymbolID == v0.SymbolID || got.DocumentSymbolResultSHA256 == v0.DocumentSymbolResultSHA256 {
		t.Fatal("v1 domains reused v0")
	}
	changed := q
	changed.DocumentVersion = "5"
	version, err := SelectDocumentSymbolCandidateV1(changed, []byte(base))
	if err != nil || version.SymbolID == got.SymbolID {
		t.Fatal("version identity unchanged", err)
	}
	changed = q
	changed.SourceDigest = "sha256:" + strings.Repeat("b", 64)
	source, err := SelectDocumentSymbolCandidateV1(changed, []byte(base))
	if err != nil || source.SymbolID == got.SymbolID {
		t.Fatal("source identity unchanged", err)
	}
	changed = q
	changed.Character = 4
	query, err := SelectDocumentSymbolCandidateV1(changed, []byte(base))
	if err != nil || query.ID == got.ID {
		t.Fatal("query identity unchanged", err)
	}
	result, err := SelectDocumentSymbolCandidateV1(q, []byte(base+" "))
	if err != nil || result.DocumentSymbolResultSHA256 == got.DocumentSymbolResultSHA256 {
		t.Fatal("result digest unchanged", err)
	}
}

func TestDocumentSymbolCandidateV1FinalizationBytes(t *testing.T) {
	q := candidateQuery()
	base := strings.Replace(hierarchical, `"line":0,"character":6`, `"line":5,"character":0`, 1)
	raw := strings.Replace(base, `}]`, `},{"name":"Deep","kind":12,"range":{"start":{"line":2,"character":0},"end":{"line":5,"character":0}},"selectionRange":{"start":{"line":2,"character":3},"end":{"line":2,"character":5}}}]`, 1)
	rawCalls, identityBuilds := 0, 0
	got, err := selectDocumentSymbolCandidateV1(q, []byte(raw), func(operation string) {
		switch operation {
		case "raw":
			rawCalls++
		case "identity":
			identityBuilds++
		default:
			t.Fatalf("unknown finalization operation: %s", operation)
		}
	})
	if err != nil || got.SymbolName != "Deep" {
		t.Fatalf("baseline fixture invalid: %+v %v", got, err)
	}
	if got.Version != "lsp-trace.private.adr0011-document-symbol-query-target.v1" || got.ID != "sha256:6b3b87d337e442d59732198b71f4f86f062e293710f386086a1c215cb1196329" || got.SymbolID != "sha256:b199896fd9c5e086407b281c87881a88581cb6ef0142353e13fbb6cd56f974a4" || got.DocumentSymbolResultSHA256 != "sha256:1125a87dceeeae88a2d3b338a5cd2be55e9ad448c562579cafc68a0b57a7878a" {
		t.Fatalf("ASSERT_V1_FINALIZATION_BYTES: chosen candidate changed: %+v", got)
	}
	if rawCalls != 1 || identityBuilds != 1 {
		t.Fatalf("ASSERT_V1_FINALIZATION_ONCE: HashRawCalls=%d IdentityBuilds=%d, want 1 each", rawCalls, identityBuilds)
	}
	t.Logf("ASSERT_V1_FINALIZATION_ONCE PASS HashRawCalls=%d IdentityBuilds=%d", rawCalls, identityBuilds)
}

func TestDocumentSymbolCandidateV1LinearComparisons(t *testing.T) {
	const n = 1000
	matches := make([]Candidate, n)
	for i := range matches {
		matches[i].SelectionRange = Range{Start: Position{Line: uint32(i)}, End: Position{Line: 2000}}
	}
	index, comparisons := mostSpecificV1(matches)
	if index != n-1 {
		t.Fatalf("ASSERT_V1_LINEAR_COMPARISONS: wrong strict minimum index=%d", index)
	}
	if comparisons > 2*n {
		t.Fatalf("ASSERT_V1_LINEAR_COMPARISONS: comparisons=%d exceed linear bound=%d", comparisons, 2*n)
	}
	fmt.Printf("ASSERT_V1_LINEAR_COMPARISONS PASS comparisons=%d bound=%d\n", comparisons, 2*n)
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
