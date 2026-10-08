package adr0007sourcetextsearchv4private

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestMutationSensitivityAgainstFrozenOracleBytes(t *testing.T) {
	root := filepath.Join("..", "..", "docs", "pilot", "adr0007", "source-text-search-v4", "cases")
	cases := []struct {
		name, id string
		mutate   func(*TerminalResult)
	}{
		{"overlap stride +2", "case-25-overlap_literal", func(tr *TerminalResult) {
			if len(tr.Matches) > 1 {
				tr.Matches = tr.Matches[:1]
			}
		}},
		{"UTF16 nonBMP unit 1", "case-33-non_bmp_utf16", func(tr *TerminalResult) { tr.Accounting.UUTF16Units = 1 }},
		{"deadline/cancel order swap", "case-09-control_simultaneous_deadline_cancel", func(tr *TerminalResult) {
			if tr.Failure != nil {
				tr.Failure.Code = "CANCELLED"
			}
		}},
		{"accounting coefficient", "case-29-lf_positions", func(tr *TerminalResult) { tr.Accounting.WWork++ }},
		{"limit > to >=", "case-43-max_matches_equal", func(tr *TerminalResult) {
			tr.Outcome = "FAILED"
			tr.Failure = &Failure{Code: "RESOURCE_EXHAUSTED", Detail: map[string]any{"limit": "max_matches"}}
		}},
		{"admission bypass", "case-21-file_digest_mutation", func(tr *TerminalResult) { tr.Outcome = "COMPLETE"; tr.Failure = nil }},
		{"range member reorder", "case-24-multi_path_byte_order", func(tr *TerminalResult) {
			for i, j := 0, len(tr.Matches)-1; i < j; i, j = i+1, j-1 {
				tr.Matches[i], tr.Matches[j] = tr.Matches[j], tr.Matches[i]
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(root, tc.id, "attempt.json"))
			if err != nil {
				t.Fatal(err)
			}
			exp, err := os.ReadFile(filepath.Join(root, tc.id, "expected.oracle.terminal.json"))
			if err != nil {
				t.Fatal(err)
			}
			tr := EvaluateRaw(raw)
			if !bytes.Equal(Canon(tr), exp) {
				t.Fatalf("baseline production no longer matches oracle for %s", tc.id)
			}
			tc.mutate(&tr)
			if bytes.Equal(Canon(tr), exp) {
				t.Fatalf("mutation %s was not detected by oracle bytes", tc.name)
			}
		})
	}
}
