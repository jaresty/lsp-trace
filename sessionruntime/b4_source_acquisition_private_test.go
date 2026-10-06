package sessionruntime

import (
	"bytes"
	"context"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"

	"lsp-trace/internal/session"
)

func TestPrivateB4SourceHasNoCallerByteAdmissionOrDuplicateRegistry(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("ASSERT_B4_NO_CALLER_BYTE_ADMISSION_SOURCE_LOCATION")
	}
	production, err := os.ReadFile(strings.TrimSuffix(file, "b4_source_acquisition_private_test.go") + "b4_definition_private.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"B4DefinitionSourceAcquisition", "AcquirePrivateB4DefinitionSource", "privateB4SourceReservation", "privateB4Sources"} {
		if strings.Contains(string(production), forbidden) {
			t.Fatalf("ASSERT_B4_NO_CALLER_BYTE_ADMISSION_SURFACE forbidden=%s", forbidden)
		}
	}
}

func privateB4PreparedSource(t *testing.T, content []byte) (*Manager, DocumentRequest, B4DefinitionSourceLease) {
	t.Helper()
	m, req, _, _ := supplyFixture(t, content)
	prepared := m.PrepareDocument(context.Background(), req)
	if prepared.Failure != "" || prepared.Supply == nil {
		t.Fatalf("BLOCKED_NOT_RED prepared source: %+v", prepared)
	}
	lease, status := m.PreparePrivateB4DefinitionSource(B4DefinitionSourceReference{
		SessionID: req.SessionID, Generation: req.Generation, URI: req.URI, DocumentVersion: prepared.Version,
	})
	if status != PrivateB4Selected || lease.state == nil {
		t.Fatalf("BLOCKED_NOT_RED source lease: status=%s", status)
	}
	return m, req, lease
}

func TestPrivateB4SourceRequiresExactPreparedRecord(t *testing.T) {
	content := []byte("package exact\n")
	m, req, lease := privateB4PreparedSource(t, content)
	if lease.state.manager != m || lease.state.session != m.sessions[req.SessionID] ||
		lease.state.source.SessionID != req.SessionID || lease.state.source.Generation != req.Generation ||
		lease.state.source.URI != req.URI || lease.state.source.DocumentVersion != 1 ||
		lease.state.source.AcquisitionID == "" || lease.state.source.SHA256 != privateB4Hash(content) ||
		!bytes.Equal(lease.state.source.Bytes, content) {
		t.Fatalf("ASSERT_B4_EXACT_PREPARED_SOURCE_BINDING source=%+v", lease.state.source)
	}
	for _, ref := range []B4DefinitionSourceReference{
		{SessionID: req.SessionID, Generation: req.Generation + 1, URI: req.URI, DocumentVersion: 1},
		{SessionID: req.SessionID, Generation: req.Generation, URI: req.URI, DocumentVersion: 2},
		{SessionID: req.SessionID, Generation: req.Generation, URI: "file:///outside.go", DocumentVersion: 1},
	} {
		if got, status := m.PreparePrivateB4DefinitionSource(ref); status != PrivateB4Unavailable || got.state != nil {
			t.Fatalf("ASSERT_B4_PREPARED_SOURCE_REFUSAL ref=%+v status=%s", ref, status)
		}
	}
}

func TestPrivateB4SourceExactBytesReachAdmissionAndStaleVersionRefuses(t *testing.T) {
	content := []byte("package bid30\n")
	m, req, lease := privateB4PreparedSource(t, content)
	reservation := &privateB4Reservation{sourceLeases: []B4DefinitionSourceLease{lease}}
	m.mu.Lock()
	failure := m.admitPrivateB4SourcesLocked(RoundTripRequest{SessionID: req.SessionID, Generation: req.Generation, MaxBytes: 64, CaptureDefinitionResponseFrameMaxBytes: 64, CaptureMethodRequestFrameMaxBytes: 64}, reservation)
	m.mu.Unlock()
	if failure != "" || len(reservation.capture.TargetSources) != 1 || !bytes.Equal(reservation.capture.TargetSources[0].Bytes, content) {
		t.Fatalf("ASSERT_B4_EXACT_PREPARED_BYTES_REACH_BID30 failure=%s sources=%+v", failure, reservation.capture.TargetSources)
	}

	m2, req2, stale := privateB4PreparedSource(t, []byte("package old\n"))
	m2.mu.Lock()
	doc := m2.sessions[req2.SessionID].documents[req2.URI]
	doc.version++
	m2.sessions[req2.SessionID].documents[req2.URI] = doc
	failure = m2.admitPrivateB4SourcesLocked(RoundTripRequest{SessionID: req2.SessionID, Generation: req2.Generation, MaxBytes: 64, CaptureDefinitionResponseFrameMaxBytes: 64, CaptureMethodRequestFrameMaxBytes: 64}, &privateB4Reservation{sourceLeases: []B4DefinitionSourceLease{stale}})
	m2.mu.Unlock()
	if failure != session.ToolNotImplemented || stale.state.state.Load() != privateB4SourceHeld {
		t.Fatalf("ASSERT_B4_STALE_DOCUMENT_VERSION_REFUSED failure=%s state=%d", failure, stale.state.state.Load())
	}
}

func TestPrivateB4SourceReleaseRequiresBoundIngressState(t *testing.T) {
	m, _, lease := privateB4PreparedSource(t, []byte("package binding\n"))
	before := m.privateB4SourceIngress.snapshot()
	bytesBefore := append([]byte(nil), lease.state.source.Bytes...)
	entry := lease.state.ingress.entry()
	entry.state = &privateB4SourceLease{manager: m}
	if status := m.ReleasePrivateB4DefinitionSource(lease); status != PrivateB4Unavailable || lease.state.state.Load() != privateB4SourceHeld || !bytes.Equal(lease.state.source.Bytes, bytesBefore) || m.privateB4SourceIngress.snapshot() != before {
		t.Fatalf("ASSERT_C15_SOURCE_RELEASE_BINDING status=%s state=%d snapshot=%+v", status, lease.state.state.Load(), m.privateB4SourceIngress.snapshot())
	}
	entry.state = lease.state
	if status := m.ReleasePrivateB4DefinitionSource(lease); status != PrivateB4Selected {
		t.Fatalf("ASSERT_C15_SOURCE_RELEASE_BINDING_RETRY status=%s", status)
	}
}

func TestPrivateB4SourceReleaseReplayAndConcurrentTransfer(t *testing.T) {
	m, req, released := privateB4PreparedSource(t, []byte("package release\n"))
	var wg sync.WaitGroup
	results := make(chan PrivateB4Status, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); results <- m.ReleasePrivateB4DefinitionSource(released) }()
	}
	wg.Wait()
	close(results)
	selected := 0
	for status := range results {
		if status == PrivateB4Selected {
			selected++
		}
	}
	if selected != 1 || m.ReleasePrivateB4DefinitionSource(released) != PrivateB4Unavailable {
		t.Fatalf("ASSERT_B4_RELEASE_REPLAY_CONCURRENT_ONE_WINNER winners=%d", selected)
	}

	_, _, transferred := privateB4PreparedSource(t, []byte("package transfer\n"))
	reservation := &privateB4Reservation{sourceLeases: []B4DefinitionSourceLease{transferred}}
	m = transferred.state.manager
	m.mu.Lock()
	failure := m.admitPrivateB4SourcesLocked(RoundTripRequest{SessionID: transferred.state.source.SessionID, Generation: transferred.state.source.Generation, MaxBytes: 64, CaptureDefinitionResponseFrameMaxBytes: 64, CaptureMethodRequestFrameMaxBytes: 64}, reservation)
	m.mu.Unlock()
	if failure != "" || m.ReleasePrivateB4DefinitionSource(transferred) != PrivateB4Unavailable {
		t.Fatalf("ASSERT_B4_TRANSFER_REPLAY_REFUSED failure=%s", failure)
	}
	_ = req
}

func TestPrivateB4SourceInstallPathIsProspectivelyInfallible(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("ASSERT_B4_POST_TRANSFER_SOURCE_LOCATION")
	}
	production, err := os.ReadFile(strings.TrimSuffix(file, "b4_source_acquisition_private_test.go") + "b4_definition_private.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(production)
	installStart := strings.Index(source, "func (m *Manager) installPrivateB4Locked")
	installEnd := strings.Index(source[installStart:], "\n}\n")
	if installStart < 0 || installEnd < 0 {
		t.Fatal("ASSERT_B4_POST_TRANSFER_FUNCTION_BOUNDARIES")
	}
	managerSource, err := os.ReadFile(strings.TrimSuffix(file, "b4_source_acquisition_private_test.go") + "sessionruntime.go")
	if err != nil {
		t.Fatal(err)
	}
	manager := string(managerSource)
	if !strings.Contains(manager, "privateB4Leases          [privateB4MaxSlots]privateB4Slot") ||
		strings.Contains(manager, "privateB4Leases: make(") {
		t.Fatal("ASSERT_B4_RESERVATION_STORAGE_FIXED_ARRAY")
	}
	admitStart := strings.Index(source, "func (m *Manager) admitPrivateB4SourcesLocked")
	if admitStart < 0 {
		t.Fatal("ASSERT_B4_ADMISSION_FUNCTION_BOUNDARY")
	}
	admit := source[admitStart:installStart]
	slotSelection := strings.Index(admit, "reservation.slot = emptySlot")
	transfer := strings.Index(admit, "source.state.Store(privateB4SourceTransferred)")
	if slotSelection < 0 || transfer < 0 || slotSelection >= transfer {
		t.Fatal("ASSERT_B4_EXACT_SLOT_SELECTED_BEFORE_TRANSFER")
	}
	if strings.Contains(admit, "CompareAndSwap(privateB4SourceHeld, privateB4SourceTransferred)") || strings.Contains(admit, "rollback") {
		t.Fatal("ASSERT_B4_TRANSFER_HAS_NO_POST_START_REFUSAL_PATH")
	}
	install := source[installStart : installStart+installEnd+3]
	if strings.Contains(install, "privateB4Leases[reservation.token]") || !strings.Contains(install, "privateB4Leases[reservation.slot]") {
		t.Fatalf("ASSERT_B4_POST_TRANSFER_INSTALL_USES_PRESELECTED_SLOT body=%s", install)
	}
	for _, forbidden := range []string{"make(", "if ", "return"} {
		if strings.Contains(install, forbidden) {
			t.Fatalf("ASSERT_B4_POST_TRANSFER_INSTALL_INFALLIBLE forbidden=%q body=%s", forbidden, install)
		}
	}
}

func TestPrivateB4SourceInstallPreparedSlotAllocations(t *testing.T) {
	reservation := &privateB4Reservation{slot: 2, maxCharge: 37}
	reservation.token[0] = 9
	m := &Manager{}
	allocs := testing.AllocsPerRun(1000, func() {
		m.privateB4Leases[reservation.slot] = privateB4Slot{}
		m.privateB4Bytes = 0
		m.installPrivateB4Locked(RoundTripRequest{}, nil, reservation)
	})
	if allocs != 0 || !m.privateB4Leases[reservation.slot].occupied ||
		m.privateB4Leases[reservation.slot].token != reservation.token ||
		m.privateB4Leases[reservation.slot].reservation != reservation || m.privateB4Bytes != reservation.maxCharge {
		t.Fatalf("ASSERT_B4_PREPARED_SLOT_INSTALL_ZERO_ALLOCS allocs=%v slot=%+v bytes=%d", allocs, m.privateB4Leases[reservation.slot], m.privateB4Bytes)
	}
}

func TestPrivateB4SourceTransferBoundaryInstallsWithoutFurtherRefusal(t *testing.T) {
	m, req, lease := privateB4PreparedSource(t, []byte("package nofail\n"))
	reservation := &privateB4Reservation{sourceLeases: []B4DefinitionSourceLease{lease}}
	reservation.token[0] = 1
	m.mu.Lock()
	failure := m.admitPrivateB4SourcesLocked(RoundTripRequest{SessionID: req.SessionID, Generation: req.Generation, MaxBytes: 64, CaptureDefinitionResponseFrameMaxBytes: 64, CaptureMethodRequestFrameMaxBytes: 64}, reservation)
	if failure == "" {
		m.installPrivateB4Locked(RoundTripRequest{SessionID: req.SessionID, Generation: req.Generation}, m.sessions[req.SessionID], reservation)
	}
	installed, charged := m.privateB4ReservationLocked(reservation.token), m.privateB4Bytes
	m.mu.Unlock()
	if failure != "" || lease.state.state.Load() != privateB4SourceTransferred || installed != reservation || reservation.session != m.sessions[req.SessionID] || charged != reservation.maxCharge {
		t.Fatalf("ASSERT_B4_POST_TRANSFER_INSTALL_NO_FAIL failure=%s state=%d installed=%t charged=%d/%d", failure, lease.state.state.Load(), installed == reservation, charged, reservation.maxCharge)
	}
}

func TestPrivateB4SourceTokenCollisionRefusalZeroEffect(t *testing.T) {
	m, req, lease := privateB4PreparedSource(t, []byte("package collision\n"))
	candidate := &privateB4Reservation{sourceLeases: []B4DefinitionSourceLease{lease}, slot: 3, maxCharge: 41}
	candidate.token[0] = 7
	conflict := &privateB4Reservation{token: candidate.token, slot: 1, maxCharge: 23}
	m.mu.Lock()
	m.privateB4Leases[conflict.slot] = privateB4Slot{occupied: true, token: conflict.token, reservation: conflict}
	m.privateB4Bytes = conflict.maxCharge
	failure := m.admitPrivateB4SourcesLocked(RoundTripRequest{SessionID: req.SessionID, Generation: req.Generation, MaxBytes: 64, CaptureDefinitionResponseFrameMaxBytes: 64, CaptureMethodRequestFrameMaxBytes: 64}, candidate)
	slots, charged := m.privateB4LeaseCountLocked(), m.privateB4Bytes
	m.mu.Unlock()
	if failure != session.ResourceExhausted || slots != 1 || charged != conflict.maxCharge ||
		lease.state.state.Load() != privateB4SourceHeld || m.ReleasePrivateB4DefinitionSource(lease) != PrivateB4Selected {
		t.Fatalf("ASSERT_B4_TOKEN_COLLISION_ZERO_EFFECT failure=%s slots=%d bytes=%d state=%d", failure, slots, charged, lease.state.state.Load())
	}
}

func TestPrivateB4SourceCapacityRefusalZeroEffect(t *testing.T) {
	m, req, lease := privateB4PreparedSource(t, []byte("package capacity\n"))
	m.mu.Lock()
	for i := range m.privateB4Leases {
		var token [32]byte
		token[0] = byte(i + 1)
		reservation := &privateB4Reservation{token: token, slot: i}
		m.privateB4Leases[i] = privateB4Slot{occupied: true, token: token, reservation: reservation}
	}
	beforeBytes := m.privateB4Bytes
	failure := m.admitPrivateB4SourcesLocked(RoundTripRequest{SessionID: req.SessionID, Generation: req.Generation, MaxBytes: 64, CaptureDefinitionResponseFrameMaxBytes: 64, CaptureMethodRequestFrameMaxBytes: 64}, &privateB4Reservation{sourceLeases: []B4DefinitionSourceLease{lease}})
	afterBytes, slots := m.privateB4Bytes, m.privateB4LeaseCountLocked()
	m.mu.Unlock()
	if failure != session.ResourceExhausted || slots != privateB4MaxSlots || beforeBytes != afterBytes || lease.state.state.Load() != privateB4SourceHeld || m.ReleasePrivateB4DefinitionSource(lease) != PrivateB4Selected {
		t.Fatalf("ASSERT_B4_CAPACITY_REFUSAL_ZERO_EFFECT failure=%s slots=%d bytes=%d/%d state=%d", failure, slots, beforeBytes, afterBytes, lease.state.state.Load())
	}
}
