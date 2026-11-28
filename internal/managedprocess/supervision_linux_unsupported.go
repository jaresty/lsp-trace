//go:build !linux || (!amd64 && !arm64)

package managedprocess

import (
	"context"
	"io"
)

// Complementary compile-time dispatch stub. No factory, pipe, or process effect.
func startOwnedLinux(_ *Manager, _ context.Context, _ Spec, _ io.Writer) (*Process, StartObservation) {
	return nil, StartObservation{Kind: StartUnavailable, Reason: "Linux owned-group candidate unsupported", Err: ErrPrivateProviderWindow}
}
