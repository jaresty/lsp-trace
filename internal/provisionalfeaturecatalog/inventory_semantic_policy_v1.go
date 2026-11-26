package provisionalfeaturecatalog

import "time"

// InventorySemanticPolicyV1 reserves response-lease capacity, not measured
// active execution or byte IO. No overall job deadline is implied.
type InventorySemanticPolicyV1 struct {
	Schema                           string
	TaskWindowNS, CumulativeWindowNS int64
	Stages                           int
}

const InventorySemanticPolicySchemaV1 = "lsp-trace.inventory-semantic-policy.v1"

// Implementation configuration-validation bound, NOT a product requirement.
// Four windows fit int64; callers needing more must explicitly revise validation.
const InventorySemanticMaxTaskWindowV1 = 24 * time.Hour

func NewInventorySemanticPolicyV1(window time.Duration) (InventorySemanticPolicyV1, error) {
	if window <= 0 || window > InventorySemanticMaxTaskWindowV1 {
		return InventorySemanticPolicyV1{}, ErrCallerBudgetV3
	}
	return InventorySemanticPolicyV1{InventorySemanticPolicySchemaV1, int64(window), 4 * int64(window), 4}, nil
}
func (p InventorySemanticPolicyV1) Valid() bool {
	return p.Schema == InventorySemanticPolicySchemaV1 && p.Stages == 4 && p.TaskWindowNS > 0 && p.TaskWindowNS <= int64(InventorySemanticMaxTaskWindowV1) && p.CumulativeWindowNS == 4*p.TaskWindowNS
}

// NewInventorySemanticBudgetV1 is data-only spending within an already claimed
// v10 slice. It does not enable callback Exchange or extend native authority.
// The separate active-operation deadline remains <=30s.
func NewInventorySemanticBudgetV1(deadline time.Time, allowance, prior uint64, semantic time.Duration, p InventorySemanticPolicyV1) (*CallerBudgetV3, error) {
	if !p.Valid() || semantic < 0 || int64(semantic) > p.CumulativeWindowNS || int64(semantic)%p.TaskWindowNS != 0 || allowance == 0 || prior > InventoryReferenceLimitV1 || allowance > InventoryReferenceLimitV1-prior || !time.Now().Before(deadline) || time.Until(deadline) > 30*time.Second {
		return nil, ErrCallerBudgetV3
	}
	return &CallerBudgetV3{deadline: deadline, limit: prior + allowance, spent: prior, semantic: semantic, dataOnly: true}, nil
}
