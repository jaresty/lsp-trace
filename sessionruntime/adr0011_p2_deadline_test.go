package sessionruntime

import (
	"context"
	"testing"
	"time"
)

func TestC07C08PrivateP2DeadlineSelection(t *testing.T) {
	base := time.Unix(100, 0)
	cases := []struct {
		name                                                             string
		now, requestStart, attemptStart, requestDeadline, callerDeadline time.Time
		want                                                             privateP2DeadlineKind
		allow                                                            bool
	}{
		{"request equality", base.Add(15 * time.Second), base, base, time.Time{}, time.Time{}, privateP2RequestDeadline, false},
		{"request plus one", base.Add(15*time.Second + time.Nanosecond), base, base, time.Time{}, time.Time{}, privateP2RequestDeadline, false},
		{"attempt equality", base.Add(60 * time.Second), base.Add(50 * time.Second), base, time.Time{}, time.Time{}, privateP2AttemptDeadline, false},
		{"attempt plus one", base.Add(60*time.Second + time.Nanosecond), base.Add(50 * time.Second), base, time.Time{}, time.Time{}, privateP2AttemptDeadline, false},
		{"earlier caller", base.Add(5 * time.Second), base, base, time.Time{}, base.Add(5 * time.Second), privateP2CallerDeadline, false},
		{"before all", base.Add(5 * time.Second), base, base, time.Time{}, time.Time{}, privateP2RequestDeadline, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			deadline, kind := privateP2EffectiveDeadline(tc.requestStart, tc.attemptStart, tc.requestDeadline, tc.callerDeadline)
			if kind != tc.want || privateP2MayStartBlocking(tc.now, deadline) != tc.allow {
				t.Fatalf("ASSERT_C07_C08_DEADLINE kind=%v allow=%v deadline=%v", kind, privateP2MayStartBlocking(tc.now, deadline), deadline)
			}
		})
	}
}

func TestC07PrivateP2DeadlineWinsSimultaneousCancellation(t *testing.T) {
	base := time.Unix(100, 0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	deadline, kind := privateP2EffectiveDeadline(base, base, base.Add(15*time.Second), time.Time{})
	if got := privateP2ReachedFailure(ctx, base.Add(15*time.Second), deadline, kind); got != privateP2RequestDeadline {
		t.Fatalf("ASSERT_C07_DEADLINE_PRECEDENCE got=%v", got)
	}
}
