package directionalraw

import (
	"errors"
	"testing"
)

func TestBidirectionalRootPrivateCarrierSemantics(t *testing.T) {
	const exact32 = uint64(32 << 20)
	root, err := NewBidirectionalRootPrivate(NewReservation("carrier-unit-only"), BidirectionalPlanPrivate{IncomingBytes: exact32, OutgoingBytes: exact32})
	if errors.Is(err, ErrBehaviorNotImplementedPrivate) {
		t.Fatal("ASSERT_SKELETON_ERR_BEHAVIOR_NOT_IMPLEMENTED_DIRECTIONAL")
	}
	if err != nil {
		t.Fatal(err)
	}
	incoming, err := root.ChildPrivate("incoming")
	if err != nil {
		t.Fatal(err)
	}
	outgoing, err := root.ChildPrivate("outgoing")
	if err != nil {
		t.Fatal(err)
	}
	if err := incoming.ReservePrivate(exact32); err != nil {
		t.Fatalf("ASSERT_CARRIER_EXACT32_INCOMING: %v", err)
	}
	if err := outgoing.ReservePrivate(exact32); err != nil {
		t.Fatalf("ASSERT_CARRIER_EXACT32_OUTGOING: %v", err)
	}
	state, err := root.SnapshotPrivate()
	if err != nil {
		t.Fatal(err)
	}
	if state.IncomingAdmittedBytes != exact32 || state.OutgoingAdmittedBytes != exact32 || state.RootAdmittedBytes != 64<<20 {
		t.Fatalf("ASSERT_CUMULATIVE_ADMITTED_INCOMING32_OUTGOING32_ROOT64: %+v", state)
	}
	if state.IncomingLiveBytes != exact32 || state.OutgoingLiveBytes != exact32 || state.LiveBytes != 64<<20 {
		t.Fatalf("ASSERT_CARRIER_AGGREGATE64_DIRECTION_BOUND: %+v", state)
	}

	peerRoot, err := NewBidirectionalRootPrivate(NewReservation("carrier-no-transfer"), BidirectionalPlanPrivate{IncomingBytes: 1, OutgoingBytes: exact32 + 1})
	if err != nil {
		t.Fatal(err)
	}
	peerIncoming, err := peerRoot.ChildPrivate("incoming")
	if err != nil {
		t.Fatal(err)
	}
	peerOutgoing, err := peerRoot.ChildPrivate("outgoing")
	if err != nil {
		t.Fatal(err)
	}
	peerIncomingView, err := peerIncoming.ViewPrivate()
	if err != nil {
		t.Fatal(err)
	}
	peerOutgoingView, err := peerOutgoing.ViewPrivate()
	if err != nil {
		t.Fatal(err)
	}
	if err := peerIncoming.ReservePrivate(1); err != nil {
		t.Fatal(err)
	}
	if err := peerOutgoing.ReservePrivate(exact32 + 1); !errors.Is(err, ErrReservationRefused) {
		t.Fatalf("ASSERT_PLUS_ONE_EXACT_RESERVATION_REFUSED: err=%T %v", err, err)
	}
	peerReceipts, err := peerRoot.ReceiptsPrivate()
	if err != nil {
		t.Fatal(err)
	}
	if len(peerReceipts) != 2 || peerReceipts[0].Direction != "incoming" || peerReceipts[0].AdmittedBytes != 1 || peerReceipts[1].Direction != "outgoing" || !peerReceipts[1].RefusedBeforeWrite || peerReceipts[1].RequestBytes != 0 || peerReceipts[1].ResponseBytes != 0 || peerReceipts[1].AdmittedBytes != 0 {
		t.Fatalf("ASSERT_PLUS_ONE_ZERO_TARGET_DIRECTION_WRITE_BYTES: %+v", peerReceipts)
	}
	peerBeforeRelease, err := peerRoot.SnapshotPrivate()
	if err != nil {
		t.Fatal(err)
	}
	if peerBeforeRelease.IncomingAdmittedBytes != 1 || peerBeforeRelease.OutgoingAdmittedBytes != 0 || peerBeforeRelease.RootAdmittedBytes != 1 || peerBeforeRelease.IncomingLiveBytes != 1 || peerBeforeRelease.OutgoingLiveBytes != 0 || peerBeforeRelease.LiveBytes != 1 {
		t.Fatalf("ASSERT_CARRIER_NO_CROSS_TRANSFER: %+v", peerBeforeRelease)
	}
	if err := peerRoot.Release(); err != nil {
		t.Fatal(err)
	}
	peerReleased, err := peerRoot.SnapshotPrivate()
	if err != nil {
		t.Fatal(err)
	}
	if !peerReleased.Released || peerReleased.ReleaseCount != 1 || peerReleased.IncomingReleasedBytes != 1 || peerReleased.OutgoingReleasedBytes != 0 || peerReleased.RootReleasedBytes != 1 || peerReleased.IncomingLiveBytes != 0 || peerReleased.OutgoingLiveBytes != 0 || peerReleased.LiveBytes != 0 {
		t.Fatalf("ASSERT_PLUS_ONE_RELEASE_ONCE_ALL_LIVE_ZERO: %+v", peerReleased)
	}
	if _, valid, err := peerIncomingView.LiveBytesPrivate(); err != nil || valid {
		t.Fatalf("ASSERT_PLUS_ONE_INVALIDATES_INCOMING_VIEW: valid=%t err=%v", valid, err)
	}
	if _, valid, err := peerOutgoingView.LiveBytesPrivate(); err != nil || valid {
		t.Fatalf("ASSERT_PLUS_ONE_INVALIDATES_OUTGOING_VIEW: valid=%t err=%v", valid, err)
	}

	incomingView, err := incoming.ViewPrivate()
	if err != nil {
		t.Fatal(err)
	}
	outgoingView, err := outgoing.ViewPrivate()
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Release(); err != nil {
		t.Fatal(err)
	}
	released, err := root.SnapshotPrivate()
	if err != nil {
		t.Fatal(err)
	}
	if released.IncomingReleasedBytes != exact32 || released.OutgoingReleasedBytes != exact32 || released.RootReleasedBytes != 64<<20 {
		t.Fatalf("ASSERT_CUMULATIVE_RELEASED_INCOMING32_OUTGOING32_ROOT64: %+v", released)
	}
	if released.ReleaseCount != 1 || released.IncomingLiveBytes != 0 || released.OutgoingLiveBytes != 0 || released.LiveBytes != 0 {
		t.Fatalf("ASSERT_CARRIER_RELEASE_EXACTLY_ONCE_LIVE_ZERO: %+v", released)
	}
	if _, valid, err := incomingView.LiveBytesPrivate(); err != nil || valid {
		t.Fatalf("ASSERT_CARRIER_RELEASE_INVALIDATES_INCOMING_VIEW: valid=%t err=%v", valid, err)
	}
	if _, valid, err := outgoingView.LiveBytesPrivate(); err != nil || valid {
		t.Fatalf("ASSERT_CARRIER_RELEASE_INVALIDATES_OUTGOING_VIEW: valid=%t err=%v", valid, err)
	}
	if err := outgoing.ReservePrivate(1); !errors.Is(err, ErrReservationRefused) {
		t.Fatalf("ASSERT_CARRIER_RELEASE_INVALIDATES_ALLOCATION: %v", err)
	}
	for _, receipt := range peerReceipts {
		if receipt.PreparationBytes != 0 && receipt.AdmittedBytes == receipt.PreparationBytes {
			t.Fatalf("ASSERT_NO_TRANSFER_OR_PREPARATION_COUNTER_IN_CUMULATIVE_TOTALS: %+v", receipt)
		}
	}
}

func TestBidirectionalRootPrivateFalseTerminalReleaseTruth(t *testing.T) {
	reservation := NewReservation("false-terminal")
	terminalCalls := 0
	if !reservation.AttachPrivate(func(string, string, int) ([]byte, error) { return nil, nil }, func(Observation) {}, func() bool {
		terminalCalls++
		return terminalCalls > 1
	}, func() {}) {
		t.Fatal("ASSERT_FALSE_TERMINAL_ATTACH")
	}
	root, err := NewBidirectionalRootPrivate(reservation, BidirectionalPlanPrivate{})
	if err != nil {
		t.Fatal(err)
	}
	child, _ := root.ChildPrivate("incoming")
	if err := child.ReservePrivate(7); err != nil {
		t.Fatal(err)
	}
	if err := root.Release(); err == nil {
		t.Fatal("ASSERT_FALSE_TERMINAL_PROPAGATED")
	}
	failed, _ := root.SnapshotPrivate()
	if failed.Released || failed.ReleaseCount != 0 || failed.LiveBytes != 7 || failed.RootReleasedBytes != 0 || terminalCalls != 1 {
		t.Fatalf("ASSERT_FALSE_TERMINAL_ACCOUNTING_UNCHANGED: state=%+v calls=%d", failed, terminalCalls)
	}
	if err := root.Release(); err != nil {
		t.Fatal(err)
	}
	if err := root.Release(); err != nil {
		t.Fatal(err)
	}
	released, _ := root.SnapshotPrivate()
	if !released.Released || released.ReleaseCount != 1 || released.LiveBytes != 0 || released.RootReleasedBytes != 7 || terminalCalls != 2 {
		t.Fatalf("ASSERT_FIRST_SUCCESS_CONVERGES_DUPLICATE_IDEMPOTENT: state=%+v calls=%d", released, terminalCalls)
	}
}
