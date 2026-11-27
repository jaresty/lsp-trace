package publication

import (
	"errors"
	"testing"
)

func TestPublicationCompletionResultCombination(t *testing.T) {
	priorCause := errors.New("prior cause")
	quotaCause := errors.New("quota completion cause")
	for _, tc := range []struct {
		name      string
		prior     Result
		committed bool
	}{
		{"uncommitted", failure("select", CodeOutputSelectorUnsafe, true, false, priorCause), false},
		{"committed-without-receipt", failure("reread", CodePublicationFailed, true, true, priorCause), true},
		{"receipt", Result{Receipt: &Receipt{Digest: "verified"}}, true},
		{"receipt-and-failure", Result{Receipt: &Receipt{}, Failure: &Failure{cause: priorCause}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("no-completion-error", func(t *testing.T) {
				if got := withStorageCompletion(tc.prior, nil); got != tc.prior {
					t.Fatal("NIL_COMPLETION_CHANGED_RESULT", got, tc.prior)
				}
			})
			t.Run("completion-error", func(t *testing.T) {
				got := withStorageCompletion(tc.prior, quotaCause)
				if got.Failure == nil || got.Receipt != nil || got.Err() == nil {
					t.Fatal("COMPLETION_FALSE_SUCCESS", got)
				}
				if got.Failure.AtomicRename != tc.committed {
					t.Error("COMBINATION_COMMIT_CHANGED", got.Failure)
				}
				if !errors.Is(got.Err(), quotaCause) {
					t.Error("COMBINATION_QUOTA_CAUSE_LOST", got.Err())
				}
				if tc.prior.Failure != nil && (!errors.Is(got.Err(), priorCause) || !errors.Is(got.Err(), tc.prior.Failure)) {
					t.Error("COMBINATION_PRIOR_CAUSE_LOST", got.Err())
				}
				if got.Failure.Stage != "storage" || got.Failure.Code != CodePublicationFailed || got.Failure.Cleanup {
					t.Error("COMPLETION_FAILURE_CONTRACT", got.Failure)
				}
			})
		})
	}
}
