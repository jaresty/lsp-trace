//go:build darwin || linux

package applicationinventory

import (
	"context"
	is "lsp-trace/internal/inventorysuccessor"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/scopejob"
)

// ResumableAdmissionConfigV1 is a new initial profile, not an upgrade. Only
// frozen 17-member one-segment jobs are eligible for the two native batches.
func ResumableAdmissionConfigV1(enumeration scopejob.Config, admission OwnedInventoryConfig) (OwnedInventoryConfig, error) {
	c, e := SelectedAdmissionConfigV1(enumeration, admission)
	if e != nil {
		return c, e
	}
	c.Schema = is.InventoryResumeConfigVersionV1
	c.BudgetBytes = cat.InventoryResumeLimitV1
	c.MaxAdmissions = 2
	profile := is.NewInventoryResumeProfileV1()
	c.ResumeProfile = &profile
	if !resumeConfigValidV1(c) {
		return OwnedInventoryConfig{}, ErrDurableInventory
	}
	return c, nil
}
func resumeConfigValidV1(c OwnedInventoryConfig) bool {
	if c.Schema != is.InventoryResumeConfigVersionV1 || c.BudgetBytes != cat.InventoryResumeLimitV1 || c.MaxAdmissions != 2 || c.ResumeProfile == nil || !c.ResumeProfile.Valid() {
		return false
	}
	c.ResumeProfile = nil
	c.Schema = is.InventorySelectedConfigVersionV1
	c.BudgetBytes = cat.InventoryAdmissionLimitV4
	c.MaxAdmissions = 1
	return selectedInventoryConfigValidV1(c)
}
func resumeHeadValidV1(h durableInventoryHead) bool {
	if (h.Schema != is.InventoryResumeHeadVersionV1 && !is.InventoryReferenceHead(h.Schema)) || h.Limit != h.acquisitionLimit() || h.Used > h.Limit || h.Work != h.Used*32 {
		return false
	}
	if h.Phase == "Creating" {
		return ownedCreationValid(h)
	}
	if h.Phase == "Admitted" && h.Checkpoint == nil {
		return false
	}
	ordinal := 0
	if h.Checkpoint == nil && h.Phase != "Admitted" && (h.Input != (publication.StoredObject{}) || h.OutputBytes != 0) {
		return false
	}
	if h.Checkpoint != nil {
		c := h.Checkpoint
		q, e := ownedCreationQuote(h.State.Bytes)
		if e != nil || !c.Valid(h.RunID) || h.Enumeration == nil || c.Native.Binding.DenominatorDigest != "sha256:"+h.Enumeration.Last || c.Native.Policy != h.Enumeration.Policy+"/"+h.State.ID || c.Used != q+enumerationFixed+scopejob.BatchBytes+is.InventoryNativeControlForHeadV1(h.Schema)+is.InventorySelectedControlForHeadV1(h.Schema) {
			return false
		}
		ordinal = 1
		if h.Phase != "Admitted" && (h.Input != c.Admission || h.OutputBytes != c.OutputBytes || h.Sequence != 1) {
			return false
		}
		if h.Phase == "Admitted" && (h.Sequence != 2 || h.Input == c.Admission || h.OutputBytes <= c.OutputBytes) {
			return false
		}
		if h.Used < c.Used || h.Packets < 6 {
			return false
		}
		h.Used -= is.InventoryNativeControlForHeadV1(h.Schema) + is.InventorySelectedControlForHeadV1(h.Schema)
		h.Packets -= 2
		h.Sequence--
		h.Checkpoint = nil
	}
	if h.Native != nil {
		n := *h.Native
		if !n.Valid() || n.Schema != "lsp-trace.inventory-native-claim.v2" || n.Ordinal != ordinal {
			return false
		}
		n.Schema = "lsp-trace.inventory-native-claim.v1"
		n.Ordinal, n.Binding.Ordinal = 0, 0
		n.ChildNamespace = is.InventoryNamespaceV1("native", h.RunID, 0)
		h.Native = &n
		if h.Used < cat.InventoryNativeControlBytesV2-is.InventoryNativeControlBytesV1 {
			return false
		}
		h.Used -= cat.InventoryNativeControlBytesV2 - is.InventoryNativeControlBytesV1
	}
	if h.Selected != nil {
		c := *h.Selected
		if !c.Valid() || c.Schema != "lsp-trace.inventory-selected-claim.v2" || c.Ordinal != ordinal {
			return false
		}
		c.Schema = "lsp-trace.inventory-selected-claim.v1"
		c.Ordinal = 0
		c.Namespace = is.InventoryNamespaceV1("selected", h.RunID, 0)
		h.Selected = &c
		if h.Used < cat.InventoryCheckpointBytesV1 {
			return false
		}
		h.Used -= cat.InventoryCheckpointBytesV1
		if is.InventoryReferenceHead(h.Schema) {
			if h.Used < cat.InventoryReferenceRetentionV1 {
				return false
			}
			h.Used -= cat.InventoryReferenceRetentionV1
		}
	}
	if ordinal == 1 && h.Phase != "Admitted" {
		h.Input = publication.StoredObject{}
		h.OutputBytes = 0
	}
	h.Schema = is.InventorySelectedHeadVersionV1
	h.Limit = cat.InventoryAdmissionLimitV4
	h.Work = h.Used * 32
	return selectedInventoryHeadValidV1(h)
}

func (s DurableStore) verifyResumeCheckpointV1(ctx context.Context, h durableInventoryHead) error {
	if !is.InventoryResumableHeadV1(h.Schema) {
		return ErrDurableInventory
	}
	if h.Sequence == 0 && h.Checkpoint == nil {
		return nil
	}
	if h.Sequence != 1 || h.Checkpoint == nil || h.Input != h.Checkpoint.Admission || h.OutputBytes != h.Checkpoint.OutputBytes {
		return ErrDurableInventory
	}
	return is.VerifyInventoryCheckpointForHeadV1(ctx, s.Root, h.Schema, h.RunID, *h.Checkpoint)
}
