package session

import "testing"

func TestPublicStatesClosed(t *testing.T) {
	want := []State{Starting, Initializing, Ready, Draining, Stopping, Stopped, Crashed, Poisoned}
	got := PublicStates()
	if len(got) != len(want) {
		t.Fatalf("state closure: got %d want 8", len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("state closure[%d]: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestTransitionVectors(t *testing.T) {
	expected := []TransitionVector{
		{"", FirstStart, Verified, Starting, true},
		{Stopped, RestartAccepted, PredecessorReaped, Starting, true}, {Crashed, RestartAccepted, PredecessorReaped, Starting, true}, {Poisoned, RestartAccepted, PredecessorReaped, Starting, true},
		{Starting, PipesEstablished, Any, Initializing, true}, {Starting, SpawnFailed, NoChild, Stopped, true}, {Starting, SetupFailed, PossibleChild, Stopping, true},
		{Starting, IntentAccepted, NoChild, Stopped, true}, {Initializing, IntentAccepted, NoChild, Stopped, true}, {Starting, IntentAccepted, PossibleChild, Stopping, true}, {Initializing, IntentAccepted, PossibleChild, Stopping, true},
		{Initializing, Initialized, Any, Ready, true}, {Initializing, InitFailed, Any, Poisoned, true},
		{Ready, GracefulIntent, HasWork, Draining, true}, {Ready, GracefulIntent, NoWork, Stopping, true}, {Draining, WorkDrained, Any, Stopping, true},
		{Starting, UnexpectedDeath, Any, Crashed, true}, {Initializing, UnexpectedDeath, Any, Crashed, true}, {Ready, UnexpectedDeath, Any, Crashed, true}, {Draining, UnexpectedDeath, Any, Crashed, true},
		{Starting, Poison, Any, Poisoned, true}, {Initializing, Poison, Any, Poisoned, true}, {Ready, Poison, Any, Poisoned, true}, {Draining, Poison, Any, Poisoned, true}, {Stopping, Poison, Any, Poisoned, true},
		{Stopping, Reaped, Any, Stopped, true}, {Crashed, Reaped, Any, Crashed, true}, {Poisoned, Reaped, Any, Poisoned, true},
	}
	allowed := map[[3]string]State{}
	for _, v := range expected {
		allowed[[3]string{string(v.From), string(v.Event), string(v.Guard)}] = v.To
	}
	states := append([]State{""}, PublicStates()...)
	events := []Event{FirstStart, RestartAccepted, PipesEstablished, SpawnFailed, SetupFailed, IntentAccepted, Initialized, InitFailed, GracefulIntent, WorkDrained, UnexpectedDeath, Poison, Reaped}
	guards := []Guard{Verified, PredecessorReaped, NoChild, PossibleChild, HasWork, NoWork, Any}
	for _, s := range states {
		for _, e := range events {
			for _, g := range guards {
				want, ok := allowed[[3]string{string(s), string(e), string(g)}]
				got, err := NextState(s, e, g)
				if ok && (err != nil || got != want) {
					t.Errorf("allowed transition %s/%s/%s: got %s,%v want %s", s, e, g, got, err, want)
				}
				if !ok && err == nil {
					t.Errorf("forbidden transition %s/%s/%s accepted as %s", s, e, g, got)
				}
			}
		}
	}
}

func TestGenerationOwnershipAndRestart(t *testing.T) {
	g := FirstGeneration()
	if g.Number != 1 {
		t.Fatalf("first generation: %d", g.Number)
	}
	r := RequestIdentity{SessionID: "s", ManagerRequestID: "r"}
	if err := g.Bind(&r); err != nil || r.Generation != 1 {
		t.Fatalf("bind: %+v %v", r, err)
	}
	if _, err := g.Successor(false, true); err == nil {
		t.Fatal("successor without death accepted")
	}
	if _, err := g.Successor(true, false); err == nil {
		t.Fatal("successor without reap accepted")
	}
	next, err := g.Successor(true, true)
	if err != nil || next.Number != 2 {
		t.Fatalf("successor: %+v %v", next, err)
	}
	if err := g.Bind(&RequestIdentity{}); err == nil {
		t.Fatal("terminal generation regained ownership")
	}
	if r.Generation != 1 {
		t.Fatal("request generation changed")
	}
}

func TestOrderedWinnerRejectsDuplicateManagerEventIdentity(t *testing.T) {
	got := Winner([]OrderedEvent{{Seq: 3, Kind: "stop"}, {Seq: 3, Kind: "initialize"}})
	if got != (OrderedEvent{}) {
		t.Fatalf("duplicate manager event identity accepted: %+v", got)
	}
}

func TestOrderedWinnerCancellationAndShutdown(t *testing.T) {
	if got := Winner([]OrderedEvent{{Seq: 9, Kind: "death"}, {Seq: 3, Kind: "poison"}}); got.Kind != "poison" {
		t.Fatalf("winner: %+v", got)
	}
	i := Intent{Active: true, Callers: map[string]Caller{"a": {}, "b": {}}}
	i.CancelCaller("a")
	if !i.Active || !i.Callers["a"].Terminal || i.Callers["b"].Terminal {
		t.Fatalf("cancellation mutated shared intent: %+v", i)
	}
	want := []ShutdownPhase{TerminalizeQueued, BoundActive, SendShutdown, SendExit, ClosePipes, ObserveDeath, ReapChild}
	got := ShutdownOrder()
	for n := range want {
		if got[n] != want[n] {
			t.Fatalf("shutdown[%d]: got %s want %s", n, got[n], want[n])
		}
	}
}
