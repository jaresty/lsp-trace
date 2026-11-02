//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"path/filepath"
	"time"
)

// Explicit parent approval at PREPAY_CHILD, never inferred from the first bind.
// Only native Requested (1..MaxTargets) and its two digest consequences may vary.
// All source identity, producer, roots, policy, prompts, seeds and costs are exact.
type applicationDerivationContract struct {
	Source       Config
	Cohort       CohortConfig
	MaxRequested int
}

func normalizeApplicationDerivation(source Config, cohort CohortConfig) (Config, CohortConfig) {
	source.Requested = 0
	source.ApprovalSHA256 = ""
	cohort.Requested = 0
	cohort.SourceConfigSHA256 = ""
	return source, cohort
}

func (a *ApplicationJournal) AdmitUnitWithDerivation(u UnitBinding, b ApplicationBounds, source Config, cohort CohortConfig, now time.Time) error {
	j := a.implementation
	if !validApplicationUnit(j.config, u) {
		return ErrInvalid
	}
	root := filepath.Join(j.config.Store, "units", u.UnitID[7:])
	if !validApplicationUnit(j.config, u) || u.RevisionSHA256 != hash([]byte(j.config.RevisionAssertion)) || source.Store != filepath.Join(root, "original") || cohort.Store != filepath.Join(root, "cohort") || source.ScopeSHA256 != u.ScopeSHA256 || cohort.ScopeSHA256 != u.ScopeSHA256 || cohort.RolePromptSHA256 != j.config.RolePromptSHA256 || source.Requested != 0 || cohort.Requested != 0 {
		return ErrInvalid
	}
	max := source.Preparation.MaxTargets
	probe := source
	probe.Requested = 1
	probe.ApprovalSHA256 = ConfigDigest(probe)
	cp := cohort
	cp.Requested = 1
	cp.SourceConfigSHA256 = ConfigDigest(probe)
	if max < 1 || max > 64 || ValidateConfig(probe) != nil || ValidateCohortConfig(cp) != nil {
		return ErrInvalid
	}
	source, cohort = normalizeApplicationDerivation(source, cohort)
	j.pendingDerivation = &applicationDerivationContract{source, cohort, max}
	defer func() { j.pendingDerivation = nil }()
	return j.admitUnit(u, b, now)
}
func applicationMatchesDerivation(c *applicationDerivationContract, source Config, cohort CohortConfig) bool {
	if c == nil || source.Requested < 1 || source.Requested > c.MaxRequested || cohort.Requested != source.Requested || source.ApprovalSHA256 != ConfigDigest(source) || cohort.SourceConfigSHA256 != ConfigDigest(source) {
		return false
	}
	source, cohort = normalizeApplicationDerivation(source, cohort)
	return bytes.Equal(mustJSON(source), mustJSON(c.Source)) && bytes.Equal(mustJSON(cohort), mustJSON(c.Cohort))
}
