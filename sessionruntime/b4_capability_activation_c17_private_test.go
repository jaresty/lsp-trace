package sessionruntime

import (
	"bytes"
	"errors"
	"testing"

	"lsp-trace/internal/session"
)

func TestPrivateB4C17CapabilityOwnerResponderSemantics(t *testing.T) {
	profile, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	tests := []struct {
		name, method, params string
		wantCount            uint64
	}{
		{"valid register", "client/registerCapability", `{"registrations":[{"id":"a","method":"m"}]}`, 1},
		{"valid multi unregister", "client/unregisterCapability", string(capabilityParams("unregisterations", 3)), 3},
		{"valid empty", "client/registerCapability", `{"registrations":[]}`, 0},
		{"malformed recognized", "client/registerCapability", `{"registrations":[{"id":1,"method":"m"}]}`, 0},
		{"non capability", "workspace/unknown", `{}`, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			before := profile.eventSnapshot().Admitted
			called := 0
			got, poison := writePrivateB4CapabilityResponseC17(profile, tc.method, []byte(tc.params), func() (session.Failure, bool) {
				called++
				return "", false
			})
			if got != "" || poison || called != 1 {
				t.Fatalf("ASSERT_C17_CAPABILITY_OWNER_RESPONSE failure=%q poison=%t called=%d", got, poison, called)
			}
			if after := profile.eventSnapshot().Admitted; after != before+tc.wantCount {
				t.Fatalf("ASSERT_C17_CAPABILITY_OWNER_CHARGE before=%d after=%d wantDelta=%d", before, after, tc.wantCount)
			}
		})
	}
}

func TestPrivateB4C17CapabilityOwnerPreservesFailurePoisonAndRollback(t *testing.T) {
	profile, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	wantFailure := session.SessionPoisoned
	got, poison := writePrivateB4CapabilityResponseC17(profile, "client/registerCapability", capabilityParams("registrations", 3), func() (session.Failure, bool) {
		return wantFailure, true
	})
	if got != wantFailure || !poison {
		t.Fatalf("ASSERT_C17_CAPABILITY_OWNER_FAILURE_TUPLE failure=%q poison=%t", got, poison)
	}
	if snapshot := profile.eventSnapshot(); snapshot.Admitted != 0 || snapshot.InFlight != 0 || snapshot.ActiveCallbacks != 0 {
		t.Fatalf("ASSERT_C17_CAPABILITY_OWNER_FAILURE_ROLLBACK snapshot=%+v", snapshot)
	}
}

func TestPrivateB4C17CapabilityDuplicateValidationPrecedesCapacity(t *testing.T) {
	profile, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	if got, poison := profile.withCapabilityBatchAdmission(privateB4EventLimit-1, func() (session.Failure, bool) { return "", false }); got != "" || poison {
		t.Fatalf("ASSERT_C17_CAPABILITY_DUPLICATE_PREFILL failure=%q poison=%t", got, poison)
	}
	before := profile.eventSnapshot()
	called := false
	duplicate := []byte(`{"registrations":[{"id":"same","method":"m"},{"id":"same","method":"m"}]}`)
	got, poison := writePrivateB4CapabilityResponseC17(profile, "client/registerCapability", duplicate, func() (session.Failure, bool) {
		called = true
		return "", false
	})
	if got != "" || poison || !called {
		t.Fatalf("ASSERT_C17_CAPABILITY_DUPLICATE_PRESERVES_RESPONDER failure=%q poison=%t called=%t", got, poison, called)
	}
	after := profile.eventSnapshot()
	if after != before || after.TerminalRequested || after.TerminalFailure != "" {
		t.Fatalf("ASSERT_C17_CAPABILITY_DUPLICATE_BEFORE_CAPACITY before=%+v after=%+v", before, after)
	}
}

func TestPrivateB4C17CapabilityOwnerCapacitySuppressesTransport(t *testing.T) {
	profile, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	if got, poison := profile.withCapabilityBatchAdmission(privateB4EventLimit, func() (session.Failure, bool) { return "", false }); got != "" || poison {
		t.Fatalf("ASSERT_C17_CAPABILITY_OWNER_FILL failure=%q poison=%t", got, poison)
	}
	called := false
	got, poison := writePrivateB4CapabilityResponseC17(profile, "client/unregisterCapability", []byte(`{"unregisterations":[{"id":"next","method":"m"}]}`), func() (session.Failure, bool) {
		called = true
		return "", false
	})
	if got != session.ResourceExhausted || poison || called {
		t.Fatalf("ASSERT_C17_CAPABILITY_OWNER_ATOMIC_REFUSAL failure=%q poison=%t called=%t", got, poison, called)
	}
}

func TestPrivateB4C17CapabilityDefaultOffAndMalformedPreserveResponderBytes(t *testing.T) {
	cases := []struct {
		name, method, params string
	}{
		{"default off", "client/registerCapability", `{"registrations":[{"id":"a","method":"m"}]}`},
		{"malformed", "client/registerCapability", `{"registrations":[{"id":1,"method":"m"}]}`},
		{"empty", "client/unregisterCapability", `{"unregisterations":[]}`},
		{"non capability", "workspace/unknown", `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := []byte("canonical method-not-found response")
			var got bytes.Buffer
			responder := func() (session.Failure, bool) { _, _ = got.Write(want); return "", false }
			var profile *privateB4EventAccountC17
			if tc.name != "default off" {
				profile, _ = newPrivateB4EventAccountC17()
			}
			failure, poison := writePrivateB4CapabilityResponseC17(profile, tc.method, []byte(tc.params), responder)
			if failure != "" || poison || !bytes.Equal(got.Bytes(), want) {
				t.Fatalf("ASSERT_C17_CAPABILITY_RESPONDER_PRESERVED failure=%q poison=%t got=%q", failure, poison, got.Bytes())
			}
		})
	}
}

func TestPrivateB4C17CapabilityOwnerPanicRollback(t *testing.T) {
	profile, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("ASSERT_C17_CAPABILITY_OWNER_PANIC_PROPAGATED")
			}
		}()
		_, _ = writePrivateB4CapabilityResponseC17(profile, "client/registerCapability", capabilityParams("registrations", 3), func() (session.Failure, bool) {
			panic(errors.New("transport panic"))
		})
	}()
	if snapshot := profile.eventSnapshot(); snapshot.Admitted != 0 || snapshot.InFlight != 0 || snapshot.ActiveCallbacks != 0 {
		t.Fatalf("ASSERT_C17_CAPABILITY_OWNER_PANIC_ROLLBACK snapshot=%+v", snapshot)
	}
}
