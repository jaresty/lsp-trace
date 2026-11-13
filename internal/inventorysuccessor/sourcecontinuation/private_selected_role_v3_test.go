package sourcecontinuation

import (
	"context"
	inv "lsp-trace/internal/inventorysuccessor"
	"testing"
)

// CONTRACT only: data restrictions cannot manufacture a native operation.
func TestSelectedRoleV3ClosedPairs(t *testing.T) {
	kinds := []string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"}
	roles := []string{"CALLER_OCCURRENCE", "CALLEE_TARGET", "REFERENCING_OCCURRENCE", "REFERENCED_SYMBOL", "QUERY_OCCURRENCE", "DEFINITION_TARGET"}
	count := 0
	for ki, k := range kinds {
		for ri, r := range roles {
			v := PrivateSelectedRoleV3{k, r}
			want := ri/2 == ki
			if v.Valid() != want {
				t.Fatal("ASSERT_CONFIGURATION_V3_CLOSED_PAIR", k, r, want)
			}
			if want {
				count++
			}
		}
	}
	if count != 6 {
		t.Fatal("ASSERT_CONFIGURATION_V3_CLOSED_PAIR_COUNT", count)
	}
	for _, r := range []PrivateSelectedRoleV3{{}, {Kind: "CALLS", Role: "*"}, {Kind: "calls", Role: "CALLEE_TARGET"}, {Kind: "CALLS.*", Role: "CALLEE_TARGET"}, {Kind: "CALLS", Role: "CALLEE_TARGET "}, {Kind: "RESOLVES_TO_DEFINITION", Role: "QUERY_SYMBOL"}} {
		if r.Valid() {
			t.Fatal("ASSERT_CONFIGURATION_V3_CLOSED_PAIR", r)
		}
	}
	t.Log("ASSERT_CONFIGURATION_V3_CLOSED_PAIR: PASS; six literal data-only pairs")
}
func TestSelectedRoleV3ModesAndNoAuthority(t *testing.T) {
	for _, v := range []struct {
		role   PrivateSelectedRoleV3
		family byte
	}{{PrivateSelectedRoleV3{"CALLS", "CALLEE_TARGET"}, 'C'}, {PrivateSelectedRoleV3{"REFERENCES_SYMBOL", "REFERENCED_SYMBOL"}, 'R'}, {PrivateSelectedRoleV3{"RESOLVES_TO_DEFINITION", "DEFINITION_TARGET"}, 'D'}} {
		for _, mode := range []string{"C", "R", "D", "CR", "CD", "RD", "CRD"} {
			want := false
			for i := range mode {
				want = want || mode[i] == v.family
			}
			if v.role.matchesMode(mode) != want {
				t.Fatal("ASSERT_CONFIGURATION_V3_ROLE_MODE", v.role, mode)
			}
		}
		if v.role.matchesMode("") {
			t.Fatal("empty mode")
		}
		result, e := PrepareSelectedRoleV3(context.Background(), inv.PrivateSelectedSourceOperationV2{}, v.role)
		if e == nil || result.Available() {
			t.Fatal("ASSERT_CONFIGURATION_V3_NO_AUTHORITY")
		}
	}
	t.Log("ASSERT_CONFIGURATION_V3_ROLE_MODE: PASS; no fake native authority")
}
