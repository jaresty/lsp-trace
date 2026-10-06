package sessionruntime

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

func TestPrivateB4OwnedMethodPairLeaseLifecycleTimeoutReleaseRetry(t *testing.T) {
	f := stageBControls(t)
	m, child, _, key, result := stageBStopSelectedWithPair(t, f, true)
	lease, ok := result.PrivateOwnedMethodPairLease()
	if !ok {
		t.Fatal("ASSERT_C15_PAIR_LIFECYCLE_LEASE")
	}
	served := make(chan error, 1)
	go func() {
		reader := lspwire.NewReader(child.input, lspwire.DefaultLimits())
		message, err := reader.Read()
		if err != nil || message.Method != "shutdown" {
			served <- errors.New("shutdown")
			return
		}
		if err = lspwire.NewWriter(child.output, lspwire.DefaultLimits()).Write(lspwire.Message{JSONRPC: lspwire.Version, ID: message.ID, Result: json.RawMessage("null")}); err != nil {
			served <- err
			return
		}
		message, err = reader.Read()
		if err != nil || message.Method != "exit" {
			served <- errors.New("exit")
			return
		}
		served <- nil
	}()
	intent := m.Stop(context.Background(), key.SessionID, "pair-stop")
	if intent.Failure != "" || intent.IntentID == "" {
		t.Fatalf("ASSERT_C15_PAIR_LIFECYCLE_INTENT %+v", intent)
	}
	lease.owner.mu.Lock()
	for !lease.owner.admissionClosed {
		lease.owner.cond.Wait()
	}
	lease.owner.mu.Unlock()
	timedResults := make(chan session.LifecycleResult, 2)
	for i := 0; i < 2; i++ {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			timedResults <- m.Stop(ctx, key.SessionID, "pair-stop")
		}()
	}
	for i := 0; i < 2; i++ {
		if timed := <-timedResults; timed.Failure != session.RequestTimeout || timed.IntentID != intent.IntentID {
			t.Fatalf("ASSERT_C15_PAIR_LIFECYCLE_TIMEOUT %+v", timed)
		}
	}
	if err := lease.WithPair(func(OwnedMethodPairView) error { return nil }); !errors.Is(err, ErrOwnedMethodPairLeaseClosed) {
		t.Fatal("ASSERT_C15_PAIR_LIFECYCLE_TERMINAL_ADMISSION")
	}
	if lease.owner.charge.account.snapshot().Live == 0 || !lease.Release() {
		t.Fatal("ASSERT_C15_PAIR_LIFECYCLE_VALID_AFTER_TIMEOUT")
	}
	lifecycleWait(t, m, intent.IntentID)
	m.mu.Lock()
	deleted := m.sessions[key.SessionID] == nil
	m.mu.Unlock()
	if !deleted {
		t.Fatal("ASSERT_C15_PAIR_LIFECYCLE_SESSION_NOT_DELETED")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	completed := m.Stop(ctx, key.SessionID, "pair-stop")
	cancel()
	if completed.Failure != "" || completed.Outcome != session.OutcomeComplete {
		t.Fatalf("ASSERT_C15_PAIR_LIFECYCLE_RETRY %+v", completed)
	}
	if err := <-served; err != nil {
		t.Fatal(err)
	}
}

func TestPrivateB4OwnedMethodPairLeaseProductionPlusOneRefusesAtomically(t *testing.T) {
	f := newFullP1Fixture(t)
	f.req.CaptureOwnedMethodPair = true
	var account *privateB4ByteAccountV2
	var filler privateB4ByteLeaseV2
	var before privateB4ByteLedgerSnapshot
	afterOwned := 0
	f.m.b4OwnedPairHooks.beforeReserve = func(a *privateB4ByteAccountV2, required uint64) {
		account = a
		remaining := privateB4MaxOwnedBytes - a.snapshot().Live
		var failure session.Failure
		filler, failure = a.reserve(remaining - required + 1)
		if failure != "" {
			t.Fatal(failure)
		}
		before = a.snapshot()
	}
	f.m.b4OwnedPairHooks.afterOwned = func(*privateB4ByteAccountV2, uint64) { afterOwned++ }
	f.m.b4OwnedPairHooks.refused = func(a *privateB4ByteAccountV2, _ uint64) {
		if a.snapshot() != before {
			t.Fatalf("ASSERT_C15_PAIR_PRODUCTION_PLUS_ONE_MUTATION before=%+v after=%+v", before, a.snapshot())
		}
	}
	result, managerLease := f.m.RoundTripPrivateB4(context.Background(), f.req, f.owner)
	if result.Failure != session.ResourceExhausted || managerLease != (B4DefinitionLease{}) || afterOwned != 0 {
		t.Fatalf("ASSERT_C15_PAIR_PRODUCTION_PLUS_ONE failure=%q lease=%+v owned=%d", result.Failure, managerLease, afterOwned)
	}
	if _, ok := result.PrivateOwnedMethodPairLease(); ok {
		t.Fatal("ASSERT_C15_PAIR_PRODUCTION_PLUS_ONE_LEASE")
	}
	if _, ok := result.CompletedOwnedMethodPair(); ok {
		t.Fatal("ASSERT_C15_PAIR_PRODUCTION_PLUS_ONE_LEGACY")
	}
	filler.release()
	if account.snapshot().Live != 0 {
		t.Fatalf("ASSERT_C15_PAIR_PRODUCTION_PLUS_ONE_CLEANUP %+v", account.snapshot())
	}
}

func TestPrivateB4OwnedMethodPairLeaseProductionPublication(t *testing.T) {
	f := newFullP1Fixture(t)
	f.req.CaptureOwnedMethodPair = true
	var account *privateB4ByteAccountV2
	f.m.b4ParamsHook = func(a *privateB4ByteAccountV2) { account = a }
	result, managerLease, selection := f.transact(t)
	lease, ok := result.PrivateOwnedMethodPairLease()
	if !ok {
		t.Fatal("ASSERT_C15_PAIR_PRODUCTION_LEASE")
	}
	if _, legacy := result.CompletedOwnedMethodPair(); legacy {
		t.Fatal("ASSERT_C15_PAIR_PRIVATE_LEGACY_PUBLICATION")
	}
	if err := lease.WithPair(func(v OwnedMethodPairView) error {
		if v.Method != f.req.Method || len(v.Params) == 0 || len(v.Result) == 0 {
			return errors.New("wrong production view")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	requestLease, _ := result.PrivateB4RequestFrameLease()
	responseLease, _ := result.PrivateB4ResponseFrameLease()
	resultLease, _ := result.PrivateB4ResultLease()
	if _, status := consumePrivateB4SnapshotForTest(f.m, managerLease, selection); status != PrivateB4Selected {
		t.Fatalf("ASSERT_C15_PAIR_PRODUCTION_CONSUME status=%s", status)
	}
	if !requestLease.Release() || !responseLease.Release() || !resultLease.Release() || account.snapshot().Live == 0 {
		t.Fatalf("ASSERT_C15_PAIR_PRODUCTION_RETAINED snapshot=%+v", account.snapshot())
	}
	if !lease.Release() || account.snapshot().Live != 0 {
		t.Fatalf("ASSERT_C15_PAIR_PRODUCTION_RELEASE snapshot=%+v", account.snapshot())
	}
}

func TestPrivateOwnedMethodPairLeaseCopiedHandlesDrainAndRelease(t *testing.T) {
	a := mustPrivateB4ByteAccountV2(t)
	before := a.snapshot()
	owner, failure := newPrivateB4OwnedMethodPairOwner(a, OwnedMethodPair{Params: []byte("params"), Result: []byte("result")})
	if failure != "" {
		t.Fatal(failure)
	}
	want := uint64(len("params") + len("result"))
	if got := a.snapshot(); got.Live != before.Live+want || got.Cumulative != before.Cumulative+want {
		t.Fatalf("ASSERT_C15_PAIR_OWNED snapshot=%+v", got)
	}
	lease := OwnedMethodPairLease{owner: owner}
	copyLease := lease
	entered := make(chan struct{})
	leave := make(chan struct{})
	callbackDone := make(chan error, 1)
	go func() {
		callbackDone <- lease.WithPair(func(v OwnedMethodPairView) error {
			if string(v.Params) != "params" || string(v.Result) != "result" {
				return errors.New("wrong view")
			}
			close(entered)
			<-leave
			return nil
		})
	}()
	<-entered
	releaseDone := make(chan bool, 1)
	go func() { releaseDone <- copyLease.Release() }()
	owner.mu.Lock()
	for !owner.releaseStarted {
		owner.cond.Wait()
	}
	owner.mu.Unlock()
	select {
	case <-releaseDone:
		t.Fatal("ASSERT_C15_PAIR_RELEASE_BEFORE_CALLBACK_DRAIN")
	default:
	}
	if err := lease.WithPair(func(OwnedMethodPairView) error { return nil }); !errors.Is(err, ErrOwnedMethodPairLeaseClosed) {
		t.Fatalf("ASSERT_C15_PAIR_POST_CLOSE_ADMISSION err=%v", err)
	}
	close(leave)
	if err := <-callbackDone; err != nil {
		t.Fatal(err)
	}
	if !<-releaseDone || lease.Release() || copyLease.Release() {
		t.Fatal("ASSERT_C15_PAIR_EXACT_ONCE_RELEASE")
	}
	if err := lease.WithPair(func(OwnedMethodPairView) error { return nil }); !errors.Is(err, ErrOwnedMethodPairLeaseClosed) {
		t.Fatal("ASSERT_C15_PAIR_STALE_ACCESS")
	}
	a.requestTerminalRelease()
	if got := a.snapshot(); got.Live != 0 || got.Cumulative != before.Cumulative+want {
		t.Fatalf("ASSERT_C15_PAIR_QUIESCENCE snapshot=%+v", got)
	}
}

func TestPrivateOwnedMethodPairLeaseCallbackReentersManagerAndAccount(t *testing.T) {
	m := newC15DiagnosticManager(t)
	a := mustPrivateB4ByteAccountV2(t)
	owner, failure := newPrivateB4OwnedMethodPairOwner(a, OwnedMethodPair{Params: []byte("p"), Result: []byte("r")})
	if failure != "" {
		t.Fatal(failure)
	}
	lease := OwnedMethodPairLease{owner: owner}
	if err := lease.WithPair(func(OwnedMethodPairView) error {
		_ = m.Census()
		if a.snapshot().Live == 0 {
			return errors.New("missing charge")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !lease.Release() {
		t.Fatal("ASSERT_C15_PAIR_CALLBACK_REENTRY_RELEASE")
	}
	a.requestTerminalRelease()
}

func TestPrivateOwnedMethodPairLeaseTerminalClosesAdmissionAndDrainsCallback(t *testing.T) {
	a := mustPrivateB4ByteAccountV2(t)
	owner, failure := newPrivateB4OwnedMethodPairOwner(a, OwnedMethodPair{Params: []byte("p"), Result: []byte("r")})
	if failure != "" {
		t.Fatal(failure)
	}
	lease := OwnedMethodPairLease{owner: owner}
	entered := make(chan struct{})
	leave := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- lease.WithPair(func(OwnedMethodPairView) error { close(entered); <-leave; return nil })
	}()
	<-entered
	before := a.snapshot()
	owner.requestTerminal()
	if err := lease.WithPair(func(OwnedMethodPairView) error { return nil }); !errors.Is(err, ErrOwnedMethodPairLeaseClosed) || a.snapshot() != before {
		t.Fatalf("ASSERT_C15_PAIR_TERMINAL_ADMISSION err=%v before=%+v after=%+v", err, before, a.snapshot())
	}
	released := make(chan bool, 1)
	go func() { released <- lease.Release() }()
	select {
	case <-released:
		t.Fatal("ASSERT_C15_PAIR_TERMINAL_CALLBACK_EARLY_RELEASE")
	default:
	}
	close(leave)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !<-released {
		t.Fatal("ASSERT_C15_PAIR_TERMINAL_RELEASE")
	}
	a.requestTerminalRelease()
	if a.snapshot().Live != 0 {
		t.Fatal("ASSERT_C15_PAIR_TERMINAL_QUIESCENCE")
	}
}

func TestPrivateOwnedMethodPairLeasePostAccountTerminalRefusesAtomically(t *testing.T) {
	a := mustPrivateB4ByteAccountV2(t)
	a.requestTerminalRelease()
	before := a.snapshot()
	owner, failure := newPrivateB4OwnedMethodPairOwner(a, OwnedMethodPair{Params: []byte("p"), Result: []byte("r")})
	if failure == "" || owner != nil || a.snapshot() != before {
		t.Fatalf("ASSERT_C15_PAIR_POST_TERMINAL_REFUSAL failure=%q owner=%v before=%+v after=%+v", failure, owner, before, a.snapshot())
	}
}

func TestPrivateOwnedMethodPairLeaseEqualityAndActualPlusOne(t *testing.T) {
	pair := OwnedMethodPair{Params: []byte("params"), Result: []byte("result")}
	required := uint64(len(pair.Params) + len(pair.Result))
	a := mustPrivateB4ByteAccountV2(t)
	before := a.snapshot()
	filler, failure := a.reserve(privateB4MaxOwnedBytes - before.Live - required)
	if failure != "" {
		t.Fatal(failure)
	}
	owner, failure := newPrivateB4OwnedMethodPairOwner(a, pair)
	if failure != "" || a.snapshot().Live != privateB4MaxOwnedBytes {
		t.Fatalf("ASSERT_C15_PAIR_EQUALITY failure=%q snapshot=%+v", failure, a.snapshot())
	}
	if !((OwnedMethodPairLease{owner: owner}).Release()) {
		t.Fatal("ASSERT_C15_PAIR_EQUALITY_RELEASE")
	}
	filler.release()
	a.requestTerminalRelease()
	if a.snapshot().Live != 0 {
		t.Fatal("ASSERT_C15_PAIR_EQUALITY_QUIESCENCE")
	}

	b := mustPrivateB4ByteAccountV2(t)
	start := b.snapshot()
	fill, failure := b.reserve(privateB4MaxOwnedBytes - start.Live - required + 1)
	if failure != "" {
		t.Fatal(failure)
	}
	blocked := b.snapshot()
	if got, failure := newPrivateB4OwnedMethodPairOwner(b, pair); failure == "" || got != nil || b.snapshot() != blocked {
		t.Fatalf("ASSERT_C15_PAIR_PLUS_ONE failure=%q owner=%v before=%+v after=%+v", failure, got, blocked, b.snapshot())
	}
	fill.release()
	b.requestTerminalRelease()
	if b.snapshot().Live != 0 {
		t.Fatal("ASSERT_C15_PAIR_PLUS_ONE_QUIESCENCE")
	}
}

func TestPrivateOwnedMethodPairLeaseReleaseRaceOneWinner(t *testing.T) {
	a := mustPrivateB4ByteAccountV2(t)
	owner, failure := newPrivateB4OwnedMethodPairOwner(a, OwnedMethodPair{Params: []byte("p"), Result: []byte("r")})
	if failure != "" {
		t.Fatal(failure)
	}
	lease := OwnedMethodPairLease{owner: owner}
	var winners atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(l OwnedMethodPairLease) {
			defer wg.Done()
			if l.Release() {
				winners.Add(1)
			}
		}(lease)
	}
	wg.Wait()
	if winners.Load() != 1 {
		t.Fatalf("ASSERT_C15_PAIR_RELEASE_WINNERS %d", winners.Load())
	}
	a.requestTerminalRelease()
	if a.snapshot().Live != 0 {
		t.Fatal("ASSERT_C15_PAIR_RELEASE_RACE_QUIESCENCE")
	}
}
