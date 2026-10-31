package censuslineagepilot

import (
	"fmt"
	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
	"sort"
)

func validateTerminalSourceLedgerPrivateV2(d censusacquisition.Discovery, files []censusacquisition.SourceFile, maxSymbols, maxStringBytes int) error {
	// Every nominated file must be selected even when its symbol set is empty.
	// FileLedger alone cannot establish the separate accounting disposition.
	if len(d.Accounting.Files) != len(files) {
		return ErrInvalid
	}
	for i, file := range d.Accounting.Files {
		if file.Ordinal != i || file.Disposition != census.FileSelected {
			return ErrInvalid
		}
	}
	// Detached projection retains names, not original Detail or tree structure.
	// This enforces only a necessary per-file byte lower bound, not the full
	// acquisition MaxStringBytes/MaxDepth observation (see normative boundary).
	nameBytes := make([]int, len(files))
	for _, symbol := range d.EnumerationSymbols {
		if symbol.FileOrdinal < 0 || symbol.FileOrdinal >= len(files) || len(symbol.Name) > maxStringBytes-nameBytes[symbol.FileOrdinal] {
			return ErrInvalid
		}
		nameBytes[symbol.FileOrdinal] += len(symbol.Name)
	}
	if len(d.Targets) > 0 {
		return validatePacketDiscoveryPrivateV2(d, files, maxSymbols)
	}
	if len(d.EnumerationSymbols) > maxSymbols || len(d.FileLedger.Entries) != len(files) || len(d.Accounting.Files) != len(files) || len(d.Accounting.Symbols) != len(d.EnumerationSymbols) || len(d.SymbolLedger.Entries) != len(d.EnumerationSymbols) {
		return ErrInvalid
	}
	ordered := append([]censusacquisition.SourceFile(nil), files...)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Path+"\x00"+ordered[i].URI+"\x00"+ordered[i].LanguageID < ordered[j].Path+"\x00"+ordered[j].URI+"\x00"+ordered[j].LanguageID
	})
	for i, file := range ordered {
		entry := d.FileLedger.Entries[i]
		if entry.Ordinal != i || entry.Identity != file.Path || entry.Disposition != censusacquisition.FileProcessed || d.Accounting.Files[i].Ordinal != i {
			return ErrInvalid
		}
	}
	for i, symbol := range d.EnumerationSymbols {
		if symbol.Ordinal != i || symbol.FileOrdinal < 0 || symbol.FileOrdinal >= len(ordered) || d.Accounting.Symbols[i].Ordinal != i || d.Accounting.Symbols[i].Disposition != census.SymbolNonCallable || symbol.Disposition != census.SymbolNonCallable || symbol.Kind == 6 || symbol.Kind == 9 || symbol.Kind == 12 {
			return ErrInvalid
		}
		identity := fmt.Sprintf("%s#%d:%d:%d:%s:%d", ordered[symbol.FileOrdinal].Path, symbol.SelectionStart.Line, symbol.SelectionStart.Character, symbol.Kind, symbol.Name, i)
		entry := d.SymbolLedger.Entries[i]
		if entry.Ordinal != i || entry.Identity != identity || entry.Disposition != string(census.SymbolNonCallable) {
			return ErrInvalid
		}
	}
	return nil
}
