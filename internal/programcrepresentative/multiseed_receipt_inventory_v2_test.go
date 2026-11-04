package programcrepresentative

import "testing"

func TestMultiSeedV2ReceiptOrdinalsRemainPublicationScoped(t *testing.T) {
	t.Log("procedure: two exact plan positions/distinct receipts may share positive source receipt ordinal; duplicate plan positions/selectors and wrong counts refuse; frozen v1 remains unchanged")
	fixture := multiSeedSubjectExpectedFixtureV2(t)
	receipts := append([]ReceiptInventoryEntryDTO{}, fixture.ReceiptInventory...)
	second := receipts[0]
	second.PlanOrdinal = 1
	second.ReceiptSelector = rawDigest([]byte("different canonical receipt"))
	second.ReceiptSHA256 = second.ReceiptSelector
	receipts = append(receipts, second)
	head := fixture.TerminalProgramCHead
	head.CommittedPrefix = 2
	head.ConstituentCount = 2
	if err := validateReceiptAndHeadV2(receipts, head); err != nil {
		t.Fatalf("PLAN_ORDINAL_UNIQUE_RECEIPT_ORDINAL_POSITIVE: %v", err)
	}
	if err := validateReceiptAndHead(receipts, head); err == nil {
		t.Fatal("FROZEN_V1_RECEIPT_ORDINAL_REFUSAL_UNCHANGED")
	}
	for _, name := range []string{"plan-ordinal", "selector", "nonpositive", "head-count"} {
		t.Run(name, func(t *testing.T) {
			r := append([]ReceiptInventoryEntryDTO{}, receipts...)
			h := head
			switch name {
			case "plan-ordinal":
				r[1].PlanOrdinal = 0
			case "selector":
				r[1].ReceiptSelector = r[0].ReceiptSelector
				r[1].ReceiptSHA256 = r[0].ReceiptSHA256
			case "nonpositive":
				r[1].ReceiptOrdinal = 0
			case "head-count":
				h.CommittedPrefix = 1
			}
			if err := validateReceiptAndHeadV2(r, h); err == nil {
				t.Fatal("INVALID_V2_RECEIPT_INVENTORY_REFUSAL")
			}
		})
	}
}
