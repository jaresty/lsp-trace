//go:build darwin || linux

package privatehostproductv3

import (
	"context"
	pilot "lsp-trace/internal/censuslineagepilot"
)

// Same existing page ledger, not a per-cohort/application-budget reset. The
// conservative frame-processing reservation precedes disclosure; it does not
// claim to measure all host/report IO or practical cost amplification.
type cohortReservedBackend struct {
	owner   pilot.RetainedPageCheckpointOwnerPrivateV3
	source  Config
	config  CohortConfig
	backend CohortBackend
}

func (b cohortReservedBackend) ExchangeCohort(ctx context.Context, raw []byte) ([]byte, error) {
	n := uint64(len(raw) + b.config.MaxResponseBytes)
	cost := pilot.PageCostPrivateV3{LogicalBytes: n*16 + 65536, ReadHashBytes: n*32 + 65536, WriteBytes: n*4 + 65536, Work: n + 1024}
	if e := b.owner.ReserveHostWorkPrivateV3(ctx, cost, b.source.Exchange); e != nil {
		return nil, e
	}
	return b.backend.ExchangeCohort(ctx, raw)
}
func (b cohortReservedBackend) CohortLifecycle() Lifecycle {
	if observed, ok := b.backend.(interface{ CohortLifecycle() Lifecycle }); ok {
		return observed.CohortLifecycle()
	}
	return Lifecycle{RoleCleanup: "UNOBSERVED_HOST_ROLE_RETURN"}
}
func (b cohortReservedBackend) CohortFrontier() (uint64, string, pilot.PageCostPrivateV3, error) {
	_, g, tip, cost, e := b.owner.HostFrontierPrivateV3()
	return g, tip, cost, e
}
