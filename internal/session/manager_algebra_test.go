package session

import "testing"

func TestManagerLifecycleCallerBoundReturnsSelectedResourceExhausted(t *testing.T) {
	m, _ := NewManager(ManagerConfig{MaxSessions: 1})
	m.RegisterLifecycle("s", 1, Ready, false)
	first := m.Lifecycle(LifecycleRequest{SessionID: "s", Generation: 1, Operation: LifecycleStop, CallerID: "caller-000"})
	if first.IntentID == "" {
		t.Fatalf("ASSERT_CALLER_BOUND_SETUP: %+v", first)
	}
	for i := 1; i < defaultMaxIntentCallers; i++ {
		got := m.Lifecycle(LifecycleRequest{SessionID: "s", Generation: 1, Operation: LifecycleStop, CallerID: callerID(i)})
		if !got.Joined {
			t.Fatalf("ASSERT_CALLER_BOUND_BELOW_LIMIT[%d]: %+v", i, got)
		}
	}
	before := len(m.lifecycles["s"].incumbent.callers)
	got := m.Lifecycle(LifecycleRequest{SessionID: "s", Generation: 1, Operation: LifecycleStop, CallerID: "caller-overflow"})
	after := len(m.lifecycles["s"].incumbent.callers)
	if got.Joined || got.Failure != ResourceExhausted || got.Failure == LifecycleConflict || got.IntentID != first.IntentID || before != defaultMaxIntentCallers || after != before {
		t.Fatalf("ASSERT_CALLER_BOUND_RESOURCE_EXHAUSTED_NO_MUTATION: before=%d after=%d result=%+v", before, after, got)
	}
}

func TestLifecycleResultAlgebraTriplesAndReplayMatrix(t *testing.T) {
	m, _ := NewManager(ManagerConfig{MaxSessions: 1})
	m.RegisterLifecycle("s", 1, Stopped, true)
	status := m.Lifecycle(LifecycleRequest{SessionID: "s", Generation: 1, Operation: LifecycleStatus})
	if status.Outcome != OutcomeComplete || status.OperationStatus != StatusSucceeded || status.Failure != "" {
		t.Fatalf("ASSERT_RESULT_STATUS_SUCCESS_TRIPLE: %+v", status)
	}
	noop := m.Lifecycle(LifecycleRequest{SessionID: "s", Generation: 1, Operation: LifecycleStop, CallerID: "noop"})
	if noop.Outcome != OutcomeComplete || noop.OperationStatus != StatusNoop || !noop.Noop {
		t.Fatalf("ASSERT_RESULT_STOP_NOOP_TRIPLE: %+v", noop)
	}
	restart := m.Lifecycle(LifecycleRequest{SessionID: "s", Generation: 1, Operation: LifecycleRestart, CallerID: "restart"})
	failed := m.CompleteLifecycleObserved("s", restart.IntentID, LifecycleCompletion{ShutdownComplete: true, UnsafeIOAbsent: true, TerminateSucceeded: true, DeathObserved: true, NoContainedSurvivors: true, StderrDrainComplete: true, Reaped: true})
	if failed.Outcome != OutcomeDomainError || failed.OperationStatus != StatusFailed || failed.Failure != InitializationFailure {
		t.Fatalf("ASSERT_RESULT_RESTART_INIT_FAILURE_TRIPLE: %+v", failed)
	}
	retry := m.Lifecycle(LifecycleRequest{SessionID: "s", Generation: failed.Generation, Operation: LifecycleRestart, CallerID: "retry"})
	if retry.Replayed || retry.IntentID == restart.IntentID || retry.Failure != "" {
		t.Fatalf("ASSERT_REPLAY_NEGATIVE_RESTART_FAILURE_INSERTS: %+v", retry)
	}
	restarted := m.CompleteLifecycleObserved("s", retry.IntentID, LifecycleCompletion{ShutdownComplete: true, UnsafeIOAbsent: true, TerminateSucceeded: true, DeathObserved: true, NoContainedSurvivors: true, StderrDrainComplete: true, Reaped: true, InitializationComplete: true})
	laterStop := m.Lifecycle(LifecycleRequest{SessionID: "s", Generation: restarted.Generation, Operation: LifecycleStop, CallerID: "later-stop"})
	if laterStop.Replayed || laterStop.IntentID == restart.IntentID || laterStop.Failure != "" {
		t.Fatalf("ASSERT_REPLAY_NEGATIVE_DIFFERENT_KIND_INSERTS: %+v", laterStop)
	}
}

func TestLifecycleTeardownFirstApplicableObservationPrecedence(t *testing.T) {
	tests := []struct {
		name string
		in   LifecycleCompletion
		want Failure
	}{
		{"shutdown", LifecycleCompletion{}, SessionPoisoned},
		{"unsafe-io", LifecycleCompletion{ShutdownComplete: true}, SessionPoisoned},
		{"terminate", LifecycleCompletion{ShutdownComplete: true, UnsafeIOAbsent: true}, SessionReapIncomplete},
		{"death", LifecycleCompletion{ShutdownComplete: true, UnsafeIOAbsent: true, TerminateSucceeded: true}, SessionReapIncomplete},
		{"survivors", LifecycleCompletion{ShutdownComplete: true, UnsafeIOAbsent: true, TerminateSucceeded: true, DeathObserved: true}, SessionReapIncomplete},
		{"drain", LifecycleCompletion{ShutdownComplete: true, UnsafeIOAbsent: true, TerminateSucceeded: true, DeathObserved: true, NoContainedSurvivors: true}, SessionPoisoned},
		{"reap", LifecycleCompletion{ShutdownComplete: true, UnsafeIOAbsent: true, TerminateSucceeded: true, DeathObserved: true, NoContainedSurvivors: true, StderrDrainComplete: true}, SessionReapIncomplete},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := NewManager(ManagerConfig{MaxSessions: 1})
			m.RegisterLifecycle("s", 3, Ready, false)
			intent := m.Lifecycle(LifecycleRequest{SessionID: "s", Generation: 3, Operation: LifecycleRestart, CallerID: "c"})
			got := m.CompleteLifecycleObserved("s", intent.IntentID, tc.in)
			if got.Failure != tc.want || got.Outcome != OutcomeDomainError || got.OperationStatus != StatusFailed || got.Generation != 3 {
				t.Fatalf("ASSERT_TEARDOWN_FIRST_APPLICABLE_%s: %+v", tc.name, got)
			}
		})
	}
}

func TestLifecycleCallerDetachDeliveryAndRestartLedger(t *testing.T) {
	m, _ := NewManager(ManagerConfig{MaxSessions: 1})
	m.RegisterLifecycle("s", 8, Ready, false)
	intent := m.Lifecycle(LifecycleRequest{SessionID: "s", Generation: 8, Operation: LifecycleRestart, CallerID: "initiator"})
	joined := m.Lifecycle(LifecycleRequest{SessionID: "s", Generation: 8, Operation: LifecycleRestart, CallerID: "joiner"})
	if !joined.Joined {
		t.Fatalf("ASSERT_CALLER_ATTACH_JOIN: %+v", joined)
	}
	if !m.DetachLifecycleCaller("s", intent.IntentID, "initiator", RequestCancelled) {
		t.Fatal("ASSERT_CALLER_CANCEL_DETACH")
	}
	if got, ok := m.LifecycleCallerResult("s", intent.IntentID, "initiator"); !ok || got.Outcome != OutcomeCancelled || got.OperationStatus != StatusCancelled || got.Failure != RequestCancelled {
		t.Fatalf("ASSERT_CALLER_CANCEL_DELIVERY: ok=%v got=%+v", ok, got)
	}
	completed := m.CompleteLifecycleObserved("s", intent.IntentID, LifecycleCompletion{ShutdownComplete: true, UnsafeIOAbsent: true, TerminateSucceeded: true, DeathObserved: true, NoContainedSurvivors: true, StderrDrainComplete: true, Reaped: true, InitializationComplete: true})
	if completed.State != Ready || completed.Generation != 9 || completed.Outcome != OutcomeComplete || completed.OperationStatus != StatusSucceeded {
		t.Fatalf("ASSERT_RESTART_SUCCESS_RESULT: %+v", completed)
	}
	if got, ok := m.LifecycleCallerResult("s", intent.IntentID, "joiner"); !ok || got.Generation != 9 || got.Outcome != OutcomeComplete || got.OperationStatus != StatusSucceeded {
		t.Fatalf("ASSERT_CALLER_TERMINAL_DELIVERY: ok=%v got=%+v", ok, got)
	}
	ledger := m.LifecycleLedger()
	want := []string{"register", "insert", "join", "detach", "death", "drain", "reap", "allocate", "initialized", "ready", "bind", "terminal"}
	if len(ledger) != len(want) {
		t.Fatalf("ASSERT_RESTART_EVENT_LEDGER_LENGTH: got=%v", ledger)
	}
	for i, action := range want {
		if ledger[i].Action != action || ledger[i].Seq != uint64(i+1) {
			t.Fatalf("ASSERT_RESTART_EVENT_ORDER[%d]: %+v", i, ledger)
		}
	}
	if m.lifecycles["s"].generation != 9 || m.lifecycles["s"].state != Ready {
		t.Fatalf("ASSERT_RESTART_NO_OVERLAP_READY_BEFORE_BIND: %+v", m.lifecycles["s"])
	}
}

func TestLifecycleInitializationIntentOrderingAndPoisonEvents(t *testing.T) {
	m, _ := NewManager(ManagerConfig{MaxSessions: 1})
	m.RegisterLifecycle("intent-first", 1, Initializing, false)
	intent := m.Lifecycle(LifecycleRequest{SessionID: "intent-first", Generation: 1, Operation: LifecycleStop, CallerID: "stop", ChildRisk: true})
	if got := m.ObserveInitialization("intent-first", 1, true); got.State != Stopping || got.Failure != "" {
		t.Fatalf("ASSERT_INIT_AFTER_INTENT_DISCARDED: intent=%+v got=%+v", intent, got)
	}
	m.RegisterLifecycle("init-first", 1, Initializing, false)
	if got := m.ObserveInitialization("init-first", 1, true); got.State != Ready {
		t.Fatalf("ASSERT_INIT_BEFORE_INTENT_READY: %+v", got)
	}
	if got := m.Lifecycle(LifecycleRequest{SessionID: "init-first", Generation: 1, Operation: LifecycleStop, CallerID: "stop"}); got.PriorState != Ready || got.State != Stopping {
		t.Fatalf("ASSERT_INTENT_AFTER_INIT_READY_BRANCH: %+v", got)
	}
	m.RegisterLifecycle("poison", 1, Ready, false)
	if got := m.ObservePoison("poison", 1); got.State != Poisoned || got.Failure != SessionPoisoned {
		t.Fatalf("ASSERT_POISON_EVENT: %+v", got)
	}
}

func callerID(i int) string {
	const digits = "0123456789"
	return "caller-" + string([]byte{digits[(i/100)%10], digits[(i/10)%10], digits[i%10]})
}
