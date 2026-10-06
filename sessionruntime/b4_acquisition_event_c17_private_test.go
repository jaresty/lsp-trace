package sessionruntime

import (
	"context"
	"errors"
	"testing"
	"time"

	"lsp-trace/internal/session"
)

func privateB4C17CommittedIdentities(t *testing.T, profile *privateB4EventAccountC17) []privateB4EventIdentityC17 {
	t.Helper()
	profile.mu.Lock()
	defer profile.mu.Unlock()
	identities := make([]privateB4EventIdentityC17, 0, profile.admitted)
	for _, entry := range profile.entries {
		if entry.state == 2 {
			identities = append(identities, entry.identity)
		}
	}
	return identities
}

func privateB4C17WantAcquisition(elementCount int) []privateB4EventIdentityC17 {
	identities := make([]privateB4EventIdentityC17, 0, 2+2*elementCount)
	identities = append(identities, privateB4EventIdentityC17{Family: "ACQUISITION", Kind: "QUERY_BEGIN", Ordinal: 0})
	for ordinal := 0; ordinal < elementCount; ordinal++ {
		identities = append(identities,
			privateB4EventIdentityC17{Family: "ACQUISITION", Kind: "ELEMENT_BEGIN", Ordinal: ordinal},
			privateB4EventIdentityC17{Family: "ACQUISITION", Kind: "ELEMENT_TERMINAL", Ordinal: ordinal},
		)
	}
	return append(identities, privateB4EventIdentityC17{Family: "ACQUISITION", Kind: "QUERY_TERMINAL", Ordinal: 0})
}

func TestPrivateB4C17AcquisitionExactSequenceIncludingEmpty(t *testing.T) {
	for _, elementCount := range []int{0, 1, 3} {
		t.Run(string(rune('0'+elementCount)), func(t *testing.T) {
			profile, failure := newPrivateB4EventAccountC17()
			if failure != "" {
				t.Fatal(failure)
			}
			token, err := profile.beginAcquisitionAdmission(elementCount)
			if err != nil {
				t.Fatalf("ASSERT_C17_ACQUISITION_BEGIN elements=%d err=%v", elementCount, err)
			}
			if snapshot := profile.eventSnapshot(); snapshot.Admitted != 0 || snapshot.InFlight != uint64(2+2*elementCount) || snapshot.ActiveCallbacks != 1 {
				t.Fatalf("ASSERT_C17_ACQUISITION_PROVISIONAL elements=%d snapshot=%+v", elementCount, snapshot)
			}
			if err := token.commit(); err != nil {
				t.Fatalf("ASSERT_C17_ACQUISITION_COMMIT elements=%d err=%v", elementCount, err)
			}
			want := privateB4C17WantAcquisition(elementCount)
			got := privateB4C17CommittedIdentities(t, profile)
			if len(got) != len(want) {
				t.Fatalf("ASSERT_C17_ACQUISITION_COUNT elements=%d got=%+v want=%+v", elementCount, got, want)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("ASSERT_C17_ACQUISITION_ORDER index=%d got=%+v want=%+v", i, got[i], want[i])
				}
			}
		})
	}
}

func TestPrivateB4C17AcquisitionRollbackRetryAndExactOnceSettle(t *testing.T) {
	profile, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	token, err := profile.beginAcquisitionAdmission(2)
	if err != nil {
		t.Fatal(err)
	}
	if err := token.rollback(); err != nil {
		t.Fatalf("ASSERT_C17_ACQUISITION_ROLLBACK err=%v", err)
	}
	if snapshot := profile.eventSnapshot(); snapshot.Admitted != 0 || snapshot.InFlight != 0 || snapshot.ActiveCallbacks != 0 || snapshot.TerminalRequested {
		t.Fatalf("ASSERT_C17_ACQUISITION_ROLLBACK_ZERO_EFFECT snapshot=%+v", snapshot)
	}
	if err := token.rollback(); !errors.Is(err, errPrivateB4EventAdmission) {
		t.Fatalf("ASSERT_C17_ACQUISITION_DUPLICATE_ROLLBACK err=%v", err)
	}
	if err := token.commit(); !errors.Is(err, errPrivateB4EventAdmission) {
		t.Fatalf("ASSERT_C17_ACQUISITION_COMMIT_AFTER_ROLLBACK err=%v", err)
	}

	retry, err := profile.beginAcquisitionAdmission(2)
	if err != nil {
		t.Fatalf("ASSERT_C17_ACQUISITION_RETRY err=%v", err)
	}
	if err := retry.commit(); err != nil {
		t.Fatal(err)
	}
	before := profile.eventSnapshot()
	if err := retry.commit(); !errors.Is(err, errPrivateB4EventAdmission) {
		t.Fatalf("ASSERT_C17_ACQUISITION_DUPLICATE_COMMIT err=%v", err)
	}
	if err := retry.rollback(); !errors.Is(err, errPrivateB4EventAdmission) {
		t.Fatalf("ASSERT_C17_ACQUISITION_ROLLBACK_AFTER_COMMIT err=%v", err)
	}
	if after := profile.eventSnapshot(); after != before {
		t.Fatalf("ASSERT_C17_ACQUISITION_DUPLICATE_SETTLE_MUTATION before=%+v after=%+v", before, after)
	}
}

func TestPrivateB4C17AcquisitionEqualityPlusOneAtomic(t *testing.T) {
	atLimit, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	for i := 0; i < privateB4EventLimit-2; i++ {
		if err := atLimit.WithEventAdmission(privateB4EventIdentityC17{Family: "CAPABILITY", Kind: "CAPABILITY_ENTRY", Ordinal: i}, func() error { return nil }); err != nil {
			t.Fatal(err)
		}
	}
	token, err := atLimit.beginAcquisitionAdmission(0)
	if err != nil {
		t.Fatalf("ASSERT_C17_ACQUISITION_EVENT_8192 err=%v", err)
	}
	if err := token.commit(); err != nil {
		t.Fatal(err)
	}
	if snapshot := atLimit.eventSnapshot(); snapshot.Admitted != privateB4EventLimit || snapshot.TerminalRequested {
		t.Fatalf("ASSERT_C17_ACQUISITION_EVENT_8192_COMMITTED snapshot=%+v", snapshot)
	}

	plusOne, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	for i := 0; i < privateB4EventLimit-1; i++ {
		if err := plusOne.WithEventAdmission(privateB4EventIdentityC17{Family: "CAPABILITY", Kind: "CAPABILITY_ENTRY", Ordinal: i}, func() error { return nil }); err != nil {
			t.Fatal(err)
		}
	}
	if token, err := plusOne.beginAcquisitionAdmission(0); !errors.Is(err, errPrivateB4EventAdmission) || token != nil {
		t.Fatalf("ASSERT_C17_ACQUISITION_EVENT_8193_REFUSED token=%v err=%v", token, err)
	}
	if snapshot := plusOne.eventSnapshot(); snapshot.Admitted != privateB4EventLimit-1 || snapshot.InFlight != 0 || snapshot.ActiveCallbacks != 0 || !snapshot.TerminalRequested || snapshot.TerminalFailure != session.ResourceExhausted {
		t.Fatalf("ASSERT_C17_ACQUISITION_EVENT_8193_ATOMIC snapshot=%+v", snapshot)
	}
}

func TestPrivateB4C17AcquisitionSharesCapabilityAndTargetBudget(t *testing.T) {
	profile, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	if failure, poison := profile.withCapabilityBatchAdmission(2, func() (session.Failure, bool) { return "", false }); failure != "" || poison {
		t.Fatalf("ASSERT_C17_ACQUISITION_MIXED_CAPABILITY failure=%s poison=%t", failure, poison)
	}
	if err := profile.withTargetAppendAdmission(9, func() error { return nil }); err != nil {
		t.Fatalf("ASSERT_C17_ACQUISITION_MIXED_TARGET err=%v", err)
	}
	token, err := profile.beginAcquisitionAdmission(1)
	if err != nil {
		t.Fatal(err)
	}
	if err := token.commit(); err != nil {
		t.Fatal(err)
	}
	if snapshot := profile.eventSnapshot(); snapshot.Admitted != 8 {
		t.Fatalf("ASSERT_C17_ACQUISITION_ONE_SHARED_BUDGET snapshot=%+v", snapshot)
	}
}

func TestPrivateB4C17AcquisitionTokenJoinsTerminalRelease(t *testing.T) {
	profile, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	token, err := profile.beginAcquisitionAdmission(1)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if failure := profile.requestTerminalReleaseC17(ctx); failure != session.RequestTimeout {
		t.Fatalf("ASSERT_C17_ACQUISITION_TERMINAL_WAITS failure=%s", failure)
	}
	if snapshot := profile.eventSnapshot(); !snapshot.TerminalRequested || snapshot.BackingReleased || snapshot.ActiveCallbacks != 1 {
		t.Fatalf("ASSERT_C17_ACQUISITION_TERMINAL_PENDING snapshot=%+v", snapshot)
	}
	if err := token.rollback(); err != nil {
		t.Fatal(err)
	}
	if failure := profile.requestTerminalReleaseC17(context.Background()); failure != "" {
		t.Fatal(failure)
	}
	if snapshot := profile.eventSnapshot(); !snapshot.BackingReleased || snapshot.InFlight != 0 || snapshot.ActiveCallbacks != 0 {
		t.Fatalf("ASSERT_C17_ACQUISITION_TERMINAL_SETTLED snapshot=%+v", snapshot)
	}
}
