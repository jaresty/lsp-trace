//go:build darwin || linux

package applicationinventory

import (
	"context"
	"strings"

	"lsp-trace/internal/publication"
)

func durablePlanningQuote(n int) uint64 { return uint64(n)*512 + 32768 }

func successorInputUpper(h durableInventoryHead) int {
	switch h.Phase {
	case "Describe":
		ordinal := 0
		for _, n := range h.Completion.Inputs {
			if n > 0 {
				ordinal++
				if ordinal == h.Sequence+1 {
					return n
				}
			}
		}
		return h.Completion.BaseUpper
	case "GroupDecision":
		return h.Completion.ProposedUpper
	}
	return 0
}
func successorEscrow(h durableInventoryHead) (uint64, uint64, error) {
	n := successorInputUpper(h)
	if n == 0 {
		return 0, 0, nil
	}
	build, e := chunkBuildQuote(n)
	if e != nil {
		return 0, 0, e
	}
	delivery, packets, e := chunkDeliveryQuote(n, int(chunkIndexUpper(n)), 0)
	total, ok := capacitySum(build, delivery)
	if e != nil || !ok {
		return 0, 0, ErrDurableInventory
	}
	return total, packets, nil
}

// submitPlan is length-only preflight, funded by the prior claim. It neither parses a judgment nor opens an
// evidence object. The canonical adapter's actual byte length bounds its future
// record, including escaping, exactly as the original adapter ceiling did.
// Invalid responses are NOT accepted by this sizing: the original parser still
// runs after the permanent claim with its unchanged allocation and grammar.
func submitPlan(h durableInventoryHead, answerBytes int) (durableCompletionPlan, uint64, uint64, error) {
	p := h.Completion
	if len(p.Inputs) != len(p.Payloads) || len(p.Inputs) != len(p.Adapters) || len(p.Inputs) > 10000 {
		return p, 0, 0, ErrDurableInventory
	}
	adapters := append([]int(nil), p.Adapters...)
	if h.Phase == "Describe" && answerBytes > 0 && answerBytes <= p.Limits.MaxAdapterBytes {
		ordinal := 0
		for i, n := range p.Inputs {
			if n > 0 {
				ordinal++
				if ordinal == h.Sequence {
					adapters[i] = answerBytes
					break
				}
			}
		}
	}
	refs := make([]publication.StoredObject, len(p.Inputs))
	for i, n := range p.Inputs {
		if n > 0 {
			refs[i] = publication.StoredObject{ID: strings.Repeat("0", 64), Bytes: n}
		}
	}
	next, e := quoteDurableCompletionBounds(refs, p.InitialStateBytes, p.Manifest, p.Limits, p.Payloads, adapters)
	if e != nil || len(next.Stages) != len(p.Stages) {
		return p, 0, 0, ErrDurableInventory
	}
	next.PreparationUsed = p.PreparationUsed
	// Spent stages are never requoted/refunded. The current semantic stage keeps
	// its original full allocation too; only its successor construction is added.
	for i := 0; i <= h.CompletionCursor; i++ {
		next.Stages[i] = p.Stages[i]
	}
	nextBytes := 0
	switch h.Phase {
	case "Describe":
		ordinal := 0
		for _, n := range p.Inputs {
			if n > 0 {
				ordinal++
				if ordinal == h.Sequence+1 {
					nextBytes = n
					break
				}
			}
		}
		if nextBytes == 0 {
			nextBytes = next.BaseUpper
		}
	case "GroupDecision":
		nextBytes = next.ProposedUpper
	case "Reviewer": // Published has no required caller evidence pass.
	default:
		return p, 0, 0, ErrDurableInventory
	}
	var build, delivery, packets uint64
	if nextBytes > 0 {
		build, e = chunkBuildQuote(nextBytes)
		if e != nil {
			return p, 0, 0, e
		}
		delivery, packets, e = chunkDeliveryQuote(nextBytes, int(chunkIndexUpper(nextBytes)), 0)
		if e != nil {
			return p, 0, 0, e
		}
	}
	allocation, ok := capacitySum(next.Stages[h.CompletionCursor], build)
	if !ok {
		return p, 0, 0, ErrDurableInventory
	}
	next.Stages[h.CompletionCursor] = allocation
	future := h
	future.Completion = next
	future.Sequence++
	switch h.Phase {
	case "Describe":
		if h.Sequence == len(next.Stages)-3 {
			future.Phase = "GroupDecision"
		}
	case "GroupDecision":
		future.Phase = "Reviewer"
	case "Reviewer":
		future.Phase = "Published"
	}
	escrow, escrowPackets, e := successorEscrow(future)
	if e != nil {
		return p, 0, 0, e
	}
	delivery, ok = capacitySum(delivery, escrow)
	if !ok {
		return p, 0, 0, ErrDurableInventory
	}
	return next, delivery, packets + escrowPackets, nil
}

func (s DurableStore) claimSubmit(ctx context.Context, h durableInventoryHead, old []byte, answerBytes int) (durableInventoryHead, []byte, uint64, error) {
	bad := func() (durableInventoryHead, []byte, uint64, error) { return h, nil, 0, ErrDurableInventory }
	if ctx == nil || ctx.Err() != nil || h.Status != "READY" || !h.completionValid() || s.accepted(h) != nil {
		return bad()
	}
	original := h
	baseAllocation := h.Completion.Stages[h.CompletionCursor]
	// Fund sizing/array construction before doing it. A lost process here leaves
	// the ordinary CLAIMED state and cannot renew this planning allowance.
	var e error
	h, old, e = s.reserve(ctx, h, old, baseAllocation, true, true)
	if e != nil {
		return bad()
	}
	next, delivery, packets, e := submitPlan(original, answerBytes)
	if e != nil {
		return bad()
	}
	currentRead, currentPackets, ok := h.deliveryRemaining()
	if !ok {
		return bad()
	}
	allocation := next.Stages[original.CompletionCursor]
	if allocation < baseAllocation {
		return bad()
	}
	topup := allocation - baseAllocation
	tail, ok := capacitySum(next.Stages[h.CompletionCursor:]...)
	required, sumOK := capacitySum(topup, currentRead, tail, delivery)
	if !ok || !sumOK || required > h.Limit-h.Used || required > ((1<<32)-h.Work)/32 || h.Packets+currentPackets+uint64(len(next.Stages)-h.CompletionCursor)+packets > 64 {
		return bad()
	}
	h.Completion = next
	h.SuccessorReserve, h.SuccessorPackets = 0, 0
	// Skipping evidence does not create a refund or free packet credits. This is
	// conservative accounting, not an assertion that the caller read the input.
	h.Delivery.Next = h.Input.Bytes
	h.DeliverySpent += currentRead
	h.Used += topup + currentRead
	h.Work = h.Used * 32
	h.Packets += currentPackets
	h.Status = "CLAIMED"
	raw, e := s.change(ctx, old, h)
	return h, raw, allocation, e
}
