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

func notificationFixture() []lspwire.Message {
	return []lspwire.Message{{JSONRPC: lspwire.Version, Method: "$/progress", Params: []byte(`{"value":{}}`)}}
}

func TestPrivateRetainedNotificationLeaseCopiedHandlesDrainAndRelease(t *testing.T) {
	a := mustPrivateB4ByteAccountV2(t)
	source := notificationFixture()
	want, ok := retainedNotificationCapacity(source)
	if !ok {
		t.Fatal("ASSERT_C15_NOTIFICATION_CAPACITY")
	}
	before := a.snapshot()
	owner, failure := newPrivateB4RetainedNotificationOwner(a, source)
	if failure != "" || owner == nil {
		t.Fatalf("ASSERT_C15_NOTIFICATION_OWNER failure=%q", failure)
	}
	if got := a.snapshot(); got.Live != before.Live+want || got.Cumulative != before.Cumulative+want {
		t.Fatalf("ASSERT_C15_NOTIFICATION_CHARGE got=%+v want=%d", got, want)
	}
	lease := RetainedNotificationLease{owner: owner}
	copyLease := lease
	entered := make(chan struct{})
	leave := make(chan struct{})
	callbackDone := make(chan error, 1)
	go func() {
		callbackDone <- lease.WithNotifications(func(messages []lspwire.Message) error {
			if len(messages) != 1 || messages[0].Method != "$/progress" || string(messages[0].Params) != `{"value":{}}` {
				return errors.New("wrong notification view")
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
	if err := lease.WithNotifications(func([]lspwire.Message) error { return nil }); !errors.Is(err, ErrRetainedNotificationLeaseClosed) {
		t.Fatalf("ASSERT_C15_NOTIFICATION_POST_CLOSE err=%v", err)
	}
	close(leave)
	if err := <-callbackDone; err != nil {
		t.Fatal(err)
	}
	if !<-releaseDone || lease.Release() || copyLease.Release() {
		t.Fatal("ASSERT_C15_NOTIFICATION_EXACT_ONCE")
	}
	a.requestTerminalRelease()
	if got := a.snapshot(); got.Live != 0 || got.Cumulative != before.Cumulative+want {
		t.Fatalf("ASSERT_C15_NOTIFICATION_QUIESCENCE %+v", got)
	}
}

func TestPrivateRetainedNotificationLeaseSourceIsolationAndPanicDrain(t *testing.T) {
	a := mustPrivateB4ByteAccountV2(t)
	source := notificationFixture()
	owner, failure := newPrivateB4RetainedNotificationOwner(a, source)
	if failure != "" {
		t.Fatal(failure)
	}
	lease := RetainedNotificationLease{owner: owner}
	source[0].JSONRPC = "changed"
	source[0].Method = "changed"
	copy(source[0].Params, `{"bad":true}`)
	panicked := make(chan any, 1)
	go func() {
		defer func() { panicked <- recover() }()
		_ = lease.WithNotifications(func(messages []lspwire.Message) error {
			if messages[0].JSONRPC != lspwire.Version || messages[0].Method != "$/progress" || string(messages[0].Params) != `{"value":{}}` {
				panic("source isolation")
			}
			panic("callback")
		})
	}()
	if recovered := <-panicked; recovered != "callback" {
		t.Fatalf("ASSERT_C15_NOTIFICATION_SOURCE_ISOLATION panic=%v", recovered)
	}
	if !lease.Release() {
		t.Fatal("ASSERT_C15_NOTIFICATION_PANIC_DRAIN")
	}
	a.requestTerminalRelease()
	if a.snapshot().Live != 0 {
		t.Fatal("ASSERT_C15_NOTIFICATION_PANIC_QUIESCENCE")
	}
}

func TestPrivateB4RetainedNotificationLeaseLifecycleTimeoutReleaseRetry(t *testing.T) {
	f := stageBControls(t)
	m, child, _, key, result := stageBStopSelectedWithOptions(t, f, false, true)
	lease, ok := result.PrivateRetainedNotificationLease()
	if !ok || len(result.Notifications) != 0 {
		t.Fatal("ASSERT_C15_NOTIFICATION_LIFECYCLE_LEASE")
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
	intent := m.Stop(context.Background(), key.SessionID, "notification-stop")
	if intent.Failure != "" || intent.IntentID == "" {
		t.Fatalf("ASSERT_C15_NOTIFICATION_LIFECYCLE_INTENT %+v", intent)
	}
	lease.owner.mu.Lock()
	for !lease.owner.admissionClosed {
		lease.owner.cond.Wait()
	}
	lease.owner.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	timed := m.Stop(ctx, key.SessionID, "notification-stop")
	cancel()
	if timed.Failure != session.RequestTimeout || timed.IntentID != intent.IntentID {
		t.Fatalf("ASSERT_C15_NOTIFICATION_LIFECYCLE_TIMEOUT %+v", timed)
	}
	if err := lease.WithNotifications(func([]lspwire.Message) error { return nil }); !errors.Is(err, ErrRetainedNotificationLeaseClosed) {
		t.Fatal("ASSERT_C15_NOTIFICATION_LIFECYCLE_TERMINAL_ADMISSION")
	}
	if !lease.Release() {
		t.Fatal("ASSERT_C15_NOTIFICATION_LIFECYCLE_RELEASE")
	}
	lifecycleWait(t, m, intent.IntentID)
	m.mu.Lock()
	deleted := m.sessions[key.SessionID] == nil
	m.mu.Unlock()
	if !deleted {
		t.Fatal("ASSERT_C15_NOTIFICATION_LIFECYCLE_SESSION_NOT_DELETED")
	}
	retryCtx, retryCancel := context.WithTimeout(context.Background(), time.Second)
	completed := m.Stop(retryCtx, key.SessionID, "notification-stop")
	retryCancel()
	if completed.Failure != "" || completed.Outcome != session.OutcomeComplete {
		t.Fatalf("ASSERT_C15_NOTIFICATION_LIFECYCLE_RETRY %+v", completed)
	}
	if err := <-served; err != nil {
		t.Fatal(err)
	}
}

func TestPrivateRetainedNotificationLeaseTerminalClosesAdmissionAndDrainsCallback(t *testing.T) {
	a := mustPrivateB4ByteAccountV2(t)
	owner, failure := newPrivateB4RetainedNotificationOwner(a, notificationFixture())
	if failure != "" {
		t.Fatal(failure)
	}
	lease := RetainedNotificationLease{owner: owner}
	entered := make(chan struct{})
	leave := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- lease.WithNotifications(func([]lspwire.Message) error { close(entered); <-leave; return nil })
	}()
	<-entered
	owner.requestTerminal()
	if err := lease.WithNotifications(func([]lspwire.Message) error { return nil }); !errors.Is(err, ErrRetainedNotificationLeaseClosed) {
		t.Fatalf("ASSERT_C15_NOTIFICATION_TERMINAL_ADMISSION err=%v", err)
	}
	released := make(chan bool, 1)
	go func() { released <- lease.Release() }()
	select {
	case <-released:
		t.Fatal("ASSERT_C15_NOTIFICATION_TERMINAL_PREMATURE_RELEASE")
	default:
	}
	close(leave)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !<-released {
		t.Fatal("ASSERT_C15_NOTIFICATION_TERMINAL_RELEASE")
	}
	a.requestTerminalRelease()
	if a.snapshot().Live != 0 {
		t.Fatal("ASSERT_C15_NOTIFICATION_TERMINAL_QUIESCENCE")
	}
}

func TestPrivateRetainedNotificationLeaseReserveBoundaries(t *testing.T) {
	source := notificationFixture()
	want, _ := retainedNotificationCapacity(source)
	a := mustPrivateB4ByteAccountV2(t)
	filler, failure := a.reserve(privateB4MaxOwnedBytes - a.snapshot().Live - want)
	if failure != "" {
		t.Fatal(failure)
	}
	owner, failure := newPrivateB4RetainedNotificationOwner(a, source)
	if failure != "" || owner == nil || a.snapshot().Live != privateB4MaxOwnedBytes {
		t.Fatalf("ASSERT_C15_NOTIFICATION_EQUALITY failure=%q snapshot=%+v", failure, a.snapshot())
	}
	(RetainedNotificationLease{owner: owner}).Release()
	filler.release()
	a.requestTerminalRelease()

	b := mustPrivateB4ByteAccountV2(t)
	fill, failure := b.reserve(privateB4MaxOwnedBytes - b.snapshot().Live - want + 1)
	if failure != "" {
		t.Fatal(failure)
	}
	before := b.snapshot()
	got, failure := newPrivateB4RetainedNotificationOwner(b, source)
	if failure == "" || got != nil || b.snapshot() != before {
		t.Fatalf("ASSERT_C15_NOTIFICATION_PLUS_ONE failure=%q owner=%v before=%+v after=%+v", failure, got, before, b.snapshot())
	}
	fill.release()
	b.requestTerminalRelease()
}

func TestPrivateRetainedNotificationLeaseProductionPlusOneRefusesAtomically(t *testing.T) {
	f := newFullP1FixtureWithNotifications(t, 1)
	f.req.MaxMessages = 2
	required, _ := retainedNotificationCapacity(notificationFixture())
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
	if result.Failure != session.ResourceExhausted || managerLease != (B4DefinitionLease{}) || len(result.Notifications) != 0 {
		t.Fatalf("ASSERT_C15_NOTIFICATION_PRODUCTION_PLUS_ONE failure=%q manager=%v legacy=%d", result.Failure, managerLease != (B4DefinitionLease{}), len(result.Notifications))
	}
	after := account.snapshot()
	if _, ok := result.PrivateRetainedNotificationLease(); ok || after.Live != before.Live {
		t.Fatalf("ASSERT_C15_NOTIFICATION_PRODUCTION_PLUS_ONE_PUBLICATION lease=%t before=%+v after=%+v", ok, before, after)
	}
	filler.release()
	if account.snapshot().Live != 0 {
		t.Fatalf("ASSERT_C15_NOTIFICATION_PRODUCTION_PLUS_ONE_CLEANUP %+v", account.snapshot())
	}
}

func TestOrdinaryRoundTripRetainedNotificationCompatibility(t *testing.T) {
	f := newFullP1FixtureWithNotifications(t, 1)
	f.req.MaxMessages = 2
	result := f.m.RoundTrip(context.Background(), f.req)
	if result.Failure != "" || len(result.Notifications) != 1 || result.Notifications[0].Method != "$/progress" {
		t.Fatalf("ASSERT_C15_NOTIFICATION_ORDINARY_COMPAT failure=%q notifications=%d", result.Failure, len(result.Notifications))
	}
	if _, ok := result.PrivateRetainedNotificationLease(); ok {
		t.Fatal("ASSERT_C15_NOTIFICATION_ORDINARY_LEASE")
	}
}

func TestPrivateRetainedNotificationLeaseProductionPublication(t *testing.T) {
	f := newFullP1FixtureWithNotifications(t, 1)
	f.req.MaxMessages = 2
	var account *privateB4ByteAccountV2
	f.m.b4ParamsHook = func(a *privateB4ByteAccountV2) { account = a }
	result, managerLease, selection := f.transact(t)
	if len(result.Notifications) != 0 {
		t.Fatalf("ASSERT_C15_NOTIFICATION_LEGACY_PUBLICATION count=%d", len(result.Notifications))
	}
	lease, ok := result.PrivateRetainedNotificationLease()
	if !ok {
		t.Fatal("ASSERT_C15_NOTIFICATION_PRODUCTION_LEASE")
	}
	if err := lease.WithNotifications(func(messages []lspwire.Message) error {
		if len(messages) != 1 || messages[0].Method != "$/progress" {
			return errors.New("wrong production notifications")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	requestLease, _ := result.PrivateB4RequestFrameLease()
	responseLease, _ := result.PrivateB4ResponseFrameLease()
	resultLease, _ := result.PrivateB4ResultLease()
	if _, status := consumePrivateB4SnapshotForTest(f.m, managerLease, selection); status != PrivateB4Selected {
		t.Fatalf("ASSERT_C15_NOTIFICATION_CONSUME status=%s", status)
	}
	if !requestLease.Release() || !responseLease.Release() || !resultLease.Release() || account.snapshot().Live == 0 {
		t.Fatalf("ASSERT_C15_NOTIFICATION_RETAINED %+v", account.snapshot())
	}
	if !lease.Release() || account.snapshot().Live != 0 {
		t.Fatalf("ASSERT_C15_NOTIFICATION_RELEASE %+v", account.snapshot())
	}
}

func TestPrivateRetainedNotificationLeaseZeroNotificationNoLease(t *testing.T) {
	f := newFullP1Fixture(t)
	result, managerLease, selection := f.transact(t)
	if len(result.Notifications) != 0 {
		t.Fatal("ASSERT_C15_NOTIFICATION_ZERO_LEGACY")
	}
	if _, ok := result.PrivateRetainedNotificationLease(); ok {
		t.Fatal("ASSERT_C15_NOTIFICATION_ZERO_LEASE")
	}
	requestLease, _ := result.PrivateB4RequestFrameLease()
	responseLease, _ := result.PrivateB4ResponseFrameLease()
	resultLease, _ := result.PrivateB4ResultLease()
	if _, status := consumePrivateB4SnapshotForTest(f.m, managerLease, selection); status != PrivateB4Selected {
		t.Fatal("ASSERT_C15_NOTIFICATION_ZERO_CONSUME")
	}
	if !requestLease.Release() || !responseLease.Release() || !resultLease.Release() {
		t.Fatal("ASSERT_C15_NOTIFICATION_ZERO_CALLER_RELEASE")
	}
}
