package sessionruntime

import "testing"

func TestPrivateDiagnosticHistoryWrappedNonHeadDeletionPreservesOrder(t *testing.T) {
	h := new(privateDiagnosticHistory)
	refs := make([]privateDiagnosticHistoryRef, managerPrivateDiagnosticHistorySlots)
	for i := range refs {
		refs[i], _ = h.reserveSlot(uint64(i + 1))
	}
	for i := 0; i < 7; i++ {
		if !h.releaseSlot(refs[i]) {
			t.Fatal("ASSERT_C15_FIFO_HEAD_ADVANCE")
		}
	}
	for i := 0; i < 7; i++ {
		ref, failure := h.reserveSlot(uint64(managerPrivateDiagnosticHistorySlots + i + 1))
		if failure != "" {
			t.Fatal(failure)
		}
		refs[i] = ref
	}
	position := 101
	index := (int(h.head) + position) % managerPrivateDiagnosticHistorySlots
	removedSlot := h.order[index]
	removed := privateDiagnosticHistoryRef{generation: h.slots[removedSlot].generation, slot: removedSlot, qualified: true}
	removedSequence := h.slots[removedSlot].sequence
	before := make([]uint64, 0, h.count)
	for i := 0; i < int(h.count); i++ {
		before = append(before, h.slots[h.order[(int(h.head)+i)%managerPrivateDiagnosticHistorySlots]].sequence)
	}
	if !h.releaseSlot(removed) || h.resolve(removed, removedSequence) != nil {
		t.Fatal("ASSERT_C15_FIFO_NON_HEAD_RELEASE")
	}
	for i := 0; i < int(h.count); i++ {
		got := h.slots[h.order[(int(h.head)+i)%managerPrivateDiagnosticHistorySlots]].sequence
		want := before[i]
		if i >= position {
			want = before[i+1]
		}
		if got != want {
			t.Fatalf("ASSERT_C15_FIFO_NON_HEAD_ORDER i=%d got=%d want=%d", i, got, want)
		}
	}
}

func TestPrivateDiagnosticHistoryFIFOWraparoundAndActiveProtection(t *testing.T) {
	h := new(privateDiagnosticHistory)
	refs := make([]privateDiagnosticHistoryRef, managerPrivateDiagnosticHistorySlots)
	for i := range refs {
		ref, failure := h.reserveSlot(uint64(i + 1))
		if failure != "" {
			t.Fatal(failure)
		}
		refs[i] = ref
	}
	// Only the second operation is terminal: the active oldest operation must not be selected.
	h.resolve(refs[1], 2).closed = true
	if got, ok := h.oldestClosed(privateDiagnosticHistoryRef{}); !ok || got != refs[1] {
		t.Fatalf("ASSERT_C15_FIFO_ACTIVE_PROTECTION got=%+v ok=%t", got, ok)
	}
	// Terminalize all entries, then repeatedly retire the logical head and append at the tail.
	for i := range refs {
		h.resolve(refs[i], uint64(i+1)).closed = true
	}
	for cycle := 0; cycle < managerPrivateDiagnosticHistorySlots+17; cycle++ {
		oldest, ok := h.oldestClosed(privateDiagnosticHistoryRef{})
		if !ok {
			t.Fatal("ASSERT_C15_FIFO_MISSING_OLDEST")
		}
		oldSequence := h.slots[oldest.slot].sequence
		if !h.releaseSlot(oldest) || h.resolve(oldest, oldSequence) != nil {
			t.Fatal("ASSERT_C15_FIFO_STALE_RELEASE")
		}
		sequence := uint64(managerPrivateDiagnosticHistorySlots + cycle + 1)
		replacement, failure := h.reserveSlot(sequence)
		if failure != "" {
			t.Fatal(failure)
		}
		h.resolve(replacement, sequence).closed = true
		if replacement.generation == oldest.generation {
			t.Fatal("ASSERT_C15_FIFO_GENERATION_REUSE")
		}
	}
	if h.count != managerPrivateDiagnosticHistorySlots || h.head == 0 {
		t.Fatalf("ASSERT_C15_FIFO_WRAPAROUND head=%d count=%d", h.head, h.count)
	}
	oldest, ok := h.oldestClosed(privateDiagnosticHistoryRef{})
	if !ok || h.slots[oldest.slot].sequence != uint64(managerPrivateDiagnosticHistorySlots+18) {
		t.Fatalf("ASSERT_C15_FIFO_ORDER oldest=%+v sequence=%d", oldest, h.slots[oldest.slot].sequence)
	}
}
