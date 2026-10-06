package sessionruntime

import (
	"sync"
	"testing"
	"time"
	"unsafe"

	"lsp-trace/internal/session"
)

func TestPrivateB4ByteAccountV2TerminalJoinAndPostTerminalRefusal(t *testing.T) {
	account, failure := newPrivateB4ByteAccountV2()
	if failure != "" {
		t.Fatal(failure)
	}
	general, failure := account.reserve(11)
	if failure != "" {
		t.Fatal(failure)
	}
	descriptor, failure := account.reserveDescriptor(13)
	if failure != "" {
		t.Fatal(failure)
	}
	if got := account.snapshot(); got != (privateB4ByteLedgerSnapshot{Live: privateB4ByteLedgerV2TableBytes + 24, Cumulative: privateB4ByteLedgerV2TableBytes + 11}) {
		t.Fatalf("ASSERT_C15_V2_ACCOUNT_CHARGES snapshot=%+v", got)
	}
	account.requestTerminalRelease()
	if got := account.snapshot(); got.Live != privateB4ByteLedgerV2TableBytes+24 {
		t.Fatalf("ASSERT_C15_V2_ACCOUNT_PREMATURE_BASELINE_RELEASE snapshot=%+v", got)
	}
	before := account.snapshot()
	if _, failure = account.reserve(1); failure != session.ResourceExhausted || account.snapshot() != before {
		t.Fatalf("ASSERT_C15_V2_ACCOUNT_POST_TERMINAL_GENERAL failure=%q", failure)
	}
	if _, failure = account.reserveDescriptor(1); failure != session.ResourceExhausted || account.snapshot() != before {
		t.Fatalf("ASSERT_C15_V2_ACCOUNT_POST_TERMINAL_DESCRIPTOR failure=%q", failure)
	}
	general.release()
	if got := account.snapshot(); got.Live != privateB4ByteLedgerV2TableBytes+13 {
		t.Fatalf("ASSERT_C15_V2_ACCOUNT_DESCRIPTOR_LAST snapshot=%+v", got)
	}
	descriptor.release()
	if got := account.snapshot(); got != (privateB4ByteLedgerSnapshot{Live: 0, Cumulative: privateB4ByteLedgerV2TableBytes + 11}) {
		t.Fatalf("ASSERT_C15_V2_ACCOUNT_FINAL snapshot=%+v", got)
	}
	account.requestTerminalRelease()
	descriptor.release()
	general.release()
	if got := account.snapshot(); got.Live != 0 {
		t.Fatalf("ASSERT_C15_V2_ACCOUNT_REPLAY snapshot=%+v", got)
	}
}

func TestPrivateB4ByteAccountV2QuiescenceBeforeTerminalDoesNotRelease(t *testing.T) {
	account, failure := newPrivateB4ByteAccountV2()
	if failure != "" {
		t.Fatal(failure)
	}
	lease, failure := account.reserve(7)
	if failure != "" {
		t.Fatal(failure)
	}
	lease.release()
	if got := account.snapshot(); got.Live != privateB4ByteLedgerV2TableBytes || got.Cumulative != privateB4ByteLedgerV2TableBytes+7 {
		t.Fatalf("ASSERT_C15_V2_ACCOUNT_PRETERMINAL_BASELINE snapshot=%+v", got)
	}
	retry, failure := account.reserve(9)
	if failure != "" {
		t.Fatalf("ASSERT_C15_V2_ACCOUNT_PRETERMINAL_REUSE failure=%q", failure)
	}
	retry.release()
	account.requestTerminalRelease()
	if got := account.snapshot(); got != (privateB4ByteLedgerSnapshot{Cumulative: privateB4ByteLedgerV2TableBytes + 16}) {
		t.Fatalf("ASSERT_C15_V2_ACCOUNT_EMPTY_TERMINAL snapshot=%+v", got)
	}
}

func TestPrivateB4FrozenLegacyLedgerAndLeaseLayout(t *testing.T) {
	var entry privateB4ByteEntry
	var ledger privateB4ByteLedger
	var lease privateB4ByteLease
	if unsafe.Sizeof(entry) != 24 || unsafe.Alignof(entry) != 8 || unsafe.Offsetof(entry.capacity) != 0 || unsafe.Offsetof(entry.generation) != 8 || unsafe.Offsetof(entry.references) != 16 || unsafe.Offsetof(entry.active) != 20 ||
		unsafe.Sizeof(ledger) != 24600 || unsafe.Alignof(ledger) != 8 || unsafe.Offsetof(ledger.live) != 0 || unsafe.Offsetof(ledger.cumulative) != 8 || unsafe.Offsetof(ledger.nextGen) != 16 || unsafe.Offsetof(ledger.entries) != 24 || len(ledger.entries) != 1024 ||
		unsafe.Sizeof(lease) != 32 || unsafe.Alignof(lease) != 8 || unsafe.Offsetof(lease.ledger) != 0 || unsafe.Offsetof(lease.slot) != 8 || unsafe.Offsetof(lease.generation) != 16 || unsafe.Offsetof(lease.active) != 24 {
		t.Fatal("ASSERT_C15_FROZEN_LEGACY_LAYOUT")
	}
}

func TestPrivateB4ByteAccountV2CopiedLeasesDescriptorIsolationAndExactOnce(t *testing.T) {
	account, failure := newPrivateB4ByteAccountV2()
	if failure != "" {
		t.Fatal(failure)
	}
	general, failure := account.reserve(17)
	if failure != "" {
		t.Fatal(failure)
	}
	generalCopy := general
	descriptor, failure := account.reserveDescriptor(19)
	if failure != "" {
		t.Fatal(failure)
	}
	descriptorCopy := descriptor
	beforeDuplicate := account.snapshot()
	if _, failure := account.reserveDescriptor(1); failure != session.ResourceExhausted || account.snapshot() != beforeDuplicate {
		t.Fatalf("ASSERT_C15_V2_DESCRIPTOR_DUPLICATE failure=%q", failure)
	}
	account.requestTerminalRelease()
	var wg sync.WaitGroup
	for _, release := range []func(){general.release, generalCopy.release, descriptor.release, descriptorCopy.release} {
		wg.Add(1)
		go func(release func()) { defer wg.Done(); release() }(release)
	}
	wg.Wait()
	general.release()
	generalCopy.release()
	descriptor.release()
	descriptorCopy.release()
	account.requestTerminalRelease()
	if got := account.snapshot(); got != (privateB4ByteLedgerSnapshot{Cumulative: privateB4ByteLedgerV2TableBytes + 17}) || !account.backingReleased {
		t.Fatalf("ASSERT_C15_V2_COPIED_EXACT_ONCE snapshot=%+v released=%t", got, account.backingReleased)
	}
}

func TestPrivateB4ByteAccountV2ConcurrentTerminalJoin(t *testing.T) {
	for i := 0; i < 100; i++ {
		account, failure := newPrivateB4ByteAccountV2()
		if failure != "" {
			t.Fatal(failure)
		}
		general, _ := account.reserve(1)
		descriptor, _ := account.reserveDescriptor(1)
		var wg sync.WaitGroup
		wg.Add(4)
		go func() { defer wg.Done(); account.requestTerminalRelease() }()
		go func() { defer wg.Done(); general.release() }()
		go func() { defer wg.Done(); descriptor.release() }()
		go func() {
			defer wg.Done()
			lease, failure := account.reserve(1)
			if failure == "" {
				lease.release()
			}
		}()
		done := make(chan struct{})
		go func() { wg.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("ASSERT_C15_V2_ACCOUNT_DEADLOCK")
		}
		account.requestTerminalRelease()
		if got := account.snapshot(); got.Live != 0 {
			t.Fatalf("ASSERT_C15_V2_CONCURRENT_FINAL snapshot=%+v", got)
		}
	}
}
