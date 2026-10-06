package sessionruntime

import (
	"context"
	"testing"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

func TestPrivateB4AcceptedSuccessDecodedCustodyExactEqualityAndRelease(t *testing.T) {
	f := newFullP1Fixture(t)
	var ledger *privateB4ByteAccountV2
	var filler privateB4ByteLeaseV2
	var required uint64
	var before, owned, released privateB4ByteLedgerSnapshot
	f.m.b4DecodedHooks.reserve = func(got *privateB4ByteAccountV2, capacity uint64) {
		ledger, required, before = got, capacity, got.snapshot()
		var failure session.Failure
		filler, failure = got.reserve(privateB4MaxOwnedBytes - before.Live - capacity)
		if failure != "" {
			t.Fatalf("ASSERT_C15_DECODED_EQUALITY_FILL failure=%q", failure)
		}
	}
	f.m.b4DecodedHooks.owned = func(got *privateB4ByteAccountV2, capacity uint64) {
		if got != ledger || capacity != required {
			t.Fatal("ASSERT_C15_DECODED_HOOK_IDENTITY")
		}
		owned = got.snapshot()
	}
	f.m.b4DecodedHooks.released = func(got *privateB4ByteAccountV2) { released = got.snapshot() }
	result, managerLease, selection := f.transact(t)
	if required == 0 || owned.Live != privateB4MaxOwnedBytes || owned.Cumulative != before.Cumulative+(privateB4MaxOwnedBytes-before.Live-required)+required {
		t.Fatalf("ASSERT_C15_DECODED_EQUALITY required=%d before=%+v owned=%+v", required, before, owned)
	}
	frameLease, ok := result.PrivateB4RequestFrameLease()
	if !ok {
		t.Fatal("ASSERT_C15_DECODED_REQUEST_FRAME_LEASE")
	}
	responseLease, ok := result.PrivateB4ResponseFrameLease()
	if !ok {
		t.Fatal("ASSERT_C15_DECODED_RESPONSE_FRAME_LEASE")
	}
	resultLease, ok := result.PrivateB4ResultLease()
	if !ok {
		t.Fatal("ASSERT_C15_DECODED_RESULT_LEASE")
	}
	if released.Live != owned.Live-required || released.Cumulative != owned.Cumulative {
		t.Fatalf("ASSERT_C15_DECODED_RELEASE before=%+v released=%+v required=%d", owned, released, required)
	}
	if _, status := consumePrivateB4SnapshotForTest(f.m, managerLease, selection); status != PrivateB4Selected {
		t.Fatalf("ASSERT_C15_DECODED_CONSUME status=%s", status)
	}
	filler.release()
	if !frameLease.Release() || !responseLease.Release() || !resultLease.Release() || ledger.snapshot().Live != 0 {
		t.Fatalf("ASSERT_C15_DECODED_FINAL_CLEANUP snapshot=%+v", ledger.snapshot())
	}
}

func TestPrivateB4AcceptedSuccessDecodedCustodyPlusOneRefusesAtomically(t *testing.T) {
	f := newFullP1Fixture(t)
	var ledger *privateB4ByteAccountV2
	var filler privateB4ByteLeaseV2
	var before privateB4ByteLedgerSnapshot
	ownedCalls := 0
	var refused privateB4ByteLedgerSnapshot
	f.m.b4DecodedHooks.reserve = func(got *privateB4ByteAccountV2, capacity uint64) {
		ledger = got
		remaining := privateB4MaxOwnedBytes - got.snapshot().Live
		var failure session.Failure
		filler, failure = got.reserve(remaining - capacity + 1)
		if failure != "" {
			t.Fatalf("ASSERT_C15_DECODED_PLUS_ONE_FILL failure=%q", failure)
		}
		before = got.snapshot()
	}
	f.m.b4DecodedHooks.owned = func(*privateB4ByteAccountV2, uint64) { ownedCalls++ }
	f.m.b4DecodedHooks.refused = func(got *privateB4ByteAccountV2, _ uint64) { refused = got.snapshot() }
	result, managerLease := f.m.RoundTripPrivateB4(context.Background(), f.req, f.owner)
	if result.Failure != session.ResourceExhausted || managerLease != (B4DefinitionLease{}) || ownedCalls != 0 {
		t.Fatalf("ASSERT_C15_DECODED_PLUS_ONE result=%+v lease=%+v owned=%d", result, managerLease, ownedCalls)
	}
	if refused != before {
		t.Fatalf("ASSERT_C15_DECODED_PLUS_ONE_ATOMIC before=%+v refused=%+v", before, refused)
	}
	filler.release()
	if ledger.snapshot().Live != 0 {
		t.Fatalf("ASSERT_C15_DECODED_PLUS_ONE_CLEANUP snapshot=%+v", ledger.snapshot())
	}
}

func TestPrivateB4DecodedCustodyNotAcquiredBeforeAcceptedSuccess(t *testing.T) {
	f := newFullP1FixtureWithNotifications(t, 1)
	f.req.MaxMessages = 2
	calls := 0
	f.m.b4DecodedHooks.reserve = func(*privateB4ByteAccountV2, uint64) { calls++ }
	result, managerLease, selection := f.transact(t)
	notificationLease, notificationsOK := result.PrivateRetainedNotificationLease()
	if calls != 1 || len(result.Notifications) != 0 || !notificationsOK {
		t.Fatalf("ASSERT_C15_DECODED_WINNER_ONLY calls=%d legacy_notifications=%d lease=%t", calls, len(result.Notifications), notificationsOK)
	}
	if err := notificationLease.WithNotifications(func(messages []lspwire.Message) error {
		if len(messages) != 1 {
			t.Fatalf("ASSERT_C15_DECODED_NOTIFICATION_COUNT %d", len(messages))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, status := preparePrivateB4SnapshotForTest(f.m, managerLease, selection); status != PrivateB4Selected {
			t.Fatalf("ASSERT_C15_DECODED_RETRY_%d status=%s", i, status)
		}
	}
	if calls != 1 {
		t.Fatalf("ASSERT_C15_DECODED_RETRY_REACQUIRE calls=%d", calls)
	}
	frameLease, _ := result.PrivateB4RequestFrameLease()
	responseLease, _ := result.PrivateB4ResponseFrameLease()
	resultLease, _ := result.PrivateB4ResultLease()
	_, _ = consumePrivateB4SnapshotForTest(f.m, managerLease, selection)
	frameLease.Release()
	responseLease.Release()
	resultLease.Release()
	notificationLease.Release()
}
