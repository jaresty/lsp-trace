//go:build darwin || linux

package publication

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestProducerPacketTopologyExtractionPreservesOldCloseErrors(t *testing.T) {
	outputs, _, _ := retentionTopologyFixturePrivate(t)
	ops, log := trackingObservedRootOps(t)
	close := ops.closeFile
	sentinel := errors.New("owned old-wrapper acquired-chain close failure")
	ops.closeFile = func(f *os.File) error { return errors.Join(close(f), sentinel) }
	err := checkPostPreparationRetentionTopologyV1Private(context.Background(), [2]*ContentRetentionRoot{outputs[0], outputs[1]}, []*ObservedRoot{nil}, ops)
	if !errors.Is(err, ErrObservedRootAdmission) || !errors.Is(err, sentinel) {
		t.Fatal("ASSERT_PACKET_EXTRACTION_PRESERVES_OLD_VALIDATION_CLOSE_ORDER", err)
	}
	log.assertClosed(t)
}
