package main

import (
	"sync"

	"lsp-trace/internal/censuscontrolflow"
)

type censusControlFlowObserver struct {
	mu          sync.Mutex
	firstCause  censuscontrolflow.FirstCause
	corePhase   censuscontrolflow.CorePhase
	batchRecord censuscontrolflow.BatchRecord
}

func newCensusControlFlowObserver() *censusControlFlowObserver {
	return &censusControlFlowObserver{corePhase: censuscontrolflow.PhaseNotEntered, batchRecord: censuscontrolflow.BatchNotAttempted}
}

func (o *censusControlFlowObserver) cause(cause censuscontrolflow.FirstCause, phase censuscontrolflow.CorePhase) {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.firstCause != "" {
		return
	}
	o.firstCause, o.corePhase = cause, phase
}

func (o *censusControlFlowObserver) batch(record censuscontrolflow.BatchRecord) {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.batchRecord == censuscontrolflow.BatchNotAttempted {
		o.batchRecord = record
	}
}

func (o *censusControlFlowObserver) record() (censuscontrolflow.Record, bool) {
	if o == nil {
		return censuscontrolflow.Record{}, false
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.firstCause == "" {
		return censuscontrolflow.Record{}, false
	}
	return censuscontrolflow.Record{SchemaVersion: censuscontrolflow.SchemaVersion, FirstCause: o.firstCause, CorePhase: o.corePhase, BatchRecord: o.batchRecord}, true
}

func censusControlFlowPhase(stage censusFailureStage) censuscontrolflow.CorePhase {
	switch stage {
	case censusStageDiscovery:
		return censuscontrolflow.PhaseDiscovery
	case censusStageAcquisition:
		return censuscontrolflow.PhaseBatchAcquire
	case censusStageAssembly:
		return censuscontrolflow.PhaseManifest
	default:
		return censuscontrolflow.PhaseCompleted
	}
}
