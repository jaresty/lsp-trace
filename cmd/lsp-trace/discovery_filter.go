package main

import (
	"fmt"
	"path"
	"strings"

	"lsp-trace/internal/discoveryfilter"
	"lsp-trace/internal/slicer"
)

type discoveryAccounting struct {
	FilesEnumerated           int
	FilesSelected             int
	FilesExcluded             int
	FilesUnsupported          int
	FilesDocumentSupplyFailed int
	FilesDocumentSymbolFailed int
	FilesProcessed            int
	FilesIncomplete           int
	SymbolsEnumerated         int
	SymbolsSelected           int
	SymbolsExcluded           int
	SymbolsUnsupported        int
	SymbolsPreparationFailed  int
	SymbolsPrepared           int
	SymbolsIncomplete         int
}

func (a discoveryAccounting) ValidateClosedCensus() error {
	if a.FilesEnumerated != a.FilesExcluded+a.FilesSelected {
		return fmt.Errorf("file enumeration denominator does not reconcile")
	}
	if a.FilesSelected != a.FilesUnsupported+a.FilesDocumentSupplyFailed+a.FilesDocumentSymbolFailed+a.FilesProcessed+a.FilesIncomplete {
		return fmt.Errorf("selected file denominator does not reconcile")
	}
	if a.SymbolsEnumerated != a.SymbolsExcluded+a.SymbolsSelected {
		return fmt.Errorf("symbol enumeration denominator does not reconcile")
	}
	if a.SymbolsSelected != a.SymbolsUnsupported+a.SymbolsPreparationFailed+a.SymbolsPrepared+a.SymbolsIncomplete {
		return fmt.Errorf("selected symbol denominator does not reconcile")
	}
	return nil
}

func validateDiscoveryPattern(pattern string) error {
	return discoveryfilter.ValidatePattern(pattern)
}

func validateDiscoveryPatterns(includes, excludes []string) error {
	return discoveryfilter.ValidatePatterns(includes, excludes)
}

func matchDiscoveryPattern(pattern, candidate string) bool {
	pattern = strings.TrimSpace(strings.ReplaceAll(pattern, "\\", "/"))
	candidate = strings.TrimPrefix(strings.ReplaceAll(candidate, "\\", "/"), "./")
	if pattern == "" || strings.HasPrefix(pattern, "!") {
		return false
	}
	anchored := strings.HasPrefix(pattern, "/")
	pattern = strings.TrimPrefix(pattern, "/")
	if !strings.Contains(pattern, "/") && !anchored {
		matched, _ := path.Match(pattern, path.Base(candidate))
		return matched
	}
	if strings.HasSuffix(pattern, "/") {
		pattern += "**"
	}
	return matchDiscoverySegments(strings.Split(pattern, "/"), strings.Split(candidate, "/"))
}

func matchDiscoverySegments(pattern, candidate []string) bool {
	if len(pattern) == 0 {
		return len(candidate) == 0
	}
	if pattern[0] == "**" {
		return matchDiscoverySegments(pattern[1:], candidate) || (len(candidate) > 0 && matchDiscoverySegments(pattern, candidate[1:]))
	}
	if len(candidate) == 0 {
		return false
	}
	matched, err := path.Match(pattern[0], candidate[0])
	return err == nil && matched && matchDiscoverySegments(pattern[1:], candidate[1:])
}

func matchesAnyDiscoveryPattern(patterns []string, candidate string) bool {
	for _, pattern := range patterns {
		if matchDiscoveryPattern(pattern, candidate) {
			return true
		}
	}
	return false
}

func resolveSliceSourcesWithAccounting(cfg sliceConfig) (string, string, []resolvedSliceSource, discoveryAccounting, error) {
	if err := validateDiscoveryPatterns(cfg.includes, cfg.excludes); err != nil {
		return "", "", nil, discoveryAccounting{}, err
	}
	workspaceURI, scopeURI, sources, err := resolveSliceSources(cfg)
	accounting := discoveryAccounting{FilesEnumerated: len(sources)}
	if err != nil {
		return workspaceURI, scopeURI, nil, accounting, err
	}
	selected := make([]resolvedSliceSource, 0, len(sources))
	for _, source := range sources {
		candidate := path.Clean(strings.ReplaceAll(source.path, "\\", "/"))
		included := len(cfg.includes) == 0 || matchesAnyDiscoveryPattern(cfg.includes, candidate)
		excluded := matchesAnyDiscoveryPattern(cfg.excludes, candidate)
		if !included || excluded {
			accounting.FilesExcluded++
			continue
		}
		selected = append(selected, source)
	}
	accounting.FilesSelected = len(selected)
	if len(selected) == 0 {
		return workspaceURI, scopeURI, nil, accounting, &noDiscoveryFilesError{accounting: accounting}
	}
	if len(selected) == 1 {
		scopeURI = selected[0].uri
	} else {
		scopeURI = workspaceURI
	}
	return workspaceURI, scopeURI, selected, accounting, nil
}

func validateDiscoverySymbolCensus(discovery slicer.Discovery) error {
	counts := discovery.PreparationAccounting
	if counts.DocumentSymbols != counts.Attempted || counts.Attempted != len(discovery.PreparationDispositions) {
		return fmt.Errorf("returned symbol denominator does not reconcile")
	}
	prepared, unsupported, failed := 0, 0, 0
	for _, disposition := range discovery.PreparationDispositions {
		switch disposition.Status {
		case "prepared":
			prepared++
		case "not_preparable":
			unsupported++
		case "failed":
			failed++
		default:
			return fmt.Errorf("returned symbol disposition does not reconcile")
		}
	}
	if counts.Prepared != prepared || counts.NotPreparable != unsupported || counts.Failed != failed || counts.Attempted != prepared+unsupported+failed {
		return fmt.Errorf("returned symbol dispositions do not reconcile")
	}
	return nil
}

func censusSelectedDiscoverySources(sources []resolvedSliceSource, accounting discoveryAccounting, prepare func(resolvedSliceSource) error, discover func(resolvedSliceSource) slicer.Discovery) (map[string]slicer.Discovery, discoveryAccounting, error) {
	discoveries := make(map[string]slicer.Discovery, len(sources))
	failed := false
	for _, source := range sources {
		if err := prepare(source); err != nil {
			accounting.FilesDocumentSupplyFailed++
			failed = true
			continue
		}
		discovery := discover(source)
		counts := discovery.PreparationAccounting
		accounting.SymbolsEnumerated += counts.DocumentSymbols
		accounting.SymbolsSelected += counts.DocumentSymbols // every returned symbol is selected for preparation
		dispositioned := 0
		for _, disposition := range discovery.PreparationDispositions {
			switch disposition.Status {
			case "prepared":
				accounting.SymbolsPrepared++
				dispositioned++
			case "not_preparable":
				accounting.SymbolsUnsupported++
				dispositioned++
			case "failed":
				accounting.SymbolsPreparationFailed++
				dispositioned++
			}
		}
		if dispositioned < counts.DocumentSymbols {
			accounting.SymbolsIncomplete += counts.DocumentSymbols - dispositioned
		}
		if err := validateDiscoverySymbolCensus(discovery); err != nil {
			accounting.FilesIncomplete++
			failed = true
			continue
		}
		discoveries[source.uri] = discovery
		fileDisposition := "processed"
		for _, diagnostic := range discovery.Diagnostics {
			if diagnostic.Phase == "slice-symbols" {
				fileDisposition = "document-symbol-failed"
				if strings.Contains(diagnostic.Message, "unsupported") {
					fileDisposition = "unsupported"
				}
				break
			}
		}
		switch fileDisposition {
		case "unsupported":
			accounting.FilesUnsupported++
			failed = true
		case "document-symbol-failed":
			accounting.FilesDocumentSymbolFailed++
			failed = true
		case "processed":
			if discovery.PreparationCensusComplete {
				accounting.FilesProcessed++
			} else {
				accounting.FilesIncomplete++
				failed = true
			}
		}
	}
	if err := accounting.ValidateClosedCensus(); err != nil {
		return discoveries, accounting, err
	}
	if failed {
		return discoveries, accounting, fmt.Errorf("automatic seed discovery incomplete")
	}
	return discoveries, accounting, nil
}

type noDiscoveryFilesError struct{ accounting discoveryAccounting }

func (e *noDiscoveryFilesError) Error() string {
	return "automatic discovery filters selected no files"
}

func formatDiscoveryAccounting(a discoveryAccounting) string {
	return fmt.Sprintf("automatic discovery accounting: files_enumerated=%d files_selected=%d files_excluded=%d files_unsupported=%d files_document_supply_failed=%d files_document_symbol_failed=%d files_processed=%d files_incomplete=%d symbols_enumerated=%d symbols_selected=%d symbols_excluded=%d symbols_unsupported=%d symbols_preparation_failed=%d symbols_prepared=%d symbols_incomplete=%d; operational census only; does not claim endpoint or source completeness",
		a.FilesEnumerated, a.FilesSelected, a.FilesExcluded, a.FilesUnsupported, a.FilesDocumentSupplyFailed, a.FilesDocumentSymbolFailed, a.FilesProcessed, a.FilesIncomplete, a.SymbolsEnumerated, a.SymbolsSelected, a.SymbolsExcluded, a.SymbolsUnsupported, a.SymbolsPreparationFailed, a.SymbolsPrepared, a.SymbolsIncomplete)
}
