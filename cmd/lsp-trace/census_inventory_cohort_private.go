//go:build darwin || linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	pilot "lsp-trace/internal/censuslineagepilot"
	prep "lsp-trace/internal/privatedescribeprep"
	"lsp-trace/internal/publication"
)

func (c censusInventoryConfigPrivate) hasAutomation() bool {
	return c.Automation != nil || c.CohortAutomation != nil
}
func (c censusInventoryConfigPrivate) automationDirectory() string {
	if c.CohortAutomation != nil {
		return c.CohortAutomation.Directory
	}
	if c.Automation != nil {
		return c.Automation.Directory
	}
	return ""
}
func (c censusInventoryConfigPrivate) automationScope() string {
	if c.CohortAutomation != nil {
		return c.CohortAutomation.ScopeSHA256
	}
	if c.Automation != nil {
		return c.Automation.ScopeSHA256
	}
	return ""
}
func (c censusInventoryConfigPrivate) validateAutomation() error {
	if c.CohortAutomation != nil {
		if c.Automation != nil {
			return errors.New("private inventory automation modes are mutually exclusive")
		}
		return prep.ValidateCohortHandoffPrivateV2(*c.CohortAutomation)
	}
	if c.Automation != nil {
		return prep.ValidateAutomaticHandoffPrivateV2(*c.Automation)
	}
	return nil
}

// No acquisition authority is reconstructed here. The CLI still holds h and the
// original roots. This sibling is reachable only after runtime scope admission.
func runCensusCohortPrivate(ctx context.Context, c censusInventoryConfigPrivate, r *censusInventoryReportPrivate, h prep.PostPreparationPrivateV2, outputs [2]*publication.ContentRetentionRoot, protected []*publication.ObservedRoot, roots []string) (outcome prep.PostPublicationOutcomePrivateV2, err error) {
	cfg := *c.CohortAutomation
	binding := struct {
		Schema, Profile, Protocol                             string
		Authority                                             int
		Accepted, Qualified                                   bool
		FeatureIdentity, SourceCompleteness, AbsenceInference string
		ScopeSHA256, PreparationSHA256                        string
		CohortAutomation                                      prep.CohortHandoffConfigPrivateV2
		Roots                                                 []string
	}{"private-census-inventory-cohort-host-export.v1", censusInventoryProfilePrivate, prep.CohortHandoffProtocolPrivateV2, 0, false, false, "UNRESOLVED", "UNKNOWN", "PROHIBITED", r.ScopeSHA256, r.PreparationSHA256, cfg, roots}
	if err = writeCensusInventoryFilePrivate(r.HostExportBindingPath, binding); err != nil {
		return
	}
	raw, _ := json.Marshal(binding)
	r.HostExportBindingSHA256 = pilot.Digest(raw)
	i, v, e := prep.NewCohortHandoffPrivateV2(cfg, r.ScopeSHA256, h)
	if e != nil {
		return outcome, e
	}
	result, e := prep.AutomateCohortPrivateV2(ctx, h, cfg.Limits, cfg.MaxItems, i, v)
	inventory := result.Inventory()
	r.Inventory = &inventory
	r.Status = inventory.Status
	if e == nil {
		outcome, e = prep.RetainAutomaticPrivateV2(ctx, h, result, outputs, protected)
		r.Outcome = &outcome
	}
	return outcome, errors.Join(e, prep.FinishCohortHandoffPrivateV2(i, r.Outcome, e))
}
