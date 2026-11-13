//go:build darwin || linux

package sourceoperator

import (
	"context"
	"time"

	inv "lsp-trace/internal/inventorysuccessor"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
)

type privateSelectedRequestV2 struct {
	Owner         *inv.PrivateManagedInputOwnerV2
	Capture       inv.PrivateMixedInputRequestV2
	Configuration inv.PrivateSelectedConfigurationV2
	Endpoint      *sc.PrivateSelectedEndpointV2
	DTarget       bool                      // closed policy, copied by value before M
	Role          *sc.PrivateSelectedRoleV3 // exclusive, closed, copied before M
}

// Private, fixed, untimed M -> P -> S. No Host, worker, callback, persistent
// store, publication roots, Open or Complete surface exists. Existing admitted
// funding is required; plans never initialize jobs or issue grants. Fresh run,
// namespace, lineage and three distinct attempts are caller declarations whose
// replay/retirement is still enforced by the existing canonical native chain.
func privatePrepareSelectedKindsV2(ctx context.Context, q privateSelectedRequestV2) (out sc.PrivateSelectedResultV2, err error) {
	if q.DTarget && (q.Endpoint != nil || q.Role != nil || !privateRDTargetInputV2(q.Capture, q.Configuration)) {
		return out, inv.ErrPrivateSelectedV2
	}
	if ctx == nil || ctx.Err() != nil || q.Owner == nil || q.Capture.Timing != nil || q.Capture.Start.Deadline.IsZero() || !time.Now().Before(q.Capture.Start.Deadline) || time.Until(q.Capture.Start.Deadline) > 15*time.Second {
		return out, inv.ErrPrivateSelectedV2
	}
	// Same original deadline; the outer 30s limit is never used to renew this one.
	execution, cancel := context.WithDeadline(ctx, q.Capture.Start.Deadline)
	defer func() {
		cancel()
		if err != nil || ctx.Err() != nil || !time.Now().Before(q.Capture.Start.Deadline) {
			out = sc.PrivateSelectedResultV2{}
			if err == nil {
				err = inv.ErrPrivateSelectedV2
			}
		}
	}()
	if q.Role != nil {
		copy := *q.Role
		q.Role = &copy
		if q.Endpoint != nil || q.DTarget || !privateSelectedRoleInputV3(q.Capture, q.Configuration, copy) {
			return out, inv.ErrPrivateSelectedV2
		}
	}
	if q.Endpoint != nil {
		copy := *q.Endpoint
		q.Endpoint = &copy
	}
	plan, err := inv.NewPrivateSelectedPreparationPlanV2(q.Owner, q.Capture, q.Configuration)
	if err != nil {
		return out, err
	}
	capture, err := q.Owner.ReserveSelectedMixedInputV2(execution, q.Configuration.Attempts[0], q.Capture, plan)
	if err != nil {
		return out, err
	}
	captured, err := inv.ExecuteSelectedMixedInputV2(execution, capture, q.Capture, plan)
	if err != nil {
		return out, err
	}
	preparation, err := q.Owner.ReserveSelectedPreparationV2(execution, captured)
	if err != nil {
		return out, err
	}
	prepared, err := inv.ExecuteSelectedPreparationV2(execution, preparation)
	if err != nil {
		return out, err
	}
	source, err := q.Owner.ReserveSelectedSourceV2(execution, prepared)
	if err != nil {
		return out, err
	}
	if q.Role != nil {
		return sc.PrepareSelectedRoleV3(execution, source, *q.Role)
	}
	if q.DTarget {
		return sc.PrepareSelectedDTargetV2(execution, source)
	}
	return sc.PrepareSelectedV2(execution, source, q.Endpoint)
}
