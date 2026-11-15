package sessionruntime

// Private discovery is a separate, target-free resource enclosure. The limits
// below are declarations, not funding or native evidence. Its only output is
// neutral census metadata; none of these values can construct native custody.
import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"sync"
	"time"

	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/session"
)

var ErrPrivateDiscovery = errors.New("private discovery enclosure refused")

type PrivateDiscoveryLimits struct {
	SourceBytes, RequestBytes, ResponseBytes, WireBytes, RetainedBytes, Work uint64
	Messages, Requests                                                       uint64
	FrameBytes, HeaderBytes                                                  uint64
}

func (l PrivateDiscoveryLimits) Valid() bool {
	return l.SourceBytes > 1 && l.SourceBytes <= 16*(MaxDocumentSupplyBytes+1) &&
		l.RequestBytes > 0 && l.RequestBytes <= 64<<20 && l.ResponseBytes > 0 && l.ResponseBytes <= 64<<20 &&
		l.WireBytes > 0 && l.WireBytes <= 128<<20 && l.RetainedBytes > 0 && l.RetainedBytes <= 512<<20 &&
		l.Work > 0 && l.Work <= 1<<32 && l.Messages > 0 && l.Messages <= 4096 && l.Requests > 0 && l.Requests <= 1024 &&
		l.FrameBytes > 0 && l.FrameBytes <= 4<<20 && l.HeaderBytes > 0 && l.HeaderBytes <= 16<<10
}

type PrivateDiscoveryConstruction struct {
	// This factory is invoked only inside the already reserved operation. It
	// constructs a process starter, never target/native authority.
	NewStarter func() (Starter, error)
	Start      StartRequest
	Limits     PrivateDiscoveryLimits
	Deadline   time.Time
	Members    []PrivateDiscoveryMember
}
type PrivateDiscoveryMember struct{ Path, URI, LanguageID string }
type PrivateDiscoveryObservation struct {
	CleanupJoined bool
	Failed        bool
}

type privateDiscoveryContextKey struct{}
type privateDiscoveryWindow struct {
	mu                sync.Mutex
	ctx               context.Context
	limits, remaining PrivateDiscoveryLimits
	failed            bool
	refusal           PrivateDiscoveryRefusal
	manager           *Manager
	child             *privateDiscoveryChild
	backendCtx        context.Context
	expectedSpec      managedprocess.Spec
	backendOwner      *managedprocess.Manager
	backendProfile    managedprocess.PrivateProviderWindowProfile
	backendPaid       bool
}

// Pay the entire nonrefundable backend pool, not retained bytes observed later.
// Census shares one 4096-byte pool across two invocations; scratch/parser/member bookkeeping
// has an additional fixed retained/work allowance. No new currency is created.
func (w *privateDiscoveryWindow) prepayBackend() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.backendPaid || w.failed || w.ctx.Err() != nil {
		w.failLocked(PrivateDiscoveryRefusal{Stage: PrivateDiscoveryBackend})
		return false
	}
	raw := uint64(managedprocess.PrivateProviderStderrReadLimit + managedprocess.PrivateProviderCensusReadLimit)
	retained := uint64(managedprocess.PrivateProviderStderrReadLimit + managedprocess.PrivateProviderStderrScratch + 16384)
	work := 32*(raw+retained) + 65536
	if !w.reserveLocked(PrivateDiscoveryBackend, privateDiscoveryCharge{&w.remaining.ResponseBytes, raw}, privateDiscoveryCharge{&w.remaining.WireBytes, raw}, privateDiscoveryCharge{&w.remaining.RetainedBytes, retained}, privateDiscoveryCharge{&w.remaining.Work, work}) {
		return false
	}
	w.backendPaid = true
	return true
}

// bindManaged is reached from the actual concrete ManagedStarter before its
// Manager.Start. One context belongs to the complete discovery lifetime; retries
// may inspect it but cannot install a fresh residual or change owner/spec/time.
func (w *privateDiscoveryWindow) bindManaged(ctx context.Context, m *managedprocess.Manager, spec managedprocess.Spec) (context.Context, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	d, ok := w.ctx.Deadline()
	if !ok || ctx.Err() != nil || w.ctx.Err() != nil || w.failed || !w.backendPaid || !reflect.DeepEqual(spec, w.expectedSpec) {
		w.failLocked(PrivateDiscoveryRefusal{Stage: PrivateDiscoveryUnknownStage})
		return nil, ErrPrivateDiscovery
	}
	p := m.PrivateProviderWindowProfile()
	if !p.Available || !p.LocalDarwin || p.StderrReadLimit != managedprocess.PrivateProviderStderrReadLimit || p.StderrScratch != managedprocess.PrivateProviderStderrScratch || p.CensusReadLimit != managedprocess.PrivateProviderCensusReadLimit || p.StderrRetention < 0 || p.StderrRetention > managedprocess.PrivateProviderStderrReadLimit {
		w.failLocked(PrivateDiscoveryRefusal{Stage: PrivateDiscoveryUnknownStage})
		return nil, ErrPrivateDiscovery
	}
	if w.backendOwner != nil {
		if w.backendOwner != m || w.backendProfile != p {
			w.failLocked(PrivateDiscoveryRefusal{Stage: PrivateDiscoveryUnknownStage})
			return nil, ErrPrivateDiscovery
		}
		return w.backendCtx, nil
	}
	bound, e := m.PrivateProviderWindowContext(w.ctx, p, d)
	if e != nil {
		w.failLocked(PrivateDiscoveryRefusal{Stage: PrivateDiscoveryUnknownStage})
		return nil, ErrPrivateDiscovery
	}
	w.backendOwner = m
	w.backendProfile = p
	w.backendCtx = bound
	return bound, nil
}
func privateDiscoveryFrom(ctx context.Context) *privateDiscoveryWindow {
	if ctx == nil {
		return nil
	}
	w, _ := ctx.Value(privateDiscoveryContextKey{}).(*privateDiscoveryWindow)
	return w
}
func (w *privateDiscoveryWindow) spend(field *uint64, n uint64) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.reserveLocked(PrivateDiscoveryAccounting, privateDiscoveryCharge{field, n})
}
func (w *privateDiscoveryWindow) work(n uint64) bool { return w.spend(&w.remaining.Work, n) }
func (w *privateDiscoveryWindow) sourceLimit() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.failed || w.ctx.Err() != nil {
		w.failLocked(PrivateDiscoveryRefusal{Stage: PrivateDiscoverySource})
		return 0
	}
	if w.remaining.SourceBytes <= 1 {
		w.failLocked(PrivateDiscoveryRefusal{Stage: PrivateDiscoverySource, Currency: PrivateDiscoverySourceBytes, Requested: 2, Remaining: w.remaining.SourceBytes, AmountsAvailable: true})
		return 0
	}
	n := w.remaining.SourceBytes - 1
	if n > MaxDocumentSupplyBytes {
		n = MaxDocumentSupplyBytes
	}
	// n is capped by MaxDocumentSupplyBytes before multiplication. Reserve all
	// currencies, including the rejecting byte, before counters or source opens.
	cost := 32*(n+1) + 65536
	if !w.reserveLocked(PrivateDiscoverySource, privateDiscoveryCharge{&w.remaining.SourceBytes, n + 1}, privateDiscoveryCharge{&w.remaining.Work, cost}, privateDiscoveryCharge{&w.remaining.RetainedBytes, 16 * (n + 1)}) {
		return 0
	}
	return int(n)
}
func (w *privateDiscoveryWindow) quoteRequest(params int) bool {
	if params < 0 || params > 64<<10 {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.reserveLocked(PrivateDiscoveryAccounting, privateDiscoveryCharge{&w.remaining.Requests, 1}, privateDiscoveryCharge{&w.remaining.Work, 16 * (uint64(params) + 4096)})
}
func (w *privateDiscoveryWindow) frame(c wireChild) (lspwire.Message, lspwire.ReadFrameObservation, []byte, bool, error) {
	dc, ok := c.(*privateDiscoveryChild)
	if !ok {
		w.mu.Lock()
		w.failLocked(PrivateDiscoveryRefusal{Stage: PrivateDiscoveryFrame})
		w.mu.Unlock()
		return lspwire.Message{}, lspwire.ReadFrameObservation{}, nil, false, ErrPrivateDiscovery
	}
	f, err := w.frameRecord(dc)
	if err != nil {
		return lspwire.Message{}, lspwire.ReadFrameObservation{}, nil, false, err
	}
	return f.message, f.observation, f.raw, true, nil
}

type privateDiscoveryStarter struct {
	next   Starter
	window *privateDiscoveryWindow
}

func (s privateDiscoveryStarter) Start(ctx context.Context, spec managedprocess.Spec) (Child, managedprocess.StartObservation) {
	if s.window.ctx.Err() != nil || !s.window.work(65536) {
		return nil, managedprocess.StartObservation{Kind: managedprocess.StartUnavailable, Reason: "private discovery refused"}
	}
	c, o := s.next.Start(s.window.ctx, spec)
	if c != nil {
		wc, ok := c.(wireChild)
		if !ok {
			c.Teardown(s.window.ctx)
			c.Close()
			return nil, managedprocess.StartObservation{Kind: managedprocess.StartUnavailable, Reason: "private discovery wire unavailable"}
		}
		d := &privateDiscoveryChild{wireChild: wc, window: s.window}
		s.window.mu.Lock()
		s.window.child = d
		s.window.mu.Unlock()
		return d, o
	}
	return c, o
}

type privateDiscoveryChild struct {
	wireChild
	window                  *privateDiscoveryWindow
	observationMu           sync.Mutex
	teardownOnce, closeOnce sync.Once
	teardown                managedprocess.TeardownObservation
	resources               managedprocess.ResourceObservation
}

func (c *privateDiscoveryChild) Stdin() io.WriteCloser {
	return privateDiscoveryWriter{c.wireChild.Stdin(), c.window}
}
func (c *privateDiscoveryChild) Stdout() io.ReadCloser {
	return privateDiscoveryReader{c.wireChild.Stdout(), c.window}
}
func (c *privateDiscoveryChild) Teardown(context.Context) managedprocess.TeardownObservation {
	c.teardownOnce.Do(func() {
		observed := c.wireChild.Teardown(c.window.ctx)
		c.observationMu.Lock()
		c.teardown = observed
		c.observationMu.Unlock()
	})
	c.observationMu.Lock()
	defer c.observationMu.Unlock()
	return c.teardown
}
func (c *privateDiscoveryChild) Close() managedprocess.ResourceObservation {
	c.closeOnce.Do(func() {
		observed := c.wireChild.Close()
		c.observationMu.Lock()
		c.resources = observed
		c.observationMu.Unlock()
	})
	c.observationMu.Lock()
	defer c.observationMu.Unlock()
	return c.resources
}
func (c *privateDiscoveryChild) cleanupJoined() bool {
	c.observationMu.Lock()
	defer c.observationMu.Unlock()
	return c.teardown.Death.Reap.Kind == managedprocess.ReapComplete && c.resources.Kind == managedprocess.ResourcesClosed && c.teardown.Census.Bounded && !c.teardown.Census.Truncated && c.teardown.Census.Err == nil && c.teardown.Census.Members == 0 && c.teardown.Death.StderrDelivery.Err == nil
}
func (c *privateDiscoveryChild) Observe() managedprocess.SurvivorObservation {
	if o, ok := c.wireChild.(interface {
		Observe() managedprocess.SurvivorObservation
	}); ok {
		return o.Observe()
	}
	return managedprocess.SurvivorObservation{Kind: managedprocess.SurvivorRunning}
}
func (c *privateDiscoveryChild) ReconcileReap(ctx context.Context) managedprocess.TeardownObservation {
	if r, ok := c.wireChild.(reapReconciler); ok {
		return r.ReconcileReap(c.window.ctx)
	}
	return managedprocess.TeardownObservation{}
}

type privateDiscoveryWriter struct {
	io.WriteCloser
	w *privateDiscoveryWindow
}

func (x privateDiscoveryWriter) Write(p []byte) (int, error) {
	n := uint64(len(p))
	x.w.mu.Lock()
	if !x.w.reserveLocked(PrivateDiscoveryWrite, privateDiscoveryCharge{&x.w.remaining.Messages, 1}, privateDiscoveryCharge{&x.w.remaining.RequestBytes, n}, privateDiscoveryCharge{&x.w.remaining.WireBytes, n}) {
		x.w.mu.Unlock()
		return 0, ErrPrivateDiscovery
	}
	x.w.mu.Unlock()
	return x.WriteCloser.Write(p)
}

type privateDiscoveryReader struct {
	io.ReadCloser
	w *privateDiscoveryWindow
}

func (x privateDiscoveryReader) Read(p []byte) (int, error) {
	x.w.mu.Lock()
	if x.w.failed || x.w.ctx.Err() != nil {
		x.w.failLocked(PrivateDiscoveryRefusal{Stage: PrivateDiscoveryRead})
		x.w.mu.Unlock()
		return 0, ErrPrivateDiscovery
	}
	n := min(uint64(len(p)), x.w.remaining.ResponseBytes, x.w.remaining.WireBytes)
	if n == 0 {
		field := &x.w.remaining.ResponseBytes
		if x.w.remaining.WireBytes == 0 {
			field = &x.w.remaining.WireBytes
		}
		if len(p) == 0 {
			x.w.failLocked(PrivateDiscoveryRefusal{Stage: PrivateDiscoveryRead})
		} else {
			x.w.reserveLocked(PrivateDiscoveryRead, privateDiscoveryCharge{field, 1})
		}
		x.w.mu.Unlock()
		return 0, ErrPrivateDiscovery
	}
	// Reserve before the syscall, not after observing the length. Short reads do
	// not refund logical currency. The caller never receives unadmitted bytes.
	x.w.remaining.ResponseBytes -= n
	x.w.remaining.WireBytes -= n
	x.w.mu.Unlock()
	return x.ReadCloser.Read(p[:int(n)])
}

// RunPrivateDiscovery constructs and owns the entire Manager lifetime. Callers
// must hold a confirmed kernel reservation before calling this concrete runner.
// The runner itself creates no funding, kernel records, native proof or grants.
func RunPrivateDiscovery(parent context.Context, c PrivateDiscoveryConstruction, consume func(context.Context, *PrivateDiscoverySession) error) (observation PrivateDiscoveryObservation, err error) {
	observation.Failed = true
	if parent == nil || consume == nil || c.NewStarter == nil || !c.Limits.Valid() || len(c.Members) == 0 || len(c.Members) > 16 || c.Deadline.IsZero() || !time.Now().Before(c.Deadline) || time.Until(c.Deadline) > 15*time.Second || c.Start.SeedBinding != nil || len(c.Start.Profile.Workspace().String()) > 4096 {
		return observation, ErrPrivateDiscovery
	}
	for _, f := range c.Members {
		if len(f.URI) == 0 || len(f.URI) > 4096 || len(f.Path) > 4096 || len(f.LanguageID) > 64 {
			return observation, ErrPrivateDiscovery
		}
	}
	ctx, cancel := context.WithDeadline(parent, c.Deadline)
	defer cancel()
	w := &privateDiscoveryWindow{ctx: ctx, limits: c.Limits, remaining: c.Limits, expectedSpec: c.Start.Process}
	ctx = context.WithValue(ctx, privateDiscoveryContextKey{}, w)
	w.ctx = ctx
	defer func() {
		if err != nil {
			err = w.refusalErrorFrom(err, PrivateDiscoveryUnknownStage, false)
		}
	}()
	effectiveDeadline, _ := ctx.Deadline()
	// Fixed Manager/supervisor bookkeeping is reserved before either constructor.
	if !w.work(4<<20) || !w.spend(&w.remaining.RetainedBytes, 4<<20) || !w.prepayBackend() {
		return observation, ErrPrivateDiscovery
	}
	starter, e := c.NewStarter()
	if e != nil || starter == nil {
		return observation, ErrPrivateDiscovery
	}
	m, e := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 128, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 128, MaxObservations: 64}, Starter: privateDiscoveryStarter{starter, w}, Wire: lspwire.Limits{MaxBodyBytes: int64(c.Limits.FrameBytes), MaxHeaderBytes: int64(c.Limits.HeaderBytes)}})
	if e != nil {
		return observation, e
	}
	w.manager = m
	defer func() {
		joined := m.Shutdown(ctx) == nil && ctx.Err() == nil && m.Census().Workers == 0
		w.mu.Lock()
		failed := w.failed
		child := w.child
		w.mu.Unlock()
		joined = joined && (child == nil || child.cleanupJoined())
		observation = PrivateDiscoveryObservation{CleanupJoined: joined, Failed: err != nil || failed || !joined}
		if observation.Failed {
			fallback := PrivateDiscoveryUnknownStage
			if err == nil && !joined {
				fallback = PrivateDiscoveryCleanup
			}
			err = w.refusalErrorFrom(err, fallback, !joined)
		}
	}()
	started := m.Start(ctx, c.Start)
	if started.Failure != "" {
		return observation, ErrPrivateDiscovery
	}
	ready := m.BeginReadiness(ctx, started.SessionID, started.Generation, effectiveDeadline)
	ready, _ = m.WaitReadiness(ctx, ready.ID)
	if ready.State != ReadinessReady {
		return observation, ErrPrivateDiscovery
	}
	metadata, failure := m.Metadata(started.SessionID, started.Generation)
	if failure != "" {
		return observation, ErrPrivateDiscovery
	}
	surface := &PrivateDiscoverySession{client: privateDiscoveryClient{m: m, w: w, id: started.SessionID, generation: started.Generation, deadline: effectiveDeadline, metadata: metadata}, members: append([]PrivateDiscoveryMember(nil), c.Members...), supplied: make(map[string]bool)}
	err = surface.finalize(consume(ctx, surface))
	stop := m.Stop(ctx, started.SessionID, "private-discovery")
	if stop.Failure != "" {
		err = w.refusalErrorFrom(err, PrivateDiscoveryUnknownStage, false)
	}
	return observation, err
}

// PrivateDiscoverySession exposes no Manager, underlying transport, grouping,
// prepared handoff, native target or other proof-producing operation.
type PrivateDiscoverySession struct {
	client   privateDiscoveryClient
	members  []PrivateDiscoveryMember
	mu       sync.Mutex
	closed   bool
	supplied map[string]bool
}

func (s *PrivateDiscoverySession) admitURI(uri, language string, requireSupplied bool) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	for _, f := range s.members {
		if f.URI == uri && (language == "" || language == f.LanguageID) {
			return s.supplied[uri] == requireSupplied
		}
	}
	return false
}
func (s *PrivateDiscoverySession) isClosed() bool {
	if s == nil {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}
func (s *PrivateDiscoverySession) SessionID() string  { return s.client.id }
func (s *PrivateDiscoverySession) Generation() uint64 { return s.client.generation }
func (s *PrivateDiscoverySession) SupportsDocumentSymbols() bool {
	return !s.isClosed() && s.client.SupportsDocumentSymbols()
}
func (s *PrivateDiscoverySession) SupportsCallHierarchy() bool {
	return !s.isClosed() && s.client.SupportsCallHierarchy()
}
func (s *PrivateDiscoverySession) Supply(ctx context.Context, uri, language string) error {
	if !s.admitURI(uri, language, false) {
		return ErrPrivateDiscovery
	}
	r := s.client.m.PrepareDocument(s.client.w.ctx, DocumentRequest{SessionID: s.client.id, Generation: s.client.generation, URI: uri, LanguageID: language, CaptureSupply: true, PreparedNoFollow: true, PrivateSourceByteLimit: MaxDocumentSupplyBytes})
	if r.Failure != "" || r.Supply == nil {
		return ErrPrivateDiscovery
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrPrivateDiscovery
	}
	s.supplied[uri] = true
	return nil
}
func (s *PrivateDiscoverySession) DocumentSymbols(ctx context.Context, p lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error) {
	if !s.admitURI(p.TextDocument.URI, "", true) {
		return nil, ErrPrivateDiscovery
	}
	return s.client.DocumentSymbols(s.client.w.ctx, p)
}
func (s *PrivateDiscoverySession) PrepareCallHierarchy(ctx context.Context, p lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	if !s.admitURI(p.TextDocument.URI, "", true) {
		return nil, ErrPrivateDiscovery
	}
	return s.client.PrepareCallHierarchy(s.client.w.ctx, p)
}

type privateDiscoveryClient struct {
	m          *Manager
	w          *privateDiscoveryWindow
	id         string
	generation uint64
	deadline   time.Time
	metadata   SessionMetadata
}

func (c privateDiscoveryClient) SupportsDocumentSymbols() bool {
	return c.metadata.DocumentSymbolSupport
}
func (c privateDiscoveryClient) SupportsCallHierarchy() bool { return c.metadata.CallHierarchySupport }
func (c privateDiscoveryClient) call(ctx context.Context, method string, p any, out any) error {
	// Both closed parameter types have bounded URI/position values. The census
	// adapter already bounds retained symbol strings; quote the maximal JSON form.
	var uri string
	switch value := p.(type) {
	case lsp.DocumentSymbolParams:
		uri = value.TextDocument.URI
	case lsp.PrepareCallHierarchyParams:
		uri = value.TextDocument.URI
	default:
		return ErrPrivateDiscovery
	}
	if len(uri) == 0 || len(uri) > 4096 || !c.w.work(64<<10) {
		return ErrPrivateDiscovery
	}
	raw, e := json.Marshal(p)
	if e != nil || len(raw) > 64<<10 {
		return ErrPrivateDiscovery
	}
	c.w.mu.Lock()
	b := min(c.w.remaining.ResponseBytes, c.w.limits.FrameBytes)
	n := c.w.remaining.Messages
	c.w.mu.Unlock()
	if b == 0 || n == 0 {
		return ErrPrivateDiscovery
	}
	r := c.m.RoundTrip(ctx, RoundTripRequest{SessionID: c.id, Generation: c.generation, Method: method, Params: raw, Deadline: c.deadline, MaxBytes: int64(b), MaxMessages: int(n)})
	if r.Failure != session.Failure("") || r.ServerError != nil {
		return ErrPrivateDiscovery
	}
	if !c.w.work(64*uint64(len(r.Result))) || !c.w.spend(&c.w.remaining.RetainedBytes, 64*uint64(len(r.Result))) {
		return ErrPrivateDiscovery
	}
	return json.Unmarshal(r.Result, out)
}
func (c privateDiscoveryClient) DocumentSymbols(ctx context.Context, p lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error) {
	var out []lsp.DocumentSymbol
	e := c.call(ctx, "textDocument/documentSymbol", p, &out)
	return out, e
}
func (c privateDiscoveryClient) PrepareCallHierarchy(ctx context.Context, p lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	var out []lsp.CallHierarchyItem
	e := c.call(ctx, "textDocument/prepareCallHierarchy", p, &out)
	return out, e
}
