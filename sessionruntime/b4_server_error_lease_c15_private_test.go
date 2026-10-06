package sessionruntime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

func serverErrorFixture() *lspwire.RPCError {
	return &lspwire.RPCError{Code: -32603, Message: "server boom", Data: []byte(`{"detail":"x"}`)}
}

func TestPrivateServerErrorLeaseCopiedHandlesPanicDrainAndRelease(t *testing.T) {
	a := mustPrivateB4ByteAccountV2(t)
	source := serverErrorFixture()
	want, _ := privateB4ServerErrorCapacity(source)
	before := a.snapshot()
	owner, failure := newPrivateB4ServerErrorOwner(a, source)
	if failure != "" || owner == nil {
		t.Fatalf("ASSERT_C15_SERVER_ERROR_OWNER failure=%q", failure)
	}
	source.Message = "changed"
	copy(source.Data, `{"bad":true}`)
	lease := ServerErrorLease{owner: owner}
	copyLease := lease
	panicked := make(chan any, 1)
	go func() {
		defer func() { panicked <- recover() }()
		_ = lease.WithServerError(func(v ServerErrorView) error {
			if v.Code != -32603 || v.Message != "server boom" || string(v.Data) != `{"detail":"x"}` {
				panic("source isolation")
			}
			panic("callback")
		})
	}()
	if recovered := <-panicked; recovered != "callback" {
		t.Fatalf("ASSERT_C15_SERVER_ERROR_ISOLATION panic=%v", recovered)
	}
	if !copyLease.Release() || lease.Release() {
		t.Fatal("ASSERT_C15_SERVER_ERROR_EXACT_ONCE")
	}
	if err := lease.WithServerError(func(ServerErrorView) error { return nil }); !errors.Is(err, ErrServerErrorLeaseClosed) {
		t.Fatalf("ASSERT_C15_SERVER_ERROR_STALE err=%v", err)
	}
	a.requestTerminalRelease()
	if got := a.snapshot(); got.Live != 0 || got.Cumulative != before.Cumulative+want {
		t.Fatalf("ASSERT_C15_SERVER_ERROR_QUIESCENCE %+v", got)
	}
}

func TestPrivateServerErrorLeaseLifecycleTimeoutReleaseRetry(t *testing.T) {
	f := stageBControls(t)
	m, child, key, result := stageBStopSelectedWithServerError(t, f, serverErrorFixture())
	lease, ok := result.PrivateServerErrorLease()
	if !ok || result.ServerError != nil {
		t.Fatal("ASSERT_C15_SERVER_ERROR_LIFECYCLE_LEASE")
	}
	callbackEntered := make(chan struct{})
	callbackLeave := make(chan struct{})
	callbackDone := make(chan error, 1)
	go func() {
		callbackDone <- lease.WithServerError(func(ServerErrorView) error {
			close(callbackEntered)
			<-callbackLeave
			return nil
		})
	}()
	<-callbackEntered
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
	intent := m.Stop(context.Background(), key.SessionID, "server-error-stop")
	if intent.Failure != "" || intent.IntentID == "" {
		t.Fatalf("ASSERT_C15_SERVER_ERROR_LIFECYCLE_INTENT %+v", intent)
	}
	lease.owner.mu.Lock()
	for !lease.owner.admissionClosed {
		lease.owner.cond.Wait()
	}
	lease.owner.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	timed := m.Stop(ctx, key.SessionID, "server-error-stop")
	cancel()
	if timed.Failure != session.RequestTimeout || timed.IntentID != intent.IntentID {
		t.Fatalf("ASSERT_C15_SERVER_ERROR_LIFECYCLE_TIMEOUT %+v", timed)
	}
	if err := lease.WithServerError(func(ServerErrorView) error { return nil }); !errors.Is(err, ErrServerErrorLeaseClosed) {
		t.Fatal("ASSERT_C15_SERVER_ERROR_LIFECYCLE_TERMINAL_ADMISSION")
	}
	releaseDone := make(chan bool, 1)
	go func() { releaseDone <- lease.Release() }()
	select {
	case <-releaseDone:
		t.Fatal("ASSERT_C15_SERVER_ERROR_LIFECYCLE_RELEASE_BEFORE_CALLBACK")
	default:
	}
	close(callbackLeave)
	if err := <-callbackDone; err != nil {
		t.Fatal(err)
	}
	if !<-releaseDone {
		t.Fatal("ASSERT_C15_SERVER_ERROR_LIFECYCLE_RELEASE")
	}
	lifecycleWait(t, m, intent.IntentID)
	m.mu.Lock()
	deleted := m.sessions[key.SessionID] == nil
	m.mu.Unlock()
	if !deleted {
		t.Fatal("ASSERT_C15_SERVER_ERROR_LIFECYCLE_SESSION_NOT_DELETED")
	}
	retryCtx, retryCancel := context.WithTimeout(context.Background(), time.Second)
	completed := m.Stop(retryCtx, key.SessionID, "server-error-stop")
	retryCancel()
	if completed.Failure != "" || completed.Outcome != session.OutcomeComplete {
		t.Fatalf("ASSERT_C15_SERVER_ERROR_LIFECYCLE_RETRY %+v", completed)
	}
	if err := <-served; err != nil {
		t.Fatal(err)
	}
}

func TestPrivateServerErrorLeaseConcurrentCopiedHandleRelease(t *testing.T) {
	a := mustPrivateB4ByteAccountV2(t)
	owner, failure := newPrivateB4ServerErrorOwner(a, serverErrorFixture())
	if failure != "" {
		t.Fatal(failure)
	}
	lease := ServerErrorLease{owner: owner}
	results := make(chan bool, 16)
	for i := 0; i < 16; i++ {
		copyLease := lease
		go func() { results <- copyLease.Release() }()
	}
	winners := 0
	for i := 0; i < 16; i++ {
		if <-results {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("ASSERT_C15_SERVER_ERROR_RELEASE_WINNERS %d", winners)
	}
	a.requestTerminalRelease()
	if a.snapshot().Live != 0 {
		t.Fatal("ASSERT_C15_SERVER_ERROR_RELEASE_RACE_QUIESCENCE")
	}
}

func TestPrivateServerErrorLeaseReserveBoundaries(t *testing.T) {
	source := serverErrorFixture()
	want, _ := privateB4ServerErrorCapacity(source)
	a := mustPrivateB4ByteAccountV2(t)
	filler, failure := a.reserve(privateB4MaxOwnedBytes - a.snapshot().Live - want)
	if failure != "" {
		t.Fatal(failure)
	}
	owner, failure := newPrivateB4ServerErrorOwner(a, source)
	if failure != "" || owner == nil || a.snapshot().Live != privateB4MaxOwnedBytes {
		t.Fatalf("ASSERT_C15_SERVER_ERROR_EQUALITY failure=%q snapshot=%+v", failure, a.snapshot())
	}
	(ServerErrorLease{owner: owner}).Release()
	filler.release()
	a.requestTerminalRelease()

	b := mustPrivateB4ByteAccountV2(t)
	fill, failure := b.reserve(privateB4MaxOwnedBytes - b.snapshot().Live - want + 1)
	if failure != "" {
		t.Fatal(failure)
	}
	before := b.snapshot()
	got, failure := newPrivateB4ServerErrorOwner(b, source)
	if failure == "" || got != nil || b.snapshot() != before {
		t.Fatalf("ASSERT_C15_SERVER_ERROR_PLUS_ONE failure=%q owner=%v before=%+v after=%+v", failure, got, before, b.snapshot())
	}
	fill.release()
	b.requestTerminalRelease()
}

func TestOrdinaryRoundTripServerErrorCompatibility(t *testing.T) {
	source := serverErrorFixture()
	f := newFullP1FixtureWithServerError(t, source)
	result := f.m.RoundTrip(context.Background(), f.req)
	if result.Failure != "" || result.ServerError == nil || result.ServerError.Code != source.Code || result.ServerError.Message != source.Message || string(result.ServerError.Data) != string(source.Data) {
		t.Fatalf("ASSERT_C15_SERVER_ERROR_ORDINARY_COMPAT failure=%q server=%+v", result.Failure, result.ServerError)
	}
	if _, ok := result.PrivateServerErrorLease(); ok {
		t.Fatal("ASSERT_C15_SERVER_ERROR_ORDINARY_LEASE")
	}
}

func TestPrivateServerErrorLeaseProductionPublicationAndRelease(t *testing.T) {
	f := newFullP1FixtureWithServerError(t, serverErrorFixture())
	var account *privateB4ByteAccountV2
	f.m.b4ParamsHook = func(a *privateB4ByteAccountV2) { account = a }
	result, managerLease := f.m.RoundTripPrivateB4(context.Background(), f.req, f.owner)
	if result.Failure != "" || result.ServerError != nil || managerLease != (B4DefinitionLease{}) {
		t.Fatalf("ASSERT_C15_SERVER_ERROR_PRODUCTION failure=%q legacy=%v manager=%v", result.Failure, result.ServerError, managerLease != (B4DefinitionLease{}))
	}
	lease, ok := result.PrivateServerErrorLease()
	if !ok {
		t.Fatal("ASSERT_C15_SERVER_ERROR_PRODUCTION_LEASE")
	}
	if err := lease.WithServerError(func(v ServerErrorView) error {
		if v.Code != -32603 || v.Message != "server boom" || string(v.Data) != `{"detail":"x"}` {
			return errors.New("wrong server error")
		}
		_ = f.m.Census()
		_ = account.snapshot()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if account.snapshot().Live == 0 || !lease.Release() || account.snapshot().Live != 0 {
		t.Fatalf("ASSERT_C15_SERVER_ERROR_PRODUCTION_RELEASE %+v", account.snapshot())
	}
}

func TestPrivateServerErrorLeaseProductionPlusOneRefusesAtomically(t *testing.T) {
	source := serverErrorFixture()
	f := newFullP1FixtureWithServerError(t, source)
	required, _ := privateB4ServerErrorCapacity(source)
	var account *privateB4ByteAccountV2
	var filler privateB4ByteLeaseV2
	var before privateB4ByteLedgerSnapshot
	f.m.b4ParamsHook = func(a *privateB4ByteAccountV2) {
		account = a
		remaining := privateB4MaxOwnedBytes - a.snapshot().Live
		var failure session.Failure
		filler, failure = a.reserve(remaining - required + 1)
		if failure != "" {
			t.Fatal(failure)
		}
		before = a.snapshot()
	}
	result, managerLease := f.m.RoundTripPrivateB4(context.Background(), f.req, f.owner)
	after := account.snapshot()
	if result.Failure != session.ResourceExhausted || result.ServerError != nil || managerLease != (B4DefinitionLease{}) || after.Live != before.Live {
		t.Fatalf("ASSERT_C15_SERVER_ERROR_PRODUCTION_PLUS_ONE failure=%q legacy=%v manager=%v before=%+v after=%+v", result.Failure, result.ServerError, managerLease != (B4DefinitionLease{}), before, after)
	}
	if _, ok := result.PrivateServerErrorLease(); ok {
		t.Fatal("ASSERT_C15_SERVER_ERROR_PRODUCTION_PLUS_ONE_LEASE")
	}
	filler.release()
	if account.snapshot().Live != 0 {
		t.Fatalf("ASSERT_C15_SERVER_ERROR_PRODUCTION_PLUS_ONE_CLEANUP %+v", account.snapshot())
	}
}

func TestPrivateServerErrorLeaseAbsentPublishesNoLease(t *testing.T) {
	f := newFullP1Fixture(t)
	result, managerLease, selection := f.transact(t)
	if _, ok := result.PrivateServerErrorLease(); ok {
		t.Fatal("ASSERT_C15_SERVER_ERROR_ABSENT_LEASE")
	}
	requestLease, _ := result.PrivateB4RequestFrameLease()
	responseLease, _ := result.PrivateB4ResponseFrameLease()
	resultLease, _ := result.PrivateB4ResultLease()
	_, _ = consumePrivateB4SnapshotForTest(f.m, managerLease, selection)
	requestLease.Release()
	responseLease.Release()
	resultLease.Release()
}
