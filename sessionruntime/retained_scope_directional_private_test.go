package sessionruntime

import (
	"context"
	"testing"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/directionalraw"
	"lsp-trace/internal/session"
)

// This is a runtime conjunction probe, not an acquisition producer: the real
// reservation starts unattached and only Manager may install its custody owner.
func TestRetainedScopeDirectionalPrivateConjunction(t *testing.T) {
	m, s, _, members := retainedScopeFixture(t)
	g, failure := m.BindRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, members)
	if failure != "" {
		t.Fatal(failure)
	}
	reservation := directionalraw.NewReservation("retained-scope-runtime-conjunction")
	defer reservation.Release()
	for _, method := range []string{"textDocument/prepareCallHierarchy", "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls"} {
		req := retainedScopeRequest(s, g, method, members[0].URI)
		got, lease := m.RoundTripDirectionalPrivate(reservation.Context(context.Background()), req)
		if got.Failure != "" {
			t.Fatalf("ASSERT_RETAINED_DIRECTIONAL_CONJUNCTION %s: failure=%s attached=%t", method, got.Failure, reservation.Attached())
		}
		if err := acquisition.BindDirectionalResponse(lease); err != nil {
			t.Fatalf("ASSERT_RETAINED_DIRECTIONAL_BIND %s: %v", method, err)
		}
		if _, ok := lease.CompletedMetadataPrivate(); !ok {
			t.Fatalf("ASSERT_RETAINED_DIRECTIONAL_COMPLETION %s", method)
		}
	}
	if !reservation.Attached() || m.directionalV5 == nil || m.directionalV5.account == nil || m.directionalV5.owner == nil {
		t.Fatal("ASSERT_RETAINED_DIRECTIONAL_MANAGER_OWNER")
	}
	if !reservation.Release() || !reservation.Closed() {
		t.Fatal("ASSERT_RETAINED_DIRECTIONAL_RELEASE")
	}
	live, _, allocations, releases, _, occupied := m.DirectionalAccountingPrivate()
	if live != 0 || occupied || allocations != 2 || releases != 1 {
		t.Fatalf("ASSERT_RETAINED_DIRECTIONAL_ACCOUNT_RELEASE live=%d allocations=%d releases=%d occupied=%t", live, allocations, releases, occupied)
	}
	prepLive, _, prepAllocations, prepReleases, prepOccupied := m.DirectionalPreparationAccountingPrivate()
	if prepLive != 0 || prepOccupied || prepAllocations != 2 || prepReleases != 1 {
		t.Fatalf("ASSERT_RETAINED_DIRECTIONAL_PREPARATION_RELEASE live=%d allocations=%d releases=%d", prepLive, prepAllocations, prepReleases)
	}
}

func TestRetainedScopeDirectionalPrivateAuthorityRefusals(t *testing.T) {
	for _, kind := range []string{"missing", "copied", "foreign", "retired", "session", "generation", "member", "external", "cancelled", "poisoned"} {
		t.Run(kind, func(t *testing.T) {
			m, s, child, members := retainedScopeFixture(t)
			ctx := context.Background()
			var guard *RetainedScopeGuardPrivate
			if kind == "retired" {
				if err := m.WithReadySessionLease(ctx, SessionLeaseRequest{SessionID: s.SessionID, Generation: s.Generation, Workspace: m.sessions[s.SessionID].record.Profile.Workspace().String()}, func(c context.Context) error {
					var f session.Failure
					guard, f = m.BindRetainedScopePrivate(c, s.SessionID, s.Generation, members)
					if f != "" {
						t.Fatal(f)
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			} else {
				var f session.Failure
				guard, f = m.BindRetainedScopePrivate(ctx, s.SessionID, s.Generation, members)
				if f != "" {
					t.Fatal(f)
				}
			}
			req := retainedScopeRequest(s, guard, "callHierarchy/incomingCalls", members[0].URI)
			switch kind {
			case "missing":
				req.RetainedScopePrivate = nil
			case "copied":
				cp := *guard
				req.RetainedScopePrivate = &cp
			case "foreign":
				other, os, _, om := retainedScopeFixture(t)
				req.RetainedScopePrivate, _ = other.BindRetainedScopePrivate(ctx, os.SessionID, os.Generation, om)
			case "session":
				req.SessionID = "other"
			case "generation":
				req.Generation++
			case "member":
				d := m.sessions[s.SessionID].documents[members[5].URI]
				d.version++
				m.sessions[s.SessionID].documents[members[5].URI] = d
			case "external":
				req = retainedScopeRequest(s, guard, req.Method, "file:///external.go")
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "poisoned":
				m.Poison(s.SessionID, s.Generation)
			}
			r := directionalraw.NewReservation("guard-refusal-fixture")
			defer r.Release()
			before, _ := child.snapshot()
			result, lease := m.RoundTripDirectionalPrivate(r.Context(ctx), req)
			after, _ := child.snapshot()
			if result.Failure == "" || r.Attached() || len(before) != len(after) {
				t.Fatalf("ASSERT_DIRECTIONAL_AUTHORITY_ZERO_TRAFFIC failure=%s attached=%t before=%d after=%d", result.Failure, r.Attached(), len(before), len(after))
			}
			if _, ok := lease.CompletedMetadataPrivate(); ok {
				t.Fatal("ASSERT_DIRECTIONAL_REFUSAL_NO_LEASE")
			}
		})
	}
}
