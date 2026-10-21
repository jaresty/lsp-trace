package censusacquisition

import (
	"errors"
	"math"

	"lsp-trace/internal/acquisition"
)

const legacyAcquisitionMaxRequests = 1000

// AcquisitionMaxRequests returns the coordinator request allowance for Census.
// Ordinary acquisition retains its historical fixed allowance. Private broad
// acquisition derives the maximum from the admitted per-batch target and node
// limits and the coordinator's actual request model: one document supply and
// one sibling document-symbol request per target, globally bounded prepare
// probes, and one outgoing plus one incoming query per admitted node.
func AcquisitionMaxRequests(private bool, maxNodes, maxBatchTargets int) (int, error) {
	if !private {
		return legacyAcquisitionMaxRequests, nil
	}
	if maxNodes < 1 || maxNodes > 10000 || maxBatchTargets < 1 || maxBatchTargets > maxCensusBatchTargets {
		return 0, errors.New("private census acquisition plan bounds invalid")
	}
	nodes, ok := checkedTwice(maxNodes)
	if !ok {
		return 0, errors.New("private census acquisition request budget overflow")
	}
	targets, ok := checkedTwice(maxBatchTargets)
	if !ok {
		return 0, errors.New("private census acquisition request budget overflow")
	}
	budget, ok := checkedAdd(nodes, targets)
	if !ok {
		return 0, errors.New("private census acquisition request budget overflow")
	}
	budget, ok = checkedAdd(budget, acquisition.MaxPrepareProbes)
	if !ok {
		return 0, errors.New("private census acquisition request budget overflow")
	}
	return budget, nil
}

func checkedTwice(value int) (int, bool) {
	if value < 0 || value > math.MaxInt/2 {
		return 0, false
	}
	return value * 2, true
}

func checkedAdd(left, right int) (int, bool) {
	if right < 0 || left > math.MaxInt-right {
		return 0, false
	}
	return left + right, true
}
