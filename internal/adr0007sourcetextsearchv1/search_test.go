package adr0007sourcetextsearchv1

import "testing"

func TestLiteralOverlapsAndUTF16(t *testing.T) {
	rs := LiteralMatches([]byte("ababa"), "aba")
	if len(rs) != 2 || rs[0].ByteStart != 0 || rs[1].ByteStart != 2 {
		t.Fatalf("overlap mismatch: %#v", rs)
	}
	src := []byte("A😀A")
	rs = LiteralMatches(src, "A")
	if got := rs[1].UTF16Start; got != 3 {
		t.Fatalf("non-BMP UTF16 offset=%d", got)
	}
}

func TestSearchFailClosedEqualityAndLimits(t *testing.T) {
	src := []byte("aa")
	in := Input{SchemaVersion: SchemaInputV1, Query: "a", Source: SourceBinding{Path: "p", Revision: "r", FileSHA256: "sha256:961b6dd3ede3cb8ecbaacbd68de040cd78eb2ed5889130cceb4c49268ea4d506", ObjectID: "o", AdmissionID: "a", Seal: "s"}, Limits: Limits{MaxMatches: 2, MaxOutputBytes: 50000, MaxWork: 50000, MaxPathBytes: 10, MaxSourceBytes: 2}}
	res, fail := Search(in, src)
	if fail != nil {
		t.Fatalf("unexpected failure: %#v", fail)
	}
	if len(res.Matches) != 2 || res.Accounting.Work > in.Limits.MaxWork {
		t.Fatalf("bad result %#v", res)
	}
	in.Limits.MaxMatches = 1
	_, fail = Search(in, src)
	if fail == nil || fail.Accounting.Work != 0 || fail.Accounting.Matches != 0 {
		t.Fatalf("failure not zero-accounted: %#v", fail)
	}
}

func TestLineEndingsAndBareCR(t *testing.T) {
	src := []byte("x\r\nx\nx\ry")
	rs := LiteralMatches(src, "x")
	if len(rs) != 3 || rs[1].ByteStart != 3 || rs[2].ByteStart != 5 {
		t.Fatalf("line ending offsets %#v", rs)
	}
}

func TestLocationCandidateNeverExecutes(t *testing.T) {
	c := ComposeLocationCandidate("c943a484/root sha1195...", "f0f8b49a", "16f40dcb", "f498...", SourceBinding{Path: "p"}, []Match{{Range: Range{ByteStart: 1, ByteEnd: 2}}})
	if c.ExecutedLocation || len(c.Ranges) != 1 {
		t.Fatalf("bad candidate %#v", c)
	}
}
