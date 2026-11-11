//go:build lsp_trace_controlled_describe_fixture

package inventorysuccessor

import (
	"context"
	"errors"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
	"testing"
)

func TestPrivatePrepareWindowNativePropagation(t *testing.T) {
	t.Log("PROCEDURE: fixed pipe fixture actual Prepare/Capture/issuance/Finish; existing CAS seam refuses Observe; no live provider or actual job")
	q, f := prepareDescribeNativeFixtureV1(t, -1)
	calls := 0
	marker := errors.New("fixed unit Observe refusal")
	q.Owner.k.cas = func(ctx context.Context, root *publication.Root, selector string, previous publication.BoundFilePredecessor, raw []byte, validate func([]byte) error) (*publication.CompareAndReplaceReceipt, error) {
		calls++
		if calls == 2 {
			return nil, marker
		}
		return publication.CompareAndReplaceBoundFile(ctx, root, selector, previous, raw, validate)
	}
	out, err := PrepareDescribeV1(context.Background(), q)
	codes := ProjectPrivatePrepareFailureV1(err)
	want := PrivatePrepareLeaseObservationV1{Boundary: sessionruntime.PrivateLeaseBoundaryResultReturned, ResultObserved: PrivatePrepareObservedYesV1, InnerResult: sessionruntime.PrivateLeaseResultOK, CallbackSample: PrivatePrepareCallbackCallerPostReturnLockV1, CallbackEntered: PrivatePrepareObservedYesV1, CaptureResult: sessionruntime.PrivateGroupingCaptureOK, IssuanceResult: PrivatePrepareCheckpointOKV1, CallsResult: PrivatePrepareCheckpointNotApplicableV1}
	if err == nil || len(out.Packet) != 0 || out.Continuation != (PreparedDescribeAnchorsV1{}) || codes.Stage != privatePrepareObserveV1 || codes.Lease != want || !errors.Is(err, ErrPreparedDescribeV1) || !errors.Is(err, ErrPrivateMixedInputV2) || !errors.Is(err, marker) || calls != 2 || f.Manager().Census().Workers != 0 {
		t.Fatal("ASSERT_LEASE_NATIVE_PROPAGATION: FAIL", codes.Stage.String(), codes.Lease)
	}
	t.Log("ASSERT_LEASE_NATIVE_PROPAGATION: PASS")
}
