package censussourceassembly

import (
	"errors"
	"reflect"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/censussourceplan"
)

type Outcome string

const (
	OutcomeCompleteNonempty Outcome = "COMPLETE_NONEMPTY"
	OutcomeSuccessfulEmpty  Outcome = "SUCCESSFUL_EMPTY"
	OutcomePartial          Outcome = "PARTIAL"
	OutcomeFailed           Outcome = "FAILED"
	OutcomeRefused          Outcome = "REFUSED"
	OutcomeUnknown          Outcome = "UNKNOWN"
)

type TerminalRecord struct {
	PlanIdentity string
	Authority    uint8
	Ordinal      int
	Members      []censusacquisition.SourceFile
	Discovery    censusacquisition.Discovery
	Outcome      Outcome
}

func NewTerminalRecord(plan censussourceplan.Plan, partitionSize, ordinal int, discovery censusacquisition.Discovery) (*TerminalRecord, error) {
	if err := censussourceplan.Validate(plan, partitionSize); err != nil {
		return nil, err
	}
	if plan.Authority != 0 || ordinal < 0 || ordinal >= len(plan.Partitions) || plan.Partitions[ordinal].Ordinal != ordinal {
		return nil, censussourceplan.ErrInvalidIdentity
	}
	outcome := Outcome(censusacquisition.TerminalDiscoveryOutcome(discovery))
	if err := censusacquisition.ValidateTerminalDiscovery(discovery, string(outcome)); err != nil {
		return nil, err
	}
	return &TerminalRecord{
		PlanIdentity: plan.Identity,
		Authority:    plan.Authority,
		Ordinal:      ordinal,
		Members:      cloneSlice(plan.Partitions[ordinal].Members),
		Discovery:    cloneDiscovery(discovery),
		Outcome:      outcome,
	}, nil
}

func AdaptTerminal(plan censussourceplan.Plan, partitionSize int, record *TerminalRecord) (censussourcecontinuation.Terminal, error) {
	if record == nil || censussourceplan.Validate(plan, partitionSize) != nil || record.PlanIdentity != plan.Identity || record.Authority != 0 || record.Ordinal < 0 || record.Ordinal >= len(plan.Partitions) || !reflect.DeepEqual(record.Members, plan.Partitions[record.Ordinal].Members) {
		return censussourcecontinuation.Terminal{}, errors.New("invalid census source terminal record")
	}
	if err := censusacquisition.ValidateTerminalDiscovery(record.Discovery, string(record.Outcome)); err != nil {
		return censussourcecontinuation.Terminal{}, errors.New("invalid census source terminal discovery")
	}
	outcome := map[Outcome]censussourcecontinuation.Outcome{
		OutcomeCompleteNonempty: censussourcecontinuation.OutcomeCompleteNonempty,
		OutcomeSuccessfulEmpty:  censussourcecontinuation.OutcomeSuccessfulEmpty,
		OutcomePartial:          censussourcecontinuation.OutcomePartial,
		OutcomeFailed:           censussourcecontinuation.OutcomeFailed,
		OutcomeRefused:          censussourcecontinuation.OutcomeRefused,
		OutcomeUnknown:          censussourcecontinuation.OutcomeUnknown,
	}[record.Outcome]
	if outcome == "" {
		return censussourcecontinuation.Terminal{}, errors.New("invalid census source terminal outcome")
	}
	return censussourcecontinuation.Terminal{
		PlanIdentity: record.PlanIdentity,
		Authority:    record.Authority,
		Ordinal:      record.Ordinal,
		SourceFiles:  cloneSlice(record.Members),
		Discovery:    cloneDiscovery(record.Discovery),
		Outcome:      outcome,
	}, nil
}

func cloneSlice[T any](values []T) []T {
	if values == nil {
		return nil
	}
	clone := make([]T, len(values))
	copy(clone, values)
	return clone
}

// CloneDiscovery preserves nilness and returns discovery custody without mutable aliases.
func CloneDiscovery(discovery censusacquisition.Discovery) censusacquisition.Discovery {
	return cloneDiscovery(discovery)
}

func cloneDiscovery(discovery censusacquisition.Discovery) censusacquisition.Discovery {
	clone := discovery
	clone.Accounting.Files = cloneSlice(discovery.Accounting.Files)
	clone.Accounting.Symbols = cloneSlice(discovery.Accounting.Symbols)
	clone.FileLedger.Entries = cloneSlice(discovery.FileLedger.Entries)
	clone.SymbolLedger.Entries = cloneSlice(discovery.SymbolLedger.Entries)
	clone.Targets = cloneSlice(discovery.Targets)
	for i := range clone.Targets {
		clone.Targets[i].CanonicalSeedV2 = cloneSlice(discovery.Targets[i].CanonicalSeedV2)
	}
	clone.EnumerationSymbols = cloneSlice(discovery.EnumerationSymbols)
	clone.ProviderFailures = cloneSlice(discovery.ProviderFailures)
	for i := range clone.ProviderFailures {
		clone.ProviderFailures[i].FileOrdinal = cloneInt(discovery.ProviderFailures[i].FileOrdinal)
		clone.ProviderFailures[i].SymbolOrdinal = cloneInt(discovery.ProviderFailures[i].SymbolOrdinal)
	}
	clone.IncompleteCauses = cloneSlice(discovery.IncompleteCauses)
	for i := range clone.IncompleteCauses {
		clone.IncompleteCauses[i].FileOrdinal = cloneInt(discovery.IncompleteCauses[i].FileOrdinal)
		clone.IncompleteCauses[i].SymbolOrdinal = cloneInt(discovery.IncompleteCauses[i].SymbolOrdinal)
	}
	clone.Observations = cloneSlice(discovery.Observations)
	for i := range clone.Observations {
		clone.Observations[i].Observed = cloneUint64(discovery.Observations[i].Observed)
		clone.Observations[i].Limit = cloneUint64(discovery.Observations[i].Limit)
		clone.Observations[i].Omitted = cloneUint64(discovery.Observations[i].Omitted)
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
