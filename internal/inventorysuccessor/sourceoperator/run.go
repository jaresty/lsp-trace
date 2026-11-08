//go:build darwin || linux

// Package sourceoperator connects existing private owner custody to source-native
// file delivery. It cannot issue jobs, start roles, invoke models or resume custody.
package sourceoperator

import (
	"context"
	"errors"
	"fmt"
	"time"

	cc "lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/describeworker"
	inv "lsp-trace/internal/inventorysuccessor"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	host "lsp-trace/internal/privatedescribeprep"
	"lsp-trace/internal/programcadmission"
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
	ctx, cancel := context.WithDeadline(ctx, q.Capture.Start.Deadline)
	defer cancel()
	// Root admission writes nothing; all roots already exist. Topology is checked
	// before provider/source/mailbox effects and again around each output effect.
	roots, e := openPublication(ctx, q)
	if e != nil {
		return out, e
	}
	defer func() {
		// Registered first, executed last: the later eligible.Close defer finishes
		// before these publication/protected roots and the original-time check.
		err = errors.Join(err, roots.close())
		err = finalizeTerminal(ctx, q.Capture.Start.Deadline, &out, err)
	}()
	capture, e := q.Owner.ReserveMixedInputV2(ctx, q.CaptureAttempt, q.Capture)
	if e != nil {
		return out, e
	}
	input, e := inv.ExecuteMixedInputV2(ctx, capture, q.Capture)
	if e != nil {
		return out, e
	}
	prep := inv.PrivateSourceRPreparationRequestV1{Input: input, Root: q.Capture.Root, ExpectedHead: q.Owner.PreparationHeadV1(), Attempt: q.PreparationAttempt, Workspace: q.Capture.Start.Profile.Workspace().String(), Revision: q.Capture.Revision, RunID: q.RunID, SeedNamespace: q.SeedNamespace, Seed: q.Seed, Deadline: input.SourceDeadlineV1()}
	operation, e := q.Owner.ReserveRPreparationV1(ctx, prep)
	if e != nil {
		return out, e
	}
	prepared, e := inv.ExecuteRPreparationV1(ctx, operation, prep)
	if e != nil {
		return out, e
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
	record := source.Requests[0] // exact admitted record, never reconstructed DTO custody
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
	eligible, e := sc.PrepareSourceHost(ctx, mailbox, acquisition, q.ResponseLimits)
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
	native, e := roots.sealNative(delivered, source)
	if e != nil {
		return out, e
	}
	out.NativeID = native.id
	if e = roots.publish(ctx, native, &out); e != nil {
		return out, e
	}
	out.Disposition = "SOURCE_PROVISIONAL_DELIVERED_PAIR"
	return out, nil
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
