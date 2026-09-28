package adr0011methodresult

import (
	"encoding/json"
	"fmt"
	"lsp-trace/internal/lspwire"
	"strings"
	"testing"
)

func TestADR0011CompleteReferenceEvaluation(t *testing.T) {
	valid := `{"uri":"file:///one","range":{"start":{"line":1,"character":0},"end":{"line":1,"character":1}}}`
	cases := []struct {
		name, raw, outcome, disposition string
		e, eb, et, p, a, ordinal        int
	}{
		{"null", "null", "COMPLETE_EMPTY", "EMPTY", 0, 0, 0, 0, 0, -1},
		{"array-empty", "[]", "COMPLETE_EMPTY", "EMPTY", 0, 0, 0, 0, 0, -1},
		{"equal-locations", "[" + valid + "," + valid + "]", "COMPLETE", "ITEMS", 2, 2, 2, 2, 2, -1},
		{"malformed-first", "[{\"uri\":9}," + valid + "]", "MALFORMED", "MALFORMED", 2, 1, 1, 0, 0, 0},
		{"malformed-later", "[" + valid + ",{\"uri\":9}]", "MALFORMED", "MALFORMED", 2, 2, 2, 0, 0, 1},
		{"duplicate-later", "[" + valid + ",{\"uri\":\"file:///a\",\"uri\":\"file:///b\",\"range\":{\"start\":{\"line\":1,\"character\":0},\"end\":{\"line\":1,\"character\":1}}}]", "MALFORMED", "MALFORMED", 2, 2, 2, 0, 0, 1},
		{"exact-limit", "[" + strings.TrimSuffix(strings.Repeat(valid+",", 1000), ",") + "]", "COMPLETE", "ITEMS", 1000, 1000, 1000, 1000, 1000, -1},
		{"over-limit", "[" + strings.TrimSuffix(strings.Repeat(valid+",", 1001), ",") + "]", "RESOURCE_LIMIT", "LIMITED", 1001, 0, 0, 0, 0, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key := lspwire.RequestKey{Generation: 7, ID: 31}
			got := evaluateCompleteReferences(key, json.RawMessage(tc.raw))
			if got.Key != key || got.RawDigest != chainDigest([]byte(tc.raw)) || got.N != 1 || got.B != 1 || got.T != 0 || got.E != tc.e || got.EB != tc.eb || got.ET != tc.et || got.P != tc.p || got.A != 0 || got.Outcome != tc.outcome || got.Disposition != tc.disposition || got.FailureOrdinal != tc.ordinal {
				t.Fatalf("ASSERT_ADR0011_EVALUATION_%s: got=%+v want E/EB/ET/P/A=%d/%d/%d/%d/%d outcome=%s ordinal=%d", tc.name, got, tc.e, tc.eb, tc.et, tc.p, tc.a, tc.outcome, tc.ordinal)
			}
			begun, terminal := 0, 0
			for _, event := range got.Events {
				if event.Transition == "BEGIN" {
					begun++
				}
				if event.Transition == "TERMINAL" {
					terminal++
				}
			}
			if begun != got.EB || terminal != got.ET {
				t.Fatalf("ASSERT_ADR0011_EVALUATION_EVENTS_%s: begin=%d terminal=%d counts=%d/%d", tc.name, begun, terminal, got.EB, got.ET)
			}
			if got.P != len(got.Items) || got.A != 0 {
				t.Fatalf("ASSERT_ADR0011_EVALUATION_ADMISSION_%s: %s", tc.name, fmt.Sprint(got))
			}
		})
	}
}
