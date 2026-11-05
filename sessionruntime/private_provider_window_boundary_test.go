package sessionruntime

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

func boundarySuffix(t *testing.T, m *Manager, st StartResult, start StartRequest, ctx context.Context, uri string) {
	t.Helper()
	ok, c, err := repairReadinessSuffix(m, st, start, ctx, uri)
	if !ok || err != nil || !c.Completed {
		t.Errorf("ASSERT_BOUNDARY_OWNED_SUFFIX: suffix=%t completed=%t err=%v", ok, c.Completed, err)
	}
}

func TestPrivateProviderWindowBoundaryLifecycle(t *testing.T) {
	for _, same := range []bool{false, true} {
		for _, method := range []string{"stop", "restart", "shutdown-response"} {
			t.Run(fmt.Sprintf("same=%t/%s", same, method), func(t *testing.T) {
				m, start, ctx, s := windowTestSetup(t, PrivateProviderWindowPlan{Definitions: true}, "")
				st := windowTestReady(t, m, start, ctx)
				b, bs, child, writes := repairOrdinary(t, m, start, same)
				before := writes.Load()
				fingerprint := repairReadinessFingerprint(privateProviderWindowFrom(ctx))
				b.mu.Lock()
				operations, workers := len(b.operations), b.workers
				r := b.sessions[bs.SessionID]
				b.mu.Unlock()
				var got session.LifecycleResult
				switch method {
				case "stop":
					got = b.Stop(ctx, bs.SessionID, "boundary")
				case "restart":
					got = b.Restart(ctx, bs.SessionID, "boundary")
				default:
					got = b.terminateWithShutdownResponsePrivateV2(ctx, bs.SessionID, "boundary", false, nil)
				}
				b.mu.Lock()
				unchanged := b.sessions[bs.SessionID] == r && r.record.Generation == bs.Generation && len(b.operations) == operations && b.workers == workers
				b.mu.Unlock()
				select {
				case <-child.closed:
					unchanged = false
				default:
				}
				if got.Failure == "" || got.IntentID != "" || !unchanged || writes.Load() != before || fingerprint != repairReadinessFingerprint(privateProviderWindowFrom(ctx)) {
					t.Errorf("ASSERT_BOUNDARY_FOREIGN_LIFECYCLE: result=%+v unchanged=%t writes=%d", got, unchanged, writes.Load()-before)
				} else {
					t.Log("ASSERT_BOUNDARY_FOREIGN_LIFECYCLE: PASS")
				}
				boundarySuffix(t, m, st, start, ctx, s.child.uri)
			})
		}
	}
}

func TestPrivateProviderWindowBoundaryCachedLifecycle(t *testing.T) {
	for _, same := range []bool{false, true} {
		t.Run(fmt.Sprintf("same=%t", same), func(t *testing.T) {
			m, start, ctx, s := windowTestSetup(t, PrivateProviderWindowPlan{Definitions: true}, "")
			st := windowTestReady(t, m, start, ctx)
			b, bs, _, _ := repairOrdinary(t, m, start, same)
			actual := b.Stop(context.Background(), bs.SessionID, "cached-boundary")
			if actual.Failure != "" || actual.IntentID == "" || !b.waitLifecycleTerminal(ctx, actual.IntentID) {
				t.Fatal("ordinary Stop did not join")
			}
			b.mu.Lock()
			gone := b.sessions[bs.SessionID] == nil
			b.mu.Unlock()
			if !gone {
				t.Fatal("ordinary stopped record retained unexpectedly")
			}
			got := b.Stop(ctx, bs.SessionID, "cached-boundary")
			if got.Failure == "" || got.IntentID != "" || got.Replayed {
				t.Errorf("ASSERT_BOUNDARY_CACHED_LIFECYCLE: result=%+v", got)
			} else {
				t.Log("ASSERT_BOUNDARY_CACHED_LIFECYCLE: PASS")
			}
			boundarySuffix(t, m, st, start, ctx, s.child.uri)
		})
	}
}

func TestPrivateProviderWindowBoundaryPrivatePeer(t *testing.T) {
	m, start, ctx, s := windowTestSetup(t, PrivateProviderWindowPlan{Definitions: true}, "")
	st := windowTestReady(t, m, start, ctx)
	b, bst, bctx, bs := windowTestSetup(t, PrivateProviderWindowPlan{Definitions: true}, "")
	bt := windowTestReady(t, b, bst, bctx)
	aBefore, bBefore := repairReadinessFingerprint(privateProviderWindowFrom(ctx)), repairReadinessFingerprint(privateProviderWindowFrom(bctx))
	got := b.Stop(ctx, bt.SessionID, "foreign-private")
	if got.Failure == "" || got.IntentID != "" || aBefore != repairReadinessFingerprint(privateProviderWindowFrom(ctx)) || bBefore != repairReadinessFingerprint(privateProviderWindowFrom(bctx)) {
		t.Error("ASSERT_BOUNDARY_PRIVATE_PEER: refused without either schedule mutation required")
	} else {
		t.Log("ASSERT_BOUNDARY_PRIVATE_PEER: PASS")
	}
	boundarySuffix(t, m, st, start, ctx, s.child.uri)
	boundarySuffix(t, b, bt, bst, bctx, bs.child.uri)
}

// A bounded negative fixture: this is not evidence of successful provider work.
func TestPrivateProviderWindowBoundaryReconcile(t *testing.T) {
	m, start, ctx, s := windowTestSetup(t, PrivateProviderWindowPlan{Definitions: true}, "")
	st := windowTestReady(t, m, start, ctx)
	b, bs, _, _ := repairOrdinary(t, m, start, false)
	fake := newReapJoinChild()
	fake.join <- completedReapObservation()
	b.mu.Lock()
	r := b.sessions[bs.SessionID]
	original := r.process
	r.process = fake
	r.reap = &reapReconciliation{generation: bs.Generation, child: fake}
	b.mu.Unlock()
	got := b.Stop(ctx, bs.SessionID, "negative-reconcile")
	b.mu.Lock()
	r.process = original
	r.reap = nil
	b.mu.Unlock()
	if fake.joins.Load() != 0 || fake.teardowns.Load() != 0 || fake.closeCalls.Load() != 0 || got.Failure == "" || got.IntentID != "" {
		t.Errorf("ASSERT_BOUNDARY_RECONCILE_PRE_EFFECT: joins=%d result=%+v", fake.joins.Load(), got)
	} else {
		t.Log("ASSERT_BOUNDARY_RECONCILE_PRE_EFFECT: PASS")
	}
	boundarySuffix(t, m, st, start, ctx, s.child.uri)
}

func TestPrivateProviderWindowBoundaryDerive(t *testing.T) {
	for _, kind := range []string{"window-ordinary-parent", "ordinary-private-parent", "window-missing-parent"} {
		t.Run(kind, func(t *testing.T) {
			m, start, ctx, s := windowTestSetup(t, PrivateProviderWindowPlan{Definitions: true}, "")
			st := windowTestReady(t, m, start, ctx)
			b, bs, _, _ := repairOrdinary(t, m, start, false)
			callctx := ctx
			if kind == "ordinary-private-parent" {
				b, bs, callctx = m, st, context.Background()
			}
			if kind == "window-missing-parent" {
				bs.SessionID = "missing"
			}
			calls := 0
			b.gitWorktreeList = func(context.Context, string) ([]byte, error) {
				calls++
				return nil, errors.New("negative git counter: no external work")
			}
			got := b.DeriveWorkspace(callctx, DeriveWorkspaceRequest{SessionID: bs.SessionID, Generation: bs.Generation, WorkspaceURI: "file://" + start.Profile.Workspace().String()})
			if got.Failure == "" || calls != 0 {
				t.Errorf("ASSERT_BOUNDARY_DERIVE_ZERO_GIT: failure=%s calls=%d", got.Failure, calls)
			} else {
				t.Log("ASSERT_BOUNDARY_DERIVE_ZERO_GIT: PASS")
			}
			boundarySuffix(t, m, st, start, ctx, s.child.uri)
		})
	}
}

func TestPrivateProviderWindowBoundaryGlobalShutdown(t *testing.T) {
	for _, same := range []bool{false, true} {
		t.Run(fmt.Sprintf("same=%t", same), func(t *testing.T) {
			m, start, ctx, s := windowTestSetup(t, PrivateProviderWindowPlan{Definitions: true}, "")
			st := windowTestReady(t, m, start, ctx)
			b, bs, _, _ := repairOrdinary(t, m, start, same)
			// Hold a real ordinary lease across refusal, then join its callback.
			b.mu.Lock()
			r := b.sessions[bs.SessionID]
			workspace := r.record.Profile.Workspace().String()
			b.mu.Unlock()
			entered := make(chan context.Context, 1)
			release := make(chan struct{})
			done := make(chan error, 1)
			go func() {
				done <- b.WithReadySessionLease(context.Background(), SessionLeaseRequest{SessionID: bs.SessionID, Generation: bs.Generation, Workspace: workspace}, func(lease context.Context) error { entered <- lease; <-release; return nil })
			}()
			var lease context.Context
			select {
			case lease = <-entered:
			case err := <-done:
				t.Fatalf("ordinary lease not admitted: %v", err)
			}
			b.mu.Lock()
			before := fmt.Sprintf("%d/%t", r.leaseNonce, r.leaseClosing)
			b.mu.Unlock()
			// Old Shutdown would wait for A's retained worker on the own-manager case.
			derived, cancel := context.WithCancel(ctx)
			cancel()
			err := b.Shutdown(derived)
			b.mu.Lock()
			unchanged := !b.closed && before == fmt.Sprintf("%d/%t", r.leaseNonce, r.leaseClosing)
			b.mu.Unlock()
			if err == nil || !unchanged || ctx.Err() != nil || lease.Err() != nil {
				t.Errorf("ASSERT_BOUNDARY_GLOBAL_SHUTDOWN: err=%v unchanged=%t lease=%v", err, unchanged, lease.Err())
			} else {
				t.Log("ASSERT_BOUNDARY_GLOBAL_SHUTDOWN: PASS")
			}
			close(release)
			if e := <-done; e != nil {
				t.Fatal(e)
			}
			boundarySuffix(t, m, st, start, ctx, s.child.uri)
		})
	}
}

func TestPrivateProviderWindowBoundaryLegacyPending(t *testing.T) {
	for _, kind := range []string{"begin", "cancel", "complete", "expire", "retired-begin", "retired-complete"} {
		t.Run(kind, func(t *testing.T) {
			m, start, ctx, s := windowTestSetup(t, PrivateProviderWindowPlan{Definitions: true}, "")
			st := windowTestReady(t, m, start, ctx)
			w := privateProviderWindowFrom(ctx)
			fingerprint := repairReadinessFingerprint(w)
			m.mu.Lock()
			r := m.sessions[st.SessionID]
			oldProcess := r.process
			// Independent synthetic negative pending state under the actual private record.
			key := r.pending.Begin(st.Generation)
			r.requests[key] = &Request{Key: key, Deadline: time.Now().Add(-time.Second)}
			beforeRequests, beforeCancels := len(r.requests), r.cancels
			if kind == "retired-begin" || kind == "retired-complete" {
				r.process = nil
			}
			m.mu.Unlock()
			refused := false
			switch kind {
			case "begin", "retired-begin":
				k, f := m.BeginRequest(st.SessionID, st.Generation, time.Time{})
				refused = f != "" && k == (lspwire.RequestKey{})
			case "cancel":
				state, e := m.CancelRequest(st.SessionID, key)
				refused = state == lspwire.CancelNotPending && e != nil
			case "complete", "retired-complete":
				refused = m.CompleteResponse(st.SessionID, key) == lspwire.ResponseUnknown
			case "expire":
				refused = m.Expire(time.Now()) == 0
			}
			m.mu.Lock()
			unchanged := len(r.requests) == beforeRequests && r.cancels == beforeCancels && r.requests[key] != nil && r.requests[key].Terminal == ""
			r.process = oldProcess
			// Remove synthetic state before the actual A suffix; Accept must still accept it.
			accepted := r.pending.Accept(key) == lspwire.ResponseAccepted
			for k := range r.requests {
				delete(r.requests, k)
				r.pending.Accept(k)
			}
			m.mu.Unlock()
			if !refused || !unchanged || !accepted || fingerprint != repairReadinessFingerprint(w) {
				t.Errorf("ASSERT_BOUNDARY_LEGACY_PENDING: kind=%s refused=%t unchanged=%t pending=%t", kind, refused, unchanged, accepted)
			} else {
				t.Log("ASSERT_BOUNDARY_LEGACY_PENDING: PASS")
			}
			boundarySuffix(t, m, st, start, ctx, s.child.uri)
		})
	}
}

func TestPrivateProviderWindowBoundaryExpiredOwnedCleanup(t *testing.T) {
	m, start, ctx, s := windowTestSetup(t, PrivateProviderWindowPlan{Definitions: true}, "")
	windowTestReady(t, m, start, ctx)
	w := privateProviderWindowFrom(ctx)
	// Original fixture deadline, not a fabricated positive completion.
	<-ctx.Done()
	select {
	case <-w.finishDone:
	case <-time.After(2 * time.Second):
		t.Fatal("ASSERT_BOUNDARY_EXPIRED_CLEANUP: finisher did not join")
	}
	w.mu.Lock()
	completed := w.completion.Completed
	child := w.child
	w.mu.Unlock()
	m.mu.Lock()
	admitted := false
	for _, op := range m.operations {
		if op.CallerID == "private-provider-window" && op.Generation == w.start.Generation && op.State == OperationFailed {
			admitted = true
		}
	}
	m.mu.Unlock()
	select {
	case <-s.child.closed:
	default:
		t.Error("ASSERT_BOUNDARY_EXPIRED_CLEANUP: child not closed")
	}
	if !admitted || !child.cleanupJoined() || completed || !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Errorf("ASSERT_BOUNDARY_EXPIRED_CLEANUP: admitted=%t cleanup=%t completed=%t err=%v", admitted, child.cleanupJoined(), completed, ctx.Err())
	} else {
		t.Log("ASSERT_BOUNDARY_EXPIRED_CLEANUP: PASS")
	}
}

func TestPrivateProviderWindowBoundaryOrdinaryDeriveCounter(t *testing.T) {
	m, start, _, _ := windowTestSetup(t, PrivateProviderWindowPlan{Definitions: true}, "")
	b, bs, _, _ := repairOrdinary(t, m, start, false)
	calls := 0
	b.gitWorktreeList = func(context.Context, string) ([]byte, error) {
		calls++
		return nil, errors.New("ordinary negative counter: no external Git or derived session")
	}
	got := b.DeriveWorkspace(context.Background(), DeriveWorkspaceRequest{SessionID: bs.SessionID, Generation: bs.Generation, WorkspaceURI: "file://" + start.Profile.Workspace().String()})
	if calls != 1 || got.Failure != session.Failure("GIT_WORKTREE_QUERY_FAILED") {
		t.Errorf("ASSERT_BOUNDARY_ORDINARY_DERIVE: calls=%d failure=%s", calls, got.Failure)
	} else {
		t.Log("ASSERT_BOUNDARY_ORDINARY_DERIVE: PASS")
	}
}

func TestPrivateProviderWindowBoundaryOrdinaryPending(t *testing.T) {
	m, start, _, _ := windowTestSetup(t, PrivateProviderWindowPlan{Definitions: true}, "")
	b, bs, _, writes := repairOrdinary(t, m, start, false)
	key, f := b.BeginRequest(bs.SessionID, bs.Generation, time.Time{})
	if f != "" {
		t.Fatal(f)
	}
	before := writes.Load()
	state, err := b.CancelRequest(bs.SessionID, key)
	if state != lspwire.CancelWritten || err != nil || writes.Load() <= before {
		t.Fatal("ASSERT_BOUNDARY_ORDINARY_CANCEL")
	}
	key, f = b.BeginRequest(bs.SessionID, bs.Generation, time.Time{})
	if f != "" || b.CompleteResponse(bs.SessionID, key) != lspwire.ResponseAccepted {
		t.Fatal("ASSERT_BOUNDARY_ORDINARY_COMPLETE")
	}
	_, f = b.BeginRequest(bs.SessionID, bs.Generation, time.Now().Add(-time.Second))
	if f != "" || b.Expire(time.Now()) != 1 {
		t.Fatal("ASSERT_BOUNDARY_ORDINARY_EXPIRE")
	}
	if err := b.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Log("ASSERT_BOUNDARY_ORDINARY_PENDING_SHUTDOWN: PASS")
}
