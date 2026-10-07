package adr0011work_test

import (
	"errors"
	"math"
	"testing"
	"time"

	"lsp-trace/internal/adr0011work"
)

func TestObjectsMaterializedReservationCommitsOnlyConfirmedItem(t *testing.T) {
	account := adr0011work.New("occurrence-a")
	if got := account.Snapshot(); got.ObjectsMaterialized != 0 {
		t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_NEW_ZERO got=%d", got.ObjectsMaterialized)
	}
	inside := uint64(99)
	if err := account.WithObjectMaterialization("occurrence-a", func() error {
		inside = account.Snapshot().ObjectsMaterialized
		return nil // the caller has successfully constructed one Item
	}); err != nil {
		t.Fatal(err)
	}
	if got := account.Snapshot(); inside != 0 || got.ObjectsMaterialized != 1 {
		t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_COMMIT_AFTER_EXISTENCE inside=%d after=%d", inside, got.ObjectsMaterialized)
	}
}

func TestObjectsMaterializedReservationRefusalPrecedesEffectAndIsTerminal(t *testing.T) {
	account := adr0011work.New("occurrence-a")
	if err := account.ChargeObjectsMaterialized("occurrence-a", math.MaxUint64); err != nil {
		t.Fatal(err)
	}
	before := account.Snapshot()
	called := false
	if err := account.WithObjectMaterialization("occurrence-a", func() error { called = true; return nil }); !errors.Is(err, adr0011work.ErrOverflow) {
		t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_OVERFLOW_ERROR got=%v", err)
	}
	after := account.Snapshot()
	if called || after.ObjectsMaterialized != before.ObjectsMaterialized || !after.Terminal || !errors.Is(after.Failure, adr0011work.ErrOverflow) {
		t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_OVERFLOW_PRE_EFFECT called=%t before=%+v after=%+v", called, before, after)
	}
	if err := account.WithObjectMaterialization("occurrence-a", func() error { t.Fatal("ASSERT_OBJECTS_MATERIALIZED_TERMINAL_CALLBACK"); return nil }); !errors.Is(err, adr0011work.ErrTerminal) {
		t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_OVERFLOW_TERMINAL got=%v", err)
	}
}

func TestObjectsMaterializedReservationIdentityPrecedesEffect(t *testing.T) {
	account := adr0011work.New("occurrence-a")
	called := false
	if err := account.WithObjectMaterialization("occurrence-b", func() error { called = true; return nil }); !errors.Is(err, adr0011work.ErrIdentityMismatch) {
		t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_IDENTITY_ERROR got=%v", err)
	}
	if got := account.Snapshot(); called || got.ObjectsMaterialized != 0 || got.Terminal {
		t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_IDENTITY_PRE_EFFECT called=%t snapshot=%+v", called, got)
	}
}

func TestObjectsMaterializedReservationCallbackErrorRollsBack(t *testing.T) {
	account := adr0011work.New("occurrence-a")
	callbackErr := errors.New("item not constructed")
	if err := account.WithObjectMaterialization("occurrence-a", func() error { return callbackErr }); !errors.Is(err, callbackErr) {
		t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_CALLBACK_ERROR got=%v", err)
	}
	if got := account.Snapshot(); got.ObjectsMaterialized != 0 || got.Terminal {
		t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_CALLBACK_ROLLBACK snapshot=%+v", got)
	}
}

func TestObjectsMaterializedReservationPanicReleasesAndRepanics(t *testing.T) {
	account := adr0011work.New("occurrence-a")
	panicValue := &struct{ label string }{"materialize panic"}
	func() {
		defer func() {
			if got := recover(); got != panicValue {
				t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_REPANIC got=%#v", got)
			}
		}()
		_ = account.WithObjectMaterialization("occurrence-a", func() error { panic(panicValue) })
	}()
	if got := account.Snapshot(); got.ObjectsMaterialized != 0 || got.Terminal {
		t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_PANIC_RELEASE snapshot=%+v", got)
	}
	if err := account.WithObjectMaterialization("occurrence-a", func() error { return nil }); err != nil {
		t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_AFTER_PANIC_REUSABLE got=%v", err)
	}
}

func TestObjectsMaterializedSealWaitsForAdmittedCallbackThenFreezes(t *testing.T) {
	account := adr0011work.New("occurrence-a")
	entered := make(chan struct{})
	release := make(chan struct{})
	materialized := make(chan error, 1)
	go func() {
		materialized <- account.WithObjectMaterialization("occurrence-a", func() error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered

	sealed := make(chan adr0011work.Snapshot, 1)
	go func() { sealed <- account.Seal() }()
	select {
	case got := <-sealed:
		t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_SEAL_WAITED early=%+v", got)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if err := <-materialized; err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-sealed:
		if got.ObjectsMaterialized != 1 || !got.Terminal {
			t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_SEAL_AFTER_CALLBACK snapshot=%+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("ASSERT_OBJECTS_MATERIALIZED_SEAL_COMPLETES")
	}
	if err := account.WithObjectMaterialization("occurrence-a", func() error { t.Fatal("ASSERT_OBJECTS_MATERIALIZED_POST_SEAL_CALLBACK"); return nil }); !errors.Is(err, adr0011work.ErrTerminal) {
		t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_POST_SEAL_REJECTED got=%v", err)
	}
}
