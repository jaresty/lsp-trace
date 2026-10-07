package sessionruntime

import (
	"errors"
	"math"
	"testing"

	"lsp-trace/internal/adr0011work"
)

func c17TargetEventsBorrow(t *testing.T, identity string) (PrivateB4DefinitionBorrow, *privateB4EventAccountC17, *adr0011work.Account) {
	t.Helper()
	profile, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatalf("BLOCKED_NOT_RED: C17 target profile: %s", failure)
	}
	account := adr0011work.New(identity)
	return PrivateB4DefinitionBorrow{
		c17:          profile,
		workAccount:  account,
		workIdentity: identity,
	}, profile, account
}

func TestADR0011C17TargetEventsEmittedAcceptedOriginalOrdinalsAndRetry(t *testing.T) {
	borrow, profile, account := c17TargetEventsBorrow(t, "target-events-ordinals")
	var appended []uint64
	for _, ordinal := range []uint64{41, 97} {
		ordinal := ordinal
		if err := borrow.WithTargetAppendAdmission(ordinal, func() error {
			appended = append(appended, ordinal)
			return nil
		}); err != nil {
			t.Fatalf("ASSERT_TARGET_EVENTS_ACCEPTED ordinal=%d err=%v", ordinal, err)
		}
	}
	if got := profile.eventSnapshot(); got.Admitted != 4 || got.InFlight != 0 {
		t.Fatalf("BLOCKED_NOT_RED: C17 canonical pair authority snapshot=%+v", got)
	}
	wantIdentities := []privateB4EventIdentityC17{
		{Family: "TARGET", Kind: "TARGET_BEGIN", Ordinal: 41},
		{Family: "TARGET", Kind: "TARGET_TERMINAL", Ordinal: 41},
		{Family: "TARGET", Kind: "TARGET_BEGIN", Ordinal: 97},
		{Family: "TARGET", Kind: "TARGET_TERMINAL", Ordinal: 97},
	}
	if got := privateB4C17CommittedIdentities(t, profile); len(got) != len(wantIdentities) {
		t.Fatalf("ASSERT_TARGET_EVENTS_ORIGINAL_ORDINAL_IDENTITIES got=%+v want=%+v", got, wantIdentities)
	} else {
		for i := range wantIdentities {
			if got[i] != wantIdentities[i] {
				t.Fatalf("ASSERT_TARGET_EVENTS_ORIGINAL_ORDINAL_IDENTITY index=%d got=%+v want=%+v", i, got[i], wantIdentities[i])
			}
		}
	}
	if got := account.Snapshot(); got.EventsEmitted != 4 || got.Terminal {
		t.Fatalf("ASSERT_TARGET_EVENTS_EXACT_TWO_PER_ORIGINAL_ORDINAL appended=%v snapshot=%+v", appended, got)
	}

	called := false
	if err := borrow.WithTargetAppendAdmission(41, func() error { called = true; return nil }); !errors.Is(err, errPrivateB4EventAdmission) {
		t.Fatalf("ASSERT_TARGET_EVENTS_DUPLICATE_TYPED err=%v", err)
	}
	if got := account.Snapshot(); called || got.EventsEmitted != 4 {
		t.Fatalf("ASSERT_TARGET_EVENTS_COMMITTED_RETRY_ZERO called=%t snapshot=%+v", called, got)
	}
}

func TestADR0011C17TargetEventsEmittedCallbackErrorAndPanicRollback(t *testing.T) {
	borrow, _, account := c17TargetEventsBorrow(t, "target-events-rollback")
	callbackErr := errors.New("canonical target append failed")
	if err := borrow.WithTargetAppendAdmission(11, func() error { return callbackErr }); !errors.Is(err, callbackErr) {
		t.Fatalf("ASSERT_TARGET_EVENTS_CALLBACK_ERROR err=%v", err)
	}
	if got := account.Snapshot(); got.EventsEmitted != 0 || got.Terminal {
		t.Fatalf("ASSERT_TARGET_EVENTS_CALLBACK_ERROR_ROLLBACK snapshot=%+v", got)
	}
	if err := borrow.WithTargetAppendAdmission(11, func() error { return nil }); err != nil {
		t.Fatalf("ASSERT_TARGET_EVENTS_ERROR_ORDINAL_REUSABLE err=%v", err)
	}
	if got := account.Snapshot().EventsEmitted; got != 2 {
		t.Fatalf("ASSERT_TARGET_EVENTS_ERROR_RETRY_EXACT_TWO got=%d", got)
	}

	panicValue := &struct{ name string }{"target append panic"}
	func() {
		defer func() {
			if got := recover(); got != panicValue {
				t.Fatalf("ASSERT_TARGET_EVENTS_CALLBACK_REPANIC got=%#v", got)
			}
		}()
		_ = borrow.WithTargetAppendAdmission(12, func() error { panic(panicValue) })
	}()
	if got := account.Snapshot(); got.EventsEmitted != 2 || got.Terminal {
		t.Fatalf("ASSERT_TARGET_EVENTS_CALLBACK_PANIC_ROLLBACK snapshot=%+v", got)
	}
	if err := borrow.WithTargetAppendAdmission(12, func() error { return nil }); err != nil {
		t.Fatalf("ASSERT_TARGET_EVENTS_PANIC_ORDINAL_REUSABLE err=%v", err)
	}
	if got := account.Snapshot().EventsEmitted; got != 4 {
		t.Fatalf("ASSERT_TARGET_EVENTS_PANIC_RETRY_EXACT_TWO got=%d", got)
	}
}

func TestADR0011C17TargetEventsEmittedExactWorkIdentityMismatchZeroEffect(t *testing.T) {
	borrow, profile, account := c17TargetEventsBorrow(t, "target-events-owner")
	borrow.workIdentity = "different-target-events-owner"
	beforeC17 := profile.eventSnapshot()
	beforeWork := account.Snapshot()
	called := false
	if err := borrow.WithTargetAppendAdmission(7, func() error { called = true; return nil }); !errors.Is(err, adr0011work.ErrIdentityMismatch) {
		t.Fatalf("ASSERT_TARGET_EVENTS_IDENTITY_MISMATCH_TYPED err=%v", err)
	}
	if afterC17, afterWork := profile.eventSnapshot(), account.Snapshot(); called || afterC17 != beforeC17 || afterWork != beforeWork {
		t.Fatalf("ASSERT_TARGET_EVENTS_IDENTITY_MISMATCH_ZERO_EFFECT called=%t c17_before=%+v c17_after=%+v work_before=%+v work_after=%+v", called, beforeC17, afterC17, beforeWork, afterWork)
	}
}

func TestADR0011C17TargetEventsEmittedOverflowPrecedesCandidateVisibility(t *testing.T) {
	borrow, profile, account := c17TargetEventsBorrow(t, "target-events-overflow")
	if err := account.ChargeEventsEmitted("target-events-overflow", math.MaxUint64-1); err != nil {
		t.Fatalf("BLOCKED_NOT_RED: seed account independently of C17 cap: %v", err)
	}
	visible := false
	err := borrow.WithTargetAppendAdmission(73, func() error {
		visible = true
		return nil
	})
	if !errors.Is(err, adr0011work.ErrOverflow) {
		t.Fatalf("ASSERT_TARGET_EVENTS_OVERFLOW_TYPED err=%v", err)
	}
	if got := account.Snapshot(); visible || got.EventsEmitted != math.MaxUint64-1 || !got.Terminal || !errors.Is(got.Failure, adr0011work.ErrOverflow) {
		t.Fatalf("ASSERT_TARGET_EVENTS_OVERFLOW_PRE_CALLBACK visible=%t snapshot=%+v", visible, got)
	}
	if got := profile.eventSnapshot(); got.Admitted != 0 || got.InFlight != 0 {
		t.Fatalf("ASSERT_TARGET_EVENTS_ACCOUNT_PRECEDES_C17_ADMISSION snapshot=%+v", got)
	}
}

func TestADR0011C17TargetEventsEmittedPartialRefusalAndLaterFailure(t *testing.T) {
	t.Run("later production borrow operation does not refund committed pair", func(t *testing.T) {
		f := newPrivateB4C17PublicationFixture(t)
		account := adr0011work.New(f.selection.Transaction)
		f.manager.mu.Lock()
		slot := f.manager.privateB4Leases[f.reservation.slot]
		slot.workAccount = account
		f.manager.privateB4Leases[f.reservation.slot] = slot
		f.manager.mu.Unlock()

		_, status, err := f.commit(func(borrow PrivateB4DefinitionBorrow) bool {
			return borrow.workAccount == account && borrow.workIdentity == f.selection.Transaction
		}, func(borrow PrivateB4DefinitionBorrow) error {
			if err := borrow.WithTargetAppendAdmission(509, func() error { return nil }); err != nil {
				return err
			}
			return borrow.WithTargetAppendAdmission(509, func() error {
				t.Fatal("ASSERT_TARGET_EVENTS_LATER_DUPLICATE_CALLBACK_NOT_RUN")
				return nil
			})
		})
		if status != PrivateB4Selected || !errors.Is(err, errPrivateB4EventAdmission) {
			t.Fatalf("ASSERT_TARGET_EVENTS_ACTUAL_POST_APPEND_FAILURE status=%s err=%v", status, err)
		}
		if got := account.Snapshot(); got.EventsEmitted != 2 || got.Terminal {
			t.Fatalf("ASSERT_TARGET_EVENTS_COMMITTED2_PERSISTS snapshot=%+v", got)
		}
	})
}

func TestADR0011C17TargetEventsEmittedDefaultOffUnchanged(t *testing.T) {
	account := adr0011work.New("target-events-default-off")
	called := false
	borrow := PrivateB4DefinitionBorrow{workAccount: account, workIdentity: "target-events-default-off"}
	if err := borrow.WithTargetAppendAdmission(7, func() error { called = true; return nil }); !errors.Is(err, ErrPrivateB4C17NotEnabled) {
		t.Fatalf("ASSERT_TARGET_EVENTS_DEFAULT_OFF_TYPED err=%v", err)
	}
	if got := account.Snapshot(); called || got.EventsEmitted != 0 || got.Terminal {
		t.Fatalf("ASSERT_TARGET_EVENTS_DEFAULT_OFF_ZERO_EFFECT called=%t snapshot=%+v", called, got)
	}
}
