package adr0011work

import (
	"errors"
	"math"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDocumentsAcquiredStartsZeroAndCommitsFirstSuccessfulIdentity(t *testing.T) {
	account := New("transaction-a")
	if got := account.Snapshot().DocumentsAcquired; got != 0 {
		t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_NEW_ZERO got=%d", got)
	}

	inside := uint64(99)
	if err := account.WithDocumentAcquisition("transaction-a", "file:///document-a", func() (bool, error) {
		inside = account.Snapshot().DocumentsAcquired
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
	if got := account.Snapshot().DocumentsAcquired; inside != 0 || got != 1 {
		t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_COMMIT_AFTER_EFFECT inside=%d after=%d", inside, got)
	}
}

func TestDocumentsAcquiredSequentialDuplicateRunsEveryCallbackAndCountsOnce(t *testing.T) {
	account := New("transaction-a")
	var callbacks int
	for _, path := range []string{"fresh-success", "custody-valid-cache"} {
		if err := account.WithDocumentAcquisition("transaction-a", "file:///canonical-document", func() (bool, error) {
			callbacks++
			return true, nil
		}); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	if got := account.Snapshot().DocumentsAcquired; callbacks != 2 || got != 1 {
		t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_DUPLICATE_CALLBACKS_NOT_SUPPRESSED callbacks=%d count=%d", callbacks, got)
	}
}

func TestDocumentsAcquiredConcurrentDuplicateRunsBothCallbacksAndCountsOnce(t *testing.T) {
	account := New("transaction-a")
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	results := make(chan error, 2)
	var callbacks atomic.Int32

	for range 2 {
		go func() {
			results <- account.WithDocumentAcquisition("transaction-a", "file:///same-document", func() (bool, error) {
				callbacks.Add(1)
				entered <- struct{}{}
				<-release
				return true, nil
			})
		}()
	}
	for range 2 {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("ASSERT_DOCUMENTS_ACQUIRED_CONCURRENT_DUPLICATES_BOTH_ENTER")
		}
	}
	close(release)
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if got := account.Snapshot().DocumentsAcquired; callbacks.Load() != 2 || got != 1 {
		t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_CONCURRENT_DUPLICATE_COUNT callbacks=%d count=%d", callbacks.Load(), got)
	}
}

func TestDocumentsAcquiredDistinctExactIdentitiesEachCount(t *testing.T) {
	account := New("transaction-a")
	for _, identity := range []string{"file:///document", "file:///document#fragment", "FILE:///document", " file:///document "} {
		if err := account.WithDocumentAcquisition("transaction-a", identity, func() (bool, error) { return true, nil }); err != nil {
			t.Fatalf("identity %q: %v", identity, err)
		}
	}
	if got := account.Snapshot().DocumentsAcquired; got != 4 {
		t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_DISTINCT_EXACT_IDENTITIES got=%d", got)
	}
}

func TestDocumentsAcquiredFalseAndErrorRollBackThenRetrySuccess(t *testing.T) {
	account := New("transaction-a")
	callbackErr := errors.New("document unavailable")
	if err := account.WithDocumentAcquisition("transaction-a", "file:///retry", func() (bool, error) { return false, nil }); err != nil {
		t.Fatalf("false result: %v", err)
	}
	if err := account.WithDocumentAcquisition("transaction-a", "file:///retry", func() (bool, error) { return true, callbackErr }); !errors.Is(err, callbackErr) {
		t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_CALLBACK_ERROR got=%v", err)
	}
	if got := account.Snapshot(); got.DocumentsAcquired != 0 || got.Terminal {
		t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_FALSE_ERROR_ROLLBACK snapshot=%+v", got)
	}
	if err := account.WithDocumentAcquisition("transaction-a", "file:///retry", func() (bool, error) { return true, nil }); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if got := account.Snapshot().DocumentsAcquired; got != 1 {
		t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_RETRY_COMMIT got=%d", got)
	}
}

func TestDocumentsAcquiredPanicCleansUpRepanicsAndAllowsReuse(t *testing.T) {
	account := New("transaction-a")
	panicValue := &struct{ label string }{"acquire panic"}
	func() {
		defer func() {
			if got := recover(); got != panicValue {
				t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_REPANIC got=%#v", got)
			}
		}()
		_ = account.WithDocumentAcquisition("transaction-a", "file:///panic", func() (bool, error) { panic(panicValue) })
	}()
	if got := account.Snapshot(); got.DocumentsAcquired != 0 || got.Terminal {
		t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_PANIC_ROLLBACK snapshot=%+v", got)
	}
	if err := account.WithDocumentAcquisition("transaction-a", "file:///panic", func() (bool, error) { return true, nil }); err != nil {
		t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_AFTER_PANIC_REUSABLE got=%v", err)
	}
	if got := account.Snapshot().DocumentsAcquired; got != 1 {
		t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_AFTER_PANIC_COMMIT got=%d", got)
	}
}

func TestDocumentsAcquiredTransactionIdentityRefusalPrecedesEffect(t *testing.T) {
	account := New("transaction-a")
	called := false
	if err := account.WithDocumentAcquisition("transaction-b", "file:///document", func() (bool, error) {
		called = true
		return true, nil
	}); !errors.Is(err, ErrIdentityMismatch) {
		t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_TRANSACTION_IDENTITY_ERROR got=%v", err)
	}
	if got := account.Snapshot(); called || got.DocumentsAcquired != 0 || got.Terminal {
		t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_TRANSACTION_IDENTITY_PRE_EFFECT called=%t snapshot=%+v", called, got)
	}
}

func TestDocumentsAcquiredOverflowRefusesBeforeEffectUsingPrivatePreload(t *testing.T) {
	account := New("transaction-a")
	account.private.mu.Lock()
	account.private.work.documentsAcquired = math.MaxUint64
	account.private.mu.Unlock()

	called := false
	if err := account.WithDocumentAcquisition("transaction-a", "file:///overflow", func() (bool, error) {
		called = true
		return true, nil
	}); !errors.Is(err, ErrOverflow) {
		t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_OVERFLOW_ERROR got=%v", err)
	}
	if got := account.Snapshot(); called || got.DocumentsAcquired != math.MaxUint64 || !got.Terminal || !errors.Is(got.Failure, ErrOverflow) {
		t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_OVERFLOW_PRE_EFFECT called=%t snapshot=%+v", called, got)
	}
}

func TestDocumentsAcquiredSealWaitsForAllDuplicateCallbacksAndBlocksNew(t *testing.T) {
	account := New("transaction-a")
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() {
			results <- account.WithDocumentAcquisition("transaction-a", "file:///same-document", func() (bool, error) {
				entered <- struct{}{}
				<-release
				return true, nil
			})
		}()
	}
	for range 2 {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("ASSERT_DOCUMENTS_ACQUIRED_SEAL_DUPLICATES_ENTER")
		}
	}

	sealed := make(chan Snapshot, 1)
	go func() { sealed <- account.Seal() }()
	waitForDocumentsAccountTerminal(t, account)
	if err := account.WithDocumentAcquisition("transaction-a", "file:///after-seal", func() (bool, error) {
		t.Fatal("ASSERT_DOCUMENTS_ACQUIRED_POST_SEAL_CALLBACK")
		return true, nil
	}); !errors.Is(err, ErrTerminal) {
		t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_POST_SEAL_REJECTED got=%v", err)
	}
	select {
	case got := <-sealed:
		t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_SEAL_WAITS_DUPLICATES early=%+v", got)
	default:
	}
	close(release)
	for range 2 {
		select {
		case err := <-results:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("ASSERT_DOCUMENTS_ACQUIRED_SEAL_DUPLICATES_COMPLETE")
		}
	}
	select {
	case got := <-sealed:
		if got.DocumentsAcquired != 1 || !got.Terminal {
			t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_SEAL_AFTER_DUPLICATES snapshot=%+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("ASSERT_DOCUMENTS_ACQUIRED_SEAL_COMPLETES")
	}
}

func TestDocumentsAcquiredEmptyIdentityIsExactAndNotNormalized(t *testing.T) {
	account := New("transaction-a")
	for range 2 {
		if err := account.WithDocumentAcquisition("transaction-a", "", func() (bool, error) { return true, nil }); err != nil {
			t.Fatal(err)
		}
	}
	if err := account.WithDocumentAcquisition("transaction-a", " ", func() (bool, error) { return true, nil }); err != nil {
		t.Fatal(err)
	}
	if got := account.Snapshot().DocumentsAcquired; got != 2 {
		t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_EMPTY_EXACT count=%d", got)
	}
}

func TestDocumentsAcquiredCoexistsWithObjectAndEventReservations(t *testing.T) {
	account := New("transaction-a")
	objectEntered := make(chan struct{})
	eventEntered := make(chan struct{})
	release := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := account.WithObjectMaterialization("transaction-a", func() error { close(objectEntered); <-release; return nil }); err != nil {
			t.Errorf("object: %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		if err := account.WithEventsEmitted("transaction-a", 3, func() error { close(eventEntered); <-release; return nil }); err != nil {
			t.Errorf("events: %v", err)
		}
	}()
	<-objectEntered
	<-eventEntered
	if err := account.WithDocumentAcquisition("transaction-a", "file:///coexist", func() (bool, error) { return true, nil }); err != nil {
		t.Fatal(err)
	}
	close(release)
	wg.Wait()
	if got := account.Snapshot(); got.DocumentsAcquired != 1 || got.ObjectsMaterialized != 1 || got.EventsEmitted != 3 {
		t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_RESERVATIONS_COEXIST snapshot=%+v", got)
	}
}

func TestDocumentsAcquiredCommittedIdentitiesAreNotExposedBySnapshot(t *testing.T) {
	account := New("transaction-a")
	if err := account.WithDocumentAcquisition("transaction-a", "file:///private", func() (bool, error) { return true, nil }); err != nil {
		t.Fatal(err)
	}
	typ := reflect.TypeOf(account.Snapshot())
	var exported []string
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.IsExported() {
			exported = append(exported, field.Name)
		}
	}
	want := []string{
		"Identity",
		"BytesScanned",
		"MessagesProcessed",
		"ObjectsMaterialized",
		"EventsEmitted",
		"DocumentsAcquired",
		"LogicalBufferBytesReserved",
		"Terminal",
		"Failure",
	}
	if !reflect.DeepEqual(exported, want) {
		t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_SNAPSHOT_EXPORTED_SURFACE got=%v want=%v", exported, want)
	}
}

func waitForDocumentsAccountTerminal(t *testing.T, account *Account) {
	t.Helper()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	for !account.Snapshot().Terminal {
		select {
		case <-deadline.C:
			t.Fatal("ASSERT_DOCUMENTS_ACQUIRED_TERMINAL_OBSERVED")
		default:
			time.Sleep(time.Millisecond)
		}
	}
}
