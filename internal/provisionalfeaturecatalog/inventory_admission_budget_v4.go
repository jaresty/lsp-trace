package provisionalfeaturecatalog

import "time"

// Initial-only inventory admission profile, never an enlargement of V3 jobs.
// Control: four head transactions at the reader's 64KiB rejection bound
// (32 passes; accepted non-bootstrap heads remain <=16KiB), four 32KiB
// config inspections (8 passes), two read/decode passes of the <=4MiB
// enumeration at final status, and 32KiB scalar/descriptor cleanup. Failure
// and completion use this prepaid allowance; no second charge or refund.
const InventorySelectedControlBytesV1 uint64 = 4*32*65536 + 4*8*32768 + 2*(4<<20) + 32768
const InventoryAdmissionBytesV4 uint64 = 8 << 20

// The first term is the existing finite creation/enumeration/discovery envelope.
// Neither native logical bytes nor any physical storage are scalar H.
const InventoryAdmissionLimitV4 uint64 = 64<<20 + InventorySelectedControlBytesV1 + InventoryAdmissionBytesV4

// NewInventoryAdmissionBudgetV4 consumes only the fixed admission slice, with
// the full cumulative predecessor spend. It confers no native authority.
func NewInventoryAdmissionBudgetV4(deadline time.Time, allowance, previouslySpent uint64) (*CallerBudgetV3, error) {
	if allowance != InventoryAdmissionBytesV4 || previouslySpent > InventoryAdmissionLimitV4 || allowance > InventoryAdmissionLimitV4-previouslySpent || !time.Now().Before(deadline) || time.Until(deadline) > 15*time.Second {
		return nil, ErrCallerBudgetV3
	}
	return &CallerBudgetV3{deadline: deadline, limit: previouslySpent + allowance, spent: previouslySpent}, nil
}
