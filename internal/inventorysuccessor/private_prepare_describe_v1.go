package inventorysuccessor

import (
	"context"
	"lsp-trace/internal/programcadmission"
	"time"
)

// PrepareDescribeRequestV1 uses an already canonically admitted, funded owner.
// Its root and Manager are BORROWED. No HOST, timing extension, local worker,
// model, callback or publication configuration is accepted.
type PrepareDescribeRequestV1 struct {
	Owner                                                                   *PrivateManagedInputOwnerV2
	Capture                                                                 PrivateMixedInputRequestV2
	CaptureAttempt, PreparationAttempt, PacketAttempt, RunID, SeedNamespace string
	Seed                                                                    uint64
	Persist                                                                 PreparedDescribePersistRequestV1
	PacketLimit                                                             uint64
}

// StagedPrepareDescribeV1 is detached DATA ONLY. The outer owner must withhold
// it until every owned cleanup and the original deadline have succeeded. Neither
// this DTO nor its continuation references can be supplied as native authority.
type StagedPrepareDescribeV1 struct {
	Packet       []byte
	Continuation PreparedDescribeAnchorsV1
}

// PrepareDescribePacketV1 is the exact bounded evaluator data envelope. Labels
// attribute roles; they do not authenticate, fund or authorize those roles.
type PrepareDescribePacketV1 struct {
	Schema                                                                                  string
	Authority                                                                               int
	Accepted                                                                                bool
	FeatureIdentity, Completeness, AbsenceInference, HOSTCost                               string
	RoleLabels                                                                              [2]string
	Continuation                                                                            PreparedDescribeAnchorsV1
	Question, Request, ResponseSchema, Definition, Provenance                               []byte
	QuestionSHA256, RequestSHA256, ResponseSchemaSHA256, DefinitionSHA256, ProvenanceSHA256 string
}

func PrepareDescribeV1(ctx context.Context, q PrepareDescribeRequestV1) (StagedPrepareDescribeV1, error) {
	fail := func(stage PrivatePrepareStageV1, cause error) (StagedPrepareDescribeV1, error) {
		// Inline the closed value construction: the existing effects guard remains
		// unchanged and no new executable callback/operation enters this pipeline.
		codes := PrivatePrepareFailureCodesV1{Stage: stage, Reason: privatePrepareAutoReasonV1}
		reasonSource := cause
		if inner, ok := cause.(*privatePrepareFailureV1); ok && inner != nil {
			codes, reasonSource = inner.codes, inner.reasonSource
		}
		return StagedPrepareDescribeV1{}, &privatePrepareFailureV1{codes: codes, broad: ErrPreparedDescribeV1, cause: cause, reasonSource: reasonSource}
	}
	var contextCause error
	if ctx != nil {
		contextCause = ctx.Err()
	}
	now := time.Now()
	if ctx == nil || ctx.Err() != nil || q.Owner == nil || q.Capture.Manager == nil || q.Capture.Root == nil || q.Capture.Timing != nil || !q.Persist.Deadline.Equal(q.Capture.Start.Deadline) || !preparedDescribeRequestV1(q.Persist, now) || q.PacketLimit == 0 || q.PacketLimit > 1<<20 || !hashV2(q.PacketAttempt) || !hashV2(q.CaptureAttempt) || !hashV2(q.PreparationAttempt) || !preparedDescribeTextV1(q.RunID) || !preparedDescribeTextV1(q.SeedNamespace) || len(q.Capture.Kinds) != 1 || q.Capture.Kinds[0] != programcadmission.TypedReferencesSymbol {
		if q.Persist.Deadline != (time.Time{}) && !now.Before(q.Persist.Deadline) {
			contextCause = context.DeadlineExceeded
		}
		return fail(privatePrepareReserveV1, contextCause)
	}
	q.Owner.mu.Lock()
	valid := q.Owner.k != nil && q.Owner.k.root == q.Capture.Root && q.Owner.head == q.Persist.InitialHead && q.Owner.k.root.Path() == q.Persist.RootPath && !q.Owner.uncertain
	q.Owner.mu.Unlock()
	if !valid {
		return fail(privatePrepareReserveV1, nil)
	}
	control, cancel := context.WithDeadline(ctx, q.Persist.Deadline)
	defer cancel()
	capture, err := q.Owner.ReserveMixedInputV2(control, q.CaptureAttempt, q.Capture)
	if err != nil {
		return fail(privatePrepareReserveV1, err)
	}
	input, err := ExecuteMixedInputV2(control, capture, q.Capture)
	if err != nil {
		return fail(privatePrepareCaptureV1, err)
	}
	// ExecuteMixedInputV2 already Consume/Take/Finish/joins and confirms native
	// completion. Never Finish it again or reinterpret unavailable as empty.
	prepQ := PrivateSourceRPreparationRequestV1{Input: input, Root: q.Capture.Root, ExpectedHead: q.Owner.PreparationHeadV1(), Attempt: q.PreparationAttempt, Workspace: q.Capture.Start.Profile.Workspace().String(), Revision: q.Capture.Revision, RunID: q.RunID, SeedNamespace: q.SeedNamespace, Seed: q.Seed, Deadline: q.Persist.Deadline}
	prep, err := q.Owner.ReserveRPreparationV1(control, prepQ)
	if err != nil {
		return fail(privatePrepareRprepV1, err)
	}
	prepared, err := ExecuteRPreparationV1(control, prep, prepQ)
	if err != nil {
		return fail(privatePrepareRprepV1, err)
	}
	persist, err := q.Owner.ReservePreparedDescribeV1(control, prep, prepared, q.Persist)
	if err != nil {
		return fail(privatePreparePersistV1, err)
	}
	committed, err := CommitPreparedDescribeV1(control, persist)
	if err != nil {
		return fail(privatePreparePersistV1, err)
	}
	packet, err := q.Owner.preparedDescribePacketV1(control, committed, q.PacketAttempt, q.Persist.Deadline, q.PacketLimit)
	if err != nil {
		return fail(privatePreparePacketV1, err)
	}
	cancel()
	if control.Err() != context.Canceled || ctx.Err() != nil || !time.Now().Before(q.Persist.Deadline) {
		cause := ctx.Err()
		if !time.Now().Before(q.Persist.Deadline) {
			cause = context.DeadlineExceeded
		}
		return fail(privatePreparePacketV1, cause)
	}
	return StagedPrepareDescribeV1{Packet: packet, Continuation: committed.Anchors()}, nil
}
