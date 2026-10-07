package adr0007locationv2

import "testing"

func TestCustodyV2StrictReplayConflictAndReview(t *testing.T) {
	k := AttemptKey{"01", "producer", 1}
	l, e := Commit(Ledger{}, k, []byte("{\"a\":1}\n"))
	if e != nil {
		t.Fatal(e)
	}
	l, e = Commit(l, k, []byte("{\"a\":2}\n"))
	if e != nil || len(l.Attempts) != 1 || len(l.Conflicts) != 1 || l.Account.ProducerAttempts != 1 {
		t.Fatal(l, e)
	}
	l, e = AddReview(l, Review{Key: AttemptKey{"01", "reviewer", 1}, SchemaValid: true, OutcomeMatches: true, AccountingComplete: true, WitnessesConcrete: true, BindingExact: false, Verdict: true})
	if e != nil || l.Reviews[0].Verdict {
		t.Fatal("review recomputation")
	}
	x := cloneLedger(l)
	x.Raw[0].Raw[0] = 'x'
	if l.Raw[0].Raw[0] == 'x' {
		t.Fatal("alias")
	}
}
func TestCustodyV2RejectsDuplicateUnknownTrailingAndBadBase64(t *testing.T) {
	for _, raw := range [][]byte{[]byte("{\"a\":1,\"a\":2}\n"), []byte("{}\n{}\n")} {
		var v map[string]any
		if StrictDecode(raw, &v) == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	var r RawRecord
	if StrictDecode([]byte("{\"byteLength\":1,\"digest\":\"x\",\"key\":{\"caseId\":\"c\",\"ordinal\":1,\"role\":\"producer\"},\"raw\":\"***\",\"schema\":\"x\"}\n"), &r) == nil {
		t.Fatal("base64")
	}
}
