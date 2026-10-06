package sessionruntime

import (
	"bytes"
	"testing"
	"unsafe"

	"lsp-trace/internal/session"
)

func privateB4C15FinalCaptureBytes(c PrivateB4DefinitionCapture) uint64 {
	total := uint64(len(c.RequestFrame) + len(c.RequestParams) + len(c.ResponseFrame))
	for i := range c.TargetSources {
		total += uint64(len(c.TargetSources[i].Bytes))
	}
	return total
}

func privateB4C15ResponseFrameLease(t *testing.T, result RoundTripResult) (B4ResponseFrameLease, uint64) {
	t.Helper()
	lease, ok := result.PrivateB4ResponseFrameLease()
	if !ok {
		t.Fatal("ASSERT_C15_CALLER_RESPONSE_FRAME_LEASE")
	}
	var charge uint64
	if err := lease.WithBytes(func(frame []byte) error { charge = uint64(len(frame)); return nil }); err != nil {
		t.Fatal(err)
	}
	return lease, charge
}

func privateB4C15ResultLease(t *testing.T, result RoundTripResult) (B4ResultLease, uint64) {
	t.Helper()
	lease, ok := result.PrivateB4ResultLease()
	if !ok {
		t.Fatal("ASSERT_C15_CALLER_RESULT_LEASE")
	}
	var charge uint64
	if err := lease.WithBytes(func(result []byte) error { charge = uint64(len(result)); return nil }); err != nil {
		t.Fatal(err)
	}
	return lease, charge
}

func TestPrivateB4C15SourceAndFinalCopiesChargeCallbackAndRelease(t *testing.T) {
	f := newFullP1Fixture(t)
	result, lease, selection := f.transact(t)
	frameLease, ok := result.PrivateB4RequestFrameLease()
	if !ok {
		t.Fatal("ASSERT_C15_CALLER_FRAME_LEASE")
	}
	var frameCharge uint64
	if err := frameLease.WithBytes(func(frame []byte) error { frameCharge = uint64(len(frame)); return nil }); err != nil {
		t.Fatal(err)
	}
	responseLease, responseCharge := privateB4C15ResponseFrameLease(t, result)
	resultLease, resultCharge := privateB4C15ResultLease(t, result)

	f.m.mu.Lock()
	reservation := f.m.privateB4ReservationLocked(lease.token)
	if reservation == nil || reservation.explicitBytes == nil {
		f.m.mu.Unlock()
		t.Fatal("ASSERT_C15_RESERVATION_LEDGER_ATTACHED")
	}
	sourceCharge := uint64(len(f.source.Bytes))
	descriptorCharge := privateB4SourceIngressStateBytes
	sourceHandleCharge := uint64(len(f.owner.TargetSources)) * uint64(unsafe.Sizeof(B4DefinitionSourceLease{}))
	captureParamsCharge := uint64(len(f.req.Params))
	persistentCharge := sourceHandleCharge + captureParamsCharge
	if got := reservation.explicitBytes.snapshot(); got.Live != privateB4ByteLedgerV2TableBytes+descriptorCharge+sourceCharge+persistentCharge+frameCharge+responseCharge+resultCharge || got.Cumulative < privateB4ByteLedgerV2TableBytes+sourceCharge+persistentCharge+frameCharge+responseCharge+resultCharge {
		f.m.mu.Unlock()
		t.Fatalf("ASSERT_C15_SOURCE_COPY_CHARGED_SEPARATELY snapshot=%+v want=%d", got, sourceCharge)
	}
	f.m.mu.Unlock()

	prepared, status := preparePrivateB4SnapshotForTest(f.m, lease, selection)
	if status != PrivateB4Selected {
		t.Fatalf("ASSERT_C15_PREPARE_SELECTED status=%s", status)
	}
	finalCharge := privateB4C15FinalCaptureBytes(prepared)
	f.m.mu.Lock()
	reservation = f.m.privateB4ReservationLocked(lease.token)
	preparedSnapshot := reservation.explicitBytes.snapshot()
	f.m.mu.Unlock()
	if preparedSnapshot.Live != privateB4ByteLedgerV2TableBytes+descriptorCharge+sourceCharge+persistentCharge+frameCharge+responseCharge+resultCharge+finalCharge || preparedSnapshot.Cumulative < privateB4ByteLedgerV2TableBytes+sourceCharge+persistentCharge+frameCharge+responseCharge+resultCharge+finalCharge {
		t.Fatalf("ASSERT_C15_FINAL_COPIES_CHARGED_SEPARATELY snapshot=%+v source=%d final=%d", preparedSnapshot, sourceCharge, finalCharge)
	}

	prepared.RequestParams[0] ^= 1
	prepared.TargetSources[0].Bytes[0] ^= 1
	if bytes.Equal(prepared.RequestParams, f.expectedParams) || bytes.Equal(prepared.TargetSources[0].Bytes, f.source.Bytes) {
		t.Fatal("ASSERT_C15_FINAL_CAPTURE_INDEPENDENT_COPIES")
	}
	f.m.mu.Lock()
	if got := reservation.explicitBytes.snapshot(); got != preparedSnapshot {
		f.m.mu.Unlock()
		t.Fatalf("ASSERT_C15_CALLER_ALIASES_NO_RECHARGE before=%+v after=%+v", preparedSnapshot, got)
	}
	f.m.mu.Unlock()

	if _, status = commitPrivateB4SnapshotForTest(f.m, lease, selection, func(PrivateB4DefinitionCapture) bool { return false }, func(PrivateB4DefinitionCapture) {
		t.Fatal("ASSERT_C15_PREPARE_FAILURE_NO_PUBLISH")
	}); status != PrivateB4Unavailable {
		t.Fatalf("ASSERT_C15_PREPARE_FAILURE_STATUS status=%s", status)
	}
	f.m.mu.Lock()
	failedSnapshot := reservation.explicitBytes.snapshot()
	f.m.mu.Unlock()
	if failedSnapshot.Live != privateB4ByteLedgerV2TableBytes+descriptorCharge+sourceCharge+persistentCharge+frameCharge+responseCharge+resultCharge+2*finalCharge || failedSnapshot.Cumulative < privateB4ByteLedgerV2TableBytes+sourceCharge+persistentCharge+frameCharge+responseCharge+resultCharge+2*finalCharge {
		t.Fatalf("ASSERT_C15_PREPARE_FAILURE_RETAINS_CHARGE snapshot=%+v", failedSnapshot)
	}

	var callbackSnapshot privateB4ByteLedgerSnapshot
	if _, status = commitPrivateB4SnapshotForTest(f.m, lease, selection, func(PrivateB4DefinitionCapture) bool { return true }, func(PrivateB4DefinitionCapture) {
		callbackSnapshot = reservation.explicitBytes.snapshot()
	}); status != PrivateB4Selected {
		t.Fatalf("ASSERT_C15_RETRY_COMMIT status=%s", status)
	}
	wantCallback := privateB4ByteLedgerV2TableBytes + descriptorCharge + sourceCharge + persistentCharge + frameCharge + responseCharge + resultCharge + 3*finalCharge
	if callbackSnapshot.Live != wantCallback || callbackSnapshot.Cumulative < wantCallback-descriptorCharge {
		t.Fatalf("ASSERT_C15_CALLBACK_OBSERVES_LIVE_EXPLICIT_CHARGE snapshot=%+v want=%d", callbackSnapshot, wantCallback)
	}
	if got := reservation.explicitBytes.snapshot(); got.Live != privateB4ByteLedgerV2TableBytes+frameCharge+responseCharge+resultCharge || got.Cumulative < wantCallback-descriptorCharge {
		t.Fatalf("ASSERT_C15_COMMIT_PRESERVES_CALLER_LEASES snapshot=%+v", got)
	}
	if !frameLease.Release() || !responseLease.Release() || !resultLease.Release() {
		t.Fatal("ASSERT_C15_CALLER_LEASE_RELEASE")
	}
	if got := reservation.explicitBytes.snapshot(); got.Live != 0 || got.Cumulative < wantCallback-descriptorCharge {
		t.Fatalf("ASSERT_C15_COMMIT_RELEASE_EXACT_ONCE snapshot=%+v", got)
	}
	if _, replay := consumePrivateB4SnapshotForTest(f.m, lease, selection); replay != PrivateB4Unavailable {
		t.Fatalf("ASSERT_C15_CONSUME_REPLAY status=%s", replay)
	}
	if got := reservation.explicitBytes.snapshot(); got.Live != 0 || got.Cumulative < wantCallback-descriptorCharge {
		t.Fatalf("ASSERT_C15_REPLAY_RELEASE_INERT snapshot=%+v", got)
	}
}

func TestPrivateB4C15SourceRefusalPrecedesCopyAndMutation(t *testing.T) {
	f := newFullP1Fixture(t)
	reservation := &privateB4Reservation{
		sourceLeases:  append([]B4DefinitionSourceLease(nil), f.owner.TargetSources...),
		explicitBytes: mustPrivateB4ByteAccountV2(t),
	}
	reservation.token[0] = 99
	sourceBytes := f.owner.TargetSources[0].state.source.Bytes
	beforeSource := append([]byte(nil), sourceBytes...)
	filler, failure := reservation.explicitBytes.reserve(privateB4MaxOwnedBytes - privateB4ByteLedgerV2TableBytes - uint64(len(sourceBytes)) + 1)
	if failure != "" {
		t.Fatal(failure)
	}
	defer filler.release()
	beforeLedger := reservation.explicitBytes.snapshot()
	beforeManagerBytes := f.m.privateB4Bytes
	beforeWrites := f.child.writes.Load()

	f.m.mu.Lock()
	failure = f.m.admitPrivateB4SourcesLocked(f.req, reservation)
	f.m.mu.Unlock()
	if failure != session.ResourceExhausted {
		t.Fatalf("ASSERT_C15_SOURCE_REFUSAL failure=%s", failure)
	}
	if after := reservation.explicitBytes.snapshot(); after != beforeLedger {
		t.Fatalf("ASSERT_C15_SOURCE_REFUSAL_LEDGER_ZERO_EFFECT before=%+v after=%+v", beforeLedger, after)
	}
	if reservation.capture.TargetSources != nil || reservation.documentsAcquired != 0 || reservation.admittedLengths != nil ||
		f.m.privateB4Bytes != beforeManagerBytes || f.child.writes.Load() != beforeWrites ||
		f.owner.TargetSources[0].state.state.Load() != privateB4SourceHeld || !bytes.Equal(sourceBytes, beforeSource) {
		t.Fatalf("ASSERT_C15_SOURCE_REFUSAL_STATE_ZERO_EFFECT capture=%d documents=%d lengths=%v manager=%d/%d writes=%d/%d state=%d",
			len(reservation.capture.TargetSources), reservation.documentsAcquired, reservation.admittedLengths,
			f.m.privateB4Bytes, beforeManagerBytes, f.child.writes.Load(), beforeWrites, f.owner.TargetSources[0].state.state.Load())
	}
}

func TestPrivateB4C15FinalCaptureRefusalPrecedesCopyAndMutation(t *testing.T) {
	f := newFullP1Fixture(t)
	result, lease, selection := f.transact(t)
	frameLease, ok := result.PrivateB4RequestFrameLease()
	if !ok {
		t.Fatal("ASSERT_C15_FINAL_CALLER_FRAME_LEASE")
	}
	var frameCharge uint64
	if err := frameLease.WithBytes(func(frame []byte) error { frameCharge = uint64(len(frame)); return nil }); err != nil {
		t.Fatal(err)
	}
	responseLease, responseCharge := privateB4C15ResponseFrameLease(t, result)
	resultLease, resultCharge := privateB4C15ResultLease(t, result)
	f.m.mu.Lock()
	reservation := f.m.privateB4ReservationLocked(lease.token)
	original := reservation.capture
	finalBytes := privateB4C15FinalCaptureBytes(original)
	filler, failure := reservation.explicitBytes.reserve(privateB4MaxOwnedBytes - reservation.explicitBytes.snapshot().Live - finalBytes + 1)
	if failure != "" {
		f.m.mu.Unlock()
		t.Fatal(failure)
	}
	before := reservation.explicitBytes.snapshot()
	f.m.mu.Unlock()

	publishCalls := 0
	capture, status := commitPrivateB4SnapshotForTest(f.m, lease, selection, func(PrivateB4DefinitionCapture) bool { return true }, func(PrivateB4DefinitionCapture) {
		publishCalls++
	})
	if status != PrivateB4Unavailable || capture.SessionID != "" || publishCalls != 0 {
		t.Fatalf("ASSERT_C15_FINAL_REFUSAL status=%s capture=%+v publishes=%d", status, capture, publishCalls)
	}
	if after := reservation.explicitBytes.snapshot(); after != before {
		t.Fatalf("ASSERT_C15_FINAL_REFUSAL_LEDGER_ZERO_EFFECT before=%+v after=%+v", before, after)
	}
	if &reservation.capture.RequestParams[0] != &original.RequestParams[0] ||
		&reservation.capture.ResponseFrame[0] != &original.ResponseFrame[0] || &reservation.capture.TargetSources[0].Bytes[0] != &original.TargetSources[0].Bytes[0] {
		t.Fatal("ASSERT_C15_FINAL_REFUSAL_COPY_ZERO_EFFECT")
	}
	filler.release()
	if _, status = commitPrivateB4SnapshotForTest(f.m, lease, selection, func(PrivateB4DefinitionCapture) bool { return true }, func(PrivateB4DefinitionCapture) {
		publishCalls++
	}); status != PrivateB4Selected || publishCalls != 1 {
		t.Fatalf("ASSERT_C15_FINAL_RELEASE_RETRY status=%s publishes=%d", status, publishCalls)
	}
	if got := reservation.explicitBytes.snapshot(); got.Live != privateB4ByteLedgerV2TableBytes+frameCharge+responseCharge+resultCharge {
		t.Fatalf("ASSERT_C15_FINAL_COMMON_RELEASE_PRESERVES_CALLER snapshot=%+v", got)
	}
	if !frameLease.Release() || !responseLease.Release() || !resultLease.Release() || reservation.explicitBytes.snapshot().Live != 0 {
		t.Fatalf("ASSERT_C15_FINAL_RELEASE_RETRY_CLEANUP snapshot=%+v", reservation.explicitBytes.snapshot())
	}
}

func TestPrivateB4C15RepeatedPrepareMetadataExhaustionAtomic(t *testing.T) {
	f := newFullP1Fixture(t)
	result, lease, selection := f.transact(t)
	frameLease, ok := result.PrivateB4RequestFrameLease()
	if !ok {
		t.Fatal("ASSERT_C15_METADATA_CALLER_FRAME_LEASE")
	}
	var frameCharge uint64
	if err := frameLease.WithBytes(func(frame []byte) error { frameCharge = uint64(len(frame)); return nil }); err != nil {
		t.Fatal(err)
	}
	responseLease, responseCharge := privateB4C15ResponseFrameLease(t, result)
	resultLease, resultCharge := privateB4C15ResultLease(t, result)
	f.m.mu.Lock()
	reservation := f.m.privateB4ReservationLocked(lease.token)
	sourceSnapshot := reservation.explicitBytes.snapshot()
	f.m.mu.Unlock()

	initialLeaseCount := reservation.explicitLeaseCount
	initialActiveSlots := 0
	reservation.explicitBytes.mu.Lock()
	for i := range reservation.explicitBytes.table.general.entries {
		if reservation.explicitBytes.table.general.entries[i].active {
			initialActiveSlots++
		}
	}
	reservation.explicitBytes.mu.Unlock()
	leasesPerPrepare := 3 + len(reservation.capture.TargetSources)
	successfulPrepares := (privateB4ByteSlots - initialActiveSlots) / leasesPerPrepare
	var oneCharge uint64
	for i := 0; i < successfulPrepares; i++ {
		capture, status := preparePrivateB4SnapshotForTest(f.m, lease, selection)
		if status != PrivateB4Selected {
			t.Fatalf("ASSERT_C15_METADATA_PREPARE_%d status=%s", i, status)
		}
		charge := privateB4C15FinalCaptureBytes(capture)
		if i == 0 {
			oneCharge = charge
		} else if charge != oneCharge {
			t.Fatalf("ASSERT_C15_METADATA_STABLE_CHARGE got=%d want=%d", charge, oneCharge)
		}
	}
	before := reservation.explicitBytes.snapshot()
	wantLeaseCount := initialLeaseCount + successfulPrepares*leasesPerPrepare
	if reservation.explicitLeaseCount != wantLeaseCount || before.Live != sourceSnapshot.Live+uint64(successfulPrepares)*oneCharge || before.Live >= privateB4MaxOwnedBytes {
		t.Fatalf("ASSERT_C15_METADATA_EXACT_BOUNDARY leases=%d/%d prepares=%d snapshot=%+v source=%+v charge=%d", reservation.explicitLeaseCount, wantLeaseCount, successfulPrepares, before, sourceSnapshot, oneCharge)
	}
	capture, status := preparePrivateB4SnapshotForTest(f.m, lease, selection)
	if status != PrivateB4Unavailable || capture.SessionID != "" {
		t.Fatalf("ASSERT_C15_METADATA_EXHAUSTION status=%s capture=%+v", status, capture)
	}
	if after := reservation.explicitBytes.snapshot(); after != before || reservation.explicitLeaseCount != wantLeaseCount {
		t.Fatalf("ASSERT_C15_METADATA_ATOMIC_REFUSAL before=%+v after=%+v leases=%d", before, after, reservation.explicitLeaseCount)
	}
	f.m.mu.Lock()
	f.m.releasePrivateB4Locked(reservation)
	f.m.mu.Unlock()
	if got := reservation.explicitBytes.snapshot(); got.Live != privateB4ByteLedgerV2TableBytes+frameCharge+responseCharge+resultCharge || got.Cumulative != before.Cumulative {
		t.Fatalf("ASSERT_C15_METADATA_COMMON_RELEASE_PRESERVES_CALLER snapshot=%+v", got)
	}
	if !frameLease.Release() || !responseLease.Release() || !resultLease.Release() {
		t.Fatal("ASSERT_C15_METADATA_CALLER_RELEASE")
	}
	if got := reservation.explicitBytes.snapshot(); got.Live != 0 || got.Cumulative != before.Cumulative {
		t.Fatalf("ASSERT_C15_METADATA_RELEASE_CLEANUP snapshot=%+v", got)
	}
}
