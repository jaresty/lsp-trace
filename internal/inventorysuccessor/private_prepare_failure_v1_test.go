package inventorysuccessor

import (
	"context"
	"errors"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
	"strings"
	"testing"
	"time"
)

type prepareHostileCauseV1 struct{}

func (prepareHostileCauseV1) Error() string { panic("raw secret Error invoked") }
func (prepareHostileCauseV1) Is(error) bool { panic("opaque Is invoked") }
func (prepareHostileCauseV1) Unwrap() error { panic("opaque Unwrap invoked") }

type prepareHostileSliceV1 []string

func (prepareHostileSliceV1) Error() string { panic("noncomparable raw cause invoked") }

type privatePrepareCauseSpyV1 struct{ calls *int }

func (s privatePrepareCauseSpyV1) Error() string { *s.calls++; return "opaque-test-value" }
func (s privatePrepareCauseSpyV1) Is(error) bool { *s.calls++; return false }
func (s privatePrepareCauseSpyV1) Unwrap() error { *s.calls++; return nil }

func TestPrivatePrepareFailureNoCauseMethods(t *testing.T) {
	t.Log("PROCEDURE: counted opaque cause Error/Is/Unwrap methods; closed projection must invoke zero")
	calls := 0
	spy := privatePrepareCauseSpyV1{calls: &calls}
	for _, err := range []error{spy, privatePrepareFailureAtV1(privatePrepareCaptureV1, ErrPrivateMixedInputV2, spy)} {
		got := ProjectPrivatePrepareFailureV1(err)
		if calls != 0 || got.Reason != privatePrepareRefusedV1 {
			t.Fatal("ASSERT_DIAG_NO_CAUSE_METHODS: FAIL", calls)
		}
	}
	t.Log("ASSERT_DIAG_NO_CAUSE_METHODS: PASS")
}

func TestPrivatePrepareFailureClosedProjection(t *testing.T) {
	t.Log("PROCEDURE: closed immutable error projection; fixed stage/reason table; opaque hostile and overlarge graphs never traversed")
	for _, stage := range []PrivatePrepareStageV1{privatePrepareReserveV1, privatePrepareReplayV1, privatePrepareWindowV1, privatePrepareStartV1, privatePrepareReadinessV1, privatePrepareCaptureV1, privatePrepareFinishV1, privatePrepareObserveV1, privatePrepareRprepV1, privatePreparePersistV1, privatePreparePacketV1} {
		err := privatePrepareFailureAtV1(stage, ErrPrivateMixedInputV2, context.DeadlineExceeded)
		wrapped := privatePrepareFailureAtV1(privatePrepareCaptureV1, ErrPreparedDescribeV1, err)
		got := ProjectPrivatePrepareFailureV1(wrapped)
		if got.Stage != stage || got.Reason != privatePrepareDeadlineV1 {
			t.Fatal("ASSERT_DIAG_FIRST_STAGE: FAIL")
		}
		if !errors.Is(wrapped, ErrPreparedDescribeV1) || !errors.Is(wrapped, ErrPrivateMixedInputV2) || !errors.Is(wrapped, context.DeadlineExceeded) {
			t.Fatal("ASSERT_DIAG_SENTINELS: FAIL")
		}
		got.Stage = 255
		if ProjectPrivatePrepareFailureV1(wrapped).Stage != stage {
			t.Fatal("ASSERT_DIAG_IMMUTABLE: FAIL")
		}
	}
	for _, tc := range []struct {
		cause error
		want  PrivatePrepareReasonV1
	}{{context.DeadlineExceeded, privatePrepareDeadlineV1}, {context.Canceled, privatePrepareCanceledV1}, {ErrPrivateMixedInputUnavailableV2, privatePrepareUnavailableV1}, {ErrPrivateMixedInputEmptyV2, privatePrepareEmptyV1}, {sessionruntime.ErrPrivateProviderWindowUnjoined, privatePrepareUnjoinedV1}, {errUncertainV2, privatePrepareCommitUncertainV1}, {prepareHostileCauseV1{}, privatePrepareRefusedV1}, {prepareHostileSliceV1{"secret"}, privatePrepareRefusedV1}, {errors.New(strings.Repeat("raw-secret", 1<<17)), privatePrepareRefusedV1}} {
		got := ProjectPrivatePrepareFailureV1(privatePrepareFailureAtV1(privatePrepareCaptureV1, ErrPrivateMixedInputV2, tc.cause))
		if got.Reason != tc.want {
			t.Fatal("ASSERT_DIAG_CLOSED_REASONS: FAIL")
		}
	}
	// A huge external graph is opaque, not a request to traverse its nodes. No
	// open-ended queue, truncation allocation, or raw-error length is consulted.
	graph := make([]error, 4096)
	for i := range graph {
		graph[i] = prepareHostileCauseV1{}
	}
	if got := ProjectPrivatePrepareFailureV1(errors.Join(graph...)); got.Stage != PrivatePrepareStageNoneV1 || got.Reason != privatePrepareRefusedV1 {
		t.Fatal("ASSERT_DIAG_CLOSED_BOUND: FAIL")
	}
	err := privatePrepareFailureAtV1(privatePreparePacketV1, ErrPreparedDescribeV1, context.Canceled)
	for i := 0; i < 4096; i++ {
		err = privatePrepareFailureAtV1(privatePrepareCaptureV1, ErrPreparedDescribeV1, err)
	}
	if got := ProjectPrivatePrepareFailureV1(err); got.Stage != privatePreparePacketV1 || got.Reason != privatePrepareCanceledV1 {
		t.Fatal("ASSERT_DIAG_CLOSED_BOUND: FAIL")
	}
	if PrivatePrepareStageV1(255).String() != "NONE" || PrivatePrepareReasonV1(254).String() != "REFUSED" {
		t.Fatal("ASSERT_DIAG_CLOSED_BOUND: FAIL")
	}
	t.Log("ASSERT_DIAG_FIRST_STAGE: PASS; ASSERT_DIAG_SENTINELS: PASS; ASSERT_DIAG_IMMUTABLE: PASS; ASSERT_DIAG_CLOSED_REASONS: PASS; ASSERT_DIAG_CLOSED_BOUND: PASS")
}

func TestPrivatePrepareFailureCleanupProjection(t *testing.T) {
	t.Log("PROCEDURE: separate first execution and six cleanup slots; no new calls; successful repeated Finish result remains successful")
	primary := privatePrepareFailureAtV1(privatePrepareReadinessV1, ErrPrivateMixedInputV2, context.Canceled)
	withFinish := privatePrepareDeferredFinishV1(primary, sessionruntime.ErrPrivateProviderWindowUnjoined)
	terminal := errors.Join(withFinish, prepareHostileCauseV1{}, context.DeadlineExceeded)
	err := PrivatePrepareTerminalFailureV1(withFinish, terminal, prepareHostileCauseV1{}, prepareHostileCauseV1{}, prepareHostileCauseV1{}, context.Canceled, context.DeadlineExceeded)
	got := ProjectPrivatePrepareFailureV1(err)
	want := [6]PrivatePrepareReasonV1{privatePrepareUnjoinedV1, privatePrepareRefusedV1, privatePrepareRefusedV1, privatePrepareRefusedV1, privatePrepareCanceledV1, privatePrepareDeadlineV1}
	if got.Stage != privatePrepareReadinessV1 || got.Reason != privatePrepareCanceledV1 || got.Cleanup != want {
		t.Fatal("ASSERT_DIAG_CLEANUP_DISTINCT: FAIL")
	}
	if privatePrepareDeferredFinishV1(nil, context.DeadlineExceeded) != nil || PrivatePrepareTerminalFailureV1(nil, nil, nil, nil, nil, nil, nil) != nil {
		t.Fatal("ASSERT_DIAG_NO_SUCCESS_CHANGE: FAIL")
	}
	only := PrivatePrepareTerminalFailureV1(nil, context.Canceled, nil, nil, nil, context.Canceled, nil)
	if got := ProjectPrivatePrepareFailureV1(only); got.Stage != PrivatePrepareStageNoneV1 || got.Reason != privatePrepareNoReasonV1 || got.Cleanup[4] != privatePrepareCanceledV1 {
		t.Fatal("ASSERT_DIAG_CLEANUP_ONLY: FAIL")
	}
	t.Log("ASSERT_DIAG_CLEANUP_DISTINCT: PASS; ASSERT_DIAG_NO_SUCCESS_CHANGE: PASS; ASSERT_DIAG_CLEANUP_ONLY: PASS")
}

func TestPrivatePrepareFailureObserveActual(t *testing.T) {
	t.Log("PROCEDURE: real funded managed Reserve/consume then native Observe CAS fault; test-only source state is NOT a real producer proof")
	for _, mode := range []string{"ordinary", "commit-uncertain", "healthy"} {
		t.Run(mode, func(t *testing.T) {
			owner, x := controlledManagedOwnerV2(t)
			b := managedBindingTestV2(x)
			op, e := owner.Reserve(context.Background(), hashLabelV2("diag-observe"), b)
			if e != nil {
				t.Fatal("SETUP_OBSERVE_RESERVE")
			}
			ctx, cancel, e := op.consumeFixed(context.Background(), b)
			if e != nil {
				t.Fatal("SETUP_OBSERVE_CONSUME")
			}
			defer cancel()
			op.state.source = hashLabelV2("TEST_ONLY_FIXED_COMPLETION_SOURCE_NOT_PRODUCTION_PROOF")
			before, e := owner.k.reopen(context.Background(), owner.job, owner.limits)
			if e != nil {
				t.Fatal("SETUP_OBSERVE_BEFORE")
			}
			calls := 0
			marker := errors.New("private test-only CAS marker")
			owner.k.cas = func(ctx context.Context, r *publication.Root, s string, p publication.BoundFilePredecessor, raw []byte, v func([]byte) error) (*publication.CompareAndReplaceReceipt, error) {
				calls++
				if mode == "ordinary" {
					return nil, marker
				}
				receipt, err := publication.CompareAndReplaceBoundFile(ctx, r, s, p, raw, v)
				if err == nil && mode == "commit-uncertain" {
					return receipt, marker
				}
				return receipt, err
			}
			e = op.completeFixed(ctx)
			after, readErr := owner.k.reopen(context.Background(), owner.job, owner.limits)
			if calls != 1 || readErr != nil || before.Windows != after.Windows || before.Charged != after.Charged {
				t.Fatal("ASSERT_DIAG_OBSERVE_TRACE: FAIL")
			}
			if mode == "healthy" {
				if e != nil || !op.state.finished || owner.uncertain || after.Disposition != "OBSERVED" {
					t.Fatal("ASSERT_DIAG_OBSERVE_TRACE: FAIL")
				}
				return
			}
			codes := ProjectPrivatePrepareFailureV1(e)
			if e == nil || codes.Stage != privatePrepareObserveV1 || !owner.uncertain || op.state.finished || !errors.Is(e, ErrPrivateManagedInputOperationV2) {
				t.Fatal("ASSERT_DIAG_ACTUAL_OBSERVE: FAIL")
			}
			if mode == "ordinary" && (!errors.Is(e, marker) || after.Disposition != "PENDING" || codes.Reason != privatePrepareRefusedV1) {
				t.Fatal("ASSERT_DIAG_OBSERVE_CAUSE: FAIL")
			}
			if mode == "commit-uncertain" && (!errors.Is(e, errUncertainV2) || codes.Reason != privatePrepareCommitUncertainV1 || after.Disposition != "OBSERVED" || owner.head != before.Identity) {
				t.Fatal("ASSERT_DIAG_OBSERVE_UNCERTAIN: FAIL")
			}
		})
	}
	t.Log("ASSERT_DIAG_ACTUAL_OBSERVE: PASS; ASSERT_DIAG_OBSERVE_CAUSE: PASS; ASSERT_DIAG_OBSERVE_UNCERTAIN: PASS; ASSERT_DIAG_OBSERVE_TRACE: PASS")
}

func TestPrivatePrepareFailurePreflightActual(t *testing.T) {
	for _, mode := range []string{"zero", "canceled", "deadline"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			q := PrepareDescribeRequestV1{}
			want := privatePrepareRefusedV1
			if mode == "canceled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
				want = privatePrepareCanceledV1
			}
			if mode == "deadline" {
				q.Persist.Deadline = time.Now().Add(-time.Second)
				want = privatePrepareDeadlineV1
			}
			out, e := PrepareDescribeV1(ctx, q)
			c := ProjectPrivatePrepareFailureV1(e)
			if len(out.Packet) != 0 || c.Stage != privatePrepareReserveV1 || c.Reason != want || !errors.Is(e, ErrPreparedDescribeV1) || e.Error() != ErrPreparedDescribeV1.Error() {
				t.Fatal("ASSERT_DIAG_ACTUAL_RESERVE: FAIL")
			}
		})
	}
	t.Log("ASSERT_DIAG_ACTUAL_RESERVE: PASS")
}
