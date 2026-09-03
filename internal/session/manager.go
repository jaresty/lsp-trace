package session

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"
)

type ManagerConfig struct {
	MaxSessions           int
	MaxTerminalHistory    int
	MaxListPage           int
	MaxListSnapshots      int
	CursorRetentionEvents uint64
}

const (
	defaultMaxTerminalHistory    = 256
	defaultMaxListPage           = 100
	defaultMaxListSnapshots      = 16
	defaultCursorRetentionEvents = 64
	defaultMaxIntentHistory      = 32
	defaultMaxLifecycleLedger    = 1024
	defaultMaxIntentCallers      = 128
)

type AdmissionKind string

const (
	AdmissionFree    AdmissionKind = "FREE"
	AdmissionEvict   AdmissionKind = "EVICT"
	AdmissionBlocked AdmissionKind = "BLOCKED"
)

type Admission struct {
	Kind                           AdmissionKind
	SessionID, Victim, Reservation string
}
type TerminalRecord struct {
	SessionID                             string
	ManagerSeq, LocalSeq                  uint64
	CreationManagerSeq, LastUseManagerSeq uint64
	State                                 string
}
type ListOptions struct {
	Limit             int
	Cursor, SessionID string
}
type Page struct {
	Records    []TerminalRecord
	NextCursor string
	Truncated  bool
	Code       Failure
}
type liveRecord struct {
	sessionID             string
	managerSeq            uint64
	reserved, quarantined bool
}
type reservation struct {
	id, victim, contender string
	contenderSeq          uint64
	victimPriorState      State
}

type evictionWaiter struct {
	contender string
	sequence  uint64
	result    Admission
}

type Manager struct {
	mu               sync.Mutex
	max              int
	live             map[string]*liveRecord
	reservations     map[string]reservation
	waiters          []*evictionWaiter
	listSnapshots    map[string][]TerminalRecord
	issuedCursors    map[string]string
	snapshotExpiry   map[string]uint64
	snapshotOrder    []string
	cursorOrder      []string
	nextReservation  uint64
	nextManagerEvent uint64
	maxHistory       int
	maxListPage      int
	maxSnapshots     int
	cursorTTL        uint64
	lifecycles       map[string]*managedLifecycle
	lifecycleLedger  []LifecycleLedgerEntry
	nextIntent       uint64
}

type LifecycleOperation string

const (
	LifecycleStatus  LifecycleOperation = "STATUS"
	LifecycleStop    LifecycleOperation = "STOP"
	LifecycleRestart LifecycleOperation = "RESTART"
)

type LifecycleRequest struct {
	SessionID  string
	Generation uint64
	Operation  LifecycleOperation
	CallerID   string
	ChildRisk  bool
	HasWork    bool
}

type LifecycleOutcome string
type LifecycleOperationStatus string

const (
	OutcomeComplete    LifecycleOutcome = "COMPLETE"
	OutcomeDomainError LifecycleOutcome = "DOMAIN_ERROR"
	OutcomeCancelled   LifecycleOutcome = "CANCELLED"
	OutcomeTimedOut    LifecycleOutcome = "TIMED_OUT"

	StatusSucceeded LifecycleOperationStatus = "SUCCEEDED"
	StatusFailed    LifecycleOperationStatus = "FAILED"
	StatusCancelled LifecycleOperationStatus = "CANCELLED"
	StatusTimedOut  LifecycleOperationStatus = "TIMED_OUT"
	StatusNoop      LifecycleOperationStatus = "NOOP"
)

type LifecycleResult struct {
	State, PriorState State
	Generation        uint64
	IntentID          string
	Kind              RestartValue
	Outcome           LifecycleOutcome
	OperationStatus   LifecycleOperationStatus
	Failure           Failure
	Joined, Replayed  bool
	Noop, Superseded  bool
}

type LifecycleCompletion struct {
	ShutdownComplete       bool
	UnsafeIOAbsent         bool
	TerminateSucceeded     bool
	DeathObserved          bool
	NoContainedSurvivors   bool
	StderrDrainComplete    bool
	Reaped                 bool
	InitializationComplete bool
}

type LifecycleLedgerEntry struct {
	Seq, Generation                       uint64
	SessionID, IntentID, CallerID, Action string
	State                                 State
	Kind                                  RestartValue
	Failure                               Failure
}

type lifecycleCaller struct {
	attached        bool
	boundGeneration uint64
	failure         Failure
	result          LifecycleResult
}

type lifecycleIntent struct {
	id               string
	kind             RestartValue
	targetGeneration uint64
	terminal         bool
	result           LifecycleResult
	callers          map[string]*lifecycleCaller
}

type managedLifecycle struct {
	generation                            uint64
	state                                 State
	reaped, hasWork, unsafe               bool
	queued, active                        int
	nextEvent                             uint64
	creationManagerSeq, lastUseManagerSeq uint64
	incumbent                             *lifecycleIntent
	history                               []*lifecycleIntent
}

func NewManager(config ManagerConfig) (*Manager, error) {
	if config.MaxSessions <= 0 {
		return nil, errors.New("max sessions must be positive")
	}
	if config.MaxTerminalHistory <= 0 {
		config.MaxTerminalHistory = defaultMaxTerminalHistory
	}
	if config.MaxListPage <= 0 {
		config.MaxListPage = defaultMaxListPage
	}
	if config.MaxListSnapshots <= 0 {
		config.MaxListSnapshots = defaultMaxListSnapshots
	}
	if config.CursorRetentionEvents == 0 {
		config.CursorRetentionEvents = defaultCursorRetentionEvents
	}
	return &Manager{
		max: config.MaxSessions, maxHistory: config.MaxTerminalHistory, maxListPage: config.MaxListPage,
		maxSnapshots: config.MaxListSnapshots, cursorTTL: config.CursorRetentionEvents,
		live: make(map[string]*liveRecord), reservations: make(map[string]reservation),
		listSnapshots: make(map[string][]TerminalRecord), issuedCursors: make(map[string]string),
		snapshotExpiry: make(map[string]uint64), lifecycles: make(map[string]*managedLifecycle),
	}, nil
}

func (m *Manager) RegisterLifecycle(sessionID string, generation uint64, state State, reaped bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if sessionID == "" || generation == 0 || !validLifecycleState(state) || m.lifecycles[sessionID] != nil {
		return
	}
	managerSeq := m.allocateManagerEvent()
	m.lifecycles[sessionID] = &managedLifecycle{generation: generation, state: state, reaped: reaped, creationManagerSeq: managerSeq, lastUseManagerSeq: managerSeq}
	m.appendLifecycle(sessionID, generation, "register", "", "", state, "", "")
}

func (m *Manager) Lifecycle(request LifecycleRequest) (result LifecycleResult) {
	defer func() { result = classifyLifecycleResult(result) }()
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.lifecycles[request.SessionID]
	if s == nil {
		return LifecycleResult{Failure: SessionNotFound}
	}
	if request.Generation != 0 && request.Generation != s.generation {
		m.observeLifecycleDecision(request.SessionID, s, "stale", request)
		return LifecycleResult{State: s.state, Generation: s.generation, Failure: StaleGeneration}
	}
	if request.Operation == LifecycleStatus {
		m.observeLifecycleDecision(request.SessionID, s, "status", request)
		return LifecycleResult{State: s.state, Generation: s.generation}
	}
	if request.Operation != LifecycleStop && request.Operation != LifecycleRestart {
		m.observeLifecycleDecision(request.SessionID, s, "conflict", request)
		return LifecycleResult{State: s.state, Generation: s.generation, Failure: LifecycleConflict}
	}
	kind := StopIntent
	if request.Operation == LifecycleRestart {
		kind = RestartIntent
	}
	if s.incumbent != nil && !s.incumbent.terminal {
		if request.CallerID == "" {
			m.observeLifecycleDecision(request.SessionID, s, "conflict", request)
			return LifecycleResult{State: s.state, Generation: s.generation, Failure: LifecycleConflict}
		}
		if s.incumbent.targetGeneration != s.generation {
			m.observeLifecycleDecision(request.SessionID, s, "stale", request)
			return LifecycleResult{State: s.state, Generation: s.generation, Failure: StaleGeneration}
		}
		if s.incumbent.kind != kind {
			m.observeLifecycleDecision(request.SessionID, s, "conflict", request)
			return LifecycleResult{State: s.state, Generation: s.generation, Failure: LifecycleConflict}
		}
		if _, duplicate := s.incumbent.callers[request.CallerID]; duplicate {
			m.observeLifecycleDecision(request.SessionID, s, "conflict", request)
			return LifecycleResult{State: s.state, Generation: s.generation, Failure: LifecycleConflict}
		}
		if len(s.incumbent.callers) >= defaultMaxIntentCallers {
			m.observeLifecycleDecision(request.SessionID, s, "resource-exhausted", request)
			return LifecycleResult{State: s.state, Generation: s.generation, IntentID: s.incumbent.id, Kind: kind, Failure: ResourceExhausted}
		}
		s.incumbent.callers[request.CallerID] = &lifecycleCaller{attached: true}
		result := LifecycleResult{State: s.state, Generation: s.generation, IntentID: s.incumbent.id, Kind: kind, Joined: true}
		m.appendLifecycle(request.SessionID, s.generation, "join", s.incumbent.id, request.CallerID, s.state, kind, "")
		return result
	}
	if replay := compatibleTerminalReplay(s, kind); replay != nil {
		result := replay.result
		result.Replayed = true
		m.appendLifecycle(request.SessionID, s.generation, "replay", replay.id, request.CallerID, s.state, kind, result.Failure)
		return result
	}
	if kind == StopIntent {
		if s.state == Stopped || ((s.state == Crashed || s.state == Poisoned) && s.reaped) {
			m.observeLifecycleDecision(request.SessionID, s, "noop", request)
			return LifecycleResult{State: s.state, Generation: s.generation, Kind: kind, Noop: true}
		}
	} else if (s.state == Crashed || s.state == Poisoned) && !s.reaped {
		m.observeLifecycleDecision(request.SessionID, s, "reap-incomplete", request)
		return LifecycleResult{State: s.state, Generation: s.generation, Kind: kind, Failure: SessionReapIncomplete}
	}
	if s.state == Draining || s.state == Stopping {
		prior := s.state
		s.state = Poisoned
		m.appendLifecycle(request.SessionID, s.generation, "invariant-failure", "", request.CallerID, s.state, kind, SessionPoisoned)
		return LifecycleResult{State: Poisoned, PriorState: prior, Generation: s.generation, Kind: kind, Failure: SessionPoisoned}
	}
	if request.CallerID == "" {
		m.observeLifecycleDecision(request.SessionID, s, "conflict", request)
		return LifecycleResult{State: s.state, Generation: s.generation, Failure: LifecycleConflict}
	}
	m.nextIntent++
	intentID := fmt.Sprintf("i%d", m.nextIntent)
	intent := &lifecycleIntent{id: intentID, kind: kind, targetGeneration: s.generation, callers: map[string]*lifecycleCaller{request.CallerID: {attached: true}}}
	s.incumbent = intent
	prior := s.state
	s.hasWork = request.HasWork
	s.state = lifecycleIntentState(s.state, request.ChildRisk, request.HasWork)
	result = LifecycleResult{State: s.state, PriorState: prior, Generation: s.generation, IntentID: intentID, Kind: kind}
	m.appendLifecycle(request.SessionID, s.generation, "insert", intentID, request.CallerID, s.state, kind, "")
	return result
}

func lifecycleIntentState(state State, childRisk, hasWork bool) State {
	switch state {
	case Starting, Initializing:
		if childRisk {
			return Stopping
		}
		return Stopped
	case Ready:
		if hasWork {
			return Draining
		}
		return Stopping
	default:
		return state
	}
}

func compatibleTerminalReplay(s *managedLifecycle, kind RestartValue) *lifecycleIntent {
	if s.state != Stopped || kind != StopIntent {
		return nil
	}
	for i := len(s.history) - 1; i >= 0; i-- {
		if s.history[i].kind == StopIntent && s.history[i].targetGeneration == s.generation {
			return s.history[i]
		}
	}
	return nil
}

func classifyLifecycleResult(result LifecycleResult) LifecycleResult {
	if result.Failure == RequestCancelled {
		result.Outcome, result.OperationStatus = OutcomeCancelled, StatusCancelled
	} else if result.Failure == RequestTimeout {
		result.Outcome, result.OperationStatus = OutcomeTimedOut, StatusTimedOut
	} else if result.Failure != "" {
		result.Outcome, result.OperationStatus = OutcomeDomainError, StatusFailed
	} else if result.Noop {
		result.Outcome, result.OperationStatus = OutcomeComplete, StatusNoop
	} else {
		result.Outcome, result.OperationStatus = OutcomeComplete, StatusSucceeded
	}
	return result
}

func (m *Manager) CompleteLifecycle(sessionID, intentID string, deathObserved, reaped, ready bool) LifecycleResult {
	return m.CompleteLifecycleObserved(sessionID, intentID, LifecycleCompletion{
		ShutdownComplete: true, UnsafeIOAbsent: true, TerminateSucceeded: true,
		DeathObserved: deathObserved, NoContainedSurvivors: deathObserved,
		StderrDrainComplete: deathObserved, Reaped: reaped, InitializationComplete: ready,
	})
}

func (m *Manager) CompleteLifecycleObserved(sessionID, intentID string, observed LifecycleCompletion) LifecycleResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.lifecycles[sessionID]
	if s == nil {
		return classifyLifecycleResult(LifecycleResult{Failure: SessionNotFound})
	}
	intent := s.incumbent
	if intent == nil || intent.id != intentID {
		m.appendLifecycle(sessionID, s.generation, "completion-conflict", intentID, "", s.state, "", LifecycleConflict)
		return classifyLifecycleResult(LifecycleResult{State: s.state, Generation: s.generation, Failure: LifecycleConflict})
	}
	failure := firstTeardownFailure(observed)
	if observed.DeathObserved {
		m.appendLifecycle(sessionID, s.generation, "death", intent.id, "", s.state, intent.kind, "")
	}
	if observed.StderrDrainComplete {
		m.appendLifecycle(sessionID, s.generation, "drain", intent.id, "", s.state, intent.kind, "")
	}
	if observed.Reaped {
		m.appendLifecycle(sessionID, s.generation, "reap", intent.id, "", s.state, intent.kind, "")
	}
	if failure != "" {
		m.appendLifecycle(sessionID, s.generation, "poison", intent.id, "", Poisoned, intent.kind, failure)
		return m.terminalizeLifecycle(sessionID, s, intent, classifyLifecycleResult(LifecycleResult{State: Poisoned, Generation: s.generation, IntentID: intent.id, Kind: intent.kind, Failure: failure}))
	}
	s.reaped = true
	result := LifecycleResult{State: Stopped, Generation: s.generation, IntentID: intent.id, Kind: intent.kind}
	if intent.kind == RestartIntent {
		s.generation++
		result.Generation = s.generation
		s.state = Starting
		m.appendLifecycle(sessionID, s.generation, "allocate", intent.id, "", Starting, intent.kind, "")
		if !observed.InitializationComplete {
			m.appendLifecycle(sessionID, s.generation, "poison", intent.id, "", Poisoned, intent.kind, InitializationFailure)
			result.State, result.Failure = Poisoned, InitializationFailure
			return m.terminalizeLifecycle(sessionID, s, intent, classifyLifecycleResult(result))
		}
		m.appendLifecycle(sessionID, s.generation, "initialized", intent.id, "", Initializing, intent.kind, "")
		s.state, result.State = Ready, Ready
		m.appendLifecycle(sessionID, s.generation, "ready", intent.id, "", Ready, intent.kind, "")
		callerIDs := make([]string, 0, len(intent.callers))
		for callerID, caller := range intent.callers {
			if caller.attached {
				callerIDs = append(callerIDs, callerID)
			}
		}
		sort.Strings(callerIDs)
		for _, callerID := range callerIDs {
			intent.callers[callerID].boundGeneration = s.generation
			m.appendLifecycle(sessionID, s.generation, "bind", intent.id, callerID, Ready, intent.kind, "")
		}
	} else {
		s.state = Stopped
	}
	return m.terminalizeLifecycle(sessionID, s, intent, classifyLifecycleResult(result))
}

func firstTeardownFailure(observed LifecycleCompletion) Failure {
	switch {
	case !observed.ShutdownComplete:
		return SessionPoisoned
	case !observed.UnsafeIOAbsent:
		return SessionPoisoned
	case !observed.TerminateSucceeded:
		return SessionReapIncomplete
	case !observed.DeathObserved:
		return SessionReapIncomplete
	case !observed.NoContainedSurvivors:
		return SessionReapIncomplete
	case !observed.StderrDrainComplete:
		return SessionPoisoned
	case !observed.Reaped:
		return SessionReapIncomplete
	default:
		return ""
	}
}

func (m *Manager) terminalizeLifecycle(sessionID string, s *managedLifecycle, intent *lifecycleIntent, result LifecycleResult) LifecycleResult {
	result = classifyLifecycleResult(result)
	s.state = result.State
	intent.terminal = true
	intent.result = result
	for _, caller := range intent.callers {
		if caller.attached {
			caller.result = result
		}
	}
	s.history = append(s.history, intent)
	if overflow := len(s.history) - defaultMaxIntentHistory; overflow > 0 {
		copy(s.history, s.history[overflow:])
		s.history = s.history[:defaultMaxIntentHistory]
	}
	s.incumbent = nil
	m.appendLifecycle(sessionID, s.generation, "terminal", intent.id, "", s.state, intent.kind, result.Failure)
	return result
}

func validLifecycleState(state State) bool {
	for _, candidate := range PublicStates() {
		if state == candidate {
			return true
		}
	}
	return false
}

func (m *Manager) DetachLifecycleCaller(sessionID, intentID, callerID string, failure Failure) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.lifecycles[sessionID]
	if s == nil || s.incumbent == nil || s.incumbent.id != intentID {
		return false
	}
	caller := s.incumbent.callers[callerID]
	if caller == nil || !caller.attached || (failure != RequestCancelled && failure != RequestTimeout) {
		return false
	}
	caller.attached, caller.failure = false, failure
	caller.result = classifyLifecycleResult(LifecycleResult{State: s.state, Generation: s.generation, IntentID: intentID, Kind: s.incumbent.kind, Failure: failure})
	m.appendLifecycle(sessionID, s.generation, "detach", intentID, callerID, s.state, s.incumbent.kind, failure)
	return true
}

func (m *Manager) LifecycleCallerResult(sessionID, intentID, callerID string) (LifecycleResult, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.lifecycles[sessionID]
	if s == nil {
		return LifecycleResult{}, false
	}
	intents := append([]*lifecycleIntent(nil), s.history...)
	if s.incumbent != nil {
		intents = append(intents, s.incumbent)
	}
	for _, intent := range intents {
		if intent.id == intentID {
			caller := intent.callers[callerID]
			if caller == nil || (caller.result == LifecycleResult{}) {
				return LifecycleResult{}, false
			}
			return caller.result, true
		}
	}
	return LifecycleResult{}, false
}

func (m *Manager) ObserveInitialization(sessionID string, generation uint64, complete bool) LifecycleResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.lifecycles[sessionID]
	if s == nil {
		return classifyLifecycleResult(LifecycleResult{Failure: SessionNotFound})
	}
	if generation != s.generation {
		m.appendLifecycle(sessionID, s.generation, "initialization-stale", "", "", s.state, "", StaleGeneration)
		return classifyLifecycleResult(LifecycleResult{State: s.state, Generation: s.generation, Failure: StaleGeneration})
	}
	if s.incumbent != nil {
		m.appendLifecycle(sessionID, generation, "initialization-discarded", s.incumbent.id, "", s.state, s.incumbent.kind, "")
		return classifyLifecycleResult(LifecycleResult{State: s.state, Generation: generation})
	}
	if s.state != Initializing {
		return classifyLifecycleResult(LifecycleResult{State: s.state, Generation: generation, Failure: SessionPoisoned})
	}
	if !complete {
		s.state = Poisoned
		m.appendLifecycle(sessionID, generation, "poison", "", "", Poisoned, "", InitializationFailure)
		return classifyLifecycleResult(LifecycleResult{State: Poisoned, Generation: generation, Failure: InitializationFailure})
	}
	s.state = Ready
	m.appendLifecycle(sessionID, generation, "initialized", "", "", Ready, "", "")
	return classifyLifecycleResult(LifecycleResult{State: Ready, Generation: generation})
}

func (m *Manager) ObservePoison(sessionID string, generation uint64) LifecycleResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.lifecycles[sessionID]
	if s == nil {
		return classifyLifecycleResult(LifecycleResult{Failure: SessionNotFound})
	}
	if generation != s.generation {
		m.appendLifecycle(sessionID, s.generation, "poison-stale", "", "", s.state, "", StaleGeneration)
		return classifyLifecycleResult(LifecycleResult{State: s.state, Generation: s.generation, Failure: StaleGeneration})
	}
	s.state = Poisoned
	m.appendLifecycle(sessionID, generation, "poison", "", "", Poisoned, "", SessionPoisoned)
	return classifyLifecycleResult(LifecycleResult{State: Poisoned, Generation: generation, Failure: SessionPoisoned})
}

func (m *Manager) LifecycleLedger() []LifecycleLedgerEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]LifecycleLedgerEntry(nil), m.lifecycleLedger...)
}

func (m *Manager) allocateManagerEvent() uint64 {
	if m.nextManagerEvent != ^uint64(0) {
		m.nextManagerEvent++
	}
	return m.nextManagerEvent
}

func (m *Manager) observeLifecycleDecision(sessionID string, s *managedLifecycle, action string, request LifecycleRequest) {
	m.appendLifecycle(sessionID, s.generation, action, "", request.CallerID, s.state, "", "")
}

func (m *Manager) appendLifecycle(sessionID string, generation uint64, action, intentID, callerID string, state State, kind RestartValue, failure Failure) {
	s := m.lifecycles[sessionID]
	if s == nil || s.nextEvent == ^uint64(0) {
		return
	}
	s.nextEvent++
	m.lifecycleLedger = append(m.lifecycleLedger, LifecycleLedgerEntry{Seq: s.nextEvent, SessionID: sessionID, Generation: generation, Action: action, IntentID: intentID, CallerID: callerID, State: state, Kind: kind, Failure: failure})
	m.expireListState()
	if overflow := len(m.lifecycleLedger) - defaultMaxLifecycleLedger; overflow > 0 {
		copy(m.lifecycleLedger, m.lifecycleLedger[overflow:])
		m.lifecycleLedger = m.lifecycleLedger[:defaultMaxLifecycleLedger]
	}
}
func (m *Manager) Admit(sessionID string, _ uint64) Admission {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sessionID == "" || m.nextManagerEvent == ^uint64(0) {
		return Admission{Kind: AdmissionBlocked, SessionID: sessionID}
	}
	m.nextManagerEvent++
	managerSeq := m.nextManagerEvent
	if existing := m.live[sessionID]; existing != nil {
		if lifecycle := m.lifecycles[sessionID]; lifecycle != nil {
			lifecycle.lastUseManagerSeq = managerSeq
		}
		return Admission{Kind: AdmissionFree, SessionID: sessionID}
	}
	if len(m.live) < m.max {
		m.live[sessionID] = &liveRecord{sessionID: sessionID, managerSeq: managerSeq}
		if m.lifecycles[sessionID] == nil {
			m.lifecycles[sessionID] = &managedLifecycle{generation: 1, state: Stopped, reaped: true, creationManagerSeq: managerSeq, lastUseManagerSeq: managerSeq}
			m.appendLifecycle(sessionID, 1, "admit", "", "", Stopped, "", "")
		}
		return Admission{Kind: AdmissionFree, SessionID: sessionID}
	}
	for _, waiter := range m.waiters {
		if waiter.contender == sessionID {
			return waiter.result
		}
	}
	if len(m.waiters) >= m.max {
		return Admission{Kind: AdmissionBlocked, SessionID: sessionID}
	}
	waiter := &evictionWaiter{contender: sessionID, sequence: managerSeq, result: Admission{Kind: AdmissionBlocked, SessionID: sessionID}}
	if !m.reserveVictim(waiter) {
		return waiter.result
	}
	m.waiters = append(m.waiters, waiter)
	return waiter.result
}

func (m *Manager) reserveVictim(waiter *evictionWaiter) bool {
	victims := make([]*liveRecord, 0, len(m.live))
	for _, record := range m.live {
		lifecycle := m.lifecycles[record.sessionID]
		if evictionClass(record, lifecycle) >= 0 {
			victims = append(victims, record)
		}
	}
	sort.Slice(victims, func(i, j int) bool {
		leftLifecycle, rightLifecycle := m.lifecycles[victims[i].sessionID], m.lifecycles[victims[j].sessionID]
		if leftClass, rightClass := evictionClass(victims[i], leftLifecycle), evictionClass(victims[j], rightLifecycle); leftClass != rightClass {
			return leftClass < rightClass
		}
		if leftLifecycle.lastUseManagerSeq != rightLifecycle.lastUseManagerSeq {
			return leftLifecycle.lastUseManagerSeq < rightLifecycle.lastUseManagerSeq
		}
		if leftLifecycle.creationManagerSeq != rightLifecycle.creationManagerSeq {
			return leftLifecycle.creationManagerSeq < rightLifecycle.creationManagerSeq
		}
		return victims[i].sessionID < victims[j].sessionID
	})
	for _, victim := range victims {
		victimLifecycle := m.lifecycles[victim.sessionID]
		victim.reserved = true
		m.nextReservation++
		id := fmt.Sprintf("r%d", m.nextReservation)
		m.nextIntent++
		intentID := fmt.Sprintf("i%d", m.nextIntent)
		intent := &lifecycleIntent{id: intentID, kind: EvictIntent, targetGeneration: victimLifecycle.generation, callers: map[string]*lifecycleCaller{waiter.contender: {attached: true}}}
		victimLifecycle.incumbent = intent
		prior := victimLifecycle.state
		victimLifecycle.state = lifecycleIntentState(victimLifecycle.state, false, false)
		m.appendLifecycle(victim.sessionID, victimLifecycle.generation, "insert", intentID, waiter.contender, victimLifecycle.state, EvictIntent, "")
		m.reservations[id] = reservation{id: id, victim: victim.sessionID, contender: waiter.contender, contenderSeq: waiter.sequence, victimPriorState: prior}
		waiter.result = Admission{Kind: AdmissionEvict, SessionID: waiter.contender, Victim: victim.sessionID, Reservation: id}
		return true
	}
	waiter.result = Admission{Kind: AdmissionBlocked, SessionID: waiter.contender}
	return false
}

func (m *Manager) releaseVictimSlot(r reservation) {
	delete(m.reservations, r.id)
	victimLifecycle := m.lifecycles[r.victim]
	intent := victimLifecycle.incumbent
	m.terminalizeLifecycle(r.victim, victimLifecycle, intent, LifecycleResult{State: Stopped, Generation: victimLifecycle.generation, IntentID: intent.id, Kind: EvictIntent})
	delete(m.live, r.victim)
	delete(m.lifecycles, r.victim)
	for _, waiter := range m.waiters {
		if waiter.result.Reservation == r.id {
			waiter.result = Admission{Kind: AdmissionBlocked, SessionID: waiter.contender}
			break
		}
	}
}

func (m *Manager) supersedeWaiterReservation(waiter *evictionWaiter) bool {
	id := waiter.result.Reservation
	if id == "" {
		return true
	}
	r, ok := m.reservations[id]
	if !ok || r.contender != waiter.contender {
		return false
	}
	victim := m.live[r.victim]
	victimLifecycle := m.lifecycles[r.victim]
	if victim == nil || !victim.reserved || victimLifecycle == nil || victimLifecycle.incumbent == nil || victimLifecycle.incumbent.kind != EvictIntent {
		return false
	}
	intent := victimLifecycle.incumbent
	result := LifecycleResult{State: r.victimPriorState, PriorState: victimLifecycle.state, Generation: victimLifecycle.generation, IntentID: intent.id, Kind: EvictIntent, Superseded: true}
	m.terminalizeLifecycle(r.victim, victimLifecycle, intent, result)
	victim.reserved = false
	delete(m.reservations, id)
	return true
}

func (m *Manager) reprocessWaiters() Admission {
	sort.SliceStable(m.waiters, func(i, j int) bool {
		if m.waiters[i].sequence != m.waiters[j].sequence {
			return m.waiters[i].sequence < m.waiters[j].sequence
		}
		return m.waiters[i].contender < m.waiters[j].contender
	})
	selected := Admission{Kind: AdmissionBlocked}
	kept := m.waiters[:0]
	for _, waiter := range m.waiters {
		if m.live[waiter.contender] != nil {
			if !m.supersedeWaiterReservation(waiter) {
				kept = append(kept, waiter)
			}
			continue
		}
		if len(m.live) < m.max {
			if !m.supersedeWaiterReservation(waiter) {
				kept = append(kept, waiter)
				continue
			}
			m.live[waiter.contender] = &liveRecord{sessionID: waiter.contender, managerSeq: waiter.sequence}
			m.lifecycles[waiter.contender] = &managedLifecycle{generation: 1, state: Stopped, reaped: true, creationManagerSeq: waiter.sequence, lastUseManagerSeq: waiter.sequence}
			m.appendLifecycle(waiter.contender, 1, "admit", "", "", Stopped, "", "")
			admitted := Admission{Kind: AdmissionFree, SessionID: waiter.contender}
			if selected.Kind != AdmissionFree {
				selected = admitted
			}
			continue
		}
		if waiter.result.Reservation == "" {
			m.reserveVictim(waiter)
		}
		kept = append(kept, waiter)
	}
	m.waiters = kept
	return selected
}

func (m *Manager) CompleteEviction(id string, success bool) Admission {
	m.mu.Lock()
	defer m.mu.Unlock()

	r, ok := m.reservations[id]
	if !ok {
		return Admission{Kind: AdmissionBlocked}
	}
	victim := m.live[r.victim]
	victimLifecycle := m.lifecycles[r.victim]
	if victim == nil || !victim.reserved || victimLifecycle == nil || victimLifecycle.incumbent == nil || victimLifecycle.incumbent.kind != EvictIntent || victimLifecycle.incumbent.callers[r.contender] == nil {
		return Admission{Kind: AdmissionBlocked, SessionID: r.contender, Victim: r.victim}
	}
	intent := victimLifecycle.incumbent
	if !success {
		delete(m.reservations, id)
		m.removeWaiter(r.contender)
		m.terminalizeLifecycle(r.victim, victimLifecycle, intent, LifecycleResult{State: Poisoned, Generation: victimLifecycle.generation, IntentID: intent.id, Kind: EvictIntent, Failure: SessionPoisoned})
		victim.reserved = false
		victim.quarantined = true
		return Admission{Kind: AdmissionBlocked, SessionID: r.contender, Victim: r.victim}
	}
	m.releaseVictimSlot(r)
	return m.reprocessWaiters()
}

func (m *Manager) removeWaiter(contender string) {
	for i, waiter := range m.waiters {
		if waiter.contender == contender {
			m.waiters = append(m.waiters[:i], m.waiters[i+1:]...)
			return
		}
	}
}

// ReleaseStopped frees admission capacity only for the exact generation whose
// lifecycle is confirmed stopped and reaped. Lifecycle history remains retained.
func (m *Manager) ReleaseStopped(sessionID string, generation uint64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	lifecycle := m.lifecycles[sessionID]
	if lifecycle == nil || lifecycle.generation != generation || lifecycle.state != Stopped || !lifecycle.reaped || lifecycle.incumbent != nil {
		return false
	}
	if _, ok := m.live[sessionID]; !ok {
		return false
	}
	delete(m.live, sessionID)
	return true
}

func (m *Manager) LiveCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.live)
}

func evictionClass(record *liveRecord, lifecycle *managedLifecycle) int {
	if record == nil || lifecycle == nil || record.reserved || record.quarantined || lifecycle.incumbent != nil || lifecycle.hasWork || lifecycle.queued != 0 || lifecycle.active != 0 || lifecycle.unsafe {
		return -1
	}
	switch lifecycle.state {
	case Stopped, Crashed, Poisoned:
		if lifecycle.reaped {
			return 0
		}
	case Ready:
		return 1
	}
	return -1
}

const (
	listCursorVersion  = 1
	managerGlobalOrder = "manager-global"
	sessionLocalOrder  = "session-local"
)

type listCursor struct {
	Version  int    `json:"version"`
	Filter   string `json:"filter"`
	Order    string `json:"order"`
	Snapshot string `json:"snapshot"`
	Position int    `json:"position"`
}

func (m *Manager) List(options ListOptions) Page {
	m.mu.Lock()
	defer m.mu.Unlock()

	order := managerGlobalOrder
	if options.SessionID != "" {
		order = sessionLocalOrder
	}

	start := 0
	var records []TerminalRecord
	var snapshotEvent uint64
	if options.Cursor != "" {
		m.expireListState()
		issuedSnapshot, issued := m.issuedCursors[options.Cursor]
		cursor, ok := decodeListCursor(options.Cursor)
		if !issued || !ok || issuedSnapshot != cursor.Snapshot || cursor.Version != listCursorVersion || cursor.Filter != options.SessionID || cursor.Order != order {
			return Page{Code: ListCursorInvalid}
		}
		snapshot, exists := m.listSnapshots[cursor.Snapshot]
		if !exists || cursor.Position <= 0 || cursor.Position >= len(snapshot) {
			return Page{Code: ListCursorInvalid}
		}
		records = snapshot
		start = cursor.Position
	} else {
		snapshotEvent = m.allocateManagerEvent()
		records = m.orderedRecords(options.SessionID, order)
	}

	limit := options.Limit
	if limit <= 0 || limit > m.maxListPage {
		limit = m.maxListPage
	}
	end := start + limit
	if end > len(records) {
		end = len(records)
	}
	page := Page{Records: append([]TerminalRecord(nil), records[start:end]...), Truncated: end < len(records)}
	if page.Truncated {
		snapshotID := listSnapshotID(snapshotEvent, options.SessionID, order, records)
		m.retainListSnapshot(snapshotID, records)
		page.NextCursor = encodeListCursor(listCursor{Version: listCursorVersion, Filter: options.SessionID, Order: order, Snapshot: snapshotID, Position: end})
		m.retainListCursor(page.NextCursor, snapshotID)
	}
	return page
}

func (m *Manager) retainListSnapshot(id string, records []TerminalRecord) {
	if _, exists := m.listSnapshots[id]; !exists {
		m.listSnapshots[id] = append([]TerminalRecord(nil), records...)
		m.snapshotOrder = append(m.snapshotOrder, id)
	}
	m.snapshotExpiry[id] = m.nextManagerEvent + m.cursorTTL
	for len(m.snapshotOrder) > m.maxSnapshots {
		m.dropListSnapshot(m.snapshotOrder[0])
	}
}

func (m *Manager) retainListCursor(cursor, snapshot string) {
	if _, exists := m.issuedCursors[cursor]; !exists {
		m.cursorOrder = append(m.cursorOrder, cursor)
	}
	m.issuedCursors[cursor] = snapshot
	for len(m.cursorOrder) > m.maxSnapshots {
		delete(m.issuedCursors, m.cursorOrder[0])
		m.cursorOrder = m.cursorOrder[1:]
	}
}

func (m *Manager) expireListState() {
	for len(m.snapshotOrder) > 0 {
		id := m.snapshotOrder[0]
		if m.nextManagerEvent < m.snapshotExpiry[id] {
			break
		}
		m.dropListSnapshot(id)
	}
}

func (m *Manager) dropListSnapshot(id string) {
	delete(m.listSnapshots, id)
	delete(m.snapshotExpiry, id)
	if len(m.snapshotOrder) > 0 && m.snapshotOrder[0] == id {
		m.snapshotOrder = m.snapshotOrder[1:]
	} else {
		for i, candidate := range m.snapshotOrder {
			if candidate == id {
				m.snapshotOrder = append(m.snapshotOrder[:i], m.snapshotOrder[i+1:]...)
				break
			}
		}
	}
	kept := m.cursorOrder[:0]
	for _, cursor := range m.cursorOrder {
		if m.issuedCursors[cursor] == id {
			delete(m.issuedCursors, cursor)
		} else {
			kept = append(kept, cursor)
		}
	}
	m.cursorOrder = kept
}

func (m *Manager) orderedRecords(sessionID, order string) []TerminalRecord {
	records := make([]TerminalRecord, 0, len(m.lifecycles))
	for id, lifecycle := range m.lifecycles {
		if sessionID != "" && id != sessionID {
			continue
		}
		records = append(records, TerminalRecord{
			SessionID: id, ManagerSeq: lifecycle.lastUseManagerSeq, LocalSeq: lifecycle.nextEvent,
			CreationManagerSeq: lifecycle.creationManagerSeq, LastUseManagerSeq: lifecycle.lastUseManagerSeq,
			State: string(lifecycle.state),
		})
	}
	if order == sessionLocalOrder {
		sort.SliceStable(records, func(i, j int) bool { return records[i].LocalSeq < records[j].LocalSeq })
		return records
	}
	sort.SliceStable(records, func(i, j int) bool {
		left, right := records[i], records[j]
		if stateRank(left.State) != stateRank(right.State) {
			return stateRank(left.State) < stateRank(right.State)
		}
		if left.LastUseManagerSeq != right.LastUseManagerSeq {
			return left.LastUseManagerSeq < right.LastUseManagerSeq
		}
		if left.CreationManagerSeq != right.CreationManagerSeq {
			return left.CreationManagerSeq < right.CreationManagerSeq
		}
		return bytes.Compare([]byte(left.SessionID), []byte(right.SessionID)) < 0
	})
	return records
}

func stateRank(state string) int {
	for rank, candidate := range []string{"STARTING", "INITIALIZING", "READY", "DRAINING", "STOPPING", "STOPPED", "CRASHED", "POISONED"} {
		if state == candidate {
			return rank
		}
	}
	return 8
}

func listSnapshotID(event uint64, filter, order string, records []TerminalRecord) string {
	raw, _ := json.Marshal(struct {
		Filter  string           `json:"filter"`
		Order   string           `json:"order"`
		Records []TerminalRecord `json:"records"`
	}{filter, order, records})
	digest := sha256.Sum256(raw)
	return fmt.Sprintf("list-%d-%s", event, hex.EncodeToString(digest[:]))
}

func encodeListCursor(cursor listCursor) string {
	raw, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeListCursor(encoded string) (listCursor, bool) {
	raw, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil {
		return listCursor{}, false
	}
	var cursor listCursor
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cursor); err != nil {
		return listCursor{}, false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return listCursor{}, false
	}
	return cursor, cursor.Snapshot != ""
}
