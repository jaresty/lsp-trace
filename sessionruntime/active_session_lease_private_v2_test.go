package sessionruntime

import (
	"context"
	"testing"
	"time"

	"lsp-trace/internal/session"
)

func activeLeaseFixturePrivateV2(t *testing.T) (*Manager, SessionLeaseRequest, *roundTripChild) {
	t.Helper()
	m, s, child, _ := ownedDocumentFixture(t, true)
	t.Cleanup(func() { child.Teardown(context.Background()); child.Close(); m.Shutdown(context.Background()) })
	workspace, ok := m.WorkspaceRoot(s.SessionID, s.Generation)
	if !ok {
		t.Fatal("workspace")
	}
	return m, SessionLeaseRequest{SessionID: s.SessionID, Generation: s.Generation, Workspace: workspace}, child
}

func TestActiveSessionLeasePrivateV2ExactTokenAndLifetime(t *testing.T) {
	m, req, child := activeLeaseFixturePrivateV2(t)
	if f := m.CheckActiveSessionLeasePrivateV2(context.Background(), req); f != session.LifecycleConflict {
		t.Fatalf("tokenless: %s", f)
	}
	// This is intentionally NOT a change to ordinary tokenless admission.
	m.mu.Lock()
	ordinary := m.validSessionLeaseLocked(context.Background(), m.sessions[req.SessionID])
	m.mu.Unlock()
	if !ordinary {
		t.Fatal("ASSERT_ACTIVE_LEASE_ORDINARY_TOKENLESS_UNCHANGED")
	}
	var retired context.Context
	err := m.WithReadySessionLease(context.Background(), req, func(ctx context.Context) error {
		retired = ctx
		before := retainedScopeV2WireRequests(child)
		m.mu.Lock()
		r := m.sessions[req.SessionID]
		documents, workers, token, nonce := len(r.documents), m.workers, r.leaseToken, r.leaseNonce
		m.mu.Unlock()
		for _, candidate := range []context.Context{ctx, context.WithValue(ctx, struct{ child bool }{true}, "child")} {
			if f := m.CheckActiveSessionLeasePrivateV2(candidate, req); f != "" {
				t.Fatalf("active exact token: %s", f)
			}
		}
		bounded, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		if f := m.CheckActiveSessionLeasePrivateV2(bounded, req); f != "" {
			t.Fatalf("active deadline child: %s", f)
		}
		if f := m.CheckActiveSessionLeasePrivateV2(context.Background(), req); f != session.LifecycleConflict {
			t.Fatalf("tokenless while leased: %s", f)
		}
		// A genuine pointer under an unrelated key is not the Manager's namespace.
		wrongNamespace := context.WithValue(context.Background(), struct{ foreign bool }{true}, ctx.Value(sessionLeaseContextKey{}))
		if f := m.CheckActiveSessionLeasePrivateV2(wrongNamespace, req); f != session.LifecycleConflict {
			t.Fatalf("foreign token namespace: %s", f)
		}
		m.mu.Lock()
		unchanged := len(r.documents) == documents && m.workers == workers && r.leaseToken == token && r.leaseNonce == nonce
		m.mu.Unlock()
		if !unchanged || retainedScopeV2WireRequests(child) != before {
			t.Fatal("ASSERT_ACTIVE_LEASE_CHECK_READ_ONLY")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if retired.Err() != nil {
		t.Fatal("fixture requires an uncancelled retired context")
	}
	if f := m.CheckActiveSessionLeasePrivateV2(retired, req); f != session.LifecycleConflict {
		t.Fatalf("retired uncancelled context: %s", f)
	}
	if err = m.WithReadySessionLease(context.Background(), req, func(current context.Context) error {
		if f := m.CheckActiveSessionLeasePrivateV2(current, req); f != "" {
			t.Fatal(f)
		}
		if f := m.CheckActiveSessionLeasePrivateV2(retired, req); f != session.LifecycleConflict {
			t.Fatalf("other lease token: %s", f)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	t.Log("ASSERT_ACTIVE_LEASE_EXACT_TOKEN_AND_RETIREMENT: PASS")
}

func TestActiveSessionLeasePrivateV2Refusals(t *testing.T) {
	m, req, _ := activeLeaseFixturePrivateV2(t)
	err := m.WithReadySessionLease(context.Background(), req, func(ctx context.Context) error {
		for _, kind := range []string{"nil-context", "nil-manager", "cancelled", "expired", "missing-session", "generation", "workspace"} {
			t.Run(kind, func(t *testing.T) {
				candidate, request, manager := ctx, req, m
				want := session.LifecycleConflict
				switch kind {
				case "nil-context":
					candidate = nil
				case "nil-manager":
					manager = nil
				case "cancelled":
					cancelled, cancel := context.WithCancel(ctx)
					cancel()
					candidate = cancelled
					want = contextFailure(candidate)
				case "expired":
					expired, cancel := context.WithDeadline(ctx, time.Now().Add(-time.Second))
					defer cancel()
					candidate = expired
					want = session.RequestTimeout
				case "missing-session":
					request.SessionID += "-missing"
					want = session.SessionNotFound
				case "generation":
					request.Generation++
					want = session.StaleGeneration
				case "workspace":
					request.Workspace += "/foreign"
				}
				if f := manager.CheckActiveSessionLeasePrivateV2(candidate, request); f != want {
					t.Fatalf("%s: got %s want %s", kind, f, want)
				}
			})
		}
		for _, kind := range []string{"closed", "not-ready", "poisoned", "protocol-owned", "lifecycle-owned", "closing", "zero-count", "two-count", "missing-token", "copied-token", "nonce"} {
			t.Run(kind, func(t *testing.T) {
				m.mu.Lock()
				r := m.sessions[req.SessionID]
				state, closed, protocol, lifecycle, closing, count, token, nonce := r.record.State, m.closed, r.protocolOwned, r.lifecycleOwned, r.leaseClosing, r.leaseCount, r.leaseToken, r.leaseNonce
				want := session.LifecycleConflict
				switch kind {
				case "closed":
					m.closed = true
				case "not-ready":
					r.record.State = "STARTING"
				case "poisoned":
					r.record.State = session.Poisoned
					want = session.SessionPoisoned
				case "protocol-owned":
					r.protocolOwned = true
				case "lifecycle-owned":
					r.lifecycleOwned = true
				case "closing":
					r.leaseClosing = true
				case "zero-count":
					r.leaseCount = 0
				case "two-count":
					r.leaseCount = 2
				case "missing-token":
					r.leaseToken = nil
				case "copied-token":
					copied := *r.leaseToken
					r.leaseToken = &copied
				case "nonce":
					r.leaseNonce++
				}
				m.mu.Unlock()
				got := m.CheckActiveSessionLeasePrivateV2(ctx, req)
				m.mu.Lock()
				r.record.State, m.closed, r.protocolOwned, r.lifecycleOwned, r.leaseClosing, r.leaseCount, r.leaseToken, r.leaseNonce = state, closed, protocol, lifecycle, closing, count, token, nonce
				m.mu.Unlock()
				if got != want {
					t.Fatalf("%s: got %s want %s", kind, got, want)
				}
			})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestActiveSessionLeasePrivateV2ForeignManager(t *testing.T) {
	m, req, _ := activeLeaseFixturePrivateV2(t)
	other, otherReq, _ := activeLeaseFixturePrivateV2(t)
	err := m.WithReadySessionLease(context.Background(), req, func(ctx context.Context) error {
		if f := other.CheckActiveSessionLeasePrivateV2(ctx, otherReq); f != session.LifecycleConflict {
			t.Fatalf("foreign Manager token without its lease: %s", f)
		}
		return other.WithReadySessionLease(context.Background(), otherReq, func(otherCtx context.Context) error {
			if f := m.CheckActiveSessionLeasePrivateV2(otherCtx, req); f != session.LifecycleConflict {
				t.Fatalf("foreign active token: %s", f)
			}
			if f := other.CheckActiveSessionLeasePrivateV2(ctx, otherReq); f != session.LifecycleConflict {
				t.Fatalf("other active lease: %s", f)
			}
			return nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
}
