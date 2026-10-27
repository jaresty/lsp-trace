//go:build !darwin && !linux

package publication

import "context"

type scopedExecutionOpsPrivate struct{}

func defaultScopedExecutionOpsPrivate() scopedExecutionOpsPrivate { return scopedExecutionOpsPrivate{} }
func scopedExecutionSupportedPrivate() bool                       { return false }
func loadScopedExecutionPrivate(context.Context, *ScopedExecutionJournalPrivate, scopedExecutionOpsPrivate) ([]byte, error) {
	return nil, ErrObservedRootUnsupported
}
func compareScopedExecutionPrivate(context.Context, *ScopedExecutionJournalPrivate, []byte, scopedExecutionOpsPrivate) (*CompareAndReplaceReceipt, error) {
	return nil, ErrObservedRootUnsupported
}
