package sessionruntime

import (
	"context"
	"testing"

	"lsp-trace/internal/session"
)

func TestPrivateB4C17TransactionSlotActivationAndRelease(t *testing.T) {
	bytes, failure := newPrivateB4ByteAccountV2()
	if failure != "" {
		t.Fatal(failure)
	}
	c16, failure := newPrivateB4AccountC16WithBytes(bytes)
	if failure != "" {
		t.Fatal(failure)
	}
	c17, failure := newPrivateB4EventAccountC17WithBytes(bytes)
	if failure != "" {
		t.Fatal(failure)
	}
	reservation := &privateB4Reservation{slot: 0, maxCharge: 1, explicitBytes: bytes}
	reservation.token[0] = 1
	transaction := &privateB4Transaction{
		legacy: reservation,
		c16:    &privateB4ReservationC16{legacy: reservation, profile: c16},
		c17:    &privateB4ReservationC17{legacy: reservation, profile: c17},
	}
	manager := &Manager{}
	manager.installPrivateB4TransactionLocked(RoundTripRequest{}, nil, transaction)
	slot := manager.privateB4Leases[0]
	if slot.reservation != reservation || slot.c16 != transaction.c16 || slot.c17 != transaction.c17 {
		t.Fatalf("ASSERT_C17_TRANSACTION_SLOT_CUSTODY slot=%+v", slot)
	}
	before := bytes.snapshot()
	if before.Live == 0 {
		t.Fatalf("ASSERT_C17_SHARED_ACCOUNT_LIVE snapshot=%+v", before)
	}
	manager.releasePrivateB4Locked(reservation)
	if manager.privateB4Leases[0] != (privateB4Slot{}) || manager.privateB4Bytes != 0 {
		t.Fatalf("ASSERT_C17_TRANSACTION_SLOT_RELEASE slot=%+v bytes=%d", manager.privateB4Leases[0], manager.privateB4Bytes)
	}
	if snapshot := c17.eventSnapshot(); !snapshot.TerminalRequested || !snapshot.BackingReleased {
		t.Fatalf("ASSERT_C17_TRANSACTION_TERMINAL snapshot=%+v", snapshot)
	}
	if snapshot := c16.objectSnapshot(); !snapshot.TerminalRequested || !snapshot.BackingReleased {
		t.Fatalf("ASSERT_C17_PRESERVES_C16_TERMINAL snapshot=%+v", snapshot)
	}
	if snapshot := bytes.snapshot(); snapshot.Live != 0 {
		t.Fatalf("ASSERT_C17_SHARED_ACCOUNT_RELEASED snapshot=%+v", snapshot)
	}
}

func TestPrivateB4C17OrdinaryPathRejectsPrivateMarker(t *testing.T) {
	result := (&Manager{}).RoundTrip(context.Background(), RoundTripRequest{EnablePrivateC17EventAccounting: true})
	if result.Failure != session.ToolNotImplemented {
		t.Fatalf("ASSERT_C17_NON_PRIVATE_MARKER_REJECTED failure=%s", result.Failure)
	}
}

func TestPrivateB4C17DefaultOffSlotIsolation(t *testing.T) {
	bytes, failure := newPrivateB4ByteAccountV2()
	if failure != "" {
		t.Fatal(failure)
	}
	reservation := &privateB4Reservation{slot: 0, explicitBytes: bytes}
	reservation.token[0] = 1
	manager := &Manager{}
	manager.installPrivateB4TransactionLocked(RoundTripRequest{}, nil, &privateB4Transaction{legacy: reservation})
	if slot := manager.privateB4Leases[0]; slot.c16 != nil || slot.c17 != nil {
		t.Fatalf("ASSERT_C17_DEFAULT_OFF_SLOT slot=%+v", slot)
	}
	manager.releasePrivateB4Locked(reservation)
}

func TestPrivateB4C17RetirementJoinReleasesOutsideManagerLock(t *testing.T) {
	bytes, failure := newPrivateB4ByteAccountV2()
	if failure != "" {
		t.Fatal(failure)
	}
	c17, failure := newPrivateB4EventAccountC17WithBytes(bytes)
	if failure != "" {
		t.Fatal(failure)
	}
	reservation := &privateB4Reservation{slot: 0, explicitBytes: bytes}
	reservation.token[0] = 1
	manager := &Manager{}
	manager.privateB4Leases[0] = privateB4Slot{
		occupied: true, token: reservation.token, reservation: reservation,
		c17: &privateB4ReservationC17{legacy: reservation, profile: c17},
	}
	join := privateB4RetirementJoin{manager: manager, accountingCount: 1}
	join.accountingReservations[0] = reservation
	join.c17Profiles[0] = manager.privateB4Leases[0].c17

	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- c17.WithEventAdmission(c17Event(0), func() error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered
	joined := make(chan struct{})
	go func() {
		join.wait()
		close(joined)
	}()
	manager.mu.Lock()
	manager.mu.Unlock()
	select {
	case <-joined:
		t.Fatal("ASSERT_C17_RETIREMENT_JOIN_WAITS_FOR_CALLBACK")
	default:
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	<-joined
	if snapshot := c17.eventSnapshot(); !snapshot.BackingReleased {
		t.Fatalf("ASSERT_C17_RETIREMENT_RELEASE snapshot=%+v", snapshot)
	}
	if snapshot := bytes.snapshot(); snapshot.Live != 0 {
		t.Fatalf("ASSERT_C17_RETIREMENT_SHARED_BYTES snapshot=%+v", snapshot)
	}
	if failure := c17.requestTerminalReleaseC17(context.Background()); failure != "" {
		t.Fatal(failure)
	}
}
