package inventorysuccessor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"lsp-trace/internal/adr0011methodsyntax"
	"lsp-trace/internal/adr0011querytarget"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/referencegrouping"
	"sort"
	"sync"
	"time"

	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

var ErrPrivateManagedInputOperationV2 = errors.New("private managed input operation refused")

// PrivateManagedInputVerificationV2 bounds existing kernel verification, not
// authority or funding. No head, authorization, decision or ceiling is created.
type PrivateManagedInputVerificationV2 struct{ Records, Bytes, Work uint64 }

// PrivateManagedInputOwnerV2 is an owner-local connection to EXISTING accounting
// custody. The zero value cannot grant anything. It is not Manager/source custody.
type PrivateManagedInputOwnerV2 struct {
	mu        sync.Mutex
	k         *kernelV2
	job       jobV2
	head      string
	limits    verificationLimitsV2
	uncertain bool
}

// OpenPrivateManagedInputOwnerV2 only reopens exact canonical existing records.
// Callers must already possess a private root with an initialized V2 job and its
// accepted authorization/head chain. Unchecked job DTOs cannot initialize one.
// Identity assurance remains CALLER_ASSERTED_NOT_AUTHENTICATED, as in the kernel.
func OpenPrivateManagedInputOwnerV2(ctx context.Context, root *publication.Root, canonicalJob []byte, expectedHead string, limits PrivateManagedInputVerificationV2) (*PrivateManagedInputOwnerV2, error) {
	var j jobV2
	if ctx == nil || root == nil || !hashV2(expectedHead) || parseV2(canonicalJob, &j) != nil {
		return nil, ErrPrivateManagedInputOperationV2
	}
	l := verificationLimitsV2{limits.Records, limits.Bytes, limits.Work}
	k := &kernelV2{root: root}
	h, err := k.reopen(ctx, j, l)
	if err != nil || h.Identity != expectedHead || !readyV2(h) {
		return nil, ErrPrivateManagedInputOperationV2
	}
	return &PrivateManagedInputOwnerV2{k: k, job: j, head: h.Identity, limits: l}, nil
}

// PrivateManagedInputBindingV2 is an expenditure request, never a receipt. The
// source digest is deliberately unknown before bounded no-follow preparation.
// Handoff is zero for ACQUIRE, exact opaque runtime custody for ISSUE/REPLAY.
// Policy/revision strings are declarations, not attestations. Kinds preserve order.
type PrivateManagedInputBindingV2 struct {
	Manager                                                                 *sessionruntime.Manager
	Root                                                                    *publication.Root
	Session                                                                 sessionruntime.SessionLeaseRequest
	URI, Occurrence, Revision, Privacy, OccurrencePolicy, CompositionPolicy string
	Line, Character                                                         uint32
	Kinds                                                                   [3]string
	KindCount                                                               uint8
	Deadline                                                                time.Time
	Mode                                                                    string
	Handoff                                                                 sessionruntime.PrivateGroupingOwnedHandoff
	CallsDigest                                                             string
}

type PrivateManagedInputOperationV2 struct {
	state *privateManagedInputOperationV2
}
type privateManagedInputOperationV2 struct {
	mu             sync.Mutex
	owner          *PrivateManagedInputOwnerV2
	binding        PrivateManagedInputBindingV2
	attempt, head  string
	used, finished bool
	execution      context.Context
	cancel         context.CancelFunc
	failed         bool
	handoff        sessionruntime.PrivateGroupingOwnedHandoff
	source         string
	envelope       sessionruntime.PrivateGroupingResourceEnvelope
}

func validPrivateManagedBindingV2(b PrivateManagedInputBindingV2) bool {
	if b.Manager == nil || b.Root == nil || b.Session.Generation == 0 || b.KindCount == 0 || b.KindCount > 3 || b.Deadline.IsZero() {
		return false
	}
	for _, s := range []string{b.Session.SessionID, b.Session.Workspace, b.URI, b.Occurrence, b.Revision, b.Privacy, b.OccurrencePolicy, b.CompositionPolicy} {
		if !sessionruntime.PrivateGroupingFieldValid(s) {
			return false
		}
	}
	for i, k := range b.Kinds {
		if i >= int(b.KindCount) {
			if k != "" {
				return false
			}
			continue
		}
		if k != "CALLS" && k != "REFERENCES_SYMBOL" && k != "RESOLVES_TO_DEFINITION" {
			return false
		}
		for j := 0; j < i; j++ {
			if b.Kinds[j] == k {
				return false
			}
		}
	}
	if b.CallsDigest != "" && !hashV2(b.CallsDigest) {
		return false
	}
	zero := sessionruntime.PrivateGroupingOwnedHandoff{}
	switch b.Mode {
	case "ACQUIRE":
		return b.Handoff == zero
	case "ISSUE", "REPLAY":
		return b.Handoff != zero
	default:
		return false
	}
}
func samePrivateManagedBindingV2(a, b PrivateManagedInputBindingV2) bool {
	// time.Equal permits identical instants represented in another time zone.
	d := a.Deadline.Equal(b.Deadline)
	a.Deadline = time.Time{}
	b.Deadline = time.Time{}
	return d && a == b
}

// privateManagedInputChargeV2 is a CLOSED logical currency contract, independent
// of caller caps. S=runtime-enforced private no-follow source ceiling (+1);
// Q=closed query/output ceiling. Three native operations admit <=32KiB params.
// The 64KiB/four-message transaction settings remain post-decode checks; concrete
// runtime-owned body/header/frame and shared residuals bound raw acquisition.
// P=7 records (six predecessors + input). O=1024 occurrences. The
// multiplicities bound adapter passes, including initial replay, failure after
// either R or D, all publications and readbacks, and TWO kernel verifications
// (reserve/observe). Kernel metadata costs are not physical RAM/CPU/IO proofs.
// No pilot proposal numbers, caller expenditure knobs, refunds or replay credit.
func privateManagedInputChargeV2(b PrivateManagedInputBindingV2, now time.Time) (countersV2, error) {
	e, ok := b.Manager.PrivateGroupingResourceEnvelope()
	if !ok {
		return countersV2{}, ErrPrivateManagedInputOperationV2
	}
	return privateManagedInputEnvelopeChargeV2(b, now, e)
}
func privateManagedInputEnvelopeChargeV2(b PrivateManagedInputBindingV2, now time.Time, e sessionruntime.PrivateGroupingResourceEnvelope) (countersV2, error) {
	if !validPrivateManagedBindingV2(b) || !now.Before(b.Deadline) || b.Deadline.Sub(now) > 15*time.Second {
		return countersV2{}, ErrPrivateManagedInputOperationV2
	}
	bounds := e.Bounds()
	if bounds.SourceReadBytes == 0 {
		return countersV2{}, ErrPrivateManagedInputOperationV2
	}
	s := bounds.SourceReadBytes // includes the rejecting +1 byte
	const q uint64 = sessionruntime.PrivateGroupingQueryMaxBytes
	const p uint64 = 7
	const o uint64 = 1024
	const verifyBytes uint64 = 2 * (64 << 20)
	const verifyRecords uint64 = 2 * 4096
	captureReq, captureResp, native, messages := uint64(0), uint64(0), uint64(0), uint64(0)
	if b.Mode == "ACQUIRE" {
		// Includes source/URI/language JSON escaping, request/cancel framing, and
		// all rejecting header bytes; never substitute the retained-byte cap.
		// Four notifications + four controls + the rejecting extra frame per method.
		captureReq = bounds.RequestBytes
		captureResp = bounds.ResponseBytes
		native = bounds.NativeRequests
		messages = bounds.Messages
	}
	// Each parser/hash/copy pass is charged at its admitted byte ceiling, even on
	// partial failure. Replay needs no capture and performs no publication.
	passes := uint64(32)
	// Four publication-stage byte allowances cover artifact, verification and
	// binding/readback metadata without claiming measured filesystem I/O.
	writes := 4*p*(q+(64<<10)) + 2*(recordLimitV2+2*headLimitV2)
	records := verifyRecords + 64*o + p*4
	if b.Mode == "REPLAY" {
		passes = 8
		writes = 2 * (recordLimitV2 + 2*headLimitV2)
		records = verifyRecords + 16*o + p*2
	}
	data := s + captureReq + captureResp + p*q
	return countersV2{
		LogicalBytes: passes * data, ReadHashBytes: verifyBytes + passes*data,
		WriteBytes: writes, Records: records, ParserBytes: verifyBytes + passes*data,
		ProviderRequests: native, ProviderMessages: messages,
		Work: 2*16384 + passes*(data+o*o), ActiveNS: uint64(b.Deadline.Sub(now)),
		Calls: 2*16384 + records - verifyRecords + 64 + native, RequestBytes: captureReq, ResponseBytes: captureResp,
		WireBytes: captureReq + captureResp, OutputBytes: p * q,
	}, nil
}

// PrivateManagedPilotPreflightV2 is a finite compatibility calculation, not a
// grant or provider receipt. The private path covers D/R+documentSymbol only;
// initialization, shutdown, CALLS, and typed process supervision remain unknown.
// A full-schedule FIT is deliberately impossible until those paths are covered.
type PrivateManagedPilotPreflightV2 struct {
	Disposition, Bound, Scope                                                  string
	SemanticRequests, ClientRequests, Slots, WireLimit, PrivateSubsetWireBound uint64
}

func PreflightPrivateManagedPilotV2(m *sessionruntime.Manager) PrivateManagedPilotPreflightV2 {
	r := PrivateManagedPilotPreflightV2{Disposition: "REFUSED", Bound: "UNKNOWN_RUNTIME_ENVELOPE", Scope: "PRIVATE_R_D_ONLY; INIT_SHUTDOWN_CALLS_UNCOVERED", SemanticRequests: 52, ClientRequests: 56, Slots: 150, WireLimit: 8 << 20}
	e, ok := m.PrivateGroupingResourceEnvelope()
	if !ok {
		return r
	}
	b := e.Bounds()
	// Fixed preregistered 52 semantic requests, nine possible frames each.
	// One source notification plus each request's sixfold params/framing/cancel.
	// Hard runtime caps make these products finite well below uint64 overflow.
	r.PrivateSubsetWireBound = 6*(b.SourceBytes+2*4096) + 8192 + 52*(6*(32<<10)+8192+9*(b.FrameBytes+1))
	r.Bound = "FULL_SCHEDULE_PATHS_UNCOVERED"
	if r.PrivateSubsetWireBound > r.WireLimit {
		r.Bound = "SEMANTIC_RAW_WIRE_BOUND_EXCEEDS_8MIB"
	}
	return r
}

// Reserve publishes the existing cumulative kernel reservation first. ONLY a
// confirmed successful commit returns a capability. Accounting artifacts may
// have been written on errors; no downstream permission escapes those errors.
func (o *PrivateManagedInputOwnerV2) Reserve(ctx context.Context, attempt string, b PrivateManagedInputBindingV2) (PrivateManagedInputOperationV2, error) {
	fail := func() (PrivateManagedInputOperationV2, error) {
		return PrivateManagedInputOperationV2{}, ErrPrivateManagedInputOperationV2
	}
	if o == nil || ctx == nil || ctx.Err() != nil || !hashV2(attempt) {
		return fail()
	}
	now := time.Now()
	envelope, ok := b.Manager.PrivateGroupingResourceEnvelope()
	if !ok {
		return fail()
	}
	c, err := privateManagedInputEnvelopeChargeV2(b, now, envelope)
	if err != nil {
		return fail()
	}
	if d, ok := ctx.Deadline(); ok && d.Before(b.Deadline) {
		return fail()
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.k == nil || o.uncertain {
		return fail()
	}
	h, err := o.k.reserve(ctx, o.job, o.head, attempt, c, b.Deadline.UnixNano(), now.UnixNano(), o.limits)
	if err != nil {
		// Only actual CAS uncertainty poisons the connection. An ordinary losing
		// reservation must not invalidate another already-confirmed operation on
		// this owner. All errors still return ZERO permission; subsequent attempts
		// reverify the existing kernel head and its sticky disposition.
		if errors.Is(err, errUncertainV2) {
			o.uncertain = true
		}
		return fail()
	}
	o.head = h.Identity
	return PrivateManagedInputOperationV2{&privateManagedInputOperationV2{owner: o, binding: b, attempt: attempt, head: h.Identity, envelope: envelope}}, nil
}

// Consume retires the capability BEFORE checking binding or returning a context.
// Failed/mismatched/expired use never revives it. A copied value shares retirement.
func (p PrivateManagedInputOperationV2) consumeFixed(ctx context.Context, b PrivateManagedInputBindingV2) (context.Context, context.CancelFunc, error) {
	if p.state == nil || ctx == nil {
		return nil, nil, ErrPrivateManagedInputOperationV2
	}
	s := p.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.used {
		return nil, nil, ErrPrivateManagedInputOperationV2
	}
	s.used = true
	currentEnvelope, envelopeOK := b.Manager.PrivateGroupingResourceEnvelope()
	if ctx.Err() != nil || !envelopeOK || currentEnvelope != s.envelope || !samePrivateManagedBindingV2(s.binding, b) || !time.Now().Before(s.binding.Deadline) {
		s.failed = true
		return nil, nil, ErrPrivateManagedInputOperationV2
	}
	c, cancel := context.WithDeadline(ctx, s.binding.Deadline)
	if b.Mode == "ACQUIRE" {
		bounded, err := b.Manager.PrivateGroupingEnvelopeContext(c, s.envelope, b.Deadline)
		if err != nil {
			cancel()
			s.failed = true
			return nil, nil, ErrPrivateManagedInputOperationV2
		}
		c = bounded
	}
	s.execution = c
	s.cancel = cancel
	return c, cancel, nil
}

// bindOwnedFixed is reachable only from the fixed owner's runtime-consume
// callback, AFTER that callback checks exact Manager/lease/query/source custody.
// Public BindOwned never calls this helper and cannot bind a caller hash.
func (p PrivateManagedInputOperationV2) bindOwnedFixed(h sessionruntime.PrivateGroupingOwnedHandoff, source string) error {
	if p.state == nil || h == (sessionruntime.PrivateGroupingOwnedHandoff{}) || !hashV2(source) {
		return ErrPrivateManagedInputOperationV2
	}
	s := p.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.used || s.execution == nil || s.failed || s.execution.Err() != nil || s.finished || s.source != "" || !time.Now().Before(s.binding.Deadline) || (s.binding.Mode != "ACQUIRE" && s.binding.Handoff != h) {
		return ErrPrivateManagedInputOperationV2
	}
	s.handoff = h
	s.source = source
	return nil
}

// completeFixed is unexported. Its only production callers are the fixed
// Acquire/Issue/paid-Replay entries AFTER their entire initial work succeeds.
// The context must be the still-live context created by private consumption,
// not a caller's fresh replacement. Failure never refunds or revives authority.
func (p PrivateManagedInputOperationV2) completeFixed(ctx context.Context) error {
	if p.state == nil || ctx == nil {
		return ErrPrivateManagedInputOperationV2
	}
	s := p.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.used || s.execution == nil || ctx != s.execution || s.failed || ctx.Err() != nil || !time.Now().Before(s.binding.Deadline) || s.finished || s.source == "" {
		s.failed = true
		return ErrPrivateManagedInputOperationV2
	}
	c := s.execution
	o := s.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.uncertain || o.head != s.head {
		return ErrPrivateManagedInputOperationV2
	}
	h, err := o.k.observe(c, o.job, s.head, s.attempt, "OBSERVED", time.Now().UnixNano(), o.limits)
	if err != nil {
		o.uncertain = true
		return ErrPrivateManagedInputOperationV2
	}
	o.head = h.Identity
	s.finished = true
	return nil
}

// Manual accounting helpers are deliberately closed. Retirement is not successful
// admission or evidence of execution. These compatibility refusals cannot emit
// OBSERVED, bind caller hashes, or return a usable execution context.
func (p PrivateManagedInputOperationV2) denyManual() error {
	if p.state == nil {
		return ErrPrivateManagedInputOperationV2
	}
	s := p.state
	s.mu.Lock()
	var cancel context.CancelFunc
	if !s.finished {
		s.used = true
		s.failed = true
		cancel = s.cancel
	}
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return ErrPrivateManagedInputOperationV2
}
func (p PrivateManagedInputOperationV2) Consume(context.Context, PrivateManagedInputBindingV2) (context.Context, context.CancelFunc, error) {
	return nil, nil, p.denyManual()
}
func (p PrivateManagedInputOperationV2) BindOwned(sessionruntime.PrivateGroupingOwnedHandoff, string) error {
	return p.denyManual()
}
func (p PrivateManagedInputOperationV2) Complete(context.Context) error { return p.denyManual() }
func (p PrivateManagedInputOperationV2) abortFixed() {
	if p.state == nil {
		return
	}
	s := p.state
	s.mu.Lock()
	var cancel context.CancelFunc
	if !s.finished {
		s.failed = true
		cancel = s.cancel
	}
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

const chainMaxBytes = 1500000

type Range = methodsyntax.Range

var ErrPrivateManagedGroupingPublication = errors.New("ADR0011 chain not verified")

func chainPublish(root *publication.Root, role string, b []byte) (string, string, error) {
	selector := fmt.Sprintf("adr0011-%s-%x.json", role, sha256.Sum256(b))
	receipt, err := publication.PublishBoundFile(root, selector, b, func(got []byte) error {
		if !bytes.Equal(got, b) {
			return ErrPrivateManagedGroupingPublication
		}
		return nil
	})
	if err != nil || receipt == nil || receipt.VerificationStatus != "VERIFIED" || receipt.Digest != privateGroupingDigest(b) {
		return "", "", ErrPrivateManagedGroupingPublication
	}
	got, err := publication.ReadVerifiedBoundFile(root, selector, chainMaxBytes)
	if err != nil || !bytes.Equal(got, b) {
		return "", "", ErrPrivateManagedGroupingPublication
	}
	return selector, receipt.Digest, nil
}

var ErrPrivateManagedGrouping = errors.New("private managed grouping input not issued")

// PrivateManagedGroupingInput is structural input only, never policy qualification.
// Public fields are copies; Replay compares them with the private issued originals.
type PrivateManagedGroupingInput struct {
	Input                referencegrouping.AdmittedInput
	TypedInput           programcadmission.TypedInput
	Predecessors         []referencegrouping.PredecessorRecord
	Ledger               []PrivateManagedGroupingOccurrence
	Dispositions         map[string]string
	originals            []referencegrouping.PredecessorRecord
	inputOriginal        []byte
	publications         []privateGroupingPublication
	ledgerOriginal       []PrivateManagedGroupingOccurrence
	dispositionsOriginal map[string]string
	replayBinding        PrivateManagedInputBindingV2
	sourceOriginal       string
}
type privateGroupingPublication struct {
	selector, digest string
	raw              []byte
}
type PrivateManagedGroupingOccurrence struct {
	Identity, Kind, From, To string
	Ordinal                  int
}
type PrivateManagedGroupingRequest struct {
	Session                                          sessionruntime.SessionLeaseRequest
	URI, OccurrenceID, Revision, Privacy             string
	OccurrenceAdmissionPolicyID, CompositionPolicyID string
	Line, Character                                  uint32
	Kinds                                            []programcadmission.RelationKind
	Root                                             *publication.Root
	Deadline                                         time.Time
	// Optional already-admitted CALLS; never relabeled from D/R or constructed here.
	Calls programcadmission.CompositeProjectionAdmission
	// Operation is confirmed existing-kernel expenditure authority, not source custody.
	Operation   PrivateManagedInputOperationV2
	nativeProof []byte // only the fixed mixed producer fills this; never a public admission DTO
}

// PrivateManagedGroupingOperationBinding constructs a typed expenditure request,
// not permission. Source custody is checked later by the existing runtime owner.
func PrivateManagedGroupingOperationBinding(m *sessionruntime.Manager, q PrivateManagedGroupingRequest, mode string, h sessionruntime.PrivateGroupingOwnedHandoff) (PrivateManagedInputBindingV2, error) {
	if !privateGroupingRequestFieldsValid(q) {
		return PrivateManagedInputBindingV2{}, ErrPrivateManagedGrouping
	}
	b := PrivateManagedInputBindingV2{Manager: m, Root: q.Root, Session: q.Session, URI: q.URI, Occurrence: q.OccurrenceID, Revision: q.Revision, Privacy: q.Privacy, OccurrencePolicy: q.OccurrenceAdmissionPolicyID, CompositionPolicy: q.CompositionPolicyID, Line: q.Line, Character: q.Character, KindCount: uint8(len(q.Kinds)), Deadline: q.Deadline, Mode: mode, Handoff: h}
	for i, k := range q.Kinds {
		b.Kinds[i] = string(k)
	}
	if q.Calls.Valid() {
		s := q.Calls.SourceBinding()
		b.CallsDigest = privateGroupingDigest([]byte(s.CompositeID + "\x00" + s.CompositeOutputSHA256))
	}
	return b, nil
}

// AcquirePrivateManagedGroupingInput is an additive private, operator-driven
// entry. It requires an existing READY session and never acquires/starts one.
// A confirmed one-shot reservation is consumed before lease acquisition. Full
// acquisition/evaluation process supervision remains out of scope.
func AcquirePrivateManagedGroupingInput(ctx context.Context, m *sessionruntime.Manager, q PrivateManagedGroupingRequest) (PrivateManagedGroupingInput, error) {
	if !privateGroupingRequestFieldsValid(q) || ctx == nil || m == nil || q.Root == nil {
		return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
	}
	binding, err := PrivateManagedGroupingOperationBinding(m, q, "ACQUIRE", sessionruntime.PrivateGroupingOwnedHandoff{})
	if err != nil {
		return PrivateManagedGroupingInput{}, err
	}
	ctx, cancel, err := q.Operation.consumeFixed(ctx, binding)
	if err != nil {
		return PrivateManagedGroupingInput{}, errors.Join(ErrPrivateManagedGrouping, err)
	}
	defer cancel()
	defer q.Operation.abortFixed()
	refs, defs := false, false
	seen := map[programcadmission.RelationKind]bool{}
	for _, k := range q.Kinds {
		if seen[k] {
			return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
		}
		seen[k] = true
		switch k {
		case programcadmission.TypedReferencesSymbol:
			refs = true
		case programcadmission.TypedResolvesToDefinition:
			defs = true
		case programcadmission.TypedCalls:
		default:
			return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
		}
	}
	if !refs && !defs {
		return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
	}
	var out PrivateManagedGroupingInput
	err = m.WithReadySessionLease(ctx, q.Session, func(leased context.Context) error {
		h, err := m.CapturePrivateGroupingOwned(leased, sessionruntime.PrivateGroupingOwnedRequest{Session: q.Session, URI: q.URI, OccurrenceID: q.OccurrenceID, Revision: q.Revision, Privacy: q.Privacy, Line: q.Line, Character: q.Character, References: refs, Definitions: defs, Deadline: q.Deadline})
		if err != nil {
			return err
		}
		out, err = issuePrivateManagedGroupingInput(leased, m, h, q)
		return err
	})
	if err != nil {
		return PrivateManagedGroupingInput{}, errors.Join(ErrPrivateManagedGrouping, err)
	}
	if err = q.Operation.completeFixed(ctx); err != nil {
		return PrivateManagedGroupingInput{}, errors.Join(ErrPrivateManagedGrouping, err)
	}
	return out, nil
}

// IssuePrivateManagedGroupingInput consumes opaque current runtime custody, not
// an OwnedMethodPair snapshot or a caller-supplied query receipt. It must run
// inside the same active session lease that created h. Once the consume callback
// is admitted, failure retires the handle too.
func IssuePrivateManagedGroupingInput(ctx context.Context, m *sessionruntime.Manager, h sessionruntime.PrivateGroupingOwnedHandoff, q PrivateManagedGroupingRequest) (PrivateManagedGroupingInput, error) {
	if !privateGroupingRequestFieldsValid(q) || ctx == nil || m == nil || q.Root == nil || q.Deadline.IsZero() || !time.Now().Before(q.Deadline) {
		return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
	}
	binding, err := PrivateManagedGroupingOperationBinding(m, q, "ISSUE", h)
	if err != nil {
		return PrivateManagedGroupingInput{}, err
	}
	ctx, cancel, err := q.Operation.consumeFixed(ctx, binding)
	if err != nil {
		return PrivateManagedGroupingInput{}, errors.Join(ErrPrivateManagedGrouping, err)
	}
	defer cancel()
	defer q.Operation.abortFixed()
	out, err := issuePrivateManagedGroupingInput(ctx, m, h, q)
	if err != nil {
		return PrivateManagedGroupingInput{}, err
	}
	if err = q.Operation.completeFixed(ctx); err != nil {
		return PrivateManagedGroupingInput{}, errors.Join(ErrPrivateManagedGrouping, err)
	}
	return out, nil
}

// Only the already consumed Acquire/Issue operation can reach this continuation.
func issuePrivateManagedGroupingInput(ctx context.Context, m *sessionruntime.Manager, h sessionruntime.PrivateGroupingOwnedHandoff, q PrivateManagedGroupingRequest) (PrivateManagedGroupingInput, error) {
	var out PrivateManagedGroupingInput
	err := m.ConsumePrivateGroupingOwned(ctx, h, func(v sessionruntime.PrivateGroupingOwnedView) error {
		if !q.Deadline.Equal(v.Deadline) {
			return ErrPrivateManagedGrouping
		}
		if v.Workspace != q.Session.Workspace || v.Query.SessionID != q.Session.SessionID || v.Query.Generation != q.Session.Generation || v.Query.URI != q.URI || v.Query.OccurrenceID != q.OccurrenceID || v.Query.Line != q.Line || v.Query.Character != q.Character || v.Revision != q.Revision || v.Privacy != q.Privacy {
			return ErrPrivateManagedGrouping
		}
		// Re-select from the exact owned result before any publication or admission.
		target, err := adr0011querytarget.SelectDocumentSymbolCandidateV1(v.Query, v.Symbol.Result)
		if err != nil || target != v.Target || v.Symbol.Source == nil || v.Query.SourceDigest != v.Symbol.Source.SHA256 || v.Query.SourceDigest != privateGroupingDigest(v.Source) {
			return ErrPrivateManagedGrouping
		}
		if err := q.Operation.bindOwnedFixed(h, v.Query.SourceDigest); err != nil {
			return err
		}
		queryOriginal, err := privateGroupingQueryBytes(v)
		if err != nil {
			return err
		}
		out, err = privateGroupingBuild(v, q, queryOriginal)
		if err != nil {
			return fmt.Errorf("private grouping build: %w", err)
		}
		if len(out.Ledger) == 0 {
			return nil
		} // no false grouping claim for empty/unavailable
		// Final exact-byte publication/readback precedes accepted return. This reuses
		// the existing bound-file mechanism; it is not a new authority framework.
		for _, p := range out.Predecessors {
			if err := ctx.Err(); err != nil {
				return err
			}
			selector, digest, err := chainPublish(q.Root, "private-grouping", p.Bytes)
			if err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			out.publications = append(out.publications, privateGroupingPublication{selector, digest, append([]byte(nil), p.Bytes...)})
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		raw := out.Input.CanonicalBytes()
		selector, digest, err := chainPublish(q.Root, "private-grouping-input", raw)
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		out.publications = append(out.publications, privateGroupingPublication{selector, digest, append([]byte(nil), raw...)})
		out.originals = privateGroupingClonePredecessors(out.Predecessors)
		out.ledgerOriginal = append([]PrivateManagedGroupingOccurrence(nil), out.Ledger...)
		out.dispositionsOriginal = make(map[string]string, len(out.Dispositions))
		for method, disposition := range out.Dispositions {
			out.dispositionsOriginal[method] = disposition
		}
		out.inputOriginal = append([]byte(nil), raw...)
		out.replayBinding, err = PrivateManagedGroupingOperationBinding(m, q, "REPLAY", h)
		if err != nil {
			return err
		}
		out.sourceOriginal = v.Query.SourceDigest
		return out.replayPrivate(ctx, q.Root)
	})
	if err != nil {
		return PrivateManagedGroupingInput{}, errors.Join(ErrPrivateManagedGrouping, err)
	}
	return out, nil
}

// Replay checks exact issued bytes and their original bound-file readbacks; the
// ordinary grouping parser alone does not authenticate native query execution.
func (in PrivateManagedGroupingInput) Replay(root *publication.Root) error {
	// A previous initial replay is not a reusable allowance for new work.
	return ErrPrivateManagedGrouping
}

// ReplayReserved requires another confirmed operation; no acquisition is repeated.
func (in PrivateManagedGroupingInput) ReplayReserved(ctx context.Context, operation PrivateManagedInputOperationV2) error {
	ctx, cancel, err := operation.consumeFixed(ctx, in.replayBinding)
	if err != nil {
		return errors.Join(ErrPrivateManagedGrouping, err)
	}
	defer cancel()
	defer operation.abortFixed()
	if err = operation.bindOwnedFixed(in.replayBinding.Handoff, in.sourceOriginal); err != nil {
		return err
	}
	if err = in.replayPrivate(ctx, in.replayBinding.Root); err != nil {
		return err
	}
	return operation.completeFixed(ctx)
}

// ReplayOperationBinding returns a copy of declarations, never a permission.
func (in PrivateManagedGroupingInput) ReplayOperationBinding() PrivateManagedInputBindingV2 {
	return in.replayBinding
}

func (in PrivateManagedGroupingInput) replayPrivate(ctx context.Context, root *publication.Root) error {
	if ctx == nil || ctx.Err() != nil {
		return ErrPrivateManagedGrouping
	}
	if root == nil || len(in.inputOriginal) == 0 || len(in.originals) != len(in.Predecessors) || !bytes.Equal(in.Input.CanonicalBytes(), in.inputOriginal) {
		return ErrPrivateManagedGrouping
	}
	if len(in.Ledger) != len(in.ledgerOriginal) || len(in.Dispositions) != len(in.dispositionsOriginal) {
		return ErrPrivateManagedGrouping
	}
	for i, item := range in.ledgerOriginal {
		if in.Ledger[i] != item {
			return ErrPrivateManagedGrouping
		}
	}
	for method, disposition := range in.dispositionsOriginal {
		if in.Dispositions[method] != disposition {
			return ErrPrivateManagedGrouping
		}
	}
	for i, p := range in.originals {
		candidate := in.Predecessors[i]
		if p.Role != candidate.Role || p.Selector != candidate.Selector || p.Schema != candidate.Schema || !bytes.Equal(p.Bytes, candidate.Bytes) {
			return ErrPrivateManagedGrouping
		}
	}
	if len(in.publications) != len(in.originals)+1 {
		return ErrPrivateManagedGrouping
	}
	for _, p := range in.publications {
		if ctx.Err() != nil {
			return ErrPrivateManagedGrouping
		}
		raw, err := publication.ReadVerifiedBoundFile(root, p.selector, chainMaxBytes)
		if err != nil || privateGroupingDigest(raw) != p.digest || !bytes.Equal(raw, p.raw) {
			return ErrPrivateManagedGrouping
		}
	}
	if ctx.Err() != nil {
		return ErrPrivateManagedGrouping
	}
	admitted, err := referencegrouping.ParseAndAdmit(in.inputOriginal, in.originals, privateGroupingParseLimits())
	if err != nil || ctx.Err() != nil || !admitted.MatchesTypedInput(in.TypedInput) {
		return ErrPrivateManagedGrouping
	}
	return nil
}

// Native byte slices are encoded as base64 by encoding/json. RawMessage would
// silently normalize lexical originals, so it is deliberately not used here.
type privateGroupingNative struct {
	Method, SessionID     string
	Generation, RequestID uint64
	Params, Result        []byte
	Write                 sessionruntime.RequestWriteObservation
	Read                  sessionruntime.ResponseReadObservation
	Source                *sessionruntime.OwnedDocumentBinding
}

func privateGroupingNativePair(p sessionruntime.OwnedMethodPair) privateGroupingNative {
	return privateGroupingNative{p.Method, p.SessionID, p.Generation, p.Key.ID, []byte(p.Params), []byte(p.Result), p.Write, p.Read, p.Source}
}

type privateGroupingSupply struct {
	Classification, SessionID string
	Generation                uint64
	URI                       string
	DocumentVersion           int
	Method                    string
	Content, Params           []byte
}

func privateGroupingQueryBytes(v sessionruntime.PrivateGroupingOwnedView) ([]byte, error) {
	bound, ok := sessionruntime.PrivateGroupingQueryEncodedBound(v)
	if !ok {
		return nil, ErrPrivateManagedGrouping
	}
	methods := make([]privateGroupingNative, len(v.Methods))
	for i, p := range v.Methods {
		methods[i] = privateGroupingNativePair(p)
	}
	return privateGroupingJSON(struct {
		Query                      adr0011querytarget.Query
		Target                     adr0011querytarget.Candidate
		Source                     []byte
		Supply                     privateGroupingSupply
		Metadata                   sessionruntime.SessionMetadata
		Revision, Custody, Privacy string
		Workspace                  string
		Deadline                   time.Time
		Symbol                     privateGroupingNative
		Methods                    []privateGroupingNative
		Unavailable                []string
	}{v.Query, v.Target, v.Source, privateGroupingSupply{v.Supply.Classification, v.Supply.SessionID, v.Supply.Generation, v.Supply.URI, v.Supply.DocumentVersion, v.Supply.Method, v.Supply.Content, []byte(v.Supply.Params)}, v.Metadata, v.Revision, "CALLER_ASSERTED", v.Privacy, v.Workspace, v.Deadline, privateGroupingNativePair(v.Symbol), methods, v.Unavailable}, bound)
}
func privateGroupingBuild(v sessionruntime.PrivateGroupingOwnedView, q PrivateManagedGroupingRequest, query []byte) (PrivateManagedGroupingInput, error) {
	out := PrivateManagedGroupingInput{Dispositions: map[string]string{}, Ledger: []PrivateManagedGroupingOccurrence{}}
	// This reports the current provider capability, not the provenance of an
	// optional separately admitted CALLS composite supplied by the operator.
	if !v.Metadata.CallHierarchySupport {
		out.Dispositions["CALLS"] = "UNAVAILABLE"
	}
	for _, m := range v.Unavailable {
		out.Dispositions[m] = "UNAVAILABLE"
	}
	envelope := programcadmission.UntrustedMethodEvidenceEnvelope{Family: programcadmission.MethodEvidenceFamily, Version: programcadmission.MethodEvidenceFamily, ClaimCeiling: programcadmission.MethodEvidenceClaimCeiling, Completeness: "UNKNOWN", Provider: v.Metadata.ProviderName, ProviderVersion: v.Metadata.ProviderVersion, SessionID: v.Query.SessionID, Generation: v.Query.Generation, Revision: v.Revision, Custody: "CALLER_ASSERTED", PositionEncoding: v.Query.Encoding}
	// Missing provider metadata is unavailable provenance, not invented identity.
	if envelope.Provider == "" || envelope.ProviderVersion == "" {
		return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
	}
	selected := map[programcadmission.RelationKind]bool{}
	for _, kind := range q.Kinds {
		if kind != programcadmission.TypedCalls && kind != programcadmission.TypedReferencesSymbol && kind != programcadmission.TypedResolvesToDefinition {
			return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
		}
		if selected[kind] {
			return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
		}
		selected[kind] = true
	}
	observed := make(map[programcadmission.RelationKind]bool)
	for _, method := range v.Unavailable {
		if method == "textDocument/references" {
			observed[programcadmission.TypedReferencesSymbol] = true
		} else if method == "textDocument/definition" {
			observed[programcadmission.TypedResolvesToDefinition] = true
		} else {
			return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
		}
	}
	for _, pair := range v.Methods {
		if pair.Method == "textDocument/references" {
			observed[programcadmission.TypedReferencesSymbol] = true
		} else if pair.Method == "textDocument/definition" {
			observed[programcadmission.TypedResolvesToDefinition] = true
		} else {
			return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
		}
	}
	for _, kind := range []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition} {
		if selected[kind] != observed[kind] {
			return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
		}
	}
	for _, pair := range v.Methods {
		kind, sr, tr := programcadmission.TypedReferencesSymbol, "REFERENCING_OCCURRENCE", "REFERENCED_SYMBOL"
		if pair.Method == "textDocument/definition" {
			kind, sr, tr = programcadmission.TypedResolvesToDefinition, "QUERY_SYMBOL", "DEFINITION_TARGET"
		}
		if !selected[kind] {
			return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
		}
		parsed, failure := methodsyntax.ParseRaw(pair.Method, pair.Result, 256)
		if failure != nil {
			return PrivateManagedGroupingInput{}, failure
		}
		if len(parsed.Items) == 0 {
			out.Dispositions[pair.Method] = "OBSERVED_EMPTY"
		} else {
			out.Dispositions[pair.Method] = "OBSERVED_NONEMPTY"
		}
		for _, item := range parsed.Items {
			endpointBound, ok := privateGroupingStringsBound(1024, item.URI)
			if !ok {
				return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
			}
			endpointRaw, err := privateGroupingJSON(struct {
				URI         string
				Range       Range
				TargetRange *Range
			}{item.URI, item.Range, item.TargetRange}, endpointBound)
			if err != nil {
				return PrivateManagedGroupingInput{}, err
			}
			endpoint := privateGroupingDigest(endpointRaw)
			from, to := endpoint, v.Target.SymbolID
			if kind == programcadmission.TypedResolvesToDefinition {
				from, to = v.Target.SymbolID, endpoint
			}
			identity := privateGroupingDigest([]byte(fmt.Sprintf("private-owned-grouping\x00%s\x00%d\x00%d\x00%s\x00%d", pair.SessionID, pair.Generation, pair.Key.ID, pair.Method, item.Ordinal)))
			o := programcadmission.MethodOccurrenceEvidence{Identity: identity, From: from, To: to, Kind: kind, Ordinal: item.Ordinal, SourceRole: sr, TargetRole: tr, Method: pair.Method, QuerySelector: "methods/query.json", QueryDigest: privateGroupingDigest(query), QueryLength: len(query), EvidenceSelector: "methods/evidence.json", EvidenceDigest: privateGroupingDigest(pair.Result), EvidenceLength: len(pair.Result), Provider: envelope.Provider, ProviderVersion: envelope.ProviderVersion, SessionID: envelope.SessionID, Generation: envelope.Generation, Revision: envelope.Revision, Custody: envelope.Custody, PositionEncoding: envelope.PositionEncoding}
			envelope.Occurrences = append(envelope.Occurrences, o)
			out.Ledger = append(out.Ledger, PrivateManagedGroupingOccurrence{o.Identity, string(o.Kind), o.From, o.To, o.Ordinal})
		}
	}
	if len(out.Ledger) == 0 {
		// The unchanged typed boundary rejects selected zero-occurrence method
		// families. Do not silently discard a separately supplied CALLS input.
		if selected[programcadmission.TypedCalls] {
			if len(q.nativeProof) == 0 || len(q.Kinds) != 1 {
				return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
			}
			typed, err := programcadmission.PrepareTypedInput(programcadmission.TypedInputRequest{Kinds: q.Kinds, Calls: q.Calls})
			if err != nil {
				return PrivateManagedGroupingInput{}, err
			}
			out.TypedInput = typed
			for _, o := range typed.Occurrences {
				out.Ledger = append(out.Ledger, PrivateManagedGroupingOccurrence{o.Identity, string(o.Kind), o.From, o.To, o.Ordinal})
			}
			return privateGroupingAssemble(out, query, programcadmission.MethodProjectionAdmission{}, q)
		}
		return out, nil
	}
	methods, err := programcadmission.AdmitUntrustedMethodEvidenceEnvelope(envelope)
	if err != nil {
		return PrivateManagedGroupingInput{}, err
	}
	typed, err := programcadmission.PrepareTypedInput(programcadmission.TypedInputRequest{Methods: methods, EnableMethods: true, Kinds: q.Kinds, Calls: q.Calls})
	if err != nil {
		return PrivateManagedGroupingInput{}, err
	}
	out.TypedInput = typed
	return privateGroupingAssemble(out, query, methods, q)
}
func privateGroupingAssemble(out PrivateManagedGroupingInput, query []byte, methods programcadmission.MethodProjectionAdmission, q PrivateManagedGroupingRequest) (PrivateManagedGroupingInput, error) {
	type mapping struct {
		Identity         string `json:"identity"`
		Kind             string `json:"kind"`
		From             string `json:"from"`
		To               string `json:"to"`
		Ordinal          int    `json:"ordinal"`
		QuerySelector    string `json:"query_selector"`
		EvidenceSelector string `json:"evidence_selector"`
	}
	methodRows := methods.Occurrences() // existing admitted getter; <= 2*256 parsed items
	if len(methodRows) > 512 || len(out.TypedInput.Occurrences) > 1024 {
		return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
	}
	mappingBound := uint64(1024)
	for _, o := range methodRows {
		var ok bool
		mappingBound, ok = privateGroupingStringsBound(mappingBound, o.Identity, string(o.Kind), o.From, o.To, o.QuerySelector, o.EvidenceSelector)
		if !ok {
			return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
		}
		mappingBound, ok = sessionruntime.PrivateGroupingCheckedEncodedSize(mappingBound, 0, 0, 256)
		if !ok {
			return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
		}
	}
	mappings := make([]mapping, 0, len(methodRows))
	for _, o := range methodRows {
		mappings = append(mappings, mapping{o.Identity, string(o.Kind), o.From, o.To, o.Ordinal, o.QuerySelector, o.EvidenceSelector})
	}
	sort.Slice(mappings, func(i, j int) bool { return mappings[i].Identity < mappings[j].Identity })
	mapped, err := privateGroupingJSON(struct {
		Family   string    `json:"family"`
		Version  string    `json:"version"`
		Mappings []mapping `json:"mappings"`
	}{"METHOD_MAPPING_RECEIPT", "v1", mappings}, mappingBound)
	if err != nil {
		return PrivateManagedGroupingInput{}, err
	}
	type row struct {
		Identity   string `json:"identity"`
		Kind       string `json:"kind"`
		From       string `json:"from"`
		To         string `json:"to"`
		Ordinal    int    `json:"ordinal"`
		RelationID string `json:"relation_id"`
	}
	ledgerBound := uint64(1024)
	for _, o := range out.TypedInput.Occurrences {
		var ok bool
		ledgerBound, ok = privateGroupingStringsBound(ledgerBound, o.Identity, string(o.Kind), o.From, o.To, o.RelationID)
		if !ok {
			return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
		}
		ledgerBound, ok = sessionruntime.PrivateGroupingCheckedEncodedSize(ledgerBound, 0, 0, 256)
		if !ok {
			return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
		}
	}
	rows := make([]row, len(out.TypedInput.Occurrences))
	custody := []referencegrouping.EndpointCustody{}
	seen := map[string]bool{}
	outcomes := []referencegrouping.CandidateOutcome{}
	for i, o := range out.TypedInput.Occurrences {
		rows[i] = row{o.Identity, string(o.Kind), o.From, o.To, o.Ordinal, o.RelationID}
		outcomes = append(outcomes, referencegrouping.CandidateOutcome{OccurrenceIdentity: o.Identity, Disposition: "ADMITTED"})
		selector := "methods/evidence.json"
		if o.Kind == programcadmission.TypedCalls {
			selector = "calls/acquisition.json"
		}
		for _, endpoint := range []string{o.From, o.To} {
			key := selector + "\x00" + endpoint
			if !seen[key] {
				seen[key] = true
				custody = append(custody, referencegrouping.EndpointCustody{Endpoint: endpoint, PredecessorSelector: selector, Custody: "CALLER_ASSERTED"})
			}
		}
	}
	ledger, err := privateGroupingJSON(struct {
		Domain      string `json:"domain"`
		Version     string `json:"version"`
		Claim       string `json:"claim"`
		Occurrences []row  `json:"occurrences"`
	}{"PROGRAM_C_TYPED_OCCURRENCES", "v1", "ADMITTED_TYPED_OCCURRENCES_EXACT", rows}, ledgerBound)
	if err != nil {
		return PrivateManagedGroupingInput{}, err
	}
	out.Predecessors = []referencegrouping.PredecessorRecord{{Role: "METHOD_QUERY", Selector: "methods/query.json", Schema: "lsp-trace.private.program-c-method-query.v0", Bytes: query}, {Role: "METHOD_EVIDENCE", Selector: "methods/evidence.json", Schema: programcadmission.MethodEvidenceFamily, Bytes: methods.CanonicalBytes()}, {Role: "METHOD_MAPPING_RECEIPT", Selector: "methods/mapping-receipt.json", Schema: "lsp-trace.private.program-c-method-mapping-receipt.v1", Bytes: mapped}, {Role: "OCCURRENCE_LEDGER", Selector: "ledgers/occurrences.json", Schema: "lsp-trace.private.program-c-occurrence-ledger.v1", Bytes: ledger}}
	if len(methodRows) == 0 {
		out.Predecessors = out.Predecessors[3:]
	}
	projection, err := programcadmission.PreparePrivateLeidenProjection(out.TypedInput)
	if err != nil {
		return PrivateManagedGroupingInput{}, err
	}
	if retained := projection.RetainedCallsCanonicalBytes(); len(retained) > 0 {
		// SourceBinding is the admission package's closed mirror/subset of the
		// retained composite (no caller-added fields). Six times its original
		// JSON bytes bounds re-escaping; 16 KiB covers added fixed identity keys
		// and scalar envelope fields. Check before cloning/marshalling metadata.
		bindingBound, ok := sessionruntime.PrivateGroupingCheckedEncodedSize(16<<10, uint64(len(retained)), 6, 0)
		if !ok {
			return PrivateManagedGroupingInput{}, fmt.Errorf("%w: composite binding bound bytes=%d", ErrPrivateManagedGrouping, len(retained))
		}
		binding, err := privateGroupingJSON(projection.CompositeSourceBinding(), bindingBound)
		if err != nil {
			return PrivateManagedGroupingInput{}, err
		}
		out.Predecessors = append(out.Predecessors, referencegrouping.PredecessorRecord{Role: "RETAINED_COMPOSITE", Selector: "calls/composite.json", Schema: "lsp-trace.private.program-c-composite.v1", Bytes: retained}, referencegrouping.PredecessorRecord{Role: "ACQUISITION_RECEIPT", Selector: "calls/acquisition.json", Schema: "lsp-trace.private.program-c-acquisition-receipt.v1", Bytes: binding})
	}
	if len(q.nativeProof) > 0 {
		out.Predecessors = append(out.Predecessors, referencegrouping.PredecessorRecord{Role: "NATIVE_EVIDENCE", Selector: "calls/native-evidence.json", Schema: referencegrouping.PrivateNativeEvidenceSchemaV2, Bytes: q.nativeProof})
	}
	// The new path has a checked aggregate retention budget BEFORE BuildInput's
	// encoding and before any publication. Original paths retain their old bounds.
	if len(q.nativeProof) > 0 {
		if len(out.Predecessors)+1 > 8 {
			return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
		}
		var total uint64
		for _, p := range out.Predecessors {
			if uint64(len(p.Bytes)) > (1<<20)-total {
				return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
			}
			total += uint64(len(p.Bytes))
		}
		metadata := make([]referencegrouping.PredecessorRecord, len(out.Predecessors))
		for i, p := range out.Predecessors {
			metadata[i] = referencegrouping.PredecessorRecord{Role: p.Role, Selector: p.Selector, Schema: p.Schema, ClaimedDigest: privateGroupingDigest(p.Bytes)}
		}
		bound, ok := referencegrouping.PrivateNativeEncodedBoundV2(struct {
			Rows         any
			Custody      any
			Predecessors any
		}{rows, custody, metadata}, 1<<20)
		if !ok || total > 1<<20 || bound > (1<<20)-total || 65536 > (1<<20)-total-bound {
			return PrivateManagedGroupingInput{}, fmt.Errorf("%w: aggregate retention bound total=%d encoding=%d ok=%v", ErrPrivateManagedGrouping, total, bound, ok)
		}
	}
	n := len(rows)
	artifact, err := referencegrouping.BuildInput(referencegrouping.InputRequest{TypedInput: out.TypedInput, Predecessors: out.Predecessors, OccurrenceLedgerBytes: ledger, EndpointCustody: custody, OccurrenceAdmissionPolicyID: q.OccurrenceAdmissionPolicyID, CompositionPolicyID: q.CompositionPolicyID, PrivacyPolicy: q.Privacy, EligibleKinds: []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}, Limits: referencegrouping.ResourceLimits{MaxArtifacts: 8, MaxNodes: 1024, MaxOccurrences: 1024, MaxContributions: 1024, MaxCanonicalBytes: 1 << 20, MaxStringBytes: 4096}, CandidateOutcomes: outcomes, Accounting: referencegrouping.Accounting{Candidates: n, Included: n, Terminal: true}})
	if err != nil {
		return PrivateManagedGroupingInput{}, err
	}
	if len(q.nativeProof) > 0 {
		total := len(artifact.CanonicalBytes())
		for _, p := range out.Predecessors {
			if len(p.Bytes) > (1<<20)-total {
				return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
			}
			total += len(p.Bytes)
		}
	}
	out.Input, err = referencegrouping.ParseAndAdmit(artifact.CanonicalBytes(), out.Predecessors, privateGroupingParseLimits())
	if err != nil {
		return PrivateManagedGroupingInput{}, err
	}
	return out, nil
}
func privateGroupingParseLimits() referencegrouping.ParseLimits {
	return referencegrouping.ParseLimits{MaxRawBytes: 1 << 20, MaxArtifacts: 8, MaxNodes: 1024, MaxOccurrences: 1024, MaxContributions: 1024, MaxCanonicalBytes: 1 << 20, MaxStringBytes: 4096}
}
func privateGroupingDigest(b []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(b)) }

// Every call site supplies its typed, checked preflight bound; this function
// only marshals an already-admitted value. The post-check is a backstop, not the
// admission mechanism. It is deliberately not a replaceable serializer hook.
func privateGroupingJSON(v any, admittedBound uint64) ([]byte, error) {
	if admittedBound == 0 || admittedBound > uint64(chainMaxBytes) {
		return nil, ErrPrivateManagedGrouping
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if len(raw) >= chainMaxBytes || uint64(len(raw)) >= admittedBound {
		return nil, ErrPrivateManagedGrouping
	}
	return append(raw, '\n'), nil
}
func privateGroupingStringsBound(total uint64, fields ...string) (uint64, bool) {
	for _, field := range fields {
		if field != "" && !sessionruntime.PrivateGroupingFieldValid(field) {
			return 0, false
		}
		var ok bool
		total, ok = sessionruntime.PrivateGroupingCheckedEncodedSize(total, uint64(len(field)), 6, 2)
		if !ok {
			return 0, false
		}
	}
	return total, true
}
func privateGroupingRequestFieldsValid(q PrivateManagedGroupingRequest) bool {
	if len(q.Kinds) == 0 || len(q.Kinds) > sessionruntime.PrivateGroupingMaxKinds || q.Session.Generation == 0 {
		return false
	}
	for _, field := range []string{q.Session.SessionID, q.Session.Workspace, q.URI, q.OccurrenceID, q.Revision, q.Privacy, q.OccurrenceAdmissionPolicyID, q.CompositionPolicyID} {
		if !sessionruntime.PrivateGroupingFieldValid(field) {
			return false
		}
	}
	for i, kind := range q.Kinds {
		if kind != programcadmission.TypedCalls && kind != programcadmission.TypedReferencesSymbol && kind != programcadmission.TypedResolvesToDefinition {
			return false
		}
		for j := 0; j < i; j++ {
			if q.Kinds[j] == kind {
				return false
			}
		}
	}
	// The existing opaque admission getter owns its own finite predecessor cap;
	// reject before deriving typed/output collections from a large CALLS set.
	if q.Calls.Valid() && len(q.Calls.Occurrences()) > 512 {
		return false
	}
	return true
}

func privateGroupingClonePredecessors(in []referencegrouping.PredecessorRecord) []referencegrouping.PredecessorRecord {
	out := append([]referencegrouping.PredecessorRecord(nil), in...)
	for i := range out {
		out[i].Bytes = append([]byte(nil), out[i].Bytes...)
	}
	return out
}
