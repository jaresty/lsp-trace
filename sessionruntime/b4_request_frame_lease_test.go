package sessionruntime

import (
	"bytes"
	"errors"
	"sync"
	"testing"
	"time"
)

func privateB4BorrowedRequestFrame(t *testing.T, result RoundTripResult) ([]byte, bool) {
	t.Helper()
	lease, ok := result.PrivateB4RequestFrameLease()
	if !ok {
		return nil, false
	}
	var copied []byte
	if err := lease.WithBytes(func(frame []byte) error { copied = append([]byte(nil), frame...); return nil }); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lease.Release() })
	return copied, true
}

func testB4RequestFrameLease(t *testing.T, frame []byte) (B4RequestFrameLease, *privateB4ByteAccountV2) {
	t.Helper()
	ledger := mustPrivateB4ByteAccountV2(t)
	charge, failure := ledger.reserve(uint64(len(frame)))
	if failure != "" {
		t.Fatal(failure)
	}
	owner := &privateB4RequestFrameOwner{state: privateB4RequestFrameOpen, bytes: append([]byte(nil), frame...), charge: charge}
	return B4RequestFrameLease{owner: owner}, ledger
}

func TestPrivateB4RequestFrameLeaseSynchronousRelease(t *testing.T) {
	lease, ledger := testB4RequestFrameLease(t, []byte("frame"))
	if err := lease.WithBytes(func(got []byte) error {
		if !bytes.Equal(got, []byte("frame")) {
			t.Fatalf("bytes=%q", got)
		}
		if lease.Release() {
			t.Fatal("release finalized during active borrow")
		}
		if !bytes.Equal(got, []byte("frame")) {
			t.Fatal("backing cleared before callback exit")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if ledger.snapshot().Live != privateB4ByteLedgerV2TableBytes {
		t.Fatalf("live=%d", ledger.snapshot().Live)
	}
	if err := lease.WithBytes(func([]byte) error { return nil }); !errors.Is(err, errPrivateB4RequestFrameReleased) {
		t.Fatalf("after release=%v", err)
	}
	if lease.Release() {
		t.Fatal("duplicate release finalized")
	}
}

func TestPrivateB4RequestFrameLeaseConcurrentBorrowRelease(t *testing.T) {
	lease, ledger := testB4RequestFrameLease(t, []byte("frame"))
	entered := make(chan struct{}, 2)
	releaseFirst := make(chan struct{})
	done := make(chan error, 2)
	fn := func(got []byte) error {
		entered <- struct{}{}
		<-releaseFirst
		if !bytes.Equal(got, []byte("frame")) {
			return errors.New("changed")
		}
		return nil
	}
	go func() { done <- lease.WithBytes(fn) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first borrow")
	}
	go func() { done <- lease.WithBytes(fn) }()
	deadline := time.After(time.Second)
	for {
		lease.owner.mu.Lock()
		active := lease.owner.active
		lease.owner.mu.Unlock()
		if active == 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("second borrow not accepted")
		default:
		}
	}
	if lease.Release() {
		t.Fatal("release waited/finalized")
	}
	if err := lease.WithBytes(func([]byte) error { return nil }); !errors.Is(err, errPrivateB4RequestFrameClosing) {
		t.Fatalf("closing borrow=%v", err)
	}
	close(releaseFirst)
	for i := 0; i < 2; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	if ledger.snapshot().Live != privateB4ByteLedgerV2TableBytes {
		t.Fatalf("live=%d", ledger.snapshot().Live)
	}
}

func TestPrivateB4RequestFrameLeaseCallbackErrorDoesNotRelease(t *testing.T) {
	lease, ledger := testB4RequestFrameLease(t, []byte("frame"))
	want := errors.New("callback")
	if err := lease.WithBytes(func([]byte) error { return want }); !errors.Is(err, want) {
		t.Fatalf("err=%v", err)
	}
	if ledger.snapshot().Live != privateB4ByteLedgerV2TableBytes+5 {
		t.Fatalf("live=%d", ledger.snapshot().Live)
	}
	if !lease.Release() || ledger.snapshot().Live != privateB4ByteLedgerV2TableBytes {
		t.Fatal("immediate release")
	}
}

func TestPrivateB4RequestFrameLeaseConcurrentDuplicateRelease(t *testing.T) {
	lease, ledger := testB4RequestFrameLease(t, []byte("frame"))
	var wg sync.WaitGroup
	results := make(chan bool, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- lease.Release() }()
	}
	wg.Wait()
	close(results)
	wins := 0
	for result := range results {
		if result {
			wins++
		}
	}
	if wins != 1 || ledger.snapshot().Live != privateB4ByteLedgerV2TableBytes {
		t.Fatalf("wins=%d snapshot=%+v", wins, ledger.snapshot())
	}
}

func TestPrivateB4RequestFrameLeaseIntegratedResult(t *testing.T) {
	f := newFullP1Fixture(t)
	result, definitionLease, selection := f.transact(t)
	lease, ok := result.PrivateB4RequestFrameLease()
	if !ok {
		t.Fatal("missing request-frame lease")
	}
	if _, ok := result.CompletedMethodRequestFrame(); ok {
		t.Fatal("private B4 defensive frame remained available")
	}
	if err := lease.WithBytes(func(got []byte) error {
		if !bytes.HasPrefix(got, []byte("Content-Length: ")) || !bytes.Contains(got, f.expectedParams) {
			t.Fatalf("request frame mismatch")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, status := consumePrivateB4SnapshotForTest(f.m, definitionLease, selection); status != PrivateB4Selected {
		t.Fatalf("consume=%s", status)
	}
	if err := lease.WithBytes(func([]byte) error { return nil }); err != nil {
		t.Fatalf("common cleanup released caller lease: %v", err)
	}
	if !lease.Release() {
		t.Fatal("caller release")
	}
}
