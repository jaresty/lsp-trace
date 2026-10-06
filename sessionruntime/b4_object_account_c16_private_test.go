package sessionruntime

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-trace/internal/session"
)

func TestPrivateB4C16ObjectEqualityPlusOneAndRollback(t *testing.T) {
	profile, failure := newPrivateB4AccountC16()
	if failure != "" {
		t.Fatal(failure)
	}
	for i := uint64(0); i < privateB4ObjectLimit; i++ {
		if err := profile.WithObjectAdmission(func() error { return nil }); err != nil {
			t.Fatalf("ASSERT_C16_EQUALITY_4096 ordinal=%d err=%v", i, err)
		}
	}
	before := profile.objectSnapshot()
	called := false
	if err := profile.WithObjectAdmission(func() error { called = true; return nil }); !errors.Is(err, errPrivateB4ObjectAdmission) {
		t.Fatalf("ASSERT_C16_PLUS_ONE_REFUSAL err=%v", err)
	}
	if called {
		t.Fatal("ASSERT_C16_PLUS_ONE_BEFORE_CALLBACK")
	}
	if after := profile.objectSnapshot(); after != before {
		t.Fatalf("ASSERT_C16_PLUS_ONE_ATOMIC before=%+v after=%+v", before, after)
	}

	rollback, failure := newPrivateB4AccountC16()
	if failure != "" {
		t.Fatal(failure)
	}
	callbackErr := errors.New("materialization failed")
	if err := rollback.WithObjectAdmission(func() error { return callbackErr }); !errors.Is(err, callbackErr) {
		t.Fatalf("ASSERT_C16_CALLBACK_ERROR err=%v", err)
	}
	if snapshot := rollback.objectSnapshot(); snapshot.Materialized != 0 || snapshot.InFlight != 0 || snapshot.ActiveCallbacks != 0 {
		t.Fatalf("ASSERT_C16_CALLBACK_ROLLBACK snapshot=%+v", snapshot)
	}
}

func TestPrivateB4C16ConcurrentAdmissionsExposeSettledCounts(t *testing.T) {
	profile, failure := newPrivateB4AccountC16()
	if failure != "" {
		t.Fatal(failure)
	}
	const count = 32
	entered := make(chan struct{}, count)
	release := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(count)
	for i := 0; i < count; i++ {
		go func() {
			defer wg.Done()
			if err := profile.WithObjectAdmission(func() error {
				entered <- struct{}{}
				<-release
				return nil
			}); err != nil {
				t.Errorf("ASSERT_C16_CONCURRENT_ADMISSION err=%v", err)
			}
		}()
	}
	for i := 0; i < count; i++ {
		<-entered
	}
	if snapshot := profile.objectSnapshot(); snapshot.InFlight != count || snapshot.ActiveCallbacks != count || snapshot.Materialized != 0 {
		t.Fatalf("ASSERT_C16_CONCURRENT_INFLIGHT snapshot=%+v", snapshot)
	}
	close(release)
	wg.Wait()
	if snapshot := profile.objectSnapshot(); snapshot.InFlight != 0 || snapshot.ActiveCallbacks != 0 || snapshot.Materialized != count {
		t.Fatalf("ASSERT_C16_CONCURRENT_SETTLED snapshot=%+v", snapshot)
	}
}

func TestPrivateB4C16TerminalTimeoutRetryAndClosedAdmission(t *testing.T) {
	profile, failure := newPrivateB4AccountC16()
	if failure != "" {
		t.Fatal(failure)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- profile.WithObjectAdmission(func() error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if failure := profile.requestTerminalReleaseC16(ctx); failure != session.RequestTimeout {
		t.Fatalf("ASSERT_C16_TERMINAL_TIMEOUT failure=%s", failure)
	}
	if snapshot := profile.objectSnapshot(); !snapshot.TerminalRequested || snapshot.BackingReleased || snapshot.ActiveCallbacks != 1 {
		t.Fatalf("ASSERT_C16_TIMEOUT_PRESERVES_CALLBACK snapshot=%+v", snapshot)
	}
	var callbackRan atomic.Bool
	if err := profile.WithObjectAdmission(func() error { callbackRan.Store(true); return nil }); !errors.Is(err, errPrivateB4ObjectAdmission) || callbackRan.Load() {
		t.Fatalf("ASSERT_C16_TERMINAL_CLOSES_ADMISSION err=%v callback=%t", err, callbackRan.Load())
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if failure := profile.requestTerminalReleaseC16(context.Background()); failure != "" {
		t.Fatalf("ASSERT_C16_TERMINAL_RETRY failure=%s", failure)
	}
	if snapshot := profile.objectSnapshot(); snapshot.Materialized != 1 || snapshot.InFlight != 0 || snapshot.ActiveCallbacks != 0 || !snapshot.BackingReleased {
		t.Fatalf("ASSERT_C16_TERMINAL_SETTLED snapshot=%+v", snapshot)
	}
}

func TestPrivateB4C16ProfileSelfCostCombinedC15AndLegacyUnchanged(t *testing.T) {
	legacy, failure := newPrivateB4ByteAccountV2()
	if failure != "" {
		t.Fatal(failure)
	}
	if got := legacy.snapshot(); got.Live != privateB4ByteLedgerV2TableBytes || got.Cumulative != privateB4ByteLedgerV2TableBytes {
		t.Fatalf("ASSERT_C16_LEGACY_C15_UNCHANGED snapshot=%+v", got)
	}
	legacy.requestTerminalRelease()

	profile, failure := newPrivateB4AccountC16()
	if failure != "" {
		t.Fatal(failure)
	}
	cost := privateB4AccountC16SelfCost()
	initial := profile.bytes.snapshot()
	if initial.Live != privateB4ByteLedgerV2TableBytes+cost || initial.Cumulative != initial.Live {
		t.Fatalf("ASSERT_C16_EXACT_SELF_COST cost=%d snapshot=%+v", cost, initial)
	}
	remaining := privateB4MaxOwnedBytes - initial.Live
	lease, failure := profile.bytes.reserve(remaining)
	if failure != "" {
		t.Fatalf("ASSERT_C16_COMBINED_C15_EQUALITY failure=%s", failure)
	}
	before := profile.bytes.snapshot()
	if refused, failure := profile.bytes.reserve(1); failure != session.ResourceExhausted || refused.lease.active || profile.bytes.snapshot() != before {
		t.Fatalf("ASSERT_C16_COMBINED_C15_PLUS_ONE failure=%s before=%+v after=%+v", failure, before, profile.bytes.snapshot())
	}
	lease.release()
	if failure := profile.requestTerminalReleaseC16(context.Background()); failure != "" {
		t.Fatal(failure)
	}
	if got := profile.bytes.snapshot(); got.Live != 0 || got.Cumulative != privateB4MaxOwnedBytes {
		t.Fatalf("ASSERT_C16_PROFILE_TERMINAL_RELEASE snapshot=%+v", got)
	}
}
