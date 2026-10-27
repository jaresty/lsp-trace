//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

func TestScopeAdapterNativeWireIntersection(t *testing.T) {
	now := time.Unix(1700000000, 0)
	maxInt := int(^uint(0) >> 1)
	for _, tc := range []struct {
		name                                                          string
		messages, bytes, remaining, envelope, wantMessages, wantBytes int
		invalid                                                       bool
	}{
		{"native-default", 64, scopeAdapterResponse, scopeAdapterCapture, 0, 64, scopeAdapterResponse, false},
		{"lower-messages", 1, 1024, scopeAdapterCapture, 0, 1, 1024, false},
		{"equal-private", 64, scopeAdapterWireBytes, scopeAdapterCapture, 0, 64, scopeAdapterWireBytes, false},
		{"oversized-tightened", maxInt, maxInt, scopeAdapterCapture, 0, 64, scopeAdapterWireBytes, false},
		{"aggregate-wire-remainder", 64, scopeAdapterWireBytes, 1024, 0, 64, 1024, false},
		{"minimum-body", 1, scopeMinimumResponseBody, scopeAdapterCapture, 0, 1, scopeMinimumResponseBody, false},
		{"native-no-added-envelope", 2, 1024, 1024, 0, 2, 1024, false},
		{"direct-fixed-envelope", 64, scopeAdapterWireBytes, 1024, scopeAdapterResponseEnvelope, 64, 2048, false},
		{"zero-messages", 0, 1024, scopeAdapterCapture, 0, 0, 0, true},
		{"negative-messages", -1, 1024, scopeAdapterCapture, 0, 0, 0, true},
		{"zero-bytes", 1, 0, scopeAdapterCapture, 0, 0, 0, true},
		{"negative-bytes", 1, -1, scopeAdapterCapture, 0, 0, 0, true},
		{"unrepresentable-one-byte", 1, 1, scopeAdapterCapture, 0, 0, 0, true},
		{"below-minimum-body", 1, scopeMinimumResponseBody - 1, scopeAdapterCapture, 0, 0, 0, true},
		{"empty-aggregate", 1, 1024, 0, 0, 0, 0, true},
		{"small-aggregate", 1, 1024, 1, 0, 0, 0, true},
		{"invalid-aggregate", 1, 1024, scopeAdapterCapture + 1, 0, 0, 0, true},
		{"invalid-envelope", 1, 1024, 1024, 1, 0, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := scopeIntersectWireBounds(now, now.Add(time.Minute), time.Time{}, tc.remaining, tc.messages, tc.bytes, tc.envelope)
			if tc.invalid {
				if !errors.Is(err, ErrInvalid) {
					t.Fatal("invalid bounds accepted", got, err)
				}
				return
			}
			if err != nil || got.messages != tc.wantMessages || got.bytes != tc.wantBytes || got.deadline != now.Add(scopeAdapterRequestTimeout) {
				t.Fatal(got, err)
			}
			if tc.envelope == 0 && got.bytes > tc.bytes {
				t.Fatal("native wire ceiling inflated")
			}
		})
	}
	for _, tc := range []struct {
		name                   string
		lifetime, caller, want time.Duration
	}{
		{"private-request-timeout", time.Minute, 0, scopeAdapterRequestTimeout},
		{"caller-shorter", time.Minute, time.Second, time.Second},
		{"lifetime-shorter", time.Second, 2 * time.Second, time.Second},
		{"caller-cannot-widen", time.Second, time.Hour, time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			caller := time.Time{}
			if tc.caller != 0 {
				caller = now.Add(tc.caller)
			}
			got, err := scopeIntersectWireBounds(now, now.Add(tc.lifetime), caller, scopeAdapterCapture, 64, 1024, 0)
			if err != nil || got.deadline != now.Add(tc.want) {
				t.Fatal("deadline intersection", got, err)
			}
		})
	}
	if _, err := scopeIntersectWireBounds(now, now, time.Time{}, scopeAdapterCapture, 64, 1024, 0); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("expired lifetime", err)
	}
}

func TestScopeAdapterNativeWireInvalidPretransport(t *testing.T) {
	for _, tc := range []struct {
		name            string
		messages, bytes int
	}{{"zero-messages", 0, 1024}, {"negative-messages", -1, 1024}, {"zero-bytes", 1, 0}, {"negative-bytes", 1, -1}, {"one-byte", 1, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			a, child := reviewAdapter(t, func(lspwire.Message) json.RawMessage { return json.RawMessage(`[]`) })
			p, _ := json.Marshal(lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: a.files[0].URI}})
			r := acquisition.WireRequest{Context: reviewNativeRequest(a).Context, Method: "textDocument/documentSymbol", Params: p, MaxMessages: tc.messages, MaxBytes: tc.bytes}
			before := child.stdoutBytes.Load()
			err := a.invoke(func() error { _, e := a.nativeWireRoundTripLocked(context.Background(), r); return e })
			if !errors.Is(err, ErrInvalid) || len(semanticMessages(child)) != 0 || child.stdoutBytes.Load() != before || a.requests != 0 || a.terminal != nil {
				t.Fatal("invalid native bounds reached transport or latched without consumption", err)
			}
			if _, err = a.DocumentSymbols(context.Background(), lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: a.files[0].URI}}); err != nil || len(semanticMessages(child)) != 1 {
				t.Fatal("harmless prewrite refusal inflated/lost fixed direct defaults", err)
			}
		})
	}
}

func TestScopeAdapterNativeWirePrivateCeilings(t *testing.T) {
	for _, kind := range []string{"messages", "bytes", "aggregate"} {
		t.Run(kind, func(t *testing.T) {
			a, child := reviewAdapter(t, nil)
			raw := json.RawMessage("[" + reviewItem(a.files[0].URI) + "]")
			messages, wireBytes := 4096, 16<<20
			if kind == "messages" {
				child.mu.Lock()
				child.notifications = 65
				child.mu.Unlock()
			}
			if kind == "bytes" {
				raw = reviewSizedItem(a.files[0].URI, scopeAdapterWireBytes+1)
			}
			if kind == "aggregate" {
				a.captured = scopeAdapterCapture - len(raw)
			}
			child.mu.Lock()
			child.response = func(lspwire.Message) json.RawMessage { return raw }
			child.mu.Unlock()
			p, _ := json.Marshal(lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: a.files[0].URI}})
			r := acquisition.WireRequest{Context: reviewNativeRequest(a).Context, Method: "textDocument/prepareCallHierarchy", Params: p, MaxMessages: messages, MaxBytes: wireBytes}
			err := a.invoke(func() error { _, e := a.nativeWireRoundTripLocked(context.Background(), r); return e })
			var failure scopeRuntimeFailurePrivate
			if !errors.As(err, &failure) || failure.Failure != session.ResourceExhausted || len(semanticMessages(child)) != 1 {
				t.Fatal("private/native/remaining wire intersection not Manager-enforced", kind, err)
			}
			n := len(semanticMessages(child))
			if _, err = a.DocumentSymbols(context.Background(), lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: a.files[0].URI}}); err == nil || len(semanticMessages(child)) != n {
				t.Fatal("budget failure retried")
			}
		})
	}
}

func TestScopeAdapterNativeWireCanceledDeadline(t *testing.T) {
	a, child := reviewAdapter(t, func(lspwire.Message) json.RawMessage { return json.RawMessage(`[]`) })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := a.wireBoundsLocked(ctx, 1, 1024, 0)
	if !errors.Is(err, context.Canceled) || len(semanticMessages(child)) != 0 {
		t.Fatal("canceled native request widened", err)
	}
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	bounds, err := a.wireBoundsLocked(ctx, 1, 1024, 0)
	deadline, _ := ctx.Deadline()
	if err != nil || bounds.deadline.After(deadline) || bounds.deadline.After(a.deadline) {
		t.Fatal("native context deadline widened", bounds, err)
	}
}
