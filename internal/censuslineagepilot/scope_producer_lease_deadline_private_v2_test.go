//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

func producerSupplyAndQueryTrafficPrivateV2(c *scopeAdapterChild) int {
	n := 0
	for _, msg := range c.snapshot() {
		switch msg.Method {
		case "textDocument/didOpen", "textDocument/didChange", "textDocument/documentSymbol", "textDocument/prepareCallHierarchy", "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls":
			n++
		}
	}
	return n
}

func TestScopeProducerPrivateV2RejectsUnleasedBeforeSupply(t *testing.T) {
	spec, m, id, child := producerFixturePrivateV2(t, 1, 0)
	f, err := FreezeScopePrivateV2(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	before := producerSupplyAndQueryTrafficPrivateV2(child)
	o, _, err := DiscoverScopePrivateV2(context.Background(), f, m, id)
	// This diagnostic runs only on the violating implementation. It demonstrates
	// that tokenless discovery could even reach the real closed capture route.
	if err == nil && o.valid() {
		b, _ := o.Batch(0)
		pair, e := AcquireScopeBatchPrivateV2(context.Background(), m, o, b)
		t.Logf("unleased old route: origin=%t capture=%t error=%v", o.valid(), pair.valid(), e)
	}
	after := producerSupplyAndQueryTrafficPrivateV2(child)
	if err == nil || o.valid() || before != after {
		t.Fatalf("ASSERT_V2_ACTIVE_LEASE_BEFORE_SUPPLY: origin=%t error=%v traffic=%d->%d", o.valid(), err, before, after)
	}
	var failure scopeRuntimeFailurePrivate
	if !errors.As(err, &failure) || failure.Failure != session.LifecycleConflict {
		t.Fatalf("ASSERT_V2_ACTIVE_LEASE_FAILURE_CLASS: %v", err)
	}
	t.Log("ASSERT_V2_ACTIVE_LEASE_BEFORE_SUPPLY: PASS")
}

func TestScopeProducerPrivateV2LeaseCheckPrecedesSourceRevalidation(t *testing.T) {
	for _, kind := range []string{"tokenless", "retired"} {
		t.Run(kind, func(t *testing.T) {
			spec, m, id, child := producerFixturePrivateV2(t, 1, 0)
			f, err := FreezeScopePrivateV2(context.Background(), spec)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			if kind == "retired" {
				producerLeasePrivateV2(t, m, id, spec.Root, func(lease context.Context) error { ctx = lease; return nil })
				if ctx.Err() != nil {
					t.Fatal("fixture must expose the uncancelled retired-token case")
				}
			}
			if err = os.Remove(filepath.Join(spec.Root, spec.Members[0].CanonicalRelativePath)); err != nil {
				t.Fatal(err)
			}
			before := producerSupplyAndQueryTrafficPrivateV2(child)
			_, err = newScopeRuntimeAdapterPrivateV2(ctx, f, m, id)
			var failure scopeRuntimeFailurePrivate
			if !errors.As(err, &failure) || failure.Failure != session.LifecycleConflict || producerSupplyAndQueryTrafficPrivateV2(child) != before {
				t.Fatalf("ASSERT_V2_LEASE_CHECK_BEFORE_SOURCE_IO: %s: %v", kind, err)
			}
		})
	}
}

func TestScopeProducerPrivateV2AdapterActiveLeaseBoundary(t *testing.T) {
	spec, m, id, _ := producerFixturePrivateV2(t, 1, 0)
	f, err := FreezeScopePrivateV2(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	var adapter *scopeRuntimeAdapter
	var retired context.Context
	producerLeasePrivateV2(t, m, id, spec.Root, func(ctx context.Context) error {
		retired = ctx
		var e error
		adapter, e = newScopeRuntimeAdapterPrivateV2(ctx, f, m, id)
		if e != nil {
			return e
		}
		if adapter.lifetimeV2 != ctx || ctx.Err() != nil {
			t.Fatal("ASSERT_V2_ORIGINAL_LIFETIME_NOT_CONSTRUCTION_CHILD")
		}
		child, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		if e = adapter.check(child); e != nil {
			return e
		}
		var failure scopeRuntimeFailurePrivate
		if e = adapter.check(context.Background()); !errors.As(e, &failure) || failure.Failure != session.LifecycleConflict {
			t.Fatalf("ASSERT_V2_OPERATION_REQUIRES_ACTIVE_TOKEN: %v", e)
		}
		return adapter.check(ctx)
	})
	if retired.Err() != nil {
		t.Fatal("fixture requires uncancelled retired lifetime")
	}
	if err = os.Remove(filepath.Join(spec.Root, spec.Members[0].CanonicalRelativePath)); err != nil {
		t.Fatal(err)
	}
	var failure scopeRuntimeFailurePrivate
	if err = adapter.check(retired); !errors.As(err, &failure) || failure.Failure != session.LifecycleConflict {
		t.Fatalf("ASSERT_V2_ADAPTER_RETIRED_BEFORE_SOURCE_IO: %v", err)
	}
}

func TestScopeProducerPrivateV2BlockedPreparationDeadline(t *testing.T) {
	blockedPreparationPolicyPrivateV2(t, 600)
}

func TestScopeProducerPrivateV2BlockedPreparationRequestCap(t *testing.T) {
	// If request timeout is dropped but the 5s overall cap remains, the same
	// conservative 3s observation window still rejects the implementation.
	blockedPreparationPolicyPrivateV2(t, 5000)
}

func blockedPreparationPolicyPrivateV2(t *testing.T, overallMS int) {
	t.Helper()
	paused, writing, closed := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var writeOnce sync.Once
	var spec ScopeSpecPrivateV2
	_, m, id, child, _, _ := adapterFixtureWithChildSetupPrivate(t, "", func(c *scopeAdapterChild) {
		c.closedPrivateTest = closed
		// Read initialization completely, then stop draining the actual pipe. The
		// next PrepareDocument notification write cannot complete without cancellation.
		c.afterInitializedPrivateTest = func() { close(paused); <-closed }
		c.writeObservedPrivateTest = func() {
			select {
			case <-paused:
				writeOnce.Do(func() { close(writing) })
			default:
			}
		}
	}, func(root string) {
		spec = producerSpecPrivateV2(t, root, 1, 0)
		raw := append([]byte("package owned\n"), bytes.Repeat([]byte("// owned blocked notification payload\n"), 12000)...)
		if len(raw) > 1<<20 {
			t.Fatal("fixture source too large")
		}
		p := filepath.Join(root, spec.Members[0].CanonicalRelativePath)
		if err := os.WriteFile(p, raw, 0600); err != nil {
			t.Fatal(err)
		}
		spec.Members[0].ExpectedSHA256 = Digest(raw)
		spec.Members[0].ByteLength = uint64(len(raw))
		spec.EffectivePolicy.TimeoutMS = overallMS
		spec.EffectivePolicy.RequestTimeoutMS = 150
	})
	f, err := FreezeScopePrivateV2(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	var origin EnumerationOriginPrivateV2
	var operationErr error
	watchdogFired := make(chan struct{})
	watchdog := time.AfterFunc(10*time.Second, func() { close(watchdogFired); child.Close() })
	defer watchdog.Stop()
	go func() {
		defer close(done)
		operationErr = m.WithReadySessionLease(context.Background(), sessionruntime.SessionLeaseRequest{SessionID: id.SessionID, Generation: id.Generation, Workspace: spec.Root}, func(lease context.Context) error {
			var e error
			origin, _, e = DiscoverScopePrivateV2(lease, f, m, id)
			return e
		})
	}()
	defer func() {
		child.Close()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("ASSERT_V2_BLOCKED_FIXTURE_CLEANUP_DRAIN")
		}
	}()
	select {
	case <-writing: // Actual notification write has entered the undrained io.Pipe.
	case <-done:
		// Both channels may already be ready if the test goroutine was delayed.
		// Select ordering must not turn a completed witnessed write into a failure.
		select {
		case <-writing:
		default:
			t.Fatalf("ASSERT_V2_BLOCKED_WRITE_EXERCISED: producer returned before a write attempt: %v", operationErr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ASSERT_V2_BLOCKED_WRITE_EXERCISED: no write attempt")
	}
	select {
	case <-done:
		if operationErr == nil || origin.valid() {
			t.Fatalf("ASSERT_V2_PREPARATION_POLICY_STOP: origin=%t error=%v", origin.valid(), operationErr)
		}
		var failure scopeRuntimeFailurePrivate
		if !errors.As(operationErr, &failure) || failure.Failure != session.RequestTimeout {
			t.Fatalf("ASSERT_V2_PREPARATION_TIMEOUT_CLASS: %v", operationErr)
		}
		if got := m.CheckActiveSessionLeasePrivateV2(context.Background(), sessionruntime.SessionLeaseRequest{SessionID: id.SessionID, Generation: id.Generation, Workspace: spec.Root}); got != session.SessionPoisoned {
			t.Fatalf("ASSERT_V2_CANCELLED_WRITE_POISON_PRESERVED: %s", got)
		}
		select {
		case <-watchdogFired:
			t.Fatal("ASSERT_V2_PREPARATION_POLICY_STOP: external watchdog caused return")
		default:
		}
		if len(semanticMessages(child)) != 0 {
			t.Fatal("ASSERT_V2_PREPARATION_NO_DISCOVERY_AFTER_TIMEOUT")
		}
		t.Log("ASSERT_V2_PREPARATION_POLICY_STOP: PASS (blocked real notification write, no external cleanup)")
	case <-time.After(3 * time.Second):
		// Conservative 3s observation window vs a 150ms request; the overall
		// cap is either 600ms or 5s. The separate 10s watchdog is cleanup, not success.
		t.Error("ASSERT_V2_PREPARATION_POLICY_STOP: real notification write remained blocked beyond policy window")
		child.Close()
	}
}
