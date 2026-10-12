package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"

	"lsp-trace/internal/adr0011work"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

func logicalBufferSuccessFrame(result json.RawMessage) ([]byte, lspwire.Message) {
	body := append([]byte(`{"jsonrpc":"2.0","id":1,"result":`), result...)
	body = append(body, '}')
	frame := append(successorHeaderForLogicalBuffer(len(body)), body...)
	return frame, lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(`1`), Result: result}
}

func successorHeaderForLogicalBuffer(n int) []byte {
	return []byte("Content-Length: " + itoaLogicalBuffer(n) + "\r\n\r\n")
}

func itoaLogicalBuffer(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func logicalBufferOwner(t *testing.T, identity string, work *adr0011work.Account) (privateB4SuccessorAllocationOwner, *privateB4ByteAccountV2) {
	t.Helper()
	c15, failure := newPrivateB4ByteAccountV2()
	if failure != "" {
		t.Fatalf("BLOCKED_NOT_RED: C15 account: %s", failure)
	}
	// RED: production must bind the real successor owner to the same definition
	// transaction work account and occurrence identity before touching C15.
	return privateB4SuccessorAllocationOwner{account: c15, workAccount: work, identity: identity}, c15
}

func logicalBufferReadSuccess(t *testing.T, owner privateB4SuccessorAllocationOwner, result json.RawMessage) (lspwire.Message, *privateB4Reservation, int, int) {
	t.Helper()
	frame, _ := logicalBufferSuccessFrame(result)
	reservation := &privateB4Reservation{}
	capture := privateB4ResponseCapture{reservation: reservation}
	opts := privateB4SuccessorOptions(&privateB4Reservation{method: "textDocument/definition"})
	opts.AllocationOwner = owner
	opts.Capture = capture
	r, err := lspwire.NewSuccessorIngressReader(bytes.NewReader(frame), opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	msg, err := r.ReadFrame()
	if err != nil {
		t.Fatal(err)
	}
	clone, cloneLease, err := lspwire.CloneSuccessorMessageOwned(msg, owner)
	if err != nil {
		t.Fatal(err)
	}
	cloneLease.Release()
	return clone, reservation, len(successorHeaderForLogicalBuffer(len(frame) - len(successorHeaderForLogicalBuffer(0)))), len(frame)
}

func logicalBufferExactSuccessfulSum(frame []byte, msg lspwire.Message, headerBytes uint64, resultLen int) uint64 {
	headerLen := len(frame) - int(jsonBodyLength(frame))
	bodyLen := len(frame) - headerLen
	// Seven production roles, with consume-scratch reached once for each actual
	// consumed segment (header and body), are cumulative reservations, not live bytes.
	return 4096 + (headerBytes + 1) + uint64(headerLen+bodyLen) + uint64(bodyLen) + uint64(len(frame)) + uint64(resultLen) + lspwire.SuccessorMessageRetainedCapacity(msg)
}

func jsonBodyLength(frame []byte) uint64 {
	i := bytes.Index(frame, []byte("\r\n\r\n"))
	if i < 0 {
		return 0
	}
	return uint64(len(frame) - i - 4)
}

func TestADR0011C17SuccessorLogicalBufferSuccessfulMessageExactReachedSumAndNoLeaseRefund(t *testing.T) {
	const identity = "logical-success"
	work := adr0011work.New(identity)
	owner, c15 := logicalBufferOwner(t, identity, work)
	result := json.RawMessage(`[{"uri":"file:///target.go"}]`)
	frame, decoded := logicalBufferSuccessFrame(result)
	clone, reservation, _, _ := logicalBufferReadSuccess(t, owner, result)
	want := logicalBufferExactSuccessfulSum(frame, decoded, 65536, len(result))
	if clone.Kind() != lspwire.KindSuccessResponse || work.Snapshot().LogicalBufferBytesReserved != want {
		t.Fatalf("ASSERT_LOGICAL_BUFFER_SUCCESS_EXACT got=%+v want=%d clone=%+v", work.Snapshot(), want, clone)
	}
	before := work.Snapshot()
	reservation.responseFrameOwner.releaseManager()
	reservation.resultOwner.releaseManager()
	if after := work.Snapshot(); after.LogicalBufferBytesReserved != before.LogicalBufferBytesReserved {
		t.Fatalf("ASSERT_LOGICAL_BUFFER_RELEASE_NO_REFUND before=%+v after=%+v", before, after)
	}
	c15.requestTerminalRelease()
}

func TestADR0011C17SuccessorLogicalBufferMalformedCountsSuccessfulPriorReservations(t *testing.T) {
	const identity = "logical-malformed"
	work := adr0011work.New(identity)
	owner, c15 := logicalBufferOwner(t, identity, work)
	body := []byte(`{"jsonrpc":"2.0","id":1,"result":`)
	header := successorHeaderForLogicalBuffer(len(body))
	reservation := &privateB4Reservation{}
	opts := privateB4SuccessorOptions(&privateB4Reservation{method: "textDocument/definition"})
	opts.AllocationOwner, opts.Capture = owner, privateB4ResponseCapture{reservation: reservation}
	r, err := lspwire.NewSuccessorIngressReader(bytes.NewReader(append(header, body...)), opts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReadFrame(); err == nil {
		t.Fatal("BLOCKED_NOT_RED: malformed fixture accepted")
	}
	want := uint64(4096) + opts.Limits.HeaderBytes + 1 + uint64(len(header)+len(body)) + uint64(len(body)) + uint64(len(header)+len(body))
	if got := work.Snapshot().LogicalBufferBytesReserved; got != want {
		t.Fatalf("ASSERT_LOGICAL_BUFFER_MALFORMED_PRIOR_REACHED got=%d want=%d", got, want)
	}
	r.Close()
	reservation.responseFrameOwner.releaseManager()
	c15.requestTerminalRelease()
}

func TestADR0011C17SuccessorLogicalBufferRefusalOverflowMismatchAndReplay(t *testing.T) {
	t.Run("work refusal is zero for refused role and precedes C15", func(t *testing.T) {
		work := adr0011work.New("refuse")
		if err := work.ChargeLogicalBufferBytesReserved("refuse", math.MaxUint64); err != nil {
			t.Fatal(err)
		}
		owner, c15 := logicalBufferOwner(t, "refuse", work)
		before := c15.snapshot()
		if lease, err := owner.ReserveSuccessorAllocation(lspwire.SuccessorAllocationPrefetch, 1); err == nil || lease != nil {
			t.Fatalf("ASSERT_LOGICAL_BUFFER_WORK_REFUSAL lease=%v err=%v", lease, err)
		}
		if after := c15.snapshot(); after != before {
			t.Fatalf("ASSERT_LOGICAL_BUFFER_REFUSED_ROLE_ZERO_C15 before=%+v after=%+v", before, after)
		}
	})

	t.Run("identity mismatch precedes C15", func(t *testing.T) {
		work := adr0011work.New("right")
		owner, c15 := logicalBufferOwner(t, "wrong", work)
		before := c15.snapshot()
		if lease, err := owner.ReserveSuccessorAllocation(lspwire.SuccessorAllocationHeader, 7); !errors.Is(err, adr0011work.ErrIdentityMismatch) || lease != nil {
			t.Fatalf("ASSERT_LOGICAL_BUFFER_IDENTITY_PRE_C15 lease=%v err=%v", lease, err)
		}
		if c15.snapshot() != before || work.Snapshot().LogicalBufferBytesReserved != 0 {
			t.Fatal("ASSERT_LOGICAL_BUFFER_IDENTITY_ZERO_EFFECT")
		}
	})

	t.Run("C15 refusal rolls back work and stays distinct from work overflow", func(t *testing.T) {
		work := adr0011work.New("c15")
		owner, c15 := logicalBufferOwner(t, "c15", work)
		fill, failure := c15.reserve(privateB4MaxOwnedBytes - c15.snapshot().Live)
		if failure != "" {
			t.Fatal(failure)
		}
		if lease, err := owner.ReserveSuccessorAllocation(lspwire.SuccessorAllocationBody, 1); err == nil || lease != nil {
			t.Fatalf("ASSERT_LOGICAL_BUFFER_C15_REFUSAL lease=%v err=%v", lease, err)
		}
		if got := work.Snapshot(); got.LogicalBufferBytesReserved != 0 || got.Terminal {
			t.Fatalf("ASSERT_LOGICAL_BUFFER_C15_ROLLBACK_DISTINCT %+v", got)
		}
		fill.release()
	})

	t.Run("replay counts anew", func(t *testing.T) {
		work := adr0011work.New("replay")
		owner, c15 := logicalBufferOwner(t, "replay", work)
		lease1, err := owner.ReserveSuccessorAllocation(lspwire.SuccessorAllocationDecodedMessage, 9)
		if err != nil {
			t.Fatal(err)
		}
		lease1.Release()
		lease2, err := owner.ReserveSuccessorAllocation(lspwire.SuccessorAllocationDecodedMessage, 9)
		if err != nil {
			t.Fatal(err)
		}
		lease2.Release()
		if got := work.Snapshot().LogicalBufferBytesReserved; got != 18 {
			t.Fatalf("ASSERT_LOGICAL_BUFFER_REPLAY_ANEW got=%d", got)
		}
		c15.requestTerminalRelease()
	})
}

func TestADR0011C17SuccessorLogicalBufferSealWaitsOwnerCallback(t *testing.T) {
	t.Skip("BLOCKED_NOT_RED: real owner has no test-only between-work-admission-and-C15-reservation seam; production integration must add a private callback hook before this causal Seal race can run")
}

type logicalBufferSnapshotDelta struct {
	live       int64
	cumulative uint64
}

type logicalBufferDefaultOffObservation struct {
	result struct {
		Result                    string
		ServerError               *lspwire.RPCError
		Failure                   session.Failure
		Messages, RequestMessages int
		Bytes, RequestBytes       int64
		Notifications, Responses  []lspwire.Message
	}
	ownedSnapshot                            privateB4ByteLedgerSnapshot
	consumeDelta, releaseDelta               logicalBufferSnapshotDelta
	requestLease, responseLease, resultLease bool
	requestFrame, responseFrame, resultBytes []byte
	capture                                  PrivateB4DefinitionCapture
	consumeStatus                            PrivateB4Status
	releases                                 [3]bool
}

func logicalBufferDelta(before, after privateB4ByteLedgerSnapshot) logicalBufferSnapshotDelta {
	return logicalBufferSnapshotDelta{
		live:       int64(after.Live) - int64(before.Live),
		cumulative: after.Cumulative - before.Cumulative,
	}
}

func observeLogicalBufferDefaultOff(t *testing.T, withDisabledAccounting bool) logicalBufferDefaultOffObservation {
	t.Helper()
	f := newFullP1Fixture(t)
	f.req.EnablePrivateC16ObjectAccounting = false
	f.req.EnablePrivateC17EventAccounting = false
	var c15 *privateB4ByteAccountV2
	f.m.b4ParamsHook = func(got *privateB4ByteAccountV2) { c15 = got }
	ctx := context.Background()
	var supplied *adr0011work.Account
	if withDisabledAccounting {
		supplied = adr0011work.New(f.owner.Transaction)
		ctx = adr0011work.WithAccount(ctx, supplied)
	}
	result, managerLease := f.m.RoundTripPrivateB4(ctx, f.req, f.owner)
	selection := f.selectionBase
	selection.Key = result.Key
	if c15 == nil || result.Failure != "" || managerLease == (B4DefinitionLease{}) {
		t.Fatalf("BLOCKED_NOT_RED: default-off transaction failure=%s lease=%+v", result.Failure, managerLease)
	}
	if supplied != nil {
		snapshot := supplied.Snapshot()
		if snapshot.Identity != f.owner.Transaction || snapshot.BytesScanned != 0 || snapshot.MessagesProcessed != 1 || snapshot.ObjectsMaterialized != 0 || snapshot.EventsEmitted != 0 || snapshot.DocumentsAcquired != 0 || snapshot.LogicalBufferBytesReserved == 0 || snapshot.Terminal || snapshot.Failure != nil {
			t.Fatalf("ASSERT_LOGICAL_BUFFER_DEFAULT_OFF_EXACT_ACCOUNT_OWNERSHIP got=%+v", snapshot)
		}
	}

	var got logicalBufferDefaultOffObservation
	got.result.Result, got.result.ServerError, got.result.Failure = string(result.Result), result.ServerError, result.Failure
	got.result.Messages, got.result.Bytes = result.Messages, result.Bytes
	got.result.RequestMessages, got.result.RequestBytes = result.RequestMessages, result.RequestBytes
	got.result.Notifications, got.result.Responses = result.Notifications, result.Responses
	got.ownedSnapshot = c15.snapshot()
	requestLease, requestOK := result.PrivateB4RequestFrameLease()
	responseLease, responseOK := result.PrivateB4ResponseFrameLease()
	resultLease, resultOK := result.PrivateB4ResultLease()
	got.requestLease, got.responseLease, got.resultLease = requestOK, responseOK, resultOK
	if !got.requestLease || !got.responseLease || !got.resultLease || got.ownedSnapshot.Live == 0 {
		t.Fatalf("ASSERT_LOGICAL_BUFFER_DEFAULT_OFF_LEASE_PRESENCE observation=%+v", got)
	}
	if err := requestLease.WithBytes(func(b []byte) error { got.requestFrame = append([]byte(nil), b...); return nil }); err != nil {
		t.Fatal(err)
	}
	if err := responseLease.WithBytes(func(b []byte) error { got.responseFrame = append([]byte(nil), b...); return nil }); err != nil {
		t.Fatal(err)
	}
	if err := resultLease.WithBytes(func(b []byte) error { got.resultBytes = append([]byte(nil), b...); return nil }); err != nil {
		t.Fatal(err)
	}
	got.capture, got.consumeStatus = consumePrivateB4SnapshotForTest(f.m, managerLease, selection)
	for i := range got.capture.TargetSources {
		got.capture.TargetSources[i].AcquisitionID = ""
	}
	consumedSnapshot := c15.snapshot()
	got.consumeDelta = logicalBufferDelta(got.ownedSnapshot, consumedSnapshot)
	got.releases = [3]bool{requestLease.Release(), responseLease.Release(), resultLease.Release()}
	releasedSnapshot := c15.snapshot()
	got.releaseDelta = logicalBufferDelta(consumedSnapshot, releasedSnapshot)
	if got.consumeStatus != PrivateB4Selected || got.releases != [3]bool{true, true, true} || releasedSnapshot.Live != 0 {
		t.Fatalf("ASSERT_LOGICAL_BUFFER_DEFAULT_OFF_RELEASE_TRANSITIONS observation=%+v consumed=%+v released=%+v", got, consumedSnapshot, releasedSnapshot)
	}
	return got
}

func TestADR0011C17SuccessorLogicalBufferDefaultOffMatchesBaselineExactly(t *testing.T) {
	baseline := observeLogicalBufferDefaultOff(t, false)
	disabled := observeLogicalBufferDefaultOff(t, true)
	if !reflect.DeepEqual(disabled, baseline) {
		t.Fatalf("ASSERT_LOGICAL_BUFFER_DEFAULT_OFF_BASELINE_PARITY\nbaseline=%+v\ndisabled=%+v", baseline, disabled)
	}
}

func TestADR0011C17SuccessorLogicalBufferOverflowPrecedesC15ReservationAndAllocation(t *testing.T) {
	const identity = "allocation-overflow"
	work := adr0011work.New(identity)
	if err := work.ChargeLogicalBufferBytesReserved(identity, math.MaxUint64); err != nil {
		t.Fatal(err)
	}
	owner, c15 := logicalBufferOwner(t, identity, work)
	before := c15.snapshot()
	lease, err := owner.ReserveSuccessorAllocation(lspwire.SuccessorAllocationOriginalFrame, 1)
	if err == nil || lease != nil || c15.snapshot() != before || !errors.Is(work.Snapshot().Failure, adr0011work.ErrOverflow) {
		t.Fatalf("ASSERT_LOGICAL_BUFFER_OVERFLOW_PRE_C15 lease=%v err=%v before=%+v after=%+v work=%+v", lease, err, before, c15.snapshot(), work.Snapshot())
	}
	_ = session.ResourceExhausted // keep failure-domain distinction explicit in this integration contract
}
