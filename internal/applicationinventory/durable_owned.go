//go:build darwin || linux

package applicationinventory

import (
	"context"
	"encoding/json"
	"fmt"
	"lsp-trace/internal/scopejob"
	"reflect"
	"sync/atomic"
	"time"

	dw "lsp-trace/internal/describeworker"
	is "lsp-trace/internal/inventorysuccessor"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
)

const ownedHeadVersion = "lsp-trace.durable-inventory-head.v4"
const ownedConfigVersion = "lsp-trace.inventory-owned-config.v2"
const ownedCreateModel = "lsp-trace.inventory-create-cost.v1"
const ownedConfigMaxBytes = 32768
const ownedBootstrapMaxBytes = 65536
const ownedSealCost uint64 = 2 << 20

// OwnedInventoryConfig is finite caller work, not acquisition authority. No job
// expiry: only operations and tasks have leases. MaxAdmittedEvidenceBytes is a separate
// provisional admitted-evidence byte ceiling, not a physical directory quota
// (orphans and semantic objects are excluded). It cannot enlarge logical work.
type OwnedInventoryConfig struct {
	Schema                                                string
	Scope, Provenance, Interpreter                        string
	Policy                                                sc.DurableDescribePolicyV1
	Pins                                                  dw.HostPinsV2
	ResponseLimits                                        dw.SuppliedResponseLimits
	MaxAdmissions                                         int
	BudgetBytes, AdmissionBytes, MaxAdmittedEvidenceBytes uint64
	OperationWindow, TaskWindow                           time.Duration
	Enumeration                                           *scopejob.Config               `json:",omitempty"`
	ResourcePolicy                                        string                         `json:",omitempty"`
	EnumerationRequireGoRoot                              bool                           `json:",omitempty"`
	NativeProfile                                         *is.InventoryNativeProfileV1   `json:",omitempty"`
	SelectedProfile                                       *is.InventorySelectedProfileV1 `json:",omitempty"`
	ResumeProfile                                         *is.InventoryResumeProfileV1   `json:",omitempty"`
}

type InventoryJobStatus struct {
	Schema, RunID, Head, Phase, Status, JobExpiry string
	Used, Limit, Packets                          uint64
	Admissions                                    int
	TaskID                                        string
	Enumeration                                   *InventoryEnumerationStatus  `json:",omitempty"`
	Native                                        *is.InventoryNativeClaimV1   `json:",omitempty"`
	Selected                                      *is.InventorySelectedClaimV1 `json:",omitempty"`
	Checkpoint                                    *is.InventoryCheckpointV1    `json:",omitempty"`
}

type InventoryAdmissionBinding struct {
	Index                               int
	Provenance, RequestID, SourceUnitID string
}
type ownedAdmissionRecord struct {
	Schema, RunID, LeadID          string
	Index                          int
	Previous, Evidence             publication.StoredObject
	Binding                        InventoryAdmissionBinding
	Payload, AdmittedEvidenceBytes uint64
}

// AdmissionReservation is a one-shot operation, never a persisted capability.
// After process loss a CLAIMED head is terminal partial uncertainty.
type AdmissionReservation struct{ state *ownedAdmissionOperation }
type ownedAdmissionOperation struct {
	used        atomic.Bool
	store       DurableStore
	head        durableInventoryHead
	predecessor []byte
	config      OwnedInventoryConfig
	binding     InventoryAdmissionBinding
}

func ownedPhase(h durableInventoryHead) bool {
	if h.Schema == is.InventoryResumeHeadVersionV1 {
		return true
	}
	if h.Schema == is.InventorySelectedHeadVersionV1 {
		return true
	} // closed validator below rejects unknown phases
	return (h.Schema == ownedHeadVersion || h.Schema == is.InventoryNativeHeadVersionV1) && (h.Schema == is.InventoryNativeHeadVersionV1 && h.Phase == "NativeAcquisition" || h.Phase == "Creating" || h.Phase == "Collecting" || h.Phase == "Sealing" || h.Phase == "Enumerating" || h.Phase == "ReadyForAcquisition" || h.Phase == "EnumerationPartial")
}
func (h durableInventoryHead) operationDeadline() int64 {
	if h.Schema == ownedHeadVersion {
		return h.Deadline
	}
	return min(h.Deadline, h.RunDeadline)
}
func (h durableInventoryHead) budgetDeadline() int64 {
	if h.Schema == ownedHeadVersion {
		return h.Deadline
	}
	return h.RunDeadline
}
func validInventoryDeadlines(h durableInventoryHead) bool {
	if h.Schema == ownedHeadVersion {
		return h.RunDeadline == 0 && h.Deadline > 0 && h.AcceptanceDeadline > 0
	}
	return h.RunDeadline > h.Issued && h.RunDeadline-h.Issued <= int64(180*time.Second) && h.Deadline <= h.RunDeadline && h.AcceptanceDeadline <= h.RunDeadline
}
func ownedConfigValid(c OwnedInventoryConfig) bool {
	if c.Schema == is.InventoryResumeConfigVersionV1 {
		return resumeConfigValidV1(c)
	}
	if c.ResumeProfile != nil {
		return false
	}
	if c.SelectedProfile != nil {
		return selectedInventoryConfigValidV1(c)
	}
	if c.NativeProfile != nil {
		if c.Schema != is.InventoryNativeConfigVersionV1 || !c.NativeProfile.Valid() || c.Interpreter != "" || c.Enumeration == nil || c.Enumeration.Budget.Steps > scopejob.BatchSteps {
			return false
		}
		c.NativeProfile = nil
		c.Schema = ownedConfigVersion
		return ownedConfigValid(c)
	}
	if c.Enumeration != nil {
		canonical, err := scopejob.Canonical(*c.Enumeration)
		if err != nil || !reflect.DeepEqual(canonical, *c.Enumeration) || c.ResourcePolicy != enumerationPolicy(*c.Enumeration) || c.Enumeration.JobDeadline != nil || c.Enumeration.Budget.OperationTimeout > 30*time.Second || c.Enumeration.Budget.LogicalBytes != c.BudgetBytes || c.Enumeration.Budget.Packets > 64 {
			return false
		}
		if _, ok := scopejob.EnumerationWork(*c.Enumeration, min(scopejob.BatchSteps, c.Enumeration.Budget.Steps)); !ok || c.Enumeration.Budget.Packets < 4 || c.Enumeration.Budget.RetainedBytes < scopejob.SegmentBytes || c.OperationWindow != c.Enumeration.Budget.OperationTimeout {
			return false
		}
		// Metadata-only creation makes no invented worker/model pins. A future
		// genuine acquisition caller may supply the existing semantic configuration.
		if c.Interpreter == "" {
			return enumerationOnlyConfigValid(c)
		}
	} else if c.ResourcePolicy != "" || c.EnumerationRequireGoRoot {
		return false
	}
	if c.Schema != ownedConfigVersion || len(c.Scope) == 0 || len(c.Scope) > 4096 || len(c.Provenance) == 0 || len(c.Provenance) > 4096 || len(c.Interpreter) == 0 || len(c.Interpreter) > 256 || len(c.Policy.Reviewer) == 0 || len(c.Policy.Reviewer) > 256 || !c.Policy.ReviewMode.Valid() || !c.Policy.ReviewMode.AllowsSameActor() && c.Interpreter == c.Policy.Reviewer || !c.ResponseLimits.Valid() || c.MaxAdmissions < 1 || c.MaxAdmissions > 16 || c.BudgetBytes == 0 || c.BudgetBytes > durableRunLimit || c.AdmissionBytes < 128<<10 || c.AdmissionBytes > durableStepLimit || c.MaxAdmittedEvidenceBytes == 0 || c.MaxAdmittedEvidenceBytes > 2<<30 || c.OperationWindow <= 0 || c.OperationWindow > 30*time.Second || c.TaskWindow <= 0 || c.TaskWindow > 30*time.Second || time.Duration(c.MaxAdmissions+2)*c.TaskWindow > 120*time.Second {
		return false
	}
	for _, p := range c.Policy.RolePrompts {
		if len(p) == 0 || len(p) > 4096 {
			return false
		}
	}
	for _, p := range []string{c.Pins.WorkerSHA256, c.Pins.ModelSHA256, c.Pins.GrammarSHA256} {
		if len(p) != 71 || p[:7] != "sha256:" || !validDurableID(p[7:]) {
			return false
		}
	}
	return true // Encoded-length quote and funding are validated before effects.
}
func ownedHeadValid(h durableInventoryHead) bool {
	if h.Schema == is.InventoryResumeHeadVersionV1 {
		return resumeHeadValidV1(h)
	}
	if h.Checkpoint != nil || h.Native != nil && h.Native.Schema != "lsp-trace.inventory-native-claim.v1" || h.Selected != nil && h.Selected.Schema != "lsp-trace.inventory-selected-claim.v1" {
		return false
	}
	if h.Schema == is.InventorySelectedHeadVersionV1 {
		return selectedInventoryHeadValidV1(h)
	}
	if h.Selected != nil {
		return false
	}
	if h.Schema == is.InventoryNativeHeadVersionV1 {
		return ownedNativeHeadValid(h)
	}
	if h.Native != nil {
		return false
	}
	if h.Phase == "Creating" {
		return ownedCreationValid(h)
	}
	if h.Creation != nil {
		return false
	}
	if h.Enumeration != nil && !enumerationHeadValid(h) {
		return false
	}
	if h.Enumeration == nil && (h.Phase == "Enumerating" || h.Phase == "ReadyForAcquisition" || h.Phase == "EnumerationPartial") {
		return false
	}
	if h.Issued <= 0 || h.OutputBytes > 2<<30 || h.Input.Bytes > 16384 || h.PreviousTaskID != "" || h.InputPrefix != "" || h.Role != "" || h.Actor != "" || h.Acceptance != "" || h.AcceptanceDeadline != 0 || h.Window != 0 || h.SemanticNS != 0 || h.Completion.Version != "" || len(h.Completion.Stages) != 0 || h.CompletionCursor != 0 || h.Delivery != (inventoryDelivery{}) || h.DeliverySpent != 0 || h.SuccessorReserve != 0 || h.SuccessorPackets != 0 || h.Report != (publication.StoredObject{}) || h.Manifest != (publication.StoredObject{}) {
		return false
	}
	if (h.Status == "READY" || h.Status == "PARTIAL") && (h.TaskID != "" || h.Deadline != 0 || h.Prompt != "") {
		return false
	}
	return ownedPhase(h) && validDurableID(h.RunID) && h.State.Valid() && h.State.Bytes <= 32768 && h.RunDeadline == 0 && h.Sequence >= 0 && h.Sequence <= 16 && h.Used >= ownedCreateMinimum && h.Used <= h.Limit && h.Limit <= durableRunLimit && h.Work == h.Used*32 && h.Packets >= 1 && h.Packets <= 64 && (h.Status == "READY" && (h.Phase == "Collecting" || h.Phase == "Enumerating" || h.Phase == "ReadyForAcquisition") || h.Status == "PARTIAL" && h.Phase == "EnumerationPartial" || h.Status == "CLAIMED") && (h.Sequence == 0 && h.Input == (publication.StoredObject{}) || h.Sequence > 0 && h.Input.Valid()) && (h.Status != "CLAIMED" || validDurableID(h.TaskID) && h.Deadline > 0) && len(h.Prompt) <= 16384
}
func (s DurableStore) ownedConfig(h durableInventoryHead) (OwnedInventoryConfig, error) {
	var c OwnedInventoryConfig
	raw, e := publication.ReadObject(s.Root, "inventory-owned-config-", h.State)
	if e != nil || strictInventory(raw, &c) != nil || !ownedConfigValid(c) || c.BudgetBytes != h.Limit {
		return c, ErrDurableInventory
	}
	return c, nil
}

// Create first commits a fully funded creation intent in this same owner's
// head. A returned error may accompany a discoverable RunID and spent status.
func (s DurableStore) Create(ctx context.Context, c OwnedInventoryConfig) (InventoryJobStatus, error) {
	return s.createOwned(ctx, c, "", nil)
}

// JobStatus is bounded non-executing inspection, with no charge or lease renewal.
// CLAIMED is PARTIAL_UNCERTAIN to a reopening observer, not a request to poll
// or retry. Only the already-running one-shot operation can still commit its
// exact predecessor; once lost, there is no recovery/replay transition.
func (s DurableStore) JobStatus(ctx context.Context, run string) (result InventoryJobStatus, err error) {
	if ctx == nil {
		return result, ErrDurableInventory
	}
	started := time.Now()
	ctx, cancel := context.WithDeadline(ctx, started.Add(30*time.Second))
	defer cancel()
	defer func() {
		if e := ctx.Err(); e != nil {
			result = InventoryJobStatus{}
			err = e
		}
	}()
	h, raw, e := s.headContext(ctx, run)
	if e != nil {
		return InventoryJobStatus{}, e
	}
	if ctx == nil || ctx.Err() != nil || (h.Schema != ownedHeadVersion && h.Schema != is.InventoryNativeHeadVersionV1 && h.Schema != is.InventorySelectedHeadVersionV1 && h.Schema != is.InventoryResumeHeadVersionV1) {
		return InventoryJobStatus{}, ErrDurableInventory
	}
	status := h.Status
	if status == "CLAIMED" {
		status = "PARTIAL_UNCERTAIN"
	}
	task := h.TaskID
	if ownedPhase(h) {
		task = ""
	}
	count := h.Sequence
	if !ownedPhase(h) {
		count = len(h.Completion.Inputs)
		t, err := s.Task(ctx, run, h.TaskID)
		if err != nil {
			return InventoryJobStatus{}, err
		}
		status = t.Status
	}
	result = InventoryJobStatus{Schema: "lsp-trace.inventory-job-status.v1", RunID: run, Head: cat.NativeDigestV3(raw), Phase: h.Phase, Status: status, JobExpiry: "NONE", Used: h.Used, Limit: h.Limit, Packets: h.Packets, Admissions: count, TaskID: task}
	if h.Enumeration != nil {
		result.Schema = "lsp-trace.inventory-job-status.v2"
		var enumeration InventoryEnumerationStatus
		enumeration, err = s.inspectEnumerationAt(ctx, h, started)
		if err != nil {
			return InventoryJobStatus{}, err
		}
		result.Enumeration = &enumeration
	}
	if h.Schema == is.InventorySelectedHeadVersionV1 || h.Schema == is.InventoryResumeHeadVersionV1 {
		result.Selected = h.Selected
		result.Checkpoint = h.Checkpoint
	}
	if h.Schema == is.InventoryNativeHeadVersionV1 || h.Schema == is.InventorySelectedHeadVersionV1 || h.Schema == is.InventoryResumeHeadVersionV1 {
		result.Schema = "lsp-trace.inventory-job-status.v3"
		result.Native = h.Native
	}
	return result, nil
}

// ReserveAdmission binds the monotonic index and expected source/request IDs
// before effects. These DTO identities are constraints, never admission authority.
// Exact predecessor CAS, not automatic retry/rebase, serializes competing calls.
func (s DurableStore) ReserveAdmission(ctx context.Context, run, expectedHead string, binding InventoryAdmissionBinding) (AdmissionReservation, error) {
	h, old, e := s.head(run)
	if e != nil {
		return AdmissionReservation{}, e
	}
	if ctx == nil || ctx.Err() != nil || h.Schema != ownedHeadVersion || !ownedPhase(h) || (h.Phase != "Collecting" && h.Phase != "ReadyForAcquisition") || h.Status != "READY" || cat.NativeDigestV3(old) != expectedHead || binding.Index != h.Sequence || len(binding.Provenance) > 4096 || len(binding.RequestID) == 0 || len(binding.RequestID) > 256 || len(binding.SourceUnitID) == 0 || len(binding.SourceUnitID) > 256 {
		return AdmissionReservation{}, ErrDurableInventory
	}
	c, e := s.ownedConfig(h)
	if e != nil {
		return AdmissionReservation{}, e
	}
	if c.Interpreter == "" {
		return AdmissionReservation{}, ErrDurableInventory
	}
	if h.Enumeration != nil {
		verified, err := s.inspectEnumeration(ctx, h)
		if err != nil || !verified.Closed {
			return AdmissionReservation{}, ErrDurableInventory
		}
	}
	if h.Sequence >= c.MaxAdmissions || c.AdmissionBytes+ownedSealCost > h.Limit-h.Used || h.Packets >= 60 {
		return AdmissionReservation{}, ErrDurableInventory
	}
	raw, e := json.Marshal(binding)
	if e != nil {
		return AdmissionReservation{}, e
	}
	h.TaskID, e = newDurableID()
	if e != nil {
		return AdmissionReservation{}, e
	}
	h.Prompt = string(raw)
	h.Phase = "Collecting"
	h.Status = "CLAIMED"
	h.Deadline = time.Now().Add(c.OperationWindow).UnixNano()
	h.Used += c.AdmissionBytes
	h.Work = h.Used * 32
	h.Packets++
	ctx, cancel := context.WithDeadline(ctx, time.Unix(0, h.Deadline))
	defer cancel()
	claimed, e := s.change(ctx, old, h)
	if e != nil {
		return AdmissionReservation{}, e
	}
	return AdmissionReservation{&ownedAdmissionOperation{store: s, head: h, predecessor: claimed, config: c, binding: binding}}, nil
}

// Admit consumes the existing opaque original exactly once through retention.
// A failed/cancelled/lost commit leaves all reserved work spent and the head
// terminal partial-uncertain; orphan immutable bytes never enter the manifest.
func (s DurableStore) Admit(ctx context.Context, op AdmissionReservation, r sc.PrivateSelectedResultV2) (InventoryJobStatus, error) {
	if op.state == nil || op.state.store != s || !op.state.used.CompareAndSwap(false, true) {
		return InventoryJobStatus{}, ErrDurableInventory
	}
	o := op.state
	h := o.head
	if ctx == nil {
		return InventoryJobStatus{}, ErrDurableInventory
	}
	ctx, cancel := context.WithDeadline(ctx, time.Unix(0, h.Deadline))
	defer cancel()
	var b *cat.CallerBudgetV3
	var e error
	if h.Schema == is.InventorySelectedHeadVersionV1 || h.Schema == is.InventoryResumeHeadVersionV1 {
		if h.Selected == nil || h.Selected.Outcome != "OBSERVED" || !r.InventoryAccountingMatchesV1(h.Selected.Accounting) || time.Now().UnixNano() >= h.Selected.Deadline {
			return InventoryJobStatus{}, ErrDurableInventory
		}
		if h.Schema == is.InventoryResumeHeadVersionV1 {
			b, e = cat.NewInventoryAdmissionBudgetV5(time.Unix(0, h.Deadline), o.config.AdmissionBytes, h.Used-o.config.AdmissionBytes)
		} else {
			b, e = cat.NewInventoryAdmissionBudgetV4(time.Unix(0, h.Deadline), o.config.AdmissionBytes, h.Used-o.config.AdmissionBytes)
		}
	} else {
		b, e = cat.NewReservedCallerBudgetV3(time.Unix(0, h.Deadline), o.config.AdmissionBytes, h.Used-o.config.AdmissionBytes, 0)
	}
	if e != nil {
		return InventoryJobStatus{}, e
	}
	_, u, q, ok := r.Selected()
	if !ok || u.ID != o.binding.SourceUnitID || q.RecordID != o.binding.RequestID {
		return InventoryJobStatus{}, ErrDurableInventory
	}
	// The exact immutable retained bytes still pass the intrinsic admission,
	// matching unit/request and original readback guards inside retention.
	pins := o.config.Pins
	pins.PromptSHA256 = "sha256:" + q.Envelope.InputSHA256
	lead := fmt.Sprintf("%s-%06d", h.RunID, o.binding.Index)
	ref, payload, e := sc.RetainInventoryEvidence(ctx, s.Root, h.RunID, lead, r, sc.CallerDescribeBindingV3{AttemptID: lead, Interpreter: o.config.Interpreter, Pins: pins, Deadline: time.Unix(0, h.Deadline), Limits: o.config.ResponseLimits, Budget: b})
	if e != nil {
		return InventoryJobStatus{}, e
	}
	if h.Schema == is.InventoryResumeHeadVersionV1 && ref.Bytes > 8<<20 {
		return InventoryJobStatus{}, ErrDurableInventory
	}
	retained := h.OutputBytes + uint64(ref.Bytes)
	// Acceptance check AFTER retention, never a pre-effect disk quota.
	if retained > o.config.MaxAdmittedEvidenceBytes {
		return InventoryJobStatus{}, ErrDurableInventory
	}
	node := ownedAdmissionRecord{Schema: "lsp-trace.inventory-admission.v2", RunID: h.RunID, LeadID: lead, Index: h.Sequence, Previous: h.Input, Evidence: ref, Binding: o.binding, Payload: payload, AdmittedEvidenceBytes: retained}
	raw, e := json.Marshal(node)
	if e != nil || b.Reserve(ctx, uint64(len(raw))*16+32768) != nil {
		return InventoryJobStatus{}, ErrDurableInventory
	}
	record, e := publication.StoreObject(s.Root, "inventory-admission-", raw)
	if e != nil {
		return InventoryJobStatus{}, e
	}
	h.Input = record
	h.OutputBytes = retained
	h.Sequence++
	h.Status = "READY"
	h.TaskID = ""
	h.Prompt = ""
	h.Deadline = 0
	if h.Schema == is.InventorySelectedHeadVersionV1 {
		h.Phase = "Admitted"
		h.Status = "PARTIAL"
	}
	if h.Schema == is.InventoryResumeHeadVersionV1 {
		h.Phase, h.Status = "Admitted", "PARTIAL"
		if h.Sequence == 1 {
			cp := &is.InventoryCheckpointV1{Native: *h.Native, Selected: *h.Selected, Admission: record, Evidence: ref, Used: h.Used, OutputBytes: retained}
			if is.VerifyInventoryCheckpointV1(ctx, s.Root, h.RunID, *cp) != nil {
				return InventoryJobStatus{}, ErrDurableInventory
			}
			h.Checkpoint = cp
			h.Native, h.Selected = nil, nil
			h.Phase, h.Status = "ReadyForAcquisition", "READY"
		}
	}
	if _, e = s.change(ctx, o.predecessor, h); e != nil {
		return InventoryJobStatus{}, e
	}
	return s.JobStatus(ctx, h.RunID)
}

// Seal is the bounded bridge into the existing semantic state and controller,
// not a second run. The initial work slices remain spent in the same head.
func (s DurableStore) Seal(ctx context.Context, run, expectedHead string) (DurableTask, error) {
	bad := func(e error) (DurableTask, error) { return DurableTask{}, e }
	h, old, e := s.head(run)
	if e != nil {
		return bad(e)
	}
	if ctx == nil || ctx.Err() != nil || h.Schema == is.InventorySelectedHeadVersionV1 || !ownedPhase(h) || h.Phase != "Collecting" || h.Status != "READY" || h.Sequence < 1 || cat.NativeDigestV3(old) != expectedHead || ownedSealCost > h.Limit-h.Used {
		return bad(ErrDurableInventory)
	}
	c, e := s.ownedConfig(h)
	if e != nil {
		return bad(e)
	}
	if h.Enumeration != nil {
		verified, err := s.inspectEnumeration(ctx, h)
		if err != nil || !verified.Closed {
			return bad(ErrDurableInventory)
		}
	}
	count := h.Sequence
	last := h.Input
	h.Phase = "Sealing"
	h.Status = "CLAIMED"
	h.TaskID, e = newDurableID()
	if e != nil {
		return bad(e)
	}
	h.Deadline = time.Now().Add(c.OperationWindow).UnixNano()
	h.Used += ownedSealCost
	h.Work = h.Used * 32
	h.Packets++
	ctx, cancel := context.WithDeadline(ctx, time.Unix(0, h.Deadline))
	defer cancel()
	claimed, e := s.change(ctx, old, h)
	if e != nil {
		return bad(e)
	}
	b, e := cat.NewReservedCallerBudgetV3(time.Unix(0, h.Deadline), ownedSealCost, h.Used-ownedSealCost, 0)
	if e != nil {
		return bad(e)
	}
	if b.Reserve(ctx, uint64(count)*32768+32768) != nil {
		return bad(ErrDurableInventory)
	}
	leads := make([]Lead, count)
	refs := make([]publication.StoredObject, count)
	payloads := make([]uint64, count)
	retained := h.OutputBytes
	for i := count - 1; i >= 0; i-- {
		if !last.Valid() || last.Bytes > 16384 {
			return bad(ErrDurableInventory)
		}
		raw, e := publication.ReadObject(s.Root, "inventory-admission-", last)
		var n ownedAdmissionRecord
		if e != nil || strictInventory(raw, &n) != nil || n.Schema != "lsp-trace.inventory-admission.v2" || n.RunID != run || n.Index != i || n.Binding.Index != i || n.LeadID != fmt.Sprintf("%s-%06d", run, i) || !n.Evidence.Valid() || n.Payload == 0 || n.AdmittedEvidenceBytes != retained || uint64(n.Evidence.Bytes) > retained {
			return bad(ErrDurableInventory)
		}
		retained -= uint64(n.Evidence.Bytes)
		leads[i] = Lead{n.LeadID, n.Binding.Provenance}
		refs[i] = n.Evidence
		payloads[i] = n.Payload
		last = n.Previous
	}
	if last != (publication.StoredObject{}) || retained != 0 {
		return bad(ErrDurableInventory)
	}
	report := initialInventoryReport()
	report.ReviewMode = c.Policy.ReviewMode
	gaps := []string{"SUPPLIED_INCREMENTAL_ADMISSIONS_NOT_ENUMERATION"}
	if h.Enumeration != nil {
		gaps = []string{"ENUMERATED_METADATA_REQUIRES_NATIVE_REVALIDATION_AND_COVERAGE"}
	}
	report, cost, e := admitInventoryManifest(ctx, Request{Manifest: Manifest{Scope: c.Scope, Provenance: c.Provenance, Leads: leads, UntouchedGaps: gaps}, MaxNativeAttempts: count}, b, report)
	if e != nil {
		return bad(e)
	}
	for i := range leads {
		report.Dispositions[i] = Disposition{leads[i].ID, "PREPARED", "DESCRIBE_PENDING"}
		recordInventoryStage(&report, "native_preparation", true)
	}
	v := durableInventoryState{Schema: "lsp-trace.inventory-state.v2", RunID: run, Interpreter: c.Interpreter, Reviewer: c.Policy.Reviewer, Policy: c.Policy, Report: report, Prepared: refs, Descriptions: []publication.StoredObject{}, ManifestCost: cost}
	h.State, e = s.saveState(ctx, v, b)
	if e != nil {
		return bad(e)
	}
	h.Phase = "Describe"
	h.Sequence = 0
	h.TaskID = ""
	h.Prompt = ""
	h.OutputBytes = 0
	h.Window = int64(c.TaskWindow)
	if b.Reserve(ctx, durablePlanningQuote(count)) != nil {
		return bad(ErrDurableInventory)
	}
	if e = s.issueNext(&h, v); e != nil {
		return bad(e)
	}
	if e = s.buildDelivery(ctx, &h, b); e != nil {
		return bad(e)
	}
	h.Completion, e = quoteDurableCompletion(refs, h.State.Bytes, cost, c.ResponseLimits, payloads)
	h.Completion.PreparationUsed = h.Used
	if e != nil {
		return bad(e)
	}
	h.SuccessorReserve, h.SuccessorPackets, e = successorEscrow(h)
	if e != nil {
		return bad(e)
	}
	if e = h.startCapacityRefusal(); e != nil {
		return bad(e)
	}
	h.Acceptance = "inventory-accepted-" + run + "-" + h.TaskID
	h.AcceptanceDeadline = h.Deadline
	if _, e = s.change(ctx, claimed, h); e != nil {
		return bad(e)
	}
	if e = publication.PublishTimedCompletion(ctx, s.Root, h.Acceptance, inventoryAcceptance(h), time.Unix(0, h.AcceptanceDeadline)); e != nil {
		return bad(e)
	}
	return s.Task(ctx, run, "")
}
