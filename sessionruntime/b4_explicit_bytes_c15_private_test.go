package sessionruntime

import (
	"math"
	"testing"

	"lsp-trace/internal/session"
)

func TestPrivateB4ParamsCopyReservationPrecedesAllocation(t *testing.T) {
	params := []byte(`{"textDocument":{"uri":"file:///w/main.go"}}`)
	ledger := mustPrivateB4ByteAccountV2(t)
	copy, lease, failure := copyPrivateB4Params(ledger, params)
	if failure != "" || !lease.lease.active || string(copy) != string(params) {
		t.Fatalf("ASSERT_C15_PARAMS_EQUALITY failure=%q lease=%+v copy=%q", failure, lease, copy)
	}
	if got := ledger.snapshot(); got != (privateB4ByteLedgerSnapshot{Live: privateB4ByteLedgerV2TableBytes + uint64(len(params)), Cumulative: privateB4ByteLedgerV2TableBytes + uint64(len(params))}) {
		t.Fatalf("ASSERT_C15_PARAMS_EXACT snapshot=%+v", got)
	}
	params[0] ^= 1
	if copy[0] == params[0] {
		t.Fatal("ASSERT_C15_PARAMS_INDEPENDENT_COPY")
	}
	lease.release()
	if ledger.snapshot().Live != privateB4ByteLedgerV2TableBytes {
		t.Fatalf("ASSERT_C15_PARAMS_RELEASE snapshot=%+v", ledger.snapshot())
	}

	filler, failure := ledger.reserve(privateB4MaxOwnedBytes - privateB4ByteLedgerV2TableBytes - uint64(len(params)) + 1)
	if failure != "" {
		t.Fatal(failure)
	}
	before := ledger.snapshot()
	if refused, refusedLease, failure := copyPrivateB4Params(ledger, params); failure != session.ResourceExhausted || refused != nil || refusedLease.lease.active {
		t.Fatalf("ASSERT_C15_PARAMS_PLUS_ONE failure=%q lease=%+v copy=%q", failure, refusedLease, refused)
	}
	if after := ledger.snapshot(); after != before {
		t.Fatalf("ASSERT_C15_PARAMS_ATOMIC before=%+v after=%+v", before, after)
	}
	filler.release()
}

func TestPrivateB4OutboundTransportReservationExactAndAtomic(t *testing.T) {
	ledger := mustPrivateB4ByteAccountV2(t)
	bodyBytes := 123
	want := uint64(canonicalRequestFrameBytes(bodyBytes))
	lease, failure := ledger.reservePrivateB4OutboundTransport(bodyBytes)
	if failure != "" || !lease.lease.active || ledger.snapshot() != (privateB4ByteLedgerSnapshot{Live: privateB4ByteLedgerV2TableBytes + want, Cumulative: privateB4ByteLedgerV2TableBytes + want}) {
		t.Fatalf("ASSERT_C15_OUTBOUND_EXACT failure=%q snapshot=%+v", failure, ledger.snapshot())
	}
	lease.release()
	if got := ledger.snapshot(); got.Live != privateB4ByteLedgerV2TableBytes || got.Cumulative != privateB4ByteLedgerV2TableBytes+want {
		t.Fatalf("ASSERT_C15_OUTBOUND_RELEASE snapshot=%+v", got)
	}

	filler, failure := ledger.reserve(privateB4MaxOwnedBytes - privateB4ByteLedgerV2TableBytes - want + 1)
	if failure != "" {
		t.Fatal(failure)
	}
	before := ledger.snapshot()
	if refused, failure := ledger.reservePrivateB4OutboundTransport(bodyBytes); failure != session.ResourceExhausted || refused.lease.active {
		t.Fatalf("ASSERT_C15_OUTBOUND_PLUS_ONE failure=%q lease=%+v", failure, refused)
	}
	if after := ledger.snapshot(); after != before {
		t.Fatalf("ASSERT_C15_OUTBOUND_ATOMIC before=%+v after=%+v", before, after)
	}
	filler.release()
}

func TestPrivateB4C15LedgerV2DedicatedDescriptorAndGeneralBoundary(t *testing.T) {
	ledger, failure := newPrivateB4ByteLedgerV2()
	if failure != "" {
		t.Fatal(failure)
	}
	if got := ledger.general.snapshot(); got != (privateB4ByteLedgerSnapshot{Live: privateB4ByteLedgerV2TableBytes, Cumulative: privateB4ByteLedgerV2TableBytes}) {
		t.Fatalf("ASSERT_C15_V2_TABLE_BACKING snapshot=%+v", got)
	}
	descriptor, failure := ledger.reserveDescriptor(17)
	if failure != "" || !descriptor.active {
		t.Fatalf("ASSERT_C15_V2_DESCRIPTOR failure=%q lease=%+v", failure, descriptor)
	}
	if got := ledger.general.snapshot(); got != (privateB4ByteLedgerSnapshot{Live: privateB4ByteLedgerV2TableBytes + 17, Cumulative: privateB4ByteLedgerV2TableBytes}) {
		t.Fatalf("ASSERT_C15_V2_TRANSFER_CUMULATIVE snapshot=%+v", got)
	}
	var general [privateB4ByteSlots]privateB4ByteLease
	for i := range general {
		general[i], failure = ledger.general.reserve(1)
		if failure != "" {
			t.Fatalf("ASSERT_C15_V2_GENERAL_%d failure=%q", i, failure)
		}
	}
	before := ledger.general.snapshot()
	if _, failure = ledger.general.reserve(1); failure != session.ResourceExhausted {
		t.Fatalf("ASSERT_C15_V2_GENERAL_PLUS_ONE failure=%q", failure)
	}
	if ledger.general.snapshot() != before || !descriptor.active {
		t.Fatal("ASSERT_C15_V2_DEDICATED_ISOLATION")
	}
	for i := range general {
		general[i].release()
	}
	descriptor.release()
	if !ledger.releaseTableBacking() || ledger.general.snapshot() != (privateB4ByteLedgerSnapshot{Live: 0, Cumulative: privateB4ByteLedgerV2TableBytes + privateB4ByteSlots}) {
		t.Fatalf("ASSERT_C15_V2_EXACT_RELEASE snapshot=%+v", ledger.general.snapshot())
	}
}

func TestPrivateB4C15LedgerV2EqualityAndPlusOne(t *testing.T) {
	ledger, failure := newPrivateB4ByteLedgerV2()
	if failure != "" {
		t.Fatal(failure)
	}
	payload := privateB4MaxOwnedBytes - privateB4ByteLedgerV2TableBytes
	lease, failure := ledger.general.reserve(payload)
	if failure != "" || ledger.general.snapshot().Live != privateB4MaxOwnedBytes {
		t.Fatalf("ASSERT_C15_V2_EQUALITY failure=%q snapshot=%+v", failure, ledger.general.snapshot())
	}
	before := ledger.general.snapshot()
	if _, failure = ledger.reserveDescriptor(1); failure != session.ResourceExhausted || ledger.general.snapshot() != before {
		t.Fatalf("ASSERT_C15_V2_PLUS_ONE failure=%q before=%+v after=%+v", failure, before, ledger.general.snapshot())
	}
	lease.release()
	if !ledger.releaseTableBacking() || ledger.general.snapshot() != (privateB4ByteLedgerSnapshot{Live: 0, Cumulative: privateB4MaxOwnedBytes}) {
		t.Fatalf("ASSERT_C15_V2_TABLE_RELEASE snapshot=%+v", ledger.general.snapshot())
	}
}

func TestPrivateB4C15LedgerExactCapAndPlusOneZeroEffect(t *testing.T) {
	ledger := newPrivateB4ByteLedger()
	atCap, failure := ledger.reserve(33_554_432)
	if failure != "" {
		t.Fatalf("ASSERT_C15_EXACT_CAP_ALLOWED failure=%s", failure)
	}
	before := ledger.snapshot()
	if _, failure = ledger.reserve(1); failure != session.ResourceExhausted {
		t.Fatalf("ASSERT_C15_PLUS_ONE_REFUSED failure=%s", failure)
	}
	if after := ledger.snapshot(); after != before {
		t.Fatalf("ASSERT_C15_REFUSAL_ZERO_EFFECT before=%+v after=%+v", before, after)
	}
	atCap.release()
	if got := ledger.snapshot().Live; got != 0 {
		t.Fatalf("ASSERT_C15_RELEASE_RESTORES live=%d", got)
	}
}

func TestPrivateB4C15LedgerAliasIndependentAndExactOnceRelease(t *testing.T) {
	ledger := newPrivateB4ByteLedger()
	first, failure := ledger.reserve(11)
	if failure != "" {
		t.Fatal(failure)
	}
	alias := first.alias()
	if got := ledger.snapshot().Live; got != 11 {
		t.Fatalf("ASSERT_C15_ALIAS_CHARGED_ONCE live=%d", got)
	}
	second, failure := ledger.reserve(11)
	if failure != "" {
		t.Fatal(failure)
	}
	if got := ledger.snapshot().Live; got != 22 {
		t.Fatalf("ASSERT_C15_INDEPENDENT_COPY_CHARGED_TWICE live=%d", got)
	}
	alias.release()
	first.release()
	first.release()
	if got := ledger.snapshot().Live; got != 11 {
		t.Fatalf("ASSERT_C15_REPEATED_RELEASE_EXACT_ONCE live=%d", got)
	}
	second.release()
}

func TestPrivateB4C15LedgerGrowthPeakAndTransferContinuity(t *testing.T) {
	ledger := newPrivateB4ByteLedger()
	old, failure := ledger.reserve(9)
	if failure != "" {
		t.Fatal(failure)
	}
	grown, peak, failure := old.grow(13)
	if failure != "" {
		t.Fatal(failure)
	}
	if peak != 22 || ledger.snapshot().Live != 13 {
		t.Fatalf("ASSERT_C15_GROWTH_PEAK_OVERLAP peak=%d snapshot=%+v", peak, ledger.snapshot())
	}
	before := ledger.snapshot().Live
	moved := grown.transfer()
	if after := ledger.snapshot().Live; after != before {
		t.Fatalf("ASSERT_C15_TRANSFER_NO_GAP before=%d after=%d", before, after)
	}
	grown.release()
	if got := ledger.snapshot().Live; got != before {
		t.Fatalf("ASSERT_C15_TRANSFER_SOURCE_RELEASE_INERT live=%d", got)
	}
	moved.release()
}

func TestPrivateB4C15LedgerCumulativeDoesNotEnforceCapacity(t *testing.T) {
	ledger := newPrivateB4ByteLedger()
	for i := 0; i < 40; i++ {
		lease, failure := ledger.reserve(1 << 20)
		if failure != "" {
			t.Fatalf("ASSERT_C15_HIGH_CUMULATIVE_ALLOWED iteration=%d failure=%s", i, failure)
		}
		lease.release()
	}
	snapshot := ledger.snapshot()
	if snapshot.Live != 0 || snapshot.Cumulative != 40<<20 {
		t.Fatalf("ASSERT_C15_CUMULATIVE_OBSERVATION snapshot=%+v", snapshot)
	}
}

func TestPrivateB4C15LedgerOverflowRejectsBeforeMutation(t *testing.T) {
	ledger := newPrivateB4ByteLedger()
	ledger.live = math.MaxUint64
	ledger.cumulative = math.MaxUint64
	before := ledger.snapshot()
	if _, failure := ledger.reserve(1); failure != session.ResourceExhausted {
		t.Fatalf("ASSERT_C15_OVERFLOW_REFUSED failure=%s", failure)
	}
	if after := ledger.snapshot(); after != before {
		t.Fatalf("ASSERT_C15_OVERFLOW_ZERO_EFFECT before=%+v after=%+v", before, after)
	}
}
