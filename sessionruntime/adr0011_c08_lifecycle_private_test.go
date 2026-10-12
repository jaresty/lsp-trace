package sessionruntime

import (
	"context"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/session"
)

type c08ScriptClock struct {
	mu     sync.Mutex
	values []time.Time
	index  int
}

func (c *c08ScriptClock) set(values ...time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values = append(c.values[:0], values...)
	c.index = 0
}
func (c *c08ScriptClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.index >= len(c.values) {
		panic("ASSERT_C08_LIFECYCLE_CLOCK_EXHAUSTED")
	}
	v := c.values[c.index]
	c.index++
	return v
}
func (c *c08ScriptClock) calls() int { c.mu.Lock(); defer c.mu.Unlock(); return c.index }

type c08LifecycleState struct {
	present, protocolOwned, lifecycleOwned bool
	generation                             uint64
	attemptID                              string
	attemptStarted                         time.Time
	workers, requests, tombstones          int
	receiptOwners, borrowers               uint64
}

func c08LifecycleSnapshot(m *Manager, id string) c08LifecycleState {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.sessions[id]
	if r == nil {
		return c08LifecycleState{workers: m.workers}
	}
	s := c08LifecycleState{present: true, protocolOwned: r.protocolOwned, lifecycleOwned: r.lifecycleOwned, generation: r.record.Generation, attemptID: string(r.attemptID), attemptStarted: r.attemptStarted, workers: m.workers, requests: len(r.requests), tombstones: r.pending.TombstoneCount()}
	if r.readinessHistory != nil {
		s.receiptOwners, s.borrowers = r.readinessHistory.receiptOwners, r.readinessHistory.borrowers
	}
	return s
}

func c08ReadyManager(t *testing.T, clock *c08ScriptClock, starter Starter, epoch time.Time) (*Manager, StartResult) {
	t.Helper()
	clock.set(epoch)
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 1, MaxTombstones: 2, MaxObservations: 64, MaxOperations: 4}, Starter: starter, Now: clock.now})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
	started := m.Start(context.Background(), StartRequest{Profile: profile(t), Process: managedprocess.Spec{Path: "c08"}})
	if started.Failure != "" || m.ObserveInitialization(started.SessionID, started.Generation, true).State != session.Ready {
		t.Fatalf("ASSERT_C08_LIFECYCLE_START: %+v", started)
	}
	return m, started
}

func c08PrivateProbe(m *Manager, id string, generation uint64) RoundTripResult {
	wrong := OwnedDocumentBinding{URI: "file:///c08-missing.go", Version: 1, SHA256: "sha256:0000000000000000000000000000000000000000000000000000000000000000"}
	req := RoundTripRequest{SessionID: id, Generation: generation, Method: "textDocument/definition", Params: []byte(`{}`), ADR0011PrivateP2: true, ExpectedOwnedDocument: &wrong, MaxMessages: 1, MaxBytes: 1}
	return m.roundTripWithPrivateTransaction(context.Background(), req, nil, &privateB4Transaction{legacy: &privateB4Reservation{}})
}

func TestC08PrivateP2StopRetiresAttemptEpoch(t *testing.T) {
	base := time.Unix(1700002000, 0)
	clock := new(c08ScriptClock)
	m, started := c08ReadyManager(t, clock, oneChildStarter{referenceChild{}}, base)
	clock.set(base.Add(59*time.Second), base.Add(privateP2AttemptLimit-time.Nanosecond))
	if got := c08PrivateProbe(m, started.SessionID, started.Generation); got.Failure != DocumentSupplyUnavailable {
		t.Fatalf("ASSERT_C08_STOP_BEFORE_BOUNDARY: %s", got.Failure)
	}
	intent := m.Stop(context.Background(), started.SessionID, "c08-stop")
	terminal := waitOperation(t, m, intent.IntentID, OperationComplete)
	if terminal.Failure != "" || c08LifecycleSnapshot(m, started.SessionID).present {
		t.Fatalf("ASSERT_C08_STOP_RETIRES_EPOCH: %+v", terminal)
	}
	clock.set(base.Add(61 * time.Second))
	if got := c08PrivateProbe(m, started.SessionID, started.Generation); got.Failure != session.SessionNotFound || clock.calls() != 1 {
		t.Fatalf("ASSERT_C08_STOP_RETIRED_REQUEST got=%s calls=%d", got.Failure, clock.calls())
	}
}

func TestC08PrivateP2RestartCreatesDistinctEpoch(t *testing.T) {
	ea := time.Unix(1700003000, 0)
	eb := ea.Add(30 * time.Second)
	clock := new(c08ScriptClock)
	starter := &sequenceStarter{children: []Child{referenceChild{}, referenceChild{}}}
	m, started := c08ReadyManager(t, clock, starter, ea)
	old := c08LifecycleSnapshot(m, started.SessionID)
	clock.set(eb)
	intent := m.Restart(context.Background(), started.SessionID, "c08-restart")
	terminal := waitOperation(t, m, intent.IntentID, OperationComplete)
	if terminal.Failure != "" || m.ObserveInitialization(started.SessionID, 2, true).State != session.Ready {
		t.Fatalf("ASSERT_C08_RESTART_READY: %+v", terminal)
	}
	current := c08LifecycleSnapshot(m, started.SessionID)
	if current.generation != 2 || current.attemptID == old.attemptID || !current.attemptStarted.Equal(eb) || !current.attemptStarted.After(old.attemptStarted) {
		t.Fatalf("ASSERT_C08_RESTART_DISTINCT_EPOCH old=%+v current=%+v", old, current)
	}
	clock.set(eb.Add(time.Second))
	if got := c08PrivateProbe(m, started.SessionID, 1); got.Failure != session.StaleGeneration || clock.calls() != 1 {
		t.Fatalf("ASSERT_C08_RESTART_STALE_GENERATION got=%s calls=%d", got.Failure, clock.calls())
	}
	clock.set(eb.Add(59*time.Second), eb.Add(privateP2AttemptLimit-time.Nanosecond))
	if got := c08PrivateProbe(m, started.SessionID, 2); got.Failure != DocumentSupplyUnavailable {
		t.Fatalf("ASSERT_C08_RESTART_NEW_EPOCH_BEFORE: %s", got.Failure)
	}
	for _, offset := range []time.Duration{0, time.Nanosecond} {
		clock.set(eb.Add(59*time.Second), eb.Add(privateP2AttemptLimit+offset))
		if got := c08PrivateProbe(m, started.SessionID, 2); got.Failure != session.RequestTimeout {
			t.Fatalf("ASSERT_C08_RESTART_NEW_EPOCH_BOUNDARY offset=%s got=%s", offset, got.Failure)
		}
	}
}

func TestC08PrivateP2FailedRestartAdmitsNoEpoch(t *testing.T) {
	ea := time.Unix(1700004000, 0)
	attempted := ea.Add(30 * time.Second)
	clock := new(c08ScriptClock)
	starter := &sequenceStarter{children: []Child{referenceChild{}}}
	m, started := c08ReadyManager(t, clock, starter, ea)
	before := c08LifecycleSnapshot(m, started.SessionID)
	clock.set(attempted)
	intent := m.Restart(context.Background(), started.SessionID, "c08-failed-restart")
	terminal := waitOperation(t, m, intent.IntentID, OperationFailed)
	after := c08LifecycleSnapshot(m, started.SessionID)
	if terminal.Failure != session.SpawnFailure || !after.present || after.generation != before.generation || after.attemptID != before.attemptID || !after.attemptStarted.Equal(before.attemptStarted) {
		t.Fatalf("ASSERT_C08_FAILED_RESTART_NO_EPOCH terminal=%+v before=%+v after=%+v", terminal, before, after)
	}
	clock.set(attempted.Add(time.Second))
	if got := c08PrivateProbe(m, started.SessionID, started.Generation); got.Failure != session.LifecycleConflict || clock.calls() != 1 {
		t.Fatalf("ASSERT_C08_FAILED_RESTART_BLOCKS_WORK got=%s calls=%d", got.Failure, clock.calls())
	}
}
