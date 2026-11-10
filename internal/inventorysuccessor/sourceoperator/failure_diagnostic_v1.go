//go:build darwin || linux

package sourceoperator

import (
	"context"
	"errors"
)

// FailureStageV1 is a closed, source-free operation category, not execution credit.
type FailureStageV1 uint8

const (
	FailureUnknownV1 FailureStageV1 = iota
	FailureGuardV1
	FailurePublicationOpenV1
	FailureCaptureReserveV1
	FailureCaptureExecuteV1
	FailurePreparationReserveV1
	FailurePreparationExecuteV1
	FailureOperatorV1
	FailurePublicationV1
)

func (s FailureStageV1) String() string {
	switch s {
	case FailureGuardV1:
		return "guard"
	case FailurePublicationOpenV1:
		return "publication_open"
	case FailureCaptureReserveV1:
		return "capture_reserve"
	case FailureCaptureExecuteV1:
		return "capture_execute"
	case FailurePreparationReserveV1:
		return "preparation_reserve"
	case FailurePreparationExecuteV1:
		return "preparation_execute"
	case FailureOperatorV1:
		return "operator"
	case FailurePublicationV1:
		return "publication"
	default:
		return "unknown"
	}
}

// FailureClassV1 never formats or retains an error message.
type FailureClassV1 uint8

const (
	FailureNoneV1 FailureClassV1 = iota
	FailureOtherV1
	FailureCanceledV1
	FailureDeadlineV1
	FailureRefusedV1
	FailureAggregateV1
)

func ClassifyFailureV1(err error) FailureClassV1 {
	switch {
	case err == nil:
		return FailureNoneV1
	case errors.Is(err, context.DeadlineExceeded):
		return FailureDeadlineV1
	case errors.Is(err, context.Canceled):
		return FailureCanceledV1
	case errors.Is(err, ErrNativeAggregate):
		return FailureAggregateV1
	case errors.Is(err, ErrRefused):
		return FailureRefusedV1
	default:
		return FailureOtherV1
	}
}
func (c FailureClassV1) String() string {
	switch c {
	case FailureNoneV1:
		return "none"
	case FailureCanceledV1:
		return "canceled"
	case FailureDeadlineV1:
		return "deadline_exceeded"
	case FailureRefusedV1:
		return "refused"
	case FailureAggregateV1:
		return "native_aggregate"
	default:
		return "other"
	}
}

// FailureDiagnosticV1 is in-process metadata only. It contains no raw error.
type FailureDiagnosticV1 struct {
	Stage                                       FailureStageV1
	Class                                       FailureClassV1
	PublicationClose, PendingFinalize, Terminal bool
}

func (o Outcome) FailureDiagnosticV1() FailureDiagnosticV1 { return o.failure }
func (o *Outcome) failV1(stage FailureStageV1, err error) error {
	if err != nil && o.failure.Stage == FailureUnknownV1 {
		o.failure.Stage, o.failure.Class = stage, ClassifyFailureV1(err)
	}
	return err
}
