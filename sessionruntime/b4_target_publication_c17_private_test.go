package sessionruntime

import (
	"errors"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

// The C17 commit API is deliberately separate from the accepted C16/legacy
// method. PrivateB4Selected with a non-nil error means selection and preparation
// succeeded but publication failed terminally; PrivateB4Unavailable with a nil
// error means preparation did not commit and the reservation remains reusable.
type privateB4C17CommitContract interface {
	CommitPrivateB4DefinitionBorrowedC17(
		B4DefinitionLease,
		B4DefinitionSelectionKey,
		func(PrivateB4DefinitionBorrow) bool,
		func(PrivateB4DefinitionBorrow) error,
	) (PrivateB4DefinitionCapture, PrivateB4Status, error)
}

var _ privateB4C17CommitContract = (*Manager)(nil)

// Preserve the accepted legacy signatures: adding C17 publication errors must
// not change either existing method or its callers.
var (
	_ func(*Manager, B4DefinitionLease, B4DefinitionSelectionKey, func(PrivateB4DefinitionBorrow) bool, func(PrivateB4DefinitionBorrow)) (PrivateB4DefinitionCapture, PrivateB4Status) = (*Manager).CommitPrivateB4DefinitionBorrowed
	_ func(*Manager, B4DefinitionLease, B4DefinitionSelectionKey, func(PrivateB4DefinitionBorrow) bool) (PrivateB4DefinitionCapture, PrivateB4Status)                                  = (*Manager).ConsumePrivateB4DefinitionBorrowed
)

type privateB4C17PublicationFixture struct {
	manager     *Manager
	lease       B4DefinitionLease
	selection   B4DefinitionSelectionKey
	reservation *privateB4Reservation
	c16         *privateB4ReservationC16
	c17         *privateB4ReservationC17
}

func newPrivateB4C17PublicationFixture(t *testing.T) privateB4C17PublicationFixture {
	t.Helper()
	bytes := mustPrivateB4ByteAccountV2(t)
	c16, failure := newPrivateB4AccountC16WithBytes(bytes)
	if failure != "" {
		t.Fatal(failure)
	}
	c17, failure := newPrivateB4EventAccountC17WithBytes(bytes)
	if failure != "" {
		t.Fatal(failure)
	}
	resultCharge, failure := bytes.reserve(3)
	if failure != "" {
		t.Fatal(failure)
	}
	resultOwner := &privateB4ResultOwner{
		state: privateB4ResultOpen, managerHeld: true,
		bytes: []byte("abc"), charge: &privateB4SuccessorAllocationLease{lease: resultCharge},
	}
	runtime := &runtimeSession{record: Record{SessionID: "c17-publication", Generation: 1, State: session.Ready}}
	selection := B4DefinitionSelectionKey{SessionID: runtime.record.SessionID, Key: lspwire.RequestKey{Generation: 1, ID: 1}, Transaction: "transaction", CompletedOwnerKey: "owner"}
	reservation := &privateB4Reservation{
		slot: 0, session: runtime, selection: selection, resultOwner: resultOwner,
		explicitBytes: bytes, maxCharge: 1,
		capture: PrivateB4DefinitionCapture{SessionID: selection.SessionID, Key: selection.Key, Transaction: selection.Transaction, CompletedOwnerKey: selection.CompletedOwnerKey, ResponseFrame: []byte{1}},
	}
	reservation.token[0] = 1
	manager := &Manager{sessions: map[string]*runtimeSession{selection.SessionID: runtime}}
	c16Reservation := &privateB4ReservationC16{legacy: reservation, profile: c16}
	c17Reservation := &privateB4ReservationC17{legacy: reservation, profile: c17}
	manager.privateB4Leases[0] = privateB4Slot{occupied: true, token: reservation.token, reservation: reservation, c16: c16Reservation, c17: c17Reservation}
	manager.privateB4Bytes = reservation.maxCharge
	return privateB4C17PublicationFixture{manager: manager, lease: B4DefinitionLease{token: reservation.token}, selection: selection, reservation: reservation, c16: c16Reservation, c17: c17Reservation}
}

func (f privateB4C17PublicationFixture) commit(prepare func(PrivateB4DefinitionBorrow) bool, publish func(PrivateB4DefinitionBorrow) error) (PrivateB4DefinitionCapture, PrivateB4Status, error) {
	return f.manager.CommitPrivateB4DefinitionBorrowedC17(f.lease, f.selection, prepare, publish)
}

func TestPrivateB4C17CommitPreparationRetryAndPublicationOutsideManagerLock(t *testing.T) {
	f := newPrivateB4C17PublicationFixture(t)
	published := false
	if capture, status, err := f.commit(func(PrivateB4DefinitionBorrow) bool {
		if !f.manager.mu.TryLock() {
			t.Fatal("ASSERT_C17_PREPARE_OUTSIDE_MANAGER_LOCK")
		}
		f.manager.mu.Unlock()
		return false
	}, func(PrivateB4DefinitionBorrow) error { published = true; return nil }); err != nil || status != PrivateB4Unavailable || capture.SessionID != "" || published {
		t.Fatalf("ASSERT_C17_PREPARE_FALSE_RETRY capture=%+v status=%s err=%v published=%t", capture, status, err, published)
	}
	if f.manager.privateB4ReservationLockedForTest(f.lease.token) != f.reservation {
		t.Fatal("ASSERT_C17_PREPARE_FALSE_RESERVATION_REUSABLE")
	}
	if _, status, err := f.commit(func(PrivateB4DefinitionBorrow) bool { return true }, func(PrivateB4DefinitionBorrow) error {
		if !f.manager.mu.TryLock() {
			t.Fatal("ASSERT_C17_PUBLISH_OUTSIDE_MANAGER_LOCK")
		}
		f.manager.mu.Unlock()
		published = true
		return nil
	}); err != nil || status != PrivateB4Selected || !published {
		t.Fatalf("ASSERT_C17_RETRY_COMMIT status=%s err=%v published=%t", status, err, published)
	}
}

func (m *Manager) privateB4ReservationLockedForTest(token [32]byte) *privateB4Reservation {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.privateB4ReservationLocked(token)
}

func TestPrivateB4C17CommitKeepsBothGuardsAliveThroughPublication(t *testing.T) {
	f := newPrivateB4C17PublicationFixture(t)
	beforeEvents := f.c17.profile.eventSnapshot().Admitted
	if _, status, err := f.commit(func(PrivateB4DefinitionBorrow) bool { return true }, func(PrivateB4DefinitionBorrow) error {
		if c16 := f.c16.profile.objectSnapshot(); c16.ActiveCallbacks == 0 {
			t.Fatalf("ASSERT_C17_C16_PUBLICATION_GUARD snapshot=%+v", c16)
		}
		if c17 := f.c17.profile.eventSnapshot(); c17.ActiveCallbacks == 0 || c17.Admitted != beforeEvents {
			t.Fatalf("ASSERT_C17_PUBLICATION_GUARD_NO_CHARGE snapshot=%+v before=%d", c17, beforeEvents)
		}
		return nil
	}); err != nil || status != PrivateB4Selected {
		t.Fatalf("ASSERT_C17_GUARDED_PUBLICATION status=%s err=%v", status, err)
	}
}

func TestPrivateB4C17CommitPublicationErrorConsumesExactlyOnce(t *testing.T) {
	f := newPrivateB4C17PublicationFixture(t)
	publishErr := errors.New("c17 publication refused")
	if capture, status, err := f.commit(func(PrivateB4DefinitionBorrow) bool { return true }, func(PrivateB4DefinitionBorrow) error { return publishErr }); !errors.Is(err, publishErr) || status != PrivateB4Selected || capture.SessionID != "" {
		t.Fatalf("ASSERT_C17_PUBLICATION_ERROR_EXPLICIT capture=%+v status=%s err=%v", capture, status, err)
	}
	if reservation := f.manager.privateB4ReservationLockedForTest(f.lease.token); reservation != nil {
		t.Fatal("ASSERT_C17_PUBLICATION_ERROR_TERMINAL_CONSUMPTION")
	}
	if snapshot := f.c17.profile.eventSnapshot(); !snapshot.BackingReleased || snapshot.ActiveCallbacks != 0 {
		t.Fatalf("ASSERT_C17_PUBLICATION_ERROR_GUARD_CLEANUP snapshot=%+v", snapshot)
	}
	if _, status, err := f.commit(func(PrivateB4DefinitionBorrow) bool { return true }, func(PrivateB4DefinitionBorrow) error { return nil }); err != nil || status != PrivateB4Unavailable {
		t.Fatalf("ASSERT_C17_PUBLICATION_ERROR_EXACT_ONCE status=%s err=%v", status, err)
	}
}

func TestPrivateB4C17CommitPanicBoundary(t *testing.T) {
	t.Run("prepare panic nil preserves retry", func(t *testing.T) {
		f := newPrivateB4C17PublicationFixture(t)
		func() {
			defer func() {
				if recover() == nil {
					t.Fatal("ASSERT_C17_PREPARE_PANIC_PROPAGATED")
				}
			}()
			_, _, _ = f.commit(func(PrivateB4DefinitionBorrow) bool { panic(nil) }, func(PrivateB4DefinitionBorrow) error { return nil })
		}()
		if f.manager.privateB4ReservationLockedForTest(f.lease.token) != f.reservation {
			t.Fatal("ASSERT_C17_PREPARE_PANIC_REUSABLE")
		}
	})

	t.Run("publish panic nil consumes terminally", func(t *testing.T) {
		f := newPrivateB4C17PublicationFixture(t)
		func() {
			defer func() {
				if recover() == nil {
					t.Fatal("ASSERT_C17_PUBLISH_PANIC_PROPAGATED")
				}
			}()
			_, _, _ = f.commit(func(PrivateB4DefinitionBorrow) bool { return true }, func(PrivateB4DefinitionBorrow) error { panic(nil) })
		}()
		if f.manager.privateB4ReservationLockedForTest(f.lease.token) != nil {
			t.Fatal("ASSERT_C17_PUBLISH_PANIC_TERMINAL_CONSUMPTION")
		}
		if snapshot := f.c17.profile.eventSnapshot(); snapshot.ActiveCallbacks != 0 || !snapshot.BackingReleased {
			t.Fatalf("ASSERT_C17_PUBLISH_PANIC_GUARD_CLEANUP snapshot=%+v", snapshot)
		}
	})
}

func TestPrivateB4C17CommitRetirementWaitsForPublication(t *testing.T) {
	for _, operation := range []string{"STOP", "RESTART"} {
		t.Run(operation, func(t *testing.T) {
			f := newPrivateB4C17PublicationFixture(t)
			entered := make(chan struct{})
			release := make(chan struct{})
			commitDone := make(chan error, 1)
			go func() {
				_, _, err := f.commit(func(PrivateB4DefinitionBorrow) bool { return true }, func(PrivateB4DefinitionBorrow) error {
					close(entered)
					<-release
					return nil
				})
				commitDone <- err
			}()
			<-entered

			f.manager.mu.Lock()
			join := f.manager.retirePrivateB4StoppedLocked(f.reservation.selection.SessionID, f.reservation.selection.Key.Generation, f.reservation.session)
			f.manager.mu.Unlock()
			retired := make(chan struct{})
			go func() { join.wait(); close(retired) }()
			if !f.manager.mu.TryLock() {
				t.Fatalf("ASSERT_C17_%s_RETIREMENT_WAIT_OUTSIDE_MANAGER_LOCK", operation)
			}
			f.manager.mu.Unlock()
			select {
			case <-retired:
				t.Fatalf("ASSERT_C17_%s_RETIREMENT_RACED_PUBLICATION", operation)
			case <-time.After(20 * time.Millisecond):
			}
			close(release)
			if err := <-commitDone; err != nil {
				t.Fatal(err)
			}
			select {
			case <-retired:
			case <-time.After(time.Second):
				t.Fatalf("ASSERT_C17_%s_RETIREMENT_DID_NOT_SETTLE", operation)
			}
		})
	}
}

func TestPrivateB4C17TargetRefusalPreservesEarlierCumulativeAdmission(t *testing.T) {
	profile, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	for ordinal := 0; ordinal < (privateB4EventLimit/2)-1; ordinal++ {
		if err := profile.withTargetAppendAdmission(uint64(ordinal), func() error { return nil }); err != nil {
			t.Fatalf("BLOCKED_NOT_RED cumulative fill ordinal=%d err=%v", ordinal, err)
		}
	}
	if snapshot := profile.eventSnapshot(); snapshot.Admitted != privateB4EventLimit-2 {
		t.Fatalf("BLOCKED_NOT_RED cumulative fixture snapshot=%+v", snapshot)
	}
	if err := profile.withTargetAppendAdmission(9001, func() error { return nil }); err != nil {
		t.Fatalf("BLOCKED_NOT_RED final accepted pair err=%v", err)
	}
	beforeRefusal := profile.eventSnapshot()
	called := false
	if err := profile.withTargetAppendAdmission(9002, func() error { called = true; return nil }); !errors.Is(err, errPrivateB4EventAdmission) || called {
		t.Fatalf("ASSERT_C17_LATER_TARGET_ATOMIC_REFUSAL err=%v called=%t", err, called)
	}
	afterRefusal := profile.eventSnapshot()
	if afterRefusal.Admitted != beforeRefusal.Admitted || afterRefusal.Admitted != privateB4EventLimit || !afterRefusal.TerminalRequested {
		t.Fatalf("ASSERT_C17_EARLIER_ADMISSIONS_NEVER_ROLL_BACK before=%+v after=%+v", beforeRefusal, afterRefusal)
	}
}

func TestPrivateB4C17DefaultOffNilPublishIsUnavailable(t *testing.T) {
	f := newPrivateB4C17PublicationFixture(t)

	f.manager.mu.Lock()
	slot := f.manager.privateB4Leases[f.reservation.slot]
	slot.c17 = nil
	f.manager.privateB4Leases[f.reservation.slot] = slot
	f.manager.mu.Unlock()

	_, status, err := f.manager.CommitPrivateB4DefinitionBorrowedC17(
		f.lease,
		f.selection,
		func(PrivateB4DefinitionBorrow) bool { return true },
		nil,
	)
	if status != PrivateB4Unavailable || err != nil {
		t.Fatalf("ASSERT_C17_DEFAULT_OFF_NIL_PUBLISH_UNAVAILABLE status=%q err=%v", status, err)
	}
	if _, status := f.manager.PreparePrivateB4DefinitionBorrowed(f.lease, f.selection, func(PrivateB4DefinitionBorrow) bool { return true }); status != PrivateB4Selected {
		t.Fatalf("ASSERT_C17_DEFAULT_OFF_NIL_PUBLISH_REUSABLE status=%q", status)
	}
}

func TestPrivateB4C17LegacyC16ConsumeContractUnchanged(t *testing.T) {
	f := newPrivateB4C17PublicationFixture(t)
	prepared := 0
	if _, status := f.manager.CommitPrivateB4DefinitionBorrowed(f.lease, f.selection, func(PrivateB4DefinitionBorrow) bool { prepared++; return false }, func(PrivateB4DefinitionBorrow) { t.Fatal("ASSERT_C17_LEGACY_FALSE_PUBLISHED") }); status != PrivateB4Unavailable {
		t.Fatalf("ASSERT_C17_LEGACY_FALSE_STATUS status=%s", status)
	}
	if _, status := f.manager.ConsumePrivateB4DefinitionBorrowed(f.lease, f.selection, func(PrivateB4DefinitionBorrow) bool { prepared++; return true }); status != PrivateB4Selected || prepared != 2 {
		t.Fatalf("ASSERT_C17_LEGACY_CONSUME status=%s prepared=%d", status, prepared)
	}
}
