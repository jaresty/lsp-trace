// Package sessionruntime provides transport-neutral orchestration over the
// session algebra, resolved runtime identity, managed processes, and LSP wire.
// It neither registers nor advertises a command surface.
package sessionruntime

import (
	"context"
	"errors"
	"sync"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
)

type Limits struct {
	MaxSessions, MaxRequests, MaxChildren, MaxCancels, MaxTombstones, MaxObservations int
	// MaxOperations bounds retained lifecycle operation records. Zero defaults to MaxObservations.
	MaxOperations int
}

type OperationState string

const (
	OperationPending  OperationState = "PENDING"
	OperationComplete OperationState = "COMPLETE"
	OperationFailed   OperationState = "FAILED"
)

type OperationSnapshot struct {
	ID         string
	SessionID  string
	CallerID   string
	Generation uint64
	Restart    bool
	State      OperationState
	Failure    session.Failure
}

type Child interface {
	Teardown(context.Context) managedprocess.TeardownObservation
	Close() managedprocess.ResourceObservation
}

type Starter interface {
	Start(context.Context, managedprocess.Spec) (Child, managedprocess.StartObservation)
}

// ManagedStarter adapts the existing managed-process foundation to the narrow
// runtime seam. Production callers construct Manager with the sealed gate.
type ManagedStarter struct{ Manager *managedprocess.Manager }

func (s ManagedStarter) Start(ctx context.Context, spec managedprocess.Spec) (Child, managedprocess.StartObservation) {
	if s.Manager == nil {
		return nil, managedprocess.StartObservation{Kind: managedprocess.StartUnavailable, Reason: "containment unavailable"}
	}
	return s.Manager.Start(ctx, spec)
}

type Config struct {
	Limits  Limits
	Wire    lspwire.Limits
	Starter Starter
}
type StartRequest struct {
	Profile  runtimeprofile.Profile
	Process  managedprocess.Spec
	Deadline time.Time
}
type StartResult struct {
	SessionID  string
	Generation uint64
	State      session.State
	Failure    session.Failure
	Start      managedprocess.StartObservation
}
type Census struct{ Sessions, Generations, Requests, Children, Cancels, Tombstones, Observations, Operations, Workers int }
type Observation struct {
	Sequence, Generation uint64
	SessionID, Kind      string
	State                session.State
	Failure              session.Failure
}
type Record struct {
	SessionID  string
	Profile    runtimeprofile.Profile
	Generation uint64
	State      session.State
	Started    time.Time
}
type Request struct {
	Key      lspwire.RequestKey
	Deadline time.Time
	Terminal session.Failure
}

type runtimeSession struct {
	record   Record
	process  Child
	spec     managedprocess.Spec
	pending  *lspwire.Pending
	requests map[lspwire.RequestKey]*Request
	cancels  int
}

type Manager struct {
	mu           sync.Mutex
	limits       Limits
	wire         lspwire.Limits
	starter      Starter
	algebra      *session.Manager
	sessions     map[string]*runtimeSession
	operations   map[string]OperationSnapshot
	operationIDs []string
	observations []Observation
	sequence     uint64
	workers      int
	workerDone   chan struct{}
	closed       bool
}

func New(c Config) (*Manager, error) {
	l := c.Limits
	if l.MaxSessions <= 0 || l.MaxRequests <= 0 || l.MaxChildren <= 0 || l.MaxCancels <= 0 || l.MaxTombstones <= 0 || l.MaxObservations <= 0 {
		return nil, errors.New("sessionruntime: all limits must be positive")
	}
	if c.Starter == nil {
		return nil, errors.New("sessionruntime: starter is required")
	}
	a, err := session.NewManager(session.ManagerConfig{MaxSessions: l.MaxSessions})
	if err != nil {
		return nil, err
	}
	if l.MaxOperations <= 0 {
		l.MaxOperations = l.MaxObservations
	}
	return &Manager{limits: l, wire: c.Wire, starter: c.Starter, algebra: a, sessions: make(map[string]*runtimeSession), operations: make(map[string]OperationSnapshot), workerDone: make(chan struct{}, 1)}, nil
}

func (m *Manager) Start(ctx context.Context, req StartRequest) StartResult {
	if !req.Deadline.IsZero() && !time.Now().Before(req.Deadline) {
		return StartResult{Failure: session.RequestTimeout}
	}
	if err := ctx.Err(); err != nil {
		return StartResult{Failure: session.RequestCancelled}
	}
	id := req.Profile.SessionKey().String()
	m.mu.Lock()
	if current := m.sessions[id]; current != nil {
		result := StartResult{SessionID: id, Generation: current.record.Generation, State: current.record.State}
		m.mu.Unlock()
		return result
	}
	if len(m.sessions) >= m.limits.MaxSessions {
		m.mu.Unlock()
		return StartResult{SessionID: id, Failure: session.ResourceExhausted}
	}
	m.mu.Unlock()

	// Start is deliberately before admission. The production managedprocess gate
	// returns UNAVAILABLE before command construction, pipes, or process creation.
	child, observed := m.starter.Start(ctx, req.Process)
	if observed.Kind == managedprocess.StartUnavailable {
		return StartResult{SessionID: id, Failure: session.ProcessContainmentUnavailable, Start: observed}
	}
	if observed.Kind != managedprocess.StartStarted || child == nil {
		return StartResult{SessionID: id, Failure: session.SpawnFailure, Start: observed}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sessions[id] != nil || len(m.sessions) >= m.limits.MaxSessions {
		_ = child.Teardown(context.Background())
		_ = child.Close()
		return StartResult{SessionID: id, Failure: session.LifecycleConflict, Start: observed}
	}
	m.algebra.RegisterLifecycle(id, 1, session.Initializing, false)
	if admission := m.algebra.Admit(id, 0); admission.Kind != session.AdmissionFree {
		_ = child.Teardown(context.Background())
		_ = child.Close()
		return StartResult{SessionID: id, Failure: session.ResourceExhausted, Start: observed}
	}
	r := Record{SessionID: id, Profile: req.Profile, Generation: 1, State: session.Initializing, Started: time.Now()}
	m.sessions[id] = &runtimeSession{record: r, process: child, spec: req.Process, pending: lspwire.NewPending(m.limits.MaxTombstones), requests: make(map[lspwire.RequestKey]*Request)}
	m.observe(id, 1, "startup", session.Initializing, "")
	return StartResult{SessionID: id, Generation: 1, State: session.Initializing, Start: observed}
}

func (m *Manager) ObserveInitialization(id string, generation uint64, complete bool) session.LifecycleResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.sessions[id]
	if r == nil {
		return session.LifecycleResult{Failure: session.SessionNotFound}
	}
	result := m.algebra.ObserveInitialization(id, generation, complete)
	r.record.State = result.State
	kind := "initialization"
	if !complete {
		kind = "poison"
	}
	m.observe(id, generation, kind, result.State, result.Failure)
	return result
}

func (m *Manager) BeginRequest(id string, generation uint64, deadline time.Time) (lspwire.RequestKey, session.Failure) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.sessions[id]
	if r == nil {
		return lspwire.RequestKey{}, session.SessionNotFound
	}
	if generation != r.record.Generation {
		return lspwire.RequestKey{}, session.StaleGeneration
	}
	if len(r.requests) >= m.limits.MaxRequests {
		return lspwire.RequestKey{}, session.ResourceExhausted
	}
	key := r.pending.Begin(generation)
	r.requests[key] = &Request{Key: key, Deadline: deadline}
	m.observe(id, generation, "request", r.record.State, "")
	return key, ""
}

func (m *Manager) CompleteResponse(id string, key lspwire.ResponseKey) lspwire.ResponseDisposition {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.sessions[id]
	if r == nil {
		return lspwire.ResponseUnknown
	}
	d := r.pending.Accept(key)
	if d == lspwire.ResponseAccepted {
		delete(r.requests, key)
		m.observe(id, key.Generation, "response", r.record.State, "")
	}
	return d
}

func (m *Manager) Expire(now time.Time) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for id, r := range m.sessions {
		for key, request := range r.requests {
			if request.Terminal == "" && !request.Deadline.IsZero() && !now.Before(request.Deadline) {
				request.Terminal = session.RequestTimeout
				delete(r.requests, key)
				n++
				m.observe(id, key.Generation, "deadline", r.record.State, session.RequestTimeout)
			}
		}
	}
	return n
}

func (m *Manager) ObserveCrash(id string, generation uint64) session.Failure {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.sessions[id]
	if r == nil {
		return session.SessionNotFound
	}
	if generation != r.record.Generation {
		return session.StaleGeneration
	}
	r.record.State = session.Crashed
	m.observe(id, generation, "crash", session.Crashed, session.SessionCrashed)
	return ""
}

func (m *Manager) Poison(id string, generation uint64) session.LifecycleResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.sessions[id]
	if r == nil {
		return session.LifecycleResult{Failure: session.SessionNotFound}
	}
	result := m.algebra.ObservePoison(id, generation)
	r.record.State = result.State
	m.observe(id, generation, "poison", result.State, result.Failure)
	return result
}

func (m *Manager) Stop(ctx context.Context, id, caller string) session.LifecycleResult {
	return m.terminate(ctx, id, caller, false)
}

func (m *Manager) Restart(ctx context.Context, id, caller string) session.LifecycleResult {
	return m.terminate(ctx, id, caller, true)
}

func (m *Manager) terminate(_ context.Context, id, caller string, restart bool) session.LifecycleResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.sessions[id]
	if r == nil {
		return session.LifecycleResult{Failure: session.SessionNotFound}
	}
	if m.closed {
		return session.LifecycleResult{Failure: session.LifecycleConflict}
	}
	op := session.LifecycleStop
	kind := "teardown"
	if restart {
		op, kind = session.LifecycleRestart, "restart"
	}
	for i := len(m.operationIDs) - 1; i >= 0; i-- {
		existing := m.operations[m.operationIDs[i]]
		if existing.SessionID != id || existing.Generation != r.record.Generation || existing.Restart != restart {
			continue
		}
		if existing.CallerID == caller {
			return session.LifecycleResult{State: r.record.State, Generation: existing.Generation, IntentID: existing.ID, Replayed: true}
		}
		if existing.State == OperationPending {
			// Joining an accepted operation consumes no new runtime capacity. Let the
			// algebra bound and record the distinct caller against that exact intent.
			return m.algebra.Lifecycle(session.LifecycleRequest{SessionID: id, Generation: r.record.Generation, Operation: op, CallerID: caller, ChildRisk: true, HasWork: len(r.requests) > 0})
		}
	}
	// Runtime capacity is reserved before the session algebra can install or join
	// an intent. Holding m.mu makes the cross-layer admission indivisible to other
	// runtime callers. Non-new algebra results release the provisional worker slot.
	if m.workers >= m.limits.MaxChildren || !m.canReserveOperation() {
		return session.LifecycleResult{Failure: session.ResourceExhausted}
	}
	m.workers++
	intent := m.algebra.Lifecycle(session.LifecycleRequest{SessionID: id, Generation: r.record.Generation, Operation: op, CallerID: caller, ChildRisk: true, HasWork: len(r.requests) > 0})
	if intent.Failure != "" || intent.Noop || intent.Joined || intent.Replayed {
		m.workers--
		return intent
	}
	if !m.reserveOperation(intent.IntentID) {
		// canReserveOperation and reserveOperation run under the same lock, so this
		// is defensive only: no concurrent runtime mutation can consume capacity.
		m.workers--
		return session.LifecycleResult{Failure: session.ResourceExhausted}
	}
	snapshot := OperationSnapshot{ID: intent.IntentID, SessionID: id, CallerID: caller, Generation: r.record.Generation, Restart: restart, State: OperationPending}
	m.operations[snapshot.ID] = snapshot
	m.operationIDs = append(m.operationIDs, snapshot.ID)
	m.observe(id, snapshot.Generation, kind, intent.State, "")
	go m.runLifecycle(snapshot, r.process, r.spec)
	return intent
}

func (m *Manager) canReserveOperation() bool {
	if len(m.operations) < m.limits.MaxOperations {
		return true
	}
	for _, id := range m.operationIDs {
		if m.operations[id].State != OperationPending {
			return true
		}
	}
	return false
}

func (m *Manager) reserveOperation(id string) bool {
	if _, exists := m.operations[id]; exists {
		return true
	}
	for len(m.operations) >= m.limits.MaxOperations {
		removed := false
		for i, candidate := range m.operationIDs {
			if m.operations[candidate].State != OperationPending {
				delete(m.operations, candidate)
				m.operationIDs = append(m.operationIDs[:i], m.operationIDs[i+1:]...)
				removed = true
				break
			}
		}
		if !removed {
			return false
		}
	}
	return true
}

func (m *Manager) runLifecycle(operation OperationSnapshot, child Child, spec managedprocess.Spec) {
	teardown := child.Teardown(context.Background())
	resources := child.Close()
	death := teardown.Death.Reap.Kind == managedprocess.ReapComplete
	observed := session.LifecycleCompletion{ShutdownComplete: true, UnsafeIOAbsent: resources.Kind == managedprocess.ResourcesClosed, TerminateSucceeded: death, DeathObserved: death, NoContainedSurvivors: death, StderrDrainComplete: true, Reaped: death, InitializationComplete: true}

	m.mu.Lock()
	completed := m.algebra.CompleteLifecycleObserved(operation.SessionID, operation.ID, observed)
	r := m.sessions[operation.SessionID]
	m.observe(operation.SessionID, operation.Generation, "teardown", completed.State, completed.Failure)
	if completed.Failure != "" || !death || resources.Kind != managedprocess.ResourcesClosed {
		operation.State, operation.Failure = OperationFailed, completed.Failure
		if operation.Failure == "" {
			operation.Failure = session.SessionReapIncomplete
		}
		if r != nil {
			r.record.State = session.Poisoned
		}
		m.finishOperation(operation)
		m.mu.Unlock()
		return
	}
	if !operation.Restart {
		delete(m.sessions, operation.SessionID)
		operation.State = OperationComplete
		m.finishOperation(operation)
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	next, start := m.starter.Start(context.Background(), spec)
	m.mu.Lock()
	defer m.mu.Unlock()
	if start.Kind != managedprocess.StartStarted || next == nil || r == nil {
		operation.State, operation.Failure = OperationFailed, session.SpawnFailure
		if r != nil {
			r.record.State = session.Poisoned
		}
		m.observe(operation.SessionID, completed.Generation, "poison", session.Poisoned, session.SpawnFailure)
		m.finishOperation(operation)
		return
	}
	r.process, r.record.Generation, r.record.State = next, completed.Generation, session.Ready
	r.pending, r.requests, r.cancels = lspwire.NewPending(m.limits.MaxTombstones), make(map[lspwire.RequestKey]*Request), 0
	m.observe(operation.SessionID, completed.Generation, "startup", session.Starting, "")
	m.observe(operation.SessionID, completed.Generation, "initialization", session.Ready, "")
	operation.State = OperationComplete
	m.finishOperation(operation)
}

func (m *Manager) finishOperation(operation OperationSnapshot) {
	m.operations[operation.ID] = operation
	m.workers--
	select {
	case m.workerDone <- struct{}{}:
	default:
	}
}

func (m *Manager) Operation(id string) (OperationSnapshot, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	snapshot, ok := m.operations[id]
	return snapshot, ok
}

func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	m.closed = true
	for m.workers > 0 {
		m.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-m.workerDone:
		}
		m.mu.Lock()
	}
	m.mu.Unlock()
	return nil
}

func (m *Manager) Census() Census {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := Census{Sessions: len(m.sessions), Generations: len(m.sessions), Children: len(m.sessions), Observations: len(m.observations), Operations: len(m.operations), Workers: m.workers}
	for _, r := range m.sessions {
		c.Requests += len(r.requests)
		c.Cancels += r.cancels
		c.Tombstones += r.pending.TombstoneCount()
	}
	return c
}
func (m *Manager) Records() []Record {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Record, 0, len(m.sessions))
	for _, r := range m.sessions {
		out = append(out, r.record)
	}
	return out
}
func (m *Manager) Observations() []Observation {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Observation(nil), m.observations...)
}
func (m *Manager) observe(id string, generation uint64, kind string, state session.State, failure session.Failure) {
	m.sequence++
	m.observations = append(m.observations, Observation{Sequence: m.sequence, SessionID: id, Generation: generation, Kind: kind, State: state, Failure: failure})
	if over := len(m.observations) - m.limits.MaxObservations; over > 0 {
		copy(m.observations, m.observations[over:])
		m.observations = m.observations[:m.limits.MaxObservations]
	}
}
