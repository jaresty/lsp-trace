package inventorysuccessor

import (
	"context"
	"errors"
	"reflect"
	"time"
)

// selectedFutureFitsV2 is a pure feasibility check. Future stages are not debited.
// The authorization argument is the actual canonical M read, never a plan DTO.
func selectedFutureFitsV2(p resourceHeadV2, a authorizationV2, stages [3]countersV2, now, deadline int64) error {
	total := p.Charged
	windows := p.Windows
	for _, c := range stages {
		if validTrancheV2(a, c, now, deadline) != nil {
			return ErrPrivateSelectedBoundsV2
		}
		var err error
		total, err = addCountersV2(total, c)
		if err != nil {
			return ErrPrivateSelectedBoundsV2
		}
		windows, err = nextV2(windows)
		if err != nil {
			return ErrPrivateSelectedBoundsV2
		}
	}
	if !total.within(a.Ceiling) || windows > a.MaxWindows {
		return ErrPrivateSelectedBoundsV2
	}
	return nil
}

// Additive entry; the common kernel reserve/timing/R/CAS implementations remain
// unchanged. Verification/attempt retirement, selected predecessor, authorization
// read and publication semantics match reserve. There is NO separate reopen for
// future feasibility and NO caller callback. Only stages[0] is actually charged.
func (k *kernelV2) reserveSelectedPreparationV2(ctx context.Context, j jobV2, expected, attempt string, stages [3]countersV2, deadline, now int64, l verificationLimitsV2) (resourceHeadV2, error) {
	if !hashV2(expected) || !hashV2(attempt) {
		return resourceHeadV2{}, errRecordV2
	}
	b, err := budgetV2(l)
	if err != nil {
		return resourceHeadV2{}, err
	}
	p, raw, err := k.verify(ctx, j, b, attempt)
	if err != nil {
		return resourceHeadV2{}, err
	}
	if p.Identity != expected || !readyV2(p) || now < p.At {
		return resourceHeadV2{}, errConflictV2
	}
	a, err := k.currentAuthorization(ctx, p, b)
	if err != nil {
		return resourceHeadV2{}, err
	}
	if err = selectedFutureFitsV2(p, a, stages, now, deadline); err != nil {
		return resourceHeadV2{}, err
	}
	charged, err := addCountersV2(p.Charged, stages[0])
	seq, e := nextV2(p.Sequence)
	windows, we := nextV2(p.Windows)
	if err != nil || e != nil || we != nil {
		return resourceHeadV2{}, errRecordV2
	}
	r := reservationV2{Schema: reservationSchemaV2, Job: j.Identity, Authorization: a.Identity, Revision: a.Revision, PreviousHead: p.Identity, Sequence: seq, Attempt: attempt, Phase: "RESOURCE_WINDOW", Tranche: stages[0], Charged: charged, ClaimedAt: now, Deadline: deadline, Disposition: "PENDING"}
	selector, err := k.put(ctx, reservationsNamespaceV2, &r)
	if err != nil {
		return resourceHeadV2{}, err
	}
	h := p
	h.Identity = ""
	h.Sequence = seq
	h.Previous = p.Identity
	h.ReservationSelector = selector
	h.Charged = charged
	h.Windows = windows
	h.Attempt = attempt
	h.Disposition = "PENDING"
	h.Event = "RESERVE"
	h.At = now
	return k.commit(ctx, j, raw, h)
}

// The plan is immutable-by-interface, one-shot quotation state, not authority.
// The owner must already be canonically admitted/funded. This entry never opens,
// initializes, renews or repairs an owner and never creates a numerical grant.
func (o *PrivateManagedInputOwnerV2) ReserveSelectedMixedInputV2(ctx context.Context, attempt string, q PrivateMixedInputRequestV2, plan PrivateSelectedPreparationPlanV2) (PrivateMixedInputOperationV2, error) {
	if plan.state == nil {
		return PrivateMixedInputOperationV2{}, ErrPrivateSelectedV2
	}
	s := plan.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reserved {
		return PrivateMixedInputOperationV2{}, ErrPrivateSelectedV2
	}
	s.reserved = true
	if o == nil || o != s.owner || ctx == nil || ctx.Err() != nil || attempt != s.config.Attempts[0] || q.Timing != nil || !reflect.DeepEqual(q, s.request) {
		return PrivateMixedInputOperationV2{}, ErrPrivateSelectedV2
	}
	if d, ok := ctx.Deadline(); ok && d.Before(q.Start.Deadline) {
		return PrivateMixedInputOperationV2{}, ErrPrivateSelectedV2
	}
	now := time.Now()
	m, err := privateSelectedMixedQuoteV1(q, now, s.envelope)
	if err != nil || !m.within(s.stages[0]) {
		return PrivateMixedInputOperationV2{}, ErrPrivateSelectedBoundsV2
	}
	stages := s.stages
	stages[0] = m // actual M quote; original future quotes dominate later stages
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.k == nil || o.uncertain || q.Root != o.k.root || o.head != s.expectedHead || s.config.ExpectedSourceAuthorization != "" && o.job.SourceAuthorization != s.config.ExpectedSourceAuthorization {
		return PrivateMixedInputOperationV2{}, ErrPrivateSelectedV2
	}
	h, err := o.k.reserveSelectedPreparationV2(ctx, o.job, o.head, attempt, stages, q.Start.Deadline.UnixNano(), now.UnixNano(), o.limits)
	if err != nil {
		if errors.Is(err, errUncertainV2) {
			o.uncertain = true
		}
		return PrivateMixedInputOperationV2{}, err
	}
	o.head = h.Identity
	// Use the plan's already detached original arrays, not the borrowed caller's.
	op := PrivateMixedInputOperationV2{&privateMixedInputOperationV2{owner: o, request: s.request, envelope: s.envelope, head: h.Identity, attempt: attempt, retainedContext: ctx}}
	s.operation = op
	return op, nil
}
