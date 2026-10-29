package sessionruntime

import (
	"time"

	"lsp-trace/internal/managedprocess"
)

// CleanupStepPrivateV2 is a closed observation, not cleanup authority. Zero is
// unavailable/not attempted, never success. No provider-controlled text survives.
type CleanupStepPrivateV2 string

const (
	CleanupSucceededPrivateV2 CleanupStepPrivateV2 = "SUCCEEDED"
	CleanupFailedPrivateV2    CleanupStepPrivateV2 = "FAILED"
)

type ShutdownReadPrivateV2 string

const (
	ShutdownReadFailedPrivateV2 ShutdownReadPrivateV2 = "READ_FAILED"
	ShutdownTimeoutPrivateV2    ShutdownReadPrivateV2 = "TIMEOUT"
	ShutdownCancelledPrivateV2  ShutdownReadPrivateV2 = "CANCELLED"
	ShutdownUnexpectedPrivateV2 ShutdownReadPrivateV2 = "UNEXPECTED_FRAME"
	ShutdownMatchedPrivateV2    ShutdownReadPrivateV2 = "MATCHED_SUCCESS"
	ShutdownCountLimitPrivateV2 ShutdownReadPrivateV2 = "COUNT_LIMIT"
	ShutdownByteLimitPrivateV2  ShutdownReadPrivateV2 = "BYTE_LIMIT"
)

// LifecycleCleanupObservationPrivateV2 is a bounded scalar value. It is available
// only through the explicitly private snapshot accessor, not Operation JSON,
// ordinary Native/V1 gates, or another Manager call. Empty kinds are unknown.
// RetiredTransportReused records reuse of earlier close/death/reap observations;
// it does not assert that a shutdown exchange occurred for this operation.
// StderrDrainAssumed labels the existing algebra input; no drain proof exists.
type LifecycleCleanupObservationPrivateV2 struct {
	// Durations describe only the response phase, never a global cleanup bound.
	DeclaredResponseBudget      time.Duration
	EffectiveResponseReadBudget time.Duration
	ResponseBudgetSource        string // closed producer vocabulary: ORDINARY_FIXED or PRIVATE_V2
	ShutdownWrite               CleanupStepPrivateV2
	ShutdownRead                ShutdownReadPrivateV2
	ExitWrite                   CleanupStepPrivateV2
	// Counts cover complete validated frames only, not partial, malformed,
	// or over-budget bytes acquired before refusal. They are not wire totals.
	ReadFrames                           int
	ReadBytes                            int64
	RetiredTransportReused               bool
	TeardownAttempted                    bool
	ResourceCloseAttempted               bool
	ResourceClose                        managedprocess.ResourceKind
	Death                                managedprocess.DeathKind
	Reap                                 managedprocess.ReapKind
	ResourceError, DeathError, ReapError bool
	StderrDrainAssumed                   bool
}

// CleanupObservationPrivateV2 returns an independent value copy from this exact
// immutable operation result. Pending/absent metadata is not cleanup success,
// authentication, provenance, or permission to execute again.
func (o OperationSnapshot) CleanupObservationPrivateV2() (LifecycleCleanupObservationPrivateV2, bool) {
	return o.cleanupPrivateV2, o.cleanupPrivateV2Present
}

func cleanupObservationPrivateV2(f LifecycleCleanupObservationPrivateV2, t managedprocess.TeardownObservation, r managedprocess.ResourceObservation, reused bool) LifecycleCleanupObservationPrivateV2 {
	f.RetiredTransportReused = reused
	f.TeardownAttempted, f.ResourceCloseAttempted = !reused, !reused
	switch r.Kind {
	case managedprocess.ResourcesClosed, managedprocess.ResourcesCloseFailed:
		f.ResourceClose = r.Kind
	}
	switch t.Death.Kind {
	case managedprocess.DeathExited, managedprocess.DeathSignaled, managedprocess.DeathUnknown:
		f.Death = t.Death.Kind
	}
	switch t.Death.Reap.Kind {
	case managedprocess.ReapComplete, managedprocess.ReapFailed:
		f.Reap = t.Death.Reap.Kind
	}
	f.ResourceError, f.DeathError, f.ReapError = r.Err != nil, t.Death.Err != nil, t.Death.Reap.Err != nil
	f.StderrDrainAssumed = true
	return f
}
