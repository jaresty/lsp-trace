package adr0007sourcetextsearchv4oracle

import (
	"encoding/json"
	"testing"
)

func TestOverlapScan(t *testing.T) {
	m := Manifest{ToolingDigest: "sha256:t", PredecessorLockDigest: "sha256:p"}
	a := Attempt{SchemaVersion: "x", AttemptID: "a", Request: Request{Query: "aa", Policy: Policy{LiteralMode: "byte-literal"}, Limits: Limits{MaxFiles: 1, MaxMatches: 9, MaxOutputBytes: 9999, MaxPathBytes: 99, MaxSourceBytes: 99, MaxTotalBytes: 99, MaxWork: 99}, Sources: []SourceRef{{Path: "a.txt", Revision: "r", Ordinal: 1}}}, SourceInputs: []SourceInput{{Path: "a.txt", Revision: "r", Ordinal: 1, BytesBase64: "YWFhYQ=="}}}
	tr := EvaluateBytes(mustJSON(t, a), m)
	if tr.Outcome != "COMPLETE" || len(tr.Matches) != 3 {
		t.Fatalf("got %s %d failure=%+v", tr.Outcome, len(tr.Matches), tr.Failure)
	}
	for i, want := range []int{0, 1, 2} {
		if tr.Matches[i].ByteStart != want {
			t.Fatalf("match %d start=%d", i, tr.Matches[i].ByteStart)
		}
	}
}
func TestUTF16Positions(t *testing.T) {
	line, col := LineUTF16([]byte("😀x\nzz😀"), 4)
	if line != 0 || col != 2 {
		t.Fatalf("before x line=%d col=%d", line, col)
	}
	line, col = LineUTF16([]byte("😀x\nzz😀"), 8)
	if line != 1 || col != 2 {
		t.Fatalf("second emoji line=%d col=%d", line, col)
	}
}
func TestRejectDuplicateKeys(t *testing.T) {
	if err := rejectDuplicateKeys([]byte(`{"a":1,"a":2}`)); err == nil {
		t.Fatal("wanted duplicate error")
	}
}
func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := Canonical(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestTerminalRejectsPreviouslyOmittedNormativeFields(t *testing.T) {
	m := Manifest{ToolingDigest: "sha256:t", PredecessorLockDigest: "sha256:p"}
	a := Attempt{SchemaVersion: "x", AttemptID: "a", Request: Request{Query: "aa", Policy: Policy{LiteralMode: "byte-literal"}, Limits: Limits{MaxFiles: 1, MaxMatches: 9, MaxOutputBytes: 9999, MaxPathBytes: 99, MaxSourceBytes: 99, MaxTotalBytes: 99, MaxWork: 99}, Sources: []SourceRef{{Path: "a.txt", Revision: "r", Ordinal: 1}}}, SourceInputs: []SourceInput{{Path: "a.txt", Revision: "r", Ordinal: 1, BytesBase64: "YWFhYQ=="}}}
	tr := EvaluateBytes(mustJSON(t, a), m)
	finalizeTerminal(&tr)
	b, err := Canonical(tr)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"attempt_id", "terminal_sequence", "authority", "accepted", "completeness", "featureIdentity", "failure", "admission", "matches", "range_union_candidate", "accounting", "custody", "replay"} {
		var obj map[string]any
		if err := json.Unmarshal(b, &obj); err != nil {
			t.Fatal(err)
		}
		delete(obj, field)
		mut, _ := Canonical(obj)
		if err := ValidateTerminalBytes(mut); err == nil {
			t.Fatalf("expected missing %s to fail", field)
		}
	}
}
