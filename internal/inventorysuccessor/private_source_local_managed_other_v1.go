//go:build !darwin && !linux

package inventorysuccessor

import (
	"context"
	"time"
)

type localManagedGenerationV1 struct {
	stage privateSourceRPreparationStateV1
}
type localManagedUseV1 struct{}

func (*PrivateSourceLocalManagedV1) matches(PrivateSourceLocalConfigV1) bool { return false }
func (*localManagedUseV1) live(*privateSourceRPreparationStateV1) bool       { return false }
func (*privateSourceRPreparationStateV1) bindManagedV1(context.Context) error {
	return ErrPrivateSourceLocalV1
}
func (*privateSourceRPreparationStateV1) releaseManagedV1(bool) error { return nil }
func managedUseQuoteV1(PrivateSourceLocalConfigV1) (countersV2, error) {
	return countersV2{}, ErrPrivateSourceLocalV1
}
func managedArtifactQuoteV1(PrivateSourceLocalManagedRequestV1, time.Time, verificationLimitsV2) (countersV2, error) {
	return countersV2{}, ErrPrivateSourceLocalV1
}
func (*PrivateManagedInputOwnerV2) AdmitLocalArtifactsV1(context.Context, PrivateSourceLocalManagedRequestV1) (PrivateSourceLocalManagedV1, error) {
	return PrivateSourceLocalManagedV1{}, ErrPrivateSourceLocalV1
}
func (PrivateSourceLocalManagedV1) Revoke() error                { return ErrPrivateSourceLocalV1 }
func (PrivateSourceLocalManagedV1) Retire(context.Context) error { return ErrPrivateSourceLocalV1 }
