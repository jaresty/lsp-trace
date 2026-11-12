//go:build darwin || linux

package sourceoperator

import (
	"context"
	inv "lsp-trace/internal/inventorysuccessor"
	"testing"
)

// No provider, persistent native fixture, job/grant or product CLI is invoked.
func TestSelectedV2OperatorRefusesBeforeCapture(t *testing.T) {
	for _, ctx := range []context.Context{nil, context.Background()} {
		r, e := privatePrepareSelectedKindsV2(ctx, privateSelectedRequestV2{})
		if e == nil || r.Available() {
			t.Fatal("zero invocation became available")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e := privatePrepareSelectedKindsV2(ctx, privateSelectedRequestV2{Owner: &inv.PrivateManagedInputOwnerV2{}})
	if e == nil || r.Available() {
		t.Fatal("canceled invocation")
	}
	r, e = privatePrepareSelectedKindsV2(context.Background(), privateSelectedRequestV2{Owner: &inv.PrivateManagedInputOwnerV2{}, Capture: inv.PrivateMixedInputRequestV2{Timing: &inv.PrivateSourceTimingFrameV2{}}})
	if e == nil || r.Available() {
		t.Fatal("timing path admitted")
	}
}
