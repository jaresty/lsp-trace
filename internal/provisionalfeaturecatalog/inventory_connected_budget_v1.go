package provisionalfeaturecatalog

import "time"

// Closed initial-only connected profile. These are logical traversal units,
// neither native coordinates nor physical storage. Existing profiles are intact.
const InventoryConnectedEvidenceV1 = 32 << 10
const InventoryConnectedPayloadV1 = 64 << 10
const InventoryConnectedStateV1 = 32 << 10

// Manifest is a logical accounting operand, including its 32KiB fixed term,
// not serialized manifest bytes. The earlier 16KiB scalar candidate was too small.
const InventoryConnectedManifestV1 = 128 << 10
const InventoryConnectedAdapterV1 = 16 << 10
const InventoryConnectedSemanticV1 = 8 << 10

// Closed specialization of applicationinventory's completion and chunk formulas.
// Two descriptions, one grouping, one review/publication and one Result; four
// input builds and complete aligned reads. Regression compares the actual quote.
const connectedD = InventoryConnectedEvidenceV1 + InventoryConnectedAdapterV1 + 2048
const connectedSum = 2 * connectedD
const connectedState = InventoryConnectedStateV1 + 2*256 + 4096
const connectedBase = connectedSum + 4096
const connectedGrown = connectedBase + 6*(2*8448+4352+2*256) + 2*256 + 1024
const connectedSeal = 24*(2*InventoryConnectedPayloadV1) + 2*32768 + 16384
const connectedGroupSeal = connectedSeal + 12*(2*8448) + 2*1024
const connectedReopen = 6*connectedSum + 2*16384 + 2*1024 + 4096
const connectedCopies = 2*connectedSum + 2*4096
const connectedStateIO = 16*connectedState + 65536 + 2*512 + 32768
const connectedBaseBuild = connectedReopen + 2*4096 + 4096 + connectedCopies + connectedSeal + connectedBase + 1
const connectedDescribe = connectedStateIO + 6*InventoryConnectedEvidenceV1 + 16384 + InventoryConnectedAdapterV1*12 + InventoryConnectedSemanticV1*24 + 32768 + connectedD + 1
const connectedGroup = connectedStateIO + 8*connectedBase + 16384 + 16*65536 + 16384 + connectedGroupSeal + connectedGrown + 1
const connectedOutputs = connectedBase + 2*connectedGrown + connectedState
const connectedReview = connectedStateIO + 8*connectedGrown + 16384 + 24*65536 + 20480 + connectedGroupSeal + connectedGrown + 1 + connectedReopen + 2*192 + 2*1024 + 8192 + 9*connectedGrown + 4097 + connectedCopies + InventoryConnectedManifestV1 + 2*connectedGrown + 32768 + 4*(connectedOutputs+16384) + 16384 + 8*2048
const connectedResult = 8*connectedOutputs + 8*2048
const connectedBaseChunks = (connectedBase + 32767) / 32768
const connectedGrownChunks = (connectedGrown + 32767) / 32768

// build=14n+16index+32768; aligned delivery=8n+k*(8index+16384).
const connectedEvidenceDelivery = 22*InventoryConnectedEvidenceV1 + 16*(1024+128) + 32768 + 8*(1024+128) + 16384
const connectedBaseDelivery = 22*connectedBase + 16*(1024+128*connectedBaseChunks) + 32768 + connectedBaseChunks*(8*(1024+128*connectedBaseChunks)+16384)
const connectedGrownDelivery = 22*connectedGrown + 16*(1024+128*connectedGrownChunks) + 32768 + connectedGrownChunks*(8*(1024+128*connectedGrownChunks)+16384)
const InventoryConnectedDeliveryBytesV1 uint64 = 2*connectedDescribe + connectedBaseBuild + connectedGroup + connectedReview + connectedResult + 2*connectedEvidenceDelivery + connectedBaseDelivery + connectedGrownDelivery

// Seal pays three 32-pass intrinsic admission/evidence verification waves
// (checkpoint plus both chain entries), ten full enumeration traversals (the
// preflight and both independent ordinal inspections), four 32-pass head
// transactions at the 64KiB Sealing rejection bound, six eight-pass
// config inspections, immutable closure retention/readback, the historical 2MiB
// state/manifest/initial-delivery slice, and descriptor/failure cleanup. Native
// resources have already been closed by the two observed admission owners.
const InventoryConnectedSealBytesV1 uint64 = 3*32*(InventoryConnectedEvidenceV1+2*16384) + 10*(4<<20) + 4*32*65536 + 6*8*32768 + 16*16384 + (2 << 20) + 32768
const InventoryConnectedLimitV1 = InventoryResumeLimitV1 + InventoryConnectedSealBytesV1 + InventoryConnectedDeliveryBytesV1

func NewInventoryConnectedBudgetV1(deadline time.Time, allowance, previouslySpent uint64, semantic time.Duration) (*CallerBudgetV3, error) {
	if allowance == 0 || previouslySpent > InventoryConnectedLimitV1 || allowance > InventoryConnectedLimitV1-previouslySpent || semantic < 0 || semantic > 120*time.Second || !time.Now().Before(deadline) || time.Until(deadline) > 30*time.Second {
		return nil, ErrCallerBudgetV3
	}
	return &CallerBudgetV3{deadline: deadline, limit: previouslySpent + allowance, spent: previouslySpent, semantic: semantic}, nil
}
