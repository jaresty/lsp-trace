// Package lifecycleops provides transport-neutral lifecycle projections.
package lifecycleops

import (
	"context"
	"sort"

	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

type Failure string

const (
	FailureNone                   Failure = ""
	FailureSessionNotFound        Failure = "SESSION_NOT_FOUND"
	FailureStaleGeneration        Failure = "STALE_GENERATION"
	FailureOperationNotFound      Failure = "OPERATION_NOT_FOUND"
	FailureCapacityExhausted      Failure = "CAPACITY_EXHAUSTED"
	FailureContainmentUnavailable Failure = "CONTAINMENT_UNAVAILABLE"
	FailureReapIncomplete         Failure = "REAP_INCOMPLETE"
	FailureLifecycleConflict      Failure = "LIFECYCLE_CONFLICT"
	FailureInternal               Failure = "INTERNAL"
)

type OperationState string

const (
	Pending  OperationState = "PENDING"
	Complete OperationState = "COMPLETE"
	Failed   OperationState = "FAILED"
)

type Runtime interface {
	Records() []sessionruntime.Record
	Observations() []sessionruntime.Observation
	Census() sessionruntime.Census
	Operation(string) (sessionruntime.OperationSnapshot, bool)
	Stop(context.Context, string, string) session.LifecycleResult
	Restart(context.Context, string, string) session.LifecycleResult
}

type Service struct{ runtime Runtime }

type ListSnapshot struct {
	Sessions     []sessionruntime.Record
	Observations []sessionruntime.Observation
	Census       sessionruntime.Census
}
type LifecycleRequest struct {
	SessionID  string
	Generation uint64
	CallerID   string
}
type Acceptance struct {
	OperationID               string
	Generation                uint64
	State                     session.State
	Pending, Joined, Replayed bool
	Failure                   Failure
}
type OperationSnapshot struct {
	ID, SessionID string
	Generation    uint64
	Restart       bool
	State         OperationState
	Failure       Failure
}

func New(runtime Runtime) *Service { return &Service{runtime: runtime} }

func (s *Service) List() ListSnapshot {
	records := append([]sessionruntime.Record(nil), s.runtime.Records()...)
	sort.Slice(records, func(i, j int) bool { return records[i].SessionID < records[j].SessionID })
	observations := append([]sessionruntime.Observation(nil), s.runtime.Observations()...)
	sort.Slice(observations, func(i, j int) bool { return observations[i].Sequence < observations[j].Sequence })
	return ListSnapshot{Sessions: records, Observations: observations, Census: s.runtime.Census()}
}

func (s *Service) Status(id string, generation uint64) (sessionruntime.Record, Failure) {
	for _, record := range s.runtime.Records() {
		if record.SessionID != id {
			continue
		}
		if record.Generation != generation {
			return sessionruntime.Record{}, FailureStaleGeneration
		}
		return record, FailureNone
	}
	return sessionruntime.Record{}, FailureSessionNotFound
}

func (s *Service) Stop(ctx context.Context, request LifecycleRequest) Acceptance {
	return s.accept(ctx, request, false)
}

func (s *Service) Restart(ctx context.Context, request LifecycleRequest) Acceptance {
	return s.accept(ctx, request, true)
}

func (s *Service) accept(ctx context.Context, request LifecycleRequest, restart bool) Acceptance {
	if _, failure := s.Status(request.SessionID, request.Generation); failure != FailureNone {
		return Acceptance{Failure: failure}
	}
	var result session.LifecycleResult
	if restart {
		result = s.runtime.Restart(ctx, request.SessionID, request.CallerID)
	} else {
		result = s.runtime.Stop(ctx, request.SessionID, request.CallerID)
	}
	return Acceptance{
		OperationID: result.IntentID,
		Generation:  result.Generation,
		State:       result.State,
		Pending:     result.IntentID != "" && result.Failure == "" && !result.Noop,
		Joined:      result.Joined,
		Replayed:    result.Replayed,
		Failure:     mapFailure(result.Failure),
	}
}

func (s *Service) OperationStatus(id string) (OperationSnapshot, Failure) {
	operation, ok := s.runtime.Operation(id)
	if !ok {
		return OperationSnapshot{}, FailureOperationNotFound
	}
	return OperationSnapshot{
		ID:         operation.ID,
		SessionID:  operation.SessionID,
		Generation: operation.Generation,
		Restart:    operation.Restart,
		State:      OperationState(operation.State),
		Failure:    mapFailure(operation.Failure),
	}, FailureNone
}

func mapFailure(failure session.Failure) Failure {
	switch failure {
	case "":
		return FailureNone
	case session.SessionNotFound:
		return FailureSessionNotFound
	case session.StaleGeneration:
		return FailureStaleGeneration
	case session.ResourceExhausted:
		return FailureCapacityExhausted
	case session.ProcessContainmentUnavailable:
		return FailureContainmentUnavailable
	case session.SessionReapIncomplete:
		return FailureReapIncomplete
	case session.LifecycleConflict:
		return FailureLifecycleConflict
	default:
		return FailureInternal
	}
}
