package provisionalfeaturecatalog

import "time"

// V9 only. Neither native vectors nor the v8 enclosure is changed. Whole
// evidence/payload/state ceilings remain v8's and INCLUDE the witness. The
// manifest accounting operand grows explicitly below; it is not physical quota.
const InventoryReferenceWitnessBytesV1 = 8 << 10
const InventoryReferenceOriginalBytesV1 = 64 << 10

// Paid before borrowing the original: snapshot/hash passes, witness encodings,
// parsing and refusal cleanup. Not physical memory/IO or a provider allowance.
const InventoryReferenceRetentionV1 uint64 = 8*InventoryReferenceOriginalBytesV1 + 32*InventoryReferenceWitnessBytesV1 + (256 << 10)
const InventoryReferenceAdmissionV1 = InventoryAdmissionBytesV4 + InventoryReferenceRetentionV1
const InventoryReferenceAcquisitionV1 = InventoryResumeLimitV1 + 2*InventoryReferenceRetentionV1
const InventoryReferenceSealExtraV1 uint64 = 3*32*InventoryReferenceWitnessBytesV1 + 16*18*2048
const InventoryReferenceSealV1 = InventoryConnectedSealBytesV1 + InventoryReferenceSealExtraV1

// Each Describe increment also pays the actual <=16KiB immutable admission read
// and validation (32*16KiB+32KiB). Two description reopens occur each at base
// construction and review. The existing payload/whole-object formulas pay all
// catalog sealing, delivery, copies and publication of these included bytes.
const InventoryReferenceDescribeV1 uint64 = 2 << 20
const InventoryReferenceReopenV1 uint64 = 2 * 32 * InventoryReferenceWitnessBytesV1

// Two witness headers plus at most sixteen target provenance records, each
// <=2048 bytes, pay the unchanged manifest 128-per-item +12-per-byte formula.
const InventoryReferenceManifestV1 = InventoryConnectedManifestV1 + 18*(128+12*2048)
const InventoryReferenceManifestExtraV1 = InventoryReferenceManifestV1 - InventoryConnectedManifestV1
const InventoryReferenceSemanticExtraV1 = 2*InventoryReferenceDescribeV1 + 2*InventoryReferenceReopenV1 + InventoryReferenceManifestExtraV1
const InventoryReferenceLimitV1 = InventoryReferenceAcquisitionV1 + InventoryReferenceSealV1 + InventoryConnectedDeliveryBytesV1 + InventoryReferenceSemanticExtraV1

func NewInventoryReferenceBudgetV1(deadline time.Time, allowance, prior uint64, semantic time.Duration) (*CallerBudgetV3, error) {
	if allowance == 0 || prior > InventoryReferenceLimitV1 || allowance > InventoryReferenceLimitV1-prior || semantic < 0 || semantic > 120*time.Second || !time.Now().Before(deadline) || time.Until(deadline) > 30*time.Second {
		return nil, ErrCallerBudgetV3
	}
	return &CallerBudgetV3{deadline: deadline, limit: prior + allowance, spent: prior, semantic: semantic}, nil
}
