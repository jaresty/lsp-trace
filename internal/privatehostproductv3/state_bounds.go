//go:build darwin || linux

package privatehostproductv3

// Concrete, monotone pre-serialization bounds for this NEW progress/report
// schema, not a generic encoder or a whole-process memory measurement. Refuse
// before marshaling a total application snapshot that exceeds this private unit.
type encodedBudget struct{ left, work int }

func newEncodedBudget() *encodedBudget { return &encodedBudget{left: stateLimit, work: 100000} }
func (b *encodedBudget) fixed(n int) bool {
	b.work--
	if b.work < 0 || n < 0 || n > b.left {
		return false
	}
	b.left -= n
	return true
}
func (b *encodedBudget) strings(values ...string) bool {
	for _, v := range values {
		if !b.fixed(2) || len(v) > b.left/6 {
			return false
		}
		b.left -= 6 * len(v)
	}
	return true
}
func (b *encodedBudget) candidate(c Candidate) bool {
	if !b.fixed(4096) || !b.strings(c.ID, c.InputID, c.Target, c.Status, c.WorkingLabel, c.Explanation, c.InventoryStateDelta, c.RejectionCondition, c.Quote) {
		return false
	}
	for _, list := range [][]string{c.Predecessors, c.EvidenceRefs, c.Alternatives, c.Gaps} {
		for _, s := range list {
			if !b.strings(s) {
				return false
			}
		}
	}
	for _, ref := range c.Evidence {
		if !b.fixed(128) || !b.strings(ref.Family, ref.Ref, ref.InputID) {
			return false
		}
	}
	return true
}
func admitProgress(p progress) bool {
	b := newEncodedBudget()
	c := p.Config
	if !b.fixed(32768) || !b.strings(c.Profile, c.Application, c.ScopeSHA256, c.ProducerConfigSHA256, c.ApprovalSHA256, c.HumanReceiptSHA256, c.PreviousConfigSHA256, c.Store, c.RolePromptSHA256[0], c.RolePromptSHA256[1], p.LastStatus) {
		return false
	}
	if len(p.LastCheckpoint) > 4096 || !b.fixed(2*len(p.LastCheckpoint)) {
		return false
	}
	for range c.Seeds {
		if !b.fixed(32) {
			return false
		}
	}
	for _, candidate := range p.Candidates {
		if !b.candidate(candidate) {
			return false
		}
	}
	for _, obs := range p.Observations {
		if !b.fixed(1024) || !b.strings(obs.InputID, obs.Checkpoint) {
			return false
		}
		for _, ref := range obs.References {
			if !b.strings(ref) {
				return false
			}
		}
		for _, ref := range obs.RawReferences {
			if !b.strings(ref) {
				return false
			}
		}
		for _, body := range obs.Bodies {
			if !b.fixed(256) || !b.strings(body.Target, body.NodeID, body.SourceURI, body.OwnBodySHA256, body.Body) {
				return false
			}
		}
	}
	return true
}
func admitReport(r Report) bool {
	b := newEncodedBudget()
	if !b.fixed(32768) || !b.strings(r.Schema, r.Profile, r.Status, r.Application, r.ConfigSHA256, r.FeatureIdentity, r.SourceCompleteness, r.AbsenceInference, r.CheckpointSHA256, r.LedgerTip, r.Resume, r.Lifecycle.RoleCleanup) {
		return false
	}
	for _, c := range r.Candidates {
		if !b.candidate(c) {
			return false
		}
	}
	for _, list := range [][]string{r.Unassigned, r.NavigationHypotheses, r.Spend.Excluded, r.MachinePaths[:], r.HumanPaths[:]} {
		for _, s := range list {
			if !b.strings(s) {
				return false
			}
		}
	}
	for _, gap := range r.TargetGaps {
		if !b.fixed(256) || !b.strings(gap.Target, gap.Phase, gap.Class) {
			return false
		}
	}
	if a := r.Admission; a != nil {
		if !b.fixed(1024) || !b.strings(a.CurrentValidation, a.LedgerValidation, a.SourceValidation, a.Coverage, a.Counts, a.LedgerObservation, a.ResourceCounts, a.PriorPublicationValidation, a.PriorMachinePaths[0], a.PriorMachinePaths[1], a.PriorHumanPaths[0], a.PriorHumanPaths[1]) {
			return false
		}
	}
	for _, failure := range []*Failure{r.Failure, r.FirstFailure} {
		if failure != nil && (!b.fixed(256) || !b.strings(failure.Phase, failure.Class, failure.Resource, failure.ExactOriginalError)) {
			return false
		}
	}
	return true
}
