//go:build !darwin && !linux

package publication

import (
	"context"
	"errors"
	"testing"
)

func TestScopedExecutionJournalPrivateUnsupported(t *testing.T) {
	if scopedExecutionSupportedPrivate() {
		t.Fatal("unexpected support")
	}
	if _, err := compareScopedExecutionPrivate(context.Background(), nil, []byte("claim"), defaultScopedExecutionOpsPrivate()); !errors.Is(err, ErrObservedRootUnsupported) {
		t.Fatal(err)
	}
	if _, err := loadScopedExecutionPrivate(context.Background(), nil, defaultScopedExecutionOpsPrivate()); !errors.Is(err, ErrObservedRootUnsupported) {
		t.Fatal(err)
	}
}
