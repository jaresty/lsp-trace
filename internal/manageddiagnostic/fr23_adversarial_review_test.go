package manageddiagnostic

import "testing"

func reviewAttempt(id StartupAttemptID, seq uint64) StartupAttemptRecord {
	return StartupAttemptRecord{SchemaVersion: StartupAttemptSchemaVersion, AttemptID: id, Sequence: seq, Outcome: StartupFailed, Reason: Fact[string]{Status: Observed, Value: "startup-failed"}, Stderr: Stderr{Status: Withheld}, ProcessExit: ProcessExit{Status: Unavailable}}
}

func TestReviewAttemptBoundsZeroAndOne(t *testing.T) {
	zero := NewStore(Bounds{MaxRecords: 0, MaxBytes: 4096})
	if zero.RecordStartupAttempt(reviewAttempt("a", 1)) {
		t.Fatal("zero bound admitted")
	}
	one := NewStore(Bounds{MaxRecords: 1, MaxBytes: 4096})
	if !one.RecordStartupAttempt(reviewAttempt("a", 1)) || !one.RecordStartupAttempt(reviewAttempt("b", 2)) {
		t.Fatal("one bound rejected")
	}
	if one.StartupAttempt("a").Status != AttemptEvicted || one.StartupAttempt("b").Status != AttemptAvailable {
		t.Fatalf("boundary status: a=%+v b=%+v", one.StartupAttempt("a"), one.StartupAttempt("b"))
	}
}

func TestReviewValidatorRejectsImpossibleObservedValues(t *testing.T) {
	cases := map[string]func(*StartupAttemptRecord){
		"failed-with-observed-exit": func(r *StartupAttemptRecord) {
			r.ProcessExit = ProcessExit{Status: Observed, ObservedBeforeCleanup: Fact[bool]{Status: Observed, Value: false}, CleanupInduced: Fact[bool]{Status: Observed, Value: true}}
		},
		"admitted-with-failed-reason": func(r *StartupAttemptRecord) {
			r.Outcome = StartupAdmitted
			r.Admission = &StartupAdmission{SessionID: "s", Generation: 1}
			r.Reason = Fact[string]{Status: Observed, Value: "process-start-failed"}
		},
		"withheld-reason-has-value": func(r *StartupAttemptRecord) { r.Reason = Fact[string]{Status: Withheld, Value: "SECRET"} },
	}
	for name, mutate := range cases {
		r := reviewAttempt("attempt-"+StartupAttemptID(name), 1)
		mutate(&r)
		if ValidateStartupAttempt(r) == nil {
			t.Errorf("validator accepted %s", name)
		}
	}
}
