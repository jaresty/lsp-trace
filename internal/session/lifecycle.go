package session

import "fmt"

type State string

const (
	Starting     State = "STARTING"
	Initializing State = "INITIALIZING"
	Ready        State = "READY"
	Draining     State = "DRAINING"
	Stopping     State = "STOPPING"
	Stopped      State = "STOPPED"
	Crashed      State = "CRASHED"
	Poisoned     State = "POISONED"
)

func PublicStates() []State {
	return []State{Starting, Initializing, Ready, Draining, Stopping, Stopped, Crashed, Poisoned}
}

type Event string
type Guard string

const (
	FirstStart        Event = "FIRST_START"
	RestartAccepted   Event = "RESTART_ACCEPTED"
	PipesEstablished  Event = "PIPES_ESTABLISHED"
	SpawnFailed       Event = "SPAWN_FAILED"
	SetupFailed       Event = "SETUP_FAILED"
	IntentAccepted    Event = "INTENT_ACCEPTED"
	Initialized       Event = "INITIALIZED"
	InitFailed        Event = "INIT_FAILED"
	GracefulIntent    Event = "GRACEFUL_INTENT"
	WorkDrained       Event = "WORK_DRAINED"
	UnexpectedDeath   Event = "UNEXPECTED_DEATH"
	Poison            Event = "POISON"
	Reaped            Event = "REAPED"
	Verified          Guard = "VERIFIED"
	PredecessorReaped Guard = "PREDECESSOR_REAPED"
	NoChild           Guard = "NO_CHILD"
	PossibleChild     Guard = "POSSIBLE_CHILD"
	HasWork           Guard = "HAS_WORK"
	NoWork            Guard = "NO_WORK"
	Any               Guard = "ANY"
)

type TransitionVector struct {
	From    State
	Event   Event
	Guard   Guard
	To      State
	Allowed bool
}

var allowedTransitions = []TransitionVector{
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

func TransitionVectors() []TransitionVector {
	states := append([]State{""}, PublicStates()...)
	events := []Event{FirstStart, RestartAccepted, PipesEstablished, SpawnFailed, SetupFailed, IntentAccepted, Initialized, InitFailed, GracefulIntent, WorkDrained, UnexpectedDeath, Poison, Reaped}
	guards := []Guard{Verified, PredecessorReaped, NoChild, PossibleChild, HasWork, NoWork, Any}
	out := make([]TransitionVector, 0, len(states)*len(events)*len(guards))
	for _, s := range states {
		for _, e := range events {
			for _, g := range guards {
				v := TransitionVector{From: s, Event: e, Guard: g}
				if to, ok := lookupTransition(s, e, g); ok {
					v.To = to
					v.Allowed = true
				}
				out = append(out, v)
			}
		}
	}
	return out
}
func lookupTransition(s State, e Event, g Guard) (State, bool) {
	for _, v := range allowedTransitions {
		if v.From == s && v.Event == e && v.Guard == g {
			return v.To, true
		}
	}
	return "", false
}
func NextState(s State, e Event, g Guard) (State, error) {
	if to, ok := lookupTransition(s, e, g); ok {
		return to, nil
	}
	return "", fmt.Errorf("forbidden transition %s/%s/%s", s, e, g)
}

type Generation struct {
	Number   uint64
	terminal bool
}

func FirstGeneration() Generation { return Generation{Number: 1} }

type RequestIdentity struct {
	SessionID        string
	Generation       uint64
	ManagerRequestID string
}

func (g *Generation) Bind(r *RequestIdentity) error {
	if g.terminal {
		return fmt.Errorf("terminal generation")
	}
	if r.Generation != 0 && r.Generation != g.Number {
		return fmt.Errorf("already bound")
	}
	r.Generation = g.Number
	return nil
}
func (g *Generation) Successor(death, reaped bool) (Generation, error) {
	if !death || !reaped {
		return Generation{}, fmt.Errorf("predecessor not dead and reaped")
	}
	g.terminal = true
	return Generation{Number: g.Number + 1}, nil
}

type OrderedEvent struct {
	Seq  uint64
	Kind string
}

// Winner returns the earliest event only when every manager event identity is
// nonzero and unique. Duplicate identities are an invariant violation, not a
// lexical race tie.
func Winner(es []OrderedEvent) OrderedEvent {
	if len(es) == 0 {
		return OrderedEvent{}
	}
	seen := make(map[uint64]struct{}, len(es))
	w := es[0]
	for _, e := range es {
		if e.Seq == 0 {
			return OrderedEvent{}
		}
		if _, duplicate := seen[e.Seq]; duplicate {
			return OrderedEvent{}
		}
		seen[e.Seq] = struct{}{}
		if e.Seq < w.Seq {
			w = e
		}
	}
	return w
}

type Caller struct{ Terminal bool }
type Intent struct {
	Active  bool
	Callers map[string]Caller
}

func (i *Intent) CancelCaller(id string) { c := i.Callers[id]; c.Terminal = true; i.Callers[id] = c }

type ShutdownPhase string

const (
	TerminalizeQueued ShutdownPhase = "TERMINALIZE_QUEUED"
	BoundActive       ShutdownPhase = "BOUND_ACTIVE"
	SendShutdown      ShutdownPhase = "SEND_SHUTDOWN"
	SendExit          ShutdownPhase = "SEND_EXIT"
	ClosePipes        ShutdownPhase = "CLOSE_PIPES"
	ObserveDeath      ShutdownPhase = "OBSERVE_DEATH"
	ReapChild         ShutdownPhase = "REAP_CHILD"
)

func ShutdownOrder() []ShutdownPhase {
	return []ShutdownPhase{TerminalizeQueued, BoundActive, SendShutdown, SendExit, ClosePipes, ObserveDeath, ReapChild}
}
