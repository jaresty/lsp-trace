package session

import (
	"reflect"
	"testing"
)

func TestManagerOwnsIntegratedLifecyclePath(t *testing.T) {
	const assertion = "all lifecycle decisions execute through one manager-owned session path"
	t.Log("ASSERTION: " + assertion)
	typeOfManager := reflect.TypeOf((*Manager)(nil))
	for _, method := range []string{"RegisterLifecycle", "Lifecycle", "CompleteLifecycle", "DetachLifecycleCaller", "LifecycleLedger"} {
		if _, ok := typeOfManager.MethodByName(method); !ok {
			t.Fatalf("ASSERT_MANAGER_LIFECYCLE_PATH_PRESENT: missing (*Manager).%s", method)
		}
	}
}

func TestManagerLifecycleExhaustiveOperationRows(t *testing.T) {
	type row struct {
		state                   State
		reaped, childRisk, work bool
		stopState, restartState State
		stopNoop                bool
		restartFailure          Failure
	}
	rows := []row{
		{Starting, false, false, false, Stopped, Stopped, false, ""}, {Starting, false, true, false, Stopping, Stopping, false, ""},
		{Initializing, false, false, false, Stopped, Stopped, false, ""}, {Initializing, false, true, false, Stopping, Stopping, false, ""},
		{Ready, false, false, true, Draining, Draining, false, ""}, {Ready, false, false, false, Stopping, Stopping, false, ""},
		{Draining, false, false, false, Poisoned, Poisoned, false, SessionPoisoned}, {Stopping, false, false, false, Poisoned, Poisoned, false, SessionPoisoned},
		{Stopped, true, false, false, Stopped, Stopped, true, ""}, {Crashed, false, false, false, Crashed, Crashed, false, SessionReapIncomplete},
		{Crashed, true, false, false, Crashed, Crashed, true, ""}, {Poisoned, false, false, false, Poisoned, Poisoned, false, SessionReapIncomplete},
		{Poisoned, true, false, false, Poisoned, Poisoned, true, ""},
	}
	for n, tc := range rows {
		for _, op := range []LifecycleOperation{LifecycleStop, LifecycleRestart} {
			m, _ := NewManager(ManagerConfig{MaxSessions: 1})
			m.RegisterLifecycle("s", 7, tc.state, tc.reaped)
			status := m.Lifecycle(LifecycleRequest{SessionID: "s", Generation: 7, Operation: LifecycleStatus})
			if status.State != tc.state || status.Failure != "" {
				t.Fatalf("ASSERT_STATUS_ROW[%d]: %+v", n, status)
			}
			got := m.Lifecycle(LifecycleRequest{SessionID: "s", Generation: 7, Operation: op, CallerID: "c", ChildRisk: tc.childRisk, HasWork: tc.work})
			if op == LifecycleStop {
				if got.State != tc.stopState || got.Noop != tc.stopNoop || ((tc.state == Draining || tc.state == Stopping) && got.Failure != SessionPoisoned) {
					t.Fatalf("ASSERT_STOP_ROW[%d/%s]: %+v", n, tc.state, got)
				}
			} else if got.State != tc.restartState || got.Failure != tc.restartFailure {
				t.Fatalf("ASSERT_RESTART_ROW[%d/%s]: %+v", n, tc.state, got)
			}
		}
	}
}

func TestManagerLifecycleSelectionJoinDetachHistoryAndSuccession(t *testing.T) {
	m, _ := NewManager(ManagerConfig{MaxSessions: 1})
	if got := m.Lifecycle(LifecycleRequest{SessionID: "missing", Operation: LifecycleStop}); got.Failure != SessionNotFound {
		t.Fatalf("ASSERT_NOT_FOUND_EXACT: %+v", got)
	}
	m.RegisterLifecycle("s", 4, Ready, false)
	if got := m.Lifecycle(LifecycleRequest{SessionID: "s", Generation: 3, Operation: LifecycleStop}); got.Failure != StaleGeneration {
		t.Fatalf("ASSERT_STALE_PRECEDES_INTENT: %+v", got)
	}
	first := m.Lifecycle(LifecycleRequest{SessionID: "s", Generation: 4, Operation: LifecycleRestart, CallerID: "a"})
	joined := m.Lifecycle(LifecycleRequest{SessionID: "s", Generation: 4, Operation: LifecycleRestart, CallerID: "b"})
	if !joined.Joined || joined.IntentID != first.IntentID {
		t.Fatalf("ASSERT_SAME_KIND_EXACT_INTENT_JOIN: first=%+v joined=%+v", first, joined)
	}
	if got := m.Lifecycle(LifecycleRequest{SessionID: "s", Generation: 4, Operation: LifecycleStop, CallerID: "x"}); got.Failure != LifecycleConflict {
		t.Fatalf("ASSERT_DIFFERENT_KIND_CONFLICT: %+v", got)
	}
	if !m.DetachLifecycleCaller("s", first.IntentID, "a", RequestTimeout) {
		t.Fatal("ASSERT_CALLER_TIMEOUT_DETACH")
	}
	completed := m.CompleteLifecycle("s", first.IntentID, true, true, true)
	if completed.State != Ready || completed.Generation != 5 {
		t.Fatalf("ASSERT_RESTART_SUCCESSION_READY: %+v", completed)
	}
	history := m.lifecycles["s"].history[0]
	if history.callers["a"].boundGeneration != 0 || history.callers["b"].boundGeneration != 5 {
		t.Fatalf("ASSERT_EXACT_INTENT_STILL_ATTACHED_BINDING: a=%+v b=%+v", history.callers["a"], history.callers["b"])
	}
	if got := m.Lifecycle(LifecycleRequest{SessionID: "s", Generation: 4, Operation: LifecycleStatus}); got.Failure != StaleGeneration {
		t.Fatalf("ASSERT_STALE_PREDECESSOR_EXCLUDED: %+v", got)
	}
	if got := m.Lifecycle(LifecycleRequest{SessionID: "s", Generation: 5, Operation: LifecycleStop, CallerID: "later"}); got.Failure != "" || got.IntentID == first.IntentID {
		t.Fatalf("ASSERT_LATER_DIFFERENT_KIND_INSERTION: %+v", got)
	}
	ledger := m.LifecycleLedger()
	want := []string{"register", "stale", "insert", "join", "conflict", "detach", "death", "drain", "reap", "allocate", "initialized", "ready", "bind", "terminal", "stale", "insert"}
	if len(ledger) != len(want) {
		t.Fatalf("ASSERT_LIFECYCLE_LEDGER_COMPLETE: %+v", ledger)
	}
	for i := range want {
		if ledger[i].Seq != uint64(i+1) || ledger[i].Action != want[i] {
			t.Fatalf("ASSERT_LIFECYCLE_LEDGER_ORDER[%d]: %+v", i, ledger[i])
		}
	}
}

func TestManagerLifecycleCompletionCommitsFailureAndInitializationOutcome(t *testing.T) {
	m, _ := NewManager(ManagerConfig{MaxSessions: 1})
	m.RegisterLifecycle("reap", 9, Ready, false)
	restart := m.Lifecycle(LifecycleRequest{SessionID: "reap", Generation: 9, Operation: LifecycleRestart, CallerID: "caller"})
	failed := m.CompleteLifecycle("reap", restart.IntentID, false, true, true)
	status := m.Lifecycle(LifecycleRequest{SessionID: "reap", Generation: 9, Operation: LifecycleStatus})
	if failed.Failure != SessionReapIncomplete || status.State != Poisoned || status.Generation != 9 {
		t.Fatalf("ASSERT_AUTHORITY_COMPLETION_FAILURE_COMMITTED: failed=%+v status=%+v", failed, status)
	}
	retried := m.Lifecycle(LifecycleRequest{SessionID: "reap", Generation: 9, Operation: LifecycleRestart, CallerID: "later"})
	if retried.Replayed || retried.IntentID == restart.IntentID || retried.Failure != SessionReapIncomplete {
		t.Fatalf("ASSERT_AUTHORITY_TERMINAL_FAILURE_NOT_BROADLY_REPLAYED: first=%+v retry=%+v", failed, retried)
	}

	m.RegisterLifecycle("init", 4, Ready, false)
	init := m.Lifecycle(LifecycleRequest{SessionID: "init", Generation: 4, Operation: LifecycleRestart, CallerID: "caller"})
	initFailed := m.CompleteLifecycle("init", init.IntentID, true, true, false)
	initStatus := m.Lifecycle(LifecycleRequest{SessionID: "init", Generation: initFailed.Generation, Operation: LifecycleStatus})
	if initFailed.Failure != InitializationFailure || initFailed.Generation != 5 || initStatus.State != Poisoned || initStatus.Generation != 5 {
		t.Fatalf("ASSERT_AUTHORITY_INITIALIZATION_FAILURE_COMMITTED_NO_GENERATION_CORRUPTION: failed=%+v status=%+v", initFailed, initStatus)
	}
}

func TestManagerLifecycleRejectsInvalidOperationAndCallerIdentity(t *testing.T) {
	m, _ := NewManager(ManagerConfig{MaxSessions: 1})
	m.RegisterLifecycle("s", 1, Ready, false)
	for name, request := range map[string]LifecycleRequest{
		"unknown-operation": {SessionID: "s", Generation: 1, Operation: LifecycleOperation("UNKNOWN"), CallerID: "caller"},
		"empty-caller":      {SessionID: "s", Generation: 1, Operation: LifecycleStop},
	} {
		t.Run(name, func(t *testing.T) {
			before := len(m.LifecycleLedger())
			got := m.Lifecycle(request)
			if got.Failure != LifecycleConflict || got.IntentID != "" || len(m.LifecycleLedger()) != before+1 {
				t.Fatalf("ASSERT_AUTHORITY_INVALID_LIFECYCLE_INPUT_REJECTED_WITH_ONE_EVENT: result=%+v ledger_before=%d ledger_after=%d", got, before, len(m.LifecycleLedger()))
			}
		})
	}
	first := m.Lifecycle(LifecycleRequest{SessionID: "s", Generation: 1, Operation: LifecycleStop, CallerID: "caller"})
	duplicate := m.Lifecycle(LifecycleRequest{SessionID: "s", Generation: 1, Operation: LifecycleStop, CallerID: "caller"})
	if first.IntentID == "" || duplicate.Failure != LifecycleConflict || duplicate.Joined {
		t.Fatalf("ASSERT_AUTHORITY_DUPLICATE_CALLER_REJECTED: first=%+v duplicate=%+v", first, duplicate)
	}
}

func TestManagerLifecycleRegistrationCannotOverwriteOrCreateImpossibleState(t *testing.T) {
	m, _ := NewManager(ManagerConfig{MaxSessions: 1})
	m.RegisterLifecycle("s", 3, Ready, false)
	m.RegisterLifecycle("s", 0, State("UNKNOWN"), false)
	got := m.Lifecycle(LifecycleRequest{SessionID: "s", Generation: 3, Operation: LifecycleStatus})
	if got.Failure != "" || got.Generation != 3 || got.State != Ready {
		t.Fatalf("ASSERT_AUTHORITY_REGISTRATION_OVERWRITE_REJECTED: %+v", got)
	}
	m.RegisterLifecycle("zero", 0, Ready, false)
	if got := m.Lifecycle(LifecycleRequest{SessionID: "zero", Operation: LifecycleStatus}); got.Failure != SessionNotFound {
		t.Fatalf("ASSERT_AUTHORITY_ZERO_GENERATION_REGISTRATION_REJECTED: %+v", got)
	}
}

func TestManagerLifecycleMissingIncumbentPoisonAndReapGuards(t *testing.T) {
	for _, state := range []State{Draining, Stopping} {
		m, _ := NewManager(ManagerConfig{MaxSessions: 1})
		m.RegisterLifecycle("s", 1, state, false)
		got := m.Lifecycle(LifecycleRequest{SessionID: "s", Generation: 1, Operation: LifecycleStop})
		if got.Failure != SessionPoisoned || got.State != Poisoned || got.IntentID != "" {
			t.Fatalf("ASSERT_MISSING_INCUMBENT_POISONS[%s]: %+v", state, got)
		}
	}
	for name, tc := range map[string]struct {
		death, reaped bool
	}{
		"death": {false, true},
		"reap":  {true, false},
	} {
		m, _ := NewManager(ManagerConfig{MaxSessions: 1})
		m.RegisterLifecycle("s", 9, Ready, false)
		r := m.Lifecycle(LifecycleRequest{SessionID: "s", Generation: 9, Operation: LifecycleRestart, CallerID: "a"})
		got := m.CompleteLifecycle("s", r.IntentID, tc.death, tc.reaped, true)
		if got.Failure != SessionReapIncomplete || got.Generation != 9 || got.State != Poisoned {
			t.Fatalf("ASSERT_%s_REQUIRED_BEFORE_SUCCESSOR: %+v", name, got)
		}
	}
}
