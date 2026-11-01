//go:build darwin || linux

package censuslineagepilot

import "context"

// HostFrontierPrivateV3 is detached observational metadata, never a restored
// owner. This sibling does not change existing ledger opening or admission.
func (p RetainedPageCheckpointOwnerPrivateV3) HostFrontierPrivateV3() (RetainedPageCheckpointManifestPrivateV3, uint64, string, PageCostPrivateV3, error) {
	// Detached failure/progress reporting remains available after a source or
	// unit failure. This does not authorize another read/reservation/admission.
	if p.state == nil || p.state.self != p.state || p.state.ledger.state == nil {
		return RetainedPageCheckpointManifestPrivateV3{}, 0, "", PageCostPrivateV3{}, ErrInvalid
	}
	p.state.mu.Lock()
	defer p.state.mu.Unlock()
	g, tip, spent := p.state.ledger.Tip()
	return p.state.manifest, g, tip, spent, nil
}

// ReserveHostWorkPrivateV3 charges explicit deterministic host processing to the
// very same application journal. It grants no source/provider/model authority.
func (p RetainedPageCheckpointOwnerPrivateV3) ReserveHostWorkPrivateV3(ctx context.Context, cost, exchange PageCostPrivateV3) error {
	if ctx == nil || ctx.Err() != nil || p.state == nil || !p.state.valid() {
		return ErrInvalid
	}
	p.state.mu.Lock()
	defer p.state.mu.Unlock()
	return p.state.ledger.state.reserve(cost, &exchange)
}
