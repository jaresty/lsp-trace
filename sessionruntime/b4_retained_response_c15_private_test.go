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

func retainedResponseFixture() []lspwire.Message {
	return []lspwire.Message{
		{JSONRPC: lspwire.Version, ID: []byte("99"), Result: []byte(`{"other":1}`)},
		{JSONRPC: lspwire.Version, ID: []byte(`"bad"`), Error: &lspwire.RPCError{Code: -32603, Message: "wrong", Data: []byte(`{"d":1}`)}},
	}
}

func TestPrivateRetainedResponseLeaseTransactionalCopyDrainAndRelease(t *testing.T) {
	a := mustPrivateB4ByteAccountV2(t)
	source := retainedResponseFixture()
	want, _ := retainedResponseCapacity(source)
	before := a.snapshot()
	owner, failure := newPrivateB4RetainedResponseOwner(a, source)
	if failure != "" || owner == nil {
		t.Fatalf("ASSERT_C15_RESPONSE_OWNER failure=%q", failure)
	}
	copy(source[0].Result, `{"bad":true}`)
	source[1].Error.Message = "changed"
	copy(source[1].Error.Data, `{"x":2}`)
	lease := RetainedResponseLease{owner: owner}
	copyLease := lease
	entered := make(chan struct{})
	leave := make(chan struct{})
	callbackDone := make(chan error, 1)
	go func() {
		callbackDone <- lease.WithResponses(func(messages []lspwire.Message) error {
			if len(messages) != 2 || string(messages[0].Result) != `{"other":1}` || messages[1].Error == nil || messages[1].Error.Message != "wrong" || string(messages[1].Error.Data) != `{"d":1}` {
				return errors.New("wrong response view")
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
	if err := lease.WithResponses(func([]lspwire.Message) error { return nil }); !errors.Is(err, ErrRetainedResponseLeaseClosed) {
		t.Fatalf("ASSERT_C15_RESPONSE_POST_CLOSE err=%v", err)
	}
	close(leave)
	if err := <-callbackDone; err != nil {
		t.Fatal(err)
	}
	if !<-releaseDone || lease.Release() {
		t.Fatal("ASSERT_C15_RESPONSE_EXACT_ONCE")
	}
	a.requestTerminalRelease()
	if got := a.snapshot(); got.Live != 0 || got.Cumulative != before.Cumulative+want {
		t.Fatalf("ASSERT_C15_RESPONSE_QUIESCENCE %+v", got)
	}
}

func TestPrivateRetainedResponseLeaseLifecycleTimeoutReleaseRetry(t *testing.T) {
	f := stageBControls(t)
	responses := retainedResponseFixture()[:1]
	m, child, _, key, result := stageBStopSelectedWithResponses(t, f, responses)
	lease, ok := result.PrivateRetainedResponseLease()
	if !ok || len(result.Responses) != 0 {
		t.Fatal("ASSERT_C15_RESPONSE_LIFECYCLE_LEASE")
	}
	callbackEntered := make(chan struct{})
	callbackLeave := make(chan struct{})
	callbackDone := make(chan error, 1)
	go func() {
		callbackDone <- lease.WithResponses(func([]lspwire.Message) error { close(callbackEntered); <-callbackLeave; return nil })
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
	intent := m.Stop(context.Background(), key.SessionID, "response-stop")
	if intent.Failure != "" || intent.IntentID == "" {
		t.Fatalf("ASSERT_C15_RESPONSE_LIFECYCLE_INTENT %+v", intent)
	}
	lease.owner.mu.Lock()
	for !lease.owner.admissionClosed {
		lease.owner.cond.Wait()
	}
	lease.owner.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	timed := m.Stop(ctx, key.SessionID, "response-stop")
	cancel()
	if timed.Failure != session.RequestTimeout || timed.IntentID != intent.IntentID {
		t.Fatalf("ASSERT_C15_RESPONSE_LIFECYCLE_TIMEOUT %+v", timed)
	}
	if err := lease.WithResponses(func([]lspwire.Message) error { return nil }); !errors.Is(err, ErrRetainedResponseLeaseClosed) {
		t.Fatal("ASSERT_C15_RESPONSE_LIFECYCLE_TERMINAL_ADMISSION")
	}
	releaseDone := make(chan bool, 1)
	go func() { releaseDone <- lease.Release() }()
	select {
	case <-releaseDone:
		t.Fatal("ASSERT_C15_RESPONSE_LIFECYCLE_PREMATURE_RELEASE")
	default:
	}
	close(callbackLeave)
	if err := <-callbackDone; err != nil {
		t.Fatal(err)
	}
	if !<-releaseDone {
		t.Fatal("ASSERT_C15_RESPONSE_LIFECYCLE_RELEASE")
	}
	lifecycleWait(t, m, intent.IntentID)
	retryCtx, retryCancel := context.WithTimeout(context.Background(), time.Second)
	completed := m.Stop(retryCtx, key.SessionID, "response-stop")
	retryCancel()
	if completed.Failure != "" || completed.Outcome != session.OutcomeComplete {
		t.Fatalf("ASSERT_C15_RESPONSE_LIFECYCLE_RETRY %+v", completed)
	}
	if err := <-served; err != nil {
		t.Fatal(err)
	}
}

func TestPrivateRetainedResponseLeaseReserveBoundaries(t *testing.T) {
	source := retainedResponseFixture()
	want, _ := retainedResponseCapacity(source)
	a := mustPrivateB4ByteAccountV2(t)
	filler, failure := a.reserve(privateB4MaxOwnedBytes - a.snapshot().Live - want)
	if failure != "" {
		t.Fatal(failure)
	}
	owner, failure := newPrivateB4RetainedResponseOwner(a, source)
	if failure != "" || owner == nil || a.snapshot().Live != privateB4MaxOwnedBytes {
		t.Fatalf("ASSERT_C15_RESPONSE_EQUALITY failure=%q snapshot=%+v", failure, a.snapshot())
	}
	(RetainedResponseLease{owner: owner}).Release()
	filler.release()
	a.requestTerminalRelease()

	b := mustPrivateB4ByteAccountV2(t)
	fill, failure := b.reserve(privateB4MaxOwnedBytes - b.snapshot().Live - want + 1)
	if failure != "" {
		t.Fatal(failure)
	}
	before := b.snapshot()
	got, failure := newPrivateB4RetainedResponseOwner(b, source)
	if failure == "" || got != nil || b.snapshot() != before {
		t.Fatalf("ASSERT_C15_RESPONSE_PLUS_ONE failure=%q owner=%v before=%+v after=%+v", failure, got, before, b.snapshot())
	}
	fill.release()
	b.requestTerminalRelease()
}

func TestPrivateRetainedResponseLeaseProductionPublication(t *testing.T) {
	responses := retainedResponseFixture()
	f := newFullP1FixtureWithResponses(t, responses)
	f.req.MaxMessages = 3
	var account *privateB4ByteAccountV2
	f.m.b4ParamsHook = func(a *privateB4ByteAccountV2) { account = a }
	result, managerLease, selection := f.transact(t)
	if len(result.Responses) != 0 || result.Messages != 3 {
		t.Fatalf("ASSERT_C15_RESPONSE_LEGACY count=%d messages=%d", len(result.Responses), result.Messages)
	}
	lease, ok := result.PrivateRetainedResponseLease()
	if !ok {
		t.Fatal("ASSERT_C15_RESPONSE_PRODUCTION_LEASE")
	}
	if err := lease.WithResponses(func(messages []lspwire.Message) error {
		if len(messages) != 2 || string(messages[0].ID) != "99" || messages[1].Error == nil {
			return errors.New("wrong production responses")
		}
		_ = f.m.Census()
		_ = account.snapshot()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	requestLease, _ := result.PrivateB4RequestFrameLease()
	responseLease, _ := result.PrivateB4ResponseFrameLease()
	resultLease, _ := result.PrivateB4ResultLease()
	if _, status := consumePrivateB4SnapshotForTest(f.m, managerLease, selection); status != PrivateB4Selected {
		t.Fatal(status)
	}
	requestReleased := requestLease.Release()
	responseReleased := responseLease.Release()
	resultReleased := resultLease.Release()
	if !requestReleased || !responseReleased || !resultReleased {
		t.Fatalf("ASSERT_C15_RESPONSE_CALLER_RELEASE request=%t response=%t result=%t", requestReleased, responseReleased, resultReleased)
	}
	beforeResponseRelease := account.snapshot()
	responseOwnerBytes, _ := retainedResponseCapacity(responses)
	if beforeResponseRelease.Live != privateB4ByteLedgerV2TableBytes+responseOwnerBytes || !lease.Release() || account.snapshot().Live != 0 {
		t.Fatalf("ASSERT_C15_RESPONSE_PRODUCTION_RELEASE before=%+v response_bytes=%d after=%+v", beforeResponseRelease, responseOwnerBytes, account.snapshot())
	}
}

func TestPrivateRetainedResponseLeaseProductionPlusOneRefusesAtomically(t *testing.T) {
	responses := retainedResponseFixture()
	f := newFullP1FixtureWithResponses(t, responses)
	f.req.MaxMessages = 3
	required, _ := retainedResponseCapacity(responses)
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
	if result.Failure != session.ResourceExhausted || managerLease != (B4DefinitionLease{}) || len(result.Responses) != 0 || after.Live != before.Live {
		t.Fatalf("ASSERT_C15_RESPONSE_PRODUCTION_PLUS_ONE failure=%q manager=%v legacy=%d before=%+v after=%+v", result.Failure, managerLease != (B4DefinitionLease{}), len(result.Responses), before, after)
	}
	if _, ok := result.PrivateRetainedResponseLease(); ok {
		t.Fatal("ASSERT_C15_RESPONSE_PRODUCTION_PLUS_ONE_LEASE")
	}
	filler.release()
	if account.snapshot().Live != 0 {
		t.Fatalf("ASSERT_C15_RESPONSE_PRODUCTION_PLUS_ONE_CLEANUP %+v", account.snapshot())
	}
}

func TestPrivateRetainedResponseLeaseEmptySetNoLease(t *testing.T) {
	f := newFullP1Fixture(t)
	result, managerLease, selection := f.transact(t)
	if _, ok := result.PrivateRetainedResponseLease(); ok {
		t.Fatal("ASSERT_C15_RESPONSE_EMPTY_LEASE")
	}
	requestLease, _ := result.PrivateB4RequestFrameLease()
	responseLease, _ := result.PrivateB4ResponseFrameLease()
	resultLease, _ := result.PrivateB4ResultLease()
	_, _ = consumePrivateB4SnapshotForTest(f.m, managerLease, selection)
	requestLease.Release()
	responseLease.Release()
	resultLease.Release()
}
