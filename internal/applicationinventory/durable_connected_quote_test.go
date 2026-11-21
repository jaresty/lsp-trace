//go:build darwin || linux

package applicationinventory

import (
	"strings"
	"testing"

	dw "lsp-trace/internal/describeworker"
	"lsp-trace/internal/publication"
)

// This is a candidate downstream envelope, NOT a runnable profile or a grant.
// In particular, these scalar-only references must never be passed to Admit or
// used as proof of retained evidence. The eventual initial-only connected
// profile must enforce every bound before relying on this quotation.
func connectedCandidateQuote(t *testing.T, evidenceBytes, payloadBytes, stateBytes int, manifestBound ...uint64) (uint64, uint64) {
	t.Helper()
	refs := []publication.StoredObject{
		{ID: strings.Repeat("0", 64), Bytes: evidenceBytes},
		{ID: strings.Repeat("1", 64), Bytes: evidenceBytes},
	}
	limits := dw.SuppliedResponseLimits{MaxAdapterBytes: 16 << 10, MaxSemanticBytes: 8 << 10}
	manifest := uint64(16 << 10)
	if len(manifestBound) == 1 {
		manifest = manifestBound[0]
	}
	p, err := quoteDurableCompletion(refs, stateBytes, manifest, limits, []uint64{uint64(payloadBytes), uint64(payloadBytes)})
	if err != nil {
		t.Fatalf("candidate completion quote: %v", err)
	}
	if len(p.Stages) != 5 {
		t.Fatalf("two descriptions, grouping, review/publication, result: got %d stages", len(p.Stages))
	}
	total, ok := capacitySum(p.Stages...)
	if !ok {
		t.Fatal("completion does not fit historical semantic enclosure")
	}
	packets := uint64(len(p.Stages))
	// All four semantic task inputs get one constructed chunk index and one
	// complete paid aligned read pass. Published output is verified by Result,
	// already quoted as the fifth stage; it has no required task-input delivery.
	for i, n := range []int{evidenceBytes, evidenceBytes, p.BaseUpper, p.ProposedUpper} {
		build, err := chunkBuildQuote(n)
		if err != nil {
			t.Fatal(err)
		}
		delivery, count, err := chunkDeliveryQuote(n, int(chunkIndexUpper(n)), 0)
		if err != nil {
			t.Fatal(err)
		}
		var valid bool
		total, valid = capacitySum(total, build, delivery)
		if !valid {
			t.Fatal("whole semantic delivery/completion envelope exceeds historical limit")
		}
		packets += count
		t.Logf("CANDIDATE_DELIVERY task=%d input_upper=%d build=%d read=%d packets=%d", i, n, build, delivery, count)
	}
	t.Logf("CANDIDATE_COMPLETION stages=%v state_upper=%d base_upper=%d proposed_upper=%d semantic_bytes=%d semantic_packets=%d", p.Stages, p.StateUpper, p.BaseUpper, p.ProposedUpper, total, packets)
	return total, packets
}

func TestInventoryConnectedCandidateQuote(t *testing.T) {
	bytes, packets := connectedCandidateQuote(t, 32<<10, 64<<10, 32<<10)
	if bytes == 0 || bytes > durableRunLimit || packets > 64 {
		t.Fatal("candidate semantic envelope is not finite")
	}
	// This deliberately reports only downstream semantic machinery. It does NOT
	// certify funding for admission verification, Seal or its cleanup, nor fit of
	// acquisition and semantic packets together. Those are integration obligations.
	t.Logf("CANDIDATE_ONLY bytes=%d packets=%d; acquisition/Seal verification and combined packet admission unestablished", bytes, packets)
}

func TestInventoryConnectedMaximumAdmissionIsNotSemanticCapacity(t *testing.T) {
	refs := []publication.StoredObject{
		{ID: strings.Repeat("0", 64), Bytes: 8 << 20},
		{ID: strings.Repeat("1", 64), Bytes: 8 << 20},
	}
	_, err := quoteDurableCompletion(refs, 32<<10, 16<<10,
		dw.SuppliedResponseLimits{MaxAdapterBytes: 64 << 10, MaxSemanticBytes: 32 << 10},
		[]uint64{128 << 10, 128 << 10})
	if err == nil {
		t.Fatal("acquisition object ceiling silently treated as downstream semantic capacity")
	}
}
