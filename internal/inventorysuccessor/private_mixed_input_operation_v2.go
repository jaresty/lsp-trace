package inventorysuccessor

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

var ErrPrivateMixedInputV2 = errors.New("private mixed input checkpoint refused")
var ErrPrivateMixedInputUnavailableV2 = errors.New("private mixed input requested family unavailable")
var ErrPrivateMixedInputEmptyV2 = errors.New("private mixed input observed empty; no grouping input")

type PrivateMixedInputRequestV2 struct {
	Manager                                                                   *sessionruntime.Manager
	Root                                                                      *publication.Root
	Start                                                                     sessionruntime.StartRequest
	URI, OccurrenceID, Revision, Privacy, OccurrencePolicy, CompositionPolicy string
	Line, Character                                                           uint32
	Kinds                                                                     []programcadmission.RelationKind
}
type PrivateMixedInputOperationV2 struct{ state *privateMixedInputOperationV2 }
type privateMixedInputOperationV2 struct {
	mu            sync.Mutex
	used          bool
	owner         *PrivateManagedInputOwnerV2
	request       PrivateMixedInputRequestV2
	envelope      sessionruntime.PrivateProviderWindowEnvelope
	head, attempt string
}

func privateMixedPlanV2(q PrivateMixedInputRequestV2) (sessionruntime.PrivateProviderWindowPlan, bool) {
	var p sessionruntime.PrivateProviderWindowPlan
	if q.Manager == nil || q.Root == nil || len(q.Kinds) == 0 || len(q.Kinds) > 3 || q.Start.Deadline.IsZero() {
		return p, false
	}
	for _, s := range []string{q.URI, q.OccurrenceID, q.Revision, q.Privacy, q.OccurrencePolicy, q.CompositionPolicy} {
		if !sessionruntime.PrivateGroupingFieldValid(s) {
			return p, false
		}
	}
	for _, k := range q.Kinds {
		switch k {
		case programcadmission.TypedCalls:
			if p.Calls {
				return p, false
			}
			p.Calls = true
		case programcadmission.TypedReferencesSymbol:
			if p.References {
				return p, false
			}
			p.References = true
		case programcadmission.TypedResolvesToDefinition:
			if p.Definitions {
				return p, false
			}
			p.Definitions = true
		default:
			return p, false
		}
	}
	return p, true
}

// One finite logical quote, NOT physical resource proof or an authorization.
// W is the actual enforced window envelope (includes initialize and shutdown).
// S=8MiB covers TWO ordinary 64-file/1MiB-file/4MiB-pass source acquisitions.
// B=128MiB covers bounded transient native graphs, constituents, composition,
// proof, old private query and eight <=1MiB retained artifacts, including rejecting
// and base64/copy passes. 64 passes cover parsing, normalization, reconstruction,
// hashing, source copies, encoding, re-admission and exact readbacks. J=1024^2
// bounds raw-to-edge and typed join work. Two kernel verifications each admit
// 4096 records/64MiB/16384 work. 16384 method credits cover the unchanged local
// supervisor's finite observation schedule; these are not OS snapshot bytes.
// Original-frame reconstruction uses a component within B, not an added grant:
// three records each detach <=128KiB and <=32 responses (<=384KiB raw, <=512KiB
// base64). Frame decode/ledger/hash scratch fits a 4MiB logical component and
// eight of the 64 passes. The unchanged <=1MiB complete proof preflight refuses
// excess before encoding; raw/encoded forms are not each granted another MiB.
// These are logical upper allowances, not a physical heap/CPU/IO measurement.
// No old D/R quote is reused as a grant for C or startup. All fourteen coordinates
// are charged before Start under the unchanged existing authorization/ceilings.
func privateMixedChargeV2(now, deadline time.Time, w sessionruntime.PrivateProviderWindowBounds) (countersV2, error) {
	if !now.Before(deadline) || deadline.Sub(now) > 15*time.Second || w.WireBytes == 0 || w.WireBytes > 128<<20 {
		return countersV2{}, ErrPrivateMixedInputV2
	}
	const b uint64 = 128 << 20
	const s uint64 = 8 << 20
	const verify uint64 = 2 * (64 << 20)
	data := b + s + w.SourceReadBytes + w.RequestBytes + w.ResponseBytes
	records := uint64(2*4096 + 128*1024 + 8*8)
	calls := uint64(2*16384+16384) + records
	return countersV2{LogicalBytes: 64 * data, ReadHashBytes: verify + 64*data, WriteBytes: 32*(1<<20) + 8*(recordLimitV2+2*headLimitV2), Records: records, ParserBytes: verify + 64*data, ProviderRequests: 16, ProviderMessages: w.Frames, Work: 2*16384 + 64*(data+1024*1024) + 16384, ActiveNS: uint64(deadline.Sub(now)) + uint64(time.Second), Calls: calls, RequestBytes: w.RequestBytes, ResponseBytes: w.ResponseBytes, WireBytes: w.WireBytes, OutputBytes: 8 * (1 << 20)}, nil
}

func (o *PrivateManagedInputOwnerV2) ReserveMixedInputV2(ctx context.Context, attempt string, q PrivateMixedInputRequestV2) (PrivateMixedInputOperationV2, error) {
	fail := func() (PrivateMixedInputOperationV2, error) {
		return PrivateMixedInputOperationV2{}, ErrPrivateMixedInputV2
	}
	plan, ok := privateMixedPlanV2(q)
	if o == nil || ctx == nil || ctx.Err() != nil || !ok || !hashV2(attempt) {
		return fail()
	}
	now := time.Now()
	if d, ok := ctx.Deadline(); ok && d.Before(q.Start.Deadline) {
		return fail()
	}
	envelope, err := q.Manager.PrivateProviderWindowEnvelope(q.Start, plan)
	if err != nil {
		return fail()
	}
	charge, err := privateMixedChargeV2(now, q.Start.Deadline, envelope.Bounds())
	if err != nil {
		return fail()
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.k == nil || o.uncertain || q.Root != o.k.root {
		return fail()
	}
	h, err := o.k.reserve(ctx, o.job, o.head, attempt, charge, q.Start.Deadline.UnixNano(), now.UnixNano(), o.limits)
	if err != nil {
		if errors.Is(err, errUncertainV2) {
			o.uncertain = true
		}
		return fail()
	}
	o.head = h.Identity
	// Freeze mutable request arrays only after confirmed funding.
	q.Kinds = append([]programcadmission.RelationKind(nil), q.Kinds...)
	q.Start.Process.Args = append([]string(nil), q.Start.Process.Args...)
	q.Start.Process.Env = append([]string(nil), q.Start.Process.Env...)
	q.Start.RelationProviders = append([]string(nil), q.Start.RelationProviders...)
	return PrivateMixedInputOperationV2{&privateMixedInputOperationV2{owner: o, request: q, envelope: envelope, head: h.Identity, attempt: attempt}}, nil
}

// ExecuteMixedInputV2 is a fixed internal continuation, not a caller executor.
// The only successful return is AFTER original-lease return, window worker join,
// and confirmed Observe. Failures retain PENDING/uncertain cumulative accounting.
func ExecuteMixedInputV2(ctx context.Context, op PrivateMixedInputOperationV2, q PrivateMixedInputRequestV2) (PrivateManagedGroupingInput, error) {
	fail := func(err error) (PrivateManagedGroupingInput, error) {
		return PrivateManagedGroupingInput{}, errors.Join(ErrPrivateMixedInputV2, err)
	}
	if op.state == nil || ctx == nil {
		return fail(nil)
	}
	s := op.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.used {
		return fail(nil)
	}
	s.used = true
	if ctx.Err() != nil || !reflect.DeepEqual(q, s.request) || !time.Now().Before(q.Start.Deadline) {
		return fail(nil)
	}
	o := s.owner
	o.mu.Lock()
	live := !o.uncertain && o.head == s.head
	o.mu.Unlock()
	if !live {
		return fail(nil)
	}
	execution, cancel := context.WithDeadline(ctx, q.Start.Deadline)
	defer cancel()
	window, err := q.Manager.PrivateProviderWindowContext(execution, s.envelope, q.Start.Deadline)
	if err != nil {
		return fail(err)
	}
	// Failure cleanup uses the SAME original window; no deadline renewal/reset.
	defer func() { _, _ = q.Manager.FinishPrivateProviderWindow(window) }()
	start := q.Manager.Start(window, q.Start)
	if start.Failure != "" {
		return fail(errors.New(string(start.Failure)))
	}
	pending := q.Manager.BeginReadiness(window, start.SessionID, start.Generation, q.Start.Deadline)
	ready, ok := q.Manager.WaitReadiness(window, pending.ID)
	if !ok || ready.State != sessionruntime.ReadinessReady {
		return fail(fmt.Errorf("private mixed readiness: ok=%v state=%s failure=%s", ok, ready.State, ready.Failure))
	}
	session := sessionruntime.SessionLeaseRequest{SessionID: start.SessionID, Generation: start.Generation, Workspace: q.Start.Profile.Workspace().String()}
	plan, _ := privateMixedPlanV2(q)
	grouping := PrivateManagedGroupingRequest{Session: session, URI: q.URI, OccurrenceID: q.OccurrenceID, Revision: q.Revision, Privacy: q.Privacy, OccurrenceAdmissionPolicyID: q.OccurrencePolicy, CompositionPolicyID: q.CompositionPolicy, Line: q.Line, Character: q.Character, Kinds: q.Kinds, Root: q.Root, Deadline: q.Start.Deadline}
	// Private fixed state shares the already-confirmed reservation. No nested
	// Reserve under PENDING and no manual Complete/BindOwned surface is opened.
	binding, err := PrivateManagedGroupingOperationBinding(q.Manager, grouping, "ACQUIRE", sessionruntime.PrivateGroupingOwnedHandoff{})
	if err != nil {
		return fail(err)
	}
	operation := PrivateManagedInputOperationV2{&privateManagedInputOperationV2{owner: o, binding: binding, attempt: s.attempt, head: s.head, used: true, execution: execution, cancel: cancel}}
	grouping.Operation = operation
	defer operation.abortFixed()
	var out PrivateManagedGroupingInput
	err = q.Manager.WithReadySessionLease(window, session, func(base context.Context) error {
		req := sessionruntime.PrivateGroupingOwnedRequest{Session: session, URI: q.URI, OccurrenceID: q.OccurrenceID, Revision: q.Revision, Privacy: q.Privacy, Line: q.Line, Character: q.Character, References: plan.References, Definitions: plan.Definitions, Deadline: q.Start.Deadline}
		var h sessionruntime.PrivateGroupingOwnedHandoff
		var e error
		if plan.References || plan.Definitions {
			h, e = q.Manager.CapturePrivateGroupingOwned(base, req)
		} else {
			h, e = q.Manager.CapturePrivateGroupingQueryOwned(base, req)
		}
		if e != nil {
			return e
		}
		if plan.Calls {
			line, ch := q.Line, q.Character
			native, e := constructNativeCallsPrivateV2(base, q.Manager, PrivateNativeCallsRequestV2{Session: session, Revision: q.Revision, Privacy: q.Privacy, Directional: sessionruntime.DirectionalCallHierarchyRequestPrivate{SessionID: session.SessionID, Generation: session.Generation, URI: q.URI, LanguageID: q.Start.LanguageID, Line: &line, Character: &ch, Deadline: q.Start.Deadline, MaxMessages: 4, MaxBytes: 65536}}, true)
			if e != nil {
				return e
			}
			grouping.Calls = native.Admission.Admission
			grouping.nativeProof = native.nativeEvidence
		}
		out, e = issuePrivateManagedGroupingInput(base, q.Manager, h, grouping)
		return e
	})
	if err != nil {
		return fail(err)
	}
	if len(out.Ledger) == 0 {
		for _, kind := range q.Kinds {
			method := "CALLS"
			if kind == programcadmission.TypedReferencesSymbol {
				method = "textDocument/references"
			}
			if kind == programcadmission.TypedResolvesToDefinition {
				method = "textDocument/definition"
			}
			if out.Dispositions[method] == "UNAVAILABLE" {
				return fail(ErrPrivateMixedInputUnavailableV2)
			}
		}
		return fail(ErrPrivateMixedInputEmptyV2)
	}
	completion, err := q.Manager.FinishPrivateProviderWindow(window)
	if err != nil || !completion.Completed || q.Manager.Census().Workers != 0 {
		return fail(err)
	}
	if err = operation.completeFixed(execution); err != nil {
		return fail(err)
	}
	// The initial exact replay above is paid by this mixed reservation. A later
	// replay must not silently reuse the old D/R-only operation quote.
	if plan.Calls {
		out.replayBinding.Mode = "MIXED_NATIVE_REPLAY_UNQUOTED"
	}
	return out, nil
}
