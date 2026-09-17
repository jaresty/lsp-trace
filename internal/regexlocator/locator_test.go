package regexlocator

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"lsp-trace/internal/sourceposition"
)

func TestResolveDeterministicNthCaptureAndEncoding(t *testing.T) {
	raw := []byte("package p\nfunc First() {}\n// 😀\nfunc Target() {}\nfunc Target() {}\n")
	digest := fmt.Sprintf("sha256:%x", sha256.Sum256(raw))
	base := Request{Document: raw, Pattern: `func (Target)`, MatchIndex: 1, CaptureGroup: 1, ExpectedDigest: digest, Encoding: "utf-16", Limits: Limits{MaxDocumentBytes: 1024, MaxMatches: 10, MaxPatternBytes: 100, MaxWork: 2048}}
	got, err := Resolve(base)
	if err != nil || got.Position != (sourceposition.Position{Line: 4, Character: 5}) || got.MatchCount != 2 {
		t.Fatalf("ASSERT_REGEX_RESOLVE_NTH_CAPTURE_ENCODING: got=%+v err=%v", got, err)
	}
	again, err := Resolve(base)
	if err != nil || again != got {
		t.Fatalf("ASSERT_REGEX_RESOLVE_DETERMINISTIC: first=%+v again=%+v err=%v", got, again, err)
	}
}

func TestResolveFailsClosed(t *testing.T) {
	base := Request{Document: []byte("x x"), Pattern: "x", Encoding: "utf-8", Limits: Limits{MaxDocumentBytes: 10, MaxMatches: 2, MaxPatternBytes: 10, MaxWork: 20}}
	cases := []struct {
		name, assertion string
		mutate          func(*Request)
	}{
		{"invalid-regex", "ASSERT_REGEX_RESOLVE_INVALID_RE2", func(r *Request) { r.Pattern = "(" }},
		{"empty-match", "ASSERT_REGEX_RESOLVE_EMPTY_MATCH", func(r *Request) { r.Pattern = "x*" }},
		{"index", "ASSERT_REGEX_RESOLVE_INDEX", func(r *Request) { r.MatchIndex = 2 }},
		{"capture", "ASSERT_REGEX_RESOLVE_CAPTURE", func(r *Request) { r.CaptureGroup = 1 }},
		{"digest", "ASSERT_REGEX_RESOLVE_DIGEST", func(r *Request) {
			r.ExpectedDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}},
		{"document-limit", "ASSERT_REGEX_RESOLVE_DOCUMENT_LIMIT", func(r *Request) { r.Limits.MaxDocumentBytes = 2 }},
		{"match-limit", "ASSERT_REGEX_RESOLVE_MATCH_LIMIT", func(r *Request) { r.Limits.MaxMatches = 1 }},
		{"pattern-limit", "ASSERT_REGEX_RESOLVE_PATTERN_LIMIT", func(r *Request) { r.Limits.MaxPatternBytes = 0 }},
		{"work-limit", "ASSERT_REGEX_RESOLVE_WORK_LIMIT", func(r *Request) { r.Limits.MaxWork = 2 }},
		{"encoding", "ASSERT_REGEX_RESOLVE_ENCODING", func(r *Request) { r.Encoding = "UTF-16" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := base
			tc.mutate(&r)
			got, err := Resolve(r)
			if err == nil || got != (Result{}) {
				t.Fatalf("%s: got=%+v err=%v", tc.assertion, got, err)
			}
		})
	}
}
