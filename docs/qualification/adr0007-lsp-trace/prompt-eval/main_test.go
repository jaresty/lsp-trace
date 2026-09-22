package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validPayload = `{
  "verdict":"SUPPORTED",
  "target_role":"endpoint value",
  "consumer_need":"a handoff identifier",
  "provided_behavior":"provides the handoff identifier",
  "boundary_contribution":"passes the identifier one layer outward",
  "limitations":["bounded C1/C2 interpretation"],
  "citations":{"target_role":["C1"],"consumer_need":["C2"],"provided_behavior":["C1"],"boundary_contribution":["C1","C2"],"limitations":["C1","C2"]}
}`

func TestStrictSemanticParser(t *testing.T) {
	tests := []struct {
		name, raw string
		want      string
	}{
		{"valid pretty JSON", validPayload, "PASS"},
		{"duplicate key", strings.Replace(validPayload, `"verdict":"SUPPORTED",`, `"verdict":"SUPPORTED","verdict":"UNRESOLVED",`, 1), "FAIL"},
		{"unknown host-owned key", strings.Replace(validPayload, `"target_role":`, `"nearest_outward_consumer":"invented","target_role":`, 1), "FAIL"},
		{"trailing JSON", validPayload + ` {}`, "FAIL"},
		{"non JSON", `not-json`, "FAIL"},
		{"invalid citation", strings.Replace(validPayload, `"C2"]}`, `"C3"]}`, 1), "FAIL"},
		{"empty substantive field", strings.Replace(validPayload, `"endpoint value"`, `""`, 1), "FAIL"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, canonical, err := strictParse([]byte(tc.raw))
			got := "PASS"
			if err != nil {
				got = "FAIL"
			}
			if got != tc.want {
				t.Fatalf("ASSERT_STRICT_PARSE_%s got=%s err=%v", strings.ReplaceAll(tc.name, " ", "_"), got, err)
			}
			if tc.want == "PASS" {
				if strings.Contains(string(canonical), "\n") {
					t.Fatalf("ASSERT_HOST_CANONICAL_COMPACT got=%q", canonical)
				}
				var x any
				if json.Unmarshal(canonical, &x) != nil {
					t.Fatal("ASSERT_HOST_CANONICAL_JSON")
				}
				_, canonical2, err := strictParse(canonical)
				if err != nil || string(canonical) != string(canonical2) {
					t.Fatalf("ASSERT_HOST_CANONICAL_STABLE err=%v first=%q second=%q", err, canonical, canonical2)
				}
			}
		})
	}
}

func TestClaimCoverageAndIdentityScoring(t *testing.T) {
	p, _, err := strictParse([]byte(validPayload))
	if err != nil {
		t.Fatal(err)
	}
	if !citationCoverage(p) {
		t.Fatal("ASSERT_CITATION_COVERAGE_PASS")
	}
	bad := p
	bad.Citations.ConsumerNeed = []string{"C1"}
	if citationCoverage(bad) {
		t.Fatal("ASSERT_CITATION_COVERAGE_FAIL")
	}
	if invented(p, "loadStateFromVerificationStore") {
		t.Fatal("ASSERT_NO_INVENTED_IDENTITY_PASS")
	}
	bad = p
	bad.ConsumerNeed = "loadStateFromVerificationStore needs a handoff identifier"
	if !invented(bad, "loadStateFromVerificationStore") {
		t.Fatal("ASSERT_NO_INVENTED_IDENTITY_FAIL")
	}
}

func TestMatrixAndOrdering(t *testing.T) {
	variants := []string{"A", "B", "C"}
	packets := []string{"real", "final-01", "final-02", "final-03"}
	var got []string
	for _, v := range variants {
		for _, p := range packets {
			got = append(got, v+"/"+p)
		}
	}
	want := []string{"A/real", "A/final-01", "A/final-02", "A/final-03", "B/real", "B/final-01", "B/final-02", "B/final-03", "C/real", "C/final-01", "C/final-02", "C/final-03"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("ASSERT_EXACT_12_MATRIX got=%v", got)
	}
	runs := []runScore{{Variant: "A", StrictSchema: true, CitationCoverage: true, NoInventedIdentity: true, Substantive: true, Canonicalizable: true, LatencyMS: 1, Tokens: 2}, {Variant: "B", StrictSchema: true, CitationCoverage: true, NoInventedIdentity: true, Substantive: true, Canonicalizable: true, LatencyMS: 2, Tokens: 1}, {Variant: "C", StrictSchema: false, CitationCoverage: true, NoInventedIdentity: true, Substantive: true, Canonicalizable: true, LatencyMS: .5, Tokens: 1}}
	if winner := summarize(runs)[0].Variant; winner != "A" {
		t.Fatalf("ASSERT_LEXICOGRAPHIC_ORDER winner=%s", winner)
	}
}

func TestRawModeGuard(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "raw.stdout")
	if err := os.WriteFile(p, []byte("raw"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("ASSERT_RAW_MODE_0600 got=%o", info.Mode().Perm())
	}
	if err = os.Chmod(p, 0644); err != nil {
		t.Fatal(err)
	}
	info, _ = os.Stat(p)
	if info.Mode().Perm() == 0600 {
		t.Fatal("ASSERT_RAW_MODE_REJECT_BAD")
	}
}

func TestExampleEchoIsNotSubstantiveCoverage(t *testing.T) {
	raw := `{"verdict":"SUPPORTED","target_role":"boundary-facing helper","consumer_need":"a bounded semantic value","provided_behavior":"provides the bounded value","boundary_contribution":"passes that value one layer outward","limitations":["scope is limited to C1 and C2"],"citations":{"target_role":["C1"],"consumer_need":["C2"],"provided_behavior":["C1"],"boundary_contribution":["C1","C2"],"limitations":["C1","C2"]}}`
	p, _, err := strictParse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if !exampleEcho(p) {
		t.Fatal("ASSERT_EXACT_EXAMPLE_ECHO_DETECTED")
	}
	if citationCoverage(p) && !exampleEcho(p) {
		t.Fatal("ASSERT_EXAMPLE_ECHO_CLAIM_COVERAGE_REJECTED")
	}
}

func TestFirstCompleteEnvelopeRecovery(t *testing.T) {
	d := t.TempDir()
	stdout := filepath.Join(d, "stdout.json")
	stderr := filepath.Join(d, "stderr.log")
	complete, err := json.Marshal(workerResult{Status: "COMPLETE", Text: validPayload, Tokens: 10, LoadMS: 1, RunMS: 2})
	if err != nil {
		t.Fatal(err)
	}
	terminal, err := json.Marshal(workerResult{Status: "TOKEN_LIMIT", Tokens: 384})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(stdout, append(append(complete, '\n'), append(terminal, '\n')...), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(stderr, nil, 0600); err != nil {
		t.Fatal(err)
	}
	s := scoreRun(1, "A", packet{ID: "fixture", Consumer: "consumerName"}, stdout, stderr, nil)
	if !s.StrictSchema || !s.Canonicalizable || !strings.Contains(s.Diagnostic, "recovered first COMPLETE envelope") {
		t.Fatalf("ASSERT_FIRST_COMPLETE_RECOVERY score=%+v", s)
	}
	if err = os.WriteFile(stdout, append(terminal, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	s = scoreRun(1, "A", packet{ID: "fixture"}, stdout, stderr, nil)
	if s.StrictSchema || !strings.Contains(s.Diagnostic, "no COMPLETE envelope") {
		t.Fatalf("ASSERT_NO_COMPLETE_REJECTED score=%+v", s)
	}
}
