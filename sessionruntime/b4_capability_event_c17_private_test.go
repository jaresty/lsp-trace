package sessionruntime

import (
	"errors"
	"fmt"
	"testing"

	"lsp-trace/internal/session"
)

func capabilityParams(key string, count int) []byte {
	body := []byte(`{"` + key + `":[`)
	for i := 0; i < count; i++ {
		if i != 0 {
			body = append(body, ',')
		}
		body = append(body, fmt.Sprintf(`{"id":"id-%d","method":"method/%d","extension":true}`, i, i)...)
	}
	return append(body, ']', '}')
}

func TestPrivateB4C17CapabilityGrammarAndNormalization(t *testing.T) {
	tests := []struct {
		name, method, params string
		valid                bool
		count                uint64
	}{
		{"register", "client/registerCapability", `{"registrations":[{"id":"a","method":"m","registerOptions":{"nested":{"x":1,"x":2}}}]}`, true, 1},
		{"unregister historical spelling", "client/unregisterCapability", `{"unregisterations":[{"id":"a","method":"m"}]}`, true, 1},
		{"empty register", "client/registerCapability", `{"registrations":[]}`, true, 0},
		{"empty unregister", "client/unregisterCapability", `{"unregisterations":[]}`, true, 0},
		{"unknown params and entry members", "client/registerCapability", `{"future":1,"registrations":[{"id":"a","method":"m","future":2}]}`, true, 1},
		{"wrong corrected unregister spelling", "client/unregisterCapability", `{"unregistrations":[]}`, false, 0},
		{"duplicate params member", "client/registerCapability", `{"registrations":[],"registrations":[]}`, false, 0},
		{"duplicate entry member", "client/registerCapability", `{"registrations":[{"id":"a","id":"b","method":"m"}]}`, false, 0},
		{"request local duplicate identity", "client/registerCapability", `{"registrations":[{"id":"a","method":"m"},{"id":"a","method":"m"}]}`, false, 0},
		{"missing id", "client/registerCapability", `{"registrations":[{"method":"m"}]}`, false, 0},
		{"empty method", "client/registerCapability", `{"registrations":[{"id":"a","method":""}]}`, false, 0},
		{"wrong list type", "client/registerCapability", `{"registrations":{}}`, false, 0},
		{"trailing json", "client/registerCapability", `{"registrations":[]}x`, false, 0},
		{"non capability", "workspace/unknown", `{"registrations":[]}`, false, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			batch, valid := parsePrivateB4CapabilityBatchC17(tc.method, []byte(tc.params))
			if valid != tc.valid || batch.Count != tc.count {
				t.Fatalf("ASSERT_C17_CAPABILITY_NORMALIZATION valid=%t count=%d wantValid=%t wantCount=%d", valid, batch.Count, tc.valid, tc.count)
			}
		})
	}
}

func TestPrivateB4C17CapabilityArbitraryAtomicBatchAndCapacity(t *testing.T) {
	profile, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	called := 0
	if got, poison := profile.withCapabilityBatchAdmission(3, func() (session.Failure, bool) {
		called++
		return "", false
	}); got != "" || poison || called != 1 {
		t.Fatalf("ASSERT_C17_CAPABILITY_GT_TWO failure=%q poison=%t called=%d", got, poison, called)
	}
	if snapshot := profile.eventSnapshot(); snapshot.Admitted != 3 || snapshot.InFlight != 0 || snapshot.ActiveCallbacks != 0 {
		t.Fatalf("ASSERT_C17_CAPABILITY_GT_TWO_COMMIT snapshot=%+v", snapshot)
	}
	if got, poison := profile.withCapabilityBatchAdmission(privateB4EventLimit-3, func() (session.Failure, bool) { called++; return "", false }); got != "" || poison {
		t.Fatalf("ASSERT_C17_CAPABILITY_EQUALITY failure=%q poison=%t", got, poison)
	}
	before := profile.eventSnapshot()
	if got, poison := profile.withCapabilityBatchAdmission(1, func() (session.Failure, bool) { called++; return "", false }); got != session.ResourceExhausted || poison {
		t.Fatalf("ASSERT_C17_CAPABILITY_PLUS_ONE failure=%q poison=%t", got, poison)
	}
	if called != 2 {
		t.Fatalf("ASSERT_C17_CAPABILITY_REFUSAL_SUPPRESSES_CALLBACK called=%d", called)
	}
	after := profile.eventSnapshot()
	if after.Admitted != before.Admitted || !after.TerminalRequested || after.TerminalFailure != session.ResourceExhausted {
		t.Fatalf("ASSERT_C17_CAPABILITY_REFUSAL_CUMULATIVE before=%+v after=%+v", before, after)
	}
}

func TestPrivateB4C17CapabilityCallbackRollbackTupleAndOrdinal(t *testing.T) {
	profile, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	wantFailure := session.SessionPoisoned
	if got, poison := profile.withCapabilityBatchAdmission(4, func() (session.Failure, bool) { return wantFailure, true }); got != wantFailure || !poison {
		t.Fatalf("ASSERT_C17_CAPABILITY_CALLBACK_TUPLE failure=%q poison=%t", got, poison)
	}
	if snapshot := profile.eventSnapshot(); snapshot.Admitted != 0 || snapshot.InFlight != 0 || snapshot.ActiveCallbacks != 0 || snapshot.TerminalRequested {
		t.Fatalf("ASSERT_C17_CAPABILITY_CALLBACK_ROLLBACK snapshot=%+v", snapshot)
	}
	var first uint64
	if got, poison := profile.withCapabilityBatchAdmissionObserved(2, func(start uint64) (session.Failure, bool) { first = start; return "", false }); got != "" || poison {
		t.Fatalf("ASSERT_C17_CAPABILITY_RETRY failure=%q poison=%t", got, poison)
	}
	var second uint64
	if got, poison := profile.withCapabilityBatchAdmissionObserved(1, func(start uint64) (session.Failure, bool) { second = start; return "", false }); got != "" || poison {
		t.Fatalf("ASSERT_C17_CAPABILITY_SECOND_COMMIT failure=%q poison=%t", got, poison)
	}
	if first != 0 || second != 2 {
		t.Fatalf("ASSERT_C17_CAPABILITY_COMMITTED_ORDINALS first=%d second=%d", first, second)
	}
}

func TestPrivateB4C17CapabilityPanicRollsBackAndReusesOrdinal(t *testing.T) {
	profile, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("ASSERT_C17_CAPABILITY_PANIC_PROPAGATED")
			}
		}()
		_, _ = profile.withCapabilityBatchAdmissionObserved(2, func(uint64) (session.Failure, bool) { panic(errors.New("boom")) })
	}()
	if snapshot := profile.eventSnapshot(); snapshot.Admitted != 0 || snapshot.InFlight != 0 || snapshot.ActiveCallbacks != 0 {
		t.Fatalf("ASSERT_C17_CAPABILITY_PANIC_ROLLBACK snapshot=%+v", snapshot)
	}
	var start uint64
	_, _ = profile.withCapabilityBatchAdmissionObserved(1, func(ordinal uint64) (session.Failure, bool) { start = ordinal; return "", false })
	if start != 0 {
		t.Fatalf("ASSERT_C17_CAPABILITY_PANIC_ORDINAL_REUSE start=%d", start)
	}
}
