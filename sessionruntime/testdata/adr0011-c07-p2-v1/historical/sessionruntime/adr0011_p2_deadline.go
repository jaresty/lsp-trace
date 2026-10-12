package sessionruntime

import (
	"context"
	"time"
)

const (
	privateP2RequestLimit = 15 * time.Second
	privateP2AttemptLimit = 60 * time.Second
)

type privateP2DeadlineKind uint8

const (
	privateP2NoDeadline privateP2DeadlineKind = iota
	privateP2CallerDeadline
	privateP2RequestDeadline
	privateP2AttemptDeadline
)

func privateP2EffectiveDeadline(requestStart, attemptStart, requestDeadline, callerDeadline time.Time) (time.Time, privateP2DeadlineKind) {
	deadline, kind := requestStart.Add(privateP2RequestLimit), privateP2RequestDeadline
	attempt := attemptStart.Add(privateP2AttemptLimit)
	if !attempt.After(deadline) { // attempt wins equality.
		deadline, kind = attempt, privateP2AttemptDeadline
	}
	if !requestDeadline.IsZero() && requestDeadline.Before(deadline) {
		deadline, kind = requestDeadline, privateP2RequestDeadline
	}
	if !callerDeadline.IsZero() && callerDeadline.Before(deadline) {
		deadline, kind = callerDeadline, privateP2CallerDeadline
	}
	return deadline, kind
}

func privateP2MayStartBlocking(now, deadline time.Time) bool {
	return deadline.IsZero() || now.Before(deadline)
}

func privateP2ReachedFailure(ctx context.Context, now, deadline time.Time, kind privateP2DeadlineKind) privateP2DeadlineKind {
	if !privateP2MayStartBlocking(now, deadline) {
		return kind
	}
	if ctx != nil && ctx.Err() != nil {
		return privateP2NoDeadline
	}
	return privateP2NoDeadline
}
