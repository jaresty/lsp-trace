package sessionruntime

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-trace/internal/session"
)

func c17Event(ordinal int) privateB4EventIdentityC17 {
	return privateB4EventIdentityC17{Family: "ACQUISITION", Kind: "ELEMENT_BEGIN", Ordinal: ordinal}
}

func TestPrivateB4C17EventEqualityPlusOneRollbackAndIdentity(t *testing.T) {
	profile, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	for i := 0; i < privateB4EventLimit; i++ {
		if err := profile.WithEventAdmission(c17Event(i), func() error { return nil }); err != nil {
			t.Fatalf("ASSERT_C17_EQUALITY_8192 ordinal=%d err=%v", i, err)
		}
	}
	before := profile.eventSnapshot()
	called := false
	if err := profile.WithEventAdmission(c17Event(0), func() error { called = true; return nil }); !errors.Is(err, errPrivateB4EventAdmission) || called {
		t.Fatalf("ASSERT_C17_FULL_DUPLICATE_NO_RECHARGE err=%v callback=%t", err, called)
	}
	if duplicate := profile.eventSnapshot(); duplicate != before || duplicate.TerminalRequested || duplicate.TerminalFailure != "" {
		t.Fatalf("ASSERT_C17_FULL_DUPLICATE_NONTERMINAL before=%+v after=%+v", before, duplicate)
	}
	if err := profile.WithEventAdmission(c17Event(privateB4EventLimit), func() error { called = true; return nil }); !errors.Is(err, errPrivateB4EventAdmission) {
		t.Fatalf("ASSERT_C17_PLUS_ONE_REFUSAL err=%v", err)
	}
	if called {
		t.Fatal("ASSERT_C17_PLUS_ONE_BEFORE_CALLBACK")
	}
	if after := profile.eventSnapshot(); after.Admitted != before.Admitted || after.InFlight != before.InFlight || after.ActiveCallbacks != before.ActiveCallbacks || after.Admitted != privateB4EventLimit || !after.TerminalRequested || after.TerminalFailure != session.ResourceExhausted {
		t.Fatalf("ASSERT_C17_PLUS_ONE_ATOMIC before=%+v after=%+v", before, after)
	}

	rollback, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	identity := c17Event(0)
	callbackErr := errors.New("canonical append failed")
	if err := rollback.WithEventAdmission(identity, func() error { return callbackErr }); !errors.Is(err, callbackErr) {
		t.Fatalf("ASSERT_C17_CALLBACK_ERROR err=%v", err)
	}
	if snapshot := rollback.eventSnapshot(); snapshot.Admitted != 0 || snapshot.InFlight != 0 || snapshot.ActiveCallbacks != 0 || snapshot.TerminalRequested {
		t.Fatalf("ASSERT_C17_CALLBACK_ROLLBACK snapshot=%+v", snapshot)
	}
	if err := rollback.WithEventAdmission(identity, func() error { return nil }); err != nil {
		t.Fatalf("ASSERT_C17_ROLLED_BACK_IDENTITY_REUSABLE err=%v", err)
	}
	duplicateCalled := false
	if err := rollback.WithEventAdmission(identity, func() error { duplicateCalled = true; return nil }); !errors.Is(err, errPrivateB4EventAdmission) || duplicateCalled {
		t.Fatalf("ASSERT_C17_COMMITTED_IDENTITY_COUNTS_ONCE err=%v callback=%t", err, duplicateCalled)
	}
}

func TestPrivateB4C17MixedFamiliesShareOneCumulativeBudget(t *testing.T) {
	profile, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	identities := []privateB4EventIdentityC17{
		{Family: "ACQUISITION", Kind: "QUERY_BEGIN", Ordinal: 0},
		{Family: "CAPABILITY", Kind: "REGISTER", Ordinal: 0},
		{Family: "TARGET", Kind: "TARGET_BEGIN", Ordinal: 0},
		{Family: "TARGET", Kind: "TARGET_BEGIN", Ordinal: 1},
	}
	for _, identity := range identities {
		if err := profile.WithEventAdmission(identity, func() error { return nil }); err != nil {
			t.Fatalf("ASSERT_C17_MIXED_FAMILY_ADMISSION identity=%+v err=%v", identity, err)
		}
	}
	if snapshot := profile.eventSnapshot(); snapshot.Admitted != uint64(len(identities)) || snapshot.InFlight != 0 {
		t.Fatalf("ASSERT_C17_ONE_SHARED_DENOMINATOR snapshot=%+v", snapshot)
	}
	duplicateRan := false
	if err := profile.WithEventAdmission(identities[1], func() error { duplicateRan = true; return nil }); !errors.Is(err, errPrivateB4EventAdmission) || duplicateRan {
		t.Fatalf("ASSERT_C17_ALIAS_DOES_NOT_RECHARGE err=%v callback=%t", err, duplicateRan)
	}
	if snapshot := profile.eventSnapshot(); snapshot.Admitted != uint64(len(identities)) {
		t.Fatalf("ASSERT_C17_ALIAS_PRESERVES_SHARED_TOTAL snapshot=%+v", snapshot)
	}
}

func TestPrivateB4C17BatchValidationAndConcurrentDedup(t *testing.T) {
	profile, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	valid := c17Event(0)
	invalid := privateB4EventIdentityC17{Family: "TARGET", Kind: "TARGET_BEGIN", Ordinal: -1}
	called := false
	if err := profile.withEventAdmissions([]privateB4EventIdentityC17{valid, invalid}, func() error { called = true; return nil }); !errors.Is(err, errPrivateB4EventAdmission) || called {
		t.Fatalf("ASSERT_C17_BATCH_INVALID_REFUSAL err=%v callback=%t", err, called)
	}
	if snapshot := profile.eventSnapshot(); snapshot != (privateB4EventSnapshotC17{}) {
		t.Fatalf("ASSERT_C17_BATCH_INVALID_ZERO_EFFECT snapshot=%+v", snapshot)
	}

	entered := make(chan struct{})
	release := make(chan struct{})
	first := make(chan error, 1)
	go func() {
		first <- profile.WithEventAdmission(valid, func() error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered
	secondCalled := false
	if err := profile.WithEventAdmission(valid, func() error { secondCalled = true; return nil }); !errors.Is(err, errPrivateB4EventAdmission) || secondCalled {
		t.Fatalf("ASSERT_C17_CONCURRENT_DEDUP err=%v callback=%t", err, secondCalled)
	}
	close(release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if snapshot := profile.eventSnapshot(); snapshot.Admitted != 1 || snapshot.InFlight != 0 || snapshot.ActiveCallbacks != 0 {
		t.Fatalf("ASSERT_C17_CONCURRENT_ONE_WINNER snapshot=%+v", snapshot)
	}
}

func TestPrivateB4C17TargetPairBoundaries(t *testing.T) {
	atCap, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	for i := 0; i < privateB4EventLimit-2; i++ {
		if err := atCap.WithEventAdmission(c17Event(i), func() error { return nil }); err != nil {
			t.Fatal(err)
		}
	}
	if err := atCap.withTargetAppendAdmission(99, func() error { return nil }); err != nil {
		t.Fatalf("ASSERT_C17_TARGET_PAIR_AT_CAP err=%v", err)
	}
	if snapshot := atCap.eventSnapshot(); snapshot.Admitted != privateB4EventLimit || snapshot.TerminalRequested {
		t.Fatalf("ASSERT_C17_TARGET_PAIR_AT_CAP_COUNT snapshot=%+v", snapshot)
	}

	plusOne, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	for i := 0; i < privateB4EventLimit-1; i++ {
		if err := plusOne.WithEventAdmission(c17Event(i), func() error { return nil }); err != nil {
			t.Fatal(err)
		}
	}
	called := false
	if err := plusOne.withTargetAppendAdmission(99, func() error { called = true; return nil }); !errors.Is(err, errPrivateB4EventAdmission) || called {
		t.Fatalf("ASSERT_C17_TARGET_PAIR_PLUS_ONE err=%v callback=%t", err, called)
	}
	if snapshot := plusOne.eventSnapshot(); snapshot.Admitted != privateB4EventLimit-1 || !snapshot.TerminalRequested || snapshot.TerminalFailure != session.ResourceExhausted {
		t.Fatalf("ASSERT_C17_TARGET_PAIR_PLUS_ONE_ATOMIC snapshot=%+v", snapshot)
	}
}

func TestPrivateB4C17ConcurrentTerminalReleaseOwnedBacking(t *testing.T) {
	profile, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	var wg sync.WaitGroup
	failures := make(chan session.Failure, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			failures <- profile.requestTerminalReleaseC17(context.Background())
		}()
	}
	wg.Wait()
	close(failures)
	for failure := range failures {
		if failure != "" {
			t.Fatal(failure)
		}
	}
	if snapshot := profile.eventSnapshot(); !snapshot.BackingReleased {
		t.Fatalf("ASSERT_C17_CONCURRENT_TERMINAL snapshot=%+v", snapshot)
	}
	if bytes := profile.bytes.snapshot(); bytes.Live != 0 {
		t.Fatalf("ASSERT_C17_OWNED_BYTES_RELEASED snapshot=%+v", bytes)
	}
}

func TestPrivateB4C17PanicRollsBackIdentityAndCounters(t *testing.T) {
	profile, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	identity := privateB4EventIdentityC17{Family: "TARGET", Kind: "TARGET_TERMINAL", Ordinal: 7}
	func() {
		defer func() {
			if recovered := recover(); recovered != "append panic" {
				t.Fatalf("ASSERT_C17_PANIC_PROPAGATES recovered=%v", recovered)
			}
		}()
		_ = profile.WithEventAdmission(identity, func() error { panic("append panic") })
	}()
	if snapshot := profile.eventSnapshot(); snapshot.Admitted != 0 || snapshot.InFlight != 0 || snapshot.ActiveCallbacks != 0 || snapshot.TerminalRequested {
		t.Fatalf("ASSERT_C17_PANIC_ROLLBACK snapshot=%+v", snapshot)
	}
	if err := profile.WithEventAdmission(identity, func() error { return nil }); err != nil {
		t.Fatalf("ASSERT_C17_PANICKED_IDENTITY_REUSABLE err=%v", err)
	}
}

func TestPrivateB4C17TerminalWaitClosedAdmissionAndExactRelease(t *testing.T) {
	profile, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- profile.WithEventAdmission(c17Event(0), func() error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if failure := profile.requestTerminalReleaseC17(ctx); failure != session.RequestTimeout {
		t.Fatalf("ASSERT_C17_TERMINAL_TIMEOUT failure=%s", failure)
	}
	if snapshot := profile.eventSnapshot(); !snapshot.TerminalRequested || snapshot.BackingReleased || snapshot.ActiveCallbacks != 1 {
		t.Fatalf("ASSERT_C17_TIMEOUT_PRESERVES_CALLBACK snapshot=%+v", snapshot)
	}
	var callbackRan atomic.Bool
	if err := profile.WithEventAdmission(c17Event(1), func() error { callbackRan.Store(true); return nil }); !errors.Is(err, errPrivateB4EventAdmission) || callbackRan.Load() {
		t.Fatalf("ASSERT_C17_TERMINAL_CLOSES_ADMISSION err=%v callback=%t", err, callbackRan.Load())
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if failure := profile.requestTerminalReleaseC17(context.Background()); failure != "" {
		t.Fatalf("ASSERT_C17_TERMINAL_RETRY failure=%s", failure)
	}
	if failure := profile.requestTerminalReleaseC17(context.Background()); failure != "" {
		t.Fatalf("ASSERT_C17_TERMINAL_EXACT_ONCE failure=%s", failure)
	}
	if snapshot := profile.eventSnapshot(); snapshot.Admitted != 1 || snapshot.InFlight != 0 || snapshot.ActiveCallbacks != 0 || !snapshot.BackingReleased {
		t.Fatalf("ASSERT_C17_TERMINAL_SETTLED snapshot=%+v", snapshot)
	}
}

func TestPrivateB4C17TargetBorrowPairedAdmissionAndDefaultOff(t *testing.T) {
	called := false
	if err := (PrivateB4DefinitionBorrow{}).WithTargetAppendAdmission(0, func() error { called = true; return nil }); !errors.Is(err, ErrPrivateB4C17NotEnabled) || called {
		t.Fatalf("ASSERT_C17_TARGET_DEFAULT_OFF err=%v callback=%t", err, called)
	}

	profile, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	borrow := PrivateB4DefinitionBorrow{c17: profile}
	for _, ordinal := range []uint64{4, 5} {
		if err := borrow.WithTargetAppendAdmission(ordinal, func() error { return nil }); err != nil {
			t.Fatalf("ASSERT_C17_TARGET_PAIR ordinal=%d err=%v", ordinal, err)
		}
	}
	if snapshot := profile.eventSnapshot(); snapshot.Admitted != 4 || snapshot.InFlight != 0 {
		t.Fatalf("ASSERT_C17_TARGET_PAIR_COUNT snapshot=%+v", snapshot)
	}
	called = false
	if err := borrow.WithTargetAppendAdmission(4, func() error { called = true; return nil }); !errors.Is(err, errPrivateB4EventAdmission) || called {
		t.Fatalf("ASSERT_C17_TARGET_ALIAS_NO_RECHARGE err=%v callback=%t", err, called)
	}

	callbackErr := errors.New("target append failed")
	if err := borrow.WithTargetAppendAdmission(6, func() error { return callbackErr }); !errors.Is(err, callbackErr) {
		t.Fatalf("ASSERT_C17_TARGET_PAIR_ROLLBACK err=%v", err)
	}
	if snapshot := profile.eventSnapshot(); snapshot.Admitted != 4 || snapshot.InFlight != 0 {
		t.Fatalf("ASSERT_C17_TARGET_PAIR_ROLLBACK_COUNTS snapshot=%+v", snapshot)
	}
	if err := borrow.WithTargetAppendAdmission(6, func() error { return nil }); err != nil {
		t.Fatalf("ASSERT_C17_TARGET_PAIR_REUSABLE err=%v", err)
	}
}

func TestPrivateB4C17SelfStorageAndC16Isolation(t *testing.T) {
	bytes, failure := newPrivateB4ByteAccountV2()
	if failure != "" {
		t.Fatal(failure)
	}
	c16, failure := newPrivateB4AccountC16WithBytes(bytes)
	if failure != "" {
		t.Fatal(failure)
	}
	c16Before := c16.objectSnapshot()
	bytesBefore := bytes.snapshot()

	c17, failure := newPrivateB4EventAccountC17WithBytes(bytes)
	if failure != "" {
		t.Fatal(failure)
	}
	withC17 := bytes.snapshot()
	if cost := privateB4EventAccountC17SelfCost(); withC17.Live != bytesBefore.Live+cost || withC17.Cumulative != bytesBefore.Cumulative+cost {
		t.Fatalf("ASSERT_C17_EXACT_SELF_COST before=%+v after=%+v cost=%d", bytesBefore, withC17, cost)
	}
	if err := c17.WithEventAdmission(privateB4EventIdentityC17{Family: "CAPABILITY", Kind: "REGISTER", Ordinal: 0}, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if after := c16.objectSnapshot(); after != c16Before {
		t.Fatalf("ASSERT_C17_DOES_NOT_MUTATE_C16 before=%+v after=%+v", c16Before, after)
	}
	if failure := c17.requestTerminalReleaseC17(context.Background()); failure != "" {
		t.Fatal(failure)
	}
	afterC17 := bytes.snapshot()
	if afterC17.Live != bytesBefore.Live || afterC17.Cumulative != withC17.Cumulative {
		t.Fatalf("ASSERT_C17_RELEASES_ONLY_SELF_STORAGE before=%+v with=%+v after=%+v", bytesBefore, withC17, afterC17)
	}
	if after := c16.objectSnapshot(); after != c16Before {
		t.Fatalf("ASSERT_C17_RELEASE_PRESERVES_C16 before=%+v after=%+v", c16Before, after)
	}
	if failure := c16.requestTerminalReleaseC16(context.Background()); failure != "" {
		t.Fatal(failure)
	}
}
