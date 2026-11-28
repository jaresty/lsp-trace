package inventorysuccessor

import (
	"context"
	"crypto/sha256"
	"errors"
	"sync"
	"time"

	"lsp-trace/internal/programc"
	"lsp-trace/internal/publication"
	rg "lsp-trace/internal/referencegrouping"
)

var ErrPrivateTypedBundleOperationV2 = errors.New("private typed bundle operation refused")

// PrivateTypedBundleOwnerV2 connects only to existing canonical accounting custody.
// No constructor here initializes, renews, approves, or increases funding.
// All failures retain cumulative accounting; this is not a source/evidence owner.
// Uncertainty stops this connection, not all future explicit connections. A caller
// may close its root, select the current canonical head, and explicitly Open a new
// owner through unchanged kernel verification. That proves persisted state, never
// receipt acknowledgement, and does not recover old operations or report bytes.
type PrivateTypedBundleOwnerV2 struct {
	mu        sync.Mutex
	k         *kernelV2
	job       jobV2
	head      string
	limits    verificationLimitsV2
	uncertain bool
}

func OpenPrivateTypedBundleOwnerV2(ctx context.Context, root *publication.Root, canonicalJob []byte, expectedHead string, limits PrivateManagedInputVerificationV2) (*PrivateTypedBundleOwnerV2, error) {
	var j jobV2
	if ctx == nil || root == nil || !hashV2(expectedHead) || parseV2(canonicalJob, &j) != nil {
		return nil, ErrPrivateTypedBundleOperationV2
	}
	l := verificationLimitsV2{limits.Records, limits.Bytes, limits.Work}
	k := &kernelV2{root: root}
	h, err := k.reopen(ctx, j, l)
	if err != nil || h.Identity != expectedHead || !readyV2(h) {
		return nil, ErrPrivateTypedBundleOperationV2
	}
	return &PrivateTypedBundleOwnerV2{k: k, job: j, head: h.Identity, limits: l}, nil
}

// The expectation is independently constructed and opaque. Candidate is never
// imported to infer its own expected commitment or workload.
// Head and Attempt bind the requested accounting window; Deadline is absolute.
type PrivateTypedBundleBindingV2 struct {
	Expected      programc.GroupingBundleExpectationPrivate
	Candidate     []byte
	Attempt, Head string
	Deadline      time.Time
}
type PrivateTypedBundleOperationV2 struct {
	state *privateTypedBundleOperationV2
}
type privateTypedBundleOperationV2 struct {
	mu                       sync.Mutex
	owner                    *PrivateTypedBundleOwnerV2
	expected                 programc.GroupingBundleExpectationPrivate
	digest                   [32]byte
	length                   int
	attempt, priorHead, head string
	deadline                 time.Time
	used                     bool
}

// Closed logical credits, NOT measured allocations, process work, RSS, CPU or
// physical time. Preserve the original conservative 14-coordinate charge vector
// for this retirement: no refunds or successful-work discount. Import, execution,
// full replay, canonical copies and four kernel verifications remain charged.
// Historical transport (64MiB input/output, 64KiB stderr), observation-method
// (16384), and one-second cleanup allowances remain RESERVED LOGICAL CREDITS;
// no transport, process sampling, cleanup, or hard wall/RSS enforcement occurs.
// The deadline is an admission/release boundary, not an interruption guarantee.
// A future funding-policy change is separate from this execution retirement.
const privateTypedBundleWindowV2 = 60 * time.Second
const privateTypedBundleOutputCreditsV2 = 64 << 20
const privateTypedBundleStderrCreditsV2 = 64 << 10

func privateTypedBundleChargeV2(now, deadline time.Time) (countersV2, error) {
	if deadline.IsZero() || !now.Before(deadline) || deadline.Sub(now) > privateTypedBundleWindowV2 {
		return countersV2{}, ErrPrivateTypedBundleOperationV2
	}
	const b uint64 = programc.GroupingCaseBundleMaxBytesPrivate
	const slots uint64 = programc.GroupingCaseBundleMaxSlotsPrivate
	const n uint64 = rg.EvaluationMaxNodesPrivate
	const occ uint64 = rg.EvaluationMaxOccurrencesPrivate
	const result uint64 = rg.EvaluationMaxBytesPrivate
	const transport uint64 = uint64(programc.MaxWorkerInputBytes+privateTypedBundleOutputCreditsV2+privateTypedBundleStderrCreditsV2) + 3
	const verify uint64 = 4 * (64 << 20)
	const observationCalls uint64 = 16384
	// Every operand is a fixed admission cap or reserved logical allowance,
	// not a candidate-controlled integer.
	// Checked operations precede any candidate copy or process serialization.
	mul := func(a, b uint64) (uint64, error) {
		if b != 0 && a > ^uint64(0)/b {
			return 0, ErrPrivateTypedBundleOperationV2
		}
		return a * b, nil
	}
	sum := func(a, b uint64) (uint64, error) {
		if a > ^uint64(0)-b {
			return 0, ErrPrivateTypedBundleOperationV2
		}
		return a + b, nil
	}
	perSlot, err := mul(32, result)
	if err != nil {
		return countersV2{}, err
	}
	all, err := mul(slots, perSlot)
	if err != nil {
		return countersV2{}, err
	}
	passes, err := mul(64, b+transport)
	if err != nil {
		return countersV2{}, err
	}
	data, err := sum(all, passes)
	if err != nil {
		return countersV2{}, err
	}
	pair, err := mul(n+occ, n+occ)
	if err != nil {
		return countersV2{}, err
	}
	work, err := mul(4*slots, pair)
	if err != nil {
		return countersV2{}, err
	}
	work, err = sum(work, data+4*16384+observationCalls)
	if err != nil {
		return countersV2{}, err
	}
	records := uint64(4*4096) + slots*(16+128+8*n+16*occ)
	writes := uint64(16 * (recordLimitV2 + 2*headLimitV2))
	return countersV2{LogicalBytes: data, ReadHashBytes: verify + data, WriteBytes: writes, Records: records, ParserBytes: verify + data, ProviderRequests: 0, ProviderMessages: 0, Work: work, ActiveNS: uint64(deadline.Sub(now)) + uint64(time.Second), Calls: 4*16384 + records + 4*slots + observationCalls, RequestBytes: uint64(programc.MaxWorkerInputBytes) + 1, ResponseBytes: uint64(privateTypedBundleOutputCreditsV2+privateTypedBundleStderrCreditsV2) + 2, WireBytes: transport, OutputBytes: uint64(privateTypedBundleOutputCreditsV2)}, nil
}

func validPrivateTypedBindingV2(q PrivateTypedBundleBindingV2, now time.Time) bool {
	return len(q.Candidate) > 0 && len(q.Candidate) <= programc.GroupingCaseBundleMaxBytesPrivate && q.Expected != (programc.GroupingBundleExpectationPrivate{}) && hashV2(q.Attempt) && hashV2(q.Head) && !q.Deadline.IsZero() && now.Before(q.Deadline) && q.Deadline.Sub(now) <= privateTypedBundleWindowV2
}

// Reserve returns a capability ONLY after confirmed existing-kernel commit.
// Accounting files may exist on any error; they do not confer permission.
func (o *PrivateTypedBundleOwnerV2) Reserve(ctx context.Context, q PrivateTypedBundleBindingV2) (PrivateTypedBundleOperationV2, error) {
	fail := func() (PrivateTypedBundleOperationV2, error) {
		return PrivateTypedBundleOperationV2{}, ErrPrivateTypedBundleOperationV2
	}
	now := time.Now()
	if o == nil || ctx == nil || ctx.Err() != nil || !validPrivateTypedBindingV2(q, now) {
		return fail()
	}
	if d, ok := ctx.Deadline(); ok && d.Before(q.Deadline) {
		return fail()
	}
	charge, err := privateTypedBundleChargeV2(now, q.Deadline)
	if err != nil {
		return fail()
	}
	// The bound was checked before hashing. No unreserved heavy import occurs here.
	digest := sha256.Sum256(q.Candidate)
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.k == nil || o.uncertain || o.head != q.Head || !time.Now().Before(q.Deadline) {
		return fail()
	}
	reserved, cancel := context.WithDeadline(ctx, q.Deadline)
	defer cancel()
	h, err := o.k.reserve(reserved, o.job, q.Head, q.Attempt, charge, q.Deadline.UnixNano(), time.Now().UnixNano(), o.limits)
	if err != nil {
		if errors.Is(err, errUncertainV2) {
			o.uncertain = true
		}
		return fail()
	}
	if reserved.Err() != nil || !time.Now().Before(q.Deadline) {
		o.uncertain = true
		return fail()
	}
	o.head = h.Identity
	return PrivateTypedBundleOperationV2{&privateTypedBundleOperationV2{owner: o, expected: q.Expected, digest: digest, length: len(q.Candidate), attempt: q.Attempt, priorHead: q.Head, head: h.Identity, deadline: q.Deadline}}, nil
}

// Execute is deliberately a fixed operation, not a caller runner or completion
// protocol. The opaque handle is shared by copies and retired before any check.
func (o *PrivateTypedBundleOwnerV2) Execute(ctx context.Context, op PrivateTypedBundleOperationV2, q PrivateTypedBundleBindingV2) ([]byte, programc.ExecutionObservation, *programc.ExecutionFailure) {
	refuse := func(observation programc.ExecutionObservation, code programc.ExecutionCode) ([]byte, programc.ExecutionObservation, *programc.ExecutionFailure) {
		return nil, observation, &programc.ExecutionFailure{Code: code, Message: ErrPrivateTypedBundleOperationV2.Error()}
	}
	var observation programc.ExecutionObservation
	if op.state == nil {
		return refuse(observation, programc.CodeInternalAdmission)
	}
	s := op.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.used {
		return refuse(observation, programc.CodeInternalAdmission)
	}
	s.used = true
	if o == nil || ctx == nil || s.owner != o || ctx.Err() != nil || !validPrivateTypedBindingV2(q, time.Now()) || s.expected != q.Expected || s.length != len(q.Candidate) || s.attempt != q.Attempt || s.priorHead != q.Head || !s.deadline.Equal(q.Deadline) {
		return refuse(observation, programc.CodeInternalAdmission)
	}
	// Detach only after finite binding checks, under the confirmed reservation.
	candidate := append([]byte(nil), q.Candidate...)
	if sha256.Sum256(candidate) != s.digest {
		return refuse(observation, programc.CodeInternalAdmission)
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.k == nil || o.uncertain || o.head != s.head || !time.Now().Before(s.deadline) {
		return refuse(observation, programc.CodeInternalAdmission)
	}
	execution, cancel := context.WithDeadline(ctx, s.deadline)
	defer cancel()
	selected, err := o.k.reopen(execution, o.job, o.limits)
	if err != nil || selected.Identity != s.head || selected.Disposition != "PENDING" || selected.Attempt != s.attempt {
		o.uncertain = true
		return refuse(observation, programc.CodeInternalAdmission)
	}
	report, observed, failed := programc.ExecuteGroupingCaseBundlePrivate(execution, s.expected, candidate)
	observation = observed
	if failed != nil {
		o.uncertain = true
		return nil, observation, failed
	}
	if execution.Err() != nil || !time.Now().Before(s.deadline) {
		o.uncertain = true
		return refuse(observation, programc.CodeTimeout)
	}
	h, err := o.k.observe(execution, o.job, s.head, s.attempt, "OBSERVED", time.Now().UnixNano(), o.limits)
	if err != nil || execution.Err() != nil || !time.Now().Before(s.deadline) {
		o.uncertain = true
		return refuse(observation, programc.CodeInternalAdmission)
	}
	o.head = h.Identity
	return report, observation, nil
}
