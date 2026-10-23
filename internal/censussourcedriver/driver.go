// Package censussourcedriver repeats bounded authoritative source transitions.
// It deliberately owns no discovery, planning, persistence, or publication.
package censussourcedriver

import (
	"context"
	"errors"
)

// Status classifies the authoritative result of one source transition.
type Status string

const (
	StatusCommitted              Status = "COMMITTED"
	StatusRetained               Status = "RETAINED"
	StatusConflict               Status = "CONFLICT"
	StatusIndeterminateExecution Status = "INDETERMINATE_EXECUTION"
)

// State is the authoritative continuation state before or after a Step.
// PlanIdentity and the optional manager identity are immutable.
type State struct {
	Status            Status
	CommittedPrefix   int
	PartitionCount    int
	PlanIdentity      string
	SessionID         string
	ManagerGeneration uint64
}

// Transition performs at most one source transition from current.
type Transition func(ctx context.Context, current State) (State, error)

// Request configures one bounded sequential repetition.
type Request struct {
	Initial State
	Bound   int
	Step    Transition
}

// Stop classifies why repetition ended.
type Stop string

const (
	StopComplete               Stop = "COMPLETE"
	StopBoundReached           Stop = "BOUND_REACHED"
	StopDeadline               Stop = "DEADLINE"
	StopConflict               Stop = "CONFLICT"
	StopRetained               Stop = "RETAINED"
	StopIndeterminateExecution Stop = "INDETERMINATE_EXECUTION"
	StopFailed                 Stop = "FAILED"
)

// Result reports the last accepted authoritative state and repetition counts.
// Steps counts calls to Step. Transitions counts committed forward transitions.
type Result struct {
	Stop        Stop
	State       State
	Steps       int
	Transitions int
	Err         error
}

// ErrInvalid reports an invalid request or an impossible Step result.
var ErrInvalid = errors.New("invalid census source transition")

// Repeat calls Step synchronously until completion or a terminal stop.
func Repeat(ctx context.Context, req Request) Result {
	result := Result{State: req.Initial}
	if ctx == nil || req.Bound <= 0 || req.Step == nil || !validInitial(req.Initial) {
		return failed(result, ErrInvalid)
	}
	if complete(req.Initial) {
		result.Stop = StopComplete
		return result
	}
	if err := ctx.Err(); err != nil {
		result.Stop = StopDeadline
		result.Err = err
		return result
	}

	for result.Steps < req.Bound {
		if err := ctx.Err(); err != nil {
			result.Stop = StopDeadline
			result.Err = err
			return result
		}

		previous := result.State
		next, err := req.Step(ctx, previous)
		result.Steps++
		if err != nil {
			return failed(result, err)
		}
		if !validNext(previous, next) {
			return failed(result, ErrInvalid)
		}

		result.State = next
		switch next.Status {
		case StatusCommitted:
			result.Transitions++
			if complete(next) {
				result.Stop = StopComplete
				return result
			}
		case StatusConflict:
			result.Stop = StopConflict
			return result
		case StatusRetained:
			result.Stop = StopRetained
			return result
		case StatusIndeterminateExecution:
			result.Stop = StopIndeterminateExecution
			return result
		}
	}

	result.Stop = StopBoundReached
	return result
}

func complete(state State) bool {
	return state.Status == StatusCommitted && state.CommittedPrefix == state.PartitionCount
}

func validInitial(state State) bool {
	return state.Status == StatusCommitted && validRange(state)
}

func validNext(previous, next State) bool {
	if !validRange(next) || next.PartitionCount != previous.PartitionCount || next.PlanIdentity != previous.PlanIdentity || next.SessionID != previous.SessionID || next.ManagerGeneration != previous.ManagerGeneration {
		return false
	}
	switch next.Status {
	case StatusCommitted:
		return next.CommittedPrefix > previous.CommittedPrefix
	case StatusConflict, StatusRetained, StatusIndeterminateExecution:
		return next.CommittedPrefix == previous.CommittedPrefix
	default:
		return false
	}
}

func validRange(state State) bool {
	identityValid := state.SessionID == "" && state.ManagerGeneration == 0 || state.SessionID != "" && state.ManagerGeneration > 0
	return identityValid && state.PartitionCount >= 0 && state.CommittedPrefix >= 0 && state.CommittedPrefix <= state.PartitionCount
}

func failed(result Result, err error) Result {
	result.Stop = StopFailed
	result.Err = err
	return result
}
