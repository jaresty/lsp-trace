//go:build darwin || linux

// Package sourceoperator connects existing private owner custody to source-native
// file delivery. Its explicitly selected private local adapter can run a pinned
// worker inside existing custody; it cannot issue jobs or resume closed custody.
package sourceoperator

import (
	"context"
	"errors"
	"fmt"
	"time"

	cc "lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/describerequest"
	"lsp-trace/internal/describeworker"
	inv "lsp-trace/internal/inventorysuccessor"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	host "lsp-trace/internal/privatedescribeprep"
	pc "lsp-trace/internal/programc"
	"lsp-trace/internal/programcadmission"
	rg "lsp-trace/internal/referencegrouping"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/targetpacket"
)

type Request struct {
	Owner                                                                 *inv.PrivateManagedInputOwnerV2
	Capture                                                               inv.PrivateMixedInputRequestV2
	CaptureAttempt, PreparationAttempt, RoleAttempt, RunID, SeedNamespace string
	Seed                                                                  uint64
	Contract                                                              cc.ContinuationContract
	SourceLimits                                                          targetpacket.PrivateSourceLimits
	Host                                                                  host.SourceHostConfig
	ResponseLimits                                                        describeworker.SuppliedResponseLimits
	OutputRoots                                                           [2]string
	// Local is private and default-off. Only the explicit local-worker adapter
	// identity enables B; other local configurations retain A's disabled delivery.
	Local *inv.PrivateSourceLocalConfigV1
}
type Outcome struct {
	Schema, Disposition, NativeID string
	Published                     [2]bool
	Readback                      [2]bool
	Counts                        sc.PrivateSourceCounts
}

var ErrRefused = errors.New("private source operator refused")
var ErrNativeAggregate = errors.New("private source native retained-plus-transient aggregate refused")

// Run retains owner/originals/expected/store in this original invocation through
// terminal close. A failure may leave cumulative accounting/mailbox/output bytes;
// it never refunds, retries, renews a deadline or serializes resume authority.
func Run(ctx context.Context, q Request) (out Outcome, err error) {
	out.Schema = "SOURCE_NATIVE_V1"
	out.Disposition = "REFUSED"
	if ctx == nil || ctx.Err() != nil || q.Owner == nil || q.Owner.PreparationHeadV1() == "" || !q.SourceLimits.Valid() || !q.ResponseLimits.Valid() || q.Capture.Root == nil || q.Capture.Manager == nil || len(q.Capture.Kinds) != 1 || q.Capture.Kinds[0] != programcadmission.TypedReferencesSymbol || q.Capture.Start.Deadline.IsZero() || !time.Now().Before(q.Capture.Start.Deadline) || time.Until(q.Capture.Start.Deadline) > 15*time.Second || q.RunID == "" || len(q.RunID) > 256 || q.SeedNamespace == "" || len(q.SeedNamespace) > 256 || q.RoleAttempt == "" || len(q.RoleAttempt) > 256 {
		return out, ErrRefused
	}
	if q.Local != nil {
		// Freeze the fixed declaration before capture; the local option never
		// widens a legacy or caller contract into additional credit/headroom.
		frozen := *q.Local
		q.Local = &frozen
		l := q.Host.Mailbox.Limits
		if frozen.Config.AdapterIdentity == inv.PrivateSourceLocalWorkerAdapterV1 && (q.Host.Mailbox.TimeoutMS > 5000 || q.Host.Mailbox.OverallTimeoutMS > 5000 || q.Host.Mailbox.PollMS != 10 || q.SourceLimits.MaxWork > 4096 || q.SourceLimits.MaxObjects > 16) {
			return out, ErrRefused
		}
		if q.SourceLimits.MaxDocumentBytes > 16<<10 || q.SourceLimits.MaxTotalBytes > 64<<10 || q.Contract.PacketMaxResponseBytes() > 131072 || q.ResponseLimits.MaxAdapterBytes > 8192 || q.ResponseLimits.MaxSemanticBytes > 4096 || l.MaxCalls > 1 || l.MaxRequestBytes > 65536 || l.MaxResponseBytes > 16384 || l.MaxTotalBytes > 147456 || l.MaxOutputBytes > 65536 {
			return out, ErrRefused
		}
	}
	ctx, cancel := context.WithDeadline(ctx, q.Capture.Start.Deadline)
	defer cancel()
	// Root admission writes nothing; all roots already exist. Topology is checked
	// before provider/source/mailbox effects and again around each output effect.
	roots, e := openPublication(ctx, q)
	if e != nil {
		return out, e
	}
	var pending inv.PendingLocalSourceRV1
	var workerObservations []inv.PrivateSourceLocalWorkerObservationV1
	hasPending, localReady := false, false
	defer func() {
		// Registered first, executed last: the later eligible.Close defer finishes
		// before these publication/protected roots and the original-time check.
		closeErr := roots.close()
		err = errors.Join(err, closeErr)
		var localErr error
		if hasPending {
			prior := err
			if localReady {
				prior = closeErr
			} // delivery-disabled is not a cleanup failure
			localErr = pending.Finalize(ctx, prior)
			err = errors.Join(err, localErr)
		}
		err = finalizeTerminal(ctx, q.Capture.Start.Deadline, &out, err)
		if localReady && localErr == nil && closeErr == nil && ctx.Err() == nil && time.Now().Before(q.Capture.Start.Deadline) {
			out.Disposition = "LOCAL_VERIFIED_DELIVERY_DISABLED"
		}
	}()
	capture, e := q.Owner.ReserveMixedInputV2(ctx, q.CaptureAttempt, q.Capture)
	if e != nil {
		return out, e
	}
	input, e := inv.ExecuteMixedInputV2(ctx, capture, q.Capture)
	if e != nil {
		return out, e
	}
	prep := inv.PrivateSourceRPreparationRequestV1{Input: input, Root: q.Capture.Root, ExpectedHead: q.Owner.PreparationHeadV1(), Attempt: q.PreparationAttempt, Workspace: q.Capture.Start.Profile.Workspace().String(), Revision: q.Capture.Revision, RunID: q.RunID, SeedNamespace: q.SeedNamespace, Seed: q.Seed, Deadline: input.SourceDeadlineV1(), Local: q.Local}
	operation, e := q.Owner.ReserveRPreparationV1(ctx, prep)
	if e != nil {
		return out, e
	}
	var prepared sourcePreparationMetadata
	if q.Local != nil {
		pending, e = inv.ExecuteLocalRPreparationV1(ctx, operation, prep)
		if e != nil {
			return out, e
		}
		hasPending = true
		prepared = pending
	} else {
		prepared, e = inv.ExecuteRPreparationV1(ctx, operation, prep)
		if e != nil {
			return out, e
		}
	}
	admitted, policy, frozen, result, originals, e := prepared.Values()
	if e != nil {
		return out, e
	}
	// Preserve the original seven-mode order. Only R is funded and captured here.
	// Missing means unresolved, not empty/absent or an all-seven success claim.
	modes := [7]string{"RD", "C", "CRD", "D", "CD", "R", "CR"}
	slots := make([]cc.PrivateAllModeSlot, 7)
	entries := make([]cc.PrivateAllModeEntry, 7)
	for i, mode := range modes {
		runID := fmt.Sprintf("%s/%s", q.RunID, mode)
		if mode == "R" {
			runID = q.RunID
		}
		slots[i] = cc.PrivateAllModeSlot{Ordinal: i, Mode: mode, RunID: runID, SeedNamespace: q.SeedNamespace, Seed: q.Seed}
		entries[i] = cc.PrivateAllModeEntry{Slot: slots[i], Disposition: cc.PrivateAllModeDeclaredMissing, Lineage: q.RunID, MissingReason: "NOT_CAPTURED_BY_R_ONLY_OPERATOR;UNRESOLVED"}
		if mode == "R" {
			entries[i] = cc.PrivateAllModeEntry{Slot: slots[i], Disposition: cc.PrivateAllModeReady, Lineage: q.RunID, Input: admitted, Policy: policy, Case: frozen, Result: result}
		}
	}
	handoff, e := cc.NewPrivateAllModeHandoff(prep.Workspace, q.Capture.Revision, slots, entries)
	if e != nil {
		return out, e
	}
	store, e := sc.NewRootlessSourceHostStore(int64(q.Contract.CaptureLimits().MaxArtifactBytes))
	if e != nil {
		return out, e
	}
	objects, e := prepared.SourceObjects()
	if e != nil {
		return out, e
	}
	sources := sc.PrivateAllModeSourcesRequest{PolicyID: q.Contract.PacketSourcePolicy().PolicyID, Limits: q.SourceLimits, Deadline: input.SourceDeadlineV1(), Originals: []inv.PrivateProducerOriginals{originals}, Objects: objects}
	pause := sc.Run(ctx, sc.Request{PrivateAllMode: &handoff, PrivateSources: &sources, Workspace: prep.Workspace, Contract: q.Contract, Store: store, StopAfter: sc.StopAfterPrivateSourceRequests})
	if pause.Err != nil {
		return out, pause.Err
	}
	if pause.PrivateSources == nil || len(pause.PrivateSources.Requests) == 0 {
		return out, ErrRefused
	}
	source := pause.PrivateSources
	out.Counts = source.Counts
	record, e := originalQueryRecord(prepared, source)
	if e != nil {
		return out, e
	}
	resume := sc.ResumeRequest{PrivateSources: &sc.PrivateAllModeSourcesResume{Version: sc.PrivateAllModeSourcesVersion, Expected: source.Expected, Originals: sources.Originals, ExpectedHandoff: handoff}, Workspace: prep.Workspace, Store: store, StopAfter: sc.StopAfterPrivateSourceRequests, Selector: source.CheckpointID}
	acquisition := sc.SourceAcquisitionRequest{Resume: resume, Record: record, Attempt: sc.SuppliedSourceAttempt{ID: q.RoleAttempt, CheckpointID: source.CheckpointID, SourceUnitID: record.Envelope.AdmissionID, RequestRecordID: record.RecordID}}
	mailbox := q.Host
	// These are exact derived context pins, not a new kernel issuer or authority.
	mailbox.Mailbox.ScopeSHA256 = handoff.ID()
	mailbox.Mailbox.ApprovalSHA256 = host.AutomaticHandoffApprovalPrivateV2(mailbox.Mailbox)
	preparedUpper, ok := prepared.RetainedPayloadUpper()
	if !ok {
		return out, ErrRefused
	}
	held, suppliedUpper, deliveryUpper, semanticUpper, e := source.NativeRetentionBounds(ctx, mailbox, acquisition, q.ResponseLimits)
	if e != nil {
		return out, e
	}
	if e = roots.admitNativeMetadata(source, preparedUpper, held, suppliedUpper, deliveryUpper, semanticUpper); e != nil {
		return out, e
	}
	phases, e := source.HostPhaseBounds(ctx, mailbox, acquisition, q.ResponseLimits)
	if e != nil {
		return out, e
	}
	if e = roots.admitHostMetadata(preparedUpper, phases); e != nil {
		return out, e
	}
	if hasPending {
		// The local allowance is joined into preparedUpper, not borrowed from the
		// transport/binder headroom. Both connected aggregate guards ran above.
		if e = pending.VerifyPins(ctx); e != nil {
			return out, e
		}
		if q.Local.Config.AdapterIdentity != inv.PrivateSourceLocalWorkerAdapterV1 {
			localReady = true
			return out, inv.ErrPrivateSourceLocalDeliveryDisabledV1
		}
	}
	eligible, e := sc.PrepareSourceHost(ctx, mailbox, acquisition, q.ResponseLimits, hasPending)
	if e != nil {
		return out, e
	}
	defer func() { err = errors.Join(err, eligible.Close()) }()
	// Re-read current store retention before exchange or binder regeneration.
	// Use the original raw response ceiling even though no response exists yet.
	phases, e = source.HostPhaseBounds(ctx, mailbox, acquisition, q.ResponseLimits)
	if e != nil {
		return out, e
	}
	if e = roots.admitHostMetadata(preparedUpper, phases); e != nil {
		return out, e
	}
	if hasPending {
		// The already-prepaid local owner executes only the exact admitted prompt.
		// Its original P is still pending. HOST's existing overall deadline also
		// bounds generation; neither worker defaults nor Exchange renew that window.
		workerCtx, workerCancel := context.WithDeadline(ctx, eligible.Deadline())
		payload, workerErr := pending.ExecuteWorker(workerCtx, record.Envelope.Prompt)
		workerCancel()
		if workerErr != nil {
			return out, workerErr
		}
		observation, observationErr := pending.WorkerOutputObservation()
		if observationErr != nil {
			return out, observationErr
		}
		workerObservations = []inv.PrivateSourceLocalWorkerObservationV1{observation}
		if e = eligible.ArmLocalResponse(ctx, payload); e != nil {
			return out, e
		}
	}
	delivered, e := sc.CompleteSourceHost(ctx, eligible)
	if e != nil {
		return out, e
	}
	// Resume may have added store objects. Re-read only trusted scalar retention
	// after actual response, before the first native accessor copy/encoding/hash.
	held, suppliedUpper, deliveryUpper, semanticUpper, e = source.NativeRetentionBounds(ctx, mailbox, acquisition, q.ResponseLimits)
	if e != nil {
		return out, e
	}
	if e = roots.admitNativeMetadata(source, preparedUpper, held, suppliedUpper, deliveryUpper, semanticUpper); e != nil {
		return out, e
	}
	native, e := roots.sealNative(delivered, source, workerObservations...)
	if e != nil {
		return out, e
	}
	out.NativeID = native.id
	if e = roots.publish(ctx, native, &out); e != nil {
		return out, e
	}
	out.Disposition = "SOURCE_PROVISIONAL_DELIVERED_PAIR"
	if hasPending {
		out.Disposition = "SOURCE_LOCAL_PROVISIONAL_DELIVERED_PAIR"
	}
	return out, nil
}

// originalQueryRecord selects only the original native query target. It does not
// mutate nominations, sort, format a replacement record, or admit a public DTO.
// The later PrepareSourceHost still validates the full record against opaque
// source admission and the exact original checkpoint/producer custody.
type sourcePreparationMetadata interface {
	Values() (rg.AdmittedInput, rg.GroupingPolicy, pc.GroupingEvaluationCasePrivate, pc.GroupingEvaluationResultPrivate, inv.PrivateProducerOriginals, error)
	SourceObjects() ([]sourceobject.Object, error)
	OriginalQueryTargetID([]targetpacket.PrivateSourceUnit) (string, error)
	RetainedPayloadUpper() (uint64, bool)
}

func originalQueryRecord(prepared sourcePreparationMetadata, source *sc.PrivateAllModeSourcesResult) (describerequest.Record, error) {
	if source == nil || len(source.Units) == 0 || len(source.Units) > 1000 || len(source.Requests) == 0 || len(source.Requests) > 1000 {
		return describerequest.Record{}, ErrRefused
	}
	id, err := prepared.OriginalQueryTargetID(source.Units)
	if err != nil {
		return describerequest.Record{}, ErrRefused
	}
	return originalQueryRecordForID(id, source.Requests)
}

// id must come from OriginalQueryTargetID above. This join is not admission.
func originalQueryRecordForID(id string, records []describerequest.Record) (describerequest.Record, error) {
	if id == "" || len(records) == 0 || len(records) > 1000 {
		return describerequest.Record{}, ErrRefused
	}
	var record describerequest.Record
	matches := 0
	for _, r := range records {
		if r.Envelope.AdmissionID == id {
			if r.Lineage.PacketID != id || r.Lineage.CensusID != id || r.RecordID == "" {
				return describerequest.Record{}, ErrRefused
			}
			record = r
			matches++
		}
	}
	if matches != 1 {
		return describerequest.Record{}, ErrRefused
	}
	return record, nil
}

// finalizeTerminal runs only after the caller's last owned close. It does not
// roll back published bytes, clear actual flags, renew time or retry anything.
func finalizeTerminal(ctx context.Context, deadline time.Time, out *Outcome, err error) error {
	if ctx == nil {
		err = errors.Join(err, ErrRefused)
	} else {
		err = errors.Join(err, ctx.Err())
	}
	if deadline.IsZero() || !time.Now().Before(deadline) {
		err = errors.Join(err, context.DeadlineExceeded)
	}
	if err != nil {
		out.Disposition = "FAILED_RETAINED"
	}
	return err
}
