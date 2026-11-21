//go:build darwin || linux

package applicationinventory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	dw "lsp-trace/internal/describeworker"
	is "lsp-trace/internal/inventorysuccessor"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/scopejob"
)

const enumerationResourcePolicy = "lsp-trace.inventory-enumeration-resource.v1"
const selectionResourcePolicy = "lsp-trace.inventory-enumeration-resource.v2"

func enumerationPolicy(c scopejob.Config) string {
	if c.Selection != nil {
		return selectionResourcePolicy
	}
	return enumerationResourcePolicy
}
func enumerationSegmentSchema(c scopejob.Config) string {
	if c.Selection != nil {
		return "lsp-trace.inventory-scope-segment.v2"
	}
	return "lsp-trace.inventory-scope-segment.v1"
}

// Retain the scope policy's undiscounted 24MiB batch quote and 9MiB fixed
// allowance. Fixed allowance covers setup, two <=4MiB segment final readbacks,
// bounded head/config validation, namespace checks and <=32KiB status output.
// It is logical work in the SAME ledger, not a physical storage quota.
const enumerationFixed = 4 * (scopejob.HeadBytes + scopejob.StopBytes)
const enumerationMaxBatches = int(durableRunLimit / scopejob.BatchBytes)

type ownedEnumeration struct {
	Policy                           string
	Config                           publication.StoredObject
	Claims, Batches                  int
	Steps, PolicyWork, MetadataBytes uint64
	Last                             string
	Closed                           bool
	Reason                           string
}
type inventoryScopeSegment struct {
	Schema, RunID, Previous string
	Index                   int
	Steps                   uint64
	Batch                   scopejob.EnumerationBatch
}

type InventoryEnumerationStatus struct {
	Counts                                                                        scopejob.Counts
	Closed, UnknownRemainder, DenominatorComplete                                 bool
	SourceContentRead, AtomicSnapshot, InventoryPublished, AcquisitionImplemented bool
	Reason, CapabilityGap                                                         string
	CommittedSegmentBytes                                                         uint64
	PendingOperationDeadlineUnixNano                                              int64
	PendingOperationExpired                                                       bool
}

// EnumerationConfig selects the metadata-first workflow internally; it is not
// another CLI product mode. Semantic fields remain absent rather than fabricated.
// The existing budget-file traversal bounds are retained, but never enlarge the
// inventory owner's 64MiB/64-packet policy. No old stored config is rewritten.
func EnumerationConfig(c scopejob.Config) (OwnedInventoryConfig, error) {
	c.Budget.LogicalBytes = min(c.Budget.LogicalBytes, durableRunLimit)
	c.Budget.Packets = min(c.Budget.Packets, 64)
	c.Budget.OperationTimeout = min(c.Budget.OperationTimeout, 30*time.Second)
	canonical, err := scopejob.Canonical(c)
	if err != nil || c.JobDeadline != nil {
		return OwnedInventoryConfig{}, ErrDurableInventory
	}
	return OwnedInventoryConfig{Scope: c.Workspace, Provenance: "METADATA_ONLY_NOT_SOURCE_IDENTITY", Enumeration: &canonical, ResourcePolicy: enumerationPolicy(canonical), MaxAdmissions: 1, BudgetBytes: c.Budget.LogicalBytes, AdmissionBytes: 128 << 10, MaxAdmittedEvidenceBytes: 2 << 30, OperationWindow: c.Budget.OperationTimeout, TaskWindow: time.Second}, nil
}
func enumerationOnlyConfigValid(c OwnedInventoryConfig) bool {
	return c.Schema == ownedConfigVersion && c.Scope == c.Enumeration.Workspace && c.Provenance == "METADATA_ONLY_NOT_SOURCE_IDENTITY" && c.Policy == (sc.DurableDescribePolicyV1{}) && c.Pins == (dw.HostPinsV2{}) && c.ResponseLimits == (dw.SuppliedResponseLimits{}) && c.MaxAdmissions == 1 && c.BudgetBytes > 0 && c.BudgetBytes <= durableRunLimit && c.AdmissionBytes == 128<<10 && c.MaxAdmittedEvidenceBytes == 2<<30 && c.OperationWindow > 0 && c.OperationWindow <= 30*time.Second && c.TaskWindow == time.Second
}
func enumerationHeadValid(h durableInventoryHead) bool {
	e := h.Enumeration
	if e == nil {
		return false
	}
	if ownedPhase(h) && h.State != e.Config {
		return false
	}
	if (e.Policy != enumerationResourcePolicy && e.Policy != selectionResourcePolicy) || !e.Config.Valid() || e.Config.Bytes > ownedConfigMaxBytes || e.Claims < 0 || e.Claims > enumerationMaxBatches || e.Batches < 0 || e.Batches > e.Claims || e.Claims-e.Batches > 1 || e.Steps > uint64(e.Claims)*scopejob.BatchSteps || e.MetadataBytes > uint64(e.Batches)*scopejob.SegmentBytes || (e.Batches == 0) != (e.Last == "") || e.Batches > 0 && !validDurableID(e.Last) {
		return false
	}
	if e.Claims > 0 && h.Used < ownedCreateMinimum+enumerationFixed+uint64(e.Claims)*scopejob.BatchBytes {
		return false
	}
	if e.Closed && (e.Batches == 0 || e.Claims != e.Batches || e.Reason != "") {
		return false
	}
	switch h.Phase {
	case "Enumerating":
		return !e.Closed && e.Reason == "" && (h.Status == "CLAIMED" || h.Status == "READY" && e.Claims == 0)
	case "EnumerationPartial":
		return h.Status == "PARTIAL" && !e.Closed && e.Reason != ""
	case "ReadyForAcquisition":
		return h.Status == "READY" && e.Closed
	default:
		return e.Closed
	}
}
func enumerationDirectory(run string) string { return "inventory-scope-" + run }
func enumerationName(run string, i int) string {
	return fmt.Sprintf("%s/segment-%08d.json", enumerationDirectory(run), i)
}

func (s DurableStore) enumerationConfig(ctx context.Context, h durableInventoryHead) (OwnedInventoryConfig, error) {
	var c OwnedInventoryConfig
	if h.Enumeration == nil {
		return c, ErrDurableInventory
	}
	ref := h.Enumeration.Config
	raw, err := publication.ReadVerifiedBoundFileCheckedContext(ctx, s.Root, "inventory-owned-config-"+ref.ID+".json", int64(ref.Bytes))
	if err != nil || len(raw) != ref.Bytes || cat.NativeDigestV3(raw) != "sha256:"+ref.ID || strictInventory(raw, &c) != nil || !ownedConfigValid(c) || c.Enumeration == nil || c.ResourcePolicy != h.Enumeration.Policy || c.BudgetBytes != h.Limit {
		return c, ErrDurableInventory
	}
	return c, nil
}

// Enumerate consumes only a pristine enumeration phase. Intermediate checkpoints
// stay CLAIMED: a second process cannot restart the cursor or reset its budget.
func (s DurableStore) Enumerate(ctx context.Context, run, expectedHead string) (InventoryJobStatus, error) {
	return s.enumerate(ctx, run, expectedHead, nil)
}
func (s DurableStore) enumerate(ctx context.Context, run, expectedHead string, fault func(string) error) (InventoryJobStatus, error) {
	if ctx == nil || ctx.Err() != nil {
		return InventoryJobStatus{}, ErrDurableInventory
	}
	started := time.Now()
	preflight, preflightCancel := context.WithTimeout(ctx, 30*time.Second)
	defer preflightCancel()
	h, old, err := s.headContext(preflight, run)
	if err != nil {
		return InventoryJobStatus{}, err
	}
	if ctx == nil || ctx.Err() != nil || h.Phase != "Enumerating" || h.Status != "READY" || h.Enumeration == nil || h.Enumeration.Claims != 0 || cat.NativeDigestV3(old) != expectedHead {
		return InventoryJobStatus{}, ErrDurableInventory
	}
	c, err := s.enumerationConfig(preflight, h)
	if err != nil {
		return InventoryJobStatus{}, err
	}
	if _, err := s.inspectEnumerationAt(preflight, h, started); err != nil {
		return InventoryJobStatus{}, err
	}
	preflightCancel() // administrative inspection is not an overall job deadline
	ec := *c.Enumeration
	var cur *scopejob.Enumeration
	defer func() {
		if cur != nil {
			_ = cur.Close()
		}
	}()
	failure := func(e error) (InventoryJobStatus, error) {
		status, se := s.JobStatus(context.Background(), run)
		if se != nil {
			return InventoryJobStatus{Schema: "lsp-trace.inventory-job-status.v2", RunID: run, Phase: h.Phase, Status: "PARTIAL_UNCERTAIN", JobExpiry: "NONE"}, errors.Join(e, se)
		}
		return status, e
	}
	check := func(stage string) error {
		if fault != nil {
			if err := fault(stage); err != nil {
				return err
			}
		}
		return ctx.Err()
	}
	finish := func(reason string) (InventoryJobStatus, error) {
		op, cancel := context.WithTimeout(ctx, c.OperationWindow)
		defer cancel()
		if cur != nil {
			if e := cur.Close(); e != nil {
				reason = "CLOSE_FAILED"
			}
		}
		if op.Err() != nil {
			return failure(op.Err())
		}
		h.Status = "READY"
		h.TaskID = ""
		h.Deadline = 0
		h.Enumeration.Reason = reason
		h.Enumeration.Closed = reason == ""
		h.Phase = "ReadyForAcquisition"
		if reason != "" {
			h.Phase = "EnumerationPartial"
			h.Status = "PARTIAL"
		}
		if _, e := s.change(op, old, h); e != nil {
			return failure(e)
		}
		return s.JobStatus(op, run)
	}
	replay := scopejob.EnumerationReplay{}
	for {
		n := min(scopejob.BatchSteps, ec.Budget.Steps-h.Enumeration.Steps)
		work, ok := scopejob.EnumerationWork(ec, n)
		cost := scopejob.BatchBytes
		packets := uint64(2)
		if h.Enumeration.Claims == 0 {
			cost += enumerationFixed
			packets++ // closure is prepaid, not a fresh grant at finish
		}
		if n == 0 || !ok || work > ec.Budget.Work-h.Enumeration.PolicyWork || cost > h.Limit-h.Used || h.Packets+packets > min(uint64(64), ec.Budget.Packets) || h.Enumeration.Claims >= enumerationMaxBatches || scopejob.SegmentBytes > ec.Budget.RetainedBytes-h.Enumeration.MetadataBytes {
			// Before the first claim there is no paid closure allowance: a clean refusal
			// has no source effects. Later exhaustion uses the first claim's fixed reserve.
			if h.Enumeration.Claims == 0 {
				return failure(ErrDurableInventory)
			}
			return finish("BUDGET_EXHAUSTED")
		}
		if err := check("before-claim"); err != nil {
			return failure(err)
		}
		h.TaskID, err = newDurableID()
		if err != nil {
			return failure(err)
		}
		h.Status = "CLAIMED"
		h.Deadline = time.Now().Add(c.OperationWindow).UnixNano()
		h.Used += cost
		h.Work = h.Used * 32
		h.Packets += packets
		h.Enumeration.Claims++
		h.Enumeration.Steps += n
		h.Enumeration.PolicyWork += work
		op, cancel := context.WithDeadline(ctx, time.Unix(0, h.Deadline))
		old, err = s.change(op, old, h)
		if err != nil {
			cancel()
			return failure(err)
		}
		if err = check("after-claim"); err != nil {
			cancel()
			return failure(err)
		}
		if h.Enumeration.Claims == 1 {
			child, e := s.Root.OpenPrivateChild(enumerationDirectory(run), true)
			if e == nil {
				e = child.Close()
			}
			if e != nil {
				cancel()
				return failure(e)
			}
			if op.Err() != nil {
				cancel()
				return failure(op.Err())
			}
			cur, err = scopejob.OpenEnumeration(ec)
			if err != nil {
				cancel()
				return failure(err)
			}
			if op.Err() != nil {
				cancel()
				return failure(op.Err())
			}
			if c.EnumerationRequireGoRoot && !cur.HasGoRoot() {
				cancel()
				return failure(ErrDurableInventory)
			}
		}
		batch := cur.Batch(op, n)
		if err = op.Err(); err != nil {
			cancel()
			return failure(err)
		}
		if err = replay.Apply(op, ec, batch, n); err != nil {
			cancel()
			return failure(err)
		}
		record := inventoryScopeSegment{Schema: enumerationSegmentSchema(ec), RunID: run, Previous: h.Enumeration.Last, Index: h.Enumeration.Batches, Steps: n, Batch: batch}
		raw, e := json.Marshal(record)
		if e != nil || uint64(len(raw)) > scopejob.SegmentBytes {
			cancel()
			return failure(ErrDurableInventory)
		}
		if err = check("before-segment"); err != nil {
			cancel()
			return failure(err)
		}
		if err = op.Err(); err != nil {
			cancel()
			return failure(err)
		}
		result, e := publication.PublishBoundFile(s.Root, enumerationName(run, record.Index), raw, func(got []byte) error {
			if !bytes.Equal(raw, got) {
				return ErrDurableInventory
			}
			return nil
		})
		if e != nil || result == nil || !result.NamespaceAtomic || result.VerificationStatus != "VERIFIED" || result.DirectorySyncStatus != publication.DirectorySyncComplete || result.CloseStatus != publication.CloseComplete || op.Err() != nil {
			cancel()
			return failure(errors.Join(ErrDurableInventory, e))
		}
		if err = check("after-segment"); err != nil {
			cancel()
			return failure(err)
		}
		h.Enumeration.Last = cat.NativeDigestV3(raw)[7:]
		h.Enumeration.Batches++
		h.Enumeration.MetadataBytes += uint64(len(raw))
		old, err = s.change(op, old, h)
		cancel()
		if err != nil {
			return failure(err)
		}
		if batch.Reason != "" {
			return finish(batch.Reason)
		}
		if batch.Done {
			return finish("")
		}
	}
}

// Administrative verification reads bounded immutable segments, never source
// metadata. It does not acquire execution authority, renew leases or rewrite a job.
func (s DurableStore) inspectEnumeration(ctx context.Context, h durableInventoryHead) (InventoryEnumerationStatus, error) {
	return s.inspectEnumerationAt(ctx, h, time.Now())
}
func (s DurableStore) inspectEnumerationAt(ctx context.Context, h durableInventoryHead, started time.Time) (out InventoryEnumerationStatus, err error) {
	defer func() {
		if e := ctx.Err(); e != nil {
			out = InventoryEnumerationStatus{}
			err = e
		}
	}()
	if !enumerationHeadValid(h) {
		return out, ErrDurableInventory
	}
	c, err := s.enumerationConfig(ctx, h)
	if err != nil {
		return out, err
	}
	ec := *c.Enumeration
	op, cancel := context.WithDeadline(ctx, started.Add(c.OperationWindow))
	defer cancel()
	e := h.Enumeration
	var replay scopejob.EnumerationReplay
	previous := ""
	var metadata, steps uint64
	done := false
	reason := ""
	for i := 0; i < e.Batches; i++ {
		if metadata > ec.Budget.RetainedBytes || scopejob.SegmentBytes > ec.Budget.RetainedBytes-metadata {
			return out, ErrDurableInventory
		}
		raw, x := publication.ReadVerifiedBoundFileCheckedContext(op, s.Root, enumerationName(h.RunID, i), int64(scopejob.SegmentBytes))
		if x != nil {
			return out, x
		}
		var segment inventoryScopeSegment
		if strictInventory(raw, &segment) != nil || steps >= ec.Budget.Steps || segment.Steps != min(scopejob.BatchSteps, ec.Budget.Steps-steps) || segment.Batch.Reason == "EXPIRED" || segment.Schema != enumerationSegmentSchema(ec) || segment.RunID != h.RunID || segment.Index != i || segment.Previous != previous || done || reason != "" {
			return out, ErrDurableInventory
		}
		if x = replay.Apply(op, ec, segment.Batch, segment.Steps); x != nil {
			return out, x
		}
		steps += segment.Steps
		metadata += uint64(len(raw))
		previous = cat.NativeDigestV3(raw)[7:]
		done = segment.Batch.Done
		reason = segment.Batch.Reason
	}
	if steps > ec.Budget.Steps {
		return out, ErrDurableInventory
	}
	quote, qerr := ownedCreationQuote(h.Enumeration.Config.Bytes)
	minimum := quote
	minimumPackets := uint64(1) + uint64(e.Claims)*2
	if e.Claims > 0 {
		minimum += enumerationFixed + uint64(e.Claims)*scopejob.BatchBytes
		minimumPackets++
	}
	if qerr != nil || h.Used < minimum || h.Packets < minimumPackets || minimumPackets > ec.Budget.Packets {
		return out, ErrDurableInventory
	}
	expectedSequence := 0
	if is.InventoryResumableHeadV1(h.Schema) && h.Checkpoint != nil && h.Phase == "ReadyForAcquisition" {
		minimum += is.InventoryNativeControlForHeadV1(h.Schema) + is.InventorySelectedControlForHeadV1(h.Schema)
		minimumPackets += 2
		expectedSequence = 1
	}
	if (h.Phase == "Enumerating" || h.Phase == "EnumerationPartial" || h.Phase == "ReadyForAcquisition") && (h.Used != minimum || h.Packets != minimumPackets || h.Sequence != expectedSequence) {
		return out, ErrDurableInventory
	}
	expectedSteps := steps
	if e.Claims > e.Batches {
		if metadata > ec.Budget.RetainedBytes || scopejob.SegmentBytes > ec.Budget.RetainedBytes-metadata {
			return out, ErrDurableInventory
		}
		expectedSteps += min(scopejob.BatchSteps, ec.Budget.Steps-steps)
	}
	work, ok := scopejob.EnumerationWork(ec, expectedSteps)
	if !ok || expectedSteps != e.Steps || work != e.PolicyWork || previous != e.Last || metadata != e.MetadataBytes || e.Closed && !done || h.Phase == "EnumerationPartial" && e.Reason != "CLOSE_FAILED" && (done || reason != "" && reason != e.Reason) {
		return out, ErrDurableInventory
	}
	// Namespace enumeration is scoped to this exact inventory RunID, not a scan
	// over shared publication storage. A pending orphan is never committed evidence.
	child, x := s.Root.OpenPrivateChild(enumerationDirectory(h.RunID), false)
	if x != nil {
		if !errors.Is(x, os.ErrNotExist) || e.Batches != 0 {
			return out, x
		}
	} else {
		found := 0
		x = child.VisitNamesBounded(uint64(enumerationMaxBatches+1), func(name string) error {
			if op.Err() != nil {
				return op.Err()
			}
			for i := 0; i < e.Batches; i++ {
				if name == fmt.Sprintf("segment-%08d.json", i) {
					found++
					return nil
				}
			}
			if h.Status == "CLAIMED" && e.Claims > e.Batches && name == fmt.Sprintf("segment-%08d.json", e.Batches) {
				return nil
			}
			return ErrDurableInventory
		})
		x = errors.Join(x, child.Close(), op.Err())
		if x != nil || found != e.Batches {
			return out, errors.Join(ErrDurableInventory, x)
		}
	}
	out = InventoryEnumerationStatus{Counts: replay.Counts(), Closed: e.Closed, UnknownRemainder: !e.Closed || replay.Counts().Boundaries > 0, Reason: e.Reason, CapabilityGap: "ACQUISITION_UNAVAILABLE; metadata is not source identity; future acquisition requires paid revalidation", CommittedSegmentBytes: metadata}
	if ec.Selection != nil {
		out.CapabilityGap = "CLIENT_DECLARED_ELIGIBILITY_NOT_SERVER_CONFIRMED; " + out.CapabilityGap
	}
	out.DenominatorComplete = e.Closed && !out.UnknownRemainder
	if h.Phase == "Enumerating" && h.Status == "CLAIMED" {
		out.PendingOperationDeadlineUnixNano = h.Deadline
		out.PendingOperationExpired = time.Now().UnixNano() >= h.Deadline
	}
	if op.Err() != nil {
		return InventoryEnumerationStatus{}, op.Err()
	}
	return out, nil
}
