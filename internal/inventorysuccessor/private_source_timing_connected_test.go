//go:build (darwin || linux) && lsp_trace_controlled_clock

package inventorysuccessor_test

import (
	clock "lsp-trace/internal/privatesourceclock"
	"testing"
)

func init() { controlledTimingClock = clock.WithControlledClock }

// This uses the existing genuine owner -> SELF capture -> Local=nil P Observe ->
// actual file mailbox -> strict binder/native pair consumer. Only time advances;
// no HOST, provider acquisition, authentication or physical timing fit is claimed.
func TestConnectedHOSTTimingV2(t *testing.T) {
	t.Run("P-reservation-scope", TestHOSTTimingReservationScopeV2)
	for _, name := range []string{"positive", "replacement", "scope-cancel", "legacy", "late", "equal", "capture", "preparation", "tail", "mailbox-tail", "cancel", "forged", "incompatible", "future", "underfund-0", "underfund-1", "underfund-2", "underfund-3", "underfund-4", "underfund-5", "underfund-6", "underfund-7", "underfund-8", "underfund-9", "underfund-10", "underfund-11", "underfund-12", "underfund-13"} {
		t.Run(name, func(t *testing.T) { testSuppliedOp(t, "host-timing-"+name) })
	}
}
