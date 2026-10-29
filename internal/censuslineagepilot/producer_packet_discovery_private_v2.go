package censuslineagepilot

import (
	"fmt"
	"sort"

	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
)

// Validate the archived actual discovery, not a second query or synthesized
// enumeration. File partitions and target batches remain different denominators.
func validatePacketDiscoveryPrivateV2(d censusacquisition.Discovery, files []censusacquisition.SourceFile, maxSymbols int) error {
	if err := censusacquisition.ValidateTerminalDiscovery(d, censusacquisition.TerminalDiscoveryOutcome(d)); err != nil {
		return err
	}
	if !d.Complete || len(d.Observations) != 0 || len(d.IncompleteCauses) != 0 || len(d.FileLedger.Entries) != len(files) || len(d.EnumerationSymbols) != d.Accounting.SymbolDenominator || len(d.EnumerationSymbols) > maxSymbols {
		return ErrInvalid
	}
	ordered := append([]censusacquisition.SourceFile(nil), files...)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Path+"\x00"+ordered[i].URI+"\x00"+ordered[i].LanguageID < ordered[j].Path+"\x00"+ordered[j].URI+"\x00"+ordered[j].LanguageID
	})
	for i, file := range ordered {
		entry := d.FileLedger.Entries[i]
		if entry.Ordinal != i || entry.Identity != file.Path || entry.Disposition != censusacquisition.FileProcessed {
			return ErrInvalid
		}
	}
	targets := make(map[int]censusacquisition.PreparedTarget, len(d.Targets))
	for _, target := range d.Targets {
		if _, ok := targets[target.CensusOrdinal]; ok {
			return ErrInvalid
		}
		targets[target.CensusOrdinal] = target
	}
	for i, symbol := range d.EnumerationSymbols {
		if symbol.Ordinal != i || symbol.FileOrdinal < 0 || symbol.FileOrdinal >= len(ordered) || d.Accounting.Symbols[i].Ordinal != i || d.Accounting.Symbols[i].Disposition != symbol.Disposition {
			return ErrInvalid
		}
		file := ordered[symbol.FileOrdinal]
		entry := d.SymbolLedger.Entries[i]
		identity := fmt.Sprintf("%s#%d:%d:%d:%s:%d", file.Path, symbol.SelectionStart.Line, symbol.SelectionStart.Character, symbol.Kind, symbol.Name, i)
		if entry.Ordinal != i || entry.Identity != identity {
			return ErrInvalid
		}
		callable := symbol.Kind == 6 || symbol.Kind == 9 || symbol.Kind == 12
		if !callable && symbol.Disposition != census.SymbolNonCallable || callable && symbol.Disposition == census.SymbolNonCallable {
			return ErrInvalid
		}
		target, prepared := targets[i]
		if prepared != (symbol.Disposition == census.SymbolSelected) {
			return ErrInvalid
		}
		if prepared && (target.URI != file.URI || target.LanguageID != file.LanguageID || target.Kind != symbol.Kind || target.Name != symbol.Name || target.SymbolIdentity != identity || target.SelectionRange.Start != symbol.SelectionStart) {
			return ErrInvalid
		}
	}
	return nil
}
