package sessionruntime

import (
	"bytes"
	"sync"
	"testing"
	"time"
)

func TestPrivateB4ResultLeaseIntegratedCustody(t *testing.T) {
	f := newFullP1Fixture(t)
	result, managerLease, selection := f.transact(t)
	if len(result.Result) != 0 {
		t.Fatalf("private result exposed directly: %d bytes", len(result.Result))
	}
	lease, ok := result.PrivateB4ResultLease()
	if !ok {
		t.Fatal("missing private result lease")
	}
	if err := lease.WithBytes(func(got []byte) error {
		if !bytes.Equal(got, f.expectedResult) {
			t.Fatalf("result bytes mismatch")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, status := consumePrivateB4SnapshotForTest(f.m, managerLease, selection); status != PrivateB4Selected {
		t.Fatalf("consume status=%s", status)
	}
	if err := lease.WithBytes(func([]byte) error { return nil }); err != nil {
		t.Fatalf("manager-first release invalidated caller: %v", err)
	}
	if !lease.Release() || lease.Release() {
		t.Fatal("caller release was not exact-once")
	}
}

func TestPrivateB4ResultLeaseReleaseIsNonblockingAndSerialized(t *testing.T) {
	ledger := mustPrivateB4ByteAccountV2(t)
	charge, failure := ledger.reserve(3)
	if failure != "" {
		t.Fatal(failure)
	}
	owner := &privateB4ResultOwner{
		state: privateB4ResultOpen, managerHeld: true, callerHeld: true,
		bytes: []byte("abc"), charge: &privateB4SuccessorAllocationLease{lease: charge},
	}
	lease := B4ResultLease{owner: owner}
	entered, unblock, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		_ = lease.WithBytes(func([]byte) error {
			close(entered)
			<-unblock
			return nil
		})
		close(done)
	}()
	<-entered
	start := time.Now()
	if lease.Release() || time.Since(start) > 100*time.Millisecond {
		t.Fatal("release waited for active callback")
	}
	owner.releaseManager()
	if ledger.snapshot().Live != privateB4ByteLedgerV2TableBytes+3 {
		t.Fatal("charge released while callback active")
	}
	close(unblock)
	<-done
	if ledger.snapshot().Live != privateB4ByteLedgerV2TableBytes {
		t.Fatal("charge not released after final callback")
	}
	if err := lease.WithBytes(func([]byte) error { return nil }); err != errPrivateB4ResultReleased {
		t.Fatalf("post-release borrow err=%v", err)
	}
}

func TestPrivateB4BorrowedPrepareExcludesResultCopyAndDoesNotConsume(t *testing.T) {
	f := newFullP1Fixture(t)
	result, managerLease, selection := f.transact(t)
	resultLease, _ := privateB4C15ResultLease(t, result)
	seen := false
	capture, status := f.m.PreparePrivateB4DefinitionBorrowed(managerLease, selection, func(b PrivateB4DefinitionBorrow) bool {
		seen = len(b.Capture.Result) == 0 && bytes.Equal(b.Result, f.expectedResult)
		return seen
	})
	if status != PrivateB4Selected || !seen || len(capture.Result) != 0 {
		t.Fatalf("borrowed prepare status=%s seen=%v result=%d", status, seen, len(capture.Result))
	}
	if _, status := f.m.ConsumePrivateB4DefinitionBorrowed(managerLease, selection, func(b PrivateB4DefinitionBorrow) bool {
		return bytes.Equal(b.Result, f.expectedResult)
	}); status != PrivateB4Selected {
		t.Fatalf("post-prepare consume status=%s", status)
	}
	resultLease.Release()
}

func TestPrivateB4BorrowedCommitExcludesResultCopyAndRetries(t *testing.T) {
	f := newFullP1Fixture(t)
	result, managerLease, selection := f.transact(t)
	callerLease, resultBytes := privateB4C15ResultLease(t, result)
	f.m.mu.Lock()
	reservation := f.m.privateB4ReservationLocked(managerLease.token)
	before := reservation.explicitBytes.snapshot()
	beforeEntries := reservation.explicitLeaseCount
	f.m.mu.Unlock()

	var firstCharge uint64
	if _, status := f.m.CommitPrivateB4DefinitionBorrowed(managerLease, selection, func(b PrivateB4DefinitionBorrow) bool {
		if len(b.Capture.Result) != 0 || !bytes.Equal(b.Result, f.expectedResult) {
			t.Fatal("borrowed result mismatch or copied capture result")
		}
		firstCharge = uint64(len(b.Capture.RequestFrame) + len(b.Capture.RequestParams) + len(b.Capture.ResponseFrame))
		for i := range b.Capture.TargetSources {
			firstCharge += uint64(len(b.Capture.TargetSources[i].Bytes))
		}
		return false
	}, func(PrivateB4DefinitionBorrow) { t.Fatal("rejected prepare published") }); status != PrivateB4Unavailable {
		t.Fatalf("rejected status=%s", status)
	}
	if got := reservation.explicitBytes.snapshot(); got.Live != before.Live+firstCharge || got.Cumulative != before.Cumulative+firstCharge || reservation.explicitLeaseCount != beforeEntries+4 {
		t.Fatalf("borrowed prepare charge=%+v before=%+v entries=%d/%d metadata=%d removed-result=%d", got, before, reservation.explicitLeaseCount, beforeEntries, firstCharge, resultBytes)
	}
	published := 0
	if _, status := f.m.CommitPrivateB4DefinitionBorrowed(managerLease, selection, func(b PrivateB4DefinitionBorrow) bool {
		return bytes.Equal(b.Result, f.expectedResult)
	}, func(PrivateB4DefinitionBorrow) { published++ }); status != PrivateB4Selected || published != 1 {
		t.Fatalf("retry status=%s published=%d", status, published)
	}
	if !callerLease.Release() {
		t.Fatal("caller result release")
	}
}

func TestPrivateB4ResultLeaseCallbacksSerialize(t *testing.T) {
	owner := &privateB4ResultOwner{state: privateB4ResultOpen, callerHeld: true, bytes: []byte("x")}
	lease := B4ResultLease{owner: owner}
	var mu sync.Mutex
	active, maximum := 0, 0
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = lease.WithBytes(func([]byte) error {
				mu.Lock()
				active++
				if active > maximum {
					maximum = active
				}
				mu.Unlock()
				time.Sleep(time.Millisecond)
				mu.Lock()
				active--
				mu.Unlock()
				return nil
			})
		}()
	}
	wg.Wait()
	if maximum != 1 {
		t.Fatalf("maximum concurrent callbacks=%d", maximum)
	}
	lease.Release()
}
