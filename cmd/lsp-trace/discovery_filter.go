package main

import (
	"fmt"
	"path"
	"strings"
)

type discoveryAccounting struct {
	FilesEnumerated          int
	FilesSelected            int
	FilesExcluded            int
	SymbolsEnumerated        int
	SymbolsSelected          int
	SymbolsExcluded          int
	SymbolsUnsupported       int
	SymbolsPreparationFailed int
	SymbolsPrepared          int
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

type noDiscoveryFilesError struct{ accounting discoveryAccounting }

func (e *noDiscoveryFilesError) Error() string {
	return "automatic discovery filters selected no files"
}

func formatDiscoveryAccounting(a discoveryAccounting) string {
	return fmt.Sprintf("automatic discovery accounting: files_enumerated=%d files_selected=%d files_excluded=%d symbols_enumerated=%d symbols_selected=%d symbols_excluded=%d symbols_unsupported=%d symbols_preparation_failed=%d symbols_prepared=%d; operational census only; does not claim endpoint or source completeness",
		a.FilesEnumerated, a.FilesSelected, a.FilesExcluded, a.SymbolsEnumerated, a.SymbolsSelected, a.SymbolsExcluded, a.SymbolsUnsupported, a.SymbolsPreparationFailed, a.SymbolsPrepared)
}
