package sessionruntime

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"

	"lsp-trace/internal/adr0011work"
)

func c17ObjectsMaterializedFixture(t *testing.T, identity string) (*c16ActivationFixture, context.Context, *adr0011work.Account) {
	t.Helper()
	f := newC16ActivationFixture(t)
	f.req.EnablePrivateC16ObjectAccounting = false
	f.owner.Transaction = identity
	account := adr0011work.New(identity)
	ctx := adr0011work.WithAccount(context.Background(), account)
	got, ok := adr0011work.AccountFromContext(ctx)
	if !ok || got != account {
		t.Fatalf("BLOCKED_NOT_RED: context account identity got=%p want=%p ok=%v", got, account, ok)
	}
	return f, ctx, account
}

func c17ObjectsMaterializedLease(t *testing.T, f *c16ActivationFixture, ctx context.Context) (RoundTripResult, B4DefinitionLease, B4DefinitionSelectionKey) {
	t.Helper()
	result, lease := f.m.RoundTripPrivateB4(ctx, f.req, f.owner)
	if result.Failure != "" || result.ServerError != nil || lease == (B4DefinitionLease{}) {
		t.Fatalf("BLOCKED_NOT_RED: private B4 manager fixture failure=%s server=%v lease=%v", result.Failure, result.ServerError, lease != (B4DefinitionLease{}))
	}
	selection := B4DefinitionSelectionKey{SessionID: f.req.SessionID, Key: result.Key, Transaction: f.owner.Transaction, CompletedOwnerKey: f.owner.CompletedOwnerKey}
	return result, lease, selection
}

func TestADR0011C17ObjectsMaterializedManagerBorrowCausalBoundary(t *testing.T) {
	f, ctx, account := c17ObjectsMaterializedFixture(t, "objects-two")
	_, lease, selection := c17ObjectsMaterializedLease(t, f, ctx)

	f.m.mu.Lock()
	reservation := f.m.privateB4ReservationLocked(lease.token)
	if reservation == nil || reservation.workAccount != account {
		f.m.mu.Unlock()
		t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_RESERVATION_EXACT_POINTER got=%p want=%p", reservation.workAccount, account)
	}
	transaction := f.m.privateB4Leases[reservation.slot]
	if transaction.workAccount != account {
		f.m.mu.Unlock()
		t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_TRANSACTION_EXACT_POINTER got=%p want=%p", transaction.workAccount, account)
	}
	f.m.mu.Unlock()

	seen := 0
	_, status := f.m.ConsumePrivateB4DefinitionBorrowed(lease, selection, func(b PrivateB4DefinitionBorrow) bool {
		if b.workAccount != account {
			t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_BORROW_EXACT_POINTER got=%p want=%p", b.workAccount, account)
		}
		for range 2 { // equal candidate values remain two distinct parser ordinals
			if err := b.WithObjectAdmission(func() error { seen++; return nil }); err != nil {
				t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_WITHOUT_C16 err=%v", err)
			}
		}
		if got := account.Snapshot(); got.ObjectsMaterialized != 2 || got.Terminal {
			t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_TWO_ACTIVE snapshot=%+v", got)
		}
		return true
	})
	if status != PrivateB4Selected || seen != 2 {
		t.Fatalf("BLOCKED_NOT_RED: manager borrow did not complete status=%s seen=%d", status, seen)
	}
	if got := account.Snapshot(); got.ObjectsMaterialized != 2 || !got.Terminal {
		t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_TERMINAL_AT_OWNERSHIP_END snapshot=%+v", got)
	}
}

func TestADR0011C17ObjectsMaterializedAdmissionFailureReplayAndInvalidBoundaries(t *testing.T) {
	f, ctx, account := c17ObjectsMaterializedFixture(t, "objects-replay")
	_, lease, selection := c17ObjectsMaterializedLease(t, f, ctx)
	later := errors.New("later parser admission failure")
	_, status := f.m.PreparePrivateB4DefinitionBorrowed(lease, selection, func(b PrivateB4DefinitionBorrow) bool {
		if err := b.WithObjectAdmission(func() error { return nil }); err != nil {
			t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_FIRST_CONSTRUCTION err=%v", err)
		}
		if err := b.WithObjectAdmission(func() error { return later }); !errors.Is(err, later) {
			t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_LATER_FAILURE_TYPED err=%v", err)
		}
		if got := account.Snapshot(); got.ObjectsMaterialized != 1 || got.Terminal {
			t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_LATER_FAILURE_NO_REFUND snapshot=%+v", got)
		}
		return true
	})
	if status != PrivateB4Selected {
		t.Fatalf("BLOCKED_NOT_RED: reusable prepare status=%s", status)
	}
	_, status = f.m.ConsumePrivateB4DefinitionBorrowed(lease, selection, func(b PrivateB4DefinitionBorrow) bool {
		if err := b.WithObjectAdmission(func() error { return nil }); err != nil {
			t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_REPLAY_CONSTRUCTION err=%v", err)
		}
		return true
	})
	if status != PrivateB4Selected || account.Snapshot().ObjectsMaterialized != 2 {
		t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_REPLAY_COUNTS_AGAIN status=%s snapshot=%+v", status, account.Snapshot())
	}

	for _, name := range []string{"null", "empty", "malformed"} {
		t.Run(name, func(t *testing.T) {
			invalid := adr0011work.New("invalid-" + name)
			if got := invalid.Snapshot().ObjectsMaterialized; got != 0 {
				t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_INVALID_ZERO case=%s got=%d", name, got)
			}
		})
	}
}

func TestADR0011C17ObjectsMaterializedIdentityAndOverflowPrecedeMaterialization(t *testing.T) {
	_, _, account := c17ObjectsMaterializedFixture(t, "objects-owner")
	called := false
	mismatch := PrivateB4DefinitionBorrow{workAccount: account, workIdentity: "different-owner"}
	if err := mismatch.WithObjectAdmission(func() error { called = true; return nil }); !errors.Is(err, adr0011work.ErrIdentityMismatch) {
		t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_IDENTITY_PRE_EFFECT err=%v", err)
	}
	if called || account.Snapshot().ObjectsMaterialized != 0 {
		t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_IDENTITY_CALLBACK_SUPPRESSED called=%t snapshot=%+v", called, account.Snapshot())
	}

	overflow := adr0011work.New("objects-overflow")
	if err := overflow.ChargeObjectsMaterialized("objects-overflow", math.MaxUint64); err != nil {
		t.Fatalf("BLOCKED_NOT_RED: overflow fixture max seed: %v", err)
	}
	called = false
	borrow := PrivateB4DefinitionBorrow{workAccount: overflow, workIdentity: "objects-overflow"}
	if err := borrow.WithObjectAdmission(func() error { called = true; return nil }); !errors.Is(err, adr0011work.ErrOverflow) {
		t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_OVERFLOW_TYPED err=%v", err)
	}
	if got := overflow.Snapshot(); called || !got.Terminal || got.ObjectsMaterialized != math.MaxUint64 {
		t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_OVERFLOW_SUPPRESSES_CALLBACK called=%t snapshot=%+v", called, got)
	}
}

func TestADR0011C17ObjectsMaterializedDefaultOffResultLeaseParity(t *testing.T) {
	plain := newC16ActivationFixture(t)
	plainResult, plainLease := plain.m.RoundTripPrivateB4(context.Background(), plain.req, plain.owner)
	accounted, ctx, account := c17ObjectsMaterializedFixture(t, plain.owner.Transaction)
	accountedResult, accountedLease := accounted.m.RoundTripPrivateB4(ctx, accounted.req, accounted.owner)
	if !reflect.DeepEqual(c18MaterialResult(plainResult), c18MaterialResult(accountedResult)) ||
		(plainLease == (B4DefinitionLease{})) != (accountedLease == (B4DefinitionLease{})) {
		t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_DEFAULT_OFF_PARITY plain=%+v accounted=%+v leases=%v/%v", c18MaterialResult(plainResult), c18MaterialResult(accountedResult), plainLease != (B4DefinitionLease{}), accountedLease != (B4DefinitionLease{}))
	}
	if got := account.Snapshot(); got.ObjectsMaterialized != 0 || got.Terminal {
		t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_ACCOUNT_ACTIVE_WITHOUT_C16 snapshot=%+v", got)
	}
}
