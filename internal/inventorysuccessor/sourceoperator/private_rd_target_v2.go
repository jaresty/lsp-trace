//go:build darwin || linux

package sourceoperator

import (
	"context"

	inv "lsp-trace/internal/inventorysuccessor"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	pa "lsp-trace/internal/programcadmission"
)

// privateRDTargetInputV2 performs only bounded caller-input admission. Existing
// constructor validation/snapshots and all paid M/P/S guards remain mandatory.
func privateRDTargetInputV2(capture inv.PrivateMixedInputRequestV2, config inv.PrivateSelectedConfigurationV2) bool {
	if capture.Timing != nil || config.Mode != "" && config.Mode != "RD" || len(capture.Kinds) != 2 {
		return false
	}
	a, b := capture.Kinds[0], capture.Kinds[1]
	return a == pa.TypedReferencesSymbol && b == pa.TypedResolvesToDefinition || a == pa.TypedResolvesToDefinition && b == pa.TypedReferencesSymbol
}

// PrepareRDTargetV2 runs the existing fixed M/P/S orchestration with a closed
// unique-D-target selection policy. It requires an already provisioned owner
// and admitted funding; it does not initialize either. The result is the same
// opaque native selected result, without publication or an output surface.
// Selection does NOT establish non-query distinctness or an expected body:
// a self-definition can qualify for selection. A trusted, separately funded
// driver must prebind and compare its expected target before qualification.
func PrepareRDTargetV2(ctx context.Context, owner *inv.PrivateManagedInputOwnerV2, capture inv.PrivateMixedInputRequestV2, config inv.PrivateSelectedConfigurationV2) (sc.PrivateSelectedResultV2, error) {
	return privatePrepareSelectedKindsV2(ctx, privateSelectedRequestV2{Owner: owner, Capture: capture, Configuration: config, DTarget: true})
}
