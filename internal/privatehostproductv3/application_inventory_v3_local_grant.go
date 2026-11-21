//go:build darwin || linux

package privatehostproductv3

import "time"

// InventoryLocalGrantV3 quotes the existing controller APIs for one local run.
// This is DATA, not admission: the normal caller must separately reserve/read
// back this entire envelope before creating the application or issuing a permit.
// The V3 journal still reserves each child operation; parent and child amounts
// are not added as measured expenditure. No provider credit is granted here.
func InventoryLocalGrantV3(deadline time.Time, planned uint64, coordinator ApplicationBounds) (ApplicationGrant, error) {
	if planned < 2 || planned > 16 || !time.Now().Before(deadline) || time.Until(deadline) > 180*time.Second || !applicationNoSemantics(coordinator) || coordinator.ProviderRequests != 0 || coordinator.ProviderMessages != 0 {
		return ApplicationGrant{}, ErrInvalid
	}
	g := ApplicationGrant{Revision: 1, ExpiresUnixNano: deadline.UnixNano(), BootstrapAttempts: 2, BootstrapEach: ApplicationBounds{ReadHashBytes: 4 * ApplicationRecordLimit, WriteBytes: ApplicationRecordLimit, Records: 8, ParserBytes: 3 * ApplicationRecordLimit, Work: 32, ActiveNS: 1000000}}
	bootstrap, ok := applicationTimes(g.BootstrapEach, g.BootstrapAttempts)
	if !ok {
		return ApplicationGrant{}, ErrInvalid
	}
	// Create once for setup, open once for run. HOST/readback interfaces do not
	// reopen the application journal; no replay/recovery opens are granted.
	// Same 16 first-return retention slots used by admitInventoryV3. Processing
	// quote: <=16 fixed setup/ticket/run/reconcile/terminal API attempts plus
	// <=3 per planned exchange (request, reservation, retention bookkeeping).
	// At 16 calls this is the previous 64-attempt upper bound, never history.
	retention, ok := applicationTimes(applicationRetentionEnvelope(), 16)
	if !ok {
		return ApplicationGrant{}, ErrInvalid
	}
	overhead, ok := applicationTimes(applicationOverhead(), 16+3*planned)
	if !ok {
		return ApplicationGrant{}, ErrInvalid
	}
	for _, part := range []ApplicationBounds{bootstrap, coordinator, inventoryAllowanceV3(InventoryBindingV3{PlannedCalls: planned}), retention, overhead} {
		g.Ceiling, ok = applicationAdd(g.Ceiling, part)
		if !ok {
			return ApplicationGrant{}, ErrInvalid
		}
	}
	// The existing normal caller also reopens publication and transports framed
	// task bytes. Reserve that bounded read/copy lane, not another semantic grant.
	local := ApplicationBounds{LogicalBytes: 64 << 20, ReadHashBytes: 64 << 20, WriteBytes: ApplicationRecordLimit, Records: 128, ParserBytes: 64 << 20, Work: 1 << 20}
	g.Ceiling, ok = applicationAdd(g.Ceiling, local)
	if !ok || g.Ceiling.ActiveNS > uint64(180*time.Second) {
		return ApplicationGrant{}, ErrInvalid
	}
	return g, nil
}
