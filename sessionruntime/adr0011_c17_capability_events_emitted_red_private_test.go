package sessionruntime

import (
	"errors"
	"math"
	"sync/atomic"
	"testing"

	"lsp-trace/internal/adr0011work"
	"lsp-trace/internal/session"
)

func c17CapabilityEventsTransaction(t *testing.T, identity string, enabled bool) (*privateB4Transaction, *privateB4EventAccountC17, *adr0011work.Account) {
	t.Helper()
	account := adr0011work.New(identity)
	reservation := &privateB4Reservation{
		selection:   B4DefinitionSelectionKey{Transaction: identity},
		workAccount: account,
	}
	transaction := &privateB4Transaction{legacy: reservation, workAccount: account}
	if !enabled {
		return transaction, nil, account
	}
	profile, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatalf("BLOCKED_NOT_RED: C17 capability profile: %s", failure)
	}
	transaction.c17 = &privateB4ReservationC17{legacy: reservation, profile: profile}
	return transaction, profile, account
}

func writeC17CapabilityEvents(
	transaction *privateB4Transaction,
	method string,
	params []byte,
	responder func() (session.Failure, bool),
) (session.Failure, bool) {
	// Compile RED is intentional: the real responder boundary must receive the
	// existing request transaction so it can derive both C17 and exact work
	// account custody. A test hook or an independently supplied account would
	// not establish production custody.
	return writePrivateB4CapabilityResponseC17(transaction, method, params, responder)
}

func TestADR0011C17CapabilityEventsEmittedNormalizedBatchAndZeroCases(t *testing.T) {
	transaction, profile, account := c17CapabilityEventsTransaction(t, "capability-events-normalized", true)
	for _, tc := range []struct {
		name, method, params string
		want                 uint64
	}{
		{name: "registration", method: "client/registerCapability", params: string(capabilityParams("registrations", 3)), want: 3},
		{name: "unregistration", method: "client/unregisterCapability", params: string(capabilityParams("unregisterations", 2)), want: 2},
		{name: "empty", method: "client/registerCapability", params: `{"registrations":[]}`},
		{name: "malformed", method: "client/registerCapability", params: `{"registrations":[`},
		{name: "duplicate", method: "client/registerCapability", params: `{"registrations":[{"id":"same","method":"m"},{"id":"same","method":"m"}]}`},
		{name: "unrecognized", method: "workspace/unknown", params: `{"registrations":[{"id":"a","method":"m"}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := account.Snapshot().EventsEmitted
			called := 0
			failure, poison := writeC17CapabilityEvents(transaction, tc.method, []byte(tc.params), func() (session.Failure, bool) {
				called++
				return "", false
			})
			if failure != "" || poison || called != 1 {
				t.Fatalf("ASSERT_CAPABILITY_EVENTS_RESPONDER failure=%s poison=%t called=%d", failure, poison, called)
			}
			if got := account.Snapshot(); got.EventsEmitted != before+tc.want || got.Terminal {
				t.Fatalf("ASSERT_CAPABILITY_EVENTS_EXACT_BATCH before=%d want_delta=%d snapshot=%+v", before, tc.want, got)
			}
		})
	}
	if got := profile.eventSnapshot(); got.Admitted != 5 || got.InFlight != 0 {
		t.Fatalf("BLOCKED_NOT_RED: C17 normalized batch authority snapshot=%+v", got)
	}

	off, _, offAccount := c17CapabilityEventsTransaction(t, "capability-events-default-off", false)
	called := false
	if failure, poison := writeC17CapabilityEvents(off, "client/registerCapability", capabilityParams("registrations", 2), func() (session.Failure, bool) {
		called = true
		return "", false
	}); failure != "" || poison || !called {
		t.Fatalf("ASSERT_CAPABILITY_EVENTS_DEFAULT_OFF_RESPONDER failure=%s poison=%t called=%t", failure, poison, called)
	}
	if got := offAccount.Snapshot(); got.EventsEmitted != 0 || got.Terminal {
		t.Fatalf("ASSERT_CAPABILITY_EVENTS_DEFAULT_OFF_ZERO snapshot=%+v", got)
	}
}

func TestADR0011C17CapabilityEventsEmittedFailurePoisonPanicRollbackAndOrdinalReuse(t *testing.T) {
	transaction, profile, account := c17CapabilityEventsTransaction(t, "capability-events-rollback", true)
	wantFailure := session.SessionPoisoned
	if failure, poison := writeC17CapabilityEvents(transaction, "client/registerCapability", capabilityParams("registrations", 3), func() (session.Failure, bool) {
		return wantFailure, true
	}); failure != wantFailure || !poison {
		t.Fatalf("ASSERT_CAPABILITY_EVENTS_FAILURE_POISON failure=%s poison=%t", failure, poison)
	}
	if got := account.Snapshot(); got.EventsEmitted != 0 || got.Terminal {
		t.Fatalf("ASSERT_CAPABILITY_EVENTS_FAILURE_ROLLBACK snapshot=%+v", got)
	}

	panicValue := &struct{ name string }{"capability responder panic"}
	func() {
		defer func() {
			if got := recover(); got != panicValue {
				t.Fatalf("ASSERT_CAPABILITY_EVENTS_REPANIC got=%#v", got)
			}
		}()
		_, _ = writeC17CapabilityEvents(transaction, "client/registerCapability", capabilityParams("registrations", 2), func() (session.Failure, bool) {
			panic(panicValue)
		})
	}()
	if got := account.Snapshot(); got.EventsEmitted != 0 || got.Terminal {
		t.Fatalf("ASSERT_CAPABILITY_EVENTS_PANIC_ROLLBACK snapshot=%+v", got)
	}
	var start uint64
	if failure, poison := profile.withCapabilityBatchAdmissionObserved(1, func(ordinal uint64) (session.Failure, bool) {
		start = ordinal
		return "", false
	}); failure != "" || poison || start != 0 {
		t.Fatalf("ASSERT_CAPABILITY_EVENTS_C17_ORDINAL_REUSE failure=%s poison=%t start=%d", failure, poison, start)
	}
}

func TestADR0011C17CapabilityEventsEmittedIdentityAndOverflowPreResponder(t *testing.T) {
	t.Run("identity mismatch", func(t *testing.T) {
		transaction, profile, account := c17CapabilityEventsTransaction(t, "capability-events-owner", true)
		transaction.legacy.selection.Transaction = "different-capability-events-owner"
		before := profile.eventSnapshot()
		called := false
		failure, poison := writeC17CapabilityEvents(transaction, "client/registerCapability", capabilityParams("registrations", 1), func() (session.Failure, bool) {
			called = true
			return "", false
		})
		if failure != session.ResourceExhausted || poison || called {
			t.Fatalf("ASSERT_CAPABILITY_EVENTS_IDENTITY_PRE_EFFECT failure=%s poison=%t called=%t", failure, poison, called)
		}
		if got := account.Snapshot(); got.EventsEmitted != 0 || got.Terminal || profile.eventSnapshot() != before {
			t.Fatalf("ASSERT_CAPABILITY_EVENTS_IDENTITY_ZERO snapshot=%+v c17=%+v", got, profile.eventSnapshot())
		}
	})

	t.Run("work overflow", func(t *testing.T) {
		transaction, profile, account := c17CapabilityEventsTransaction(t, "capability-events-overflow", true)
		if err := account.ChargeEventsEmitted("capability-events-overflow", math.MaxUint64-1); err != nil {
			t.Fatalf("BLOCKED_NOT_RED: seed independent events account: %v", err)
		}
		before := profile.eventSnapshot()
		called := false
		failure, poison := writeC17CapabilityEvents(transaction, "client/unregisterCapability", capabilityParams("unregisterations", 2), func() (session.Failure, bool) {
			called = true
			return "", false
		})
		if failure != session.ResourceExhausted || poison || called {
			t.Fatalf("ASSERT_CAPABILITY_EVENTS_OVERFLOW_PRE_RESPONDER failure=%s poison=%t called=%t", failure, poison, called)
		}
		if got := account.Snapshot(); got.EventsEmitted != math.MaxUint64-1 || !got.Terminal || !errors.Is(got.Failure, adr0011work.ErrOverflow) {
			t.Fatalf("ASSERT_CAPABILITY_EVENTS_OVERFLOW_TYPED snapshot=%+v", got)
		}
		if got := profile.eventSnapshot(); got != before {
			t.Fatalf("ASSERT_CAPABILITY_EVENTS_OVERFLOW_PRE_C17 before=%+v after=%+v", before, got)
		}
	})
}

func TestADR0011C17CapabilityEventsEmittedConcurrentReservationsCannotEvadeOverflow(t *testing.T) {
	account := adr0011work.New("capability-events-concurrent")
	first, _, _ := c17CapabilityEventsTransaction(t, "unused-first", true)
	second, _, _ := c17CapabilityEventsTransaction(t, "unused-second", true)
	for _, transaction := range []*privateB4Transaction{first, second} {
		transaction.workAccount = account
		transaction.legacy.workAccount = account
		transaction.legacy.selection.Transaction = "capability-events-concurrent"
	}
	if err := account.ChargeEventsEmitted("capability-events-concurrent", math.MaxUint64-1); err != nil {
		t.Fatalf("BLOCKED_NOT_RED: seed concurrent events account: %v", err)
	}

	entered := make(chan struct{})
	release := make(chan struct{})
	firstDone := make(chan session.Failure, 1)
	go func() {
		failure, _ := writeC17CapabilityEvents(first, "client/registerCapability", capabilityParams("registrations", 1), func() (session.Failure, bool) {
			close(entered)
			<-release
			return "", false
		})
		firstDone <- failure
	}()
	<-entered
	var secondCalled atomic.Bool
	failure, poison := writeC17CapabilityEvents(second, "client/registerCapability", capabilityParams("registrations", 1), func() (session.Failure, bool) {
		secondCalled.Store(true)
		return "", false
	})
	close(release)
	if firstFailure := <-firstDone; firstFailure != "" {
		t.Fatalf("ASSERT_CAPABILITY_EVENTS_FIRST_RESERVATION failure=%s", firstFailure)
	}
	if failure != session.ResourceExhausted || poison || secondCalled.Load() {
		t.Fatalf("ASSERT_CAPABILITY_EVENTS_CONCURRENT_OVERFLOW failure=%s poison=%t called=%t", failure, poison, secondCalled.Load())
	}
	if got := account.Snapshot(); got.EventsEmitted != math.MaxUint64 || !got.Terminal || !errors.Is(got.Failure, adr0011work.ErrOverflow) {
		t.Fatalf("ASSERT_CAPABILITY_EVENTS_CONCURRENT_EXACT snapshot=%+v", got)
	}
}

func TestADR0011C17CapabilityEventsEmittedC17IsNotWorkAuthorityAndCommittedChargePersists(t *testing.T) {
	transaction, profile, account := c17CapabilityEventsTransaction(t, "capability-events-authority", true)
	if failure, poison := profile.withCapabilityBatchAdmission(privateB4EventLimit, func() (session.Failure, bool) { return "", false }); failure != "" || poison {
		t.Fatalf("BLOCKED_NOT_RED: fill independent C17 authority failure=%s poison=%t", failure, poison)
	}
	called := false
	failure, poison := writeC17CapabilityEvents(transaction, "client/registerCapability", capabilityParams("registrations", 1), func() (session.Failure, bool) {
		called = true
		return "", false
	})
	if failure != session.ResourceExhausted || poison || called {
		t.Fatalf("ASSERT_CAPABILITY_EVENTS_C17_REFUSAL failure=%s poison=%t called=%t", failure, poison, called)
	}
	if got := account.Snapshot(); got.EventsEmitted != 0 || got.Terminal {
		t.Fatalf("ASSERT_CAPABILITY_EVENTS_C17_NON_AUTHORITY snapshot=%+v", got)
	}

	fresh, _, freshAccount := c17CapabilityEventsTransaction(t, "capability-events-later-failure", true)
	if failure, poison := writeC17CapabilityEvents(fresh, "client/registerCapability", capabilityParams("registrations", 2), func() (session.Failure, bool) {
		return "", false
	}); failure != "" || poison {
		t.Fatalf("ASSERT_CAPABILITY_EVENTS_COMMIT failure=%s poison=%t", failure, poison)
	}
	// A later, separate production response failure is not part of the already
	// committed capability batch and therefore cannot refund it.
	if failure, poison := writeC17CapabilityEvents(fresh, "workspace/unknown", []byte(`{}`), func() (session.Failure, bool) {
		return session.SessionPoisoned, true
	}); failure != session.SessionPoisoned || !poison {
		t.Fatalf("ASSERT_CAPABILITY_EVENTS_LATER_FAILURE failure=%s poison=%t", failure, poison)
	}
	if got := freshAccount.Snapshot(); got.EventsEmitted != 2 {
		t.Fatalf("ASSERT_CAPABILITY_EVENTS_LATER_FAILURE_NO_REFUND snapshot=%+v", got)
	}
}
