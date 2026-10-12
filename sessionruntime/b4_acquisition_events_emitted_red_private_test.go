package sessionruntime

import (
	"errors"
	"math"
	"testing"

	"lsp-trace/internal/adr0011work"
	"lsp-trace/internal/session"
)

func privateB4C17AcquisitionWorkFixture(t *testing.T, content []byte, identity string) (*Manager, DocumentRequest, B4DefinitionSourceLease, *privateB4Reservation, *privateB4EventAccountC17, *adr0011work.Account) {
	t.Helper()
	manager, req, lease := privateB4PreparedSource(t, content)
	profile, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatalf("BLOCKED_NOT_RED: C17 acquisition profile: %s", failure)
	}
	account := adr0011work.New(identity)
	reservation := &privateB4Reservation{
		sourceLeases: []B4DefinitionSourceLease{lease},
		selection:    B4DefinitionSelectionKey{Transaction: identity},
		workAccount:  account,
	}
	reservation.token[0] = 1
	return manager, req, lease, reservation, profile, account
}

func privateB4C17AdmitAcquisitionWork(t *testing.T, manager *Manager, req DocumentRequest, reservation *privateB4Reservation, profile *privateB4EventAccountC17) session.Failure {
	t.Helper()
	manager.mu.Lock()
	defer manager.mu.Unlock()
	return manager.admitPrivateB4SourcesC17Locked(privateB4C17AcquisitionRequest(req), reservation, profile)
}

func TestADR0011C17AcquisitionEventsEmittedAcceptedBatchChargesTwoPlusTwoPerElement(t *testing.T) {
	for _, tc := range []struct {
		name       string
		withSource bool
		want       uint64
	}{{name: "zero", want: 2}, {name: "one", withSource: true, want: 4}} {
		t.Run(tc.name, func(t *testing.T) {
			manager, req, lease, reservation, profile, account := privateB4C17AcquisitionWorkFixture(t, []byte("package acquisitionevents\n"), "acquisition-events-"+tc.name)
			if !tc.withSource {
				reservation.sourceLeases = nil
			}
			if failure := privateB4C17AdmitAcquisitionWork(t, manager, req, reservation, profile); failure != "" {
				t.Fatalf("ASSERT_ACQUISITION_EVENTS_ACCEPTED failure=%s", failure)
			}
			if got := account.Snapshot(); got.EventsEmitted != tc.want || got.Terminal {
				t.Fatalf("ASSERT_ACQUISITION_EVENTS_2_PLUS_2N snapshot=%+v want=%d", got, tc.want)
			}
			if got := profile.eventSnapshot(); got.Admitted != tc.want || got.InFlight != 0 {
				t.Fatalf("BLOCKED_NOT_RED: C17 finalization snapshot=%+v want=%d", got, tc.want)
			}
			if tc.withSource && lease.state.state.Load() != privateB4SourceTransferred {
				t.Fatalf("ASSERT_ACQUISITION_EVENTS_FINALIZED_WITH_TRANSFER state=%d", lease.state.state.Load())
			}
		})
	}
}

func TestADR0011C17AcquisitionEventsEmittedWorkOverflowPrecedesIrreversibleTransfer(t *testing.T) {
	manager, req, lease, reservation, profile, account := privateB4C17AcquisitionWorkFixture(t, []byte("package acquisitionoverflow\n"), "acquisition-events-overflow")
	if err := account.ChargeEventsEmitted("acquisition-events-overflow", math.MaxUint64-3); err != nil {
		t.Fatalf("BLOCKED_NOT_RED: seed independent work account: %v", err)
	}
	beforeC17 := profile.eventSnapshot()
	failure := privateB4C17AdmitAcquisitionWork(t, manager, req, reservation, profile)
	if failure != session.ResourceExhausted {
		t.Fatalf("ASSERT_ACQUISITION_EVENTS_OVERFLOW_FAILURE failure=%s", failure)
	}
	if got := account.Snapshot(); got.EventsEmitted != math.MaxUint64-3 || !got.Terminal || !errors.Is(got.Failure, adr0011work.ErrOverflow) {
		t.Fatalf("ASSERT_ACQUISITION_EVENTS_OVERFLOW_TYPED snapshot=%+v", got)
	}
	if got := profile.eventSnapshot(); got != beforeC17 {
		t.Fatalf("ASSERT_ACQUISITION_EVENTS_WORK_PRECEDES_C17 before=%+v after=%+v", beforeC17, got)
	}
	manager.mu.Lock()
	leases := manager.privateB4LeaseCountLocked()
	manager.mu.Unlock()
	if lease.state.state.Load() != privateB4SourceHeld || leases != 0 {
		t.Fatalf("ASSERT_ACQUISITION_EVENTS_OVERFLOW_NO_TRANSFER_NO_LEASE state=%d leases=%d", lease.state.state.Load(), leases)
	}
}

func TestADR0011C17AcquisitionEventsEmittedC17RefusalAndPreFinalizationFailureRollback(t *testing.T) {
	t.Run("C17 refusal charges zero", func(t *testing.T) {
		manager, req, lease, reservation, profile, account := privateB4C17AcquisitionWorkFixture(t, []byte("package acquisitionrefusal\n"), "acquisition-events-refusal")
		for i := 0; i < privateB4EventLimit-3; i++ {
			if err := profile.WithEventAdmission(c17Event(i), func() error { return nil }); err != nil {
				t.Fatal(err)
			}
		}
		if failure := privateB4C17AdmitAcquisitionWork(t, manager, req, reservation, profile); failure != session.ResourceExhausted {
			t.Fatalf("ASSERT_ACQUISITION_EVENTS_C17_REFUSAL failure=%s", failure)
		}
		if got := account.Snapshot(); got.EventsEmitted != 0 || got.Terminal || lease.state.state.Load() != privateB4SourceHeld {
			t.Fatalf("ASSERT_ACQUISITION_EVENTS_C17_REFUSAL_ZERO snapshot=%+v state=%d", got, lease.state.state.Load())
		}
	})

	t.Run("source transfer failure rolls back work reservation", func(t *testing.T) {
		manager, req, lease, reservation, profile, account := privateB4C17AcquisitionWorkFixture(t, []byte("package acquisitionrollback\n"), "acquisition-events-rollback")
		bytesAccount, failure := newPrivateB4ByteAccountV2()
		if failure != "" {
			t.Fatal(failure)
		}
		profile, failure = newPrivateB4EventAccountC17WithBytes(bytesAccount)
		if failure != "" {
			t.Fatal(failure)
		}
		reservation.explicitBytes = bytesAccount
		blocker, failure := bytesAccount.table.reserveDescriptor(1)
		if failure != "" {
			t.Fatal(failure)
		}
		defer blocker.release()
		if failure := privateB4C17AdmitAcquisitionWork(t, manager, req, reservation, profile); failure != session.ResourceExhausted {
			t.Fatalf("ASSERT_ACQUISITION_EVENTS_PRE_FINALIZATION_FAILURE failure=%s", failure)
		}
		if got := account.Snapshot(); got.EventsEmitted != 0 || got.Terminal || lease.state.state.Load() != privateB4SourceHeld {
			t.Fatalf("ASSERT_ACQUISITION_EVENTS_PRE_FINALIZATION_ROLLBACK snapshot=%+v state=%d", got, lease.state.state.Load())
		}
	})
}

func TestADR0011C17AcquisitionEventsEmittedIdentityPreEffectAndLifecycle(t *testing.T) {
	t.Run("identity mismatch precedes effects", func(t *testing.T) {
		manager, req, lease, reservation, profile, account := privateB4C17AcquisitionWorkFixture(t, []byte("package acquisitionidentity\n"), "acquisition-events-owner")
		reservation.selection.Transaction = "different-acquisition-owner"
		before := profile.eventSnapshot()
		if failure := privateB4C17AdmitAcquisitionWork(t, manager, req, reservation, profile); failure != session.ResourceExhausted {
			t.Fatalf("ASSERT_ACQUISITION_EVENTS_IDENTITY_TYPED failure=%s", failure)
		}
		if got := account.Snapshot(); got.EventsEmitted != 0 || got.Terminal || profile.eventSnapshot() != before || lease.state.state.Load() != privateB4SourceHeld {
			t.Fatalf("ASSERT_ACQUISITION_EVENTS_IDENTITY_PRE_EFFECT snapshot=%+v c17=%+v state=%d", got, profile.eventSnapshot(), lease.state.state.Load())
		}
	})

	t.Run("committed work survives later transaction failure and account remains installed", func(t *testing.T) {
		manager, req, _, reservation, profile, account := privateB4C17AcquisitionWorkFixture(t, []byte("package acquisitionlifecycle\n"), "acquisition-events-lifecycle")
		if failure := privateB4C17AdmitAcquisitionWork(t, manager, req, reservation, profile); failure != "" {
			t.Fatal(failure)
		}
		reservation.selection.SessionID = req.SessionID
		reservation.selection.Key.Generation = req.Generation
		reservation.session = manager.sessions[req.SessionID]
		reservation.capture = PrivateB4DefinitionCapture{
			SessionID: req.SessionID, Key: reservation.selection.Key, Transaction: reservation.selection.Transaction,
			ResponseFrame: []byte{1},
		}
		resultCharge, failure := reservation.explicitBytes.reserve(3)
		if failure != "" {
			t.Fatalf("BLOCKED_NOT_RED: result owner charge: %s", failure)
		}
		reservation.resultOwner = &privateB4ResultOwner{
			state: privateB4ResultOpen, managerHeld: true,
			bytes: []byte("abc"), charge: &privateB4SuccessorAllocationLease{lease: resultCharge},
		}
		transaction := &privateB4Transaction{legacy: reservation, c17: &privateB4ReservationC17{legacy: reservation, profile: profile}, workAccount: account}
		manager.mu.Lock()
		manager.installPrivateB4TransactionLocked(privateB4C17AcquisitionRequest(req), manager.sessions[req.SessionID], transaction)
		installed := manager.privateB4Leases[reservation.slot].workAccount
		manager.mu.Unlock()
		if got := account.Snapshot(); installed != account || got.EventsEmitted != 4 || got.Terminal {
			t.Fatalf("ASSERT_ACQUISITION_EVENTS_ACCOUNT_ACTIVE_THROUGH_INSTALL installed=%p want=%p snapshot=%+v", installed, account, got)
		}

		callbackCalled := false
		_, status, err := manager.CommitPrivateB4DefinitionBorrowedC17(
			B4DefinitionLease{token: reservation.token}, reservation.selection,
			func(PrivateB4DefinitionBorrow) bool { return true },
			func(borrow PrivateB4DefinitionBorrow) error {
				return borrow.WithTargetAppendAdmission(math.MaxUint64, func() error {
					callbackCalled = true
					return nil
				})
			},
		)
		if status != PrivateB4Selected || !errors.Is(err, errPrivateB4EventAdmission) || callbackCalled {
			t.Fatalf("ASSERT_ACQUISITION_EVENTS_REAL_LATER_FAILURE status=%s err=%v callback=%t", status, err, callbackCalled)
		}
		if got := account.Snapshot(); got.EventsEmitted != 4 || !got.Terminal {
			t.Fatalf("ASSERT_ACQUISITION_EVENTS_LATER_FAILURE_PERSISTS_AND_OWNERSHIP_SEALS snapshot=%+v", got)
		}
	})
}

func TestADR0011C17AcquisitionEventsEmittedDefaultOffUnchanged(t *testing.T) {
	manager, req, lease, reservation, _, account := privateB4C17AcquisitionWorkFixture(t, []byte("package acquisitiondefaultoff\n"), "acquisition-events-default-off")
	manager.mu.Lock()
	failure := manager.admitPrivateB4SourcesLocked(privateB4C17AcquisitionRequest(req), reservation)
	manager.mu.Unlock()
	if failure != "" || lease.state.state.Load() != privateB4SourceTransferred {
		t.Fatalf("BLOCKED_NOT_RED: legacy acquisition failure=%s state=%d", failure, lease.state.state.Load())
	}
	if got := account.Snapshot(); got.EventsEmitted != 0 || got.Terminal {
		t.Fatalf("ASSERT_ACQUISITION_EVENTS_DEFAULT_OFF_UNCHANGED snapshot=%+v", got)
	}
}

func TestADR0011C17AcquisitionEventsEmittedPanicRollbackUnavailable(t *testing.T) {
	t.Skip("BLOCKED_NOT_RED: real acquisition fixture exposes no pre-finalization panic injection boundary")
}
