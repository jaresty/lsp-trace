package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/slicer"
)

func TestResolveSliceSourcesCanonicalizesMixedInputs(t *testing.T) {
	workspace := t.TempDir()
	mustWrite := func(path string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("package p\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(filepath.Join(workspace, "z.go"))
	mustWrite(filepath.Join(workspace, "nested", "a.go"))

	cfg := sliceConfig{workspace: workspace, fromFile: "nested", fromFiles: stringsFlag{"z.go", "nested", "nested/a.go", "z.go"}}
	_, _, sources, err := resolveSliceSources(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(sources))
	for i, source := range sources {
		got[i] = source.path
	}
	want := []string{filepath.Join("nested", "a.go"), "z.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ASSERT_SLICE_SOURCE_CANONICAL_ORDER_AND_DEDUP: got=%q want=%q", got, want)
	}
}

func TestResolveSliceSourcesAppliesGitignoreFiltersWithExcludePrecedence(t *testing.T) {
	workspace := t.TempDir()
	for _, name := range []string{"src/a.go", "src/generated/a.go", "src/a_test.go", "docs/a.go", "src/z.ts"} {
		path := filepath.Join(workspace, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := sliceConfig{
		workspace: workspace, fromFile: ".", fromFiles: stringsFlag{"src", "src/a.go", "."},
		includes: stringsFlag{"src/**/*.go", "docs/*.go"},
		excludes: stringsFlag{"**/*_test.go", "src/generated/**", "docs/**"},
	}
	_, _, sources, accounting, err := resolveSliceSourcesWithAccounting(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(sources))
	for i := range sources {
		got[i] = filepath.ToSlash(sources[i].path)
	}
	if want := []string{"src/a.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ASSERT_DISCOVERY_FILTER_OVERLAP_ORDER_EXCLUDE_WINS: got=%q want=%q", got, want)
	}
	if accounting.FilesEnumerated != 5 || accounting.FilesSelected != 1 || accounting.FilesExcluded != 4 {
		t.Fatalf("ASSERT_DISCOVERY_FILE_ACCOUNTING_EXACT: %#v", accounting)
	}
}

func TestDiscoveryPatternsAreWorkspaceRelativeGitignoreStyle(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"*.go", "main.go", true}, {"*.go", "src/main.go", true},
		{"/src/*.go", "src/main.go", true}, {"/src/*.go", "nested/src/main.go", false},
		{"src/**/main.go", "src/main.go", true}, {"src/**/main.go", "src/nested/main.go", true},
		{"src/?.go", "src/a.go", true}, {"src/?.go", "src/ab.go", false},
		{"src/[ab].go", "src/a.go", true}, {"src/[ab].go", "src/z.go", false},
	}
	for _, tc := range cases {
		if got := matchDiscoveryPattern(tc.pattern, tc.path); got != tc.want {
			t.Errorf("ASSERT_DISCOVERY_GITIGNORE_PATTERN[%s,%s]: got=%t want=%t", tc.pattern, tc.path, got, tc.want)
		}
	}
}

func TestAutomaticDiscoveryAccountingIsExactBoundedAndPathFree(t *testing.T) {
	got := formatDiscoveryAccounting(discoveryAccounting{
		FilesEnumerated: 5, FilesSelected: 2, FilesExcluded: 3,
		SymbolsEnumerated: 9, SymbolsSelected: 9, SymbolsUnsupported: 4,
		SymbolsPreparationFailed: 2, SymbolsPrepared: 3,
	})
	for _, want := range []string{"files_enumerated=5", "files_selected=2", "files_excluded=3", "files_unsupported=0", "files_document_supply_failed=0", "files_document_symbol_failed=0", "files_processed=0", "files_incomplete=0", "symbols_enumerated=9", "symbols_selected=9", "symbols_excluded=0", "symbols_unsupported=4", "symbols_preparation_failed=2", "symbols_prepared=3", "symbols_incomplete=0", "does not claim endpoint or source completeness"} {
		if !strings.Contains(got, want) {
			t.Fatalf("ASSERT_DISCOVERY_ACCOUNTING_EXACT_BOUNDED: missing=%q got=%q", want, got)
		}
	}
	if strings.Contains(got, workspaceSeparatorForTest()) || len(got) > 512 {
		t.Fatalf("ASSERT_DISCOVERY_ACCOUNTING_PRIVACY_BOUNDED: %q", got)
	}
}

func workspaceSeparatorForTest() string { return string(filepath.Separator) + "private" }

func TestDiscoveryCensusContinuesAfterDocumentSupplyFailure(t *testing.T) {
	sources := []resolvedSliceSource{{path: "a.go", uri: "file:///w/a.go"}, {path: "b.go", uri: "file:///w/b.go"}}
	accounting := discoveryAccounting{FilesEnumerated: 2, FilesSelected: 2}
	visited := []string{}
	discoveries, got, err := censusSelectedDiscoverySources(sources, accounting,
		func(source resolvedSliceSource) error {
			if source.path == "a.go" {
				return errors.New("supply failed")
			}
			return nil
		},
		func(source resolvedSliceSource) slicer.Discovery {
			visited = append(visited, source.path)
			return slicer.Discovery{
				PreparationAccounting:     slicer.PreparationAccounting{DocumentSymbols: 1, Attempted: 1, Prepared: 1},
				PreparationDispositions:   []slicer.PreparationDisposition{{Status: "prepared"}},
				PreparationCensusComplete: true,
			}
		})
	if err == nil || len(discoveries) != 1 || !reflect.DeepEqual(visited, []string{"b.go"}) {
		t.Fatalf("ASSERT_DISCOVERY_CONTINUES_AFTER_DOCUMENT_SUPPLY_FAILURE: visited=%q discoveries=%d err=%v", visited, len(discoveries), err)
	}
	if got.FilesDocumentSupplyFailed != 1 || got.FilesProcessed != 1 || got.FilesIncomplete != 0 {
		t.Fatalf("ASSERT_DISCOVERY_FILE_FAILURE_DISPOSITIONS_EXACT: %#v", got)
	}
	if reconcileErr := got.ValidateClosedCensus(); reconcileErr != nil {
		t.Fatalf("ASSERT_DISCOVERY_DENOMINATORS_RECONCILE_EXACTLY: %v accounting=%#v", reconcileErr, got)
	}
}

func TestDiscoveryPatternValidationRejectsInvalidSubsetBeforeMatching(t *testing.T) {
	for _, pattern := range []string{"", "   ", "!src/**", "src/[abc.go", "src/\\"} {
		if err := validateDiscoveryPattern(pattern); err == nil {
			t.Errorf("ASSERT_DISCOVERY_PATTERN_REJECTED_PRESTART[%q]: accepted", pattern)
		}
	}
	for _, pattern := range []string{"*.go", "/src/*.go", "src/**/main.go", "src/[ab].go", "docs/"} {
		if err := validateDiscoveryPattern(pattern); err != nil {
			t.Errorf("ASSERT_DISCOVERY_PATTERN_SUBSET_ACCEPTED[%q]: %v", pattern, err)
		}
	}
}

func TestDiscoveryAccountingRejectsMismatchedDenominators(t *testing.T) {
	bad := discoveryAccounting{FilesEnumerated: 3, FilesSelected: 2, FilesExcluded: 1, FilesProcessed: 1}
	if err := bad.ValidateClosedCensus(); err == nil {
		t.Fatal("ASSERT_DISCOVERY_DENOMINATORS_RECONCILE_EXACTLY: mismatch accepted")
	}
}

func TestDiscoverySymbolCensusRejectsEveryUnreconciledReturnedSymbol(t *testing.T) {
	valid := slicer.Discovery{
		PreparationAccounting:   slicer.PreparationAccounting{DocumentSymbols: 3, Attempted: 3, Prepared: 1, NotPreparable: 1, Failed: 1},
		PreparationDispositions: []slicer.PreparationDisposition{{Status: "prepared"}, {Status: "not_preparable"}, {Status: "failed"}},
	}
	if err := validateDiscoverySymbolCensus(valid); err != nil {
		t.Fatalf("ASSERT_DISCOVERY_EVERY_RETURNED_SYMBOL_RECONCILES: valid census rejected: %v", err)
	}
	cases := map[string]slicer.Discovery{
		"attempted":       {PreparationAccounting: slicer.PreparationAccounting{DocumentSymbols: 3, Attempted: 2, Prepared: 1, NotPreparable: 1}, PreparationDispositions: valid.PreparationDispositions[:2]},
		"missing":         {PreparationAccounting: valid.PreparationAccounting, PreparationDispositions: valid.PreparationDispositions[:2]},
		"unknown":         {PreparationAccounting: valid.PreparationAccounting, PreparationDispositions: []slicer.PreparationDisposition{{Status: "prepared"}, {Status: "not_preparable"}, {Status: "incomplete"}}},
		"reported-counts": {PreparationAccounting: slicer.PreparationAccounting{DocumentSymbols: 3, Attempted: 3, Prepared: 2, NotPreparable: 1}, PreparationDispositions: valid.PreparationDispositions},
	}
	for name, discovery := range cases {
		if err := validateDiscoverySymbolCensus(discovery); err == nil {
			t.Errorf("ASSERT_DISCOVERY_RETURNED_SYMBOL_GAP_FAILS_CLOSED[%s]: malformed census accepted", name)
		}
	}
}

func TestDiscoveryCensusAccountsMalformedReturnedSymbolsAsIncomplete(t *testing.T) {
	sources := []resolvedSliceSource{{path: "a.go", uri: "file:///w/a.go"}}
	_, got, err := censusSelectedDiscoverySources(sources, discoveryAccounting{FilesEnumerated: 1, FilesSelected: 1},
		func(resolvedSliceSource) error { return nil },
		func(resolvedSliceSource) slicer.Discovery {
			return slicer.Discovery{
				PreparationAccounting:   slicer.PreparationAccounting{DocumentSymbols: 2, Attempted: 1, Prepared: 1},
				PreparationDispositions: []slicer.PreparationDisposition{{Status: "prepared"}},
			}
		})
	if err == nil || got.SymbolsEnumerated != 2 || got.SymbolsSelected != 2 || got.SymbolsPrepared != 1 || got.SymbolsIncomplete != 1 || got.FilesIncomplete != 1 {
		t.Fatalf("ASSERT_DISCOVERY_MALFORMED_SYMBOL_CENSUS_EXACT: accounting=%#v err=%v", got, err)
	}
	if reconcileErr := got.ValidateClosedCensus(); reconcileErr != nil {
		t.Fatalf("ASSERT_DISCOVERY_MALFORMED_SYMBOL_DENOMINATOR_CLOSED: %v accounting=%#v", reconcileErr, got)
	}
}

func TestResolveSliceSourcesRejectsEscapeAndSkipsNestedSymlink(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "outside.go")
	if err := os.WriteFile(outsideFile, []byte("package outside\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := resolveSliceSources(sliceConfig{workspace: workspace, fromFile: "../outside.go", fromFiles: stringsFlag{"../outside.go"}}); err == nil || !strings.Contains(err.Error(), "remain within workspace") {
		t.Fatalf("ASSERT_SLICE_SOURCE_WORKSPACE_ESCAPE_REJECTED: err=%v", err)
	}

	inside := filepath.Join(workspace, "inside.go")
	if err := os.WriteFile(inside, []byte("package inside\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideFile, filepath.Join(workspace, "linked.go")); err != nil {
		t.Fatal(err)
	}
	_, _, sources, err := resolveSliceSources(sliceConfig{workspace: workspace, fromFile: ".", fromFiles: stringsFlag{"."}})
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 || sources[0].path != "inside.go" {
		t.Fatalf("ASSERT_SLICE_SOURCE_NESTED_SYMLINK_EXCLUDED: sources=%v", sources)
	}
}
