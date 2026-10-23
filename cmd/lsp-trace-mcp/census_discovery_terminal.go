package main

import (
	"errors"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censussourceplan"
)

type privateConstituentDiscoveryOutcome string

const (
	privateDiscoveryCompleteNonempty privateConstituentDiscoveryOutcome = "COMPLETE_NONEMPTY"
	privateDiscoverySuccessfulEmpty  privateConstituentDiscoveryOutcome = "SUCCESSFUL_EMPTY"
	privateDiscoveryPartial          privateConstituentDiscoveryOutcome = "PARTIAL"
	privateDiscoveryFailed           privateConstituentDiscoveryOutcome = "FAILED"
	privateDiscoveryRefused          privateConstituentDiscoveryOutcome = "REFUSED"
	privateDiscoveryUnknown          privateConstituentDiscoveryOutcome = "UNKNOWN"
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
	if err := censussourceplan.Validate(plan, censusRuntimeSourcePartitionSize); err != nil {
		return nil, err
	}
	if plan.Authority != 0 || ordinal < 0 || ordinal >= len(plan.Partitions) || plan.Partitions[ordinal].Ordinal != ordinal {
		return nil, censussourceplan.ErrInvalidIdentity
	}
	if err := discovery.Accounting.Validate(); err != nil {
		return nil, err
	}
	if discovery.Closure != censusacquisition.DiscoveryClosureUnknown && discovery.Closure != censusacquisition.DiscoveryClosureSuccessful {
		return nil, errors.New("invalid discovery closure")
	}
	if discovery.Complete && (len(discovery.Targets) == 0 || len(discovery.ProviderFailures) != 0 || len(discovery.IncompleteCauses) != 0) {
		return nil, errors.New("complete discovery conflicts with failure custody")
	}
	if discovery.Closure == censusacquisition.DiscoveryClosureSuccessful && (len(discovery.ProviderFailures) != 0 || len(discovery.IncompleteCauses) != 0) {
		return nil, errors.New("successful discovery closure conflicts with failure custody")
	}
	outcome := privateDiscoveryOutcome(discovery)
	return &privateConstituentDiscoveryTerminal{
		planIdentity: plan.Identity,
		authority:    plan.Authority,
		ordinal:      ordinal,
		members:      cloneTerminalSlice(plan.Partitions[ordinal].Members),
		discovery:    clonePrivateTerminalDiscovery(discovery),
		outcome:      outcome,
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
	clone := discovery
	clone.Accounting.Files = cloneTerminalSlice(discovery.Accounting.Files)
	clone.Accounting.Symbols = cloneTerminalSlice(discovery.Accounting.Symbols)
	clone.FileLedger.Entries = cloneTerminalSlice(discovery.FileLedger.Entries)
	clone.SymbolLedger.Entries = cloneTerminalSlice(discovery.SymbolLedger.Entries)
	clone.Targets = cloneTerminalSlice(discovery.Targets)
	for i := range clone.Targets {
		clone.Targets[i].CanonicalSeedV2 = cloneTerminalSlice(discovery.Targets[i].CanonicalSeedV2)
	}
	clone.EnumerationSymbols = cloneTerminalSlice(discovery.EnumerationSymbols)
	clone.ProviderFailures = cloneTerminalSlice(discovery.ProviderFailures)
	for i := range clone.ProviderFailures {
		clone.ProviderFailures[i].FileOrdinal = cloneInt(clone.ProviderFailures[i].FileOrdinal)
		clone.ProviderFailures[i].SymbolOrdinal = cloneInt(clone.ProviderFailures[i].SymbolOrdinal)
	}
	clone.IncompleteCauses = cloneTerminalSlice(discovery.IncompleteCauses)
	for i := range clone.IncompleteCauses {
		clone.IncompleteCauses[i].FileOrdinal = cloneInt(clone.IncompleteCauses[i].FileOrdinal)
		clone.IncompleteCauses[i].SymbolOrdinal = cloneInt(clone.IncompleteCauses[i].SymbolOrdinal)
	}
	clone.Observations = cloneTerminalSlice(discovery.Observations)
	for i := range clone.Observations {
		clone.Observations[i].Observed = cloneUint64(clone.Observations[i].Observed)
		clone.Observations[i].Limit = cloneUint64(clone.Observations[i].Limit)
		clone.Observations[i].Omitted = cloneUint64(clone.Observations[i].Omitted)
	}
	return clone
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneUint64(value *uint64) *uint64 {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}
