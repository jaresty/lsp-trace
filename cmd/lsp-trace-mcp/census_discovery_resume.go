package main

import (
	"lsp-trace/internal/censussourceassembly"
	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/censussourceplan"
)

func privateDiscoveryContinuationTerminal(plan censussourceplan.Plan, terminal *privateConstituentDiscoveryTerminal) (censussourcecontinuation.Terminal, error) {
	return privateDiscoveryContinuationTerminalAtOrdinal(plan, terminal)
}

func privateDiscoveryContinuationTerminalAtOrdinal(plan censussourceplan.Plan, terminal *privateConstituentDiscoveryTerminal) (censussourcecontinuation.Terminal, error) {
	if terminal == nil {
		return censussourceassembly.AdaptTerminal(plan, censusRuntimeSourcePartitionSize, nil)
	}
	return censussourceassembly.AdaptTerminal(plan, censusRuntimeSourcePartitionSize, &censussourceassembly.TerminalRecord{
		PlanIdentity: terminal.planIdentity,
		Authority:    terminal.authority,
		Ordinal:      terminal.ordinal,
		Members:      terminal.members,
		Discovery:    terminal.discovery,
		Outcome:      terminal.outcome,
	})
}
