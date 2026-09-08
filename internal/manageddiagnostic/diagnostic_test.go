package manageddiagnostic

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func validRecord() Record {
	return Record{
		SessionID: "session-opaque", Generation: 7, Sequence: 1,
		Phase: PhaseInitializeResponse, Terminal: TerminalResponseReceived,
		Reason:  Fact[string]{Status: Observed, Value: "initialize-success"},
		Timing:  Timing{Scope: "initialize-response", Start: Fact[int64]{Status: Observed, Value: 10}, End: Fact[int64]{Status: Observed, Value: 20}, Elapsed: Fact[int64]{Status: Observed, Value: 10}},
		Request: RequestFacts{Method: Fact[string]{Status: Observed, Value: "initialize"}, ProtocolID: Fact[uint64]{Status: Observed, Value: 1}, CallerID: Fact[string]{Status: Unavailable}, TargetID: Fact[string]{Status: Unavailable}, OwnerSequence: Fact[uint64]{Status: Observed, Value: 1}},
		Limits:  Limits{RequestedDeadlineNS: Fact[int64]{Status: Unavailable}, EffectiveDeadlineNS: Fact[int64]{Status: Observed, Value: 100}, RequestedMaxBytes: Fact[int64]{Status: Observed, Value: 1024}, EffectiveMaxBytes: Fact[int64]{Status: Observed, Value: 1024}, RequestedMaxMessages: Fact[int]{Status: Observed, Value: 1}, EffectiveMaxMessages: Fact[int]{Status: Observed, Value: 1}},
		Read:    IOFacts{State: IOComplete, Messages: 1, Bytes: 50}, Write: IOFacts{State: IOComplete, Messages: 1, Bytes: 40},
		CallHierarchy: Fact[bool]{Status: Observed, Value: true}, DocumentSupplyCompleted: Fact[bool]{Status: Unavailable}, ProcessExit: ProcessExit{Status: Unavailable},
		Stderr: Stderr{Status: Withheld, ObservedByteCount: Fact[int64]{Status: Unavailable}},
	}
}

func TestOwnedSchemaSafeAndSemantic(t *testing.T) {
	r := validRecord()
	if err := Validate(r); err != nil {
		t.Fatalf("ASSERT_DIAGNOSTIC_VALID_PARTIAL: %v", err)
	}
	for name, mutate := range map[string]func(*Record){
		"elapsed":          func(r *Record) { r.Timing.Elapsed.Value = -1 },
		"response-timeout": func(r *Record) { r.Terminal = TerminalDeadlineExceeded },
		"exit-unavailable": func(r *Record) { r.Terminal = TerminalProcessExited },
		"unsupported-unknown": func(r *Record) {
			r.Phase = PhaseCapabilityCheck
			r.Terminal = TerminalProtocolError
			r.Reason = Fact[string]{Status: Observed, Value: "unsupported-call-hierarchy"}
			r.CallHierarchy = Fact[bool]{Status: Unavailable}
		},
	} {
		bad := r.Clone()
		mutate(&bad)
		if Validate(bad) == nil {
			t.Errorf("ASSERT_DIAGNOSTIC_IMPOSSIBLE_%s: accepted", name)
		}
	}
	r.SafeSubcodes = []string{"rpc-error"}
	clone := r.Clone()
	clone.SafeSubcodes[0] = "mutated"
	if r.SafeSubcodes[0] != "rpc-error" {
		t.Fatal("ASSERT_DIAGNOSTIC_CLONE_OWNERSHIP")
	}
	b, _ := json.Marshal(r)
	for _, secret := range []string{"rawErr", "rpc_message", "params", "source", "command", "args", "env"} {
		if strings.Contains(string(b), secret) {
			t.Fatalf("ASSERT_DIAGNOSTIC_SECRET_WITHHELD: %q in %s", secret, b)
		}
	}
}

func TestBoundedExactGenerationStore(t *testing.T) {
	s := NewStore(Bounds{MaxRecords: 2, MaxBytes: 16 * 1024})
	for i := uint64(1); i <= 3; i++ {
		r := validRecord()
		r.Sequence = i
		if !s.Record(r) {
			t.Fatal("ASSERT_DIAGNOSTIC_RECORD_ADMITTED")
		}
	}
	q := s.Query("session-opaque", 7)
	if q.Status != QueryAvailable || len(q.Records) != 2 || q.EvictedRecords != 1 {
		t.Fatalf("ASSERT_DIAGNOSTIC_EXPLICIT_EVICTION: %+v", q)
	}
	q.Records[0].SafeSubcodes = append(q.Records[0].SafeSubcodes, "mutate")
	if len(s.Query("session-opaque", 7).Records[0].SafeSubcodes) != 0 {
		t.Fatal("ASSERT_DIAGNOSTIC_QUERY_CLONE")
	}
	if got := s.Query("session-opaque", 8); got.Status != QueryUnavailable {
		t.Fatalf("ASSERT_DIAGNOSTIC_STALE_NO_REPLACEMENT: %+v", got)
	}
}

func TestTypedHooksAndFakeClock(t *testing.T) {
	now := int64(100)
	clock := func() int64 { now += 5; return now }
	rec := RecordRequest(RequestObservation{SessionID: "session-opaque", Generation: 7, Sequence: 2, Method: "textDocument/prepareCallHierarchy", ProtocolID: 9, TargetID: "target-opaque", CallerID: "caller-opaque", RequestedDeadline: time.Second, EffectiveDeadline: 900 * time.Millisecond, RequestedMaxBytes: 1000, EffectiveMaxBytes: 900, RequestedMaxMessages: 3, EffectiveMaxMessages: 2, Clock: clock}, func() RequestOutcome {
		return RequestOutcome{Terminal: TerminalResponseReceived, ReadState: IOComplete, WriteState: IOComplete, RequestBytes: 44, ResponseBytes: 55, ResponseMessages: 2}
	})
	if rec.Phase != PhaseRequestDispatch || rec.Read.Messages != 2 || rec.Request.ProtocolID.Value != 9 || rec.Request.TargetID.Value != "target-opaque" || rec.Timing.Elapsed.Value != 5 {
		t.Fatalf("ASSERT_DIAGNOSTIC_ACTUAL_REQUEST_JOIN: %+v", rec)
	}
	cap := RecordCapabilityCheck(CapabilityObservation{SessionID: "session-opaque", Generation: 7, Sequence: 3, Supported: false})
	if cap.Phase != PhaseCapabilityCheck || cap.CallHierarchy.Status != Observed || cap.CallHierarchy.Value {
		t.Fatalf("ASSERT_DIAGNOSTIC_CAPABILITY_HOOK: %+v", cap)
	}
	supply := RecordDocumentSupply(DocumentSupplyObservation{SessionID: "session-opaque", Generation: 7, Sequence: 4, Completed: true, Method: "textDocument/didOpen"})
	if supply.DocumentSupplyCompleted.Status != Observed || !supply.DocumentSupplyCompleted.Value {
		t.Fatalf("ASSERT_DIAGNOSTIC_SUPPLY_PREREQUISITE: %+v", supply)
	}
}
