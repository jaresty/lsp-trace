package vcssidecar

import "testing"

func TestParseGitNumstatAggregatesPerPathAndCommit(t *testing.T) {
	raw := []byte("C\x00new\x00\x00\n3\t1\ta.go\x002\t0\ta.go\x00-\t-\tb.go\x00C\x00old\x00\x00\n4\t5\ta.go\x00")
	got, err := parseGitNumstat(raw, map[string]struct{}{"a.go": {}, "b.go": {}, "c.go": {}})
	if err != nil {
		t.Fatal(err)
	}
	if a := got["a.go"]; a.CommitCount != 2 || a.LinesAdded != 9 || a.LinesDeleted != 6 || a.LastRevision != "new" {
		t.Fatalf("a.go: %+v", a)
	}
	if b := got["b.go"]; b.CommitCount != 1 || b.BinaryChanges != 1 || b.LastRevision != "new" {
		t.Fatalf("b.go: %+v", b)
	}
	if _, ok := got["c.go"]; ok {
		t.Fatalf("zero-churn paths must be omitted by history source: %+v", got)
	}
}

func TestParseGitNumstatRejectsUnexpectedPathAndMalformedRecord(t *testing.T) {
	for _, raw := range [][]byte{
		[]byte("C\x00new\x00\x00\n1\t0\tother.go\x00"),
		[]byte("C\x00new\x00\x00\nwat\x00"),
		[]byte("1\t0\ta.go\x00"),
	} {
		if _, err := parseGitNumstat(raw, map[string]struct{}{"a.go": {}}); err == nil {
			t.Fatalf("accepted malformed stream %q", raw)
		}
	}
}
