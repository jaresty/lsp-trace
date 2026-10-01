package adr0011generic

import "testing"

func heldSyntheticRoles() []syntheticRole {
	return []syntheticRole{
		{"POLICY"}, {"SCHEMA"}, {"SOURCE"}, {"QUERY"},
		{"CAPABILITY_EVENTS"}, {"REQUEST_WRITE"}, {"INBOUND_FRAMES"},
		{"RESULT_READ"}, {"TARGET_EVENTS"}, {"TERMINAL"},
	}
}

func TestRoleInventoryAcceptsHeldCardinality(t *testing.T) {
	base := heldSyntheticRoles()
	for _, tc := range []struct {
		name    string
		held    syntheticRoleSelection
		records []syntheticRole
	}{
		{"query source only", syntheticRoleSelection{sourceCount: 1}, base},
		{"contextual process selected", syntheticRoleSelection{sourceCount: 1, process: true}, append(append([]syntheticRole(nil), base...), syntheticRole{"PROCESS"})},
		{"second held source", syntheticRoleSelection{sourceCount: 2}, append(append([]syntheticRole(nil), base...), syntheticRole{"SOURCE"})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateRoleInventory(tc.records, tc.held); err != nil {
				t.Fatalf("B1 valid held inventory rejected: %v", err)
			}
		})
	}
}

func TestRoleInventoryRejectsUnselectedRole(t *testing.T) {
	for _, role := range []string{"READBACK", "OTHER", ""} {
		records := append(heldSyntheticRoles(), syntheticRole{role})
		if err := validateRoleInventory(records, syntheticRoleSelection{sourceCount: 1}); err == nil {
			t.Fatalf("B1 unselected role %q accepted", role)
		}
	}
}

func TestRoleInventoryRejectsMissingAndDuplicate(t *testing.T) {
	base := heldSyntheticRoles()
	for _, tc := range []struct {
		name    string
		records []syntheticRole
	}{
		{"missing query", append([]syntheticRole(nil), base[:3]...)},
		{"duplicate policy", append(append([]syntheticRole(nil), base...), syntheticRole{"POLICY"})},
		{"extra source", append(append([]syntheticRole(nil), base...), syntheticRole{"SOURCE"})},
		{"missing source", append(append([]syntheticRole(nil), base[:2]...), base[3:]...)},
		{"unselected process", append(append([]syntheticRole(nil), base...), syntheticRole{"PROCESS"})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateRoleInventory(tc.records, syntheticRoleSelection{sourceCount: 1}); err == nil {
				t.Fatal("B1 missing or duplicate held role accepted")
			}
		})
	}
	if err := validateRoleInventory(base, syntheticRoleSelection{sourceCount: 1, process: true}); err == nil {
		t.Fatal("B1 selected PROCESS omission accepted")
	}
	for _, count := range []int{0, -1, 257} {
		if err := validateRoleInventory(base, syntheticRoleSelection{sourceCount: count}); err == nil {
			t.Fatalf("B1 invalid held source count %d accepted", count)
		}
	}
}
