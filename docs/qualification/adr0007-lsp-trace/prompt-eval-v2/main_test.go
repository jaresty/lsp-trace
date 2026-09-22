package main

import (
	"encoding/json"
	"strings"
	"testing"
)

const validPayload = `{"verdict":"COMPLETE","target_role":"endpoint handoff value","consumer_need":{"status":"RESOLVED","value":"a handoff identifier"},"provided_behavior":{"value":"returns the stored handoff identifier","consumer_relative":true},"boundary_contribution":"supplies that identifier one layer outward","limitations":["bounded to C1 and C2"]}`

func TestStrictV2Parser(t *testing.T) {
	for name, tc := range map[string]struct {
		raw string
		ok  bool
	}{
		"valid":          {validPayload, true},
		"duplicate":      {strings.Replace(validPayload, `"verdict":"COMPLETE"`, `"verdict":"COMPLETE","verdict":"ABSTAINED"`, 1), false},
		"unknown":        {strings.Replace(validPayload, `"target_role":`, `"consumer_identity":"invented","target_role":`, 1), false},
		"trailing":       {validPayload + `{}`, false},
		"bad unresolved": {strings.Replace(validPayload, `"status":"RESOLVED","value":"a handoff identifier"`, `"status":"UNRESOLVED","value":"invented"`, 1), false},
	} {
		t.Run(name, func(t *testing.T) {
			_, canonical, err := strictParse([]byte(tc.raw))
			if (err == nil) != tc.ok {
				t.Fatalf("ok=%t err=%v", tc.ok, err)
			}
			if tc.ok {
				var x any
				if json.Unmarshal(canonical, &x) != nil || strings.Contains(string(canonical), "\n") {
					t.Fatalf("ASSERT_V2_CANONICAL %q", canonical)
				}
			}
		})
	}
}

func TestExactMatrixAndWinnerGate(t *testing.T) {
	var got []string
	for _, v := range []string{"D", "E", "F"} {
		for _, p := range []string{"real", "final-01", "final-02", "final-03"} {
			got = append(got, v+"/"+p)
		}
	}
	want := "D/real,D/final-01,D/final-02,D/final-03,E/real,E/final-01,E/final-02,E/final-03,F/real,F/final-01,F/final-02,F/final-03"
	if strings.Join(got, ",") != want {
		t.Fatalf("ASSERT_EXACT_12_MATRIX %v", got)
	}
	if !qualifies(variantScore{Strict: 3, Substantive: 3, NoInvention: 4}) {
		t.Fatal("ASSERT_WINNER_GATE_ACCEPT")
	}
	if qualifies(variantScore{Strict: 4, Substantive: 4, NoInvention: 3}) {
		t.Fatal("ASSERT_WINNER_GATE_REJECT_INVENTION")
	}
}

func TestEOGControlEmitsOnceAndStops(t *testing.T) {
	calls := 0
	stopped := emitAndStop(workerResult{Status: "COMPLETE", Text: validPayload}, func(got workerResult) {
		calls++
		if got.Status != "COMPLETE" {
			t.Fatalf("ASSERT_EOG_COMPLETE got=%s", got.Status)
		}
	})
	if !stopped || calls != 1 {
		t.Fatalf("ASSERT_EOG_SINGLE_EMISSION stopped=%t calls=%d", stopped, calls)
	}
}

func TestGrammarUsesRuntimeCompatibleRuleNames(t *testing.T) {
	if strings.Contains(grammar, "maybe_string") || !strings.Contains(grammar, "maybe-string") {
		t.Fatal("ASSERT_GBNF_RULE_NAME_COMPATIBILITY")
	}
}

func TestPromptVariantsContainNoExampleAndRequiredDirectives(t *testing.T) {
	for _, v := range []string{"D", "E", "F"} {
		got := renderPrompt(v, "C1 packet-specific evidence")
		if strings.Contains(got, "example:") || strings.Contains(got, "for example") {
			t.Fatalf("ASSERT_NO_PROSE_EXAMPLE variant=%s", v)
		}
		if !strings.Contains(got, "PACKET:") || !strings.Contains(got, "citation_suggestions is optional") {
			t.Fatalf("ASSERT_PROMPT_CONTROLS variant=%s", v)
		}
	}
	if !strings.Contains(renderPrompt("E", "packet"), "derive each value from this packet, never copy generic wording") {
		t.Fatal("ASSERT_E_EXACT_DIRECTIVE")
	}
	if !strings.Contains(renderPrompt("F", "packet"), "Do not reveal reasoning or chain-of-thought") {
		t.Fatal("ASSERT_F_NO_COT")
	}
}
