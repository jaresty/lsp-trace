package sessionruntime

import (
	"context"
	"testing"
	"unsafe"

	"lsp-trace/internal/session"
)

func TestPrivateB4C16RejectsOrdinaryPathAndAtomicBootstrapFailure(t *testing.T) {
	result := (&Manager{}).RoundTrip(context.Background(), RoundTripRequest{EnablePrivateC16ObjectAccounting: true})
	if result.Failure != session.ToolNotImplemented {
		t.Fatalf("ASSERT_C16_NON_PRIVATE failure=%s", result.Failure)
	}
	account := mustPrivateB4ByteAccountV2(t)
	filler, failure := account.reserve(privateB4MaxOwnedBytes - privateB4ByteLedgerV2TableBytes)
	if failure != "" {
		t.Fatal(failure)
	}
	before := account.snapshot()
	profile, failure := newPrivateB4AccountC16WithBytes(account)
	if failure != session.ResourceExhausted || profile != nil || account.snapshot() != before {
		t.Fatalf("ASSERT_C16_BOOTSTRAP_ATOMIC profile=%v failure=%s before=%+v after=%+v", profile != nil, failure, before, account.snapshot())
	}
	filler.release()
	account.requestTerminalRelease()
}

func TestPrivateB4C16ExplicitActivation(t *testing.T) {
	legacySize := unsafe.Sizeof(privateB4Reservation{})
	legacy := newFullP1Fixture(t)
	legacyResult, legacyLease := legacy.m.RoundTripPrivateB4(context.Background(), legacy.req, legacy.owner)
	if legacyResult.Failure != "" || legacyLease == (B4DefinitionLease{}) {
		t.Fatalf("ASSERT_C16_MARKER_FALSE transaction=%+v", legacyResult)
	}
	legacy.m.mu.Lock()
	legacyReservation := legacy.m.privateB4ReservationLocked(legacyLease.token)
	legacySlot := legacy.m.privateB4Leases[legacyReservation.slot]
	legacy.m.mu.Unlock()
	if legacySlot.c16 != nil || unsafe.Sizeof(*legacyReservation) != legacySize {
		t.Fatalf("ASSERT_C16_MARKER_FALSE_ALLOCATION c16=%v size=%d/%d", legacySlot.c16 != nil, unsafe.Sizeof(*legacyReservation), legacySize)
	}

	enabled := newFullP1Fixture(t)
	enabled.req.EnablePrivateC16ObjectAccounting = true
	result, lease := enabled.m.RoundTripPrivateB4(context.Background(), enabled.req, enabled.owner)
	if result.Failure != "" || lease == (B4DefinitionLease{}) {
		t.Fatalf("ASSERT_C16_MARKER_TRUE transaction=%+v", result)
	}
	enabled.m.mu.Lock()
	reservation := enabled.m.privateB4ReservationLocked(lease.token)
	slot := enabled.m.privateB4Leases[reservation.slot]
	enabled.m.mu.Unlock()
	if slot.c16 == nil || slot.c16.legacy != reservation || slot.c16.profile == nil {
		t.Fatal("ASSERT_C16_DISTINCT_BOOTSTRAP")
	}
	if got := reservation.explicitBytes.snapshot(); got.Live < privateB4AccountC16SelfCost() {
		t.Fatalf("ASSERT_C16_SELF_COST %+v cost=%d", got, privateB4AccountC16SelfCost())
	}
	borrow := PrivateB4DefinitionBorrow{c16: slot.c16}
	copied := borrow
	if err := borrow.WithObjectAdmission(func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := copied.WithObjectAdmission(func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if got := slot.c16.profile.objectSnapshot(); got.Materialized != 2 {
		t.Fatalf("ASSERT_C16_COPY_CONVERGENCE %+v", got)
	}
}
