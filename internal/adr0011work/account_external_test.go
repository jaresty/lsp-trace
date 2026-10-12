package adr0011work_test

import (
	"context"
	"errors"
	"math"
	"testing"

	"lsp-trace/internal/adr0011work"
)

func TestPrivateWorkInternalBridgePreservesIdentityAndTypedRows(t *testing.T) {
	account := adr0011work.New("occurrence-a")
	if account == nil {
		t.Fatal("ASSERT_PRIVATE_WORK_BRIDGE_ACCOUNT")
	}
	ctx := adr0011work.WithAccount(context.Background(), account)
	borrowed, ok := adr0011work.AccountFromContext(ctx)
	if !ok || borrowed != account {
		t.Fatal("ASSERT_PRIVATE_WORK_BRIDGE_EXACT_BORROW")
	}
	if err := borrowed.ChargeMessagesProcessed("occurrence-a", 2); err != nil {
		t.Fatal(err)
	}
	if err := borrowed.ChargeDocumentsAcquired("occurrence-a", 1); err != nil {
		t.Fatal(err)
	}
	s := borrowed.Snapshot()
	if s.Identity != "occurrence-a" || s.MessagesProcessed != 2 || s.DocumentsAcquired != 1 || s.BytesScanned != 0 || s.ObjectsMaterialized != 0 || s.EventsEmitted != 0 || s.LogicalBufferBytesReserved != 0 || s.Terminal || s.Failure != nil {
		t.Fatalf("ASSERT_PRIVATE_WORK_BRIDGE_TYPED_SNAPSHOT: %+v", s)
	}
}

func TestPrivateWorkInternalBridgeRefusesReplacementOverflowAndTerminalMutation(t *testing.T) {
	first := adr0011work.New("occurrence-a")
	second := adr0011work.New("occurrence-b")
	ctx := adr0011work.WithAccount(context.Background(), first)
	if got := adr0011work.WithAccount(ctx, second); got != ctx {
		t.Fatal("ASSERT_PRIVATE_WORK_BRIDGE_NO_REPLACEMENT")
	}
	if err := first.ChargeBytesScanned("occurrence-b", 1); !errors.Is(err, adr0011work.ErrIdentityMismatch) {
		t.Fatalf("identity error = %v", err)
	}
	if err := first.ChargeBytesScanned("occurrence-a", math.MaxUint64); err != nil {
		t.Fatal(err)
	}
	if err := first.ChargeBytesScanned("occurrence-a", 1); !errors.Is(err, adr0011work.ErrOverflow) {
		t.Fatalf("overflow error = %v", err)
	}
	s := first.Snapshot()
	if s.BytesScanned != math.MaxUint64 || !s.Terminal || !errors.Is(s.Failure, adr0011work.ErrOverflow) {
		t.Fatalf("ASSERT_PRIVATE_WORK_BRIDGE_OVERFLOW_ZERO_EFFECT: %+v", s)
	}
	if err := first.ChargeEventsEmitted("occurrence-a", 1); !errors.Is(err, adr0011work.ErrTerminal) {
		t.Fatalf("terminal error = %v", err)
	}
	sealed := first.Seal()
	if sealed != first.Snapshot() {
		t.Fatal("ASSERT_PRIVATE_WORK_BRIDGE_STABLE_SEAL")
	}
}

func TestPrivateWorkInternalBridgeAbsentContext(t *testing.T) {
	if account, ok := adr0011work.AccountFromContext(context.Background()); ok || account != nil {
		t.Fatal("ASSERT_PRIVATE_WORK_BRIDGE_ABSENT")
	}
}
