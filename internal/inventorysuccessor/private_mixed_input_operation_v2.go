package inventorysuccessor

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"sync"
	"time"

	sourceclock "lsp-trace/internal/privatesourceclock"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

var ErrPrivateMixedInputV2 = errors.New("private mixed input checkpoint refused")
var ErrPrivateMixedInputUnavailableV2 = errors.New("private mixed input requested family unavailable")
var ErrPrivateMixedInputEmptyV2 = errors.New("private mixed input observed empty; no grouping input")

type PrivateMixedInputRequestV2 struct {
	Manager                                                                   *sessionruntime.Manager
	Root                                                                      *publication.Root
	Start                                                                     sessionruntime.StartRequest
	URI, OccurrenceID, Revision, Privacy, OccurrencePolicy, CompositionPolicy string
	Line, Character                                                           uint32
	Kinds                                                                     []programcadmission.RelationKind
	Timing                                                                    *PrivateSourceTimingFrameV2
}
type PrivateMixedInputOperationV2 struct{ state *privateMixedInputOperationV2 }
type privateMixedInputOperationV2 struct {
	mu              sync.Mutex
	used            bool
	owner           *PrivateManagedInputOwnerV2
	request         PrivateMixedInputRequestV2
	envelope        sessionruntime.PrivateProviderWindowEnvelope
	head, attempt   string
	retainedContext context.Context
}

func privateMixedPlanV2(q PrivateMixedInputRequestV2) (sessionruntime.PrivateProviderWindowPlan, bool) {
	var p sessionruntime.PrivateProviderWindowPlan
	if q.Manager == nil || q.Root == nil || len(q.Kinds) == 0 || len(q.Kinds) > 3 || q.Start.Deadline.IsZero() {
		return p, false
	}
	for _, s := range []string{q.URI, q.OccurrenceID, q.Revision, q.Privacy, q.OccurrencePolicy, q.CompositionPolicy} {
		if !sessionruntime.PrivateGroupingFieldValid(s) {
			return p, false
		}
	}
	for _, k := range q.Kinds {
		switch k {
		case programcadmission.TypedCalls:
			if p.Calls {
				return p, false
			}
			p.Calls = true
		case programcadmission.TypedReferencesSymbol:
			if p.References {
				return p, false
			}
			p.References = true
		case programcadmission.TypedResolvesToDefinition:
			if p.Definitions {
				return p, false
			}
			p.Definitions = true
		default:
			return p, false
		}
	}
	return p, true
}

// One finite logical quote, NOT physical resource proof or an authorization.
// W is the actual enforced window envelope (includes initialize and shutdown).
// S=8MiB covers TWO ordinary 64-file/1MiB-file/4MiB-pass source acquisitions.
// B=128MiB covers bounded transient native graphs, constituents, composition,
// proof, old private query and eight <=1MiB retained artifacts, including rejecting
// and base64/copy passes. 64 passes cover parsing, normalization, reconstruction,
// hashing, source copies, encoding, re-admission and exact readbacks. J=1024^2
// bounds raw-to-edge and typed join work. Two kernel verifications each admit
// 4096 records/64MiB/16384 work. 16384 method credits cover the unchanged local
// supervisor's finite observation schedule; these are not OS snapshot bytes.
// Original-frame reconstruction uses a component within B, not an added grant:
// three records each detach <=128KiB and <=32 responses (<=384KiB raw, <=512KiB
// base64). Frame decode/ledger/hash scratch fits a 4MiB logical component and
// eight of the 64 passes. The unchanged <=1MiB complete proof preflight refuses
// excess before encoding; raw/encoded forms are not each granted another MiB.
// These are logical upper allowances, not a physical heap/CPU/IO measurement.
// No old D/R quote is reused as a grant for C or startup. All fourteen coordinates
// are charged before Start under the unchanged existing authorization/ceilings.
func privateMixedChargeV2(now, deadline time.Time, w sessionruntime.PrivateProviderWindowBounds) (countersV2, error) {
	if !now.Before(deadline) || deadline.Sub(now) > 15*time.Second || w.WireBytes == 0 || w.WireBytes > 128<<20 {
		return countersV2{}, ErrPrivateMixedInputV2
	}
	const b uint64 = 128 << 20
	const s uint64 = 8 << 20
	const verify uint64 = 2 * (64 << 20)
	data := b + s + w.SourceReadBytes + w.RequestBytes + w.ResponseBytes
	records := uint64(2*4096 + 128*1024 + 8*8)
	calls := uint64(2*16384+16384) + records
	return countersV2{LogicalBytes: 64 * data, ReadHashBytes: verify + 64*data, WriteBytes: 32*(1<<20) + 8*(recordLimitV2+2*headLimitV2), Records: records, ParserBytes: verify + 64*data, ProviderRequests: 16, ProviderMessages: w.Frames, Work: 2*16384 + 64*(data+1024*1024) + 16384, ActiveNS: uint64(deadline.Sub(now)) + uint64(time.Second), Calls: calls, RequestBytes: w.RequestBytes, ResponseBytes: w.ResponseBytes, WireBytes: w.WireBytes, OutputBytes: 8 * (1 << 20)}, nil
}

func (o *PrivateManagedInputOwnerV2) ReserveMixedInputV2(ctx context.Context, attempt string, q PrivateMixedInputRequestV2) (PrivateMixedInputOperationV2, error) {
	stage := privatePrepareReserveV1
	var cause error
	fail := func() (PrivateMixedInputOperationV2, error) {
		return PrivateMixedInputOperationV2{}, privatePrepareFailureAtV1(stage, ErrPrivateMixedInputV2, cause)
	}
	plan, ok := privateMixedPlanV2(q)
	if o == nil || ctx == nil || ctx.Err() != nil || !ok || !hashV2(attempt) {
		if ctx != nil {
			cause = ctx.Err()
		}
		return fail()
	}
	now := time.Now()
	if d, ok := ctx.Deadline(); ok && d.Before(q.Start.Deadline) {
		return fail()
	}
	envelope, err := q.Manager.PrivateProviderWindowEnvelope(q.Start, plan)
	if err != nil {
		stage, cause = privatePrepareWindowV1, err
		return fail()
	}
	deadline := q.Start.Deadline
	if q.Timing != nil {
		frozen := *q.Timing
		q.Timing = &frozen
		if !frozen.Valid() || len(q.Kinds) != 1 || !plan.References || plan.Calls || plan.Definitions || !q.Start.Deadline.Equal(frozen.CaptureDeadline) {
			return fail()
		}
		deadline = frozen.TotalDeadline
		if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
			return fail()
		}
	}
	charge, err := privateMixedChargeV2(now, q.Start.Deadline, envelope.Bounds())
	if q.Timing != nil {
		charge, err = sourceTimingMixedChargeV2(*q.Timing, now, envelope.Bounds(), o.limits)
	}
	if err != nil {
		cause = err
		return fail()
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.k == nil || o.uncertain || q.Root != o.k.root {
		return fail()
	}
	if q.Timing != nil {
		if err = o.sourceTimingFundingV2(ctx, *q.Timing, charge, now); err != nil {
			cause = err
			return fail()
		}
	}
	h, err := o.k.reserve(ctx, o.job, o.head, attempt, charge, deadline.UnixNano(), now.UnixNano(), o.limits)
	if err != nil {
		if errors.Is(err, errUncertainV2) {
			o.uncertain = true
		}
		cause = err
		return fail()
	}
	o.head = h.Identity
	// Freeze mutable request arrays only after confirmed funding.
	q.Kinds = slices.Clone(q.Kinds)
	q.Start.Process.Args = slices.Clone(q.Start.Process.Args)
	q.Start.Process.Env = slices.Clone(q.Start.Process.Env)
	q.Start.RelationProviders = slices.Clone(q.Start.RelationProviders)
	return PrivateMixedInputOperationV2{&privateMixedInputOperationV2{owner: o, request: q, envelope: envelope, head: h.Identity, attempt: attempt, retainedContext: ctx}}, nil
}

// Invocation-local callback checkpoints, never a cross-package observer sink.
// Each checkpoint holds this mutex ONLY for a fixed value assignment, not work.
// Separate caller diagnostic allowance: <=4 callback locks +1 caller lock,
// <=16 fixed record updates including the native decision, four checkpoints,
// frozen caller value and failure attachment; <=256 fixed local steps. These
// separately caller-funded diagnostic limits are NOT native credit.
type privatePrepareCallbackObservationV1 struct {
	mu    sync.Mutex
	value PrivatePrepareLeaseObservationV1
}

func (r *privatePrepareCallbackObservationV1) enter() {
	r.mu.Lock()
	r.value.CallbackEntered = PrivatePrepareObservedYesV1
	r.mu.Unlock()
}
func (r *privatePrepareCallbackObservationV1) captured(result sessionruntime.PrivateGroupingCaptureResult) {
	r.mu.Lock()
	r.value.CaptureResult = result
	r.mu.Unlock()
}
func (r *privatePrepareCallbackObservationV1) called(err error) {
	result := privatePrepareCheckpointV1(err)
	r.mu.Lock()
	r.value.CallsResult = result
	r.mu.Unlock()
}
func (r *privatePrepareCallbackObservationV1) issued(err error) {
	result := privatePrepareCheckpointV1(err)
	r.mu.Lock()
	r.value.IssuanceResult = result
	r.mu.Unlock()
}
func (r *privatePrepareCallbackObservationV1) freeze(native sessionruntime.PrivateLeaseBoundaryObservation) PrivatePrepareLeaseObservationV1 {
	// FIRST caller-side lock after the observed lease returns. Worker progress
	// AFTER native wrapper return but BEFORE this acquisition MAY be included.
	r.mu.Lock()
	copy := r.value
	r.mu.Unlock()
	copy.Boundary, copy.InnerResult = native.Boundary(), native.InnerResult()
	if native.ReturnedResultObserved() {
		copy.ResultObserved = PrivatePrepareObservedYesV1
	} else {
		copy.ResultObserved = PrivatePrepareObservedNoV1
	}
	copy.CallbackSample = PrivatePrepareCallbackCallerPostReturnLockV1
	return copy
}

// ExecuteMixedInputV2 is a fixed internal continuation, not a caller executor.
// The only successful return is AFTER original-lease return, window worker join,
// and confirmed Observe. Failures retain PENDING/uncertain cumulative accounting.
func ExecuteMixedInputV2(ctx context.Context, op PrivateMixedInputOperationV2, q PrivateMixedInputRequestV2) (_ PrivateManagedGroupingInput, failure error) {
	stage := privatePrepareReplayV1
	var leaseObservation PrivatePrepareLeaseObservationV1
	leaseSampled := false
	fail := func(err error) (PrivateManagedGroupingInput, error) {
		failure := privatePrepareFailureAtV1(stage, ErrPrivateMixedInputV2, err)
		if leaseSampled {
			failure = privatePrepareFailureWithLeaseV1(failure, leaseObservation)
		}
		return PrivateManagedGroupingInput{}, failure
	}
	if op.state == nil || ctx == nil {
		return fail(nil)
	}
	s := op.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.used {
		return fail(nil)
	}
	s.used = true
	if s.request.Timing != nil && (!reflect.TypeOf(ctx).Comparable() || ctx != s.retainedContext || s.retainedContext == nil || s.retainedContext.Err() != nil) {
		return fail(nil)
	}
	if ctx.Err() != nil || !reflect.DeepEqual(q, s.request) || !time.Now().Before(q.Start.Deadline) {
		cause := ctx.Err()
		if !time.Now().Before(q.Start.Deadline) {
			cause = context.DeadlineExceeded
		}
		return fail(cause)
	}
	o := s.owner
	o.mu.Lock()
	live := !o.uncertain && o.head == s.head
	o.mu.Unlock()
	if !live {
		return fail(nil)
	}
	execution, cancel := context.WithDeadline(ctx, q.Start.Deadline)
	defer cancel()
	stage = privatePrepareWindowV1
	window, err := q.Manager.PrivateProviderWindowContext(execution, s.envelope, q.Start.Deadline)
	if err != nil {
		return fail(err)
	}
	// Failure cleanup uses the SAME original window; no deadline renewal/reset.
	defer func() {
		_, finishErr := q.Manager.FinishPrivateProviderWindow(window)
		// Retain failure-only cleanup attribution. The old repeated Finish after
		// successful Observe MUST NOT change otherwise successful availability.
		failure = privatePrepareDeferredFinishV1(failure, finishErr)
	}()
	stage = privatePrepareStartV1
	start := q.Manager.Start(window, q.Start)
	if start.Failure != "" {
		cause := execution.Err()
		if start.Failure == "FAILEDUNJOINED" {
			cause = sessionruntime.ErrPrivateProviderWindowUnjoined
		}
		return fail(cause)
	}
	stage = privatePrepareReadinessV1
	pending := q.Manager.BeginReadiness(window, start.SessionID, start.Generation, q.Start.Deadline)
	ready, ok := q.Manager.WaitReadiness(window, pending.ID)
	if !ok || ready.State != sessionruntime.ReadinessReady {
		cause := execution.Err()
		if ready.Failure == "FAILEDUNJOINED" {
			cause = sessionruntime.ErrPrivateProviderWindowUnjoined
		} else if cause == nil && ok && ready.State == sessionruntime.ReadinessFailed && ready.Failure == session.ToolNotImplemented {
			cause = ErrPrivateMixedInputUnavailableV2
		}
		return fail(cause)
	}
	stage = privatePrepareCaptureV1
	session := sessionruntime.SessionLeaseRequest{SessionID: start.SessionID, Generation: start.Generation, Workspace: q.Start.Profile.Workspace().String()}
	plan, _ := privateMixedPlanV2(q)
	grouping := PrivateManagedGroupingRequest{Session: session, URI: q.URI, OccurrenceID: q.OccurrenceID, Revision: q.Revision, Privacy: q.Privacy, OccurrenceAdmissionPolicyID: q.OccurrencePolicy, CompositionPolicyID: q.CompositionPolicy, Line: q.Line, Character: q.Character, Kinds: q.Kinds, Root: q.Root, Deadline: q.Start.Deadline}
	// Private fixed state shares the already-confirmed reservation. No nested
	// Reserve under PENDING and no manual Complete/BindOwned surface is opened.
	binding, err := PrivateManagedGroupingOperationBinding(q.Manager, grouping, "ACQUIRE", sessionruntime.PrivateGroupingOwnedHandoff{})
	if err != nil {
		return fail(err)
	}
	operation := PrivateManagedInputOperationV2{&privateManagedInputOperationV2{owner: o, binding: binding, attempt: s.attempt, head: s.head, used: true, execution: execution, cancel: cancel}}
	grouping.Operation = operation
	defer operation.abortFixed()
	var out PrivateManagedGroupingInput
	checkpoints := privatePrepareCallbackObservationV1{value: PrivatePrepareLeaseObservationV1{CallbackEntered: PrivatePrepareObservedNoV1}}
	if !plan.Calls {
		checkpoints.value.CallsResult = PrivatePrepareCheckpointNotApplicableV1
	}
	var leaseBoundary sessionruntime.PrivateLeaseBoundaryObservation
	err, leaseBoundary = q.Manager.WithReadySessionLeaseObserved(window, session, func(base context.Context) error {
		checkpoints.enter() // first callback statement, not a never-runs claim
		req := sessionruntime.PrivateGroupingOwnedRequest{Session: session, URI: q.URI, OccurrenceID: q.OccurrenceID, Revision: q.Revision, Privacy: q.Privacy, Line: q.Line, Character: q.Character, References: plan.References, Definitions: plan.Definitions, Deadline: q.Start.Deadline}
		var h sessionruntime.PrivateGroupingOwnedHandoff
		var e error
		var captureResult sessionruntime.PrivateGroupingCaptureResult
		if plan.References || plan.Definitions {
			h, e, captureResult = q.Manager.CapturePrivateGroupingOwnedObserved(base, req)
		} else {
			h, e, captureResult = q.Manager.CapturePrivateGroupingQueryOwnedObserved(base, req)
		}
		checkpoints.captured(captureResult)
		if e != nil {
			return e
		}
		if plan.Calls {
			line, ch := q.Line, q.Character
			native, e := constructNativeCallsPrivateV2(base, q.Manager, PrivateNativeCallsRequestV2{Session: session, Revision: q.Revision, Privacy: q.Privacy, Directional: sessionruntime.DirectionalCallHierarchyRequestPrivate{SessionID: session.SessionID, Generation: session.Generation, URI: q.URI, LanguageID: q.Start.LanguageID, Line: &line, Character: &ch, Deadline: q.Start.Deadline, MaxMessages: 4, MaxBytes: 65536}}, true)
			checkpoints.called(e)
			if e != nil {
				return e
			}
			grouping.Calls = native.Admission.Admission
			grouping.nativeProof = native.nativeEvidence
		}
		out, e = issuePrivateManagedGroupingInput(base, q.Manager, h, grouping)
		checkpoints.issued(e)
		return e
	})
	leaseObservation = checkpoints.freeze(leaseBoundary)
	leaseSampled = true
	if err != nil {
		return fail(err)
	}
	if len(out.Ledger) == 0 {
		for _, kind := range q.Kinds {
			method := "CALLS"
			if kind == programcadmission.TypedReferencesSymbol {
				method = "textDocument/references"
			}
			if kind == programcadmission.TypedResolvesToDefinition {
				method = "textDocument/definition"
			}
			if out.Dispositions[method] == "UNAVAILABLE" {
				return fail(ErrPrivateMixedInputUnavailableV2)
			}
		}
		return fail(ErrPrivateMixedInputEmptyV2)
	}
	stage = privatePrepareFinishV1
	completion, err := q.Manager.FinishPrivateProviderWindow(window)
	if err != nil || !completion.Completed || q.Manager.Census().Workers != 0 {
		if err == nil {
			err = sessionruntime.ErrPrivateProviderWindowUnjoined
		}
		return fail(err)
	}
	if !sourceclock.Now(ctx, "capture-complete").Before(q.Start.Deadline) {
		return fail(context.DeadlineExceeded)
	}
	stage = privatePrepareObserveV1
	if err = operation.completeFixed(execution); err != nil {
		return fail(err)
	}
	// The source token remained private through lease/window join and confirmed
	// Observe. No failed or uncertain producer can expose source originals.
	out.sourceConfirmed = true
	if q.Timing != nil {
		f := *q.Timing
		prep := time.Now().Add(time.Duration(f.Profile.PreparationMS) * time.Millisecond)
		cutoff := f.TotalDeadline.Add(-time.Duration(f.Profile.HOSTMS+f.Profile.PublicationCleanupMS) * time.Millisecond)
		if cutoff.Before(prep) {
			prep = cutoff
		}
		out.sourceTiming = &privateSourceTimingStateV2{frame: f, preparationDeadline: prep, retainedContext: s.retainedContext}
	}
	// The initial exact replay above is paid by this mixed reservation. A later
	// replay must not silently reuse the old D/R-only operation quote.
	if plan.Calls {
		out.replayBinding.Mode = "MIXED_NATIVE_REPLAY_UNQUOTED"
	}
	return out, nil
}
