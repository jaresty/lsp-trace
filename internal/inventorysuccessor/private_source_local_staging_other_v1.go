//go:build !darwin && !linux

package inventorysuccessor

import "os"

// Unsupported platforms cannot select an executor or stage for execution.
func localWorkerFixtureV1(PrivateSourceLocalConfigV1) bool           { return false }
func trustedSystemSandboxInfoV1(os.FileInfo) bool                    { return false }
func (*privateSourceRPreparationStateV1) prepareLocalStageV1() error { return ErrPrivateSourceLocalV1 }

type localStageNamespaceV1 struct{}
type localStageWriterV1 struct{}

func (*localStageWriterV1) Write([]byte) (int, error) { return 0, ErrPrivateSourceLocalV1 }
func (*privateSourceRPreparationStateV1) stageWriterV1(int) (*localStageWriterV1, error) {
	return nil, ErrPrivateSourceLocalV1
}
func finishLocalStageWriterV1(w *localStageWriterV1, _ int) error {
	if w == nil {
		return nil
	}
	return ErrPrivateSourceLocalV1
}
func (*privateSourceRPreparationStateV1) cleanupLocalStageV1() error { return nil }
