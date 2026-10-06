package sessionruntime

import "testing"

func TestC06PrivateB4UsesManagerDerivedThreeEntryReceipt(t *testing.T) {
	f := newFullP1Fixture(t)
	result, lease, selection := f.transact(t)
	if result.Failure != "" || lease == (B4DefinitionLease{}) {
		t.Fatalf("ASSERT_C06_B4_SAME_PATH: failure=%s", result.Failure)
	}
	f.m.mu.Lock()
	reservation := f.m.privateB4ReservationLocked(lease.token)
	if reservation == nil || reservation.historyMetadata == nil {
		f.m.mu.Unlock()
		t.Fatal("ASSERT_C06_B4_MANAGER_RECEIPT: missing")
	}
	metadata := *reservation.historyMetadata
	borrowers := reservation.history.borrowers
	f.m.mu.Unlock()
	if metadata.Entries != 3 || metadata.CutOrdinal != 3 || metadata.Identity == "" || metadata.Cut == "" || borrowers != 1 {
		t.Fatalf("ASSERT_C06_B4_MANAGER_RECEIPT: metadata=%+v borrowers=%d", metadata, borrowers)
	}
	if _, status := consumePrivateB4SnapshotForTest(f.m, lease, selection); status != PrivateB4Selected {
		t.Fatalf("ASSERT_C06_B4_RELEASE: %s", status)
	}
	f.m.mu.Lock()
	borrowers = reservation.history.borrowers
	f.m.mu.Unlock()
	if borrowers != 0 {
		t.Fatalf("ASSERT_C06_B4_RELEASE: borrowers=%d", borrowers)
	}
}
