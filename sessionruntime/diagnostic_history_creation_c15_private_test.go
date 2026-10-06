package sessionruntime

import (
	"testing"

	"lsp-trace/internal/session"
)

func TestPrivateDiagnosticHistorySlotEqualityPlusOneAndReuse(t *testing.T) {
	h := new(privateDiagnosticHistory)
	refs := make([]privateDiagnosticHistoryRef, managerPrivateDiagnosticHistorySlots)
	for i := range refs {
		ref, failure := h.reserveSlot(uint64(i + 1))
		if failure != "" {
			t.Fatalf("ASSERT_C15_PRIVATE_HISTORY_EQUALITY_%d failure=%q", i, failure)
		}
		refs[i] = ref
		if !ref.qualified || ref.generation == 0 || int(ref.slot) != i {
			t.Fatalf("ASSERT_C15_PRIVATE_HISTORY_REF_%d ref=%+v", i, ref)
		}
	}
	before := *h
	if _, failure := h.reserveSlot(managerPrivateDiagnosticHistorySlots + 1); failure != session.ResourceExhausted || *h != before {
		t.Fatalf("ASSERT_C15_PRIVATE_HISTORY_PLUS_ONE failure=%q", failure)
	}
	if !h.releaseSlot(refs[0]) || h.resolve(refs[0], 1) != nil {
		t.Fatal("ASSERT_C15_PRIVATE_HISTORY_STALE_AFTER_RELEASE")
	}
	reused, failure := h.reserveSlot(managerPrivateDiagnosticHistorySlots + 1)
	if failure != "" || reused.slot != refs[0].slot || reused.generation == refs[0].generation || h.resolve(refs[0], 1) != nil || h.resolve(reused, managerPrivateDiagnosticHistorySlots+1) == nil {
		t.Fatalf("ASSERT_C15_PRIVATE_HISTORY_REUSE failure=%q old=%+v new=%+v", failure, refs[0], reused)
	}
}

func TestPrivateDiagnosticHistoryRefusalAtGenerationExhaustion(t *testing.T) {
	h := new(privateDiagnosticHistory)
	h.nextGeneration = ^uint64(0)
	before := *h
	if _, failure := h.reserveSlot(1); failure != session.ResourceExhausted || *h != before {
		t.Fatalf("ASSERT_C15_PRIVATE_HISTORY_GENERATION_REFUSAL failure=%q", failure)
	}
}
