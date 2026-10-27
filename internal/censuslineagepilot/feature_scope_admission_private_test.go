//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
)

func reviewSizedItem(uri string, size int) json.RawMessage {
	base := "[" + strings.Replace(reviewItem(uri), `"kind":12`, `"kind":12,"data":""`, 1) + "]"
	return json.RawMessage(strings.Replace(base, `"data":""`, `"data":"`+strings.Repeat("x", size-len(base))+`"`, 1))
}
func TestScopeAdapterReviewResponseBoundary(t *testing.T) {
	for _, size := range []int{scopeAdapterResponse, scopeAdapterResponse + 1} {
		t.Run(string(rune(size-scopeAdapterResponse+'0')), func(t *testing.T) {
			a, child := reviewAdapter(t, nil)
			raw := reviewSizedItem(a.files[0].URI, size)
			if len(raw) != size {
				t.Fatal("fixture size")
			}
			child.mu.Lock()
			child.response = func(lspwire.Message) json.RawMessage { return raw }
			child.mu.Unlock()
			items, err := a.PrepareCallHierarchy(context.Background(), lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: a.files[0].URI}})
			if size == scopeAdapterResponse {
				if err != nil || len(items) != 1 || len(items[0].Data) == 0 {
					t.Fatal("exact 2MiB result rejected", err)
				}
			} else if err == nil {
				t.Fatal("over 2MiB accepted")
			}
			if len(semanticMessages(child)) != 1 {
				t.Fatal("expected exactly one response")
			}
			child.mu.Lock()
			child.response = func(lspwire.Message) json.RawMessage { return json.RawMessage(`[]`) }
			child.mu.Unlock()
			_, err = a.DocumentSymbols(context.Background(), lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: a.files[0].URI}})
			if size > scopeAdapterResponse {
				if err == nil || len(semanticMessages(child)) != 1 {
					t.Fatal("oversize did not latch")
				}
			} else if err != nil || len(semanticMessages(child)) != 2 {
				t.Fatal("valid boundary latched", err)
			}
		})
	}
}
func TestScopeAdapterReviewNullAndEmptyCalls(t *testing.T) {
	for _, raw := range []string{"null", "[]"} {
		t.Run(raw, func(t *testing.T) {
			a, child := reviewAdapter(t, func(lspwire.Message) json.RawMessage { return json.RawMessage(raw) })
			for _, incoming := range []bool{true, false} {
				var n bool
				var err error
				if incoming {
					var rows []lsp.CallHierarchyIncomingCall
					rows, n, err = a.IncomingCalls(context.Background(), scopeAdapterItem(a.files[0].URI))
					if (rows == nil) != (raw == "null") {
						t.Fatal("incoming empty/null lost")
					}
				} else {
					var rows []lsp.CallHierarchyOutgoingCall
					rows, n, err = a.OutgoingCalls(context.Background(), scopeAdapterItem(a.files[0].URI))
					if (rows == nil) != (raw == "null") {
						t.Fatal("outgoing empty/null lost")
					}
				}
				if err != nil || n != (raw == "null") {
					t.Fatal(n, err)
				}
			}
			if len(semanticMessages(child)) != 2 {
				t.Fatal("empty/null should remain usable")
			}
		})
	}
}

func TestScopeAdapterReviewConcurrentAdmission(t *testing.T) {
	a, child := reviewAdapter(t, nil)
	raw := reviewSizedItem(a.files[0].URI, 1<<20)
	child.mu.Lock()
	child.response = func(lspwire.Message) json.RawMessage { return raw }
	child.mu.Unlock()
	// Enough aggregate capacity for exactly one response plus a small remainder.
	a.captured = scopeAdapterCapture - len(raw) - 4096
	firstConsumed := make(chan struct{})
	secondConsumed := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseFirst := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseFirst()
	var count atomic.Int32
	a.afterResponsePrivateTest = func() {
		if count.Add(1) == 1 {
			close(firstConsumed)
			<-release
		} else {
			close(secondConsumed)
		}
	}
	secondAtAdmission := make(chan struct{})
	var attempts atomic.Int32
	a.beforeInvokePrivateTest = func() {
		if attempts.Add(1) == 2 {
			close(secondAtAdmission)
		}
	}
	p := lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: a.files[0].URI}}
	firstDone := make(chan error, 1)
	secondDone := make(chan error, 1)
	go func() { _, err := a.PrepareCallHierarchy(context.Background(), p); firstDone <- err }()
	select {
	case <-firstConsumed:
	case <-time.After(2 * time.Second):
		t.Fatal("first response not consumed")
	}
	go func() {
		_, err := a.PrepareCallHierarchy(context.Background(), p)
		secondDone <- err
	}()
	// Positive handshake at invoke's actual lock-attempt seam, not merely at
	// goroutine startup. Only positive-progress watchdogs use wall-clock time.
	select {
	case <-secondAtAdmission:
	case <-time.After(2 * time.Second):
		t.Fatal("second invocation never reached admission")
	}
	// The first invocation is still paused after Manager ownership ends and before
	// accounting/validation. Lock ownership—not a short negative sleep—excludes
	// the second invocation from transport through complete response admission.
	if a.invocationMu.TryLock() {
		a.invocationMu.Unlock()
		t.Error("response admission lock not held")
	}
	select {
	case <-secondConsumed:
		t.Error("second response consumed using unreserved allowance")
	default:
	}
	if n := len(semanticMessages(child)); n != 1 {
		t.Errorf("semantic requests while first admission pending: %d", n)
	}
	releaseFirst()
	if err := <-firstDone; err != nil {
		t.Error("first admitted response", err)
	}
	if err := <-secondDone; err == nil {
		t.Error("second response exceeded remaining allowance")
	}
	a.mu.Lock()
	captured := a.captured
	terminal := a.terminal
	a.mu.Unlock()
	if captured != scopeAdapterCapture-4096 || captured > scopeAdapterCapture || terminal == nil {
		t.Fatal("admission accounting/terminal", captured, terminal)
	}
	n := len(semanticMessages(child))
	if _, err := a.PrepareCallHierarchy(context.Background(), p); err == nil || len(semanticMessages(child)) != n {
		t.Fatal("post-error allowance reused")
	}
}
