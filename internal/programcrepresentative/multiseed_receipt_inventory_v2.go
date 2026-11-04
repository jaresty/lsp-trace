package programcrepresentative

import "fmt"

// The v2 subject preserves ADR0014 914-923: positive source receipt ordinals,
// exact plan-ordinal sequence, distinct immutable receipt selectors, and exact
// terminal count. Frozen v1's additional ordinal uniqueness remains frozen.
func validateReceiptAndHeadV2(receipts []ReceiptInventoryEntryDTO, head TerminalProgramCHeadDTO) error {
	if receipts == nil || len(receipts) == 0 || head.SchemaVersion != programCHeadSchemaVersion || head.WorkflowKey == "" || head.HeadKey == "" || head.Generation == 0 || !validDigest(head.Selector) || !validDigest(head.HeadBytesSHA256) || head.ByteLength == 0 || head.CommittedPrefix < 0 || head.ConstituentCount < 0 || len(receipts) != head.CommittedPrefix || len(receipts) != head.ConstituentCount {
		return fmt.Errorf("invalid v2 receipt inventory terminal head")
	}
	seen := map[string]bool{}
	for i, r := range receipts {
		if r.PlanOrdinal != i || r.ReceiptOrdinal <= 0 || r.ReceiptSchemaVersion != programCReceiptSchemaV3 && r.ReceiptSchemaVersion != programCReceiptSchemaV4 || !validDigest(r.ReceiptSelector) || r.ReceiptSelector != r.ReceiptSHA256 || r.ReceiptByteLength == 0 || seen[r.ReceiptSelector] {
			return fmt.Errorf("invalid v2 receipt inventory entry/order/selector")
		}
		seen[r.ReceiptSelector] = true
	}
	return nil
}
