package inventorysuccessor

import (
	"context"
	"errors"
	"lsp-trace/sessionruntime"
)

// PrivatePrepareStageV1 is DATA, not authority or evidence that an operation
// committed. Only fixed values can reach the CLI projection.
type PrivatePrepareStageV1 uint8

const (
	PrivatePrepareStageNoneV1 PrivatePrepareStageV1 = iota
	privatePrepareReserveV1
	privatePrepareReplayV1
	privatePrepareWindowV1
	privatePrepareStartV1
	privatePrepareReadinessV1
	privatePrepareCaptureV1
	privatePrepareFinishV1
	privatePrepareObserveV1
	privatePrepareRprepV1
	privatePreparePersistV1
	privatePreparePacketV1
)

func (s PrivatePrepareStageV1) String() string {
	switch s {
	case privatePrepareReserveV1:
		return "RESERVE"
	case privatePrepareReplayV1:
		return "REPLAY"
	case privatePrepareWindowV1:
		return "WINDOW"
	case privatePrepareStartV1:
		return "START"
	case privatePrepareReadinessV1:
		return "READINESS"
	case privatePrepareCaptureV1:
		return "CAPTURE"
	case privatePrepareFinishV1:
		return "FINISH"
	case privatePrepareObserveV1:
		return "OBSERVE"
	case privatePrepareRprepV1:
		return "RPREP"
	case privatePreparePersistV1:
		return "PERSIST"
	case privatePreparePacketV1:
		return "PACKET"
	default:
		return "NONE"
	}
}

type PrivatePrepareReasonV1 uint8

const (
	privatePrepareNoReasonV1 PrivatePrepareReasonV1 = iota
	privatePrepareDeadlineV1
	privatePrepareCanceledV1
	privatePrepareRefusedV1
	privatePrepareUnavailableV1
	privatePrepareEmptyV1
	privatePrepareUnjoinedV1
	privatePrepareCommitUncertainV1
	privatePrepareAutoReasonV1 PrivatePrepareReasonV1 = 255
)

func (r PrivatePrepareReasonV1) String() string {
	switch r {
	case privatePrepareNoReasonV1:
		return "NONE"
	case privatePrepareDeadlineV1:
		return "DEADLINE"
	case privatePrepareCanceledV1:
		return "CANCELED"
	case privatePrepareUnavailableV1:
		return "UNAVAILABLE"
	case privatePrepareEmptyV1:
		return "EMPTY"
	case privatePrepareUnjoinedV1:
		return "UNJOINED"
	case privatePrepareCommitUncertainV1:
		return "COMMIT_UNCERTAIN"
	default:
		return "REFUSED"
	}
}

// Closed observation codes are DATA. Unknown constructed enum values normalize
// to UNKNOWN, never authority. UNOBSERVED does not mean a failed or stalled step.
type PrivatePrepareObservedV1 uint8

const (
	PrivatePrepareObservedUnknownV1 PrivatePrepareObservedV1 = iota
	PrivatePrepareObservedYesV1
	PrivatePrepareObservedNoV1
)

func (v PrivatePrepareObservedV1) String() string {
	switch v {
	case PrivatePrepareObservedYesV1:
		return "YES"
	case PrivatePrepareObservedNoV1:
		return "NO"
	default:
		return "UNKNOWN"
	}
}

type PrivatePrepareCallbackSampleV1 uint8

const (
	PrivatePrepareCallbackUnobservedV1 PrivatePrepareCallbackSampleV1 = iota
	PrivatePrepareCallbackCallerPostReturnLockV1
)

func (v PrivatePrepareCallbackSampleV1) String() string {
	switch v {
	case PrivatePrepareCallbackUnobservedV1:
		return "UNOBSERVED"
	case PrivatePrepareCallbackCallerPostReturnLockV1:
		return "CALLER_POST_RETURN_LOCK"
	default:
		return "UNKNOWN"
	}
}

type PrivatePrepareCheckpointV1 uint8

const (
	PrivatePrepareCheckpointUnobservedV1 PrivatePrepareCheckpointV1 = iota
	PrivatePrepareCheckpointOKV1
	PrivatePrepareCheckpointUnknownV1
	PrivatePrepareCheckpointDeadlineV1
	PrivatePrepareCheckpointCanceledV1
	PrivatePrepareCheckpointUnjoinedV1
	PrivatePrepareCheckpointUnavailableV1
	PrivatePrepareCheckpointEmptyV1
	PrivatePrepareCheckpointRefusedV1
	PrivatePrepareCheckpointNotApplicableV1
)

func (v PrivatePrepareCheckpointV1) String() string {
	switch v {
	case PrivatePrepareCheckpointUnobservedV1:
		return "UNOBSERVED"
	case PrivatePrepareCheckpointOKV1:
		return "OK"
	case PrivatePrepareCheckpointDeadlineV1:
		return "DEADLINE"
	case PrivatePrepareCheckpointCanceledV1:
		return "CANCELED"
	case PrivatePrepareCheckpointUnjoinedV1:
		return "KNOWN_UNJOINED"
	case PrivatePrepareCheckpointUnavailableV1:
		return "UNAVAILABLE"
	case PrivatePrepareCheckpointEmptyV1:
		return "EMPTY"
	case PrivatePrepareCheckpointRefusedV1:
		return "REFUSED"
	case PrivatePrepareCheckpointNotApplicableV1:
		return "NOT_APPLICABLE"
	default:
		return "UNKNOWN"
	}
}

func privatePrepareCheckpointV1(err error) PrivatePrepareCheckpointV1 {
	switch err {
	case nil:
		return PrivatePrepareCheckpointOKV1
	case context.DeadlineExceeded:
		return PrivatePrepareCheckpointDeadlineV1
	case context.Canceled:
		return PrivatePrepareCheckpointCanceledV1
	case sessionruntime.ErrPrivateProviderWindowUnjoined:
		return PrivatePrepareCheckpointUnjoinedV1
	case ErrPrivateMixedInputUnavailableV2:
		return PrivatePrepareCheckpointUnavailableV1
	case ErrPrivateMixedInputEmptyV2:
		return PrivatePrepareCheckpointEmptyV1
	case ErrPrivateMixedInputV2, ErrPreparedDescribeV1, ErrPrivateManagedInputOperationV2:
		return PrivatePrepareCheckpointRefusedV1
	default:
		return PrivatePrepareCheckpointUnknownV1
	}
}

// Eight new fixed scalar slots plus stage/reason/six cleanup slots = SIXTEEN.
// Native sampling is at wrapper decision; callback sampling is ONLY at caller's
// first post-return mutex acquisition and can include intervening worker progress.
type PrivatePrepareLeaseObservationV1 struct {
	Boundary                    sessionruntime.PrivateLeaseBoundary
	ResultObserved              PrivatePrepareObservedV1
	InnerResult                 sessionruntime.PrivateLeaseResult
	CallbackSample              PrivatePrepareCallbackSampleV1
	CallbackEntered             PrivatePrepareObservedV1
	CaptureResult               sessionruntime.PrivateGroupingCaptureResult
	IssuanceResult, CallsResult PrivatePrepareCheckpointV1
}

func privatePrepareFailureWithLeaseV1(err error, observation PrivatePrepareLeaseObservationV1) error {
	p, ok := err.(*privatePrepareFailureV1)
	if !ok || p == nil || p.codes.Lease != (PrivatePrepareLeaseObservationV1{}) {
		return err // preserve an already scoped observation, stage and reason
	}
	copy := *p
	copy.codes.Lease = observation
	return &copy
}

// Six fixed cleanup slots; no raw error/path/source/custody data is exported.
// The value is detached; modifying it cannot mutate a retained error.
type PrivatePrepareFailureCodesV1 struct {
	Stage   PrivatePrepareStageV1
	Reason  PrivatePrepareReasonV1
	Cleanup [6]PrivatePrepareReasonV1 // deferred Finish, Shutdown, data, root, canceled, deadline
	Lease   PrivatePrepareLeaseObservationV1
}

// Immutable after construction. broad is always a package-owned static sentinel.
// reasonSource is compared ONLY to fixed native sentinels, never traversed.
// cause remains private and is exposed solely via Go error identity traversal;
// neither the projection nor Error serializes/traverses the cause graph.
type privatePrepareFailureV1 struct {
	codes        PrivatePrepareFailureCodesV1
	broad        error
	cause        error
	reasonSource error
}

func (e *privatePrepareFailureV1) Error() string {
	if e.broad == ErrPrivateMixedInputV2 {
		return ErrPrivateMixedInputV2.Error()
	}
	if e.broad == ErrPrivateManagedInputOperationV2 {
		return ErrPrivateManagedInputOperationV2.Error()
	}
	return ErrPreparedDescribeV1.Error()
}
func (e *privatePrepareFailureV1) Unwrap() []error { return []error{e.broad, e.cause} }

func privatePrepareReasonV1(err error) PrivatePrepareReasonV1 {
	// Direct fixed-sentinel comparisons have bounded cost even for an opaque error
	// with hostile Error/Is/Unwrap methods. Wrapped unknown causes stay REFUSED.
	switch err {
	case nil:
		return privatePrepareNoReasonV1
	case context.DeadlineExceeded:
		return privatePrepareDeadlineV1
	case context.Canceled:
		return privatePrepareCanceledV1
	case ErrPrivateMixedInputUnavailableV2:
		return privatePrepareUnavailableV1
	case ErrPrivateMixedInputEmptyV2:
		return privatePrepareEmptyV1
	case sessionruntime.ErrPrivateProviderWindowUnjoined:
		return privatePrepareUnjoinedV1
	case errUncertainV2:
		return privatePrepareCommitUncertainV1
	default:
		return privatePrepareRefusedV1
	}
}
func privatePrepareFailureAtV1(stage PrivatePrepareStageV1, broad, cause error) error {
	p := &privatePrepareFailureV1{codes: PrivatePrepareFailureCodesV1{Stage: stage, Reason: privatePrepareAutoReasonV1}, broad: broad, cause: cause, reasonSource: cause}
	if inner, ok := cause.(*privatePrepareFailureV1); ok && inner != nil {
		p.codes = inner.codes
		p.reasonSource = inner.reasonSource
	}
	return p
}

// ProjectPrivatePrepareFailureV1 inspects at most ONE closed diagnostic node and
// sixteen fixed scalar slots (within 32 nodes/16 slots/256 fixed steps). It does not
// recursively walk errors.Join, custom wrappers or arbitrary error graphs.
func ProjectPrivatePrepareFailureV1(err error) PrivatePrepareFailureCodesV1 {
	if err == nil {
		return PrivatePrepareFailureCodesV1{}
	}
	p, ok := err.(*privatePrepareFailureV1)
	if !ok || p == nil {
		return PrivatePrepareFailureCodesV1{Reason: privatePrepareRefusedV1}
	}
	out := p.codes
	if out.Reason == privatePrepareAutoReasonV1 {
		out.Reason = privatePrepareReasonV1(p.reasonSource)
		if out.Reason == privatePrepareNoReasonV1 {
			out.Reason = privatePrepareRefusedV1
		}
	}
	return out
}
func privatePrepareDeferredFinishV1(err, finish error) error {
	if err == nil || finish == nil {
		return err
	} // never alter old successful availability
	codes := ProjectPrivatePrepareFailureV1(err)
	codes.Cleanup[0] = privatePrepareReasonV1(finish)
	return &privatePrepareFailureV1{codes: codes, broad: ErrPrivateMixedInputV2, cause: errors.Join(err, finish)}
}

// PrivatePrepareTerminalFailureV1 decorates an ALREADY failed terminal result.
// It neither decides success nor calls cleanup, Observe, a writer, or a clock.
// execution is separate from terminal so errors.Join order cannot overwrite the
// first scoped stage. Cleanup-only failures retain primary NONE/NONE.
func PrivatePrepareTerminalFailureV1(execution, terminal, shutdown, data, root, canceled, deadline error) error {
	if terminal == nil {
		return nil
	}
	codes := ProjectPrivatePrepareFailureV1(execution)
	for i, e := range [5]error{shutdown, data, root, canceled, deadline} {
		if e != nil {
			codes.Cleanup[i+1] = privatePrepareReasonV1(e)
		}
	}
	return &privatePrepareFailureV1{codes: codes, broad: ErrPreparedDescribeV1, cause: terminal}
}
