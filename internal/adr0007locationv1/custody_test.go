package adr0007locationv1

import "testing"

func TestStrictCustody(t *testing.T) {
	raw := []byte("{\"a\":1}\n")
	l, e := CommitAttempt(Ledger{}, AttemptKey{"c", "producer", 1}, raw)
	if e != nil || len(l.Attempts) != 1 {
		t.Fatal(e)
	}
	raw[2] = 'z'
	if string(l.Attempts[0].Raw) != "{\"a\":1}\n" {
		t.Fatal("alias")
	}
	l, e = CommitAttempt(l, AttemptKey{"c", "producer", 1}, []byte("{\"a\":2}\n"))
	if e != nil || len(l.Conflicts) != 1 {
		t.Fatal("conflict")
	}
}
func TestStrictRejectsTrailing(t *testing.T) {
	var v map[string]any
	if StrictDecode([]byte("{}\n{}\n"), &v) == nil {
		t.Fatal("trailing")
	}
}
func TestReviewRecomputed(t *testing.T) {
	r := RecomputeReview(Review{SchemaValid: true, OutcomeMatches: true, AccountingComplete: true, WitnessesConcrete: true, BindingExact: false, Verdict: true})
	if r.Verdict {
		t.Fatal("trusted producer verdict")
	}
}
