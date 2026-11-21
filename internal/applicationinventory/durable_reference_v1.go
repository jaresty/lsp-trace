//go:build darwin || linux

package applicationinventory

import (
	is "lsp-trace/internal/inventorysuccessor"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/scopejob"
)

// Initial-only V9. Never upgrades v8 or reinterprets its stored evidence.
func ReferenceAdmissionConfigV1(e scopejob.Config, a OwnedInventoryConfig) (OwnedInventoryConfig, error) {
	c, err := ConnectedAdmissionConfigV1(e, a)
	if err != nil {
		return c, err
	}
	c.Schema = is.InventoryReferenceConfigVersionV1
	c.BudgetBytes = cat.InventoryReferenceLimitV1
	c.AdmissionBytes = cat.InventoryReferenceAdmissionV1
	p := is.NewInventoryReferenceProfileV1()
	c.ResumeProfile = &p
	if !referenceConfigValid(c) {
		return OwnedInventoryConfig{}, ErrDurableInventory
	}
	return c, nil
}
func referenceConfigValid(c OwnedInventoryConfig) bool {
	if c.Schema != is.InventoryReferenceConfigVersionV1 || c.BudgetBytes != cat.InventoryReferenceLimitV1 || c.AdmissionBytes != cat.InventoryReferenceAdmissionV1 || c.ResumeProfile == nil || *c.ResumeProfile != is.NewInventoryReferenceProfileV1() {
		return false
	}
	c.Schema = is.InventoryConnectedConfigVersionV1
	c.BudgetBytes = cat.InventoryConnectedLimitV1
	c.AdmissionBytes = cat.InventoryAdmissionBytesV4
	p := is.NewInventoryConnectedProfileV1()
	c.ResumeProfile = &p
	return connectedConfigValid(c)
}
func (h durableInventoryHead) connectedLimit() uint64 {
	if h.Schema == is.InventoryReferenceHeadVersionV1 {
		return cat.InventoryReferenceLimitV1
	}
	return cat.InventoryConnectedLimitV1
}
func (h durableInventoryHead) acquisitionLimit() uint64 {
	if h.Schema == is.InventoryReferenceHeadVersionV1 {
		return cat.InventoryReferenceAcquisitionV1
	}
	return cat.InventoryResumeLimitV1
}
func (h durableInventoryHead) connectedSealBytes() uint64 {
	if h.Schema == is.InventoryReferenceHeadVersionV1 {
		return cat.InventoryReferenceSealV1
	}
	return cat.InventoryConnectedSealBytesV1
}
func (h durableInventoryHead) manifestLimit() uint64 {
	if h.Schema == is.InventoryReferenceHeadVersionV1 {
		return cat.InventoryReferenceManifestV1
	}
	return cat.InventoryConnectedManifestV1
}
func (h durableInventoryHead) admissionSchema() string {
	if h.Schema == is.InventoryReferenceHeadVersionV1 {
		return "lsp-trace.inventory-admission.v3"
	}
	return "lsp-trace.inventory-admission.v2"
}
func (h durableInventoryHead) evidenceSchema() string {
	if h.Schema == is.InventoryReferenceHeadVersionV1 {
		return "lsp-trace.inventory-evidence.v2"
	}
	return "lsp-trace.inventory-evidence.v1"
}

// Called on each newly derived plan, never a persisted/spent plan. Existing
// submitPlan preserves all spent stages and the current stage without refunds.
func (h durableInventoryHead) referenceCompletion(p *durableCompletionPlan) {
	if h.Schema != is.InventoryReferenceHeadVersionV1 || len(p.Stages) != 5 {
		return
	}
	p.Stages[0] += cat.InventoryReferenceDescribeV1
	p.Stages[1] += cat.InventoryReferenceDescribeV1 + cat.InventoryReferenceReopenV1
	p.Stages[3] += cat.InventoryReferenceReopenV1
}
