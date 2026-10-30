package publication

import "context"

// CheckSourceProtectionSeparationPrivateV2 borrows two NONCONCURRENT read-only
// topology capabilities. It rejects identity equality and ancestry in either
// direction using the existing pinned, no-follow descriptor-chain checker.
// It grants no publication capability or protection against concurrent same-user
// namespace mutation. Close remains the caller's responsibility.
func CheckSourceProtectionSeparationPrivateV2(ctx context.Context, a, b *SourceProtectionRootPrivateV2) error {
	if a == nil || b == nil {
		return ErrObservedRootAdmission
	}
	return checkRootChainsPrivate(ctx, []topologyRootViewPrivate{
		{a.path, a.info, a.validate}, {b.path, b.info, b.validate},
	}, 1, defaultObservedRootOps())
}
