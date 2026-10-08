package sessionruntime

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"lsp-trace/internal/session"
)

// privateC15AtomicBatchObserverHost is the proposed migration-compatible C15
// batch seam. Its distinct name avoids overloading the legacy single-event
// installer, while the function signature remains production-definable.
type privateC15AtomicBatchObserverHost interface {
	installPrivateB4C15BatchObserverChecked(privateB4C15BatchObserver) session.Failure
}

type privateC15AtomicBatchMode uint8

const (
	privateC15BatchAccept privateC15AtomicBatchMode = iota
	privateC15BatchRejectFirst
	privateC15BatchRejectSecond
	privateC15BatchError
	privateC15BatchPanic
)

type privateC15AtomicBatchObserver struct {
	mu        sync.Mutex
	mode      privateC15AtomicBatchMode
	attempted [][]privateB4C15Event
	committed []privateB4C15Event
}

func (o *privateC15AtomicBatchObserver) setMode(mode privateC15AtomicBatchMode) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.mode = mode
}

func (o *privateC15AtomicBatchObserver) snapshot() []privateB4C15Event {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]privateB4C15Event(nil), o.committed...)
}

func (o *privateC15AtomicBatchObserver) attemptedSnapshot() [][]privateB4C15Event {
	o.mu.Lock()
	defer o.mu.Unlock()
	attempted := make([][]privateB4C15Event, len(o.attempted))
	for i := range o.attempted {
		attempted[i] = append([]privateB4C15Event(nil), o.attempted[i]...)
	}
	return attempted
}

func (o *privateC15AtomicBatchObserver) callCount() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.attempted)
}

// ObserveBatch validates the complete immutable value view before mutating the
// observer snapshot. Rejection, error, and panic therefore append no prefix.
func (o *privateC15AtomicBatchObserver) ObserveBatch(events []privateB4C15Event) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	candidate := append([]privateB4C15Event(nil), events...)
	o.attempted = append(o.attempted, candidate)
	if len(candidate) == 0 {
		return errors.New("empty C15 observer batch")
	}
	for i, event := range candidate {
		if event.TransactionID == "" || event.ObjectID == "" || event.SiteID == "" || event.Kind == "" || event.Sequence == 0 {
			return fmt.Errorf("invalid event %d", i)
		}
		if i > 0 && event.Sequence != candidate[i-1].Sequence+1 {
			return fmt.Errorf("non-contiguous event %d", i)
		}
	}
	switch o.mode {
	case privateC15BatchRejectFirst:
		return errors.New("reject first event")
	case privateC15BatchRejectSecond:
		if len(candidate) < 2 {
			return errors.New("second event absent")
		}
		return errors.New("reject second event")
	case privateC15BatchError:
		return errors.New("observer failure")
	case privateC15BatchPanic:
		panic("observer panic")
	}
	o.committed = append(o.committed, candidate...)
	return nil
}

type privateC15AtomicContextSnapshot struct {
	account      privateB4ByteLedgerSnapshot
	sequence     uint64
	nextObjectID uint64
	objects      map[uint64]privateB4C15ObjectState
}

func privateC15AtomicSnapshot(t *testing.T, account *privateB4ByteAccountV2) privateC15AtomicContextSnapshot {
	t.Helper()
	ledger := account.snapshot()
	account.mu.Lock()
	defer account.mu.Unlock()
	objects := make(map[uint64]privateB4C15ObjectState)
	var sequence, nextObjectID uint64
	if account.c15 != nil {
		sequence = account.c15.sequence
		nextObjectID = account.c15.nextObjectID
		for id, state := range account.c15.objects {
			objects[id] = state
		}
	}
	return privateC15AtomicContextSnapshot{
		account: ledger, sequence: sequence, nextObjectID: nextObjectID, objects: objects,
	}
}

func privateC15InstallAtomicBatchObserver(t *testing.T, account *privateB4ByteAccountV2, observer *privateC15AtomicBatchObserver) {
	t.Helper()
	host, ok := any(account).(privateC15AtomicBatchObserverHost)
	if !ok {
		t.Fatal("ASSERT_C15_ATOMIC_BATCH_TYPED_HOST_ABSENT")
	}
	if failure := host.installPrivateB4C15BatchObserverChecked(observer.ObserveBatch); failure != "" {
		t.Fatalf("ASSERT_C15_ATOMIC_BATCH_INSTALL failure=%q", failure)
	}
}

func privateC15RequireUnchanged(t *testing.T, label string, got, want privateC15AtomicContextSnapshot, observer *privateC15AtomicBatchObserver, observerWant []privateB4C15Event) {
	t.Helper()
	if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(observer.snapshot(), observerWant) {
		t.Fatalf("ASSERT_C15_ATOMIC_BATCH_UNCHANGED phase=%s\ncontext before=%+v\ncontext after=%+v\nobserver before=%+v\nobserver after=%+v", label, want, got, observerWant, observer.snapshot())
	}
}

func TestADR0011C17C15AtomicBatchObserverOwnedPairSameInstanceCausalRED(t *testing.T) {
	account := mustPrivateB4ByteAccountV2(t)
	observer := &privateC15AtomicBatchObserver{}
	privateC15InstallAtomicBatchObserver(t, account, observer)
	initial := privateC15AtomicSnapshot(t, account)
	observerInitial := observer.snapshot()

	observer.setMode(privateC15BatchRejectFirst)
	if owner, failure := newPrivateB4OwnedMethodPairOwner(account, OwnedMethodPair{Params: []byte("params"), Result: []byte("result")}); failure != session.ResourceExhausted || owner != nil {
		t.Fatalf("ASSERT_C15_ATOMIC_BATCH_REJECT_FIRST failure=%q owner=%v", failure, owner)
	}
	privateC15RequireUnchanged(t, "reject-first", privateC15AtomicSnapshot(t, account), initial, observer, observerInitial)
	attempts := observer.attemptedSnapshot()
	if len(attempts) != 1 || len(attempts[0]) != 2 {
		t.Fatalf("ASSERT_C15_ATOMIC_BATCH_REJECT_FIRST_ATTEMPT attempts=%+v", attempts)
	}
	rejectFirstAttempt := attempts[0]

	observer.setMode(privateC15BatchRejectSecond)
	if owner, failure := newPrivateB4OwnedMethodPairOwner(account, OwnedMethodPair{Params: []byte("params"), Result: []byte("result")}); failure != session.ResourceExhausted || owner != nil {
		t.Fatalf("ASSERT_C15_ATOMIC_BATCH_REJECT_SECOND failure=%q owner=%v", failure, owner)
	}
	privateC15RequireUnchanged(t, "reject-second-no-prefix", privateC15AtomicSnapshot(t, account), initial, observer, observerInitial)
	attempts = observer.attemptedSnapshot()
	if len(attempts) != 2 || !reflect.DeepEqual(attempts[1], rejectFirstAttempt) {
		t.Fatalf("ASSERT_C15_ATOMIC_BATCH_REJECT_SECOND_TENTATIVE_IDS first=%+v second=%+v", rejectFirstAttempt, attempts)
	}

	observer.setMode(privateC15BatchAccept)
	owner, failure := newPrivateB4OwnedMethodPairOwner(account, OwnedMethodPair{Params: []byte("params"), Result: []byte("result")})
	if failure != "" || owner == nil {
		t.Fatalf("ASSERT_C15_ATOMIC_BATCH_RETRY_SUCCESS failure=%q owner=%v", failure, owner)
	}
	lease := OwnedMethodPairLease{owner: owner}
	if err := lease.WithPair(func(view OwnedMethodPairView) error {
		if string(view.Params) != "params" || string(view.Result) != "result" {
			return errors.New("semantic output changed")
		}
		return nil
	}); err != nil {
		t.Fatalf("ASSERT_C15_ATOMIC_BATCH_SEMANTICS err=%v", err)
	}

	attempts = observer.attemptedSnapshot()
	if len(attempts) != 3 || !reflect.DeepEqual(attempts[2], rejectFirstAttempt) {
		t.Fatalf("ASSERT_C15_ATOMIC_BATCH_RETRY_SAME_TENTATIVE_IDS first=%+v attempts=%+v", rejectFirstAttempt, attempts)
	}
	committed := observer.snapshot()
	if len(committed) != 2 || !reflect.DeepEqual(committed, attempts[2]) {
		t.Fatalf("ASSERT_C15_ATOMIC_BATCH_COMMITTED_ONCE committed=%+v accepted=%+v", committed, attempts[2])
	}
	if committed[0].SiteID != privateC15SiteC15Copy34OwnedPairParams || committed[1].SiteID != privateC15SiteC15Copy35OwnedPairResult || committed[0].Kind != "copy" || committed[1].Kind != "copy" {
		t.Fatalf("ASSERT_C15_ATOMIC_BATCH_PARAMS_RESULT_ORDER events=%+v", committed)
	}
	if committed[1].Sequence != committed[0].Sequence+1 || committed[0].ObjectID == committed[1].ObjectID {
		t.Fatalf("ASSERT_C15_ATOMIC_BATCH_CONTIGUOUS_DISTINCT events=%+v", committed)
	}
	ids := map[string]struct{}{}
	for _, event := range committed {
		if _, duplicate := ids[event.ObjectID]; duplicate {
			t.Fatalf("ASSERT_C15_ATOMIC_BATCH_DUPLICATE_COMMITTED_ID id=%s events=%+v", event.ObjectID, committed)
		}
		ids[event.ObjectID] = struct{}{}
	}
	if observer.callCount() != 3 {
		t.Fatalf("ASSERT_C15_ATOMIC_BATCH_ONE_CALLBACK_PER_OPERATION calls=%d", observer.callCount())
	}
	if !lease.Release() || lease.Release() {
		t.Fatal("ASSERT_C15_ATOMIC_BATCH_OWNER_RELEASE_EXACT_ONCE")
	}
	finalEvents := observer.snapshot()
	terminal := map[string]int{}
	for _, event := range finalEvents {
		if event.Terminal {
			terminal[event.ObjectID]++
		}
	}
	for id := range ids {
		if terminal[id] != 1 {
			t.Fatalf("ASSERT_C15_ATOMIC_BATCH_TERMINAL_EXACT_ONCE id=%s terminal=%v events=%+v", id, terminal, finalEvents)
		}
	}
	if got := account.snapshot(); got.Live != initial.account.Live {
		t.Fatalf("ASSERT_C15_ATOMIC_BATCH_FINAL_ACCOUNTING initial=%+v final=%+v", initial.account, got)
	}
}

func TestADR0011C17C15ManagerInstalledOwnedPairBatchAtomicity(t *testing.T) {
	const (
		sessionID     = "manager-batch-session"
		generation    = uint64(7)
		transactionID = "manager-owned-pair"
	)
	observer := &privateC15AtomicBatchObserver{mode: privateC15BatchRejectSecond}
	manager := &Manager{sessions: map[string]*runtimeSession{
		sessionID: {record: Record{SessionID: sessionID, Generation: generation}},
	}}
	if failure := manager.installPrivateB4C15BatchObserverChecked(sessionID, generation, observer.ObserveBatch); failure != "" {
		t.Fatalf("ASSERT_C15_MANAGER_BATCH_INSTALL failure=%q", failure)
	}
	context, failure := manager.privateB4C15ContextFor(sessionID, generation, transactionID)
	if failure != "" || context == nil {
		t.Fatalf("ASSERT_C15_MANAGER_BATCH_LOOKUP failure=%q context=%v", failure, context)
	}
	account := mustPrivateB4ByteAccountV2(t)
	account.mu.Lock()
	account.c15 = context
	account.c15Leases = make(map[privateB4ByteLease]privateB4C15LeaseState)
	account.mu.Unlock()
	initial := privateC15AtomicSnapshot(t, account)

	if owner, gotFailure := newPrivateB4OwnedMethodPairOwner(account, OwnedMethodPair{Params: []byte("params"), Result: []byte("result")}); gotFailure != session.ResourceExhausted || owner != nil {
		t.Fatalf("ASSERT_C15_MANAGER_BATCH_REJECT_SECOND failure=%q owner=%v", gotFailure, owner)
	}
	privateC15RequireUnchanged(t, "manager-reject-second", privateC15AtomicSnapshot(t, account), initial, observer, nil)
	attempts := observer.attemptedSnapshot()
	if len(attempts) != 1 || len(attempts[0]) != 2 || observer.callCount() != 1 {
		t.Fatalf("ASSERT_C15_MANAGER_BATCH_REJECT_ONE_CALLBACK attempts=%+v calls=%d", attempts, observer.callCount())
	}
	rejected := attempts[0]

	observer.setMode(privateC15BatchAccept)
	owner, gotFailure := newPrivateB4OwnedMethodPairOwner(account, OwnedMethodPair{Params: []byte("params"), Result: []byte("result")})
	if gotFailure != "" || owner == nil {
		t.Fatalf("ASSERT_C15_MANAGER_BATCH_RETRY failure=%q owner=%v", gotFailure, owner)
	}
	attempts = observer.attemptedSnapshot()
	if len(attempts) != 2 || observer.callCount() != 2 || !reflect.DeepEqual(attempts[1], rejected) {
		t.Fatalf("ASSERT_C15_MANAGER_BATCH_RETRY_SAME_IDS attempts=%+v calls=%d", attempts, observer.callCount())
	}
	if committed := observer.snapshot(); len(committed) != 2 || !reflect.DeepEqual(committed, attempts[1]) {
		t.Fatalf("ASSERT_C15_MANAGER_BATCH_COMMIT_ONCE committed=%+v attempts=%+v", committed, attempts)
	}
	lease := OwnedMethodPairLease{owner: owner}
	if !lease.Release() || lease.Release() {
		t.Fatal("ASSERT_C15_MANAGER_BATCH_OWNER_RELEASE_EXACT_ONCE")
	}
	allBatches := observer.attemptedSnapshot()
	if len(allBatches) != 4 || len(allBatches[2]) != 1 || len(allBatches[3]) != 1 || observer.callCount() != 4 {
		t.Fatalf("ASSERT_C15_MANAGER_BATCH_ONE_CALLBACK_PER_OPERATION batches=%+v calls=%d", allBatches, observer.callCount())
	}
	terminal := map[string]int{}
	for _, event := range observer.snapshot() {
		if event.Terminal {
			terminal[event.ObjectID]++
		}
	}
	for _, event := range rejected {
		if terminal[event.ObjectID] != 1 {
			t.Fatalf("ASSERT_C15_MANAGER_BATCH_TERMINAL_EXACT_ONCE id=%s terminal=%v", event.ObjectID, terminal)
		}
	}
}

func TestADR0011C17C15ProductionHasNoSequentialObserverFallback(t *testing.T) {
	matches, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range matches {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		if strings.Contains(text, "privateB4C15Observer") {
			t.Fatalf("ASSERT_C15_PRODUCTION_SINGLE_OBSERVER_TYPE path=%s", path)
		}
		if strings.Contains(text, "for _, event := range events") {
			t.Fatalf("ASSERT_C15_PRODUCTION_SEQUENTIAL_EVENT_LOOP path=%s", path)
		}
	}
}

func TestADR0011C17C15AtomicBatchObserverFailurePreEffectRED(t *testing.T) {
	for _, mode := range []privateC15AtomicBatchMode{privateC15BatchError, privateC15BatchPanic} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			account := mustPrivateB4ByteAccountV2(t)
			observer := &privateC15AtomicBatchObserver{mode: mode}
			privateC15InstallAtomicBatchObserver(t, account, observer)
			before := privateC15AtomicSnapshot(t, account)
			observerBefore := observer.snapshot()
			owner, failure := newPrivateB4OwnedMethodPairOwner(account, OwnedMethodPair{Params: []byte("params"), Result: []byte("result")})
			if failure != session.ResourceExhausted || owner != nil {
				t.Fatalf("ASSERT_C15_ATOMIC_BATCH_FAILURE_TYPED mode=%d failure=%q owner=%v", mode, failure, owner)
			}
			privateC15RequireUnchanged(t, "failure-pre-effect", privateC15AtomicSnapshot(t, account), before, observer, observerBefore)
		})
	}
}

func TestADR0011C17C15AtomicBatchObserverSingletonAndOwnershipOperationsRED(t *testing.T) {
	account := mustPrivateB4ByteAccountV2(t)
	observer := &privateC15AtomicBatchObserver{}
	privateC15InstallAtomicBatchObserver(t, account, observer)

	lease, failure := account.reserve(1)
	if failure != "" || !lease.lease.active {
		t.Fatalf("ASSERT_C15_ATOMIC_BATCH_SINGLETON failure=%q lease=%+v", failure, lease)
	}
	alias := lease.alias()
	moved := lease.transfer()
	if !alias.lease.active || !moved.lease.active {
		t.Fatalf("ASSERT_C15_ATOMIC_BATCH_ALIAS_TRANSFER alias=%+v moved=%+v", alias, moved)
	}
	_, copied, failure := copyPrivateB4Params(account, []byte("copy"))
	if failure != "" || !copied.lease.active {
		t.Fatalf("ASSERT_C15_ATOMIC_BATCH_COPY failure=%q lease=%+v", failure, copied)
	}
	batches := observer.attemptedSnapshot()
	if len(batches) != 4 {
		t.Fatalf("ASSERT_C15_ATOMIC_BATCH_EXACT_CALLBACK_COUNT got=%d batches=%+v", len(batches), batches)
	}
	for i, batch := range batches {
		if len(batch) != 1 {
			t.Fatalf("ASSERT_C15_ATOMIC_BATCH_SINGLETON_SIZE index=%d batch=%+v", i, batch)
		}
		if i > 0 && batch[0].Sequence != batches[i-1][0].Sequence+1 {
			t.Fatalf("ASSERT_C15_ATOMIC_BATCH_SINGLETON_CONTIGUOUS index=%d batches=%+v", i, batches)
		}
	}
	reserveEvent, aliasEvent, transferEvent, copyEvent := batches[0][0], batches[1][0], batches[2][0], batches[3][0]
	if reserveEvent.Kind != "reserve" || aliasEvent.Kind != "alias" || transferEvent.Kind != "transfer" || copyEvent.Kind != "copy" {
		t.Fatalf("ASSERT_C15_ATOMIC_BATCH_OWNERSHIP_ORDER_KIND batches=%+v", batches)
	}
	if aliasEvent.SiteID != reserveEvent.SiteID || transferEvent.SiteID != reserveEvent.SiteID || copyEvent.SiteID != privateC15SiteC15Copy02ParamsIngress {
		t.Fatalf("ASSERT_C15_ATOMIC_BATCH_EXACT_SITE batches=%+v", batches)
	}
	if aliasEvent.ObjectID != reserveEvent.ObjectID || transferEvent.ObjectID != reserveEvent.ObjectID || copyEvent.ObjectID == reserveEvent.ObjectID {
		t.Fatalf("ASSERT_C15_ATOMIC_BATCH_OBJECT_IDENTITY batches=%+v", batches)
	}
	if len(observer.snapshot()) != 4 || !reflect.DeepEqual(observer.snapshot(), []privateB4C15Event{reserveEvent, aliasEvent, transferEvent, copyEvent}) {
		t.Fatalf("ASSERT_C15_ATOMIC_BATCH_COMMITTED_CALLBACK_ORDER events=%+v batches=%+v", observer.snapshot(), batches)
	}
	copied.release()
	alias.release()
	moved.release()
	allBatches := observer.attemptedSnapshot()
	if len(allBatches) != 7 {
		t.Fatalf("ASSERT_C15_ATOMIC_BATCH_RELEASE_CALLBACK_COUNT got=%d batches=%+v", len(allBatches), allBatches)
	}
	for i, batch := range allBatches {
		if len(batch) != 1 {
			t.Fatalf("ASSERT_C15_ATOMIC_BATCH_ALL_SINGLETON_SIZE index=%d batch=%+v", i, batch)
		}
		if i > 0 && batch[0].Sequence != allBatches[i-1][0].Sequence+1 {
			t.Fatalf("ASSERT_C15_ATOMIC_BATCH_ALL_CONTIGUOUS index=%d batches=%+v", i, allBatches)
		}
	}
	copyRelease, aliasRelease, transferRelease := allBatches[4][0], allBatches[5][0], allBatches[6][0]
	if copyRelease.Kind != "release" || aliasRelease.Kind != "release" || transferRelease.Kind != "release" {
		t.Fatalf("ASSERT_C15_ATOMIC_BATCH_RELEASE_ORDER_KIND batches=%+v", allBatches)
	}
	if copyRelease.SiteID != copyEvent.SiteID || aliasRelease.SiteID != reserveEvent.SiteID || transferRelease.SiteID != reserveEvent.SiteID {
		t.Fatalf("ASSERT_C15_ATOMIC_BATCH_RELEASE_SITE batches=%+v", allBatches)
	}
	if copyRelease.ObjectID != copyEvent.ObjectID || aliasRelease.ObjectID != reserveEvent.ObjectID || transferRelease.ObjectID != reserveEvent.ObjectID {
		t.Fatalf("ASSERT_C15_ATOMIC_BATCH_RELEASE_IDENTITY batches=%+v", allBatches)
	}
	if !copyRelease.Terminal || aliasRelease.Terminal || !transferRelease.Terminal {
		t.Fatalf("ASSERT_C15_ATOMIC_BATCH_RELEASE_TERMINAL_BOUNDARY batches=%+v", allBatches)
	}
	finalEvents := observer.snapshot()
	terminal := map[string]int{}
	for _, event := range finalEvents {
		if event.Terminal {
			terminal[event.ObjectID]++
		}
	}
	if terminal[copyEvent.ObjectID] != 1 || terminal[reserveEvent.ObjectID] != 1 {
		t.Fatalf("ASSERT_C15_ATOMIC_BATCH_SINGLETON_TERMINAL_EXACT_ONCE terminal=%v events=%+v", terminal, finalEvents)
	}
}

func TestADR0011C17C15AtomicBatchObserverEnabledDisabledEquivalenceRED(t *testing.T) {
	run := func(t *testing.T, observer *privateC15AtomicBatchObserver) (OwnedMethodPair, privateB4ByteLedgerSnapshot) {
		t.Helper()
		account := mustPrivateB4ByteAccountV2(t)
		if observer != nil {
			privateC15InstallAtomicBatchObserver(t, account, observer)
		}
		owner, failure := newPrivateB4OwnedMethodPairOwner(account, OwnedMethodPair{Params: []byte("params"), Result: []byte("result")})
		if failure != "" || owner == nil {
			t.Fatalf("BLOCKED_NOT_RED atomic batch equivalence setup failure=%q", failure)
		}
		lease := OwnedMethodPairLease{owner: owner}
		var got OwnedMethodPair
		if err := lease.WithPair(func(view OwnedMethodPairView) error {
			got = OwnedMethodPair{Params: append([]byte(nil), view.Params...), Result: append([]byte(nil), view.Result...)}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		lease.Release()
		return got, account.snapshot()
	}
	disabledValue, disabledAccounting := run(t, nil)
	enabledValue, enabledAccounting := run(t, &privateC15AtomicBatchObserver{})
	if !reflect.DeepEqual(enabledValue, disabledValue) || enabledAccounting != disabledAccounting {
		t.Fatalf("ASSERT_C15_ATOMIC_BATCH_ENABLED_DISABLED_EQUIVALENCE enabled=%+v/%+v disabled=%+v/%+v", enabledValue, enabledAccounting, disabledValue, disabledAccounting)
	}
}
