package sessionruntime

import (
	"errors"
	"testing"

	"lsp-trace/internal/directionalraw"
	"lsp-trace/internal/session"
)

func TestManagerDirectionalReservationDualFailureTruthAndRetryPrivate(t *testing.T) {
	reservation := directionalraw.NewReservation("manager-dual-failure")
	terminalCalls, accountReleases := 0, 0
	if !reservation.AttachPrivate(func(string, string, int) ([]byte, error) { return nil, nil }, func(directionalraw.Observation) {}, func() bool {
		terminalCalls++
		if terminalCalls == 1 {
			return false
		}
		return reservation.Terminate()
	}, func() { accountReleases++ }) {
		t.Fatal("attach")
	}
	primary := errors.New("primary directional failure")
	result := DirectionalCallHierarchyResultPrivate{}
	err := error(primary)
	finalizeDirectionalReservationPrivate(reservation, &result, &err)
	if !errors.Is(err, primary) || !errors.Is(err, directionalraw.ErrTerminalReleasePrivate) || result.Failure != session.Failure("DIRECTIONAL_RELEASE_FAILED") || reservation.Closed() || accountReleases != 0 {
		t.Fatalf("ASSERT_MANAGER_DIRECTIONAL_DUAL_FAILURE_TRUTH: err=%v result=%+v closed=%t releases=%d", err, result, reservation.Closed(), accountReleases)
	}
	result = DirectionalCallHierarchyResultPrivate{}
	err = nil
	finalizeDirectionalReservationPrivate(reservation, &result, &err)
	if err != nil || result.Failure != "" || !reservation.Closed() || terminalCalls != 2 || accountReleases != 1 {
		t.Fatalf("ASSERT_MANAGER_DIRECTIONAL_RELEASE_RETRY_CONVERGES_ONCE: err=%v result=%+v closed=%t calls=%d releases=%d", err, result, reservation.Closed(), terminalCalls, accountReleases)
	}
	if reservation.Release() || terminalCalls != 3 || accountReleases != 1 {
		t.Fatalf("ASSERT_MANAGER_DIRECTIONAL_NO_DOUBLE_RELEASE: calls=%d releases=%d", terminalCalls, accountReleases)
	}
}
