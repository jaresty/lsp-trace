package sessionruntime

import (
	"bytes"
	"errors"
	"testing"

	"lsp-trace/internal/session"
)

func privateB4C17AcquisitionRequest(req DocumentRequest) RoundTripRequest {
	return RoundTripRequest{
		SessionID: req.SessionID, Generation: req.Generation,
		MaxBytes: 64, CaptureDefinitionResponseFrameMaxBytes: 64, CaptureMethodRequestFrameMaxBytes: 64,
	}
}

func TestPrivateB4C17AcquisitionProducerChargesDeduplicatedSourcesOnce(t *testing.T) {
	content := []byte("package acquisition\n")
	manager, req, lease := privateB4PreparedSource(t, content)
	profile, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	reservation := &privateB4Reservation{sourceLeases: []B4DefinitionSourceLease{lease, lease}}
	manager.mu.Lock()
	failure = manager.admitPrivateB4SourcesC17Locked(privateB4C17AcquisitionRequest(req), reservation, profile)
	manager.mu.Unlock()
	if failure != "" {
		t.Fatalf("ASSERT_C17_ACQUISITION_PRODUCER_SUCCESS failure=%s", failure)
	}
	if lease.state.state.Load() != privateB4SourceTransferred || len(reservation.capture.TargetSources) != 1 || !bytes.Equal(reservation.capture.TargetSources[0].Bytes, content) {
		t.Fatalf("ASSERT_C17_ACQUISITION_PRODUCER_CUSTODY state=%d sources=%+v", lease.state.state.Load(), reservation.capture.TargetSources)
	}
	if snapshot := profile.eventSnapshot(); snapshot.Admitted != 4 || snapshot.InFlight != 0 || snapshot.ActiveCallbacks != 0 {
		t.Fatalf("ASSERT_C17_ACQUISITION_DEDUP_2_PLUS_2E snapshot=%+v", snapshot)
	}
	want := privateB4C17WantAcquisition(1)
	got := privateB4C17CommittedIdentities(t, profile)
	if len(got) != len(want) {
		t.Fatalf("ASSERT_C17_ACQUISITION_PRODUCER_IDENTITIES got=%+v want=%+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ASSERT_C17_ACQUISITION_PRODUCER_ORDER index=%d got=%+v want=%+v", i, got[i], want[i])
		}
	}
}

func TestPrivateB4C17AcquisitionDedupPrecedesCapacityBoundary(t *testing.T) {
	manager, req, lease := privateB4PreparedSource(t, []byte("package dedupboundary\n"))
	profile, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	for i := 0; i < privateB4EventLimit-4; i++ {
		if err := profile.WithEventAdmission(c17Event(i), func() error { return nil }); err != nil {
			t.Fatal(err)
		}
	}
	reservation := &privateB4Reservation{sourceLeases: []B4DefinitionSourceLease{lease, lease}}
	manager.mu.Lock()
	failure = manager.admitPrivateB4SourcesC17Locked(privateB4C17AcquisitionRequest(req), reservation, profile)
	manager.mu.Unlock()
	if failure != "" {
		t.Fatalf("ASSERT_C17_ACQUISITION_DEDUP_BEFORE_CAPACITY failure=%s", failure)
	}
	if snapshot := profile.eventSnapshot(); snapshot.Admitted != privateB4EventLimit || snapshot.TerminalRequested {
		t.Fatalf("ASSERT_C17_ACQUISITION_DEDUP_BOUNDARY_COUNT snapshot=%+v", snapshot)
	}
}

func TestPrivateB4C17AcquisitionMalformedValidationPrecedesFullCapacity(t *testing.T) {
	manager, req, lease := privateB4PreparedSource(t, []byte("package malformed\n"))
	profile, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	for i := 0; i < privateB4EventLimit; i++ {
		if err := profile.WithEventAdmission(c17Event(i), func() error { return nil }); err != nil {
			t.Fatal(err)
		}
	}
	before := profile.eventSnapshot()
	lease.state.source.SHA256 = "sha256:invalid"
	reservation := &privateB4Reservation{sourceLeases: []B4DefinitionSourceLease{lease}}
	manager.mu.Lock()
	failure = manager.admitPrivateB4SourcesC17Locked(privateB4C17AcquisitionRequest(req), reservation, profile)
	manager.mu.Unlock()
	if failure != session.ToolNotImplemented {
		t.Fatalf("ASSERT_C17_ACQUISITION_MALFORMED_BEFORE_CAPACITY failure=%s", failure)
	}
	if after := profile.eventSnapshot(); after != before || after.TerminalRequested {
		t.Fatalf("ASSERT_C17_ACQUISITION_MALFORMED_ZERO_CHARGE before=%+v after=%+v", before, after)
	}
	if lease.state.state.Load() != privateB4SourceHeld {
		t.Fatalf("ASSERT_C17_ACQUISITION_MALFORMED_PRESERVES_CUSTODY state=%d", lease.state.state.Load())
	}
}

func TestPrivateB4C17AcquisitionCapacityRefusalPrecedesTransfer(t *testing.T) {
	manager, req, lease := privateB4PreparedSource(t, []byte("package refusal\n"))
	profile, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	for i := 0; i < privateB4EventLimit-3; i++ {
		if err := profile.WithEventAdmission(c17Event(i), func() error { return nil }); err != nil {
			t.Fatal(err)
		}
	}
	before := profile.eventSnapshot()
	reservation := &privateB4Reservation{sourceLeases: []B4DefinitionSourceLease{lease}}
	manager.mu.Lock()
	failure = manager.admitPrivateB4SourcesC17Locked(privateB4C17AcquisitionRequest(req), reservation, profile)
	manager.mu.Unlock()
	if failure != session.ResourceExhausted {
		t.Fatalf("ASSERT_C17_ACQUISITION_CAPACITY_REFUSAL failure=%s", failure)
	}
	after := profile.eventSnapshot()
	if after.Admitted != before.Admitted || after.InFlight != 0 || after.ActiveCallbacks != 0 || !after.TerminalRequested || after.TerminalFailure != session.ResourceExhausted {
		t.Fatalf("ASSERT_C17_ACQUISITION_CAPACITY_ATOMIC before=%+v after=%+v", before, after)
	}
	if lease.state.state.Load() != privateB4SourceHeld || manager.ReleasePrivateB4DefinitionSource(lease) != PrivateB4Selected {
		t.Fatalf("ASSERT_C17_ACQUISITION_REFUSAL_BEFORE_TRANSFER state=%d", lease.state.state.Load())
	}
}

func TestPrivateB4C17AcquisitionTransferFailureRollsBackAndRetries(t *testing.T) {
	manager, req, lease := privateB4PreparedSource(t, []byte("package transferretry\n"))
	bytesAccount, failure := newPrivateB4ByteAccountV2()
	if failure != "" {
		t.Fatal(failure)
	}
	profile, failure := newPrivateB4EventAccountC17WithBytes(bytesAccount)
	if failure != "" {
		t.Fatal(failure)
	}
	blocker, failure := bytesAccount.table.reserveDescriptor(1)
	if failure != "" {
		t.Fatal(failure)
	}
	reservation := &privateB4Reservation{sourceLeases: []B4DefinitionSourceLease{lease}, explicitBytes: bytesAccount}
	manager.mu.Lock()
	failure = manager.admitPrivateB4SourcesC17Locked(privateB4C17AcquisitionRequest(req), reservation, profile)
	manager.mu.Unlock()
	if failure != session.ResourceExhausted {
		t.Fatalf("ASSERT_C17_ACQUISITION_TRANSFER_FAILURE failure=%s", failure)
	}
	if snapshot := profile.eventSnapshot(); snapshot.Admitted != 0 || snapshot.InFlight != 0 || snapshot.ActiveCallbacks != 0 || snapshot.TerminalRequested {
		t.Fatalf("ASSERT_C17_ACQUISITION_TRANSFER_FAILURE_ROLLBACK snapshot=%+v", snapshot)
	}
	if lease.state.state.Load() != privateB4SourceHeld {
		t.Fatalf("ASSERT_C17_ACQUISITION_TRANSFER_FAILURE_CUSTODY state=%d", lease.state.state.Load())
	}
	blocker.release()
	manager.mu.Lock()
	failure = manager.admitPrivateB4SourcesC17Locked(privateB4C17AcquisitionRequest(req), reservation, profile)
	manager.mu.Unlock()
	if failure != "" || lease.state.state.Load() != privateB4SourceTransferred {
		t.Fatalf("ASSERT_C17_ACQUISITION_TRANSFER_RETRY failure=%s state=%d", failure, lease.state.state.Load())
	}
	if snapshot := profile.eventSnapshot(); snapshot.Admitted != 4 || snapshot.InFlight != 0 || snapshot.ActiveCallbacks != 0 {
		t.Fatalf("ASSERT_C17_ACQUISITION_TRANSFER_RETRY_COUNT snapshot=%+v", snapshot)
	}
}

func TestPrivateB4C17AcquisitionDefaultOffPreservesLegacyAdmission(t *testing.T) {
	manager, req, lease := privateB4PreparedSource(t, []byte("package defaultoff\n"))
	reservation := &privateB4Reservation{sourceLeases: []B4DefinitionSourceLease{lease}}
	manager.mu.Lock()
	failure := manager.admitPrivateB4SourcesLocked(privateB4C17AcquisitionRequest(req), reservation)
	manager.mu.Unlock()
	if failure != "" || lease.state.state.Load() != privateB4SourceTransferred || len(reservation.capture.TargetSources) != 1 {
		t.Fatalf("ASSERT_C17_ACQUISITION_DEFAULT_OFF failure=%s state=%d sources=%d", failure, lease.state.state.Load(), len(reservation.capture.TargetSources))
	}
}

func TestPrivateB4C17AcquisitionAccountCallbackErrorPanicAndRetry(t *testing.T) {
	profile, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	callbackErr := errors.New("acquisition callback failed")
	if err := profile.withAcquisitionAdmission(1, func() error { return callbackErr }); !errors.Is(err, callbackErr) {
		t.Fatalf("ASSERT_C17_ACQUISITION_CALLBACK_ERROR err=%v", err)
	}
	if snapshot := profile.eventSnapshot(); snapshot.Admitted != 0 || snapshot.InFlight != 0 || snapshot.ActiveCallbacks != 0 {
		t.Fatalf("ASSERT_C17_ACQUISITION_CALLBACK_ERROR_ROLLBACK snapshot=%+v", snapshot)
	}
	func() {
		defer func() {
			if recovered := recover(); recovered != "acquisition panic" {
				t.Fatalf("ASSERT_C17_ACQUISITION_CALLBACK_PANIC recovered=%v", recovered)
			}
		}()
		_ = profile.withAcquisitionAdmission(1, func() error { panic("acquisition panic") })
	}()
	if snapshot := profile.eventSnapshot(); snapshot.Admitted != 0 || snapshot.InFlight != 0 || snapshot.ActiveCallbacks != 0 {
		t.Fatalf("ASSERT_C17_ACQUISITION_CALLBACK_PANIC_ROLLBACK snapshot=%+v", snapshot)
	}
	if err := profile.withAcquisitionAdmission(1, func() error { return nil }); err != nil {
		t.Fatalf("ASSERT_C17_ACQUISITION_CALLBACK_RETRY err=%v", err)
	}
	if snapshot := profile.eventSnapshot(); snapshot.Admitted != 4 {
		t.Fatalf("ASSERT_C17_ACQUISITION_CALLBACK_RETRY_COUNT snapshot=%+v", snapshot)
	}
}
