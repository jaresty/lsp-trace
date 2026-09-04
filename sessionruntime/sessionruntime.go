// Package sessionruntime provides transport-neutral orchestration over the
// session algebra, resolved runtime identity, managed processes, and LSP wire.
// It neither registers nor advertises a command surface.
package sessionruntime

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strconv"
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

type ReadinessState string

const (
	ReadinessPending ReadinessState = "PENDING"
	ReadinessReady   ReadinessState = "READY"
	ReadinessFailed  ReadinessState = "FAILED"
)

type SessionMetadata struct {
	PositionEncoding     string
	CallHierarchySupport bool
}

type ReadinessSnapshot struct {
	ID               string
	SessionID        string
	Generation       uint64
	State            ReadinessState
	Failure          session.Failure
	Metadata         SessionMetadata
	Duration         time.Duration
	RequestMessages  int
	RequestBytes     int64
	ResponseMessages int
	ResponseBytes    int64
	ThermalPhase     string
}

type readinessOperation struct {
	snapshot ReadinessSnapshot
	started  time.Time
	done     chan struct{}
}

type Child interface {
	Teardown(context.Context) managedprocess.TeardownObservation
	Close() managedprocess.ResourceObservation
}

type wireChild interface {
	Child
	Stdin() io.WriteCloser
	Stdout() io.ReadCloser
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
	Limits           Limits
	Wire             lspwire.Limits
	Starter          Starter
	ReadinessTimeout time.Duration
	// Now is an optional monotonic clock seam for deterministic runtime observations.
	Now func() time.Time
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

// RoundTripRequest describes one transport-neutral JSON-RPC transaction against
// an exact session generation.
type RoundTripRequest struct {
	SessionID   string
	Generation  uint64
	Method      string
	Params      json.RawMessage
	Deadline    time.Time
	MaxMessages int
	MaxBytes    int64
}

// RoundTripResult is the immutable terminal observation of one transaction.
type RoundTripResult struct {
	Key             lspwire.RequestKey
	Result          json.RawMessage
	ServerError     *lspwire.RPCError
	Failure         session.Failure
	Messages        int
	Bytes           int64
	RequestMessages int
	RequestBytes    int64
	Duration        time.Duration
	ThermalPhase    string
	Notifications   []lspwire.Message
	Responses       []lspwire.Message
	started         time.Time
}

// RoundTrip executes one complete protocol transaction while exclusively
// owning the exact generation's stdin/stdout stream. Concurrent transactions
// and lifecycle operations are rejected rather than queued.
func (m *Manager) RoundTrip(parent context.Context, req RoundTripRequest) RoundTripResult {
	m.mu.Lock()
	r := m.sessions[req.SessionID]
	if r == nil {
		m.mu.Unlock()
		return RoundTripResult{Failure: session.SessionNotFound}
	}
	if req.Generation != r.record.Generation {
		m.mu.Unlock()
		return RoundTripResult{Failure: session.StaleGeneration}
	}
	if r.record.State != session.Ready || r.protocolOwned {
		m.mu.Unlock()
		return RoundTripResult{Failure: session.LifecycleConflict}
	}
	child, ok := r.process.(wireChild)
	if !ok {
		m.mu.Unlock()
		return RoundTripResult{Failure: session.SessionPoisoned}
	}
	if len(r.requests) >= m.limits.MaxRequests || m.workers >= m.limits.MaxChildren {
		m.mu.Unlock()
		return RoundTripResult{Failure: session.ResourceExhausted}
	}
	key := r.pending.Begin(req.Generation)
	r.requests[key] = &Request{Key: key, Deadline: req.Deadline}
	r.protocolOwned = true
	m.workers++
	m.observe(req.SessionID, req.Generation, "request", r.record.State, "")
	m.mu.Unlock()

	result := RoundTripResult{Key: key, ThermalPhase: "WARM", started: m.now()}
	maxMessages := req.MaxMessages
	if maxMessages <= 0 {
		maxMessages = 1
	}
	maxBytes := req.MaxBytes
	if maxBytes <= 0 {
		maxBytes = m.wire.MaxBodyBytes
		if maxBytes <= 0 {
			maxBytes = lspwire.DefaultLimits().MaxBodyBytes
		}
	}
	ctx := parent
	cancel := func() {}
	if !req.Deadline.IsZero() {
		ctx, cancel = context.WithDeadline(parent, req.Deadline)
	}
	defer cancel()

	writer := lspwire.NewWriter(child.Stdin(), m.wire)
	id := json.RawMessage(strconv.FormatUint(key.ID, 10))
	requestMessage := lspwire.Message{JSONRPC: lspwire.Version, ID: id, Method: req.Method, Params: req.Params}
	requestBody, _ := json.Marshal(requestMessage)
	result.RequestMessages = 1
	result.RequestBytes = int64(len(requestBody))
	if err := writer.Write(requestMessage); err != nil {
		return m.finishRoundTrip(req.SessionID, child, result, session.SessionPoisoned, true)
	}

	type readResult struct {
		message lspwire.Message
		err     error
	}
	reads := make(chan readResult, 1)
	reader := lspwire.NewReader(child.Stdout(), m.wire)
	for result.Messages < maxMessages {
		go func() { msg, err := reader.Read(); reads <- readResult{msg, err} }()
		select {
		case <-ctx.Done():
			state, _ := r.pending.Cancel(writer, key)
			if state == lspwire.CancelWritten {
				m.mu.Lock()
				r.cancels++
				m.observe(req.SessionID, req.Generation, "cancel", r.record.State, session.RequestCancelled)
				m.mu.Unlock()
			}
			failure := session.RequestCancelled
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				failure = session.RequestTimeout
			}
			return m.finishRoundTrip(req.SessionID, child, result, failure, true)
		case read := <-reads:
			if read.err != nil {
				failure := session.SessionPoisoned
				if errors.Is(read.err, io.EOF) {
					failure = session.SessionCrashed
				}
				return m.finishRoundTrip(req.SessionID, child, result, failure, true)
			}
			body, _ := json.Marshal(read.message)
			result.Messages++
			result.Bytes += int64(len(body))
			if result.Bytes > maxBytes {
				return m.finishRoundTrip(req.SessionID, child, result, session.ResourceExhausted, true)
			}
			if read.message.Kind() == lspwire.KindNotification || read.message.Kind() == lspwire.KindRequest {
				result.Notifications = append(result.Notifications, read.message)
				continue
			}
			responseID, err := strconv.ParseUint(string(read.message.ID), 10, 64)
			if err != nil || responseID != key.ID {
				result.Responses = append(result.Responses, read.message)
				continue
			}
			disposition := r.pending.Accept(lspwire.ResponseKey{Generation: req.Generation, ID: responseID})
			if disposition != lspwire.ResponseAccepted {
				result.Responses = append(result.Responses, read.message)
				continue
			}
			result.Result, result.ServerError = append(json.RawMessage(nil), read.message.Result...), read.message.Error
			return m.finishRoundTrip(req.SessionID, child, result, "", false)
		}
	}
	return m.finishRoundTrip(req.SessionID, child, result, session.ResourceExhausted, true)
}

func (m *Manager) finishRoundTrip(id string, child Child, result RoundTripResult, failure session.Failure, poison bool) RoundTripResult {
	if poison {
		_ = child.Teardown(context.Background())
		_ = child.Close()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if r := m.sessions[id]; r != nil && r.record.Generation == result.Key.Generation {
		delete(r.requests, result.Key)
		r.protocolOwned = false
		if poison {
			r.record.State = session.Poisoned
		}
		m.observe(id, result.Key.Generation, "response", r.record.State, failure)
	}
	result.Failure = failure
	result.Duration = m.now().Sub(result.started)
	if result.Duration < 0 {
		result.Duration = 0
	}
	m.workers--
	select {
	case m.workerDone <- struct{}{}:
	default:
	}
	return result
}

type runtimeSession struct {
	record         Record
	process        Child
	spec           managedprocess.Spec
	pending        *lspwire.Pending
	requests       map[lspwire.RequestKey]*Request
	cancels        int
	protocolOwned  bool
	lifecycleOwned bool
	metadata       SessionMetadata
}

type Manager struct {
	mu               sync.Mutex
	limits           Limits
	wire             lspwire.Limits
	starter          Starter
	algebra          *session.Manager
	sessions         map[string]*runtimeSession
	operations       map[string]OperationSnapshot
	operationIDs     []string
	readiness        map[string]*readinessOperation
	readinessIDs     map[string]string
	observations     []Observation
	sequence         uint64
	readinessSeq     uint64
	readinessTimeout time.Duration
	now              func() time.Time
	workers          int
	workerDone       chan struct{}
	closed           bool
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
	readinessTimeout := c.ReadinessTimeout
	if readinessTimeout <= 0 {
		readinessTimeout = time.Second
	}
	now := c.Now
	if now == nil {
		now = time.Now
	}
	return &Manager{limits: l, wire: c.Wire, starter: c.Starter, algebra: a, sessions: make(map[string]*runtimeSession), operations: make(map[string]OperationSnapshot), readiness: make(map[string]*readinessOperation), readinessIDs: make(map[string]string), readinessTimeout: readinessTimeout, now: now, workerDone: make(chan struct{}, 1)}, nil
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

func (m *Manager) BeginReadiness(ctx context.Context, id string, generation uint64, deadline time.Time) ReadinessSnapshot {
	m.mu.Lock()
	r := m.sessions[id]
	if r == nil {
		m.mu.Unlock()
		return ReadinessSnapshot{SessionID: id, Generation: generation, State: ReadinessFailed, Failure: session.SessionNotFound}
	}
	if generation != r.record.Generation {
		m.mu.Unlock()
		return ReadinessSnapshot{SessionID: id, Generation: generation, State: ReadinessFailed, Failure: session.StaleGeneration}
	}
	if existing := m.readinessIDs[id+":"+strconv.FormatUint(generation, 10)]; existing != "" {
		snapshot := m.readiness[existing].snapshot
		m.mu.Unlock()
		return snapshot
	}
	if r.protocolOwned {
		m.mu.Unlock()
		return ReadinessSnapshot{SessionID: id, Generation: generation, State: ReadinessFailed, Failure: session.LifecycleConflict}
	}
	protocol, ok := r.process.(wireChild)
	if !ok {
		m.mu.Unlock()
		return ReadinessSnapshot{SessionID: id, Generation: generation, State: ReadinessFailed, Failure: session.InitializationFailure}
	}
	workspace := r.record.Profile.Workspace().String()
	m.readinessSeq++
	opID := "readiness-" + strconv.FormatUint(m.readinessSeq, 10)
	op := &readinessOperation{snapshot: ReadinessSnapshot{ID: opID, SessionID: id, Generation: generation, State: ReadinessPending, ThermalPhase: "COLD"}, started: m.now(), done: make(chan struct{})}
	m.readiness[opID] = op
	m.readinessIDs[id+":"+strconv.FormatUint(generation, 10)] = opID
	r.protocolOwned = true
	m.workers++
	m.mu.Unlock()
	go m.runReadiness(ctx, deadline, protocol, opID, generation, workspace)
	return op.snapshot
}

func (m *Manager) runReadiness(parent context.Context, deadline time.Time, child wireChild, opID string, generation uint64, workspace string) {
	ctx := parent
	cancel := func() {}
	if deadline.IsZero() {
		ctx, cancel = context.WithTimeout(parent, m.readinessTimeout)
	} else {
		ctx, cancel = context.WithDeadline(parent, deadline)
	}
	defer cancel()

	key := lspwire.NewPending(1).Begin(generation)
	id := strconv.FormatUint(key.ID, 10)
	writer := lspwire.NewWriter(child.Stdin(), m.wire)
	workspaceURI := (&url.URL{Scheme: "file", Path: workspace}).String()
	params, _ := json.Marshal(struct {
		ProcessID        any    `json:"processId"`
		RootURI          string `json:"rootUri"`
		WorkspaceFolders []struct {
			URI  string `json:"uri"`
			Name string `json:"name"`
		} `json:"workspaceFolders"`
		Capabilities struct{} `json:"capabilities"`
	}{ProcessID: nil, RootURI: workspaceURI, WorkspaceFolders: []struct {
		URI  string `json:"uri"`
		Name string `json:"name"`
	}{{URI: workspaceURI, Name: "workspace"}}})
	initializeMessage := lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(id), Method: "initialize", Params: params}
	initializeBody, _ := json.Marshal(initializeMessage)
	m.recordReadinessRequest(opID, int64(len(initializeBody)))
	if err := writer.Write(initializeMessage); err != nil {
		m.abortReadiness(child, opID, session.InitializationFailure)
		return
	}
	type readinessResult struct {
		metadata SessionMetadata
		err      error
	}
	response := make(chan readinessResult, 1)
	go func() {
		message, err := lspwire.NewReader(child.Stdout(), m.wire).Read()
		if err == nil {
			body, _ := json.Marshal(message)
			m.recordReadinessResponse(opID, int64(len(body)))
		}
		if err == nil && (message.Kind() != lspwire.KindSuccessResponse || string(message.ID) != id || message.Error != nil || len(message.Result) == 0) {
			err = errors.New("sessionruntime: invalid readiness response")
		}
		metadata := SessionMetadata{}
		if err == nil {
			var initialized struct {
				Capabilities struct {
					PositionEncoding      string          `json:"positionEncoding"`
					CallHierarchyProvider json.RawMessage `json:"callHierarchyProvider"`
				} `json:"capabilities"`
			}
			if decodeErr := json.Unmarshal(message.Result, &initialized); decodeErr != nil {
				err = errors.New("sessionruntime: malformed initialize result")
			} else {
				metadata.PositionEncoding = initialized.Capabilities.PositionEncoding
				if metadata.PositionEncoding == "" {
					metadata.PositionEncoding = "utf-16"
				}
				provider := initialized.Capabilities.CallHierarchyProvider
				metadata.CallHierarchySupport = string(provider) == "true" || (len(provider) > 0 && string(provider) != "false" && string(provider) != "null")
			}
		}
		response <- readinessResult{metadata: metadata, err: err}
	}()
	select {
	case observed := <-response:
		if observed.err != nil {
			m.abortReadiness(child, opID, session.InitializationFailure)
			return
		}
		initializedMessage := lspwire.Message{JSONRPC: lspwire.Version, Method: "initialized", Params: json.RawMessage(`{}`)}
		initializedBody, _ := json.Marshal(initializedMessage)
		m.recordReadinessRequest(opID, int64(len(initializedBody)))
		if err := writer.Write(initializedMessage); err != nil {
			m.abortReadiness(child, opID, session.InitializationFailure)
			return
		}
		m.finishReadiness(opID, ReadinessReady, "", observed.metadata)
	case <-ctx.Done():
		failure := session.RequestCancelled
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			failure = session.InitializationTimeout
		}
		m.abortReadiness(child, opID, failure)
	}
}

func (m *Manager) abortReadiness(child Child, id string, failure session.Failure) {
	_ = child.Teardown(context.Background())
	_ = child.Close()
	m.finishReadiness(id, ReadinessFailed, failure, SessionMetadata{})
}

func (m *Manager) recordReadinessRequest(id string, bytes int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if op := m.readiness[id]; op != nil && op.snapshot.State == ReadinessPending {
		op.snapshot.RequestMessages++
		op.snapshot.RequestBytes += bytes
	}
}

func (m *Manager) recordReadinessResponse(id string, bytes int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if op := m.readiness[id]; op != nil && op.snapshot.State == ReadinessPending {
		op.snapshot.ResponseMessages++
		op.snapshot.ResponseBytes += bytes
	}
}

func (m *Manager) finishReadiness(id string, state ReadinessState, failure session.Failure, metadata SessionMetadata) {
	m.mu.Lock()
	defer m.mu.Unlock()
	op := m.readiness[id]
	if op == nil || op.snapshot.State != ReadinessPending {
		return
	}
	op.snapshot.State, op.snapshot.Failure, op.snapshot.Metadata = state, failure, metadata
	op.snapshot.Duration = m.now().Sub(op.started)
	if op.snapshot.Duration < 0 {
		op.snapshot.Duration = 0
	}
	if r := m.sessions[op.snapshot.SessionID]; r != nil && r.record.Generation == op.snapshot.Generation {
		r.metadata = metadata
		r.protocolOwned = false
		if state == ReadinessReady {
			result := m.algebra.ObserveInitialization(op.snapshot.SessionID, op.snapshot.Generation, true)
			r.record.State = result.State
			m.observe(op.snapshot.SessionID, op.snapshot.Generation, "readiness", result.State, result.Failure)
		} else {
			result := m.algebra.ObserveInitialization(op.snapshot.SessionID, op.snapshot.Generation, false)
			r.record.State = result.State
			m.observe(op.snapshot.SessionID, op.snapshot.Generation, "readiness", result.State, failure)
		}
	}
	close(op.done)
	m.workers--
	select {
	case m.workerDone <- struct{}{}:
	default:
	}
}

func (m *Manager) Readiness(id string) (ReadinessSnapshot, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	op, ok := m.readiness[id]
	if !ok {
		return ReadinessSnapshot{}, false
	}
	return op.snapshot, true
}

func (m *Manager) WaitReadiness(ctx context.Context, id string) (ReadinessSnapshot, bool) {
	m.mu.Lock()
	op, ok := m.readiness[id]
	if !ok {
		m.mu.Unlock()
		return ReadinessSnapshot{}, false
	}
	done := op.done
	snapshot := op.snapshot
	m.mu.Unlock()
	if snapshot.State != ReadinessPending {
		return snapshot, true
	}
	select {
	case <-done:
		return m.Readiness(id)
	case <-ctx.Done():
		return snapshot, true
	}
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

func (m *Manager) CancelRequest(id string, key lspwire.RequestKey) (lspwire.CancelState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.sessions[id]
	if r == nil || key.Generation != r.record.Generation {
		return lspwire.CancelNotPending, nil
	}
	child, ok := r.process.(wireChild)
	if !ok {
		return lspwire.CancelNotPending, errors.New("sessionruntime: active child has no LSP wire")
	}
	state, err := r.pending.Cancel(lspwire.NewWriter(child.Stdin(), m.wire), key)
	if state == lspwire.CancelWritten {
		r.cancels++
		m.observe(id, key.Generation, "cancel", r.record.State, session.RequestCancelled)
	}
	return state, err
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
	if m.closed || (r.protocolOwned && !r.lifecycleOwned) {
		return session.LifecycleResult{State: r.record.State, Generation: r.record.Generation, Failure: session.LifecycleConflict}
	}
	if readinessID := m.readinessIDs[id+":"+strconv.FormatUint(r.record.Generation, 10)]; readinessID != "" {
		if readiness := m.readiness[readinessID]; readiness != nil && readiness.snapshot.State == ReadinessPending {
			return session.LifecycleResult{State: r.record.State, Generation: r.record.Generation, Failure: session.LifecycleConflict}
		}
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
	r.protocolOwned, r.lifecycleOwned = true, true
	m.observe(id, snapshot.Generation, kind, intent.State, "")
	go m.runLifecycle(snapshot, r.process, r.pending, r.spec)
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

func (m *Manager) runLifecycle(operation OperationSnapshot, child Child, pending *lspwire.Pending, spec managedprocess.Spec) {
	shutdownComplete := true
	if protocol, ok := child.(wireChild); ok {
		shutdownComplete = m.gracefulShutdown(protocol, pending, operation.Generation)
	}
	teardown := child.Teardown(context.Background())
	resources := child.Close()
	death := teardown.Death.Reap.Kind == managedprocess.ReapComplete
	observed := session.LifecycleCompletion{ShutdownComplete: shutdownComplete, UnsafeIOAbsent: resources.Kind == managedprocess.ResourcesClosed, TerminateSucceeded: death, DeathObserved: death, NoContainedSurvivors: death, StderrDrainComplete: true, Reaped: death, InitializationPending: true}

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
			r.protocolOwned, r.lifecycleOwned = false, false
		}
		m.finishOperation(operation)
		m.mu.Unlock()
		return
	}
	if !operation.Restart {
		if !m.algebra.ReleaseStopped(operation.SessionID, operation.Generation) {
			operation.State, operation.Failure = OperationFailed, session.LifecycleConflict
			if r != nil {
				r.record.State = session.Poisoned
				r.protocolOwned, r.lifecycleOwned = false, false
			}
			m.finishOperation(operation)
			m.mu.Unlock()
			return
		}
		delete(m.sessions, operation.SessionID)
		operation.State = OperationComplete
		m.finishOperation(operation)
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	next, start := m.starter.Start(context.Background(), spec)
	m.mu.Lock()
	if start.Kind != managedprocess.StartStarted || next == nil || r == nil {
		operation.State, operation.Failure = OperationFailed, session.SpawnFailure
		if r != nil {
			r.record.State = session.Poisoned
		}
		m.observe(operation.SessionID, completed.Generation, "poison", session.Poisoned, session.SpawnFailure)
		m.finishOperation(operation)
		m.mu.Unlock()
		return
	}
	r.process, r.record.Generation, r.record.State = next, completed.Generation, session.Initializing
	r.pending, r.requests, r.cancels, r.protocolOwned, r.lifecycleOwned = lspwire.NewPending(m.limits.MaxTombstones), make(map[lspwire.RequestKey]*Request), 0, false, false
	m.observe(operation.SessionID, completed.Generation, "startup", session.Starting, "")
	m.observe(operation.SessionID, completed.Generation, "initialization", session.Initializing, "")
	m.mu.Unlock()
	if _, ok := next.(wireChild); ok {
		readiness := m.BeginReadiness(context.Background(), operation.SessionID, completed.Generation, time.Time{})
		observed, _ := m.WaitReadiness(context.Background(), readiness.ID)
		if observed.State != ReadinessReady {
			operation.State, operation.Failure = OperationFailed, observed.Failure
		} else {
			operation.State = OperationComplete
		}
	} else {
		operation.State = OperationComplete
	}
	m.mu.Lock()
	m.finishOperation(operation)
	m.mu.Unlock()
}

func (m *Manager) gracefulShutdown(child wireChild, pending *lspwire.Pending, generation uint64) bool {
	key := pending.Begin(generation)
	id := strconv.FormatUint(key.ID, 10)
	writer := lspwire.NewWriter(child.Stdin(), m.wire)
	if err := writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(id), Method: "shutdown"}); err != nil {
		return false
	}
	response := make(chan error, 1)
	go func() {
		message, err := lspwire.NewReader(child.Stdout(), m.wire).Read()
		if err == nil && (message.Kind() != lspwire.KindSuccessResponse || string(message.ID) != id || pending.Accept(key) != lspwire.ResponseAccepted) {
			err = errors.New("sessionruntime: invalid shutdown response")
		}
		response <- err
	}()
	timer := time.NewTimer(250 * time.Millisecond)
	defer timer.Stop()
	select {
	case err := <-response:
		if err != nil {
			return false
		}
	case <-timer.C:
		return false
	}
	return writer.Write(lspwire.Message{JSONRPC: lspwire.Version, Method: "exit"}) == nil
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
func (m *Manager) Metadata(id string, generation uint64) (SessionMetadata, session.Failure) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.sessions[id]
	if r == nil {
		return SessionMetadata{}, session.SessionNotFound
	}
	if generation != r.record.Generation {
		return SessionMetadata{}, session.StaleGeneration
	}
	if r.record.State != session.Ready || r.protocolOwned {
		return SessionMetadata{}, session.LifecycleConflict
	}
	return r.metadata, ""
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
