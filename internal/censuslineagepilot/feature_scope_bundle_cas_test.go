//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"lsp-trace/internal/publication"
)

type bundleStaleImpostor struct{}

func (bundleStaleImpostor) Error() string        { return "opaque error claiming stale through Is" }
func (bundleStaleImpostor) Is(target error) bool { return target == publication.ErrStalePredecessor }

func TestScopeBundleCASClassificationTable(t *testing.T) {
	stale := publication.ErrStalePredecessor
	ordinary := errors.New("ordinary CAS error")
	notCommitted := func() *publication.CompareAndReplaceReceipt {
		return &publication.CompareAndReplaceReceipt{Outcome: publication.CASOutcomeNotCommitted}
	}
	cases := []struct {
		name     string
		receipt  *publication.CompareAndReplaceReceipt
		err      error
		conflict bool
	}{
		{"real-stale-shape", &publication.CompareAndReplaceReceipt{}, stale, true},
		{"wrapped-pure-stale", &publication.CompareAndReplaceReceipt{}, fmt.Errorf("predecessor: %w", stale), true},
		{"joined-only-stale", &publication.CompareAndReplaceReceipt{}, errors.Join(stale, fmt.Errorf("predecessor: %w", stale)), true},
		{"not-committed-canceled", notCommitted(), context.Canceled, false},
		{"not-committed-deadline", notCommitted(), context.DeadlineExceeded, false},
		{"unknown-outcome-stale", &publication.CompareAndReplaceReceipt{Outcome: "UNKNOWN"}, stale, false},
		{"not-committed-joined-stale-cancel", notCommitted(), errors.Join(stale, context.Canceled), false},
		{"not-committed-joined-stale-other", notCommitted(), errors.Join(stale, ordinary), false},
		{"empty-outcome-joined-stale-cancel", &publication.CompareAndReplaceReceipt{}, errors.Join(stale, context.Canceled), false},
		{"empty-outcome-joined-stale-other", &publication.CompareAndReplaceReceipt{}, errors.Join(stale, ordinary), false},
		{"not-committed-pure-stale-is-not-api-shape", notCommitted(), stale, false},
		{"not-committed-nil-error-not-api-conflict", notCommitted(), nil, false},
		{"stale-with-staging-receipt", &publication.CompareAndReplaceReceipt{BoundFileReceipt: &publication.BoundFileReceipt{}}, stale, false},
		{"outcome-committed-flag-false", &publication.CompareAndReplaceReceipt{Outcome: publication.CASOutcomeCommitted}, stale, false},
		{"flag-committed-outcome-not", &publication.CompareAndReplaceReceipt{Committed: true, Outcome: publication.CASOutcomeNotCommitted}, nil, false},
		{"flag-committed-stale-error", &publication.CompareAndReplaceReceipt{Committed: true, Outcome: publication.CASOutcomeCommitted}, stale, false},
		{"flag-committed-cancel-error", &publication.CompareAndReplaceReceipt{Committed: true, Outcome: publication.CASOutcomeCommitted}, context.Canceled, false},
		{"flag-committed-unknown-outcome", &publication.CompareAndReplaceReceipt{Committed: true, Outcome: "UNKNOWN"}, ordinary, false},
		{"nil-receipt-stale", nil, stale, false},
		{"nil-receipt-nil-error", nil, nil, false},
		{"nil-receipt-ordinary-error", nil, ordinary, false},
		{"empty-receipt-ordinary-error", &publication.CompareAndReplaceReceipt{}, ordinary, false},
		{"empty-receipt-nil-error", &publication.CompareAndReplaceReceipt{}, nil, false},
		{"opaque-Is-stale-is-not-evidence", &publication.CompareAndReplaceReceipt{}, bundleStaleImpostor{}, false},
	}
	for _, finalize := range []bool{false, true} {
		branch := "claim"
		if finalize {
			branch = "finalize"
		}
		for _, tc := range cases {
			t.Run(branch+"/"+tc.name, func(t *testing.T) {
				o, _ := bundleFixture(t, false)
				var pending *scopeBundlePending
				var previous []byte
				if finalize {
					claim, err := o.advance(context.Background(), nil)
					if err != nil {
						t.Fatal(err)
					}
					pending = claim.pending
					previous = claim.HeadBytes()
				}
				calls, pairs := 0, 0
				var replacement []byte
				o.ops.cas = func(_ context.Context, _ *publication.ContentRetentionRoot, _ string, old, new []byte) (*publication.CompareAndReplaceReceipt, error) {
					calls++
					if !bytes.Equal(old, previous) {
						t.Fatal("CAS predecessor changed")
					}
					replacement = bytes.Clone(new)
					return tc.receipt, tc.err
				}
				o.ops.pair = func(context.Context, scopeRetentionPlan, int, *publication.ContentRetentionRoot, *publication.ContentRetentionRoot) (scopeRetentionAttempt, error) {
					pairs++
					return scopeRetentionAttempt{}, errors.New("pair must not execute")
				}
				out, err := o.advance(context.Background(), pending)
				want := "INDETERMINATE_EXECUTION"
				if tc.conflict {
					want = "CONFLICT"
				}
				if out.outcome != want {
					t.Errorf("CAS disposition: got %s want %s", out.outcome, want)
				}
				if tc.err != nil && !errors.Is(err, tc.err) {
					t.Errorf("original CAS cause lost: %v", err)
				}
				if calls != 1 || pairs != 0 || out.pending != nil {
					t.Errorf("CAS execution custody: calls=%d pairs=%d pending=%v", calls, pairs, out.pending != nil)
				}
				known := tc.receipt != nil && tc.receipt.Committed
				expected := previous
				if known {
					expected = replacement
				}
				if out.cas != tc.receipt || out.casCommitted != known || !bytes.Equal(out.HeadBytes(), expected) {
					t.Error("exact known CAS/head evidence lost")
				}
				if finalize && (o.pending != nil || len(out.ReceiptBytes()) == 0 || out.attempt.primary.receipt == nil || out.attempt.backup.receipt == nil) {
					t.Error("finalize pending/attempt custody lost")
				}
			})
		}
	}
}

// The production CAS invokes its verifier while publishing the immutable stage.
// Cancel there: PublishBoundFile completes, then the real post-stage ctx check
// returns NOT_COMMITTED + Canceled with a staging receipt, not a head conflict.
func TestScopeBundleRealCASStagingCancellation(t *testing.T) {
	for _, finalize := range []bool{false, true} {
		name := "claim"
		if finalize {
			name = "finalize"
		}
		t.Run(name, func(t *testing.T) {
			o, _ := bundleFixture(t, false)
			var pending *scopeBundlePending
			var previous []byte
			if finalize {
				claim, err := o.advance(context.Background(), nil)
				if err != nil {
					t.Fatal(err)
				}
				pending = claim.pending
				previous = claim.HeadBytes()
			}
			calls, pairs, verified := 0, 0, 0
			o.ops.cas = func(ctx context.Context, r *publication.ContentRetentionRoot, key string, old, new []byte) (*publication.CompareAndReplaceReceipt, error) {
				calls++
				callCtx, cancel := context.WithCancel(ctx)
				defer cancel()
				return r.CompareScopeBundleHead(callCtx, key, old, new, func(raw []byte) error { verified++; _, err := parseScopeBundleHead(raw); cancel(); return err })
			}
			o.ops.pair = func(context.Context, scopeRetentionPlan, int, *publication.ContentRetentionRoot, *publication.ContentRetentionRoot) (scopeRetentionAttempt, error) {
				pairs++
				return scopeRetentionAttempt{}, nil
			}
			out, err := o.advance(context.Background(), pending)
			if verified == 0 || out.cas == nil || out.cas.Outcome != publication.CASOutcomeNotCommitted || out.cas.BoundFileReceipt == nil || out.cas.Committed {
				t.Fatal("real post-stage cancellation not reached", out.cas, err)
			}
			if !errors.Is(err, context.Canceled) || out.outcome != "INDETERMINATE_EXECUTION" || out.casCommitted || calls != 1 || pairs != 0 || out.pending != nil || o.pending != nil || !bytes.Equal(out.HeadBytes(), previous) {
				t.Fatal("real staging cancellation misclassified", out.outcome, err)
			}
			root, e := publication.OpenContentRetentionRoot(context.Background(), o.paths[2])
			if e != nil {
				t.Fatal(e)
			}
			raw, readErr := root.ReadScopeBundleHead(context.Background(), scopeBundleKey(*o.binding))
			closeErr := root.Close()
			if closeErr != nil {
				t.Fatal(closeErr)
			}
			if finalize {
				if readErr != nil || !bytes.Equal(raw, previous) {
					t.Fatal("cancel replaced known claim", readErr)
				}
			} else if !errors.Is(readErr, os.ErrNotExist) {
				t.Fatal("cancel committed a head", readErr)
			}
		})
	}
}

func TestScopeBundleRealPredecessorConflict(t *testing.T) {
	for _, finalize := range []bool{false, true} {
		name := "claim"
		if finalize {
			name = "finalize"
		}
		t.Run(name, func(t *testing.T) {
			o, _ := bundleFixture(t, false)
			var pending *scopeBundlePending
			var previous []byte
			if finalize {
				claim, err := o.advance(context.Background(), nil)
				if err != nil {
					t.Fatal(err)
				}
				pending = claim.pending
				previous = claim.HeadBytes()
			}
			calls, pairs := 0, 0
			o.ops.cas = func(ctx context.Context, r *publication.ContentRetentionRoot, key string, old, new []byte) (*publication.CompareAndReplaceReceipt, error) {
				calls++
				// Deliberately stale exact-byte predecessor, using the real publication CAS.
				stalePrevious := append(bytes.Clone(old), '!')
				return r.CompareScopeBundleHead(ctx, key, stalePrevious, new, func(raw []byte) error { _, err := parseScopeBundleHead(raw); return err })
			}
			o.ops.pair = func(context.Context, scopeRetentionPlan, int, *publication.ContentRetentionRoot, *publication.ContentRetentionRoot) (scopeRetentionAttempt, error) {
				pairs++
				return scopeRetentionAttempt{}, nil
			}
			out, err := o.advance(context.Background(), pending)
			if out.outcome != "CONFLICT" || !errors.Is(err, publication.ErrStalePredecessor) || out.cas == nil || out.cas.Committed || out.cas.Outcome != "" || out.cas.BoundFileReceipt != nil || calls != 1 || pairs != 0 || out.pending != nil || o.pending != nil || !bytes.Equal(out.HeadBytes(), previous) {
				t.Fatal("real stale conflict not recognized", out.outcome, err)
			}
		})
	}
}

type bundleCyclicCause struct{}

func (e *bundleCyclicCause) Error() string { return "cyclic cause" }
func (e *bundleCyclicCause) Unwrap() error { return e }

func TestScopeBundleCASConflictCauseBound(t *testing.T) {
	for _, nodes := range []int{64, 65} {
		var cause error = publication.ErrStalePredecessor
		for i := 1; i < nodes; i++ {
			cause = fmt.Errorf("wrapper: %w", cause)
		}
		if got := scopeBundlePredecessorConflict(&publication.CompareAndReplaceReceipt{}, cause); got != (nodes == 64) {
			t.Fatalf("cause budget boundary %d: %v", nodes, got)
		}
	}
	if scopeBundlePredecessorConflict(&publication.CompareAndReplaceReceipt{}, &bundleCyclicCause{}) {
		t.Fatal("cyclic cause established conflict")
	}
	causes := make([]error, 64)
	for i := range causes {
		causes[i] = publication.ErrStalePredecessor
	}
	if scopeBundlePredecessorConflict(&publication.CompareAndReplaceReceipt{}, errors.Join(causes...)) {
		t.Fatal("wide cause tree exceeded bound")
	}
}

func TestScopeBundleReturnedAccessorMutationDoesNotChangePending(t *testing.T) {
	o, _ := bundleFixture(t, false)
	claim, err := o.advance(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	head := claim.HeadBytes()
	head[0] = '!'
	receipt := claim.ReceiptBytes()
	receipt[0] = '!'
	claim.receiptPublication.FinalSelector = "changed returned publication"
	claim.attempt.primary.receipt.Digest = "changed returned copy"
	out, err := o.advance(context.Background(), claim.pending)
	if err != nil || out.outcome != "READY" || out.attempt.primary.receipt.Digest != o.item(0).digest {
		t.Fatal("returned observations aliased pending custody", out.outcome, err)
	}
}
