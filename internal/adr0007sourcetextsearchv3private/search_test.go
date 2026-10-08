package adr0007sourcetextsearchv3private

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func attemptForTest(query string, pairs ...[2]string) Attempt {
	refs := []SourceRef{}
	inputs := []SourceInput{}
	for i, p := range pairs {
		b := []byte(p[1])
		fd := digest(b)
		od := digest(append([]byte("object:"), b...))
		refs = append(refs, SourceRef{Ordinal: uint64(i), Path: p[0], Revision: "rev", FileDigest: fd, ObjectDigest: od})
		inputs = append(inputs, SourceInput{SchemaVersion: "lsp-trace.adr0007.source-text-search.source-input.private.v3", Ordinal: uint64(i), Path: p[0], Revision: "rev", FileDigest: fd, ObjectDigest: od, BytesBase64: base64.StdEncoding.EncodeToString(b)})
	}
	return Attempt{SchemaVersion: "lsp-trace.adr0007.source-text-search.attempt.private.v3", AttemptID: "attempt-1234567890abcdef1234567890abcdef", Request: Request{SchemaVersion: "lsp-trace.adr0007.source-text-search.request.private.v3", Query: query, Sources: refs, Policy: Policy{SchemaVersion: "lsp-trace.adr0007.source-text-search.policy.private.v3", LiteralMode: "EXACT_NONEMPTY_CASE_SENSITIVE_UTF8"}, Limits: Limits{SchemaVersion: "lsp-trace.adr0007.source-text-search.limits.private.v3", MaxFiles: 100, MaxMatches: 100, MaxWork: ^uint64(0), MaxOutputBytes: 1 << 24, MaxSourceBytes: 1 << 20, MaxTotalBytes: 1 << 20, MaxPathBytes: 4096}, LocationPin: LocationPin{SchemaVersion: "lsp-trace.adr0007.source-text-search.location-pin.private.v3", Operation: "RANGE_UNION"}}, ExecutionControl: ExecutionControl{SchemaVersion: "lsp-trace.adr0007.source-text-search.execution-control.private.v3"}, SourceInputs: inputs}
}

func TestOverlapAndAccounting(t *testing.T) {
	tr := Evaluate(attemptForTest("aba", [2]string{"b.txt", "ababa"}))
	if tr.Outcome != "COMPLETE" || len(tr.Matches) != 2 {
		t.Fatalf("outcome=%s matches=%d failure=%v", tr.Outcome, len(tr.Matches), tr.Failure)
	}
	if tr.Matches[0].ByteRange.Start != 0 || tr.Matches[1].ByteRange.Start != 2 {
		t.Fatalf("bad ranges: %#v", tr.Matches)
	}
	if tr.Accounting.MMatches != 2 || tr.Accounting.RRanges != 2 || tr.Accounting.UUTF16Units != 6 || tr.Accounting.WWork == 0 || tr.Accounting.BOutputBytes == 0 {
		t.Fatalf("bad accounting: %#v", tr.Accounting)
	}
	if tr.Custody.TerminalSequence != 0 || tr.Custody.TerminalCount != 1 || strings.Contains(tr.Custody.TerminalResultSHA256, strings.Repeat("0", 64)) {
		t.Fatalf("bad custody: %#v", tr.Custody)
	}
	if tr.RangeUnionCandidate == nil || len(tr.RangeUnionCandidate.Members) != 2 || tr.RangeUnionCandidate.Operation != "RANGE_UNION" || tr.RangeUnionCandidate.ExecutedLocation {
		t.Fatalf("bad candidate: %#v", tr.RangeUnionCandidate)
	}
}

func TestUTF16Positions(t *testing.T) {
	tr := Evaluate(attemptForTest("x", [2]string{"a.txt", "😀x\r\ny"}))
	if tr.Outcome != "COMPLETE" || len(tr.Matches) != 1 {
		t.Fatalf("bad terminal: %#v", tr)
	}
	got := tr.Matches[0].LSPUTF16Range.Start
	if got.Line != 0 || got.Character != 2 {
		t.Fatalf("got position %#v", got)
	}
}

func TestDeadlineBeatsCancel(t *testing.T) {
	a := attemptForTest("a", [2]string{"a.txt", "abc"})
	a.ExecutionControl.Observations = []Observation{{PollIndex: 0, Cancelled: true, DeadlineExpired: true}}
	tr := Evaluate(a)
	if tr.Outcome != "FAILED" || tr.Failure == nil || tr.Failure.Code != "DEADLINE_EXCEEDED" || tr.Accounting.FailureCounters["DEADLINE_EXCEEDED"] != 1 {
		t.Fatalf("bad terminal: %#v", tr)
	}
}

func TestResourceLimitPlusOne(t *testing.T) {
	a := attemptForTest("a", [2]string{"a.txt", "aaa"})
	a.Request.Limits.MaxMatches = 2
	tr := Evaluate(a)
	if tr.Outcome != "FAILED" || tr.Failure == nil || tr.Failure.Code != "RESOURCE_EXHAUSTED" {
		t.Fatalf("bad terminal: %#v", tr)
	}
}

func TestCorpusDiversity(t *testing.T) {
	root := filepath.Join("..", "..", "docs", "pilot", "adr0007", "source-text-search-v3", "corpus")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 48 {
		t.Fatalf("cases=%d", len(entries))
	}
	seenInputs := map[string]bool{}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(root, e.Name(), "attempt.json"))
		if err != nil {
			t.Fatal(err)
		}
		var a Attempt
		if err := json.Unmarshal(b, &a); err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		for _, in := range a.SourceInputs {
			seenInputs[in.FileDigest] = true
		}
		tr := Evaluate(a)
		if tr.TerminalSequence != 0 || tr.Custody.TerminalCount != 1 {
			t.Fatalf("%s no exact terminal", e.Name())
		}
	}
	if len(seenInputs) < 48 {
		t.Fatalf("unique input digests=%d", len(seenInputs))
	}
}
