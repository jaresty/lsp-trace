//go:build darwin || linux

package sourceoperator

import (
	"context"
	inv "lsp-trace/internal/inventorysuccessor"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	pa "lsp-trace/internal/programcadmission"
	"testing"
)

// CONTRACT only. These metadata checks are not source admission or eligibility.
func TestSelectedRoleV3CallerInput(t *testing.T) {
	for bits := 1; bits < 8; bits++ {
		kinds := []pa.RelationKind{}
		if bits&1 != 0 {
			kinds = append(kinds, pa.TypedCalls)
		}
		if bits&2 != 0 {
			kinds = append(kinds, pa.TypedReferencesSymbol)
		}
		if bits&4 != 0 {
			kinds = append(kinds, pa.TypedResolvesToDefinition)
		}
		mode := [8]string{"", "C", "R", "CR", "D", "CD", "RD", "CRD"}[bits]
		for _, v := range []struct {
			role sc.PrivateSelectedRoleV3
			bit  int
		}{{sc.PrivateSelectedRoleV3{Kind: "CALLS", Role: "CALLEE_TARGET"}, 1}, {sc.PrivateSelectedRoleV3{Kind: "REFERENCES_SYMBOL", Role: "REFERENCING_OCCURRENCE"}, 2}, {sc.PrivateSelectedRoleV3{Kind: "RESOLVES_TO_DEFINITION", Role: "DEFINITION_TARGET"}, 4}} {
			q := inv.PrivateMixedInputRequestV2{Kinds: kinds}
			c := inv.PrivateSelectedConfigurationV2{Mode: mode}
			if privateSelectedRoleInputV3(q, c, v.role) != (bits&v.bit != 0) {
				t.Fatal("ASSERT_CONFIGURATION_V3_CALLER_ROLE_INPUT", mode, v.role)
			}
			for _, foreign := range []string{"X", "CCR", "RCD", "C", "R", "D", "CR", "CD", "RD", "CRD"} {
				if foreign == mode {
					continue
				}
				c.Mode = foreign
				if privateSelectedRoleInputV3(q, c, v.role) {
					t.Fatal("ASSERT_CONFIGURATION_V3_CHANGED_CONFIGURATION", mode, foreign)
				}
			}
		}
	}
	role := sc.PrivateSelectedRoleV3{Kind: "CALLS", Role: "CALLEE_TARGET"}
	for _, kinds := range [][]pa.RelationKind{nil, {pa.TypedCalls, pa.TypedCalls}, {pa.RelationKind("FAKE")}, {pa.TypedCalls, pa.TypedReferencesSymbol, pa.TypedResolvesToDefinition, pa.TypedCalls}} {
		if privateSelectedRoleInputV3(inv.PrivateMixedInputRequestV2{Kinds: kinds}, inv.PrivateSelectedConfigurationV2{}, role) {
			t.Fatal("ASSERT_CONFIGURATION_V3_CLOSED_INPUT_KINDS")
		}
	}
	if privateSelectedRoleInputV3(inv.PrivateMixedInputRequestV2{Kinds: []pa.RelationKind{pa.TypedCalls}}, inv.PrivateSelectedConfigurationV2{Mode: "C"}, sc.PrivateSelectedRoleV3{Kind: "CALLS", Role: "DEFINITION_TARGET"}) {
		t.Fatal("ASSERT_CONFIGURATION_V3_CLOSED_PAIR")
	}
	result, e := PrepareSelectedRoleV3(context.Background(), nil, inv.PrivateMixedInputRequestV2{}, inv.PrivateSelectedConfigurationV2{}, role)
	if e == nil || result.Available() {
		t.Fatal("ASSERT_CONFIGURATION_V3_NO_AUTHORITY")
	}
	t.Log("ASSERT_CONFIGURATION_V3_CALLER_ROLE_INPUT: PASS; bounded caller metadata, not admission")
}
