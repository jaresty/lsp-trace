package manageddiagnostic

import (
	"strings"
	"testing"
)

func reviewAttempt(id StartupAttemptID, seq uint64) StartupAttemptRecord {
	return StartupAttemptRecord{SchemaVersion: StartupAttemptSchemaVersion, AttemptID: id, Sequence: seq, Outcome: StartupFailed, Reason: Fact[string]{Status: Observed, Value: "startup-failed"}, Stderr: Stderr{Status: Withheld}, ProcessExit: ProcessExit{Status: Unavailable}}
}

func TestReviewRetainedBytesCountActualStringsInBothStores(t *testing.T) {
	const huge = 10_000
	r := validRecord()
	r.SafeSubcodes = []string{strings.Repeat("X", huge)}
	if NewStore(Bounds{MaxRecords: 1, MaxBytes: 400}).Record(r) {
		t.Fatal("ASSERT_FR23_DIAGNOSTIC_ACTUAL_STRING_BYTES: huge subcode admitted")
	}
	a := reviewAttempt("attempt", 1)
	a.SafeSubcodes = []string{strings.Repeat("X", huge)}
	if NewStore(Bounds{MaxRecords: 1, MaxBytes: 400}).RecordStartupAttempt(a) {
		t.Fatal("ASSERT_FR23_ATTEMPT_ACTUAL_STRING_BYTES: huge subcode admitted")
	}
}

func TestReviewRetainedBytesExactBoundaryAndOneOver(t *testing.T) {
	r := validRecord()
	r.SafeSubcodes = []string{"actual-byte-string"}
	size := recordSize(r)
	if !NewStore(Bounds{MaxRecords: 1, MaxBytes: size}).Record(r) {
		t.Fatal("ASSERT_FR23_DIAGNOSTIC_EXACT_BYTE_BOUNDARY")
	}
	if NewStore(Bounds{MaxRecords: 1, MaxBytes: size - 1}).Record(r) {
		t.Fatal("ASSERT_FR23_DIAGNOSTIC_ONE_BYTE_OVER")
	}
	a := reviewAttempt("attempt-boundary", 1)
	a.SafeSubcodes = []string{"actual-byte-string"}
	size = startupAttemptSize(a)
	if !NewStore(Bounds{MaxRecords: 1, MaxBytes: size}).RecordStartupAttempt(a) {
		t.Fatal("ASSERT_FR23_ATTEMPT_EXACT_BYTE_BOUNDARY")
	}
	if NewStore(Bounds{MaxRecords: 1, MaxBytes: size - 1}).RecordStartupAttempt(a) {
		t.Fatal("ASSERT_FR23_ATTEMPT_ONE_BYTE_OVER")
	}
}

func TestReviewStoresRejectCallerConstructibleInvalidRecords(t *testing.T) {
	r := validRecord()
	r.SafeSubcodes = []string{strings.Repeat("X", 10_000)}
	r.SessionID = ""
	if NewStore(Bounds{MaxRecords: 1, MaxBytes: 1 << 20}).Record(r) {
		t.Fatal("ASSERT_FR23_DIAGNOSTIC_VALIDATE_BEFORE_STORE")
	}
	a := reviewAttempt("attempt-invalid", 1)
	a.SafeSubcodes = []string{strings.Repeat("X", 10_000)}
	a.SchemaVersion = "invalid"
	if NewStore(Bounds{MaxRecords: 1, MaxBytes: 1 << 20}).RecordStartupAttempt(a) {
		t.Fatal("ASSERT_FR23_ATTEMPT_VALIDATE_BEFORE_STORE")
	}
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

func TestReviewValidatorClosedPhaseTerminalIOMatrix(t *testing.T) {
	phases := []Phase{PhaseSpawn, PhaseInitializeWrite, PhaseInitializeResponse, PhaseRequestDispatch, PhaseReadinessComplete, PhaseDocumentSupply, PhaseCapabilityCheck}
	terminals := []Terminal{TerminalResponseReceived, TerminalProtocolError, TerminalTransportClosed, TerminalCancelled, TerminalDeadlineExceeded, TerminalProcessExited, TerminalUnknown}
	allowed := map[Phase]map[Terminal]bool{
		PhaseSpawn:              {TerminalProtocolError: true, TerminalTransportClosed: true, TerminalCancelled: true, TerminalDeadlineExceeded: true, TerminalProcessExited: true, TerminalUnknown: true},
		PhaseInitializeWrite:    {TerminalProtocolError: true, TerminalTransportClosed: true, TerminalCancelled: true, TerminalDeadlineExceeded: true, TerminalProcessExited: true, TerminalUnknown: true},
		PhaseInitializeResponse: {TerminalResponseReceived: true, TerminalProtocolError: true, TerminalTransportClosed: true, TerminalCancelled: true, TerminalDeadlineExceeded: true, TerminalProcessExited: true, TerminalUnknown: true},
		PhaseRequestDispatch:    {TerminalResponseReceived: true, TerminalProtocolError: true, TerminalTransportClosed: true, TerminalCancelled: true, TerminalDeadlineExceeded: true, TerminalProcessExited: true, TerminalUnknown: true},
		PhaseReadinessComplete:  {TerminalResponseReceived: true},
		PhaseDocumentSupply:     {TerminalResponseReceived: true, TerminalProtocolError: true},
		PhaseCapabilityCheck:    {TerminalResponseReceived: true, TerminalProtocolError: true},
	}
	for _, phase := range phases {
		for _, terminal := range terminals {
			r := matrixRecord(phase, terminal)
			got := Validate(r) == nil
			if got != allowed[phase][terminal] {
				t.Errorf("ASSERT_FR23_MATRIX_%s_%s: valid=%v want=%v err=%v", phase, terminal, got, allowed[phase][terminal], Validate(r))
			}
		}
	}
}

func matrixRecord(phase Phase, terminal Terminal) Record {
	r := validRecord()
	r.Phase, r.Terminal = phase, terminal
	r.Reason = Fact[string]{Status: Observed, Value: "matrix"}
	r.Substep = Fact[Substep]{Status: Unavailable}
	r.ProcessExit = ProcessExit{Status: Unavailable}
	if phase == PhaseReadinessComplete {
		r.Substep = Fact[Substep]{Status: Observed, Value: SubstepInitializedNotification}
		r.Write.Messages = 2
	}
	if phase == PhaseDocumentSupply {
		r.Read = IOFacts{State: IOUnavailable}
		r.DocumentSupplyCompleted = Fact[bool]{Status: Observed, Value: terminal == TerminalResponseReceived}
	}
	if phase == PhaseCapabilityCheck {
		r.Read, r.Write = IOFacts{State: IOUnavailable}, IOFacts{State: IOUnavailable}
		r.CallHierarchy = Fact[bool]{Status: Observed, Value: true}
	}
	if terminal == TerminalProcessExited {
		r.ProcessExit = ProcessExit{Status: Observed, ObservedBeforeCleanup: Fact[bool]{Status: Observed, Value: true}, CleanupInduced: Fact[bool]{Status: Observed, Value: false}}
	}
	return r
}

func TestReviewRequestMatchedResponseRequiresCompletedIO(t *testing.T) {
	for _, terminal := range []Terminal{TerminalResponseReceived, TerminalProtocolError} {
		for _, mutate := range []func(*Record){
			func(r *Record) { r.Write = IOFacts{State: IOUnavailable} },
			func(r *Record) { r.Write.Messages = 0 },
			func(r *Record) { r.Read = IOFacts{State: IOUnavailable} },
			func(r *Record) { r.Read.Messages = 0 },
		} {
			r := matrixRecord(PhaseRequestDispatch, terminal)
			mutate(&r)
			if Validate(r) == nil {
				t.Errorf("ASSERT_FR23_REQUEST_MATCHED_IO_%s: accepted %+v", terminal, r)
			}
		}
	}
}

func TestReviewValidatorAllowsExplicitEarlyPartial(t *testing.T) {
	for _, tc := range []struct{ phase Phase; terminal Terminal }{
		{PhaseSpawn, TerminalUnknown}, {PhaseInitializeWrite, TerminalCancelled},
		{PhaseInitializeResponse, TerminalDeadlineExceeded}, {PhaseRequestDispatch, TerminalTransportClosed},
	} {
		r := matrixRecord(tc.phase, tc.terminal)
		r.Read, r.Write = IOFacts{State: IOUnavailable}, IOFacts{State: IOUnavailable}
		r.Timing, r.Limits = Timing{}, Limits{}
		if err := Validate(r); err != nil {
			t.Errorf("ASSERT_FR23_VALID_PARTIAL_%s_%s: %v", tc.phase, tc.terminal, err)
		}
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
