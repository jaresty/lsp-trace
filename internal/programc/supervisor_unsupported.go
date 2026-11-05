//go:build !darwin || !arm64

package programc

import "context"

func supervisePrivateWorker(context.Context, workerRequest) (privateSupervisorResult, SupervisionObservation, *SupervisionFailure) {
	return privateSupervisorResult{}, SupervisionObservation{Ceiling: ObservationCeiling}, failure(CodePlatformUnsupported, nil)
}
