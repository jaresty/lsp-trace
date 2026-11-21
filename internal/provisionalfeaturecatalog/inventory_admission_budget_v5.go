package provisionalfeaturecatalog

import "time"

// V7 is initial-only. Historical profiles and costs are never reinterpreted.
// Each native control pays six 4MiB enumeration read/decode passes, four
// 32-pass head transactions at the 64KiB rejection bound, four eight-pass
// 32KiB config inspections and scalar/descriptor cleanup.
const InventoryNativeControlBytesV2 uint64 = 6*(4<<20) + 4*32*65536 + 4*8*32768 + 32768

// A checkpoint pays four verification waves (creation, batch lookup, parent
// reserve and independent permit validation), each with 32 byte traversals
// of an <=8MiB evidence object and two <=16KiB metadata objects: four intrinsic
// request identity encodings/copies/hashes, prompt hash, verified object reads,
// evidence/admission decode and bounded cleanup fit this envelope. It also pays
// four bounded head transactions, four config reads,
// two enumeration read/decode passes and descriptor/failure cleanup. These
// are prepaid bounded path allowances, not a rate limit on status requests.
const InventoryCheckpointBytesV1 uint64 = 4*32*((8<<20)+2*16384) + 4*32*65536 + 4*8*32768 + 2*(4<<20) + 32768

// The original 64MiB enclosure contains one historical native-control slice.
// Replace that slice only in this NEW initial profile, add the second native
// slice and two selected/admission/checkpoint slices. Native vectors and
// physical storage are not scalar H; the implied work enclosure is H*32.
const inventoryHistoricalNativeControlV1 uint64 = 6*(4<<20) + 4*32*16384 + 4*8*32768 + 32768
const InventoryResumeLimitV1 uint64 = (64 << 20) - inventoryHistoricalNativeControlV1 + 2*InventoryNativeControlBytesV2 + 2*(InventorySelectedControlBytesV1+InventoryAdmissionBytesV4+InventoryCheckpointBytesV1)

func NewInventoryAdmissionBudgetV5(deadline time.Time, allowance, previouslySpent uint64) (*CallerBudgetV3, error) {
	if allowance != InventoryAdmissionBytesV4 || previouslySpent > InventoryResumeLimitV1 || allowance > InventoryResumeLimitV1-previouslySpent || !time.Now().Before(deadline) || time.Until(deadline) > 15*time.Second {
		return nil, ErrCallerBudgetV3
	}
	return &CallerBudgetV3{deadline: deadline, limit: previouslySpent + allowance, spent: previouslySpent}, nil
}
