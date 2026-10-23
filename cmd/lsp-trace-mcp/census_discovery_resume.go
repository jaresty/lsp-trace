package main

import (
	"errors"
	"reflect"

	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/censussourceplan"
)

func privateDiscoveryContinuationTerminal(plan censussourceplan.Plan, terminal *privateConstituentDiscoveryTerminal) (censussourcecontinuation.Terminal, error) {
	return privateDiscoveryContinuationTerminalAtOrdinal(plan, terminal)
}

func privateDiscoveryContinuationTerminalAtOrdinal(plan censussourceplan.Plan, terminal *privateConstituentDiscoveryTerminal) (censussourcecontinuation.Terminal, error) {
	if terminal == nil || censussourceplan.Validate(plan, censusRuntimeSourcePartitionSize) != nil || terminal.planIdentity != plan.Identity || terminal.authority != 0 || terminal.ordinal < 0 || terminal.ordinal >= len(plan.Partitions) || !reflect.DeepEqual(terminal.members, plan.Partitions[terminal.ordinal].Members) {
		return censussourcecontinuation.Terminal{}, errors.New("invalid private constituent discovery terminal")
	}
	outcome := map[privateConstituentDiscoveryOutcome]censussourcecontinuation.Outcome{
		privateDiscoveryCompleteNonempty: censussourcecontinuation.OutcomeCompleteNonempty,
		privateDiscoverySuccessfulEmpty:  censussourcecontinuation.OutcomeSuccessfulEmpty,
		privateDiscoveryPartial:          censussourcecontinuation.OutcomePartial,
		privateDiscoveryFailed:           censussourcecontinuation.OutcomeFailed,
		privateDiscoveryRefused:          censussourcecontinuation.OutcomeRefused,
		privateDiscoveryUnknown:          censussourcecontinuation.OutcomeUnknown,
	}[terminal.outcome]
	if outcome == "" {
		return censussourcecontinuation.Terminal{}, errors.New("invalid private discovery outcome")
	}
	return censussourcecontinuation.Terminal{PlanIdentity: terminal.planIdentity, Authority: terminal.authority, Ordinal: terminal.ordinal, SourceFiles: cloneTerminalSlice(terminal.members), Discovery: clonePrivateTerminalDiscovery(terminal.discovery), Outcome: outcome}, nil
}
