package sessionruntime

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"reflect"
	"slices"
	"strconv"
	"sync"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/session"
)

var ErrPrivateProviderWindow = errors.New("sessionruntime: private provider window refused")
var ErrPrivateProviderWindowUnavailable = errors.New("sessionruntime: private provider window backend unavailable")
var ErrPrivateProviderWindowUnjoined = errors.New("sessionruntime: FAILEDUNJOINED")

const privateProviderWindowUnjoined session.Failure = "FAILEDUNJOINED"

const (
	privateProviderWindowControlMessages      = 4
	privateProviderWindowNotificationMessages = 4
	privateProviderWindowTransactionFrames    = privateProviderWindowControlMessages + privateProviderWindowNotificationMessages
)

// These are logical I/O bounds, never grants or physical resource guarantees.
type PrivateProviderWindowPlan struct{ Calls, References, Definitions bool }

// FramesPerExchange is the transaction aggregate; legacy phases remain capped at four.
// Frames is the unchanged shared global budget, not FramesPerExchange times exchanges.
type PrivateProviderWindowBounds struct {
	SourceBytes, SourceReadBytes, SourcePreparations                                uint64
	HeaderBytes, BodyBytes, FrameBytes, Frames, FramesPerExchange                   uint64
	RequestBytes, ResponseBytes, WireBytes                                          uint64
	StderrDeliveryBytes, CensusBytes, CensusLines, CensusMembers, CensusInvocations uint64
}
type PrivateProviderWindowEnvelope struct{ spec *privateProviderWindowSpec }
type privateProviderWindowSpec struct {
	manager    *Manager
	limits     Limits
	configured lspwire.Limits
	readiness  time.Duration
	start      StartRequest
	plan       PrivateProviderWindowPlan
	backend    managedprocess.PrivateProviderWindowProfile
	controlled Starter
	wire       lspwire.Limits
	bounds     PrivateProviderWindowBounds
	attempt    manageddiagnostic.StartupAttemptID
	sequence   uint64
	started    time.Time
}

// A package-owned test adapter may implement this unexported method. External
// custom Starters cannot claim coverage by implementing an exported assertion.
type privateProviderWindowControlledBackend interface{ privateProviderWindowControlledBackend() }

func (e PrivateProviderWindowEnvelope) Bounds() PrivateProviderWindowBounds {
	if e.spec == nil {
		return PrivateProviderWindowBounds{}
	}
	return e.spec.bounds
}
func (m *Manager) PrivateProviderWindowEnvelope(start StartRequest, plan PrivateProviderWindowPlan) (PrivateProviderWindowEnvelope, error) {
	if m == nil || (!plan.Calls && !plan.References && !plan.Definitions) || start.SeedBinding != nil || start.Deadline.IsZero() || !time.Now().Before(start.Deadline) {
		return PrivateProviderWindowEnvelope{}, ErrPrivateProviderWindow
	}
	fields := []string{start.Profile.Workspace().String(), start.Profile.ProfileName(), start.LanguageID, start.BootstrapAlias, start.Process.Path, start.Process.Dir}
	fields = append(fields, start.Process.Args...)
	fields = append(fields, start.Process.Env...)
	fields = append(fields, start.RelationProviders...)
	if len(fields) > 128 {
		return PrivateProviderWindowEnvelope{}, ErrPrivateProviderWindow
	}
	total := 0
	for _, v := range fields {
		if len(v) > 4096 {
			return PrivateProviderWindowEnvelope{}, ErrPrivateProviderWindow
		}
		total += len(v)
	}
	if total > 64<<10 {
		return PrivateProviderWindowEnvelope{}, ErrPrivateProviderWindow
	}
	m.mu.Lock()
	s := &privateProviderWindowSpec{manager: m, limits: m.limits, configured: m.wire, readiness: m.readinessTimeout, start: start, plan: plan}
	starter := m.starter
	m.mu.Unlock()
	if s.limits.MaxChildren < 2 {
		return PrivateProviderWindowEnvelope{}, ErrPrivateProviderWindowUnavailable
	}
	switch b := starter.(type) {
	case ManagedStarter:
		s.backend = b.Manager.PrivateProviderWindowProfile()
		if !s.backend.Available || !s.backend.LocalDarwin {
			return PrivateProviderWindowEnvelope{}, ErrPrivateProviderWindowUnavailable
		}
	case privateProviderWindowControlledBackend:
		s.controlled = starter
	default:
		return PrivateProviderWindowEnvelope{}, ErrPrivateProviderWindowUnavailable
	}
	s.start.Process.Args = slices.Clone(start.Process.Args)
	s.start.Process.Env = slices.Clone(start.Process.Env)
	s.start.RelationProviders = slices.Clone(start.RelationProviders)
	s.wire = m.wire
	defaults := lspwire.DefaultLimits()
	if s.wire.MaxHeaderBytes <= 0 {
		s.wire.MaxHeaderBytes = defaults.MaxHeaderBytes
	}
	if s.wire.MaxBodyBytes <= 0 {
		s.wire.MaxBodyBytes = defaults.MaxBodyBytes
	}
	if s.wire.MaxHeaderBytes > 4096 {
		s.wire.MaxHeaderBytes = 4096
	}
	if s.wire.MaxBodyBytes > 128<<10 {
		s.wire.MaxBodyBytes = 128 << 10
	}
	exchanges := uint64(3)
	preparations := uint64(1) // initialize, documentSymbol, shutdown
	if plan.References {
		exchanges++
	}
	if plan.Definitions {
		exchanges++
	}
	if plan.Calls {
		exchanges += 3
		preparations++
	}
	frame := uint64(s.wire.MaxHeaderBytes + s.wire.MaxBodyBytes)
	s.bounds = PrivateProviderWindowBounds{SourceBytes: 16 << 10, SourceReadBytes: preparations * ((16 << 10) + 1), SourcePreparations: preparations, HeaderBytes: uint64(s.wire.MaxHeaderBytes), BodyBytes: uint64(s.wire.MaxBodyBytes), FrameBytes: frame, Frames: 4 * exchanges, FramesPerExchange: privateProviderWindowTransactionFrames, RequestBytes: (exchanges + preparations + 2) * (frame + 1), ResponseBytes: 4 * exchanges * (frame + 4096 + 1), StderrDeliveryBytes: 64 << 10, CensusBytes: 4096, CensusLines: 128, CensusMembers: 64, CensusInvocations: 2}
	s.bounds.WireBytes = s.bounds.RequestBytes + s.bounds.ResponseBytes
	s.attempt, s.sequence, s.started = m.beginStartupAttempt()
	return PrivateProviderWindowEnvelope{s}, nil
}
func (s *privateProviderWindowSpec) matches(m *Manager) bool {
	if s == nil || s.manager != m {
		return false
	}
	m.mu.Lock()
	same := m.limits == s.limits && m.wire == s.configured && m.readinessTimeout == s.readiness
	starter := m.starter
	m.mu.Unlock()
	if !same {
		return false
	}
	if s.controlled != nil {
		return starter == s.controlled
	}
	b, ok := starter.(ManagedStarter)
	return ok && b.Manager.PrivateProviderWindowProfile() == s.backend
}

type privateProviderWindowKey struct{}
type privateProviderWindowInnerKey struct{}
type privateProviderWindow struct {
	mu                                                                    sync.Mutex
	spec                                                                  *privateProviderWindowSpec
	ctx                                                                   context.Context
	cancel                                                                context.CancelFunc
	deadline                                                              time.Time
	remaining, reservedRead, reservedWrite, deliveredRead, deliveredWrite uint64
	exchangeFrames, frames, preparations                                  uint64
	schedule                                                              []string
	next                                                                  int
	active                                                                int
	changed                                                               chan struct{}
	expired, failed, started, finishing, released                         bool
	child                                                                 *privateProviderWindowChild
	ownedRecord                                                           *runtimeSession
	preparationFailed                                                     bool
	start                                                                 StartResult
	finishDone                                                            chan struct{}
	completion                                                            PrivateProviderWindowCompletion
	lifecycleJoined                                                       bool
}
type PrivateProviderWindowCompletion struct {
	Completed                                                                      bool
	SessionID                                                                      string
	Generation                                                                     uint64
	AttemptID                                                                      manageddiagnostic.StartupAttemptID
	ReservedReadBytes, ReservedWriteBytes, DeliveredReadBytes, DeliveredWriteBytes uint64
}

func privateProviderWindowFrom(ctx context.Context) *privateProviderWindow {
	if ctx == nil {
		return nil
	}
	w, _ := ctx.Value(privateProviderWindowKey{}).(*privateProviderWindow)
	return w
}
func privateProviderWindowInner(ctx context.Context) bool {
	v, _ := ctx.Value(privateProviderWindowInnerKey{}).(bool)
	return v
}
func (m *Manager) PrivateProviderWindowContext(ctx context.Context, e PrivateProviderWindowEnvelope, deadline time.Time) (context.Context, error) {
	if ctx == nil || ctx.Err() != nil || !e.spec.matches(m) || !deadline.Equal(e.spec.start.Deadline) || !time.Now().Before(deadline) {
		return nil, ErrPrivateProviderWindow
	}
	if d, ok := ctx.Deadline(); ok && !d.Equal(deadline) {
		return nil, ErrPrivateProviderWindow
	}
	if w := privateProviderWindowFrom(ctx); w != nil {
		if w.spec != e.spec || !w.deadline.Equal(deadline) {
			return nil, ErrPrivateProviderWindow
		}
		return ctx, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.privateProviderWindow != nil || m.workers >= m.limits.MaxChildren {
		return nil, ErrPrivateProviderWindow
	}
	bound, cancel := context.WithDeadline(ctx, deadline)
	w := &privateProviderWindow{spec: e.spec, deadline: deadline, cancel: cancel, remaining: e.Bounds().WireBytes, changed: make(chan struct{}), finishDone: make(chan struct{})}
	w.schedule = []string{"initialize", "initialized", "prepare", "textDocument/documentSymbol"}
	if e.spec.plan.References {
		w.schedule = append(w.schedule, "textDocument/references")
	}
	if e.spec.plan.Definitions {
		w.schedule = append(w.schedule, "textDocument/definition")
	}
	if e.spec.plan.Calls {
		w.schedule = append(w.schedule, "prepare", "textDocument/prepareCallHierarchy", "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls")
	}
	w.schedule = append(w.schedule, "shutdown", "exit")
	bound = context.WithValue(bound, privateProviderWindowKey{}, w)
	w.ctx = bound
	m.privateProviderWindow = w
	m.workers++ // reserved before Starter, retained until joined cleanup
	context.AfterFunc(bound, func() { w.requestFinish(m) })
	return bound, nil
}
func (w *privateProviderWindow) notifyLocked() { close(w.changed); w.changed = make(chan struct{}) }
func (w *privateProviderWindow) liveLocked() bool {
	if w.expired {
		return false
	}
	if w.ctx.Err() != nil || !time.Now().Before(w.deadline) {
		w.expired = true
		return false
	}
	return !w.failed
}
func (w *privateProviderWindow) enter(method string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.liveLocked() || w.next >= len(w.schedule) || w.schedule[w.next] != method {
		w.failed = true
		return false
	}
	w.next++
	w.exchangeFrames = 0
	if method == "prepare" {
		w.preparations++
		if w.preparations > w.spec.bounds.SourcePreparations {
			w.failed = true
			return false
		}
	}
	return true
}

// frame preserves the legacy initialize/readiness/shutdown four-frame limit.
func (w *privateProviderWindow) frame() bool {
	return w.frameBounded(privateProviderWindowControlMessages)
}

// transactionFrame admits a finite aggregate before decode reveals the class.
// Every admitted iteration is still charged to the original shared global budget.
func (w *privateProviderWindow) transactionFrame() bool {
	return w.frameBounded(privateProviderWindowTransactionFrames)
}

func (w *privateProviderWindow) frameBounded(limit uint64) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.liveLocked() || w.exchangeFrames >= limit || w.frames >= w.spec.bounds.Frames {
		w.failed = true
		return false
	}
	w.exchangeFrames++
	w.frames++
	return true
}
func (w *privateProviderWindow) reserve(n int, read bool) (int, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if n < 0 || !w.liveLocked() || w.remaining == 0 {
		return 0, false
	}
	if uint64(n) > w.remaining {
		if !read {
			w.failed = true
			return 0, false
		}
		n = int(w.remaining)
	}
	w.remaining -= uint64(n)
	if read {
		w.reservedRead += uint64(n)
	} else {
		w.reservedWrite += uint64(n)
	}
	return n, true
}
func (w *privateProviderWindow) delivered(n int, read bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if read {
		w.deliveredRead += uint64(n)
	} else {
		w.deliveredWrite += uint64(n)
	}
}

// PrivateLeaseBoundary and PrivateLeaseResult are closed diagnostic DATA, not
// capabilities. Constructed/unknown enum values confer no authority.
type PrivateLeaseBoundary uint8

const (
	PrivateLeaseBoundaryUnobserved PrivateLeaseBoundary = iota
	PrivateLeaseBoundaryNotApplicable
	PrivateLeaseBoundaryOwnerRejected
	PrivateLeaseBoundaryAdmissionRejected
	PrivateLeaseBoundaryContextDone
	PrivateLeaseBoundaryResultReplaced
	PrivateLeaseBoundaryResultReturned
)

func (b PrivateLeaseBoundary) String() string {
	switch b {
	case PrivateLeaseBoundaryUnobserved:
		return "UNOBSERVED"
	case PrivateLeaseBoundaryNotApplicable:
		return "NOT_APPLICABLE"
	case PrivateLeaseBoundaryOwnerRejected:
		return "OWNER_REJECTED"
	case PrivateLeaseBoundaryAdmissionRejected:
		return "ADMISSION_REJECTED"
	case PrivateLeaseBoundaryContextDone:
		return "CONTEXT_DONE"
	case PrivateLeaseBoundaryResultReplaced:
		return "RESULT_REPLACED"
	case PrivateLeaseBoundaryResultReturned:
		return "RESULT_RETURNED"
	default:
		return "UNKNOWN"
	}
}

type PrivateLeaseResult uint8

const (
	PrivateLeaseResultUnobserved PrivateLeaseResult = iota
	PrivateLeaseResultOK
	PrivateLeaseResultUnjoined
	PrivateLeaseResultConflict
	PrivateLeaseResultCanceled
	PrivateLeaseResultDeadline
	PrivateLeaseResultUnknown
)

func (r PrivateLeaseResult) String() string {
	switch r {
	case PrivateLeaseResultUnobserved:
		return "UNOBSERVED"
	case PrivateLeaseResultOK:
		return "OK"
	case PrivateLeaseResultUnjoined:
		return "KNOWN_UNJOINED"
	case PrivateLeaseResultConflict:
		return "KNOWN_CONFLICT"
	case PrivateLeaseResultCanceled:
		return "CANCELED"
	case PrivateLeaseResultDeadline:
		return "DEADLINE"
	default:
		return "UNKNOWN"
	}
}

func privateLeaseResult(err error) PrivateLeaseResult {
	// Direct fixed sentinels only: never call an opaque method or walk a graph.
	switch err {
	case nil:
		return PrivateLeaseResultOK
	case ErrPrivateProviderWindowUnjoined:
		return PrivateLeaseResultUnjoined
	case errSessionLeaseConflict:
		return PrivateLeaseResultConflict
	case context.Canceled:
		return PrivateLeaseResultCanceled
	case context.DeadlineExceeded:
		return PrivateLeaseResultDeadline
	default:
		return PrivateLeaseResultUnknown
	}
}

// PrivateLeaseBoundaryObservation freezes only the wrapper's return decision.
// It does NOT sample callback progress at the caller's later mutex acquisition.
// No retained manager/window state or worker can mutate this by-value record.
type PrivateLeaseBoundaryObservation struct {
	boundary PrivateLeaseBoundary
	observed bool
	inner    PrivateLeaseResult
}

func (o PrivateLeaseBoundaryObservation) Boundary() PrivateLeaseBoundary  { return o.boundary }
func (o PrivateLeaseBoundaryObservation) ReturnedResultObserved() bool    { return o.observed }
func (o PrivateLeaseBoundaryObservation) InnerResult() PrivateLeaseResult { return o.inner }

// Error-only specialization of privateProviderWindowRun. Admission, worker
// accounting/notify-before-delivery, cancellation and final liveness are identical.
// Only <-done observes an inner result. active==0 is never receipt evidence.
func privateProviderWindowLeaseRun(w *privateProviderWindow, ctx context.Context, work func(context.Context) error) (error, PrivateLeaseBoundaryObservation) {
	w.mu.Lock()
	if !w.liveLocked() || w.active != 0 || w.finishing {
		w.mu.Unlock()
		return ErrPrivateProviderWindowUnjoined, PrivateLeaseBoundaryObservation{boundary: PrivateLeaseBoundaryAdmissionRejected}
	}
	w.active++
	w.mu.Unlock()
	done := make(chan error, 1)
	go func() {
		result := work(context.WithValue(ctx, privateProviderWindowInnerKey{}, true))
		w.mu.Lock()
		w.active--
		w.notifyLocked()
		w.mu.Unlock()
		done <- result
	}()
	select {
	case <-ctx.Done():
		w.mu.Lock()
		w.expired = true
		w.mu.Unlock()
		w.cancel()
		return ErrPrivateProviderWindowUnjoined, PrivateLeaseBoundaryObservation{boundary: PrivateLeaseBoundaryContextDone}
	case result := <-done:
		w.mu.Lock()
		live := w.liveLocked()
		w.mu.Unlock()
		observation := PrivateLeaseBoundaryObservation{boundary: PrivateLeaseBoundaryResultReturned, observed: true, inner: privateLeaseResult(result)}
		if !live {
			observation.boundary = PrivateLeaseBoundaryResultReplaced
			return ErrPrivateProviderWindowUnjoined, observation
		}
		return result, observation
	}
}

// The callback's actual work remains tracked after the caller's select returns.
// No protocol, session lease or window worker is released by cancellation.
func privateProviderWindowRun[T any](w *privateProviderWindow, ctx context.Context, refused T, work func(context.Context) T) T {
	w.mu.Lock()
	if !w.liveLocked() || w.active != 0 || w.finishing {
		w.mu.Unlock()
		return refused
	}
	w.active++
	w.mu.Unlock()
	done := make(chan T, 1)
	go func() {
		result := work(context.WithValue(ctx, privateProviderWindowInnerKey{}, true))
		w.mu.Lock()
		w.active--
		w.notifyLocked()
		w.mu.Unlock()
		done <- result
	}()
	select {
	case <-ctx.Done():
		w.mu.Lock()
		w.expired = true
		w.mu.Unlock()
		w.cancel()
		return refused
	case result := <-done:
		w.mu.Lock()
		live := w.liveLocked()
		w.mu.Unlock()
		if !live {
			return refused
		}
		return result
	}
}
func (w *privateProviderWindow) startWindow(ctx context.Context, m *Manager, req StartRequest) StartResult {
	if !w.spec.matches(m) || !reflect.DeepEqual(req, w.spec.start) {
		return StartResult{Failure: session.LifecycleConflict}
	}
	w.mu.Lock()
	if w.started {
		w.mu.Unlock()
		return StartResult{Failure: session.LifecycleConflict}
	}
	w.started = true
	w.mu.Unlock()
	return privateProviderWindowRun(w, ctx, StartResult{Failure: privateProviderWindowUnjoined}, func(inner context.Context) StartResult {
		result := m.start(inner, req)
		w.mu.Lock()
		w.start = result
		w.notifyLocked()
		w.mu.Unlock()
		return result
	})
}
func (m *Manager) privateProviderWindowStartChild(ctx context.Context, spec managedprocess.Spec) (Child, managedprocess.StartObservation) {
	w := privateProviderWindowFrom(ctx)
	if w == nil {
		return m.starter.Start(ctx, spec)
	}
	if !w.spec.matches(m) || !reflect.DeepEqual(spec, w.spec.start.Process) {
		return nil, managedprocess.StartObservation{Kind: managedprocess.StartUnavailable}
	}
	backendCtx := ctx
	if b, ok := m.starter.(ManagedStarter); ok {
		var err error
		backendCtx, err = b.Manager.PrivateProviderWindowContext(ctx, w.spec.backend, w.deadline)
		if err != nil {
			return nil, managedprocess.StartObservation{Kind: managedprocess.StartUnavailable}
		}
	}
	child, observation := m.starter.Start(backendCtx, spec)
	if observation.Kind != managedprocess.StartStarted || child == nil {
		return child, observation
	}
	wire, ok := child.(wireChild)
	if !ok {
		_ = child.Teardown(ctx)
		_ = child.Close()
		return nil, managedprocess.StartObservation{Kind: managedprocess.StartFailed}
	}
	wrapped := &privateProviderWindowChild{wireChild: wire, window: w}
	w.mu.Lock()
	w.child = wrapped
	w.mu.Unlock()
	return wrapped, observation
}
func (m *Manager) privateProviderWindowWire(ctx context.Context) lspwire.Limits {
	if w := privateProviderWindowFrom(ctx); w != nil {
		return w.spec.wire
	}
	return m.wire
}

type privateProviderWindowChild struct {
	wireChild
	window                  *privateProviderWindow
	closeOnce, teardownOnce sync.Once
	resources               managedprocess.ResourceObservation
	teardown                managedprocess.TeardownObservation
}

func (c *privateProviderWindowChild) Stdin() io.WriteCloser {
	return privateProviderWindowWriter{c, c.wireChild.Stdin()}
}
func (c *privateProviderWindowChild) Stdout() io.ReadCloser {
	return privateProviderWindowReader{c, c.wireChild.Stdout()}
}
func (c *privateProviderWindowChild) Close() managedprocess.ResourceObservation {
	c.closeOnce.Do(func() { c.resources = c.wireChild.Close() })
	return c.resources
}
func (c *privateProviderWindowChild) Teardown(_ context.Context) managedprocess.TeardownObservation {
	c.teardownOnce.Do(func() { c.teardown = c.wireChild.Teardown(c.window.ctx) })
	return c.teardown
}
func (c *privateProviderWindowChild) cleanupJoined() bool {
	joined := c.teardown.Death.Reap.Kind == managedprocess.ReapComplete && c.resources.Kind == managedprocess.ResourcesClosed
	if c.window.spec.controlled == nil {
		joined = joined && c.teardown.Census.Bounded && !c.teardown.Census.Truncated && c.teardown.Census.Err == nil && c.teardown.Census.Members == 0 && c.teardown.Death.StderrDelivery.Err == nil
	}
	return joined
}
func (c *privateProviderWindowChild) ReconcileReap(ctx context.Context) managedprocess.TeardownObservation {
	if r, ok := c.wireChild.(reapReconciler); ok {
		return r.ReconcileReap(ctx)
	}
	return managedprocess.TeardownObservation{}
}
func (c *privateProviderWindowChild) Observe() managedprocess.SurvivorObservation {
	if r, ok := c.wireChild.(interface {
		Observe() managedprocess.SurvivorObservation
	}); ok {
		return r.Observe()
	}
	return managedprocess.SurvivorObservation{Kind: managedprocess.SurvivorRunning}
}

type privateProviderWindowWriter struct {
	c *privateProviderWindowChild
	io.WriteCloser
}

func (w privateProviderWindowWriter) Write(p []byte) (int, error) {
	n, ok := w.c.window.reserve(len(p), false)
	if !ok {
		return 0, ErrPrivateProviderWindow
	}
	got, err := w.WriteCloser.Write(p[:n])
	if got < 0 || got > n {
		return 0, ErrPrivateProviderWindow
	}
	w.c.window.delivered(got, false)
	if got != n && err == nil {
		err = io.ErrShortWrite
	}
	return got, err
}

type privateProviderWindowReader struct {
	c *privateProviderWindowChild
	io.ReadCloser
}

func (r privateProviderWindowReader) Read(p []byte) (int, error) {
	n, ok := r.c.window.reserve(len(p), true)
	if !ok {
		return 0, ErrPrivateProviderWindow
	}
	got, err := r.ReadCloser.Read(p[:n])
	if got < 0 || got > n {
		return 0, ErrPrivateProviderWindow
	}
	r.c.window.delivered(got, true)
	return got, err
}

func (m *Manager) FinishPrivateProviderWindow(ctx context.Context) (PrivateProviderWindowCompletion, error) {
	w := privateProviderWindowFrom(ctx)
	if w == nil || w.spec.manager != m {
		return PrivateProviderWindowCompletion{}, ErrPrivateProviderWindow
	}
	w.requestFinish(m)
	select {
	case <-ctx.Done():
		w.mu.Lock()
		w.expired = true
		w.mu.Unlock()
		w.cancel()
		return PrivateProviderWindowCompletion{}, ErrPrivateProviderWindowUnjoined
	case <-w.finishDone:
		w.mu.Lock()
		defer w.mu.Unlock()
		if !w.liveLocked() || !w.completion.Completed {
			return PrivateProviderWindowCompletion{}, ErrPrivateProviderWindowUnjoined
		}
		return w.completion, nil
	}
}
func (w *privateProviderWindow) requestFinish(m *Manager) {
	w.mu.Lock()
	if !w.finishing {
		w.finishing = true
		go m.finishPrivateProviderWindow(w)
	}
	w.mu.Unlock()
}

func (m *Manager) finishPrivateProviderWindow(w *privateProviderWindow) {
	// This goroutine is addressable through m.privateProviderWindow and owns the
	// worker reserved at activation. It only coordinates existing lifecycle work.
	w.mu.Lock()
	for w.active != 0 {
		changed := w.changed
		w.mu.Unlock()
		<-changed
		w.mu.Lock()
	}
	started := w.start
	ownedChild := w.child
	w.mu.Unlock()
	// Failed startup already joined its own cleanup callback (or retained its
	// failed child). It must not join/admit a second lifecycle for that record.
	if ownedChild != nil && started.Failure == "" && started.SessionID != "" && started.Generation != 0 {
		for {
			m.mu.Lock()
			r := m.sessions[started.SessionID]
			busy := r != nil && r.process == ownedChild && r.protocolOwned
			if busy && r.lifecycleOwned {
				busy = false
				for _, id := range m.operationIDs {
					op := m.operations[id]
					if op.SessionID == started.SessionID && op.Generation == started.Generation && op.State == OperationPending {
						busy = true
						break
					}
				}
			}
			m.mu.Unlock()
			if !busy {
				break
			}
			<-m.workerDone
		}
		m.mu.Lock()
		current := m.sessions[started.SessionID]
		exact := current != nil && current.process == ownedChild
		m.mu.Unlock()
		var intent session.LifecycleResult
		if exact {
			intent = m.Stop(w.ctx, started.SessionID, "private-provider-window")
		}
		if intent.IntentID != "" {
			ticker := time.NewTicker(time.Millisecond)
			defer ticker.Stop()
			for {
				op, ok := m.Operation(intent.IntentID)
				if !ok || op.State != OperationPending {
					break
				}
				<-ticker.C
			}
		}
	}
	w.mu.Lock()
	c := w.child
	joined := c == nil || c.cleanupJoined() && (w.lifecycleJoined || started.Generation == 0)
	w.completion = PrivateProviderWindowCompletion{Completed: false, SessionID: started.SessionID, Generation: started.Generation, AttemptID: started.AttemptID, ReservedReadBytes: w.reservedRead, ReservedWriteBytes: w.reservedWrite, DeliveredReadBytes: w.deliveredRead, DeliveredWriteBytes: w.deliveredWrite}
	if joined {
		w.released = true
	}
	w.mu.Unlock()
	if joined {
		m.mu.Lock()
		m.workers--
		select {
		case m.workerDone <- struct{}{}:
		default:
		}
		m.mu.Unlock()
	}
	w.mu.Lock()
	w.completion.Completed = joined && w.liveLocked() && !w.preparationFailed && w.next == len(w.schedule)
	w.mu.Unlock()
	close(w.finishDone)
}

func (m *Manager) runPrivateProviderWindowReadiness(parent context.Context, child *privateProviderWindowChild, opID string, generation uint64, workspace string) {
	w := child.window
	owner := &ownedTransport{child: child}
	failureKind := session.InitializationFailure
	var initializeBytes, initializedBytes int64
	fail := func() {
		owner.retire()
		m.retainReadinessRetirement(opID, child, owner.teardown, owner.resources)
		m.terminalReadinessEvent(opID, diagnosticEventTerminalFailure)
		if parent.Err() != nil {
			failureKind = privateProviderWindowUnjoined
		}
		m.finishReadiness(opID, ReadinessFailed, failureKind, SessionMetadata{})
	}
	write := func(msg lspwire.Message) bool {
		if !w.enter(msg.Method) {
			return false
		}
		body, err := json.Marshal(msg)
		if err != nil {
			return false
		}
		if msg.Method == "initialize" {
			initializeBytes = int64(len(body))
		} else {
			initializedBytes = int64(len(body))
		}
		m.recordReadinessRequest(opID, int64(len(body)))
		m.recordReadinessEvent(opID, diagnosticEventWriteAttempt, int64(len(body)), false)
		capture := newFramedWriteCaptureBounded(child.Stdin(), int64(w.spec.bounds.FrameBytes))
		err = owner.run(parent, func() error { return lspwire.NewWriterObserved(capture, w.spec.wire, m.cWireObserver).Write(msg) })
		if err != nil {
			return false
		}
		m.recordReadinessEvent(opID, diagnosticEventWriteComplete, int64(len(body)), true)
		frame, ok := capture.retainedFrame()
		return ok && m.appendReadinessHistoryFrame(opID, privateHistoryOutbound, frame) == nil
	}
	uri := (&url.URL{Scheme: "file", Path: workspace}).String()
	params, _ := json.Marshal(map[string]any{"processId": nil, "rootUri": uri, "workspaceFolders": []map[string]string{{"uri": uri, "name": "workspace"}}, "capabilities": map[string]any{"workspace": map[string]any{"symbol": map[string]bool{"dynamicRegistration": false}}, "textDocument": map[string]any{"callHierarchy": map[string]bool{"dynamicRegistration": false}, "documentSymbol": map[string]bool{"hierarchicalDocumentSymbolSupport": true}}}})
	if !write(lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage("1"), Method: "initialize", Params: params}) {
		fail()
		return
	}
	var message lspwire.Message
	var frame []byte
	err := owner.run(parent, func() error {
		for w.frame() {
			msg, _, raw, _, e := lspwire.ReadPrivateBoundedFrameObservedAtDecode(child.Stdout(), w.spec.wire, int64(w.spec.bounds.FrameBytes), int64(w.spec.bounds.FrameBytes), m.cWireObserver, m.cDecodeEntry, m.cRetainEntry)
			if e != nil {
				return e
			}
			body, _ := json.Marshal(msg)
			m.recordReadinessResponse(opID, int64(len(body)))
			m.recordReadinessEvent(opID, diagnosticEventReadComplete, int64(len(body)), true)
			if msg.Kind() == lspwire.KindNotification {
				continue
			}
			if msg.Kind() != lspwire.KindSuccessResponse || string(msg.ID) != "1" {
				return ErrPrivateProviderWindow
			}
			message, frame = msg, raw
			return nil
		}
		return ErrPrivateProviderWindow
	})
	if err != nil || m.appendReadinessHistoryFrame(opID, privateHistoryInbound, frame) != nil {
		fail()
		return
	}
	var result struct {
		Capabilities struct {
			PositionEncoding       string          `json:"positionEncoding"`
			CallHierarchyProvider  json.RawMessage `json:"callHierarchyProvider"`
			DocumentSymbolProvider json.RawMessage `json:"documentSymbolProvider"`
			DefinitionProvider     json.RawMessage `json:"definitionProvider"`
			ReferencesProvider     json.RawMessage `json:"referencesProvider"`
		} `json:"capabilities"`
		ServerInfo struct{ Name, Version string } `json:"serverInfo"`
	}
	if json.Unmarshal(message.Result, &result) != nil {
		fail()
		return
	}
	metadata := SessionMetadata{PositionEncoding: result.Capabilities.PositionEncoding, ProviderName: result.ServerInfo.Name, ProviderVersion: result.ServerInfo.Version, CallHierarchySupport: capabilitySupported(result.Capabilities.CallHierarchyProvider), DocumentSymbolSupport: capabilitySupported(result.Capabilities.DocumentSymbolProvider), DefinitionSupport: capabilitySupported(result.Capabilities.DefinitionProvider), ReferencesSupport: capabilitySupported(result.Capabilities.ReferencesProvider)}
	if metadata.PositionEncoding == "" {
		metadata.PositionEncoding = "utf-16"
	}
	if !metadata.DocumentSymbolSupport || w.spec.plan.Calls && !metadata.CallHierarchySupport || w.spec.plan.References && !metadata.ReferencesSupport || w.spec.plan.Definitions && !metadata.DefinitionSupport {
		failureKind = session.ToolNotImplemented
		fail()
		return
	}
	if !write(lspwire.Message{JSONRPC: lspwire.Version, Method: "initialized", Params: json.RawMessage(`{}`)}) {
		fail()
		return
	}
	m.recordSuccessfulReadiness(opID, initializeBytes, initializedBytes, metadata)
	m.describeReadinessDiagnostic(opID, metadata)
	m.terminalReadinessEvent(opID, diagnosticEventTerminalResponse)
	m.finishReadiness(opID, ReadinessReady, "", metadata)
}
func (m *Manager) privateProviderWindowShutdown(child *privateProviderWindowChild, pending *lspwire.Pending, generation uint64) shutdownObservation {
	w := child.window
	owner := &ownedTransport{child: child}
	observation := shutdownObservation{raw: &shutdownRawIOPrivateV2{}, facts: LifecycleCleanupObservationPrivateV2{ProcessIOPrivateV2Availability: "UNAVAILABLE", ResponseBudgetSource: "PRIVATE_PROVIDER_WINDOW", DeclaredResponseBudget: time.Until(w.deadline), ShutdownWrite: CleanupFailedPrivateV2}}
	if !w.enter("shutdown") {
		return observation
	}
	key := pending.Begin(generation)
	id := strconv.FormatUint(key.ID, 10)
	writer := lspwire.NewWriterObserved(child.Stdin(), w.spec.wire, m.cWireObserver)
	if owner.run(w.ctx, func() error {
		return writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(id), Method: "shutdown"})
	}) != nil {
		return observation
	}
	observation.facts.ShutdownWrite = CleanupSucceededPrivateV2
	err := owner.run(w.ctx, func() error {
		for w.frame() {
			msg, frame, _, _, e := lspwire.ReadPrivateBoundedFrameObservedAtDecode(child.Stdout(), w.spec.wire, int64(w.spec.bounds.FrameBytes), 0, m.cWireObserver, m.cDecodeEntry, m.cRetainEntry)
			if e != nil {
				return e
			}
			if msg.Kind() == lspwire.KindNotification {
				continue
			}
			observation.facts.ReadFrames++
			observation.facts.ReadBytes += frame.FrameBytes
			if msg.Kind() == lspwire.KindSuccessResponse && string(msg.ID) == id && pending.Accept(key) == lspwire.ResponseAccepted {
				observation.facts.ShutdownRead = ShutdownMatchedPrivateV2
				return nil
			}
			return ErrPrivateProviderWindow
		}
		return ErrPrivateProviderWindow
	})
	if err != nil {
		return observation
	}
	observation.acknowledged = true
	if w.enter("exit") && owner.run(w.ctx, func() error { return writer.Write(lspwire.Message{JSONRPC: lspwire.Version, Method: "exit"}) }) == nil {
		observation.exitNotified = true
		observation.facts.ExitWrite = CleanupSucceededPrivateV2
	}
	return observation
}
