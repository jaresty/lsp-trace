//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

func scopeFixtureResponseBody(raw json.RawMessage) int {
	body, _ := json.Marshal(lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage("1"), Result: raw})
	return len(body)
}
func scopeFixtureResponseFrame(raw json.RawMessage) int {
	var b bytes.Buffer
	_ = lspwire.NewWriter(&b, lspwire.DefaultLimits()).Write(lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage("1"), Result: raw})
	return b.Len()
}
func scopeFixtureNotificationFrame() int {
	var b bytes.Buffer
	_ = lspwire.NewWriter(&b, lspwire.DefaultLimits()).Write(lspwire.Message{JSONRPC: lspwire.Version, Method: "window/logMessage", Params: json.RawMessage(`{"type":3,"message":"private fixture"}`)})
	return b.Len()
}
func TestScopeAdapterNativeWireMessageLimits(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		messages, notifications int
		fail                    bool
	}{
		{"one-notification-one-response-at-one", 1, 1, false},
		{"two-notifications-at-one", 1, 2, true},
		{"two-notifications-at-two", 2, 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, child := reviewAdapter(t, nil)
			raw := json.RawMessage("[" + reviewItem(a.files[0].URI) + "]")
			child.mu.Lock()
			child.notifications = tc.notifications
			child.response = func(lspwire.Message) json.RawMessage { return raw }
			child.mu.Unlock()
			r := reviewNativeRequest(a)
			r.Root.UpDepth = 0
			r.Limits.MaxMessages = tc.messages
			readBefore := child.stdoutBytes.Load()
			out, err := acquisition.Acquire(context.Background(), a.nativeClient(), r)
			if err != nil {
				t.Fatal(err)
			}
			if len(semanticMessages(child)) != 1 {
				t.Fatal("unexpected semantic attempts")
			}
			a.mu.Lock()
			terminal := a.terminal
			a.mu.Unlock()
			if tc.fail {
				var authoritative scopeRuntimeFailurePrivate
				if !errors.As(terminal, &authoritative) || authoritative.Failure != session.ResourceExhausted || !strings.Contains(terminal.Error(), string(session.ResourceExhausted)) {
					t.Fatalf("native per-class limit discarded: terminal=%v resolution=%+v", terminal, out.Targets[0].Resolution)
				}
				if _, err = a.DocumentSymbols(context.Background(), lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: a.files[0].URI}}); err == nil || len(semanticMessages(child)) != 1 {
					t.Fatal("consumed message limit failure did not latch")
				}
			} else if terminal != nil || out.Targets[0].Resolution.Status != acquisition.Resolved {
				t.Fatal("valid Manager per-class limit", terminal, out.Targets[0].Resolution)
			}
			records := a.manager.Records()
			if len(records) != 1 || records[0].Generation != a.identity.Generation || a.manager.Census().Requests != 0 {
				t.Fatal("generation changed/retried or pending request leaked")
			}
			// Instrument actual pipe reads; this is not an asserted hard MaxBytes read
			// bound. Manager checks ordinary cumulative body budgets on complete frames.
			consumed := child.stdoutBytes.Load() - readBefore
			if consumed <= 0 || consumed > int64(tc.notifications*scopeFixtureNotificationFrame()+scopeFixtureResponseFrame(raw)) {
				t.Fatal("unexpected consumed fake-wire frame bound", consumed)
			}
		})
	}
}
func TestScopeAdapterNativeWireByteLimits(t *testing.T) {
	for _, kind := range []string{"payload-only-cap", "exact-wire-body", "enough-wire-body"} {
		t.Run(kind, func(t *testing.T) {
			a, child := reviewAdapter(t, nil)
			raw := json.RawMessage("[" + reviewItem(a.files[0].URI) + "]")
			body := scopeFixtureResponseBody(raw)
			child.mu.Lock()
			child.response = func(msg lspwire.Message) json.RawMessage {
				actual, _ := json.Marshal(lspwire.Message{JSONRPC: lspwire.Version, ID: msg.ID, Result: raw})
				if len(actual) != body {
					panic("fixture request ID length changed")
				}
				return raw
			}
			child.mu.Unlock()
			r := reviewNativeRequest(a)
			r.Root.UpDepth = 0
			r.Limits.MaxResponseBytes = body
			if kind == "payload-only-cap" {
				r.Limits.MaxResponseBytes = len(raw)
			}
			if kind == "enough-wire-body" {
				r.Limits.MaxResponseBytes = body + 128
			}
			readBefore := child.stdoutBytes.Load()
			out, err := acquisition.Acquire(context.Background(), a.nativeClient(), r)
			if err != nil {
				t.Fatal(err)
			}
			if len(semanticMessages(child)) != 1 {
				t.Fatal("unexpected semantic attempts")
			}
			a.mu.Lock()
			terminal := a.terminal
			a.mu.Unlock()
			if kind == "payload-only-cap" {
				var authoritative scopeRuntimeFailurePrivate
				if !errors.As(terminal, &authoritative) || authoritative.Failure != session.ResourceExhausted || !strings.Contains(terminal.Error(), string(session.ResourceExhausted)) {
					t.Fatalf("native WIRE bytes treated as result-only bytes: terminal=%v resolution=%+v", terminal, out.Targets[0].Resolution)
				}
				if _, err = a.DocumentSymbols(context.Background(), lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: a.files[0].URI}}); err == nil || len(semanticMessages(child)) != 1 {
					t.Fatal("consumed byte limit failure did not latch")
				}
			} else if terminal != nil || out.Targets[0].Resolution.Status != acquisition.Resolved {
				t.Fatal("valid wire body ceiling refused", terminal, out.Targets[0].Resolution)
			}
			records := a.manager.Records()
			if len(records) != 1 || records[0].Generation != a.identity.Generation || a.manager.Census().Requests != 0 {
				t.Fatal("generation changed/retried or pending request leaked")
			}
			read := child.stdoutBytes.Load() - readBefore
			if read <= 0 || read > int64(scopeFixtureResponseFrame(raw)) {
				t.Fatal("unexpected actual consumed frame bound", read)
			}
		})
	}
}
