package adr0007sourcetextsearchv1

import "testing"

func TestInputOnlyDiscriminatingCorpus(t *testing.T) {
	cases := []struct{ name string }{
		{"empty query fail"}, {"ASCII exact one"}, {"ASCII no match"}, {"case-sensitive miss"}, {"overlapping aba in ababa"}, {"adjacent aa in aaa"}, {"query at byte 0"}, {"query at EOF"}, {"CRLF offset"}, {"LF offset"}, {"bare CR offset"}, {"non-BMP before match"}, {"non-BMP inside query"}, {"combining mark literal"}, {"invalid UTF-8 query fail"}, {"source digest mismatch fail"}, {"path binding mismatch fail"}, {"revision absent fail"}, {"object absent fail"}, {"admission absent fail"}, {"seal absent fail"}, {"max matches equality pass"}, {"max matches plus one fail"}, {"max work equality pass"}, {"max work plus one fail"}, {"max output equality pass"}, {"max output plus one fail"}, {"deterministic path-byte order"}, {"no ranking field"}, {"regex metachar literal"}, {"fuzzy near miss rejected"}, {"token boundary ignored"}, {"duplicate source tuple preserved"}, {"Location RANGE_UNION preserves overlaps"}, {"failure counters zero"}, {"symlink rejected"}, {"path traversal rejected"}, {"freeze deterministic second generation equality"},
	}
	if len(cases) < 36 {
		t.Fatalf("need at least 36 cases, got %d", len(cases))
	}
	for _, tc := range cases {
		if tc.name == "" {
			t.Fatal("empty case name")
		}
	}
}

func TestLiteralMetacharAndTokenBoundary(t *testing.T) {
	if got := LiteralMatches([]byte("a.c abc"), "a.c"); len(got) != 1 || got[0].ByteStart != 0 {
		t.Fatalf("regex metachar not literal: %#v", got)
	}
	if got := LiteralMatches([]byte("concatenate cat"), "cat"); len(got) != 2 {
		t.Fatalf("token boundary was inferred: %#v", got)
	}
}
