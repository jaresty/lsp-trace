package adr0011generic

import "fmt"

// Synthetic role inventory is a private, in-memory replay input. It grants no
// verified terminal, occurrence admission, or production authority.
type syntheticRole struct {
	role string
}

type syntheticRoleSelection struct {
	// The verifier holds these expectations independently of claimant records.
	sourceCount int
	process     bool
}

func validateRoleInventory(records []syntheticRole, held syntheticRoleSelection) error {
	if held.sourceCount < 1 || held.sourceCount > 256 {
		return fmt.Errorf("invalid held SOURCE cardinality %d", held.sourceCount)
	}
	expected := map[string]int{
		"POLICY": 1, "SCHEMA": 1, "SOURCE": held.sourceCount, "QUERY": 1,
		"CAPABILITY_EVENTS": 1, "REQUEST_WRITE": 1, "INBOUND_FRAMES": 1,
		"RESULT_READ": 1, "TARGET_EVENTS": 1, "TERMINAL": 1,
	}
	if held.process {
		expected["PROCESS"] = 1
	}
	if len(records) != 9+held.sourceCount+expected["PROCESS"] {
		return fmt.Errorf("synthetic role count differs from held selection")
	}
	counts := make(map[string]int, len(expected))
	for _, record := range records {
		if _, selected := expected[record.role]; !selected {
			return fmt.Errorf("unselected synthetic role %q", record.role)
		}
		counts[record.role]++
	}
	for role, want := range expected {
		if counts[role] != want {
			return fmt.Errorf("synthetic role %s count %d, held %d", role, counts[role], want)
		}
	}
	return nil
}
