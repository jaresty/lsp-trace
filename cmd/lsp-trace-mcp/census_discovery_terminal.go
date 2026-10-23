package main

import (
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censussourceassembly"
	"lsp-trace/internal/censussourceplan"
)

type privateConstituentDiscoveryOutcome = censussourceassembly.Outcome

const (
	privateDiscoveryCompleteNonempty = censussourceassembly.OutcomeCompleteNonempty
	privateDiscoverySuccessfulEmpty  = censussourceassembly.OutcomeSuccessfulEmpty
	privateDiscoveryPartial          = censussourceassembly.OutcomePartial
	privateDiscoveryFailed           = censussourceassembly.OutcomeFailed
	privateDiscoveryRefused          = censussourceassembly.OutcomeRefused
	privateDiscoveryUnknown          = censussourceassembly.OutcomeUnknown
)

type privateConstituentDiscoveryTerminal struct {
	planIdentity string
	authority    uint8
	ordinal      int
	members      []censusacquisition.SourceFile
	discovery    censusacquisition.Discovery
	outcome      privateConstituentDiscoveryOutcome
}

func newPrivateConstituentDiscoveryTerminal(plan censussourceplan.Plan, discovery censusacquisition.Discovery) (*privateConstituentDiscoveryTerminal, error) {
	return newPrivateConstituentDiscoveryTerminalAtOrdinal(plan, 0, discovery)
}

func newPrivateConstituentDiscoveryTerminalAtOrdinal(plan censussourceplan.Plan, ordinal int, discovery censusacquisition.Discovery) (*privateConstituentDiscoveryTerminal, error) {
	record, err := censussourceassembly.NewTerminalRecord(plan, censusRuntimeSourcePartitionSize, ordinal, discovery)
	if err != nil {
		return nil, err
	}
	return &privateConstituentDiscoveryTerminal{
		planIdentity: record.PlanIdentity,
		authority:    record.Authority,
		ordinal:      record.Ordinal,
		members:      record.Members,
		discovery:    record.Discovery,
		outcome:      record.Outcome,
	}, nil
}

func privateDiscoveryOutcome(discovery censusacquisition.Discovery) privateConstituentDiscoveryOutcome {
	return privateConstituentDiscoveryOutcome(censusacquisition.TerminalDiscoveryOutcome(discovery))
}

func cloneTerminalSlice[T any](values []T) []T {
	if values == nil {
		return nil
	}
	clone := make([]T, len(values))
	copy(clone, values)
	return clone
}

func clonePrivateTerminalDiscovery(discovery censusacquisition.Discovery) censusacquisition.Discovery {
	return censussourceassembly.CloneDiscovery(discovery)
}
