package sessionruntime

import (
	"bytes"
	"testing"
)

func TestPrivateB4ResponseFrameLeaseSharesOneChargeAndReleasesLastOwner(t *testing.T) {
	ledger := mustPrivateB4ByteAccountV2(t)
	charge, failure := ledger.reserve(5)
	if failure != "" {
		t.Fatal(failure)
	}
	owner := &privateB4ResponseFrameOwner{
		state: privateB4ResponseFrameOpen, managerHeld: true, callerHeld: true,
		bytes: []byte("frame"), charge: &privateB4SuccessorAllocationLease{lease: charge},
	}
	result := RoundTripResult{privateB4ResponseFrame: owner}
	lease, ok := result.PrivateB4ResponseFrameLease()
	if !ok {
		t.Fatal("ASSERT_C15_RESPONSE_FRAME_CALLER_LEASE")
	}
	if err := lease.WithBytes(func(frame []byte) error {
		if !bytes.Equal(frame, []byte("frame")) || ledger.snapshot().Live != privateB4ByteLedgerV2TableBytes+5 {
			t.Fatalf("ASSERT_C15_RESPONSE_FRAME_ALIAS frame=%q snapshot=%+v", frame, ledger.snapshot())
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	owner.releaseManager()
	if ledger.snapshot().Live != privateB4ByteLedgerV2TableBytes+5 {
		t.Fatal("ASSERT_C15_RESPONSE_FRAME_CALLER_EXTENDS_CUSTODY")
	}
	if !lease.Release() || ledger.snapshot().Live != privateB4ByteLedgerV2TableBytes {
		t.Fatalf("ASSERT_C15_RESPONSE_FRAME_FINAL_RELEASE snapshot=%+v", ledger.snapshot())
	}
	if lease.Release() {
		t.Fatal("ASSERT_C15_RESPONSE_FRAME_EXACT_ONCE")
	}
}
