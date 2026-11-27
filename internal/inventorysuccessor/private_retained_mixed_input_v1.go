package inventorysuccessor

import (
	"context"
	"errors"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
	"reflect"
	"time"
)

type privateRetainedMixedInputV1 struct {
	acquisition *privateDiscoveryOperationV2
	owned       sessionruntime.PrivateDiscoveryOwnedMethods
	index       int
	descriptor  sessionruntime.PrivateDiscoveryGroupingQuery
	native      *privateRetainedNativeCallsV1
	root        *publication.Root
	start       sessionruntime.StartRequest
	occurrence  string
}

func (r *privateRetainedMixedInputV1) matches(q PrivateMixedInputRequestV2) bool {
	if r == nil || q.Manager != nil || q.Root != r.root || q.retained != r || q.Timing != nil || len(q.Kinds) == 0 || len(q.Kinds) > 2 || q.URI != r.descriptor.URI || q.Line != r.descriptor.Line || q.Character != r.descriptor.Character || q.OccurrenceID != r.occurrence || q.Revision != r.acquisition.binding.Revision || q.Privacy != "PRIVATE" || q.OccurrencePolicy != "ordinal" || q.CompositionPolicy != "identity" || !reflect.DeepEqual(q.Start, r.start) {
		return false
	}
	kind := programcadmission.TypedCalls
	if r.descriptor.Method == "textDocument/definition" {
		kind = programcadmission.TypedResolvesToDefinition
	}
	if r.descriptor.Method == "textDocument/references" {
		kind = programcadmission.TypedReferencesSymbol
	}
	if q.Kinds[0] != kind {
		return false
	}
	if r.native == nil {
		return len(q.Kinds) == 1 && kind != programcadmission.TypedCalls
	}
	if r.native.acquisition != r.acquisition || r.native.summary.URI != q.URI {
		return false
	}
	if kind == programcadmission.TypedCalls {
		return len(q.Kinds) == 1 && r.index == 128+r.native.index
	}
	return len(q.Kinds) == 2 && q.Kinds[1] == programcadmission.TypedCalls
}
func privateRetainedMixedQuoteV1(now, deadline time.Time) (countersV2, error) {
	c, e := privateMixedChargeV2(now, deadline, sessionruntime.PrivateProviderWindowBounds{WireBytes: 1})
	c.ProviderRequests, c.ProviderMessages, c.WireBytes = 0, 0, 0
	return c, e
}
func privateSelectedMixedQuoteV1(q PrivateMixedInputRequestV2, now time.Time, envelope sessionruntime.PrivateProviderWindowEnvelope) (countersV2, error) {
	if q.retained != nil {
		if !q.retained.matches(q) {
			return countersV2{}, ErrPrivateMixedInputV2
		}
		return privateRetainedMixedQuoteV1(now, q.Start.Deadline)
	}
	return privateMixedChargeV2(now, q.Start.Deadline, envelope.Bounds())
}

// This private completion token exists only after the old acquisition's joined
// completion, actual opaque runtime consumption, unchanged issuance/replay, and
// the selected M operation's OBSERVED readback. It is not sourceConfirmed=true.
type privateRetainedCompletionV1 struct {
	operation           *privateManagedInputOperationV2
	retained            *privateRetainedMixedInputV1
	source, input, head string
}

func (c *privateRetainedCompletionV1) valid(in PrivateManagedGroupingInput) bool {
	if c == nil || c.operation == nil || c.retained == nil || c.source != in.sourceOriginal || c.input != in.sourceInputOriginal.ID() || !hashV2(c.head) {
		return false
	}
	s := c.operation
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.finished && !s.failed && s.binding.retained == c.retained && s.source == c.source
}
func bindGroupingProducerSourceV1(p PrivateManagedInputOperationV2, h sessionruntime.PrivateGroupingOwnedHandoff, r *privateRetainedMixedInputV1, source string) error {
	if r == nil {
		return p.bindOwnedFixed(h, source)
	}
	if p.state == nil || h != (sessionruntime.PrivateGroupingOwnedHandoff{}) || !hashV2(source) {
		return ErrPrivateManagedInputOperationV2
	}
	s := p.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.binding.retained != r || s.binding.Mode != "RETAINED" || !s.used || s.failed || s.finished || s.execution == nil || s.execution.Err() != nil || s.source != "" || !time.Now().Before(s.binding.Deadline) {
		return ErrPrivateManagedInputOperationV2
	}
	s.source = source
	return nil
}
func executeRetainedMixedInputV1(ctx context.Context, op PrivateMixedInputOperationV2, q PrivateMixedInputRequestV2) (PrivateManagedGroupingInput, error) {
	fail := func(e error) (PrivateManagedGroupingInput, error) {
		return PrivateManagedGroupingInput{}, errors.Join(ErrPrivateMixedInputV2, e)
	}
	if ctx == nil || op.state == nil {
		return fail(nil)
	}
	s := op.state
	s.mu.Lock()
	if s.used {
		s.mu.Unlock()
		return fail(nil)
	}
	s.used = true
	if ctx.Err() != nil || q.retained == nil || !q.retained.matches(q) || s.owner == nil || !reflect.DeepEqual(q, s.request) || s.retainedContext == nil || s.retainedContext.Err() != nil {
		s.mu.Unlock()
		return fail(nil)
	}
	s.mu.Unlock()
	// The reservation owns execution; the independent caller can cancel it but
	// cannot replace its lifetime or renew the original acquisition deadline.
	execution, cancel, err := preparedDescribeContextsV1(ctx, s.retainedContext, q.Start.Deadline)
	if err != nil {
		return fail(nil)
	}
	defer cancel()
	o := s.owner
	o.mu.Lock()
	head, e := o.k.reopen(execution, o.job, o.limits)
	valid := e == nil && !o.uncertain && o.head == s.head && head.Identity == s.head && head.Disposition == "PENDING" && head.Attempt == s.attempt
	o.mu.Unlock()
	if !valid {
		return fail(e)
	}
	r := q.retained
	a := r.acquisition
	a.mu.Lock()
	confirmed := a.completed && a.executionErr == nil && !a.observation.Failed && a.observation.CleanupJoined && a.selectedClaimed[r.index] && a.observation.OwnedMethods.Available()
	session := r.owned.GroupingSession()
	a.mu.Unlock()
	if !confirmed {
		return fail(nil)
	}
	grouping := PrivateManagedGroupingRequest{Session: session, URI: q.URI, OccurrenceID: q.OccurrenceID, Revision: q.Revision, Privacy: q.Privacy, OccurrenceAdmissionPolicyID: q.OccurrencePolicy, CompositionPolicyID: q.CompositionPolicy, Line: q.Line, Character: q.Character, Kinds: q.Kinds, Root: q.Root, Deadline: q.Start.Deadline}
	if r.native != nil {
		grouping.Calls = r.native.native.Admission.Admission
		grouping.nativeProof = r.native.native.nativeEvidence
	}
	binding, e := PrivateManagedGroupingOperationBinding(nil, grouping, "RETAINED", sessionruntime.PrivateGroupingOwnedHandoff{})
	if e != nil {
		return fail(e)
	}
	binding.retained = r
	operation := PrivateManagedInputOperationV2{state: &privateManagedInputOperationV2{owner: o, binding: binding, attempt: s.attempt, head: s.head, used: true, execution: execution, cancel: cancel}}
	defer operation.abortFixed()
	grouping.Operation = operation
	out, e := issuePrivateManagedGroupingOwnedV1(execution, nil, sessionruntime.PrivateGroupingOwnedHandoff{}, grouping, r)
	if e != nil {
		return fail(e)
	}
	if len(out.ledgerOriginal) == 0 {
		return fail(ErrPrivateMixedInputEmptyV2)
	}
	if _, ok := out.sourceEvidence.EncodedBound(); !ok {
		return fail(nil)
	}
	// AfterFunc propagation is asynchronous. Check the caller synchronously at
	// the completion boundary as well as the reservation-derived execution.
	if ctx.Err() != nil {
		return fail(ctx.Err())
	}
	if e := operation.completeFixed(execution); e != nil {
		return fail(e)
	}
	o.mu.Lock()
	readback, e := o.k.reopen(execution, o.job, o.limits)
	valid = e == nil && readback.Identity == o.head && readback.Disposition == "OBSERVED" && readback.Attempt == s.attempt
	o.mu.Unlock()
	if !valid || execution.Err() != nil || ctx.Err() != nil {
		return fail(e)
	}
	out.retainedCompletion = &privateRetainedCompletionV1{operation: operation.state, retained: r, source: out.sourceOriginal, input: out.sourceInputOriginal.ID(), head: readback.Identity}
	return out, nil
}
