//go:build darwin || linux

package sourcecontinuation

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	sourceclock "lsp-trace/internal/privatesourceclock"

	cc "lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/describeworker"
	host "lsp-trace/internal/privatedescribeprep"
)

// RootlessSourceHostStore is a closed, bounded, process-local owner. HOST entry
// does not support filesystem stores or independent process restart. The private
// concrete MemoryStore cannot be replaced by an interface, callback or embedding.
// Construction caps one object within the existing capture ceiling, not new credit.
type RootlessSourceHostStore struct {
	memory     *cc.MemoryStore
	max        int64
	gets, puts atomic.Uint64
	retained   atomic.Uint64 // cumulative stored byte lengths; duplicates overcharged
}

func NewRootlessSourceHostStore(maxObjectBytes int64) (*RootlessSourceHostStore, error) {
	if maxObjectBytes <= 0 || maxObjectBytes > cc.MaxCaptureBytes {
		return nil, errors.New("source host memory object ceiling")
	}
	return &RootlessSourceHostStore{memory: cc.NewMemoryStore(), max: maxObjectBytes}, nil
}
func (s *RootlessSourceHostStore) valid() bool {
	return s != nil && s.memory != nil && s.max > 0 && s.max <= cc.MaxCaptureBytes
}
func (s *RootlessSourceHostStore) MaxObjectBytes() int64 {
	if !s.valid() {
		return 0
	}
	return s.max
}
func (s *RootlessSourceHostStore) Put(ctx context.Context, b []byte) (string, error) {
	if !s.valid() || ctx == nil || int64(len(b)) > s.max {
		return "", errors.New("source host memory Put refused")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	for {
		old := s.retained.Load()
		if uint64(len(b)) > ^uint64(0)-old {
			return "", errors.New("source host stored retention overflow")
		}
		if s.retained.CompareAndSwap(old, old+uint64(len(b))) {
			break
		}
	}
	s.puts.Add(1)
	return s.memory.Put(ctx, b)
}
func (s *RootlessSourceHostStore) Get(ctx context.Context, id string) ([]byte, error) {
	if !s.valid() || ctx == nil || len(id) != 71 {
		return nil, errors.New("source host memory Get refused")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.gets.Add(1)
	b, err := s.memory.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > s.max {
		return nil, errors.New("source host memory object ceiling")
	}
	return b, nil
}

// OperationCounts is read-only observation, never eligibility or authority.
func (s *RootlessSourceHostStore) OperationCounts() (gets, puts uint64) {
	if s == nil {
		return 0, 0
	}
	return s.gets.Load(), s.puts.Load()
}

// SourceHostHandoff is minted only by the existing opaque source Resume join.
// A lower mailbox, its public frame, or JSON cannot mint this eligibility.
type SourceHostHandoff struct{ state *sourceHostState }
type sourceHostState struct {
	mu       sync.Mutex
	used     bool
	expected *expected
	request  SourceAcquisitionRequest
	source   host.SourceHostContext
	limits   describeworker.SuppliedResponseLimits
	extra    uint64
	mailbox  *host.SourceHostMailbox
}

// DeliveredSourceResponse preserves the adopted SUPPLIED-only binding. Its
// separate opaque receipt observes local file delivery, not model authentication.
type DeliveredSourceResponse struct {
	supplied         BoundSuppliedSourceResponse
	delivery         host.SourceHostDelivery
	terminalDeadline time.Time
}

// NativePayloadBounds observes closed-state lengths before any Bytes/Semantic
// accessor, hash or encoding. The bound canonical embeds the canonical semantic
// JSON, so its length also bounds decoded semantic strings/collection payload.
func (r DeliveredSourceResponse) NativePayloadBounds() (supplied, delivery, semantic uint64, err error) {
	if r.supplied.state == nil || !r.delivery.Delivered() {
		return 0, 0, 0, errors.New("source native delivery absent")
	}
	s := r.supplied.state
	supplied = uint64(len(s.canonical))
	if supplied == 0 || !s.bounds.allows(supplied) {
		return 0, 0, 0, errors.New("source native supplied bounds")
	}
	delivery, err = r.delivery.SerializedUpper()
	if err != nil {
		return 0, 0, 0, err
	}
	semantic = supplied
	if s.bounds.semantic < semantic {
		semantic = s.bounds.semantic
	}
	return supplied, delivery, semantic, nil
}

// NativeRetentionBounds is metadata-only and non-admitting. Original READY
// handoff/cache, source admissions/output, originals, host whole-frame buffers,
// root metadata and the rootless store are added, not treated as free headroom.
// The cache already includes canonical/decoded READY values and two source
// outputs. PreparedSourceRV1 is independently charged by the operator too.
func (out *PrivateAllModeSourcesResult) NativeRetentionBounds(ctx context.Context, c host.SourceHostConfig, q SourceAcquisitionRequest, limits describeworker.SuppliedResponseLimits) (retained, supplied, delivery, semantic uint64, err error) {
	bad := func() (uint64, uint64, uint64, uint64, error) {
		return 0, 0, 0, 0, errors.New("source native retention bounds")
	}
	if out == nil || out.Expected.state == nil || q.Resume.PrivateSources == nil || q.Resume.PrivateSources.Expected.state != out.Expected.state {
		return bad()
	}
	s, extra, e := sourceHostPreflight(ctx, c, q, limits)
	if e != nil {
		return 0, 0, 0, 0, e
	}
	store, ok := q.Resume.Store.(*RootlessSourceHostStore)
	if !ok || !store.valid() {
		return bad()
	}
	handoff, ok := s.handoff.RetainedPayloadUpper()
	if !ok {
		return bad()
	}
	retained, ok = sourceResponseAdd(handoff, s.regeneration.response, extra, store.retained.Load())
	if !ok {
		return bad()
	}
	record, ok := sourceResponseRecordUpper(q.Record)
	if !ok {
		return bad()
	}
	metadata, ok := sourceResponseStringBytes(s.handoff.ID(), s.checkpoint, q.Attempt.SourceUnitID, s.regeneration.digest, q.Resume.Workspace, q.Attempt.ID)
	if !ok {
		return bad()
	}
	b, e := sourceResponseAggregateBound(retained, record, metadata, uint64(limits.MaxAdapterBytes), uint64(limits.MaxSemanticBytes), uint64(s.contract.PacketMaxResponseBytes()), uint64(s.contract.CaptureLimits().MaxParentBytes))
	if e != nil {
		return 0, 0, 0, 0, e
	}
	// Delivery has no payload/base64: only a fixed receipt and binding strings.
	// The lower helper includes 6x every variable binding field and fixed hashes.
	delivery, e = host.SourceHostDeliveryUpper(c, hostSourceContext(s, q, s.regeneration.digest))
	if e != nil {
		return 0, 0, 0, 0, e
	}
	return retained, b.wire, delivery, b.semantic, nil
}

// SourceHostPhaseBounds is non-admitting sizing metadata, never source custody.
// Each peak includes the same simultaneously retained handoff/transport/store.
// The operator must independently add its prepared payload and root metadata.
// Keeping the phases separate makes equality/overflow checks inspectable.
type SourceHostPhaseBounds struct{ Source, Lower, Binder uint64 }

func (out *PrivateAllModeSourcesResult) HostPhaseBounds(ctx context.Context, c host.SourceHostConfig, q SourceAcquisitionRequest, limits describeworker.SuppliedResponseLimits) (SourceHostPhaseBounds, error) {
	bad := func() (SourceHostPhaseBounds, error) {
		return SourceHostPhaseBounds{}, errors.New("source host phase bounds")
	}
	if out == nil || out.Expected.state == nil || q.Resume.PrivateSources == nil || q.Resume.PrivateSources.Expected.state != out.Expected.state {
		return bad()
	}
	s, extra, err := sourceHostPreflight(ctx, c, q, limits)
	if err != nil {
		return SourceHostPhaseBounds{}, err
	}
	store, ok := q.Resume.Store.(*RootlessSourceHostStore)
	if !ok || !store.valid() {
		return bad()
	}
	handoff, ok := s.handoff.RetainedPayloadUpper()
	if !ok {
		return bad()
	}
	// Store bytes are independent of source cache bounds. Charge even duplicate
	// Puts cumulatively. Resume's lower/upper paths only Get existing materials;
	// they do not Put. The cached four-material peaks cover the freshly generated
	// materials and Get/verification copies; store retention is NOT their budget.
	// Shared original/prepared/cache payloads may be charged twice, never omitted.
	extra, ok = sourceResponseAdd(extra, store.retained.Load())
	if !ok {
		return bad()
	}
	parent := s.contract.CaptureLimits().MaxParentBytes
	if parent <= 0 || c.Mailbox.Limits.MaxResponseBytes <= 0 {
		return bad()
	}
	raw := uint64(c.Mailbox.Limits.MaxResponseBytes) // ORIGINAL ceiling, not delivered/adapter length
	source, ok := sourceAcquisitionEnvelopePeak(handoff, s.regeneration.source, s.regeneration.source, extra, raw, uint64(parent))
	if !ok {
		return bad()
	}
	lower, ok := sourceAcquisitionEnvelopePeak(handoff, s.regeneration.lower, s.regeneration.lower, extra, raw, uint64(parent))
	if !ok {
		return bad()
	}
	retained, ok := sourceResponseAdd(handoff, s.regeneration.response, extra)
	if !ok {
		return bad()
	}
	record, ok := sourceResponseRecordUpper(q.Record)
	if !ok {
		return bad()
	}
	metadata, ok := sourceResponseStringBytes(s.handoff.ID(), s.checkpoint, q.Attempt.SourceUnitID, s.regeneration.digest, q.Resume.Workspace, q.Attempt.ID)
	if !ok {
		return bad()
	}
	wire := s.contract.PacketMaxResponseBytes()
	if artifact := s.contract.CaptureLimits().MaxArtifactBytes; artifact < wire {
		wire = artifact
	}
	if wire <= 0 {
		return bad()
	}
	b, err := sourceResponseAggregateBound(retained, record, metadata, raw, uint64(limits.MaxSemanticBytes), uint64(wire), uint64(parent))
	if err != nil {
		return SourceHostPhaseBounds{}, err
	}
	return SourceHostPhaseBounds{Source: source, Lower: lower, Binder: b.peak}, nil
}

func (r DeliveredSourceResponse) Supplied() BoundSuppliedSourceResponse { return r.supplied }
func (r DeliveredSourceResponse) Delivery() host.SourceHostDelivery     { return r.delivery }
func (r DeliveredSourceResponse) TerminalDeadlineV2() (time.Time, bool) {
	return r.terminalDeadline, !r.terminalDeadline.IsZero()
}

// AfterClose is a non-admitting finalization: an error can only erase custody.
// CompleteSourceHost applies it to the actual first mailbox Close result before
// releasing its named return. Exposed for bounded deterministic failure witnesses;
// it cannot turn a zero value, DTO or failed exchange into a delivered response.
func (r DeliveredSourceResponse) AfterClose(err error) (DeliveredSourceResponse, error) {
	if err != nil {
		return DeliveredSourceResponse{}, err
	}
	return r, nil
}

// ArmLocalResponse is only non-admitting byte supply to this existing opaque
// handoff. It cannot manufacture expected state, admission, or a delivered proof.
func (h SourceHostHandoff) ArmLocalResponse(ctx context.Context, payload []byte) error {
	if h.state == nil {
		return errors.New("source host upper ownership absent")
	}
	s := h.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.used || len(payload) > s.limits.MaxAdapterBytes || !sourceHostPublicationLive(ctx, s.source.Deadline) {
		return errors.New("source host local payload refused")
	}
	return s.mailbox.ArmLocalResponse(ctx, payload)
}
func (h SourceHostHandoff) Deadline() time.Time {
	if h.state == nil {
		return time.Time{}
	}
	return h.state.mailbox.Deadline()
}
func (h SourceHostHandoff) Eligible() bool { return h.state != nil }
func (h SourceHostHandoff) Close() error {
	if h.state == nil {
		return nil
	}
	s := h.state
	s.mu.Lock()
	defer s.mu.Unlock()
	s.used = true
	return s.mailbox.Close()
}

func hostSourceContext(s *expected, q SourceAcquisitionRequest, sourceDigest string) host.SourceHostContext {
	return host.SourceHostContext{HandoffID: s.handoff.ID(), CheckpointID: q.Attempt.CheckpointID, SourceUnitID: q.Attempt.SourceUnitID, SourceDigest: sourceDigest, RequestRecordID: q.Attempt.RequestRecordID, AttemptID: q.Attempt.ID, Workspace: q.Resume.Workspace, Deadline: s.request.Deadline}
}

// Reserve transport AND the adopted binder's regeneration/parser/output envelope
// before Join/Resume/Get, marshaling, hashing, opening or publishing a mailbox.
// The limits are ceilings within the existing source contract, not new credit.
func sourceHostPreflight(ctx context.Context, c host.SourceHostConfig, q SourceAcquisitionRequest, limits describeworker.SuppliedResponseLimits) (*expected, uint64, error) {
	if q.Resume.PrivateSources != nil {
		for _, o := range q.Resume.PrivateSources.Originals {
			if _, ok := o.HOSTTimingV2(); ok {
				scope := o.HOSTRetainedContextV2()
				if scope == nil || scope.Err() != nil {
					return nil, 0, errors.New("source HOST original retained scope expired")
				}
			}
		}
	}
	// Closed admission precedes any Resume/Store Get, regeneration or mailbox IO.
	store, ok := q.Resume.Store.(*RootlessSourceHostStore)
	if !ok || !store.valid() {
		return nil, 0, errors.New("source host requires exact rootless process-local memory store; filesystem stores unsupported")
	}
	s, err := sourceAcquisitionExpected(ctx, q.Resume, q.Attempt)
	if err != nil {
		return nil, 0, err
	}
	if !limits.Valid() || limits.MaxAdapterBytes > s.contract.PacketMaxResponseBytes() || limits.MaxSemanticBytes > s.contract.PacketMaxResponseBytes() || limits.MaxAdapterBytes > c.Mailbox.Limits.MaxResponseBytes || c.Mailbox.ScopeSHA256 != s.handoff.ID() {
		return nil, 0, errors.New("source host limits/scope refused")
	}
	extra, err := host.SourceHostRetentionUpper(c, q.Record, hostSourceContext(s, q, s.regeneration.digest))
	if err != nil {
		return nil, 0, err
	}
	extra, ok = sourceResponseAdd(extra, 8192)
	if !ok {
		return nil, 0, errors.New("source host upper retention overflow")
	}
	retained, err := sourceAcquisitionEarlyEnvelope(s, extra, uint64(c.Mailbox.Limits.MaxResponseBytes))
	if err != nil {
		return nil, 0, err
	}
	retained, ok = sourceResponseAdd(retained, extra)
	if !ok {
		return nil, 0, errors.New("source host retained overflow")
	}
	record, ok := sourceResponseRecordUpper(q.Record)
	if !ok {
		return nil, 0, errors.New("source host record overflow")
	}
	metadata, ok := sourceResponseStringBytes(s.handoff.ID(), s.checkpoint, q.Attempt.SourceUnitID, s.regeneration.digest, q.Resume.Workspace, q.Attempt.ID)
	if !ok {
		return nil, 0, errors.New("source host metadata overflow")
	}
	wire := s.contract.PacketMaxResponseBytes()
	capture := s.contract.CaptureLimits()
	if capture.MaxArtifactBytes < wire {
		wire = capture.MaxArtifactBytes
	}
	if wire <= 0 || capture.MaxParentBytes <= 0 {
		return nil, 0, errors.New("source host existing ceilings")
	}
	// Reserve the complete adapter ceiling, including payloads that subsequently
	// fail semantic validation, so rejection cannot buy an unaccounted buffer.
	_, err = sourceResponseAggregateBound(retained, record, metadata, uint64(c.Mailbox.Limits.MaxResponseBytes), uint64(limits.MaxSemanticBytes), uint64(wire), uint64(capture.MaxParentBytes))
	return s, extra, err
}

func PrepareSourceHost(ctx context.Context, c host.SourceHostConfig, q SourceAcquisitionRequest, limits describeworker.SuppliedResponseLimits, boundedLocal ...bool) (SourceHostHandoff, error) {
	if len(boundedLocal) > 1 {
		return SourceHostHandoff{}, errors.New("source host read selector refused")
	}
	s, extra, err := sourceHostPreflight(ctx, c, q, limits)
	if err != nil {
		return SourceHostHandoff{}, err
	}
	if err = host.ValidateSourceHostConfig(c); err != nil {
		return SourceHostHandoff{}, err
	}
	out, sourceDigest, err := sourceResponseJoin(ctx, q.Resume, q.Record, q.Attempt)
	if err != nil {
		return SourceHostHandoff{}, err
	}
	retained, ok := sourceResponseRetainedUpper(s, out)
	cached, cacheErr := sourceAcquisitionEarlyEnvelope(s, extra, uint64(c.Mailbox.Limits.MaxResponseBytes))
	if !ok || cacheErr != nil || retained != cached || out.HandoffID != s.handoff.ID() {
		return SourceHostHandoff{}, errors.New("source host actual/cached source mismatch")
	}
	source := hostSourceContext(s, q, sourceDigest)
	actual, err := host.SourceHostRetentionUpper(c, q.Record, source)
	if err != nil {
		return SourceHostHandoff{}, err
	}
	actual, ok = sourceResponseAdd(actual, 8192)
	if !ok || actual != extra {
		return SourceHostHandoff{}, errors.New("source host exact transport retention mismatch")
	}
	if !sourceHostPublicationLive(ctx, s.request.Deadline) {
		return SourceHostHandoff{}, errors.New("source host expired before mailbox")
	}
	mailbox, err := host.NewSourceHostMailbox(ctx, c, q.Record, source, boundedLocal...)
	if err != nil {
		return SourceHostHandoff{}, err
	}
	// Copy the public header so changing the caller's Resume DTO cannot redirect
	// the owned request. Original proof payloads remain opaque and joined on Resume.
	resume := *q.Resume.PrivateSources
	q.Resume.PrivateSources = &resume
	if !sourceHostEligibilityLive(ctx, s.request.Deadline, mailbox.Deadline()) {
		mailbox.Close()
		return SourceHostHandoff{}, errors.New("source host expired at eligible publication")
	}
	return SourceHostHandoff{&sourceHostState{expected: s, request: q, source: source, limits: limits, extra: extra, mailbox: mailbox}}, nil
}

// CompleteSourceHost consumes one exchange and feeds its exact bytes ONCE to the
// adopted strict COMPLETE-adapter/SemanticV2 binder. It does not evaluate/compose,
// recapture, execute a model, or upgrade the supplied semantic origin.
func CompleteSourceHost(ctx context.Context, h SourceHostHandoff) (out DeliveredSourceResponse, err error) {
	bad := func(e error) (DeliveredSourceResponse, error) { return DeliveredSourceResponse{}, e }
	if h.state == nil {
		return bad(errors.New("source host upper ownership absent"))
	}
	s := h.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.used {
		return bad(errors.New("source host upper already consumed"))
	}
	s.used = true
	var tailDeadline time.Time
	var retainedCtx context.Context
	defer func() {
		closeErr := s.mailbox.Close()
		// The mailbox closes before exposing delivered custody. HOST's acceptance
		// child may now be expired; only the genuine original retained scope is used.
		if !tailDeadline.IsZero() && (retainedCtx == nil || retainedCtx.Err() != nil || !sourceclock.Now(retainedCtx, "host-close").Before(tailDeadline)) {
			closeErr = errors.Join(closeErr, context.DeadlineExceeded)
		}
		out, err = out.AfterClose(errors.Join(err, closeErr))
	}()
	if ctx == nil {
		return bad(errors.New("source host nil completion context"))
	}
	// Binding/completion cannot outlive the transport's original narrowed overall
	// deadline, even if the source pause itself had a longer remaining window.
	ctx, cancel := context.WithDeadline(ctx, s.mailbox.Deadline())
	defer cancel()
	if s.request.Resume.PrivateSources == nil || s.request.Resume.PrivateSources.Expected.state != s.expected || !sourceHostPublicationLive(ctx, s.source.Deadline) {
		return bad(errors.New("source host upper owner/deadline refused"))
	}
	// Recheck scalar cache/original context before exchange; completion's exact
	// source regeneration still belongs to the adopted supplied binder below.
	expected, err := sourceAcquisitionExpected(ctx, s.request.Resume, s.request.Attempt)
	if err != nil || expected != s.expected {
		return bad(errors.New("source host completion expected refused"))
	}
	payload, delivery, err := s.mailbox.Exchange(ctx)
	if err != nil {
		return bad(err)
	}
	if !delivery.Delivered() || len(payload) > s.limits.MaxAdapterBytes {
		return bad(errors.New("source host delivery/adapter ceiling"))
	}
	q := SuppliedSourceResponseRequest{Resume: s.request.Resume, Record: s.request.Record, Attempt: s.request.Attempt, AdapterLine: payload, Limits: s.limits}
	bound, err := bindSuppliedSourceResponse(ctx, q, s.extra)
	if err != nil {
		return bad(err)
	}
	if !sourceHostPublicationLive(ctx, s.source.Deadline) {
		return bad(errors.New("source host expired at completion"))
	}
	out = DeliveredSourceResponse{supplied: bound, delivery: delivery}
	for _, original := range s.expected.request.Originals {
		if f, ok := original.HOSTTimingV2(); ok {
			retainedCtx = original.HOSTRetainedContextV2()
			if retainedCtx == nil || retainedCtx.Err() != nil {
				return bad(errors.New("source HOST retained scope refused"))
			}
			tailDeadline = sourceclock.Now(retainedCtx, "host-complete").Add(time.Duration(f.Profile.PublicationCleanupMS) * time.Millisecond)
			if f.TotalDeadline.Before(tailDeadline) {
				tailDeadline = f.TotalDeadline
			}
			out.terminalDeadline = tailDeadline
			break
		}
	}
	return out, nil
}

// Use the actual established mailbox window, never a fresh overall timeout.
func sourceHostEligibilityLive(ctx context.Context, sourceDeadline, mailboxDeadline time.Time) bool {
	if ctx == nil || sourceDeadline.IsZero() || mailboxDeadline.IsZero() {
		return false
	}
	deadline := mailboxDeadline
	if sourceDeadline.Before(deadline) {
		deadline = sourceDeadline
	}
	if caller, ok := ctx.Deadline(); ok && caller.Before(deadline) {
		deadline = caller
	}
	return sourceHostPublicationLive(ctx, deadline)
}

// Separate seam permits call-local cancellation tests, never a production hook.
func sourceHostPublicationLive(ctx context.Context, deadline time.Time) bool {
	return ctx != nil && ctx.Err() == nil && !deadline.IsZero() && time.Now().Before(deadline)
}
