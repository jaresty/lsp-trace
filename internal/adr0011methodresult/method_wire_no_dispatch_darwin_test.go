//go:build darwin

package adr0011methodresult

import (
	"context"
	"os"
	"testing"
	"time"

	transport "lsp-trace/internal/adr0011methodtransport"
)

// These are private transport gates; neither outcome is an ADR method receipt.
func TestADR0011ManagedMethodNoDispatch(t *testing.T) {
	for _, tc := range []struct {
		name, method string
		advertised   bool
		stale        bool
		want         transport.Outcome
	}{
		{"D-01-capability-absent", transport.MethodDefinition, false, false, transport.OutcomeUnsupportedCapability},
		{"R-01-capability-absent", transport.MethodReferences, false, false, transport.OutcomeUnsupportedCapability},
		{"D-08-stale-generation", transport.MethodDefinition, true, true, transport.OutcomePreflightFailure},
		{"R-08-stale-generation", transport.MethodReferences, true, true, transport.OutcomePreflightFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manager, started, trace := startManagedMethodPeerWithCapabilities(t, tc.advertised)
			req := fixtureRequest(tc.method, false)
			req.SessionID, req.Generation = started.SessionID, started.Generation
			if tc.stale {
				req.Generation++
			}
			req.Deadline = time.Now().Add(5 * time.Second)
			wire := transport.New(manager).Execute(context.Background(), req)
			parsed, failed := Parse(wire, 3)
			obs, present := wire.Observation()
			if wire.Outcome() != tc.want || !present || obs.RoundTripCalled ||
				obs.LocalWriteCorrespondence != transport.LocalWriteNotObserved ||
				obs.RawResultDisposition != transport.RawResultNotInvoked ||
				len(wire.Raw()) != 0 || failed == nil || failed.Code != FailureTransport || failed.Ordinal != -1 ||
				len(parsed.Items) != 0 || parsed.Null {
				t.Fatalf("ASSERT_ADR0011_MANAGED_NO_DISPATCH: case=%s want=%s outcome=%s present=%v called=%v localWrite=%s rawDisposition=%s parseFailure=%v items=%d", tc.name, tc.want, wire.Outcome(), present, obs.RoundTripCalled, obs.LocalWriteCorrespondence, obs.RawResultDisposition, failed, len(parsed.Items))
			}
			if tc.stale && (obs.MetadataObserved || obs.DeclaredGeneration == started.Generation) ||
				!tc.stale && (!obs.MetadataObserved || obs.ReportedMethodAdvertised) {
				t.Fatalf("ASSERT_ADR0011_MANAGED_NO_DISPATCH_REASON: case=%s metadataObserved=%v advertised=%v generationMatches=%v", tc.name, obs.MetadataObserved, obs.ReportedMethodAdvertised, obs.DeclaredGeneration == started.Generation)
			}
			if _, err := os.Stat(trace); !os.IsNotExist(err) {
				t.Fatalf("ASSERT_ADR0011_MANAGED_PEER_NO_METHOD: case=%s trace unexpectedly present or stat failed: %v", tc.name, err)
			}
		})
	}
}
