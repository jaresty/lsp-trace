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
// imported in this process to infer its own expected commitment or workload.
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

// Closed logical units, NOT physical RSS, CPU instructions, disk bytes or money.
// B=8MiB whole candidate; S=128 slots; N=8192 endpoints; O=4096 occurrences;
// R=8MiB canonical per-case result; T=64MiB parent/child frame; E=64KiB stderr.
// Import reconstructs each slot once. Execute calls Eval + CheckEvaluation (2
// kernels/slot), CheckReport repeats the entire batch (2 more). Admission,
// projection, judgments, case/result hashing, canonical copies, response decode
// and four kernel verifications (Open, Reserve, pre-execution, Observe) are
// charged independently of successful output. No successful-work discount exists.
// The 32 per-slot R passes allocate 8 to import/reconstruction, 4 to frozen
// declaration checks, 8 to four evaluator/result passes, and 12 to scoring,
// aggregates and exact full-report replay/copies. The 64*(B+T) envelope covers
// raw admission/hash, bounded decoding/base64/encoding/copies in both processes,
// response canonical/hash verification, and rejecting transport bytes. These are
// closed logical allowances, not measured allocations. Four verifications each
// admit <=4096 records, <=64MiB and <=16384 verification work units. Sixteen
// accounting publication/readback waves cover reservation/head/CAS artifacts and
// their bounded metadata; there is no report publication. Records also include
// 16 predecessors, 128 judgments, 8 endpoint and 16 occurrence visits per slot.
// Work uses (N+O)^2 dense-envelope CREDITS per kernel invocation, not actual
// Gonum loop visits, instruction counts or runtime. Four kernel invocations per
// slot are included. 16384 OBSERVATION-METHOD-CALL credits cover the production
// supervisor defaults, not test overrides. With I(w,p)=ceil(w/p)+1, live sampling
// costs <=3*I(60s,20ms)=9003 (GroupState, optional zombie ProcessStart, treeRSS);
// identity setup <=2*I(1s,20ms)=102; initialTreeRSS retries <=I(2s,1ms)=2001;
// cleanup <=4*I(1s,20ms)=204 (GroupState, leader ProcessStart, confirming
// GroupState and ProcessStart). Failed iterations sleep before retrying; helper
// deadlines/context checks stop retries, and terminal branches enter cleanup at
// most once. Sixteen further credits cover finite final/failure zombie fallback
// (at most two methods), so 11326 < 16384. CommandContext/WaitDelay may extend
// latency, not these retry counts; earlier deadline failures reduce calls.
// These are logical method calls, NOT snapshot bytes, host process-table volume,
// RAM, CPU, physical performance or an empirically observed poll count.
// ActiveNS includes the existing one-second CLEANUP-ONLY allowance; neither the
// worker nor Observe gains a fresh deadline. Wall/RSS enforcement remains the
// existing supervisor's mechanism, with its documented synchronous-work limits.
func privateTypedBundleChargeV2(now, deadline time.Time) (countersV2, error) {
	if deadline.IsZero() || !now.Before(deadline) || deadline.Sub(now) > programc.SupervisorWall {
		return countersV2{}, ErrPrivateTypedBundleOperationV2
	}
	const b uint64 = programc.GroupingCaseBundleMaxBytesPrivate
	const slots uint64 = programc.GroupingCaseBundleMaxSlotsPrivate
	const n uint64 = rg.EvaluationMaxNodesPrivate
	const occ uint64 = rg.EvaluationMaxOccurrencesPrivate
	const result uint64 = rg.EvaluationMaxBytesPrivate
	const transport uint64 = uint64(programc.MaxWorkerInputBytes+programc.MaxWorkerOutputBytes+programc.MaxWorkerStderrBytes) + 3
	const verify uint64 = 4 * (64 << 20)
	const observationCalls uint64 = 16384
	// Every operand is a fixed enforced cap, not a candidate-controlled integer.
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
	return countersV2{LogicalBytes: data, ReadHashBytes: verify + data, WriteBytes: writes, Records: records, ParserBytes: verify + data, ProviderRequests: 0, ProviderMessages: 0, Work: work, ActiveNS: uint64(deadline.Sub(now)) + uint64(time.Second), Calls: 4*16384 + records + 4*slots + observationCalls, RequestBytes: uint64(programc.MaxWorkerInputBytes) + 1, ResponseBytes: uint64(programc.MaxWorkerOutputBytes+programc.MaxWorkerStderrBytes) + 2, WireBytes: transport, OutputBytes: uint64(programc.MaxWorkerOutputBytes)}, nil
}

func validPrivateTypedBindingV2(q PrivateTypedBundleBindingV2, now time.Time) bool {
	return len(q.Candidate) > 0 && len(q.Candidate) <= programc.GroupingCaseBundleMaxBytesPrivate && q.Expected != (programc.GroupingBundleExpectationPrivate{}) && hashV2(q.Attempt) && hashV2(q.Head) && !q.Deadline.IsZero() && now.Before(q.Deadline) && q.Deadline.Sub(now) <= programc.SupervisorWall
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
func (o *PrivateTypedBundleOwnerV2) Execute(ctx context.Context, op PrivateTypedBundleOperationV2, q PrivateTypedBundleBindingV2) ([]byte, programc.SupervisionObservation, *programc.SupervisionFailure) {
	refuse := func(observation programc.SupervisionObservation, code programc.SupervisionCode) ([]byte, programc.SupervisionObservation, *programc.SupervisionFailure) {
		return nil, observation, &programc.SupervisionFailure{Code: code, Message: ErrPrivateTypedBundleOperationV2.Error()}
	}
	var observation programc.SupervisionObservation
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
	report, observed, failed := programc.ExecuteGroupingCaseBundleSupervisedPrivate(execution, s.expected, candidate)
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
