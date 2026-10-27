package censuslineagepilot

import (
	"context"
	"errors"
	"testing"
)

func TestScopedExecutionClaimPrivateInvalidFirstAttemptIsSpent(t *testing.T) {
	for _, name := range []string{"nil-context", "nil-store"} {
		t.Run(name, func(t *testing.T) {
			f := executionClaimFixture(t)
			o := f.owner(t)
			s := &executionClaimMemoryPrivate{}
			var err error
			if name == "nil-context" {
				_, err = o.claim(nil, s)
			} else {
				_, err = o.claim(context.Background(), nil)
			}
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("first attempt: %v", err)
			}
			out, err := o.claim(context.Background(), s)
			if !errors.Is(err, ErrIndeterminateExecution) || out.pending != nil || s.writes != 0 {
				t.Fatalf("invalid attempt allowed retry: outcome=%s writes=%d err=%v", out.outcome, s.writes, err)
			}
		})
	}
}
