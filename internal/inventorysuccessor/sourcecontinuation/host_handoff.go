//go:build darwin || linux

package sourcecontinuation

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

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
	supplied BoundSuppliedSourceResponse
	delivery host.SourceHostDelivery
}

func (r DeliveredSourceResponse) Supplied() BoundSuppliedSourceResponse { return r.supplied }
func (r DeliveredSourceResponse) Delivery() host.SourceHostDelivery     { return r.delivery }
func (h SourceHostHandoff) Eligible() bool                              { return h.state != nil }
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

func PrepareSourceHost(ctx context.Context, c host.SourceHostConfig, q SourceAcquisitionRequest, limits describeworker.SuppliedResponseLimits) (SourceHostHandoff, error) {
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
	mailbox, err := host.NewSourceHostMailbox(ctx, c, q.Record, source)
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
func CompleteSourceHost(ctx context.Context, h SourceHostHandoff) (DeliveredSourceResponse, error) {
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
	defer s.mailbox.Close()
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
	return DeliveredSourceResponse{bound, delivery}, nil
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
