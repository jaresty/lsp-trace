//go:build darwin || linux

package privatehostproductv3

import (
	pilot "lsp-trace/internal/censuslineagepilot"
	"testing"
)

func TestHostProductReportCitationOrdering(t *testing.T) {
	c := testConfig(t)
	if err := Initialize(c); err != nil {
		t.Fatal(err)
	}
	r, err := DeliverEarlyFailure(c, "CONTROLLED", "CONTROLLED")
	if err != nil {
		t.Fatal(err)
	}
	v := Candidate{InputID: "input/target", Target: "target", Status: "PROVISIONAL", Revision: 1, InventoryStateDelta: "ADD_PROVISIONAL_CANDIDATE", WorkingLabel: "label", Explanation: "explanation", RejectionCondition: "reject", Quote: "x", QuoteEnd: 1, Alternatives: []string{"alternative"}, Gaps: []string{"gap"}}
	v.ID = "candidate-" + pilot.Digest([]byte(v.InputID))[7:23]
	for _, family := range []string{"V3_RETAINED_PREPARED_OBSERVATION", "V3_RETAINED_ASSESSMENT", "V3_DERIVED_CANONICAL", "SOURCE_SPECIFIC_NODE_ID", "OWN_AST_BODY", "ORIGINAL_RAW_BATCH_SEEDS", "ORIGINAL_RAW_CAPTURE_ORIGIN", "ORIGINAL_RAW_SOURCE_ARTIFACT", "ORIGINAL_RAW_MANIFEST_INPUT"} {
		ref := pilot.Digest([]byte(family))
		v.EvidenceRefs = append(v.EvidenceRefs, ref)
		v.Evidence = append(v.Evidence, EvidenceReference{Family: family, Ref: ref, InputID: v.InputID})
	}
	r.Completed = 1
	r.Pending = r.Requested - 1
	r.Spend.Calls = 2
	r.Candidates = []Candidate{v}
	r.Unassigned = []string{v.ID}
	if !validReportValues(r, c) {
		t.Fatal("ASSERT_COMPLETE_VALID_REPORT_FIXTURE")
	}
	max := int(^uint(0) >> 1)
	for _, tt := range []struct {
		name       string
		start, end int
		quote      string
		want       bool
	}{
		{"sign_overflow", max, -max - 1, "x", false},
		{"negative_end", 0, -1, "x", false},
		{"negative_start", -1, 0, "x", false},
		{"reversed_positive", 2, 1, "x", false},
		{"ordered_max", max - 1, max, "x", true},
		{"utf8", 1, 3, "é", true},
		{"empty", 1, 1, "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r.Candidates[0].QuoteStart = tt.start
			r.Candidates[0].QuoteEnd = tt.end
			r.Candidates[0].Quote = tt.quote
			if got := validReportValues(r, c); got != tt.want {
				t.Fatalf("ASSERT_REPORT_CITATION_ORDERING: got=%v want=%v", got, tt.want)
			}
		})
	}
}

// None_Model: byte-bound arithmetic only; no semantic inference.
func TestHostProductCandidateCitationBounds(t *testing.T) {
	max := int(^uint(0) >> 1)
	for _, tt := range []struct {
		name       string
		start, end int
		quote      string
		valid      bool
	}{
		{"sign_overflow", max, -max - 1, "x", false},
		{"negative_end", 0, -1, "x", false},
		{"negative_start", -1, 0, "x", false},
		{"reversed_positive", 2, 1, "x", false},
		{"oversized_start", max, max, "", false},
		{"oversized_end", 0, max, "x", false},
		{"ordered_right_bound", 1, 3, "é", true},
		{"whole_utf8", 0, 3, "aé", true},
		{"empty_at_end", 3, 3, "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if p := recover(); p != nil {
					t.Fatalf("ASSERT_CITATION_NO_PANIC: %v", p)
				}
			}()
			body := ownBody{Target: "target", Body: "aé"}
			body.OwnBodySHA256 = pilot.Digest([]byte(body.Body))
			v := Candidate{InputID: "input/target", Target: "target", QuoteStart: tt.start, QuoteEnd: tt.end, Quote: tt.quote, EvidenceRefs: make([]string, 9)}
			v.EvidenceRefs[4] = body.OwnBodySHA256
			p := progress{Candidates: []Candidate{v}, Observations: []preparedObservation{{InputID: "input", Bodies: []ownBody{body}}}}
			work := 1
			var s *store
			if s.candidateObservations(p, Config{}, &work) {
				t.Fatal("ASSERT_NO_PHYSICAL_EXCHANGE")
			}
			// Valid byte slices reach the next (deliberately unavailable) exchange;
			// invalid ranges must refuse without consuming that exchange's work.
			want := 1
			if tt.valid {
				want = 0
			}
			if work != want {
				t.Fatalf("ASSERT_CITATION_BOUNDARY: work=%d want=%d", work, want)
			}
		})
	}
}
