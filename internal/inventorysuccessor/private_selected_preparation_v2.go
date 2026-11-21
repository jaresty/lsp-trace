package inventorysuccessor

// This private additive path retains original process-local producers. Quotes
// are checked expenditure requests, never grants or restoration capabilities.
import (
	"context"
	"errors"
	"reflect"
	"slices"
	"sync"
	"time"

	cc "lsp-trace/internal/censuscontinuation"
	pc "lsp-trace/internal/programc"
	pa "lsp-trace/internal/programcadmission"
	rg "lsp-trace/internal/referencegrouping"
	"lsp-trace/internal/sourceobject"
	tp "lsp-trace/internal/targetpacket"
	"lsp-trace/sessionruntime"
)

var ErrPrivateSelectedV2 = errors.New("private selected preparation refused")
var ErrPrivateSelectedBoundsV2 = errors.New("private selected logical bounds refused")

// Configuration is data only. There is one actual selected producer, not seven
// nominations. Schedule order is fixed; missing entries remain missing.
type PrivateSelectedConfigurationV2 struct {
	RunID, SeedNamespace, Lineage string
	Mode                          string // derived from actual selected kinds by the opaque plan constructor
	Seed                          uint64
	Attempts                      [3]string
	Contract                      cc.ContinuationContract
	// Bounded metadata attestation only; must match Contract.ID(). Never native authority.
	ContractInput cc.ContractInput
	SourceLimits  tp.PrivateSourceLimits
	// Optional binding data, not authority. The admitted job is checked under
	// the owner lock before any selected reservation reaches the kernel.
	ExpectedSourceAuthorization string `json:",omitempty"`
}

var selectedModesV2 = [7]string{"RD", "C", "CRD", "D", "CD", "R", "CR"}
var selectedKindsV2 = [3]string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"}

func selectedModeV2(kinds []pa.RelationKind) (string, bool) {
	if len(kinds) == 0 || len(kinds) > 3 {
		return "", false
	}
	var bits uint8
	for _, k := range kinds {
		var b uint8
		switch k {
		case pa.TypedCalls:
			b = 1
		case pa.TypedReferencesSymbol:
			b = 2
		case pa.TypedResolvesToDefinition:
			b = 4
		default:
			return "", false
		}
		if bits&b != 0 {
			return "", false
		}
		bits |= b
	}
	return [8]string{"", "C", "R", "CR", "D", "CD", "RD", "CRD"}[bits], true
}

func selectedPolicyV2(mode string) (rg.GroupingPolicy, error) {
	rules := make([]rg.GroupingPolicyRule, 0, 3)
	for i, k := range selectedKindsV2 {
		letter := [3]string{"C", "R", "D"}[i]
		include := false
		for _, v := range mode {
			if string(v) == letter {
				include = true
			}
		}
		r := rg.GroupingPolicyRule{RelationKind: k, Action: "EXCLUDE", RationaleCode: "PRIVATE_SELECTED_KINDS_V2", Weight: rg.Rational{Numerator: 0, Denominator: 1}}
		if include {
			r.Action = "INCLUDE"
			r.Weight.Numerator = 1
		}
		rules = append(rules, r)
	}
	valid := false
	for _, m := range selectedModesV2 {
		valid = valid || mode == m
	}
	if !valid {
		return rg.GroupingPolicy{}, ErrPrivateSelectedV2
	}
	return rg.BuildGroupingPolicy(rg.GroupingPolicyRequest{PolicyID: "PRIVATE_SELECTED_" + mode + "_V2", Epoch: 1, EligibleKinds: slices.Clone(selectedKindsV2[:]), Rules: rules, Limits: rg.GroupingPolicyLimits{MaxRules: 3, MaxCanonicalBytes: 1 << 16, MaxStringBytes: 4096, MaxAbsoluteNumerator: 1, MaxDenominator: 1, MaxCap: 1}})
}

func selectedAddV2(v ...uint64) (uint64, error) {
	var n uint64
	for _, x := range v {
		if x > ^uint64(0)-n {
			return 0, ErrPrivateSelectedBoundsV2
		}
		n += x
	}
	return n, nil
}
func selectedMulV2(a, b uint64) (uint64, error) {
	if b != 0 && a > ^uint64(0)/b {
		return 0, ErrPrivateSelectedBoundsV2
	}
	return a * b, nil
}

// Source quote: H+3Lower+2Upper+Originals+Symbols+Methods+Requests+
// Selection+Store+Native+Cleanup. Byte estimates include REJECTED serialization,
// not merely successful L/T material. This is logical currency, not CPU/heap/IO.
// One original, one source object, NO additional definition proofs are permitted.
// Upper's pre-reconstruction work is I+N+2+33U<=W<=10000. Six missing slots
// give I>=U+6 and one READY gives N>=1, hence U<=293. U<=4O also follows
// from two endpoints plus at most two custody rows per contribution.
// Lower pre-copy estimate: private_allmode_continuation.go:119-138.
// Upper pre-serialization estimates: sourcecontinuation/continuation.go:205-250,
// 352-359. Three lower/four material and two upper/three material preparations
// follow from Run and whole Resume, not an assumed prepared-Describe allowance.
// Symbol selector: <=100000 tokens, <=10000 symbols, depth<=64/JSON512;
// mostSpecific compares <=2(n-1). Method parsers admit <=256 items.
// Request formatting/canonical validation costs <=20 identity marshals and five
// prompt hashes per unit per upper. Successful material hashes <=10L+8T;
// MemoryStore.Put has TWO hashes, separately charged; Get/Put clone L+T each.
// Native: four verifier waves (reserve/execution/Observe/final readback), five
// publication waves (reservation/head/CAS + Observe head/CAS), one failure/
// cleanup wave. CAS physical attempts and contention time are NOT bounded here.
func selectedSourceChargeV2(o uint64, artifact, parent uint64, now, deadline time.Time) (countersV2, error) {
	if o == 0 || o > 1024 || artifact == 0 || artifact > 64<<20 || parent == 0 || parent > 64<<20 || !now.Before(deadline) || deadline.Sub(now) > 15*time.Second {
		return countersV2{}, ErrPrivateSelectedBoundsV2
	}
	const e uint64 = 8 << 20
	const q uint64 = 1500000
	const d uint64 = 1 << 20
	const f uint64 = 4096
	a := min(artifact, e)
	l := min(4*a, parent)
	t := min(3*a, parent)
	u := min(uint64(293), 4*o)
	c := 2 * o
	// Finite guards bound inner constant-size expressions; term products and
	// their cumulative sum are overflow-checked.
	terms := [][2]uint64{
		{8, 64<<10 + 7*6*(4*256+2048)}, // H
		{3, 16384 + (64<<10 + 14*6*f) + 12*f + 24*e + 6*(c+4*o+2)*(5*f+2048) + 4*l},
		{2, parent + 16384 + 1000*2048 + u*(6*d+32768) + u*(6*e+32768) + 4*t},
		{2 * 64, 8 << 20}, // both prepared original reconstructions
		{2 + 4*u, 16 * (q + 64<<10)},
		{2 * u, 16 * (q + 256*(6*f+256))},
		{2 * u, 25 * e},
		{3 * 1000, 16 * f}, // selection, admission/request joins, lower material decode
		{2, l + t},         // store copies and Get copies
	}
	var b uint64
	for _, term := range terms {
		n, err := selectedMulV2(term[0], term[1])
		if err != nil {
			return countersV2{}, err
		}
		b, err = selectedAddV2(b, n)
		if err != nil {
			return countersV2{}, err
		}
	}
	records := uint64(4*4096+3*10000+2*10000+14) + 40*u
	hashes, err := selectedAddV2(b, 10*l, 8*t, 2*(l+t), 4*(64<<20))
	if err != nil {
		return countersV2{}, err
	}
	work, err := selectedAddV2(b, 4*16384, (2+4*u)*130000, 2*u*o, records)
	if err != nil {
		return countersV2{}, err
	}
	return countersV2{LogicalBytes: b, ReadHashBytes: hashes, WriteBytes: l + t + 6*(recordLimitV2+2*headLimitV2), Records: records, ParserBytes: b + 4*(64<<20), Work: work, ActiveNS: uint64(deadline.Sub(now)), Calls: 4*16384 + records, OutputBytes: l + t + 4*e}, nil
}

// P is the existing kind-neutral evaluator algorithm: selected three-rule policy,
// NewCase projection, one numerical evaluation. The R-only binding guard is NOT
// reused. Its arithmetic is exactly the already-established logical evaluator
// allowance, with actual bounded metadata, not a grant borrowed from an R job.
func selectedPreparationChargeV2(m rg.AllModeMetadataPrivate, now, deadline time.Time) (countersV2, error) {
	if m.Input.ByteLength <= 0 || m.Input.ByteLength > 1<<20 || m.Contributions <= 0 || m.Contributions > 1024 || m.Custody < 0 || m.Custody > 2*m.Contributions || m.Predecessors < 0 || m.Predecessors > 1024 {
		return countersV2{}, ErrPrivateSelectedBoundsV2
	}
	return sourceRPreparationChargeV1(m, now, deadline)
}

// The dense logical kernel term is valid only after this scalar guard. In
// particular, unrelated isolated CALLS nodes are never silently dropped.
func selectedProjectionWithinQuoteV2(nodes, occurrences, custody int) bool {
	return occurrences > 0 && occurrences <= 1024 && nodes >= 0 && nodes <= 2*occurrences && custody >= 0 && custody <= 2*occurrences
}

type PrivateSelectedPreparationPlanV2 struct{ state *selectedPlanV2 }
type selectedPlanV2 struct {
	mu                 sync.Mutex
	owner              *PrivateManagedInputOwnerV2
	request            PrivateMixedInputRequestV2
	config             PrivateSelectedConfigurationV2
	mode               string
	envelope           sessionruntime.PrivateProviderWindowEnvelope
	stages             [3]countersV2
	expectedHead       string
	reserved, executed bool
	operation          PrivateMixedInputOperationV2
}

// Fixed ContractInput metadata only: <=128 scalar/struct/array/slice values,
// depth<=8, <=16 slice/array entries, each string<=256 and aggregate<=1024.
// No caller hooks, maps, pointers, interfaces or functions are accepted.
// Checks precede both allocations and canonical serialization. The clone prevents
// a later mutation of a caller-owned metadata slice from changing the contract.
func selectedContractValueV2(dst, src reflect.Value, budget *[2]int, depth int) bool {
	if depth > 8 || budget[0] <= 0 {
		return false
	}
	budget[0]--
	switch src.Kind() {
	case reflect.String:
		if src.Len() > 256 || src.Len() > budget[1] {
			return false
		}
		budget[1] -= src.Len()
		dst.SetString(src.String())
		return true
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		dst.Set(src)
		return true
	case reflect.Struct:
		if src.NumField() > 32 {
			return false
		}
		for i := 0; i < src.NumField(); i++ {
			field := src.Type().Field(i)
			if len(field.Name) > 64 || len(field.Tag.Get("json")) > 64 || !dst.Field(i).CanSet() || !selectedContractValueV2(dst.Field(i), src.Field(i), budget, depth+1) {
				return false
			}
		}
		return true
	case reflect.Array, reflect.Slice:
		if src.Len() > 16 || src.Len() > budget[0] {
			return false
		}
		if src.Kind() == reflect.Slice && !src.IsNil() {
			dst.Set(reflect.MakeSlice(src.Type(), src.Len(), src.Len()))
		}
		for i := 0; i < src.Len(); i++ {
			if !selectedContractValueV2(dst.Index(i), src.Index(i), budget, depth+1) {
				return false
			}
		}
		return true
	default:
		return false
	}
}
func selectedCheckedContractV2(c PrivateSelectedConfigurationV2) (PrivateSelectedConfigurationV2, error) {
	var detached cc.ContractInput
	budget := [2]int{128, 1024}
	if !selectedContractValueV2(reflect.ValueOf(&detached).Elem(), reflect.ValueOf(c.ContractInput), &budget, 0) {
		return PrivateSelectedConfigurationV2{}, ErrPrivateSelectedBoundsV2
	}
	checked, err := cc.NewContinuationContract(detached)
	if err != nil || checked.ID() != c.Contract.ID() {
		return PrivateSelectedConfigurationV2{}, ErrPrivateSelectedV2
	}
	c.Contract = checked
	c.ContractInput = detached
	return c, nil
}

func selectedConfigValidV2(c PrivateSelectedConfigurationV2) bool {
	if c.ExpectedSourceAuthorization != "" && !hashV2(c.ExpectedSourceAuthorization) {
		return false
	}
	for _, s := range []string{c.RunID, c.SeedNamespace, c.Lineage} {
		if !sessionFieldRPreparationV1(s) || len(s) > 256 {
			return false
		}
	}
	for i, a := range c.Attempts {
		if !hashV2(a) {
			return false
		}
		for j := 0; j < i; j++ {
			if a == c.Attempts[j] {
				return false
			}
		}
	}
	l := c.Contract.CaptureLimits()
	s := c.SourceLimits
	return s.Valid() && l.MaxArtifactBytes > 0 && l.MaxArtifactBytes <= 64<<20 && l.MaxParentBytes > 0 && l.MaxParentBytes <= 64<<20 && l.MaxSourceBytes > 0 && l.MaxSourceBytes <= 1<<20 && l.MaxTotalSourceBytes > 0 && l.MaxTotalSourceBytes <= 8<<20 && l.MaxBindings > 0 && l.MaxBindings <= 1000 && l.MaxWork > 0 && l.MaxWork <= 10000 && s.MaxDocumentBytes <= l.MaxSourceBytes && s.MaxTotalBytes <= l.MaxTotalSourceBytes && s.MaxObjects <= l.MaxBindings && s.MaxWork <= l.MaxWork
}

// Constructor reads bounded Manager envelope metadata, not source or grants.
// Existing admission/bootstrap is caller funded. No extra canonical reopen.
func NewPrivateSelectedPreparationPlanV2(o *PrivateManagedInputOwnerV2, q PrivateMixedInputRequestV2, c PrivateSelectedConfigurationV2) (PrivateSelectedPreparationPlanV2, error) {
	plan, ok := privateMixedPlanV2(q)
	mode, mok := selectedModeV2(q.Kinds)
	if c.Mode != "" && c.Mode != mode {
		return PrivateSelectedPreparationPlanV2{}, ErrPrivateSelectedV2
	}
	c.Mode = mode
	if o == nil || (q.Manager == nil && q.retained == nil) || !ok || !mok || q.Timing != nil || !selectedConfigValidV2(c) {
		return PrivateSelectedPreparationPlanV2{}, ErrPrivateSelectedV2
	}
	var contractErr error
	c, contractErr = selectedCheckedContractV2(c)
	if contractErr != nil {
		return PrivateSelectedPreparationPlanV2{}, contractErr
	}
	// Bound borrowed array headers before cloning; strings are immutable.
	for _, a := range [][]string{q.Start.Process.Args, q.Start.Process.Env, q.Start.RelationProviders} {
		if len(a) > 128 {
			return PrivateSelectedPreparationPlanV2{}, ErrPrivateSelectedBoundsV2
		}
		for _, s := range a {
			if len(s) > 4096 {
				return PrivateSelectedPreparationPlanV2{}, ErrPrivateSelectedBoundsV2
			}
		}
	}
	var envelope sessionruntime.PrivateProviderWindowEnvelope
	var err error
	if q.retained == nil {
		envelope, err = q.Manager.PrivateProviderWindowEnvelope(q.Start, plan)
	}
	if err != nil {
		return PrivateSelectedPreparationPlanV2{}, err
	}
	now := time.Now()
	m, err := privateSelectedMixedQuoteV1(q, now, envelope)
	if err != nil {
		return PrivateSelectedPreparationPlanV2{}, err
	}
	p, err := selectedPreparationChargeV2(rg.AllModeMetadataPrivate{Input: rg.CompositionCommitment{ByteLength: 1 << 20}, Contributions: 1024, Custody: 2048, Predecessors: 1024}, now, q.Start.Deadline)
	if err != nil {
		return PrivateSelectedPreparationPlanV2{}, err
	}
	limits := c.Contract.CaptureLimits()
	s, err := selectedSourceChargeV2(1024, uint64(limits.MaxArtifactBytes), uint64(limits.MaxParentBytes), now, q.Start.Deadline)
	if err != nil {
		return PrivateSelectedPreparationPlanV2{}, err
	}
	q.Kinds = slices.Clone(q.Kinds)
	q.Start.Process.Args = slices.Clone(q.Start.Process.Args)
	q.Start.Process.Env = slices.Clone(q.Start.Process.Env)
	q.Start.RelationProviders = slices.Clone(q.Start.RelationProviders)
	return PrivateSelectedPreparationPlanV2{&selectedPlanV2{owner: o, request: q, config: c, mode: mode, envelope: envelope, stages: [3]countersV2{m, p, s}, expectedHead: o.PreparationHeadV1()}}, nil
}

type PrivateSelectedCapturedV2 struct{ state *selectedCapturedV2 }
type selectedCapturedV2 struct {
	plan      *selectedPlanV2
	input     PrivateManagedGroupingInput
	originals PrivateProducerOriginals
	head      string
	reserved  bool
}

// Fixed producer continuation: no public Input can be substituted after capture.
func selectedPlanLiveV2(p *selectedPlanV2) bool {
	return p != nil && p.operation.state != nil && p.operation.state.retainedContext != nil && p.operation.state.retainedContext.Err() == nil && time.Now().Before(p.request.Start.Deadline)
}

func ExecuteSelectedMixedInputV2(ctx context.Context, op PrivateMixedInputOperationV2, q PrivateMixedInputRequestV2, plan PrivateSelectedPreparationPlanV2) (PrivateSelectedCapturedV2, error) {
	s := plan.state
	if s == nil {
		return PrivateSelectedCapturedV2{}, ErrPrivateSelectedV2
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.executed {
		return PrivateSelectedCapturedV2{}, ErrPrivateSelectedV2
	}
	s.executed = true
	if ctx == nil || ctx.Err() != nil || !s.reserved || op.state == nil || op.state != s.operation.state || !selectedPlanLiveV2(s) || !reflect.DeepEqual(q, s.request) {
		return PrivateSelectedCapturedV2{}, ErrPrivateSelectedV2
	}
	execution, cancel := context.WithDeadline(op.state.retainedContext, q.Start.Deadline)
	defer cancel()
	stop := context.AfterFunc(ctx, cancel)
	defer stop()
	input, err := ExecuteMixedInputV2(execution, op, q)
	if err != nil {
		return PrivateSelectedCapturedV2{}, err
	}
	originals, err := input.SourceOriginalsV1()
	if err != nil {
		return PrivateSelectedCapturedV2{}, err
	}
	if execution.Err() != nil || ctx.Err() != nil || !selectedPlanLiveV2(s) {
		return PrivateSelectedCapturedV2{}, ErrPrivateSelectedV2
	}
	return PrivateSelectedCapturedV2{&selectedCapturedV2{s, input, originals, s.owner.PreparationHeadV1(), false}}, nil
}

type PrivateSelectedPreparationOperationV2 struct {
	state *selectedPreparationOperationV2
}
type selectedPreparationOperationV2 struct {
	mu       sync.Mutex
	used     bool
	captured *selectedCapturedV2
	head     string
	ctx      context.Context
}
type PreparedSelectedV2 struct{ state *preparedSelectedV2 }
type preparedSelectedV2 struct {
	mu             sync.Mutex
	sourceReserved bool
	plan           *selectedPlanV2
	input          rg.AdmittedInput
	policy         rg.GroupingPolicy
	frozen         pc.GroupingEvaluationCasePrivate
	result         pc.GroupingEvaluationResultPrivate
	originals      PrivateProducerOriginals
	objects        []sourceobject.Object
	head           string
	scope          context.Context
}

func (o *PrivateManagedInputOwnerV2) ReserveSelectedPreparationV2(ctx context.Context, c PrivateSelectedCapturedV2) (PrivateSelectedPreparationOperationV2, error) {
	if c.state == nil {
		return PrivateSelectedPreparationOperationV2{}, ErrPrivateSelectedV2
	}
	s := c.state
	p := s.plan
	p.mu.Lock()
	defer p.mu.Unlock()
	if s.reserved {
		return PrivateSelectedPreparationOperationV2{}, ErrPrivateSelectedV2
	}
	s.reserved = true
	if o == nil || o != p.owner || ctx == nil || ctx.Err() != nil || !selectedPlanLiveV2(p) {
		return PrivateSelectedPreparationOperationV2{}, ErrPrivateSelectedV2
	}
	m, err := rg.ReadAllModeMetadataPrivate(s.input.sourceInputOriginal)
	if err != nil {
		return PrivateSelectedPreparationOperationV2{}, err
	}
	now := time.Now()
	charge, err := selectedPreparationChargeV2(m, now, p.request.Start.Deadline)
	if err != nil || !charge.within(p.stages[1]) {
		return PrivateSelectedPreparationOperationV2{}, ErrPrivateSelectedBoundsV2
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.k == nil || o.uncertain || o.head != s.head || o.k.root != p.request.Root {
		return PrivateSelectedPreparationOperationV2{}, ErrPrivateSelectedV2
	}
	h, err := o.k.reserve(ctx, o.job, s.head, p.config.Attempts[1], charge, p.request.Start.Deadline.UnixNano(), now.UnixNano(), o.limits)
	if err != nil {
		if errors.Is(err, errUncertainV2) {
			o.uncertain = true
		}
		return PrivateSelectedPreparationOperationV2{}, err
	}
	o.head = h.Identity
	return PrivateSelectedPreparationOperationV2{&selectedPreparationOperationV2{captured: s, head: h.Identity, ctx: ctx}}, nil
}

func ExecuteSelectedPreparationV2(ctx context.Context, op PrivateSelectedPreparationOperationV2) (PreparedSelectedV2, error) {
	if op.state == nil {
		return PreparedSelectedV2{}, ErrPrivateSelectedV2
	}
	s := op.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.used {
		return PreparedSelectedV2{}, ErrPrivateSelectedV2
	}
	s.used = true
	p := s.captured.plan
	o := p.owner
	deadline := p.request.Start.Deadline
	if ctx == nil || ctx.Err() != nil || s.ctx.Err() != nil || !selectedPlanLiveV2(p) || !time.Now().Before(deadline) {
		return PreparedSelectedV2{}, ErrPrivateSelectedV2
	}
	execution, cancel := context.WithDeadline(s.ctx, deadline)
	defer cancel()
	stop := context.AfterFunc(ctx, cancel)
	defer stop()
	stopOriginal := context.AfterFunc(p.operation.state.retainedContext, cancel)
	defer stopOriginal()
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.uncertain || o.head != s.head || o.k.root != p.request.Root {
		return PreparedSelectedV2{}, ErrPrivateSelectedV2
	}
	h, err := o.k.reopen(execution, o.job, o.limits)
	if err != nil || h.Identity != s.head {
		return PreparedSelectedV2{}, ErrPrivateSelectedV2
	}
	input := s.captured.input
	view, err := input.sourceEvidence.Snapshot(sessionruntime.PrivateGroupingQueryMaxBytes)
	if err != nil || len(view.Source) == 0 || len(view.Source) > p.config.SourceLimits.MaxDocumentBytes || len(view.Source) > 1<<20 {
		return PreparedSelectedV2{}, ErrPrivateSelectedBoundsV2
	}
	policy, err := selectedPolicyV2(p.mode)
	if err != nil {
		return PreparedSelectedV2{}, err
	}
	frozen, err := pc.NewGroupingEvaluationCasePrivate(pc.GroupingEvaluationCaseRequestPrivate{CaseID: p.config.RunID, RunID: p.config.RunID, SelectorID: p.config.SeedNamespace, SelectorBytes: []byte(p.request.URI), Input: input.sourceInputOriginal, Policy: policy, Seed: p.config.Seed, ProjectionLimits: rg.CompositionLimits{MaxRawBytes: rg.EvaluationMaxBytesPrivate, MaxCanonicalBytes: rg.EvaluationMaxBytesPrivate, MaxOccurrences: 1024, MaxPairContributions: 1024, MaxPairs: 1024, MaxStringBytes: rg.EvaluationMaxStringPrivate, MaxDenominator: 1 << 30, MaxAccumulator: 1 << 50}})
	if err != nil {
		return PreparedSelectedV2{}, err
	}
	nodes, occurrences, custody, countError := rg.ProjectionCountsPrivate(frozen.Projection())
	if countError != nil || !selectedProjectionWithinQuoteV2(nodes, occurrences, custody) {
		return PreparedSelectedV2{}, ErrPrivateSelectedBoundsV2
	}
	result, err := pc.EvaluateGroupingCasePrivate(frozen)
	if err != nil {
		return PreparedSelectedV2{}, err
	}
	if execution.Err() != nil || ctx.Err() != nil || !selectedPlanLiveV2(p) || !time.Now().Before(deadline) {
		return PreparedSelectedV2{}, ErrPrivateSelectedV2
	}
	h, err = o.k.observe(execution, o.job, s.head, p.config.Attempts[1], "OBSERVED", time.Now().UnixNano(), o.limits)
	if err != nil {
		o.uncertain = true
		return PreparedSelectedV2{}, err
	}
	o.head = h.Identity
	if execution.Err() != nil || ctx.Err() != nil || !selectedPlanLiveV2(p) || !time.Now().Before(deadline) {
		o.uncertain = true
		return PreparedSelectedV2{}, ErrPrivateSelectedV2
	}
	objects := []sourceobject.Object{{Identity: sourceobject.Identity{Digest: view.Query.SourceDigest, ByteLength: uint64(len(view.Source))}, Bytes: view.Source}}
	return PreparedSelectedV2{&preparedSelectedV2{plan: p, input: input.sourceInputOriginal, policy: policy, frozen: frozen, result: result, originals: s.captured.originals, objects: objects, head: h.Identity, scope: s.ctx}}, nil
}

func selectedPreparedLiveV2(s *preparedSelectedV2) bool {
	return s != nil && s.scope != nil && s.scope.Err() == nil && selectedPlanLiveV2(s.plan)
}

type PrivateSelectedSourceOperationV2 struct{ state *selectedSourceOperationV2 }

// CompletedSelectedAccountingV1 is data-only provenance, never funding/admission.
// The fourteen counters retain the established currency order documented below.
type CompletedSelectedAccountingV1 struct {
	Head, Job, Authorization, Disposition, Event                              string
	Attempts                                                                  [3]string
	Windows                                                                   uint64
	Charged                                                                   [14]uint64 // logical, read/hash, write, records, parser, provider requests/messages, work, active ns, calls, request/response/wire/output bytes
	URI, Revision, Privacy, OccurrenceID, OccurrencePolicy, CompositionPolicy string
	Line, Character                                                           uint32
	NativeDeadline                                                            int64
}

type selectedSourceOperationV2 struct {
	mu                    sync.Mutex
	completed             CompletedSelectedAccountingV1
	used, begun, finished bool
	prepared              *preparedSelectedV2
	head                  string
	ctx                   context.Context
}

func (o *PrivateManagedInputOwnerV2) ReserveSelectedSourceV2(ctx context.Context, prepared PreparedSelectedV2) (PrivateSelectedSourceOperationV2, error) {
	if prepared.state == nil {
		return PrivateSelectedSourceOperationV2{}, ErrPrivateSelectedV2
	}
	s := prepared.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sourceReserved {
		return PrivateSelectedSourceOperationV2{}, ErrPrivateSelectedV2
	}
	s.sourceReserved = true
	p := s.plan
	if o == nil || o != p.owner || ctx == nil || ctx.Err() != nil || !selectedPreparedLiveV2(s) {
		return PrivateSelectedSourceOperationV2{}, ErrPrivateSelectedV2
	}
	binding, err := pc.BindGroupingPairPrivate(s.frozen, s.result, s.input)
	if err != nil {
		return PrivateSelectedSourceOperationV2{}, err
	}
	l := p.config.Contract.CaptureLimits()
	now := time.Now()
	charge, err := selectedSourceChargeV2(uint64(binding.Occurrences), uint64(l.MaxArtifactBytes), uint64(l.MaxParentBytes), now, p.request.Start.Deadline)
	if err != nil || !charge.within(p.stages[2]) {
		return PrivateSelectedSourceOperationV2{}, ErrPrivateSelectedBoundsV2
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.k == nil || o.uncertain || o.head != s.head || o.k.root != p.request.Root {
		return PrivateSelectedSourceOperationV2{}, ErrPrivateSelectedV2
	}
	h, err := o.k.reserve(ctx, o.job, s.head, p.config.Attempts[2], charge, p.request.Start.Deadline.UnixNano(), now.UnixNano(), o.limits)
	if err != nil {
		if errors.Is(err, errUncertainV2) {
			o.uncertain = true
		}
		return PrivateSelectedSourceOperationV2{}, err
	}
	o.head = h.Identity
	return PrivateSelectedSourceOperationV2{&selectedSourceOperationV2{prepared: s, head: h.Identity, ctx: ctx}}, nil
}

// Begin consumes before all failure checks. Return values are the exact originals
// held by paid P; metadata equality alone cannot construct this operation.
func (op PrivateSelectedSourceOperationV2) Begin(ctx context.Context) (cc.PrivateAllModeHandoff, PrivateSelectedConfigurationV2, PrivateProducerOriginals, []sourceobject.Object, context.Context, error) {
	bad := func() (cc.PrivateAllModeHandoff, PrivateSelectedConfigurationV2, PrivateProducerOriginals, []sourceobject.Object, context.Context, error) {
		return cc.PrivateAllModeHandoff{}, PrivateSelectedConfigurationV2{}, PrivateProducerOriginals{}, nil, nil, ErrPrivateSelectedV2
	}
	if op.state == nil {
		return bad()
	}
	s := op.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.used {
		return bad()
	}
	s.used = true
	p := s.prepared.plan
	o := p.owner
	if ctx == nil || ctx.Err() != nil || s.ctx.Err() != nil || !selectedPreparedLiveV2(s.prepared) || !time.Now().Before(p.request.Start.Deadline) {
		return bad()
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.uncertain || o.head != s.head {
		return bad()
	}
	h, err := o.k.reopen(ctx, o.job, o.limits)
	if err != nil || h.Identity != s.head {
		return bad()
	}
	slots := make([]cc.PrivateAllModeSlot, 7)
	entries := make([]cc.PrivateAllModeEntry, 7)
	for i, mode := range selectedModesV2 {
		run := p.config.RunID + "/" + mode
		if mode == p.mode {
			run = p.config.RunID
		}
		slots[i] = cc.PrivateAllModeSlot{Ordinal: i, Mode: mode, RunID: run, SeedNamespace: p.config.SeedNamespace, Seed: p.config.Seed}
		entries[i] = cc.PrivateAllModeEntry{Slot: slots[i], Disposition: cc.PrivateAllModeDeclaredMissing, Lineage: p.config.Lineage, MissingReason: "NOT_SELECTED_FOR_THIS_FRESH_ATTEMPT;UNRESOLVED"}
		if mode == p.mode {
			entries[i] = cc.PrivateAllModeEntry{Slot: slots[i], Disposition: cc.PrivateAllModeReady, Lineage: p.config.Lineage, Input: s.prepared.input, Policy: s.prepared.policy, Case: s.prepared.frozen, Result: s.prepared.result}
		}
	}
	handoff, err := cc.NewPrivateAllModeHandoff(p.request.Start.Profile.Workspace().String(), p.request.Revision, slots, entries)
	if err != nil {
		return bad()
	}
	objects := make([]sourceobject.Object, len(s.prepared.objects))
	for i, obj := range s.prepared.objects {
		objects[i] = sourceobject.Object{Identity: obj.Identity, Bytes: slices.Clone(obj.Bytes)}
	}
	s.begun = true
	return handoff, p.config, s.prepared.originals, objects, s.ctx, nil
}

// Finish cannot manufacture a source result. The concrete sourcecontinuation
// adapter is the only constructor of that opaque result after Run/whole Resume.
// Error paths leave accounting pending; no retries, refunds, resets or renewal.
func (op PrivateSelectedSourceOperationV2) Finish(ctx context.Context, workError error) error {
	if op.state == nil {
		return ErrPrivateSelectedV2
	}
	s := op.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.used || !s.begun || s.finished {
		return ErrPrivateSelectedV2
	}
	s.finished = true
	p := s.prepared.plan
	o := p.owner
	if workError != nil {
		return workError
	}
	if ctx == nil || ctx.Err() != nil || s.ctx.Err() != nil || !selectedPreparedLiveV2(s.prepared) || !time.Now().Before(p.request.Start.Deadline) {
		return ErrPrivateSelectedV2
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.uncertain || o.head != s.head {
		return ErrPrivateSelectedV2
	}
	h, err := o.k.observe(ctx, o.job, s.head, p.config.Attempts[2], "OBSERVED", time.Now().UnixNano(), o.limits)
	if err != nil {
		o.uncertain = true
		return err
	}
	o.head = h.Identity
	checked, err := o.k.reopen(ctx, o.job, o.limits)
	if err != nil || checked.Identity != h.Identity || ctx.Err() != nil || s.ctx.Err() != nil || !selectedPreparedLiveV2(s.prepared) || !time.Now().Before(p.request.Start.Deadline) {
		o.uncertain = true
		return ErrPrivateSelectedV2
	}
	c := checked.Charged
	q := p.request
	s.completed = CompletedSelectedAccountingV1{Head: checked.Identity, Job: checked.Job.Identity, Authorization: checked.Authorization, Disposition: checked.Disposition, Event: checked.Event, Attempts: p.config.Attempts, Windows: checked.Windows, Charged: [14]uint64{c.LogicalBytes, c.ReadHashBytes, c.WriteBytes, c.Records, c.ParserBytes, c.ProviderRequests, c.ProviderMessages, c.Work, c.ActiveNS, c.Calls, c.RequestBytes, c.ResponseBytes, c.WireBytes, c.OutputBytes}, URI: q.URI, Revision: q.Revision, Privacy: q.Privacy, OccurrenceID: q.OccurrenceID, OccurrencePolicy: q.OccurrencePolicy, CompositionPolicy: q.CompositionPolicy, Line: q.Line, Character: q.Character, NativeDeadline: q.Start.Deadline.UnixNano()}
	return nil
}

// CompletedAccountingV1 copies only the successful Finish/readback observation.
// It neither reopens native state nor validates caller-supplied records.
func (op PrivateSelectedSourceOperationV2) CompletedAccountingV1() (CompletedSelectedAccountingV1, bool) {
	if op.state == nil {
		return CompletedSelectedAccountingV1{}, false
	}
	s := op.state
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.completed, s.completed.Head != ""
}
