package inventorysuccessor

import (
	"context"
	"errors"
	"sync"
	"time"

	qt "lsp-trace/internal/adr0011querytarget"
	"lsp-trace/internal/describeworker"
	sourceclock "lsp-trace/internal/privatesourceclock"
	pc "lsp-trace/internal/programc"
	"lsp-trace/internal/publication"
	rg "lsp-trace/internal/referencegrouping"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/targetpacket"
	"lsp-trace/sessionruntime"
)

var ErrPrivateSourceRPreparationV1 = errors.New("private source R preparation refused")

// PrivateSourceRPreparationRequestV1 spends existing owner custody. None of its
// public metadata is a grant. Input must be the original completed R producer.
// Seed is the original scheduled seed, never replaced by an affordability probe.
type PrivateSourceRPreparationRequestV1 struct {
	Input                                                            PrivateManagedGroupingInput
	Root                                                             *publication.Root
	ExpectedHead, Attempt, Workspace, Revision, RunID, SeedNamespace string
	Seed                                                             uint64
	Deadline                                                         time.Time
	Local                                                            *PrivateSourceLocalConfigV1
}
type PrivateSourceRPreparationOperationV1 struct {
	state *privateSourceRPreparationStateV1
}
type privateSourceRPreparationStateV1 struct {
	mu                       sync.Mutex
	used                     bool
	owner                    *PrivateManagedInputOwnerV2
	request                  PrivateSourceRPreparationRequestV1
	head                     string
	local                    *PrivateSourceLocalConfigV1
	execution                context.Context
	reservedContext          context.Context
	cancel                   context.CancelFunc
	metadata                 *preparedSourceRStateV1
	verified, finalized      bool
	localVerificationStarted bool
	localWorkerStarted       bool
	localStageDir            string
	localStage               *localStageNamespaceV1
	localManagedUse          *localManagedUseV1
	localStagedConfig        describeworker.Config
	localExecutorContract    string
	localWorkerObservation   PrivateSourceLocalWorkerObservationV1
	localFailure             error
}

// PreparedSourceRV1 is minted only after confirmed Observe. JSON, hashes and zero
// values cannot reconstruct it. Copies preserve opaque in-process custody.
type PreparedSourceRV1 struct{ state *preparedSourceRStateV1 }

// sourceRMetadataV1 is internal data, not an observed preparation proof.
type sourceRMetadataV1 struct{ state *preparedSourceRStateV1 }
type preparedSourceRStateV1 struct {
	input          PrivateManagedGroupingInput
	originals      PrivateProducerOriginals
	policy         rg.GroupingPolicy
	frozen         pc.GroupingEvaluationCasePrivate
	result         pc.GroupingEvaluationResultPrivate
	objects        []sourceobject.Object
	originalTarget originalQueryTargetV1
}

// Cached only in the existing paid Snapshot continuation, never from a caller
// unit. This is comparison metadata, not source admission or resumable custody.
type originalQueryTargetV1 struct {
	uri, sourceDigest, sessionID, documentVersion, encoding, symbolID string
	workspace, revision, privacy                                      string
	generation                                                        uint64
	display, selection                                                qt.Range
}

// MatchesOriginalQueryTarget compares scalar metadata without snapshotting,
// hashing, parsing, copying source, or granting source admission. A matching DTO
// must still pass the unchanged whole-record/opaque-admission Resume join.
func (p sourceRMetadataV1) MatchesOriginalQueryTarget(u targetpacket.PrivateSourceUnit) bool {
	if p.state == nil {
		return false
	}
	o := &p.state.originalTarget
	return o.symbolID != "" && u.ID != "" &&
		u.Version == targetpacket.PrivateSourceVersion && u.ClaimCeiling == targetpacket.PrivateSourceCeiling &&
		u.InputID == "" && u.URI == o.uri && u.SourceDigest == o.sourceDigest &&
		u.Workspace == o.workspace && u.Revision == o.revision && u.Privacy == o.privacy &&
		u.SessionID == o.sessionID && u.Generation == o.generation &&
		u.DocumentVersion == o.documentVersion && u.PositionEncoding == o.encoding &&
		u.SymbolID == o.symbolID && u.SelectionRange == o.selection && u.DisplayRange == o.display
}

// OriginalQueryTargetID returns only comparison metadata, never admission. It
// rejects missing, multiple, and conflicting selected IDs without changing units.
func (p sourceRMetadataV1) OriginalQueryTargetID(units []targetpacket.PrivateSourceUnit) (string, error) {
	if len(units) == 0 || len(units) > 1000 {
		return "", ErrPrivateSourceRPreparationV1
	}
	id := ""
	for _, u := range units {
		if p.MatchesOriginalQueryTarget(u) {
			if id != "" {
				return "", ErrPrivateSourceRPreparationV1
			}
			id = u.ID
		}
	}
	if id == "" {
		return "", ErrPrivateSourceRPreparationV1
	}
	count := 0
	for _, u := range units {
		if u.ID == id {
			count++
		}
	}
	if count != 1 {
		return "", ErrPrivateSourceRPreparationV1
	}
	return id, nil
}

// RetainedPayloadUpper reads only already-admitted lengths. It includes the
// prepared input's original/query/publication snapshots and decoded mirrors,
// policy/case/result and the detached projection, plus the original source object.
// Deliberate overcounting of shared originals makes no pointer-sharing assumption.
// This is logical payload retention, not heap size or a new accounting grant.
func (p sourceRMetadataV1) RetainedPayloadUpper() (uint64, bool) {
	if p.state == nil {
		return 0, false
	}
	s := p.state
	b, err := pc.BindGroupingPairPrivate(s.frozen, s.result, s.input.sourceInputOriginal)
	if err != nil || s.originals.Bound() == 0 {
		return 0, false
	}
	n := uint64(32768)
	add := func(v uint64, copies uint64) bool {
		if copies == 0 || v > (^uint64(0)-n)/copies {
			return false
		}
		n += v * copies
		return true
	}
	for _, v := range []int{s.input.sourceInputOriginal.ByteLength(), s.policy.ByteLength(), b.Case.ByteLength, b.Result.ByteLength, b.Projection.ByteLength} {
		if v <= 0 || !add(uint64(v), 2) {
			return 0, false
		}
	}
	// Original producer proof includes typed method/query/source evidence. Four
	// covers its own snapshots, input typed mirrors, and public/original copies.
	if !add(s.originals.Bound(), 4) || !add(uint64(len(s.input.inputOriginal)), 2) || !add(uint64(len(s.input.sourceQueryOriginal)), 2) {
		return 0, false
	}
	for _, group := range [][]rg.PredecessorRecord{s.input.Predecessors, s.input.originals} {
		for _, v := range group {
			if !add(uint64(len(v.Bytes)), 2) || !add(uint64(len(v.Role)+len(v.Selector)+len(v.Schema)), 6) {
				return 0, false
			}
		}
	}
	for _, v := range s.input.publications {
		if !add(uint64(len(v.raw)), 2) || !add(uint64(len(v.selector)+len(v.digest)), 6) {
			return 0, false
		}
	}
	for _, ledger := range [][]PrivateManagedGroupingOccurrence{s.input.Ledger, s.input.ledgerOriginal} {
		for _, v := range ledger {
			if !add(uint64(64+len(v.Identity)+len(v.Kind)+len(v.From)+len(v.To)), 6) {
				return 0, false
			}
		}
	}
	for _, dispositions := range []map[string]string{s.input.Dispositions, s.input.dispositionsOriginal} {
		for k, v := range dispositions {
			if !add(uint64(32+len(k)+len(v)), 6) {
				return 0, false
			}
		}
	}
	for _, v := range s.objects {
		if !add(uint64(len(v.Bytes)), 2) || !add(uint64(len(v.Identity.Digest)+64), 6) {
			return 0, false
		}
	}
	// Charge the cached target struct and all string payloads, even shared ones.
	// Fixed overhead covers scalar/range/string headers; no serialization work.
	o := &s.originalTarget
	if !add(512, 1) {
		return 0, false
	}
	for _, v := range []string{o.uri, o.sourceDigest, o.sessionID, o.documentVersion, o.encoding, o.symbolID, o.workspace, o.revision, o.privacy} {
		if !add(uint64(len(v)), 1) {
			return 0, false
		}
	}
	return n, true
}

func (p sourceRMetadataV1) SourceObjects() ([]sourceobject.Object, error) {
	if p.state == nil {
		return nil, ErrPrivateSourceRPreparationV1
	}
	out := make([]sourceobject.Object, len(p.state.objects))
	for i, v := range p.state.objects {
		out[i] = sourceobject.Object{Identity: v.Identity, Bytes: append([]byte(nil), v.Bytes...)}
	}
	return out, nil
}
func (p sourceRMetadataV1) Values() (rg.AdmittedInput, rg.GroupingPolicy, pc.GroupingEvaluationCasePrivate, pc.GroupingEvaluationResultPrivate, PrivateProducerOriginals, error) {
	if p.state == nil {
		return rg.AdmittedInput{}, rg.GroupingPolicy{}, pc.GroupingEvaluationCasePrivate{}, pc.GroupingEvaluationResultPrivate{}, PrivateProducerOriginals{}, ErrPrivateSourceRPreparationV1
	}
	s := p.state
	return s.input.sourceInputOriginal, s.policy, s.frozen, s.result, s.originals, nil
}

// PreparationHeadV1 is a read-only compare-and-reserve pin, never admission.
func (o *PrivateManagedInputOwnerV2) PreparationHeadV1() string {
	if o == nil {
		return ""
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.k == nil || o.uncertain {
		return ""
	}
	return o.head
}
func sourceRPreparationMetadataV1(q PrivateSourceRPreparationRequestV1) (rg.AllModeMetadataPrivate, error) {
	in := q.Input
	if in.sourceTiming != nil && (in.sourceTiming.retainedContext == nil || in.sourceTiming.retainedContext.Err() != nil || !time.Now().Before(in.sourceTiming.frame.TotalDeadline)) {
		return rg.AllModeMetadataPrivate{}, ErrPrivateSourceRPreparationV1
	}
	b := in.replayBinding
	if !in.sourceConfirmed || q.Root == nil || b.Root != q.Root || b.KindCount != 1 || b.Kinds != [3]string{"REFERENCES_SYMBOL", "", ""} || q.Workspace != b.Session.Workspace || q.Revision != b.Revision || !q.Deadline.Equal(in.SourceDeadlineV1()) || !hashV2(q.ExpectedHead) || !hashV2(q.Attempt) || in.Input.ID() != in.sourceInputOriginal.ID() || len(in.inputOriginal) != in.sourceInputOriginal.ByteLength() || len(in.ledgerOriginal) == 0 || len(in.ledgerOriginal) > 1024 {
		return rg.AllModeMetadataPrivate{}, ErrPrivateSourceRPreparationV1
	}
	for _, s := range []string{q.Workspace, q.Revision, q.RunID, q.SeedNamespace, b.URI, b.Occurrence} {
		if !sessionFieldRPreparationV1(s) {
			return rg.AllModeMetadataPrivate{}, ErrPrivateSourceRPreparationV1
		}
	}
	// Only bounded opaque scalar/count metadata; no payload hash/copy/import.
	return rg.ReadAllModeMetadataPrivate(in.sourceInputOriginal)
}
func sessionFieldRPreparationV1(s string) bool { return sessionruntime.PrivateGroupingFieldValid(s) }

// Closed logical currencies, not physical heap/CPU/IO proof. I is the admitted
// canonical input; E=8MiB is the unchanged enforced evaluator canonical ceiling;
// P=64KiB is the fixed three-rule policy ceiling. 64 passes cover policy,
// projection (inside NewCase), frozen declaration, numeric result, canonical
// hashing/serialization and copies, including rejecting passes. W adds one
// (N+O)^2 dense-envelope kernel CREDIT (the existing typed-bundle convention),
// N<=2O from contribution endpoints. Three kernel verifications each admit
// 4096 records/64MiB/16384 work. Eight publication/readback waves cover accounting
// artifacts. OutputBytes covers the four retained preparation representations.
// ProviderRequests/Messages and Request/Response/WireBytes are zero: this fixed
// operation neither starts a provider nor opens a HOST transport. Those effects
// cannot borrow these credits. ActiveNS is only the original remaining deadline.
func sourceRPreparationChargeV1(m rg.AllModeMetadataPrivate, now, deadline time.Time) (countersV2, error) {
	fail := func() (countersV2, error) { return countersV2{}, ErrPrivateSourceRPreparationV1 }
	if m.Input.ByteLength <= 0 || m.Input.ByteLength > rg.EvaluationMaxBytesPrivate || m.Contributions <= 0 || m.Contributions > rg.EvaluationMaxOccurrencesPrivate || m.Custody < 0 || m.Custody > 2*rg.EvaluationMaxOccurrencesPrivate || m.Predecessors < 0 || m.Predecessors > rg.EvaluationMaxOccurrencesPrivate || !now.Before(deadline) || deadline.Sub(now) > 15*time.Second {
		return fail()
	}
	add := func(a, b uint64) (uint64, bool) {
		if a > ^uint64(0)-b {
			return 0, false
		}
		return a + b, true
	}
	mul := func(a, b uint64) (uint64, bool) {
		if b != 0 && a > ^uint64(0)/b {
			return 0, false
		}
		return a * b, true
	}
	const e uint64 = rg.EvaluationMaxBytesPrivate
	data, ok := add(uint64(m.Input.ByteLength), 4*e+(64<<10))
	if !ok {
		return fail()
	}
	data, ok = mul(64, data)
	if !ok {
		return fail()
	}
	n := uint64(m.Contributions) * 3 // N+O, with N bounded by 2O
	numeric, ok := mul(n, n)
	if !ok {
		return fail()
	}
	work, ok := add(data, numeric+3*16384)
	if !ok {
		return fail()
	}
	records := uint64(3*4096) + 16*uint64(m.Contributions+m.Custody+m.Predecessors) + 64
	return countersV2{LogicalBytes: data, ReadHashBytes: 3*(64<<20) + data, WriteBytes: 8 * (recordLimitV2 + 2*headLimitV2), Records: records, ParserBytes: 3*(64<<20) + data, Work: work, ActiveNS: uint64(deadline.Sub(now)), Calls: 3*16384 + records + 1, OutputBytes: 4*e + (64 << 10)}, nil
}
func (o *PrivateManagedInputOwnerV2) ReserveRPreparationV1(ctx context.Context, q PrivateSourceRPreparationRequestV1) (PrivateSourceRPreparationOperationV1, error) {
	fail := func() (PrivateSourceRPreparationOperationV1, error) {
		return PrivateSourceRPreparationOperationV1{}, ErrPrivateSourceRPreparationV1
	}
	if o == nil || ctx == nil || ctx.Err() != nil {
		return fail()
	}
	m, err := sourceRPreparationMetadataV1(q)
	if err != nil {
		return fail()
	}
	now := time.Now()
	charge, err := sourceRPreparationChargeV1(m, now, q.Deadline)
	if q.Input.sourceTiming != nil {
		if q.Local != nil || !now.Before(q.Input.sourceTiming.preparationDeadline) {
			return fail()
		}
		charge, err = sourceTimingPreparationChargeV2(m, now, q.Deadline)
	}
	if err != nil {
		return fail()
	}
	var local *PrivateSourceLocalConfigV1
	if q.Local != nil {
		frozen := *q.Local // fixed arrays and immutable strings; never borrow caller pointer
		if frozen.Managed != nil {
			handle := *frozen.Managed
			frozen.Managed = &handle
		}
		extra, e := sourceLocalConfigChargeV1(frozen)
		if e != nil {
			return fail()
		}
		charge, err = sourceLocalChargeV1(m, now, q.Deadline, extra)
		if err != nil {
			return fail()
		}
		local = &frozen
		q.Local = local
	}
	if d, ok := ctx.Deadline(); ok && d.Before(q.Deadline) {
		return fail()
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.k == nil || o.uncertain || o.k.root != q.Root || o.head != q.ExpectedHead || (q.Input.sourceTiming != nil && o.job.ResourceRules != q.Input.sourceTiming.frame.Profile.Rules()) {
		return fail()
	}
	h, err := o.k.reserve(ctx, o.job, q.ExpectedHead, q.Attempt, charge, q.Deadline.UnixNano(), now.UnixNano(), o.limits)
	if err != nil {
		if errors.Is(err, errUncertainV2) {
			o.uncertain = true
		}
		return fail()
	}
	o.head = h.Identity
	if ctx.Err() != nil || !time.Now().Before(q.Deadline) {
		o.uncertain = true
		return fail()
	}
	return PrivateSourceRPreparationOperationV1{&privateSourceRPreparationStateV1{owner: o, request: q, head: h.Identity, local: local, reservedContext: ctx}}, nil
}
func sameRPreparationV1(a, b PrivateSourceRPreparationRequestV1) bool {
	return sameLocalConfigV1(a.Local, b.Local) && a.Root == b.Root && a.ExpectedHead == b.ExpectedHead && a.Attempt == b.Attempt && a.Workspace == b.Workspace && a.Revision == b.Revision && a.RunID == b.RunID && a.SeedNamespace == b.SeedNamespace && a.Seed == b.Seed && a.Deadline.Equal(b.Deadline) && a.Input.sourceInputOriginal.ID() == b.Input.sourceInputOriginal.ID() && samePrivateManagedBindingV2(a.Input.replayBinding, b.Input.replayBinding) && a.Input.sourceEvidence == b.Input.sourceEvidence
}

// ExecuteRPreparationV1 is a fixed continuation, never a caller callback. Retire
// before validating even nil/cancelled/mismatched calls. Failures keep PENDING.
func ExecuteRPreparationV1(ctx context.Context, op PrivateSourceRPreparationOperationV1, q PrivateSourceRPreparationRequestV1) (PreparedSourceRV1, error) {
	m, err := executeRMetadataV1(ctx, op, q, false)
	if err != nil {
		return PreparedSourceRV1{}, err
	}
	return PreparedSourceRV1{m}, nil
}

func executeRMetadataV1(ctx context.Context, op PrivateSourceRPreparationOperationV1, q PrivateSourceRPreparationRequestV1, local bool) (*preparedSourceRStateV1, error) {
	fail := func() (*preparedSourceRStateV1, error) { return nil, ErrPrivateSourceRPreparationV1 }
	if op.state == nil {
		return fail()
	}
	s := op.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.used {
		return fail()
	}
	s.used = true
	if ctx == nil || ctx.Err() != nil || s.reservedContext == nil || s.reservedContext.Err() != nil || local != (s.local != nil) || !sameRPreparationV1(q, s.request) || !time.Now().Before(q.Deadline) {
		return fail()
	}
	if _, err := sourceRPreparationMetadataV1(q); err != nil {
		return fail()
	}
	// P's original reservation scope owns execution on every path. A live
	// caller (including M's retained scope) cannot revive a canceled P child.
	executionDeadline := q.Deadline
	if q.Input.sourceTiming != nil {
		executionDeadline = q.Input.sourceTiming.preparationDeadline
	}
	execution, cancelExecution := context.WithDeadline(s.reservedContext, executionDeadline)
	// Preserve caller cancellation without substituting the caller for P's
	// parent. The owned cancel travels with Local=true's pending lifecycle.
	stopCaller := context.AfterFunc(ctx, cancelExecution)
	var stopRetained func() bool
	if q.Input.sourceTiming != nil {
		stopRetained = context.AfterFunc(q.Input.sourceTiming.retainedContext, cancelExecution)
	}
	cancel := func() {
		stopCaller()
		if stopRetained != nil {
			stopRetained()
		}
		cancelExecution()
	}
	live := func() bool {
		return execution.Err() == nil && ctx.Err() == nil &&
			(q.Input.sourceTiming == nil || q.Input.sourceTiming.retainedContext.Err() == nil)
	}
	keep := false
	defer func() {
		if !keep {
			cancel()
		}
	}()
	o := s.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.uncertain || o.head != s.head || o.k.root != q.Root {
		return fail()
	}
	h, err := o.k.reopen(execution, o.job, o.limits)
	if err != nil || h.Identity != s.head {
		return fail()
	}
	originals, err := q.Input.SourceOriginalsV1()
	if err != nil {
		return fail()
	}
	view, err := q.Input.sourceEvidence.Snapshot(sessionruntime.PrivateGroupingQueryMaxBytes)
	if err != nil || len(view.Source) == 0 || len(view.Source) > 1<<20 {
		return fail()
	}
	objects := []sourceobject.Object{{Identity: sourceobject.Identity{Digest: view.Query.SourceDigest, ByteLength: uint64(len(view.Source))}, Bytes: view.Source}}
	originalTarget := originalQueryTargetV1{uri: view.Query.URI, sourceDigest: view.Query.SourceDigest, sessionID: view.Query.SessionID, generation: view.Query.Generation, documentVersion: view.Query.DocumentVersion, encoding: view.Query.Encoding, symbolID: view.Target.SymbolID, selection: view.Target.SelectionRange, display: view.Target.DisplayRange, workspace: view.Workspace, revision: view.Revision, privacy: view.Privacy}
	rules := []rg.GroupingPolicyRule{}
	for _, k := range []string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"} {
		r := rg.GroupingPolicyRule{RelationKind: k, Action: "EXCLUDE", RationaleCode: "PRIVATE_SOURCE_R_ONLY", Weight: rg.Rational{Numerator: 0, Denominator: 1}}
		if k == "REFERENCES_SYMBOL" {
			r.Action = "INCLUDE"
			r.Weight.Numerator = 1
		}
		rules = append(rules, r)
	}
	policy, err := rg.BuildGroupingPolicy(rg.GroupingPolicyRequest{PolicyID: "PRIVATE_SOURCE_R_ONLY", Epoch: 1, EligibleKinds: []string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"}, Rules: rules, Limits: rg.GroupingPolicyLimits{MaxRules: 3, MaxCanonicalBytes: 1 << 16, MaxStringBytes: 4096, MaxAbsoluteNumerator: 1, MaxDenominator: 1, MaxCap: 1}})
	if err != nil {
		return fail()
	}
	if !live() {
		return fail()
	}
	frozen, err := pc.NewGroupingEvaluationCasePrivate(pc.GroupingEvaluationCaseRequestPrivate{CaseID: q.RunID, RunID: q.RunID, SelectorID: q.SeedNamespace, SelectorBytes: []byte(q.Input.replayBinding.URI), Input: q.Input.sourceInputOriginal, Policy: policy, Seed: q.Seed, ProjectionLimits: rg.CompositionLimits{MaxRawBytes: rg.EvaluationMaxBytesPrivate, MaxCanonicalBytes: rg.EvaluationMaxBytesPrivate, MaxOccurrences: rg.EvaluationMaxOccurrencesPrivate, MaxPairContributions: rg.EvaluationMaxOccurrencesPrivate, MaxPairs: rg.EvaluationMaxOccurrencesPrivate, MaxStringBytes: rg.EvaluationMaxStringPrivate, MaxDenominator: 1 << 30, MaxAccumulator: 1 << 50}})
	if err != nil {
		return fail()
	}
	if !live() {
		return fail()
	}
	result, err := pc.EvaluateGroupingCasePrivate(frozen)
	// The default clock is always real; the existing tagged test clock can
	// cancel a genuine reservation here, after evaluation but before Observe.
	now := sourceclock.Now(execution, "r-preparation-before-observe")
	if err != nil || !live() || !now.Before(q.Deadline) {
		return fail()
	}
	metadata := &preparedSourceRStateV1{q.Input, originals, policy, frozen, result, objects, originalTarget}
	if local {
		s.execution, s.cancel, s.metadata = execution, cancel, metadata
		keep = true
		return metadata, nil
	}
	h, err = o.k.observe(execution, o.job, s.head, q.Attempt, "OBSERVED", time.Now().UnixNano(), o.limits)
	if err != nil {
		o.uncertain = true
		return fail()
	}
	o.head = h.Identity
	if !live() || !time.Now().Before(q.Deadline) {
		o.uncertain = true
		return fail()
	}
	return metadata, nil
}
