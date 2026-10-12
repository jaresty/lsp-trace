package adr0011work_test

import (
	"errors"
	"math"
	"runtime"
	"testing"
	"time"

	"lsp-trace/internal/adr0011work"
)

func waitForEventsAccountTerminal(t *testing.T, account *adr0011work.Account) {
	t.Helper()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	for !account.Snapshot().Terminal {
		select {
		case <-deadline.C:
			t.Fatal("ASSERT_EVENTS_EMITTED_TERMINAL_OBSERVED")
		default:
			runtime.Gosched()
		}
	}
}

func TestEventsEmittedReservationStartsZeroAndCommitsExactAmountAfterCallback(t *testing.T) {
	account := adr0011work.New("occurrence-a")
	if got := account.Snapshot().EventsEmitted; got != 0 {
		t.Fatalf("ASSERT_EVENTS_EMITTED_NEW_ZERO got=%d", got)
	}
	inside := uint64(99)
	if err := account.WithEventsEmitted("occurrence-a", 7, func() error {
		inside = account.Snapshot().EventsEmitted
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got := account.Snapshot(); inside != 0 || got.EventsEmitted != 7 {
		t.Fatalf("ASSERT_EVENTS_EMITTED_EXACT_COMMIT_AFTER_CALLBACK inside=%d snapshot=%+v", inside, got)
	}
}

func TestEventsEmittedReservationCallbackErrorRollsBack(t *testing.T) {
	account := adr0011work.New("occurrence-a")
	callbackErr := errors.New("events not emitted")
	if err := account.WithEventsEmitted("occurrence-a", 4, func() error { return callbackErr }); !errors.Is(err, callbackErr) {
		t.Fatalf("ASSERT_EVENTS_EMITTED_CALLBACK_ERROR got=%v", err)
	}
	if got := account.Snapshot(); got.EventsEmitted != 0 || got.Terminal {
		t.Fatalf("ASSERT_EVENTS_EMITTED_CALLBACK_ROLLBACK snapshot=%+v", got)
	}
	if err := account.WithEventsEmitted("occurrence-a", 2, func() error { return nil }); err != nil {
		t.Fatalf("ASSERT_EVENTS_EMITTED_AFTER_ERROR_REUSABLE got=%v", err)
	}
	if got := account.Snapshot().EventsEmitted; got != 2 {
		t.Fatalf("ASSERT_EVENTS_EMITTED_AFTER_ERROR_COMMIT got=%d", got)
	}
}

func TestEventsEmittedReservationPanicReleasesRepanicsAndAllowsReuse(t *testing.T) {
	account := adr0011work.New("occurrence-a")
	panicValue := &struct{ label string }{"emit panic"}
	func() {
		defer func() {
			if got := recover(); got != panicValue {
				t.Fatalf("ASSERT_EVENTS_EMITTED_REPANIC got=%#v", got)
			}
		}()
		_ = account.WithEventsEmitted("occurrence-a", 3, func() error { panic(panicValue) })
	}()
	if got := account.Snapshot(); got.EventsEmitted != 0 || got.Terminal {
		t.Fatalf("ASSERT_EVENTS_EMITTED_PANIC_RELEASE snapshot=%+v", got)
	}
	if err := account.WithEventsEmitted("occurrence-a", 2, func() error { return nil }); err != nil {
		t.Fatalf("ASSERT_EVENTS_EMITTED_AFTER_PANIC_REUSABLE got=%v", err)
	}
	if got := account.Snapshot().EventsEmitted; got != 2 {
		t.Fatalf("ASSERT_EVENTS_EMITTED_AFTER_PANIC_COMMIT got=%d", got)
	}
}

func TestEventsEmittedReservationMaxUintEqualitySucceeds(t *testing.T) {
	account := adr0011work.New("occurrence-a")
	if err := account.WithEventsEmitted("occurrence-a", math.MaxUint64, func() error { return nil }); err != nil {
		t.Fatalf("ASSERT_EVENTS_EMITTED_MAX_UINT_EQUALITY got=%v", err)
	}
	if got := account.Snapshot(); got.EventsEmitted != math.MaxUint64 || got.Terminal {
		t.Fatalf("ASSERT_EVENTS_EMITTED_MAX_UINT_COMMITTED snapshot=%+v", got)
	}
}

func TestEventsEmittedReservationOverflowIncludesConcurrentOutstandingAndPrecedesCallback(t *testing.T) {
	account := adr0011work.New("occurrence-a")
	entered := make(chan struct{})
	release := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- account.WithEventsEmitted("occurrence-a", math.MaxUint64, func() error {
			close(entered)
			<-release
			return errors.New("release without commit")
		})
	}()
	<-entered

	called := false
	if err := account.WithEventsEmitted("occurrence-a", 1, func() error { called = true; return nil }); !errors.Is(err, adr0011work.ErrOverflow) {
		t.Fatalf("ASSERT_EVENTS_EMITTED_OUTSTANDING_OVERFLOW_ERROR got=%v", err)
	}
	if got := account.Snapshot(); called || got.EventsEmitted != 0 || !got.Terminal || !errors.Is(got.Failure, adr0011work.ErrOverflow) {
		t.Fatalf("ASSERT_EVENTS_EMITTED_OUTSTANDING_OVERFLOW_PRE_CALLBACK called=%t snapshot=%+v", called, got)
	}
	close(release)
	if err := <-firstDone; err == nil {
		t.Fatal("ASSERT_EVENTS_EMITTED_FIRST_CALLBACK_ERROR")
	}
}

func TestEventsEmittedReservationIdentityMismatchIsNonterminalAndPrecedesCallback(t *testing.T) {
	account := adr0011work.New("occurrence-a")
	called := false
	if err := account.WithEventsEmitted("occurrence-b", 1, func() error { called = true; return nil }); !errors.Is(err, adr0011work.ErrIdentityMismatch) {
		t.Fatalf("ASSERT_EVENTS_EMITTED_IDENTITY_ERROR got=%v", err)
	}
	if got := account.Snapshot(); called || got.EventsEmitted != 0 || got.Terminal {
		t.Fatalf("ASSERT_EVENTS_EMITTED_IDENTITY_PRE_CALLBACK_NONTERMINAL called=%t snapshot=%+v", called, got)
	}
}

func TestEventsEmittedSealWaitsForReservationAndBlocksNewReservations(t *testing.T) {
	account := adr0011work.New("occurrence-a")
	entered := make(chan struct{})
	release := make(chan struct{})
	emitted := make(chan error, 1)
	go func() {
		emitted <- account.WithEventsEmitted("occurrence-a", 5, func() error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered

	sealed := make(chan adr0011work.Snapshot, 1)
	go func() { sealed <- account.Seal() }()
	waitForEventsAccountTerminal(t, account)
	if err := account.WithEventsEmitted("occurrence-a", 1, func() error {
		t.Fatal("ASSERT_EVENTS_EMITTED_SEAL_BLOCKS_NEW_CALLBACK")
		return nil
	}); !errors.Is(err, adr0011work.ErrTerminal) {
		t.Fatalf("ASSERT_EVENTS_EMITTED_SEAL_BLOCKS_NEW got=%v", err)
	}
	select {
	case got := <-sealed:
		t.Fatalf("ASSERT_EVENTS_EMITTED_SEAL_WAITED early=%+v", got)
	default:
	}
	close(release)
	if err := <-emitted; err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-sealed:
		if got.EventsEmitted != 5 || !got.Terminal {
			t.Fatalf("ASSERT_EVENTS_EMITTED_SEAL_AFTER_CALLBACK snapshot=%+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("ASSERT_EVENTS_EMITTED_SEAL_COMPLETES")
	}
}

func TestEventAndObjectReservationsCoexistAndSealWaitsForBoth(t *testing.T) {
	account := adr0011work.New("occurrence-a")
	eventEntered := make(chan struct{})
	eventRelease := make(chan struct{})
	eventDone := make(chan error, 1)
	go func() {
		eventDone <- account.WithEventsEmitted("occurrence-a", 3, func() error {
			close(eventEntered)
			<-eventRelease
			return nil
		})
	}()
	<-eventEntered

	objectEntered := make(chan struct{})
	objectRelease := make(chan struct{})
	objectDone := make(chan error, 1)
	go func() {
		objectDone <- account.WithObjectMaterialization("occurrence-a", func() error {
			close(objectEntered)
			<-objectRelease
			return nil
		})
	}()
	<-objectEntered

	sealed := make(chan adr0011work.Snapshot, 1)
	go func() { sealed <- account.Seal() }()
	waitForEventsAccountTerminal(t, account)

	close(eventRelease)
	if err := <-eventDone; err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-sealed:
		t.Fatalf("ASSERT_EVENTS_OBJECTS_SEAL_WAITS_BOTH after_event=%+v", got)
	default:
	}
	if got := account.Snapshot(); got.EventsEmitted != 3 || got.ObjectsMaterialized != 0 {
		t.Fatalf("ASSERT_EVENTS_OBJECTS_COUNTERS_INDEPENDENT_AFTER_EVENT snapshot=%+v", got)
	}

	close(objectRelease)
	if err := <-objectDone; err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-sealed:
		if got.EventsEmitted != 3 || got.ObjectsMaterialized != 1 || !got.Terminal {
			t.Fatalf("ASSERT_EVENTS_OBJECTS_SEAL_AFTER_BOTH snapshot=%+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("ASSERT_EVENTS_OBJECTS_SEAL_COMPLETES")
	}
}

func TestEventsEmittedZeroAmountStillCallsCallbackAndChargesZero(t *testing.T) {
	account := adr0011work.New("occurrence-a")
	called := 0
	if err := account.WithEventsEmitted("occurrence-a", 0, func() error { called++; return nil }); err != nil {
		t.Fatalf("ASSERT_EVENTS_EMITTED_ZERO_SUCCESS got=%v", err)
	}
	if got := account.Snapshot(); called != 1 || got.EventsEmitted != 0 || got.Terminal {
		t.Fatalf("ASSERT_EVENTS_EMITTED_ZERO_CALLBACK_ZERO_CHARGE called=%d snapshot=%+v", called, got)
	}
}

func TestEventsEmittedZeroAmountSealWaitsForCallback(t *testing.T) {
	account := adr0011work.New("occurrence-a")
	entered := make(chan struct{})
	release := make(chan struct{})
	emitted := make(chan error, 1)
	go func() {
		emitted <- account.WithEventsEmitted("occurrence-a", 0, func() error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered

	sealed := make(chan adr0011work.Snapshot, 1)
	go func() { sealed <- account.Seal() }()
	waitForEventsAccountTerminal(t, account)
	select {
	case got := <-sealed:
		t.Fatalf("ASSERT_EVENTS_EMITTED_ZERO_SEAL_WAITS early=%+v", got)
	default:
	}

	close(release)
	if err := <-emitted; err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-sealed:
		if got.EventsEmitted != 0 || !got.Terminal || got.Failure != nil {
			t.Fatalf("ASSERT_EVENTS_EMITTED_ZERO_SEAL_COMPLETES snapshot=%+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("ASSERT_EVENTS_EMITTED_ZERO_SEAL_COMPLETES")
	}
}

func TestDirectEventsChargeOverflowIncludesOutstandingReservation(t *testing.T) {
	account := adr0011work.New("occurrence-a")
	entered := make(chan struct{})
	release := make(chan struct{})
	emitted := make(chan error, 1)
	go func() {
		emitted <- account.WithEventsEmitted("occurrence-a", math.MaxUint64, func() error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered

	if err := account.ChargeEventsEmitted("occurrence-a", 1); !errors.Is(err, adr0011work.ErrOverflow) {
		t.Fatalf("ASSERT_EVENTS_EMITTED_DIRECT_CHARGE_RESERVED_OVERFLOW_ERROR got=%v", err)
	}
	if got := account.Snapshot(); got.EventsEmitted != 0 || !got.Terminal || !errors.Is(got.Failure, adr0011work.ErrOverflow) {
		t.Fatalf("ASSERT_EVENTS_EMITTED_DIRECT_CHARGE_RESERVED_OVERFLOW_PRE_EFFECT snapshot=%+v", got)
	}

	sealed := make(chan adr0011work.Snapshot, 1)
	go func() { sealed <- account.Seal() }()
	select {
	case got := <-sealed:
		t.Fatalf("ASSERT_EVENTS_EMITTED_TERMINAL_SETTLEMENT_SEAL_WAITS early=%+v", got)
	default:
	}

	close(release)
	if err := <-emitted; err != nil {
		t.Fatalf("ASSERT_EVENTS_EMITTED_TERMINAL_SETTLEMENT_CALLBACK_RESULT got=%v", err)
	}
	select {
	case got := <-sealed:
		if got.EventsEmitted != 0 || !got.Terminal || !errors.Is(got.Failure, adr0011work.ErrOverflow) {
			t.Fatalf("ASSERT_EVENTS_EMITTED_TERMINAL_SETTLEMENT_NO_COMMIT snapshot=%+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("ASSERT_EVENTS_EMITTED_TERMINAL_SETTLEMENT_SEAL_COMPLETES")
	}
}

func TestEventsEmittedNilCallbackReturnsTypedError(t *testing.T) {
	account := adr0011work.New("occurrence-a")
	if err := account.WithEventsEmitted("occurrence-a", 1, nil); !errors.Is(err, adr0011work.ErrCallbackRequired) {
		t.Fatalf("ASSERT_EVENTS_EMITTED_NIL_CALLBACK_TYPED_ERROR got=%v", err)
	}
	if got := account.Snapshot(); got.EventsEmitted != 0 || got.Terminal {
		t.Fatalf("ASSERT_EVENTS_EMITTED_NIL_CALLBACK_ZERO_EFFECT snapshot=%+v", got)
	}
}
