//go:build darwin || linux

package applicationinventory

import (
	is "lsp-trace/internal/inventorysuccessor"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/scopejob"
	"math"
	"time"
)

// SemanticAdmissionConfigV1 is initial-only: no stored job mutation/defaulting.
// Admission supplies the existing native/source configuration; p exclusively
// selects the new task lease. Historical constructors retain 30s/120s limits.
func SemanticAdmissionConfigV1(e scopejob.Config, a OwnedInventoryConfig, p cat.InventorySemanticPolicyV1) (OwnedInventoryConfig, error) {
	if !p.Valid() || a.SemanticPolicy != nil {
		return OwnedInventoryConfig{}, ErrDurableInventory
	}
	c, err := ReferenceAdmissionConfigV1(e, a)
	if err != nil {
		return c, err
	}
	c.Schema = is.InventorySemanticConfigVersionV1
	profile := is.NewInventorySemanticProfileV1()
	c.ResumeProfile = &profile
	c.SemanticPolicy = &p
	c.TaskWindow = time.Duration(p.TaskWindowNS)
	if !semanticConfigValid(c) {
		return OwnedInventoryConfig{}, ErrDurableInventory
	}
	return c, nil
}
func semanticConfigValid(c OwnedInventoryConfig) bool {
	if c.Schema != is.InventorySemanticConfigVersionV1 || c.SemanticPolicy == nil || !c.SemanticPolicy.Valid() || int64(c.TaskWindow) != c.SemanticPolicy.TaskWindowNS || c.ResumeProfile == nil || *c.ResumeProfile != is.NewInventorySemanticProfileV1() {
		return false
	}
	// Local validation projection only: never serialize this as a replacement job.
	c.Schema = is.InventoryReferenceConfigVersionV1
	c.SemanticPolicy = nil
	c.TaskWindow = 30 * time.Second
	p := is.NewInventoryReferenceProfileV1()
	c.ResumeProfile = &p
	return referenceConfigValid(c)
}
func (h durableInventoryHead) semanticTimingValid() bool {
	if h.Schema != is.InventorySemanticHeadVersionV1 {
		return h.SemanticPolicy == nil && h.TaskIssuedNS == 0 && h.Window > 0 && h.Window <= int64(30*time.Second) && h.SemanticNS >= 0 && h.SemanticNS <= int64(120*time.Second)
	}
	p := h.SemanticPolicy
	if p == nil || !p.Valid() || h.Window != p.TaskWindowNS || h.RunDeadline != 0 || h.Sequence < 1 || h.Sequence > 5 || h.TaskIssuedNS <= 0 || h.TaskIssuedNS > math.MaxInt64-h.Window || h.Deadline != h.TaskIssuedNS+h.Window {
		return false
	}
	stages := h.Sequence
	if h.Phase == "Published" {
		if stages != 5 {
			return false
		}
		stages--
	} else if stages > 4 {
		return false
	}
	return h.SemanticNS == int64(stages)*h.Window && h.SemanticNS <= p.CumulativeWindowNS
}
func (h durableInventoryHead) activeDeadline() int64 {
	end := h.operationDeadline()
	if h.Schema == is.InventorySemanticHeadVersionV1 {
		end = min(end, time.Now().Add(30*time.Second).UnixNano())
	}
	return end
}
