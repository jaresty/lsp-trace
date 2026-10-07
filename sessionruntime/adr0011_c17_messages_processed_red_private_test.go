package sessionruntime

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"lsp-trace/internal/adr0011work"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

func c17MessagesProcessedContext(t *testing.T, transaction string) (context.Context, *adr0011work.Account) {
	t.Helper()
	account := adr0011work.New(transaction)
	if account == nil {
		t.Fatal("BLOCKED_NOT_RED: private work account unavailable")
	}
	ctx := adr0011work.WithAccount(context.Background(), account)
	borrowed, ok := adr0011work.AccountFromContext(ctx)
	if !ok || borrowed != account {
		t.Fatal("BLOCKED_NOT_RED: context did not preserve exact account identity")
	}
	return ctx, account
}

func c17AssertExactAccount(t *testing.T, ctx context.Context, want *adr0011work.Account) {
	t.Helper()
	got, ok := adr0011work.AccountFromContext(ctx)
	if !ok || got != want {
		t.Fatalf("ASSERT_C17_MESSAGES_PROCESSED_ACCOUNT_IDENTITY: got=%p want=%p ok=%v", got, want, ok)
	}
}

func TestADR0011C17MessagesProcessedCausalBoundary(t *testing.T) {
	t.Run("mixed classification counts once per decoded message", func(t *testing.T) {
		ctx, account := c17MessagesProcessedContext(t, "messages-mixed")
		m, started, _ := roundTripManager(t, "success")
		t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
		result := m.RoundTrip(ctx, RoundTripRequest{
			SessionID: started.SessionID, Generation: started.Generation, Method: "test/method",
			Params: json.RawMessage(`{"x":1}`), Deadline: time.Now().Add(time.Second), MaxMessages: 3, MaxBytes: 4096,
		})
		if result.Failure != "" || result.Messages != 3 || len(result.Notifications) != 1 || len(result.Responses) != 1 || string(result.Result) != `{"ok":true}` {
			t.Fatalf("BLOCKED_NOT_RED: mixed Manager fixture did not reach notification, unmatched response, and matched response: %+v", result)
		}
		c17AssertExactAccount(t, ctx, account)
		if got := account.Snapshot().MessagesProcessed; got != 3 {
			t.Fatalf("ASSERT_C17_MESSAGES_PROCESSED_MIXED_EXACTLY_ONCE: got=%d want=3", got)
		}
	})

	t.Run("server request and matched response each count once", func(t *testing.T) {
		request := lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(`"server-request"`), Method: "workspace/unknown", Params: json.RawMessage(`{"x":1}`)}
		f := newFullP1FixtureWithResponses(t, []lspwire.Message{request})
		ctx, account := c17MessagesProcessedContext(t, f.owner.Transaction)
		f.req.MaxMessages = 2
		result, _ := f.m.RoundTripPrivateB4(ctx, f.req, f.owner)
		if result.Failure != "" || result.Messages != 2 {
			t.Fatalf("BLOCKED_NOT_RED: private server-request fixture did not reach request and matched response: %+v", result)
		}
		f.child.mu.Lock()
		replies := append([]lspwire.Message(nil), f.child.serverReplies...)
		f.child.mu.Unlock()
		if len(replies) != 1 || replies[0].Error == nil || replies[0].Error.Code != -32601 {
			t.Fatalf("BLOCKED_NOT_RED: server request was not semantically classified: %+v", replies)
		}
		c17AssertExactAccount(t, ctx, account)
		if got := account.Snapshot().MessagesProcessed; got != 2 {
			t.Fatalf("ASSERT_C17_MESSAGES_PROCESSED_SERVER_REQUEST_NO_DOUBLE_COUNT: got=%d want=2", got)
		}
	})

	t.Run("decoded message rejected later remains counted", func(t *testing.T) {
		f := newC16ActivationFixture(t)
		ctx, account := c17MessagesProcessedContext(t, f.owner.Transaction)
		f.req.CaptureDefinitionResponseFrameMaxBytes = 1
		result, lease := f.m.RoundTripPrivateB4(ctx, f.req, f.owner)
		if result.Failure != "" || result.Key.ID == 0 || result.Messages != 1 || lease != (B4DefinitionLease{}) {
			t.Fatalf("BLOCKED_NOT_RED: decoded selected response was not rejected only by later frame retention: result=%+v lease=%v", result, lease != (B4DefinitionLease{}))
		}
		if _, ok := result.CompletedResponseRead(); !ok {
			t.Fatal("BLOCKED_NOT_RED: later-rejection fixture did not complete decoded response read")
		}
		c17AssertExactAccount(t, ctx, account)
		if got := account.Snapshot().MessagesProcessed; got != 1 {
			t.Fatalf("ASSERT_C17_MESSAGES_PROCESSED_NO_REFUND_AFTER_LATER_REJECTION: got=%d want=1", got)
		}
	})

	t.Run("decode failure counts zero", func(t *testing.T) {
		ctx, account := c17MessagesProcessedContext(t, "messages-decode-failure")
		m, started, _ := roundTripManager(t, "malformed")
		t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
		result := m.RoundTrip(ctx, RoundTripRequest{SessionID: started.SessionID, Generation: started.Generation, Method: "test/method", Deadline: time.Now().Add(time.Second), MaxMessages: 1, MaxBytes: 1024})
		if result.Failure != session.SessionPoisoned || result.Messages != 0 {
			t.Fatalf("BLOCKED_NOT_RED: malformed fixture did not fail before decode: %+v", result)
		}
		c17AssertExactAccount(t, ctx, account)
		if got := account.Snapshot().MessagesProcessed; got != 0 {
			t.Fatalf("ASSERT_C17_MESSAGES_PROCESSED_DECODE_FAILURE_ZERO: got=%d want=0", got)
		}
	})
}

func TestADR0011C17MessagesProcessedCheckedOverflowBoundary(t *testing.T) {
	account := adr0011work.New("messages-overflow")
	if err := account.ChargeMessagesProcessed("messages-overflow", math.MaxUint64); err != nil {
		t.Fatalf("ASSERT_MESSAGES_PROCESSED_MAX_ACCEPTED: %v", err)
	}
	before := account.Snapshot()
	if err := account.ChargeMessagesProcessed("messages-overflow", 1); !errors.Is(err, adr0011work.ErrOverflow) {
		t.Fatalf("ASSERT_MESSAGES_PROCESSED_PLUS_ONE_OVERFLOW: %v", err)
	}
	after := account.Snapshot()
	if after.MessagesProcessed != before.MessagesProcessed || !after.Terminal || !errors.Is(after.Failure, adr0011work.ErrOverflow) {
		t.Fatalf("ASSERT_MESSAGES_PROCESSED_OVERFLOW_ZERO_EFFECT before=%+v after=%+v", before, after)
	}
}

func TestADR0011C17MessagesProcessedDefaultOffPreservesPrivateResult(t *testing.T) {
	plain := newC16ActivationFixture(t)
	plainResult, plainLease := plain.m.RoundTripPrivateB4(context.Background(), plain.req, plain.owner)
	if account, ok := adr0011work.AccountFromContext(context.Background()); ok || account != nil {
		t.Fatalf("ASSERT_C17_MESSAGES_PROCESSED_DEFAULT_OFF_NO_ACCOUNT: account=%p ok=%v", account, ok)
	}

	accounted := newC16ActivationFixture(t)
	ctx, account := c17MessagesProcessedContext(t, accounted.owner.Transaction)
	accountedResult, accountedLease := accounted.m.RoundTripPrivateB4(ctx, accounted.req, accounted.owner)
	if !reflect.DeepEqual(c18MaterialResult(plainResult), c18MaterialResult(accountedResult)) ||
		(plainLease == (B4DefinitionLease{})) != (accountedLease == (B4DefinitionLease{})) {
		t.Fatalf("ASSERT_C17_MESSAGES_PROCESSED_DEFAULT_OFF_RESULT_PARITY: plain=%+v accounted=%+v lease=%v/%v", c18MaterialResult(plainResult), c18MaterialResult(accountedResult), plainLease != (B4DefinitionLease{}), accountedLease != (B4DefinitionLease{}))
	}
	c17AssertExactAccount(t, ctx, account)
	if got := account.Snapshot().MessagesProcessed; got != 1 {
		t.Fatalf("ASSERT_C17_MESSAGES_PROCESSED_DEFAULT_OFF_ACCOUNTED_COUNT: got=%d want=1", got)
	}
}
