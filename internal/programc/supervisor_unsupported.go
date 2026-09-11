//go:build !darwin || !arm64

package programc

import "context"

func computeSupervised(context.Context, []byte, uint64) (Outcome, SupervisionObservation, *SupervisionFailure) {
	return Outcome{}, SupervisionObservation{Ceiling: ObservationCeiling}, failure(CodePlatformUnsupported, nil)
}
