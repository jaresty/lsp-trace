//go:build darwin || linux

package applicationinventory

import (
	dw "lsp-trace/internal/describeworker"
	is "lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/publication"
)

// The durable publisher emits exactly four fixed-name artifacts. Its closed
// manifest (including maximal counters, lengths and all four digests) fits2048;
// this is not a limit on semantic replies or arbitrary PublishV3 callers.
const durableManifestUpper = 2048

// Immutable, acceptance-bound completion plan. Each entry is an outer slice;
// inner reservations spend that slice, never the outer ledger a second time.
// Cursor movement and Used/Work/Packets movement occur in the same head CAS.
type durableCompletionPlan struct {
	Version         string
	Stages          []uint64
	StateUpper      uint64
	PreparationUsed uint64
	// Staged admission inputs: only unspent future bounds may tighten.
	Inputs                   []int
	Payloads                 []uint64
	Adapters                 []int
	Manifest                 uint64
	Limits                   dw.SuppliedResponseLimits
	InitialStateBytes        int
	BaseUpper, ProposedUpper int
}

func capacitySum(xs ...uint64) (uint64, bool) {
	var n uint64
	for _, x := range xs {
		if x > durableRunLimit-n {
			return 0, false
		}
		n += x
	}
	return n, true
}

// quoteDurableCompletion uses only already-owned scalar lengths. The initial
// state contains the whole census, including unprepared dispositions. Additional
// state consists of <=P description references, three catalog references, fixed
// report counters/statuses and bounded disposition text. Per lead, a new
// description reference (<128 bytes) plus closed disposition changes (<128)
// fit256; 4096 covers three catalog refs, fixed counters/stages, publication
// accounting strings and catalog digest. Existing census/policy bytes persist.
func quoteDurableCompletion(refs []publication.StoredObject, stateBytes int, manifest uint64, limits dw.SuppliedResponseLimits, sealPayloads []uint64) (durableCompletionPlan, error) {
	return quoteDurableCompletionBounds(refs, stateBytes, manifest, limits, sealPayloads, nil)
}

func quoteDurableCompletionBounds(refs []publication.StoredObject, stateBytes int, manifest uint64, limits dw.SuppliedResponseLimits, sealPayloads []uint64, adapters []int) (durableCompletionPlan, error) {
	bad := func() (durableCompletionPlan, error) { return durableCompletionPlan{}, ErrDurableInventory }
	if adapters != nil && len(adapters) != len(refs) {
		return bad()
	}
	if len(sealPayloads) != len(refs) || len(refs) > 10000 || stateBytes <= 0 || stateBytes > 8<<20 || manifest > durableRunLimit || !limits.Valid() {
		return bad()
	}
	state := uint64(stateBytes) + uint64(len(refs))*256 + 4096
	if state > 8<<20 {
		return bad()
	}
	p := uint64(0)
	sum := uint64(0)
	payloadSum := uint64(0)
	ds := make([]uint64, 0, len(refs))
	ins := make([]uint64, 0, len(refs))
	for i, r := range refs {
		if !r.Valid() {
			if r != (publication.StoredObject{}) || sealPayloads[i] != 0 {
				return bad()
			}
			continue
		}
		if r.Bytes > 8<<20 {
			return bad()
		}
		// Evidence retains the original unit/request/selector. New response JSON is
		// bounded by the canonical outer adapter plus HOST (already in input) and
		// 2048 bytes of closed response/evidence metadata, including all four
		// legal citation handles in each of five categories. Input's accounting
		// and retention-only fields are extra slack, not removed from storage.
		// HOST appears once in retained input and once in the response. The input
		// also already contains selector, unit and request; none is duplicated.
		adapter := limits.MaxAdapterBytes
		if adapters != nil {
			adapter = adapters[i]
		}
		if adapter < 1 || adapter > limits.MaxAdapterBytes {
			return bad()
		}
		d := uint64(r.Bytes) + uint64(adapter) + 2048
		if d > 8<<20 {
			return bad()
		}
		var ok bool
		sum, ok = capacitySum(sum, d)
		if !ok {
			return bad()
		}
		if sealPayloads[i] == 0 || sealPayloads[i] > 8<<20 {
			return bad()
		}
		reduction := uint64(limits.MaxAdapterBytes - adapter)
		if sealPayloads[i] <= reduction {
			return bad()
		}
		payloadSum, ok = capacitySum(payloadSum, sealPayloads[i]-reduction)
		if !ok {
			return bad()
		}
		ds = append(ds, d)
		ins = append(ins, uint64(r.Bytes))
		p++
	}
	if p > 100 {
		return bad()
	}
	// Closed catalog keys/statuses/digests plus maximum escaped256-byte actor
	// fit4096; all evidence JSON is counted separately (no identity copies).
	base := sum + 4096
	// Groups are nonempty and membership is unique, hence group count <=P.
	// All variable decision strings have a six-byte JSON escape upper. Decision
	// raw input remains capped at64KiB; do not reduce parsing reservations.
	groupStrings := min(uint64(65536), p*8448+4352+p*256)
	// Per unique member/group: quoted71-byte member ID plus closed group
	// keys/punctuation fit256. Additional predecessor and collection punctuation
	// fit1024; variable strings retain their full sixfold escape bound.
	grown := base + 6*groupStrings + p*256 + 1024
	if grown > 8<<20 {
		return bad()
	}
	// Match sealNativeV3's operand, not the entire enclosing evidence JSON.
	seal := 24*payloadSum + p*32768 + 16384
	// JSON decoding can replace each invalid UTF-8 byte by a three-byte rune.
	// Sealing charges decoded strings, unlike canonical encoding's6x raw bound
	// above. Keep12x and field limits; do not narrow accepted decision grammar.
	groupSeal := seal + 12*min(uint64(3*65536), p*8448) + p*1024
	// Two bounded census passes and an N-entry map of existing <=256-byte IDs:
	// duplicate hashing/comparison, status/reference checks and fixed run-ID work.
	// Ordinary reopen does not read or compare historical preparations.
	ownerCheck := uint64(len(refs))*1024 + 4096
	reopenDescriptions := 6*sum + p*16384 + ownerCheck
	evidenceCopies := 2*sum + p*4096
	stateIO := 16*state + 65536 + durablePlanningQuote(len(refs)) // state IO plus charged staged quote construction
	baseBuild := reopenDescriptions + p*4096 + 4096 + evidenceCopies + seal + base + 1
	plan := durableCompletionPlan{Version: durableQuoteVersion, StateUpper: state, Stages: []uint64{}, Inputs: make([]int, len(refs)), Payloads: append([]uint64(nil), sealPayloads...), Adapters: make([]int, len(refs)), Manifest: manifest, Limits: limits, InitialStateBytes: stateBytes, BaseUpper: int(base), ProposedUpper: int(grown)}
	for i, r := range refs {
		plan.Inputs[i] = r.Bytes
		plan.Adapters[i] = limits.MaxAdapterBytes
		if adapters != nil {
			plan.Adapters[i] = adapters[i]
		}
	}
	for i, in := range ins {
		q := stateIO + 6*in + 16384 + uint64(limits.MaxAdapterBytes)*12 + uint64(limits.MaxSemanticBytes)*24 + 32768 + ds[i] + 1
		if i == len(ins)-1 {
			q += baseBuild
		}
		if q > durableRunLimit {
			return bad()
		}
		plan.Stages = append(plan.Stages, q)
	}
	group := stateIO + 8*base + 16384 + 16*65536 + 16384 + groupSeal + grown + 1
	// Review: reopen proposal, bounded review/decision decoding, reseal and copy;
	// reopen descriptions and reconcile; publish four artifacts with full readback;
	// retain/read actual publication manifest; save the next state.
	report := state
	outputs := base + 2*grown + report
	review := stateIO + 8*grown + 16384 + 24*65536 + 20480 + groupSeal + grown + 1 +
		reopenDescriptions + uint64(len(refs))*192 + p*1024 + 8192 +
		9*grown + 4097 + evidenceCopies + manifest + 2*grown + 32768 +
		4*(outputs+16384) + 16384 + 8*durableManifestUpper
	result := 8*outputs + 8*durableManifestUpper
	plan.Stages = append(plan.Stages, group, review, result)
	if _, ok := capacitySum(plan.Stages...); !ok {
		return bad()
	}
	return plan, nil
}

func (h durableInventoryHead) protectedCapacity() (uint64, uint64, bool) {
	p := h.Completion
	if h.SuccessorReserve > durableRunLimit || h.SuccessorPackets > 64 {
		return 0, 0, false
	}
	if p.Version != durableQuoteVersion || len(p.Stages) < 3 || len(p.Stages) > 103 || p.StateUpper == 0 || p.StateUpper > 8<<20 || h.CompletionCursor < 0 || h.CompletionCursor > len(p.Stages) {
		return 0, 0, false
	}
	for _, q := range p.Stages {
		if q == 0 || q > durableRunLimit {
			return 0, 0, false
		}
	}
	n, ok := capacitySum(p.Stages[h.CompletionCursor:]...)
	delivery, packets, valid := h.deliveryRemaining()
	n, summed := capacitySum(n, delivery, h.SuccessorReserve)
	return n, uint64(len(p.Stages)-h.CompletionCursor) + packets + h.SuccessorPackets, ok && valid && summed
}

func (h durableInventoryHead) completionValid() bool {
	if ownedPhase(h) {
		return ownedHeadValid(h)
	}
	n, packets, ok := h.protectedCapacity()
	if !ok {
		return false
	}
	spent, valid := capacitySum(h.Completion.Stages[:h.CompletionCursor]...)
	floor, validFloor := capacitySum(h.Completion.PreparationUsed, spent, h.DeliverySpent)
	if is.InventoryConnectedHead(h.Schema) {
		validFloor = h.Completion.PreparationUsed <= h.runLimit() && spent <= h.runLimit()-h.Completion.PreparationUsed && h.DeliverySpent <= h.runLimit()-h.Completion.PreparationUsed-spent
		if validFloor {
			floor = h.Completion.PreparationUsed + spent + h.DeliverySpent
		}
		if !connectedSemanticValid(h) {
			return false
		}
	}
	if !valid || !validFloor || h.Used < floor || h.Work != h.Used*32 || h.Packets < uint64(h.CompletionCursor) {
		return false
	}
	if !ok || h.Used > h.Limit || n > h.Limit-h.Used || h.Work > h.workLimit() || n > (h.workLimit()-h.Work)/32 || h.Packets > 64 || packets > 64-h.Packets || uint64(h.State.Bytes) > h.Completion.StateUpper {
		return false
	}
	expected := h.Sequence - 1
	if h.Status == "CLAIMED" {
		expected++
	}
	if h.Phase == "Published" {
		return h.Sequence == len(h.Completion.Stages) && (h.CompletionCursor == expected || h.CompletionCursor == expected+1)
	}
	if h.CompletionCursor != expected || h.Sequence >= len(h.Completion.Stages) {
		return false
	}
	p := len(h.Completion.Stages) - 3
	switch h.Phase {
	case "Describe":
		return h.Sequence <= p
	case "GroupDecision":
		return h.Sequence == p+1
	case "Reviewer":
		return h.Sequence == p+2
	}
	return false
}
