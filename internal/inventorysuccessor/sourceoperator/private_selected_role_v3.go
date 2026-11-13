//go:build darwin || linux

package sourceoperator

import (
	"context"
	inv "lsp-trace/internal/inventorysuccessor"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	pa "lsp-trace/internal/programcadmission"
)

// Bounded caller-input check only. The opaque plan remains authoritative for
// actual kinds/mode, funding, snapshots and native operation sequencing.
func privateSelectedRoleInputV3(capture inv.PrivateMixedInputRequestV2, config inv.PrivateSelectedConfigurationV2, role sc.PrivateSelectedRoleV3) bool {
	if !role.Valid() || capture.Timing != nil || len(capture.Kinds) == 0 || len(capture.Kinds) > 3 {
		return false
	}
	var bits uint8
	found := false
	for _, k := range capture.Kinds {
		var bit uint8
		switch k {
		case pa.TypedCalls:
			bit = 1
		case pa.TypedReferencesSymbol:
			bit = 2
		case pa.TypedResolvesToDefinition:
			bit = 4
		default:
			return false
		}
		if bits&bit != 0 {
			return false
		}
		bits |= bit
		found = found || string(k) == role.Kind
	}
	actual := [8]string{"", "C", "R", "CR", "D", "CD", "RD", "CRD"}[bits]
	return found && (config.Mode == "" || config.Mode == actual)
}

// PrepareSelectedRoleV3 is explicit caller-owned convenience over the same fixed
// M/P/S chain, not funding, authority or a new acquiring CLI/default.
func PrepareSelectedRoleV3(ctx context.Context, owner *inv.PrivateManagedInputOwnerV2, capture inv.PrivateMixedInputRequestV2, config inv.PrivateSelectedConfigurationV2, role sc.PrivateSelectedRoleV3) (sc.PrivateSelectedResultV2, error) {
	return privatePrepareSelectedKindsV2(ctx, privateSelectedRequestV2{Owner: owner, Capture: capture, Configuration: config, Role: &role})
}
