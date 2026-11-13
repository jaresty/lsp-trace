package sourcecontinuation

import (
	"context"
	inv "lsp-trace/internal/inventorysuccessor"
)

// PrivateSelectedRoleV3 is only a closed selection criterion, never authority.
// Only these six literal native kind/role pairs are admitted.
type PrivateSelectedRoleV3 struct{ Kind, Role string }

func (r PrivateSelectedRoleV3) Valid() bool {
	switch r.Kind {
	case "CALLS":
		return r.Role == "CALLER_OCCURRENCE" || r.Role == "CALLEE_TARGET"
	case "REFERENCES_SYMBOL":
		return r.Role == "REFERENCING_OCCURRENCE" || r.Role == "REFERENCED_SYMBOL"
	case "RESOLVES_TO_DEFINITION":
		return r.Role == "QUERY_OCCURRENCE" || r.Role == "DEFINITION_TARGET"
	}
	return false
}
func (r PrivateSelectedRoleV3) matchesMode(mode string) bool {
	if !r.Valid() {
		return false
	}
	letter := byte('C')
	if r.Kind == "REFERENCES_SYMBOL" {
		letter = 'R'
	}
	if r.Kind == "RESOLVES_TO_DEFINITION" {
		letter = 'D'
	}
	for i := range mode {
		if mode[i] == letter {
			return true
		}
	}
	return false
}

// The role filters only held SOURCE_ADMITTED rows. Two bounded enum comparisons
// replace the more general exact selector, within the existing selection quote.
// Whole Resume, native Matches, seven Put/Get, Finish and cleanup are unchanged.
func PrepareSelectedRoleV3(ctx context.Context, op inv.PrivateSelectedSourceOperationV2, role PrivateSelectedRoleV3) (PrivateSelectedResultV2, error) {
	return prepareSelectedRoleCriterionV3(ctx, op, nil, false, &role)
}
