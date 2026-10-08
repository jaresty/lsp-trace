package adr0007sourcetextsearchv4oracle

import (
	"encoding/json"
	"testing"

	validator "lsp-trace/internal/adr0007v4contractvalidator"
)

func TestOverlapScan(t *testing.T) {
	m := Manifest{ToolingDigest: "sha256:t", PredecessorLockDigest: "sha256:p"}
	a := testAttempt()
	res, err := EvaluateBytesResult(mustJSON(t, a), m)
	if err != nil {
		t.Fatal(err)
	}
	tr := res.Terminal
	if tr.Terminal != "COMPLETE" || len(tr.Matches) != 3 {
		t.Fatalf("got %s %d failure=%+v", tr.Terminal, len(tr.Matches), tr.Failure)
	}
	for i, want := range []uint64{0, 1, 2} {
		if tr.Matches[i].StartByte != want {
			t.Fatalf("match %d start=%d", i, tr.Matches[i].StartByte)
		}
	}
	if err := ValidateResult(res); err != nil {
		t.Fatal(err)
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
func testAttempt() Attempt {
	return Attempt{SchemaVersion: AttemptSchema, AttemptID: "a", ExecutionControl: ExecutionControl{SchemaVersion: ExecutionControlSchema}, Request: Request{SchemaVersion: RequestSchema, Query: "aa", Policy: Policy{SchemaVersion: PolicySchema, LiteralMode: "byte-literal"}, Limits: Limits{SchemaVersion: LimitsSchema, MaxFiles: 1, MaxMatches: 9, MaxOutputBytes: 99999, MaxPathBytes: 99, MaxSourceBytes: 99, MaxTotalBytes: 99, MaxWork: 999999}, Sources: []SourceRef{{Path: "a.txt", Revision: "r", Ordinal: 1}}}, SourceInputs: []SourceInput{{SchemaVersion: SourceInputSchema, Path: "a.txt", Revision: "r", Ordinal: 1, BytesBase64: "YWFhYQ=="}}}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := Canonical(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestTerminalRejectsMissingNormativeFields(t *testing.T) {
	m := Manifest{ToolingDigest: "sha256:t", PredecessorLockDigest: "sha256:p"}
	a := testAttempt()
	res, err := EvaluateBytesResult(mustJSON(t, a), m)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Canonical(res.Terminal)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"attempt", "terminal", "request", "admission", "sources", "matches", "positions", "range_union_candidate", "accounting", "failure", "custody", "replay", "payload"} {
		var obj map[string]any
		if err := json.Unmarshal(b, &obj); err != nil {
			t.Fatal(err)
		}
		delete(obj, field)
		mut, _ := Canonical(obj)
		bun := validator.Bundle{SchemaBytes: res.SchemaBytes, RawAttemptBytes: res.RawAttemptBytes, TerminalBytes: string(mut), AdmittedSourceBytes: res.AdmittedSourceBytes, AdmittedBindingBytes: res.AdmittedBindingBytes, ToolingManifestBytes: res.ToolingManifestBytes, PredecessorManifestBytes: res.PredecessorBytes, PayloadFreezeBytes: res.PayloadFreezeBytes}
		if err := validator.ValidateBundle(bun); err == nil {
			t.Fatalf("expected missing %s to fail", field)
		}
	}
}
