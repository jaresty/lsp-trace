package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

func privateB4OutboundFrameBytes(t *testing.T, req RoundTripRequest) uint64 {
	t.Helper()
	body, err := json.Marshal(lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(`1`), Method: req.Method, Params: req.Params})
	if err != nil {
		t.Fatal(err)
	}
	return uint64(canonicalRequestFrameBytes(len(body)))
}

func TestPrivateB4SuccessorPrefetchChargesAndReleases(t *testing.T) {
	reservation := &privateB4Reservation{explicitBytes: mustPrivateB4ByteAccountV2(t)}
	reader, err := lspwire.NewSuccessorIngressReader(bytes.NewReader(nil), privateB4SuccessorOptions(reservation))
	if err != nil {
		t.Fatal(err)
	}
	if got := reservation.explicitBytes.snapshot(); got != (privateB4ByteLedgerSnapshot{Live: privateB4ByteLedgerV2TableBytes + 4096, Cumulative: privateB4ByteLedgerV2TableBytes + 4096}) {
		t.Fatalf("ASSERT_C15_PREFETCH_EXACT snapshot=%+v", got)
	}
	reader.Close()
	reader.Close()
	if got := reservation.explicitBytes.snapshot(); got != (privateB4ByteLedgerSnapshot{Live: privateB4ByteLedgerV2TableBytes, Cumulative: privateB4ByteLedgerV2TableBytes + 4096}) {
		t.Fatalf("ASSERT_C15_PREFETCH_RELEASE snapshot=%+v", got)
	}

	nonCustodial := &privateB4Reservation{nonCustodial: true}
	reader, err = lspwire.NewSuccessorIngressReader(bytes.NewReader(nil), privateB4SuccessorOptions(nonCustodial))
	if err != nil {
		t.Fatal(err)
	}
	reader.Close()
	if got := nonCustodial.explicitBytes.snapshot(); got != (privateB4ByteLedgerSnapshot{}) {
		t.Fatalf("ASSERT_C15_PREFETCH_P2_ISOLATION snapshot=%+v", got)
	}
}

func TestPrivateB4SuccessorHeaderChargesAndReleases(t *testing.T) {
	var framed bytes.Buffer
	message := lspwire.Message{JSONRPC: lspwire.Version, Method: "window/logMessage", Params: json.RawMessage(`{"type":3,"message":"x"}`)}
	body, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	if err := lspwire.NewWriter(&framed, lspwire.DefaultLimits()).Write(message); err != nil {
		t.Fatal(err)
	}
	reservation := &privateB4Reservation{explicitBytes: mustPrivateB4ByteAccountV2(t)}
	reader, err := lspwire.NewSuccessorIngressReader(bytes.NewReader(framed.Bytes()), privateB4SuccessorOptions(reservation))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reader.ReadFrame(); err != nil {
		t.Fatal(err)
	}
	wantCumulative := uint64(4096 + 65537 + len(body) + 2*framed.Len())
	if got := reservation.explicitBytes.snapshot(); got != (privateB4ByteLedgerSnapshot{Live: privateB4ByteLedgerV2TableBytes + uint64(4096+framed.Len()), Cumulative: privateB4ByteLedgerV2TableBytes + wantCumulative}) {
		t.Fatalf("ASSERT_C15_HEADER_FRAME_EXIT snapshot=%+v", got)
	}
	reader.Close()
	if got := reservation.explicitBytes.snapshot(); got != (privateB4ByteLedgerSnapshot{Live: privateB4ByteLedgerV2TableBytes + uint64(framed.Len()), Cumulative: privateB4ByteLedgerV2TableBytes + wantCumulative}) {
		t.Fatalf("ASSERT_C15_HEADER_READER_CLOSE snapshot=%+v", got)
	}
	reservation.capture.ResponseFrame = nil
	reservation.responseFrameOwner.releaseManager()
	if got := reservation.explicitBytes.snapshot(); got != (privateB4ByteLedgerSnapshot{Live: privateB4ByteLedgerV2TableBytes, Cumulative: privateB4ByteLedgerV2TableBytes + wantCumulative}) {
		t.Fatalf("ASSERT_C15_ORIGINAL_FRAME_RELEASE snapshot=%+v", got)
	}
}

func TestPrivateB4ParamsIntegratedRefusalPrecedesCopyAndWrite(t *testing.T) {
	f := newFullP1Fixture(t)
	var ledger *privateB4ByteAccountV2
	var filler privateB4ByteLeaseV2
	var before privateB4ByteLedgerSnapshot
	f.m.b4ParamsHook = func(got *privateB4ByteAccountV2) {
		ledger = got
		var failure session.Failure
		filler, failure = got.reserve(privateB4MaxOwnedBytes - privateB4ByteLedgerV2TableBytes - uint64(len(f.req.Params)) + 1)
		if failure != "" {
			t.Fatalf("ASSERT_C15_PARAMS_FIXTURE_FILL failure=%q", failure)
		}
		before = got.snapshot()
	}
	original := append([]byte(nil), f.req.Params...)
	result, lease := f.m.RoundTripPrivateB4(context.Background(), f.req, f.owner)
	if result.Failure != session.ResourceExhausted || lease != (B4DefinitionLease{}) {
		t.Fatalf("ASSERT_C15_PARAMS_PRECOPY_REFUSAL result=%+v lease=%+v", result, lease)
	}
	if f.child.writes.Load() != 0 || string(f.req.Params) != string(original) || f.owner.TargetSources[0].state.state.Load() != privateB4SourceHeld {
		t.Fatalf("ASSERT_C15_PARAMS_REFUSAL_EFFECT writes=%d source=%d", f.child.writes.Load(), f.owner.TargetSources[0].state.state.Load())
	}
	after := ledger.snapshot()
	if after != before {
		t.Fatalf("ASSERT_C15_PARAMS_REFUSAL_ATOMIC before=%+v after=%+v", before, after)
	}
	filler.release()
	if ledger.snapshot().Live != 0 {
		t.Fatalf("ASSERT_C15_PARAMS_REFUSAL_CLEANUP snapshot=%+v", ledger.snapshot())
	}
}

func TestPrivateB4OutboundReservationExactEqualityAndPlusOne(t *testing.T) {
	bodyBytes := 257
	transportBytes := uint64(canonicalRequestFrameBytes(bodyBytes))

	equal := mustPrivateB4ByteAccountV2(t)
	filler, failure := equal.reserve(privateB4MaxOwnedBytes - privateB4ByteLedgerV2TableBytes - transportBytes)
	if failure != "" {
		t.Fatal(failure)
	}
	lease, failure := equal.reservePrivateB4OutboundTransport(bodyBytes)
	if failure != "" || equal.snapshot().Live != privateB4MaxOwnedBytes {
		t.Fatalf("ASSERT_C15_ENCODED_EQUALITY failure=%q snapshot=%+v", failure, equal.snapshot())
	}
	lease.release()
	filler.release()

	plusOne := mustPrivateB4ByteAccountV2(t)
	filler, failure = plusOne.reserve(privateB4MaxOwnedBytes - privateB4ByteLedgerV2TableBytes - transportBytes + 1)
	if failure != "" {
		t.Fatal(failure)
	}
	before := plusOne.snapshot()
	if _, failure = plusOne.reservePrivateB4OutboundTransport(bodyBytes); failure != session.ResourceExhausted {
		t.Fatalf("ASSERT_C15_ENCODED_PLUS_ONE failure=%q", failure)
	}
	if after := plusOne.snapshot(); after != before {
		t.Fatalf("ASSERT_C15_ENCODED_PLUS_ONE_ATOMIC before=%+v after=%+v", before, after)
	}
	filler.release()
}

func TestPrivateB4OutboundTransportIntegratedRefusalPrecedesWrite(t *testing.T) {
	f := newFullP1Fixture(t)
	transportBytes := privateB4OutboundFrameBytes(t, f.req)
	var ledger *privateB4ByteAccountV2
	var filler privateB4ByteLeaseV2
	var before privateB4ByteLedgerSnapshot
	f.m.b4OutboundHook = func(got *privateB4ByteAccountV2) {
		ledger = got
		remaining := privateB4MaxOwnedBytes - got.snapshot().Live
		var failure session.Failure
		filler, failure = got.reserve(remaining - transportBytes + 1)
		if failure != "" {
			t.Fatalf("ASSERT_C15_OUTBOUND_FIXTURE_FILL failure=%q", failure)
		}
		before = got.snapshot()
	}

	result, lease := f.m.RoundTripPrivateB4(context.Background(), f.req, f.owner)
	if result.Failure != session.ResourceExhausted || lease != (B4DefinitionLease{}) {
		t.Fatalf("ASSERT_C15_OUTBOUND_PREWRITE_REFUSAL result=%+v lease=%+v", result, lease)
	}
	if f.child.writes.Load() != 0 {
		t.Fatalf("ASSERT_C15_OUTBOUND_REFUSAL_STDIN writes=%d", f.child.writes.Load())
	}
	after := ledger.snapshot()
	fillerEntry := filler.lease.entry()
	if ledger == nil || fillerEntry == nil || after.Cumulative != before.Cumulative || after.Live != privateB4ByteLedgerV2TableBytes+fillerEntry.capacity {
		t.Fatalf("ASSERT_C15_OUTBOUND_REFUSAL_ATOMIC before=%+v after=%+v", before, after)
	}
	filler.release()
	if got := ledger.snapshot().Live; got != 0 {
		t.Fatalf("ASSERT_C15_OUTBOUND_REFUSAL_CLEANUP live=%d", got)
	}
}

func TestPrivateB4OutboundTransportIntegratedSuccessReleasesTemporary(t *testing.T) {
	f := newFullP1Fixture(t)
	transportBytes := privateB4OutboundFrameBytes(t, f.req)
	var ledger *privateB4ByteAccountV2
	var before, afterWrite privateB4ByteLedgerSnapshot
	f.m.b4OutboundHook = func(got *privateB4ByteAccountV2) {
		ledger, before = got, got.snapshot()
	}
	f.m.b4AfterWriteHook = func(got *privateB4ByteAccountV2) { afterWrite = got.snapshot() }
	result, lease, selection := f.transact(t)
	wantAfterWrite := privateB4ByteLedgerSnapshot{Live: before.Live + transportBytes, Cumulative: before.Cumulative + transportBytes}
	if ledger == nil || afterWrite != wantAfterWrite {
		t.Fatalf("ASSERT_C15_OUTBOUND_SUCCESS_EXACT before=%+v afterWrite=%+v want=%+v", before, afterWrite, wantAfterWrite)
	}
	frameLease, ok := result.PrivateB4RequestFrameLease()
	if !ok {
		t.Fatal("ASSERT_C15_OUTBOUND_SUCCESS_CALLER_LEASE")
	}
	var frameBytes uint64
	if err := frameLease.WithBytes(func(frame []byte) error { frameBytes = uint64(len(frame)); return nil }); err != nil {
		t.Fatal(err)
	}
	responseLease, responseBytes := privateB4C15ResponseFrameLease(t, result)
	resultLease, resultBytes := privateB4C15ResultLease(t, result)
	if _, status := consumePrivateB4SnapshotForTest(f.m, lease, selection); status != PrivateB4Selected {
		t.Fatalf("ASSERT_C15_OUTBOUND_SUCCESS_CONSUME status=%s", status)
	}
	if got := ledger.snapshot().Live; got != privateB4ByteLedgerV2TableBytes+frameBytes+responseBytes+resultBytes {
		t.Fatalf("ASSERT_C15_OUTBOUND_SUCCESS_TEMPORARY_RELEASE live=%d caller=%d", got, frameBytes+responseBytes+resultBytes)
	}
	if !frameLease.Release() || !responseLease.Release() || !resultLease.Release() || ledger.snapshot().Live != 0 {
		t.Fatalf("ASSERT_C15_OUTBOUND_SUCCESS_CALLER_RELEASE snapshot=%+v", ledger.snapshot())
	}
}

func TestPrivateB4OutboundTransportIntegratedWriteFailureReleasesTemporary(t *testing.T) {
	f := newFullP1Fixture(t)
	transportBytes := privateB4OutboundFrameBytes(t, f.req)
	var ledger *privateB4ByteAccountV2
	var before privateB4ByteLedgerSnapshot
	f.m.b4OutboundHook = func(got *privateB4ByteAccountV2) {
		ledger, before = got, got.snapshot()
		_ = f.child.stdin.Close()
	}
	result, lease := f.m.RoundTripPrivateB4(context.Background(), f.req, f.owner)
	if result.Failure == "" || lease != (B4DefinitionLease{}) {
		t.Fatalf("ASSERT_C15_OUTBOUND_WRITE_FAILURE result=%+v lease=%+v", result, lease)
	}
	if ledger == nil || ledger.snapshot().Cumulative != before.Cumulative+transportBytes || ledger.snapshot().Live != 0 {
		t.Fatalf("ASSERT_C15_OUTBOUND_WRITE_FAILURE_CLEANUP before=%+v after=%+v", before, ledger.snapshot())
	}
}

func TestPrivateB4OutboundTransportHookIsCustodialOnly(t *testing.T) {
	for _, tc := range []struct {
		name string
		p2   bool
	}{
		{name: "ordinary"},
		{name: "non-custodial-p2", p2: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFullP1Fixture(t)
			called := false
			f.m.b4OutboundHook = func(*privateB4ByteAccountV2) { called = true }
			req := f.req
			req.ADR0011PrivateP2 = tc.p2
			if !tc.p2 {
				req.CaptureMethodRequestFrameMaxBytes = 0
			}
			if result := f.m.RoundTrip(context.Background(), req); result.Failure != "" {
				t.Fatalf("ASSERT_C15_OUTBOUND_ISOLATION_FIXTURE p2=%v result=%+v", tc.p2, result)
			}
			if called {
				t.Fatalf("ASSERT_C15_OUTBOUND_ISOLATION p2=%v", tc.p2)
			}
		})
	}
}
