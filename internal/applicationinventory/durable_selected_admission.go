//go:build darwin || linux

package applicationinventory

import (
	"context"
	is "lsp-trace/internal/inventorysuccessor"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/scopejob"
)

// SelectedAdmissionConfigV1 is a NEW finite initial profile. The existing
// semantic configuration supplies declared retention metadata only; Seal and
// semantic tasks are unavailable. Existing configs/jobs are never upgraded.
func SelectedAdmissionConfigV1(enumeration scopejob.Config, admission OwnedInventoryConfig) (OwnedInventoryConfig, error) {
	base, e := NativeBoundaryConfigV1(enumeration)
	if e != nil {
		return OwnedInventoryConfig{}, e
	}
	p := is.NewInventorySelectedProfileV1()
	admission.Schema = is.InventorySelectedConfigVersionV1
	admission.Scope = base.Scope
	admission.Enumeration = base.Enumeration
	admission.ResourcePolicy = base.ResourcePolicy
	admission.NativeProfile = base.NativeProfile
	admission.SelectedProfile = &p
	admission.BudgetBytes = p.ParentLimit
	admission.AdmissionBytes = p.AdmissionBytes
	admission.MaxAdmissions = 1
	admission.OperationWindow = base.OperationWindow
	if admission.MaxAdmittedEvidenceBytes == 0 {
		admission.MaxAdmittedEvidenceBytes = 2 << 30
	}
	if !selectedInventoryConfigValidV1(admission) {
		return OwnedInventoryConfig{}, ErrDurableInventory
	}
	return admission, nil
}
func selectedInventoryConfigValidV1(c OwnedInventoryConfig) bool {
	if c.Schema != is.InventorySelectedConfigVersionV1 || c.SelectedProfile == nil || !c.SelectedProfile.Valid() || c.NativeProfile == nil || !c.NativeProfile.Valid() || c.Enumeration == nil || c.Enumeration.Budget.Steps > scopejob.BatchSteps || c.Enumeration.Budget.LogicalBytes != durableRunLimit || c.BudgetBytes != cat.InventoryAdmissionLimitV4 || c.AdmissionBytes != cat.InventoryAdmissionBytesV4 || c.MaxAdmissions != 1 || c.Interpreter == "" {
		return false
	}
	c.Schema = ownedConfigVersion
	c.SelectedProfile = nil
	c.NativeProfile = nil
	c.BudgetBytes = durableRunLimit
	return ownedConfigValid(c)
}
func selectedInventoryHeadValidV1(h durableInventoryHead) bool {
	if h.Schema != is.InventorySelectedHeadVersionV1 || h.Limit != cat.InventoryAdmissionLimitV4 {
		return false
	}
	if h.Phase == "Creating" {
		return ownedCreationValid(h)
	}
	if h.Selected != nil {
		c := h.Selected
		q, e := ownedCreationQuote(h.State.Bytes)
		if e != nil || !c.Valid() || c.RunID != h.RunID || h.Native == nil || h.Native.Outcome != "OBSERVED" || c.Deadline != h.Native.Binding.Deadline.UnixNano() || h.Used != q+enumerationFixed+scopejob.BatchBytes+is.InventoryNativeControlBytesV1+cat.InventorySelectedControlBytesV1+cat.InventoryAdmissionBytesV4 || h.Work != h.Used*32 || h.Packets != 6 {
			return false
		}
		switch h.Phase {
		case "SelectedAdmission":
			if c.Outcome != "CLAIMED" || h.Status != "CLAIMED" || h.Deadline != c.Deadline || !validDurableID(h.TaskID) || h.Sequence != 0 || h.Input != (publication.StoredObject{}) || h.OutputBytes != 0 || h.Prompt != "" {
				return false
			}
		case "Admitted":
			if c.Outcome != "OBSERVED" || h.Status != "PARTIAL" || h.TaskID != "" || h.Deadline != 0 || h.Sequence != 1 || !h.Input.Valid() || h.Input.Bytes > 16384 || h.OutputBytes == 0 || h.OutputBytes > 2<<30 || h.Prompt != "" {
				return false
			}
		default:
			return false
		}
		h.Selected = nil
		h.Phase = "NativeAcquisition"
		h.Status = "PARTIAL"
		h.Deadline = 0
		h.TaskID = ""
		h.Sequence = 0
		h.Input = publication.StoredObject{}
		h.OutputBytes = 0
		h.Used -= cat.InventorySelectedControlBytesV1 + cat.InventoryAdmissionBytesV4
		h.Work = h.Used * 32
		h.Packets--
	} else {
		switch h.Phase {
		case "Enumerating", "EnumerationPartial", "ReadyForAcquisition", "NativeAcquisition":
		default:
			return false
		}
	}
	// Validation projection only; persisted spending/format are never rewritten.
	h.Schema = is.InventoryNativeHeadVersionV1
	h.Limit = durableRunLimit
	return ownedNativeHeadValid(h)
}

// ContinueSelectedAdmissionV1 owns the whole continuation. No callback or native
// handle escape can substitute M/P/S or create an AdmissionReservation. The
// original deadline covers initialization, all three stages, cleanup and Admit.
func (s DurableStore) ContinueSelectedAdmissionV1(ctx context.Context, run, expected string, discovery is.InventoryFundedDiscoveryV1, index int, config is.PrivateSelectedConfigurationV2) (InventoryJobStatus, error) {
	return s.continueSelectedAdmissionV1(ctx, run, expected, discovery, index, config, nil)
}

// fault is a private deterministic test seam, never supplied by the public API.
func (s DurableStore) continueSelectedAdmissionV1(ctx context.Context, run, expected string, discovery is.InventoryFundedDiscoveryV1, index int, config is.PrivateSelectedConfigurationV2, fault func(string) error) (InventoryJobStatus, error) {
	fail := func(e error) (InventoryJobStatus, error) { return InventoryJobStatus{}, e }
	if ctx == nil || ctx.Err() != nil {
		return fail(ErrDurableInventory)
	}
	h, old, e := s.headContext(ctx, run)
	if e != nil || (h.Schema != is.InventorySelectedHeadVersionV1 && !is.InventoryResumableHeadV1(h.Schema)) || h.Native == nil || h.Native.Outcome != "OBSERVED" || h.Selected != nil || h.Status != "PARTIAL" || h.Phase != "NativeAcquisition" || cat.NativeDigestV3(old) != expected || index < 0 || index >= 128 || config.RunID != run || config.ExpectedSourceAuthorization != h.Native.Binding.DenominatorDigest {
		return fail(ErrDurableInventory)
	}
	ctx, cancel := context.WithDeadline(ctx, h.Native.Binding.Deadline)
	defer cancel()
	c, e := s.ownedConfig(h)
	if e != nil {
		return fail(e)
	}
	digest, e := is.InventorySelectedConfigurationDigestV1(config)
	if e != nil {
		return fail(e)
	}
	if ctx.Err() != nil || is.InventorySelectedControlForHeadV1(h.Schema) > h.Limit-h.Used {
		return fail(ErrDurableInventory)
	}
	h.Selected = &is.InventorySelectedClaimV1{Schema: "lsp-trace.inventory-selected-claim.v1", RunID: run, ParentHead: expected, Namespace: "inventory-selected-" + run + "-000000", Configuration: digest, Index: index, Deadline: h.Native.Binding.Deadline.UnixNano(), Profile: *c.SelectedProfile, Allocated: c.SelectedProfile.Allocation, Outcome: "CLAIMED"}
	if is.InventoryResumableHeadV1(h.Schema) {
		h.Selected.Schema = "lsp-trace.inventory-selected-claim.v2"
		h.Selected.Ordinal = h.Native.Ordinal
		h.Selected.Namespace = is.InventoryNamespaceV1("selected", run, h.Native.Ordinal)
	}
	h.Phase = "SelectedAdmission"
	h.Status = "CLAIMED"
	h.Deadline = h.Selected.Deadline
	h.TaskID, e = newDurableID()
	if e != nil {
		return fail(e)
	}
	h.Used += is.InventorySelectedControlForHeadV1(h.Schema)
	h.Work = h.Used * 32
	h.Packets++
	raw, e := encodeInventoryHead(h)
	if e != nil || len(raw) > inventoryHeadLimit(h) || !h.completionValid() {
		return fail(ErrDurableInventory)
	}
	proof, e := publication.CommitBoundFile(ctx, s.Root, inventoryHeadName(run), old, raw)
	if e != nil {
		return fail(e)
	}
	selected, e := is.OpenInventoryFundedSelectedV1(ctx, proof, discovery, config)
	if e != nil {
		return fail(e)
	}
	// On every failure, close the initialized root. No failure cleanup creates a
	// parent completion or refund; the persisted CLAIMED state stays uncertain.
	defer selected.Complete(ctx)
	var result sc.PrivateSelectedResultV2
	e = selected.Execute(ctx, func(ctx context.Context, source is.PrivateSelectedSourceOperationV2) error {
		if fault != nil {
			if err := fault("before-source"); err != nil {
				return err
			}
		}
		// Closed R/query-side role: only an actually admitted row passes this filter.
		var err error
		result, err = sc.PrepareSelectedRoleV3(ctx, source, sc.PrivateSelectedRoleV3{Kind: "REFERENCES_SYMBOL", Role: "REFERENCED_SYMBOL"})
		return err
	})
	if e != nil {
		return fail(e)
	}
	accounting, e := selected.Complete(ctx)
	if e != nil {
		return fail(e)
	}
	if !result.InventoryAccountingMatchesV1(accounting) || ctx.Err() != nil {
		return fail(ErrDurableInventory)
	}
	_, unit, request, ok := result.Selected()
	if !ok {
		return fail(ErrDurableInventory)
	}
	h.Selected.Outcome = "OBSERVED"
	h.Selected.Accounting = accounting
	binding := InventoryAdmissionBinding{Index: h.Sequence, Provenance: c.Provenance, SourceUnitID: unit.ID, RequestID: request.RecordID}
	// Admission was prepaid in the selected CAS. This is not ReserveAdmission,
	// which remains closed to v6 callers; Admit uses exactly this private operation.
	op := AdmissionReservation{&ownedAdmissionOperation{store: s, head: h, predecessor: raw, config: c, binding: binding}}
	if fault != nil {
		if err := fault("before-admit"); err != nil {
			return fail(err)
		}
	}
	return s.Admit(ctx, op, result)
}
