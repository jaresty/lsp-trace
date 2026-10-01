package adr0011querytarget

import (
	"errors"
	"fmt"
	"testing"
)

func TestDocumentSymbolCandidateV1CrossingOuterCommonInner(t *testing.T) {
	q := candidateQuery()
	q.Character = 5
	item := func(name string, start, end int) string {
		return fmt.Sprintf(`{"name":%q,"kind":12,"range":{"start":{"line":1,"character":0},"end":{"line":4,"character":0}},"selectionRange":{"start":{"line":2,"character":%d},"end":{"line":2,"character":%d}}}`, name, start, end)
	}
	a, b, inner := item("OuterA", 0, 8), item("OuterB", 2, 10), item("Inner", 3, 7)
	for _, order := range [][]string{{a, b, inner}, {inner, a, b}, {b, inner, a}, {inner, b, a}} {
		raw := []byte("[" + order[0] + "," + order[1] + "," + order[2] + "]")
		got, err := SelectDocumentSymbolCandidateV1(q, raw)
		if err != nil || got.SymbolName != "Inner" || got.DisplayRange.Start != (Position{1, 0}) || got.SelectionRange.Start != (Position{2, 3}) {
			t.Fatalf("ASSERT_CROSSING_COMMON_INNER_PERMUTATION: %+v %v", got, err)
		}
	}
	for name, items := range map[string][]string{
		"crossing_without_inner": {a, b},
		"duplicate_inner":        {a, b, inner, inner},
	} {
		raw := []byte("[")
		for i, s := range items {
			if i > 0 {
				raw = append(raw, ',')
			}
			raw = append(raw, s...)
		}
		raw = append(raw, ']')
		got, err := SelectDocumentSymbolCandidateV1(q, raw)
		if !errors.Is(err, ErrUnresolved) || got != (Candidate{}) {
			t.Fatalf("ASSERT_%s_AMBIGUOUS: %+v %v", name, got, err)
		}
	}
	// Proper subset (the frozen rule) permits one shared endpoint; equal
	// ranges do not. This inner touches OuterB's start but ends strictly before it.
	touching := item("Touching", 2, 7)
	raw := []byte("[" + a + "," + b + "," + touching + "]")
	got, err := SelectDocumentSymbolCandidateV1(q, raw)
	if err != nil || got.SymbolName != "Touching" {
		t.Fatalf("ASSERT_PROPER_SUBSET_TOUCHING_BOUNDARY: %+v %v", got, err)
	}
}
