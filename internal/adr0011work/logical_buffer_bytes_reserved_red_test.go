package adr0011work_test

import (
	"errors"
	"math"
	"reflect"
	"runtime"
	"testing"
	"time"

	"lsp-trace/internal/adr0011work"
)

func TestLogicalBufferReservationStartsZeroAndCommitsExactAmountAfterCallback(t *testing.T) {
	account := adr0011work.New("occurrence-a")
	if got := account.Snapshot().LogicalBufferBytesReserved; got != 0 {
		t.Fatalf("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_NEW_ZERO got=%d", got)
	}
	inside := uint64(99)
	if err := account.WithLogicalBufferReservation("occurrence-a", 7, func() error {
		inside = account.Snapshot().LogicalBufferBytesReserved
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got := account.Snapshot(); inside != 0 || got.LogicalBufferBytesReserved != 7 {
		t.Fatalf("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_EXACT_POST_CALLBACK inside=%d snapshot=%+v", inside, got)
	}
}

func TestLogicalBufferReservationErrorRollsBackAndAllowsRetry(t *testing.T) {
	account := adr0011work.New("occurrence-a")
	callbackErr := errors.New("buffer allocation failed")
	if err := account.WithLogicalBufferReservation("occurrence-a", 4, func() error { return callbackErr }); !errors.Is(err, callbackErr) {
		t.Fatalf("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_CALLBACK_ERROR got=%v", err)
	}
	if got := account.Snapshot(); got.LogicalBufferBytesReserved != 0 || got.Terminal {
		t.Fatalf("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_ERROR_ROLLBACK snapshot=%+v", got)
	}
	if err := account.WithLogicalBufferReservation("occurrence-a", 2, func() error { return nil }); err != nil {
		t.Fatalf("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_ERROR_RETRY got=%v", err)
	}
	if got := account.Snapshot().LogicalBufferBytesReserved; got != 2 {
		t.Fatalf("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_ERROR_RETRY_COMMIT got=%d", got)
	}
}

func TestLogicalBufferReservationPanicRollsBackRepanicsAndAllowsRetry(t *testing.T) {
	account := adr0011work.New("occurrence-a")
	panicValue := &struct{ label string }{"buffer panic"}
	func() {
		defer func() {
			if got := recover(); got != panicValue {
				t.Fatalf("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_REPANIC got=%#v", got)
			}
		}()
		_ = account.WithLogicalBufferReservation("occurrence-a", 3, func() error { panic(panicValue) })
	}()
	if got := account.Snapshot(); got.LogicalBufferBytesReserved != 0 || got.Terminal {
		t.Fatalf("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_PANIC_ROLLBACK snapshot=%+v", got)
	}
	if err := account.WithLogicalBufferReservation("occurrence-a", 2, func() error { return nil }); err != nil {
		t.Fatalf("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_PANIC_RETRY got=%v", err)
	}
	if got := account.Snapshot().LogicalBufferBytesReserved; got != 2 {
		t.Fatalf("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_PANIC_RETRY_COMMIT got=%d", got)
	}
}

func TestLogicalBufferReservationMaxUintEqualitySucceeds(t *testing.T) {
	account := adr0011work.New("occurrence-a")
	if err := account.WithLogicalBufferReservation("occurrence-a", math.MaxUint64, func() error { return nil }); err != nil {
		t.Fatalf("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_MAX_UINT got=%v", err)
	}
	if got := account.Snapshot(); got.LogicalBufferBytesReserved != math.MaxUint64 || got.Terminal {
		t.Fatalf("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_MAX_UINT_COMMITTED snapshot=%+v", got)
	}
}

func TestLogicalBufferReservationOverflowIncludesCommittedAndOutstanding(t *testing.T) {
	t.Run("committed-plus-one", func(t *testing.T) {
		account := adr0011work.New("occurrence-a")
		if err := account.ChargeLogicalBufferBytesReserved("occurrence-a", math.MaxUint64); err != nil {
			t.Fatal(err)
		}
		called := false
		if err := account.WithLogicalBufferReservation("occurrence-a", 1, func() error { called = true; return nil }); !errors.Is(err, adr0011work.ErrOverflow) {
			t.Fatalf("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_COMMITTED_OVERFLOW got=%v", err)
		}
		if got := account.Snapshot(); called || got.LogicalBufferBytesReserved != math.MaxUint64 || !got.Terminal || !errors.Is(got.Failure, adr0011work.ErrOverflow) {
			t.Fatalf("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_COMMITTED_OVERFLOW_PRE_CALLBACK called=%t snapshot=%+v", called, got)
		}
	})

	t.Run("outstanding-plus-one", func(t *testing.T) {
		account := adr0011work.New("occurrence-a")
		entered := make(chan struct{})
		release := make(chan struct{})
		firstDone := make(chan error, 1)
		go func() {
			firstDone <- account.WithLogicalBufferReservation("occurrence-a", math.MaxUint64, func() error {
				close(entered)
				<-release
				return errors.New("release without commit")
			})
		}()
		awaitSignal(t, entered, "ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_OUTSTANDING_ENTER")

		called := false
		if err := account.WithLogicalBufferReservation("occurrence-a", 1, func() error { called = true; return nil }); !errors.Is(err, adr0011work.ErrOverflow) {
			t.Fatalf("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_OUTSTANDING_OVERFLOW got=%v", err)
		}
		if got := account.Snapshot(); called || got.LogicalBufferBytesReserved != 0 || !got.Terminal || !errors.Is(got.Failure, adr0011work.ErrOverflow) {
			t.Fatalf("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_OUTSTANDING_OVERFLOW_PRE_CALLBACK called=%t snapshot=%+v", called, got)
		}
		close(release)
		if err := awaitError(t, firstDone, "ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_OUTSTANDING_SETTLES"); err == nil {
			t.Fatal("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_OUTSTANDING_CALLBACK_ERROR")
		}
	})
}

func TestLogicalBufferReservationIdentityAndTerminalRefuseBeforeCallback(t *testing.T) {
	account := adr0011work.New("occurrence-a")
	called := false
	if err := account.WithLogicalBufferReservation("occurrence-b", 1, func() error { called = true; return nil }); !errors.Is(err, adr0011work.ErrIdentityMismatch) {
		t.Fatalf("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_IDENTITY got=%v", err)
	}
	if got := account.Snapshot(); called || got.Terminal || got.LogicalBufferBytesReserved != 0 {
		t.Fatalf("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_IDENTITY_ZERO_EFFECT called=%t snapshot=%+v", called, got)
	}
	account.Seal()
	if err := account.WithLogicalBufferReservation("occurrence-a", 1, func() error { called = true; return nil }); !errors.Is(err, adr0011work.ErrTerminal) {
		t.Fatalf("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_TERMINAL got=%v", err)
	}
	if called {
		t.Fatal("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_REFUSAL_PRE_CALLBACK")
	}
}

func TestLogicalBufferReservationZeroAmountCallbackMakesSealWait(t *testing.T) {
	account := adr0011work.New("occurrence-a")
	entered := make(chan struct{})
	release := make(chan struct{})
	reserved := make(chan error, 1)
	go func() {
		reserved <- account.WithLogicalBufferReservation("occurrence-a", 0, func() error {
			close(entered)
			<-release
			return nil
		})
	}()
	awaitSignal(t, entered, "ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_ZERO_ENTER")
	sealed := make(chan adr0011work.Snapshot, 1)
	go func() { sealed <- account.Seal() }()
	awaitLogicalBufferTerminal(t, account)
	assertNoSnapshot(t, sealed, "ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_ZERO_SEAL_WAITS")
	close(release)
	if err := awaitError(t, reserved, "ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_ZERO_SETTLES"); err != nil {
		t.Fatal(err)
	}
	got := awaitSnapshot(t, sealed, "ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_ZERO_SEAL_COMPLETES")
	if got.LogicalBufferBytesReserved != 0 || !got.Terminal || got.Failure != nil {
		t.Fatalf("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_ZERO_SEALED snapshot=%+v", got)
	}
}

func TestLogicalBufferReservationSealWaitsForAllSettlements(t *testing.T) {
	account := adr0011work.New("occurrence-a")
	entered := make(chan struct{}, 2)
	releaseOne := make(chan struct{})
	releaseTwo := make(chan struct{})
	results := make(chan error, 2)
	reservations := []struct {
		amount  uint64
		release <-chan struct{}
	}{{2, releaseOne}, {3, releaseTwo}}
	for _, reservation := range reservations {
		reservation := reservation
		go func() {
			results <- account.WithLogicalBufferReservation("occurrence-a", reservation.amount, func() error {
				entered <- struct{}{}
				<-reservation.release
				return nil
			})
		}()
	}
	awaitSignal(t, entered, "ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_SETTLEMENT_ONE_ENTER")
	awaitSignal(t, entered, "ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_SETTLEMENT_TWO_ENTER")
	sealed := make(chan adr0011work.Snapshot, 1)
	go func() { sealed <- account.Seal() }()
	awaitLogicalBufferTerminal(t, account)
	close(releaseOne)
	if err := awaitError(t, results, "ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_FIRST_SETTLES"); err != nil {
		t.Fatal(err)
	}
	assertNoSnapshot(t, sealed, "ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_SEAL_WAITS_ALL")
	close(releaseTwo)
	if err := awaitError(t, results, "ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_SECOND_SETTLES"); err != nil {
		t.Fatal(err)
	}
	got := awaitSnapshot(t, sealed, "ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_SEAL_AFTER_ALL")
	if got.LogicalBufferBytesReserved != 5 || !got.Terminal {
		t.Fatalf("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_ALL_COMMITTED snapshot=%+v", got)
	}
}

func TestDirectLogicalBufferChargeIncludesOutstandingReservation(t *testing.T) {
	account := adr0011work.New("occurrence-a")
	entered := make(chan struct{})
	release := make(chan struct{})
	reserved := make(chan error, 1)
	go func() {
		reserved <- account.WithLogicalBufferReservation("occurrence-a", math.MaxUint64, func() error {
			close(entered)
			<-release
			return nil
		})
	}()
	awaitSignal(t, entered, "ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_DIRECT_ENTER")
	if err := account.ChargeLogicalBufferBytesReserved("occurrence-a", 1); !errors.Is(err, adr0011work.ErrOverflow) {
		t.Fatalf("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_DIRECT_OVERFLOW got=%v", err)
	}
	if got := account.Snapshot(); got.LogicalBufferBytesReserved != 0 || !got.Terminal || !errors.Is(got.Failure, adr0011work.ErrOverflow) {
		t.Fatalf("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_DIRECT_ZERO_EFFECT snapshot=%+v", got)
	}
	sealed := make(chan adr0011work.Snapshot, 1)
	go func() { sealed <- account.Seal() }()
	assertNoSnapshot(t, sealed, "ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_DIRECT_SEAL_WAITS")
	close(release)
	if err := awaitError(t, reserved, "ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_DIRECT_SETTLES"); err != nil {
		t.Fatal(err)
	}
	got := awaitSnapshot(t, sealed, "ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_DIRECT_SEAL_COMPLETES")
	if got.LogicalBufferBytesReserved != 0 || !errors.Is(got.Failure, adr0011work.ErrOverflow) {
		t.Fatalf("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_DIRECT_NO_COMMIT snapshot=%+v", got)
	}
}

func TestLogicalBufferReservationNilCallbackReturnsTypedError(t *testing.T) {
	account := adr0011work.New("occurrence-a")
	if err := account.WithLogicalBufferReservation("occurrence-a", 1, nil); !errors.Is(err, adr0011work.ErrCallbackRequired) {
		t.Fatalf("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_NIL_CALLBACK got=%v", err)
	}
	if got := account.Snapshot(); got.LogicalBufferBytesReserved != 0 || got.Terminal {
		t.Fatalf("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_NIL_ZERO_EFFECT snapshot=%+v", got)
	}
}

func TestLogicalBufferReservationCoexistsWithObjectEventAndDocumentReservations(t *testing.T) {
	account := adr0011work.New("occurrence-a")
	entered := make(chan string, 4)
	release := make(chan struct{})
	results := make(chan error, 4)
	go func() {
		results <- account.WithObjectMaterialization("occurrence-a", func() error { entered <- "object"; <-release; return nil })
	}()
	go func() {
		results <- account.WithEventsEmitted("occurrence-a", 3, func() error { entered <- "event"; <-release; return nil })
	}()
	go func() {
		results <- account.WithDocumentAcquisition("occurrence-a", "file:///coexist", func() (bool, error) { entered <- "document"; <-release; return true, nil })
	}()
	go func() {
		results <- account.WithLogicalBufferReservation("occurrence-a", 5, func() error { entered <- "buffer"; <-release; return nil })
	}()
	for range 4 {
		awaitSignal(t, entered, "ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_COEXIST_ENTER")
	}
	sealed := make(chan adr0011work.Snapshot, 1)
	go func() { sealed <- account.Seal() }()
	awaitLogicalBufferTerminal(t, account)
	assertNoSnapshot(t, sealed, "ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_COEXIST_SEAL_WAITS")
	close(release)
	for range 4 {
		if err := awaitError(t, results, "ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_COEXIST_SETTLES"); err != nil {
			t.Fatal(err)
		}
	}
	got := awaitSnapshot(t, sealed, "ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_COEXIST_SEAL_COMPLETES")
	if got.ObjectsMaterialized != 1 || got.EventsEmitted != 3 || got.DocumentsAcquired != 1 || got.LogicalBufferBytesReserved != 5 || !got.Terminal {
		t.Fatalf("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_COEXIST_EXACT snapshot=%+v", got)
	}
}

func TestLogicalBufferReservationSnapshotExactExportedSurface(t *testing.T) {
	typ := reflect.TypeOf(adr0011work.New("occurrence-a").Snapshot())
	var exported []string
	for i := 0; i < typ.NumField(); i++ {
		if field := typ.Field(i); field.IsExported() {
			exported = append(exported, field.Name)
		}
	}
	want := []string{"Identity", "BytesScanned", "MessagesProcessed", "ObjectsMaterialized", "EventsEmitted", "DocumentsAcquired", "LogicalBufferBytesReserved", "Terminal", "Failure"}
	if !reflect.DeepEqual(exported, want) {
		t.Fatalf("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_SNAPSHOT_EXACT_SURFACE got=%v want=%v", exported, want)
	}
}

func awaitLogicalBufferTerminal(t *testing.T, account *adr0011work.Account) {
	t.Helper()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	for !account.Snapshot().Terminal {
		select {
		case <-deadline.C:
			t.Fatal("ASSERT_LOGICAL_BUFFER_BYTES_RESERVED_TERMINAL_OBSERVED")
		default:
			runtime.Gosched()
		}
	}
}

func awaitSignal[T any](t *testing.T, ch <-chan T, assertion string) T {
	t.Helper()
	select {
	case got := <-ch:
		return got
	case <-time.After(time.Second):
		t.Fatal(assertion)
		var zero T
		return zero
	}
}

func awaitError(t *testing.T, ch <-chan error, assertion string) error {
	t.Helper()
	return awaitSignal(t, ch, assertion)
}

func awaitSnapshot(t *testing.T, ch <-chan adr0011work.Snapshot, assertion string) adr0011work.Snapshot {
	t.Helper()
	return awaitSignal(t, ch, assertion)
}

func assertNoSnapshot(t *testing.T, ch <-chan adr0011work.Snapshot, assertion string) {
	t.Helper()
	select {
	case got := <-ch:
		t.Fatalf("%s early=%+v", assertion, got)
	case <-time.After(100 * time.Millisecond):
	}
}
