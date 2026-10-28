package censusacquisition

import (
	"fmt"
)

// PlanDiscovery plans canonical requests without acquiring or admitting a constituent.
// It shares the exact target, partition, identity and seed helpers used by Core.Run.
func (c Core) PlanDiscovery(s SessionIdentity, d Discovery) ([]BatchRequest, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	if d.Session != s || !d.Complete {
		return nil, ErrDiscoveryIncomplete
	}
	if err := validateDiscovery(d); err != nil {
		return nil, err
	}
	if err := d.Accounting.Validate(); err != nil {
		return nil, err
	}
	planning, err := resolvePlanning(c.Planning)
	if err != nil {
		return nil, err
	}
	targets, prepared, err := canonicalTargets(d)
	if err != nil {
		return nil, err
	}
	plan, err := planTargets(targets, planning.MaxBatchTargets)
	if err != nil {
		return nil, err
	}
	cid := stableID("census", planningIdentity(joinTargetBytes(targets), planning))
	batches := make([]BatchRequest, len(plan.Batches))
	for i, b := range plan.Batches {
		pt := make([]PreparedTarget, len(b.Targets))
		for j, t := range b.Targets {
			pt[j] = cloneTarget(prepared[t.CensusOrdinal])
		}
		seeds, err := combineSeedsForPlanning(pt, d.Workspace, planning)
		if err != nil {
			return nil, failBatch(i, err)
		}
		identity := []byte(fmt.Sprintf("%s\x00%d\x00%s", cid, i, joinTargetBytes(b.Targets)))
		batches[i] = BatchRequest{s, cid, stableID("batch", planningIdentity(identity, planning)), i, planning.DownDepth, planning.UpDepth, pt, seeds}
		if err := ValidateBatchSeeds(batches[i], d.Workspace); err != nil {
			return nil, failBatch(i, err)
		}
	}
	return batches, nil
}
