package censusacquisition

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/census"
	"lsp-trace/internal/lsp"
)

type fakeDiscoveryClient struct {
	documentSupported, callSupported bool
	symbols                          map[string][]lsp.DocumentSymbol
	documentErrors                   map[string]error
	prepare                          map[lsp.Position][]lsp.CallHierarchyItem
	prepareErrors                    map[lsp.Position]error
	documentRequests                 []string
	prepareRequests                  []lsp.Position
}

func (f *fakeDiscoveryClient) SupportsDocumentSymbols() bool { return f.documentSupported }
func (f *fakeDiscoveryClient) SupportsCallHierarchy() bool   { return f.callSupported }
func (f *fakeDiscoveryClient) DocumentSymbols(_ context.Context, p lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error) {
	uri := p.TextDocument.URI
	f.documentRequests = append(f.documentRequests, uri)
	return append([]lsp.DocumentSymbol(nil), f.symbols[uri]...), f.documentErrors[uri]
}
func (f *fakeDiscoveryClient) PrepareCallHierarchy(_ context.Context, p lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	f.prepareRequests = append(f.prepareRequests, p.Position)
	return append([]lsp.CallHierarchyItem(nil), f.prepare[p.Position]...), f.prepareErrors[p.Position]
}

type fakeSupplier struct {
	calls  []string
	errors map[string]error
}

func (f *fakeSupplier) Supply(_ context.Context, file SourceFile) error {
	f.calls = append(f.calls, file.Path)
	return f.errors[file.Path]
}

func testSource(workspace, name string) SourceFile {
	return SourceFile{Path: name, URI: (&url.URL{Scheme: "file", Path: filepath.Join(workspace, filepath.FromSlash(name))}).String(), LanguageID: "go"}
}
func sym(name string, kind int, line, char uint32, children ...lsp.DocumentSymbol) lsp.DocumentSymbol {
	p := lsp.Position{Line: line, Character: char}
	return lsp.DocumentSymbol{Name: name, Kind: kind, Range: lsp.Range{Start: p, End: p}, SelectionRange: lsp.Range{Start: p, End: lsp.Position{Line: line, Character: char + 9}}, Children: children}
}
func item(name, uri string, kind int, line, char uint32) lsp.CallHierarchyItem {
	p := lsp.Position{Line: line, Character: char}
	return lsp.CallHierarchyItem{Name: name, Kind: kind, URI: uri, Range: lsp.Range{Start: p, End: p}, SelectionRange: lsp.Range{Start: p, End: p}}
}

func TestDiscoveryAdapterDeterministicFlattenSeedsAndImmutableInputs(t *testing.T) {
	workspace := t.TempDir()
	a, z := testSource(workspace, "a.go"), testSource(workspace, "z.go")
	child := sym("Child", 6, 4, 7)
	root := sym("Root", 12, 8, 2, child)
	zsym := sym("Zed", 9, 1, 3)
	files := StaticFiles{z, a}
	original := append(StaticFiles(nil), files...)
	client := &fakeDiscoveryClient{documentSupported: true, callSupported: true, symbols: map[string][]lsp.DocumentSymbol{a.URI: {root}, z.URI: {zsym}}, prepare: map[lsp.Position][]lsp.CallHierarchyItem{child.SelectionRange.Start: {item("Child", a.URI, 6, 4, 7)}, root.SelectionRange.Start: {item("Root", a.URI, 12, 8, 2)}, zsym.SelectionRange.Start: {item("Zed", z.URI, 9, 1, 3)}}}
	supplier := &fakeSupplier{errors: map[string]error{}}
	adapter := DiscoveryAdapter{Workspace: workspace, Files: files, Supplier: supplier, Client: client}
	got, err := adapter.Discover(context.Background(), SessionIdentity{"ready", 2})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Complete {
		t.Fatal("ASSERT_COMPLETE_EXACT_BIJECTION: complete census marked incomplete")
	}
	if !reflect.DeepEqual(files, original) {
		t.Fatal("ASSERT_DISCOVERY_INPUT_IMMUTABLE: source input mutated")
	}
	if !reflect.DeepEqual(supplier.calls, []string{"a.go", "z.go"}) {
		t.Fatalf("ASSERT_FILE_CANONICAL_ORDER: %v", supplier.calls)
	}
	wantPositions := []lsp.Position{{Line: 4, Character: 7}, {Line: 8, Character: 2}, {Line: 1, Character: 3}}
	if !reflect.DeepEqual(client.prepareRequests, wantPositions) {
		t.Fatalf("ASSERT_FLATTEN_SORT_SELECTION_START: %v", client.prepareRequests)
	}
	if len(got.Targets) != 3 || got.Accounting.SymbolDenominator != 3 || got.SymbolLedger.Denominator != 3 {
		t.Fatal("ASSERT_SELECTED_PREPARED_TARGET_BIJECTION")
	}
	for i, target := range got.Targets {
		if target.CensusOrdinal != i || target.SymbolIdentity == "" || !validRange(target.SelectionRange) {
			t.Fatalf("ASSERT_PREPARED_IDENTITY_RETAINED[%d]: %+v", i, target)
		}
		var seed struct {
			Coordinate string `json:"coordinate_convention"`
			Seeds      []struct {
				Path         string `json:"path"`
				Line, Column uint64
			} `json:"seeds"`
		}
		if err := json.Unmarshal(target.CanonicalSeedV2, &seed); err != nil {
			t.Fatal(err)
		}
		if seed.Coordinate != "one-based" || len(seed.Seeds) != 1 || seed.Seeds[0].Line != uint64(target.Position().Line)+1 || seed.Seeds[0].Column != uint64(target.Position().Character)+1 {
			t.Fatalf("ASSERT_CANONICAL_ONE_BASED_SINGLE_POSITION[%d]: %s", i, target.CanonicalSeedV2)
		}
	}
	again, err := adapter.Discover(context.Background(), SessionIdentity{"ready", 2})
	if err != nil || !reflect.DeepEqual(got, again) {
		t.Fatal("ASSERT_DISCOVERY_DETERMINISTIC", err)
	}
}

func TestDiscoveryAdapterExclusionsWinBeforeIncludes(t *testing.T) {
	workspace := t.TempDir()
	keep, drop := testSource(workspace, "src/keep.go"), testSource(workspace, "src/drop.go")
	s := sym("Keep", 12, 0, 0)
	client := &fakeDiscoveryClient{documentSupported: true, callSupported: true, symbols: map[string][]lsp.DocumentSymbol{keep.URI: {s}}, prepare: map[lsp.Position][]lsp.CallHierarchyItem{s.SelectionRange.Start: {item("Keep", keep.URI, 12, 0, 0)}}}
	supplier := &fakeSupplier{errors: map[string]error{}}
	got, err := (DiscoveryAdapter{Workspace: workspace, Files: StaticFiles{drop, keep}, Supplier: supplier, Client: client, Filters: Filters{Includes: []string{"src/**"}, Excludes: []string{"**/drop.go"}}}).Discover(context.Background(), SessionIdentity{"s", 1})
	if err != nil || !got.Complete {
		t.Fatal(err, got.Complete)
	}
	if !reflect.DeepEqual(supplier.calls, []string{"src/keep.go"}) {
		t.Fatalf("ASSERT_EXCLUSION_PRECEDENCE: %v", supplier.calls)
	}
	if got.Accounting.Files[0].Disposition != census.FileExcluded || got.Accounting.Files[1].Disposition != census.FileSelected {
		t.Fatalf("ASSERT_FILE_TERMINALS: %+v", got.Accounting.Files)
	}
}

func TestDiscoveryAdapterExplicitFailureDispositionsAndIncomplete(t *testing.T) {
	workspace := t.TempDir()
	names := []string{"bad-symbols.go", "missing.go", "noncallable.go", "prepare-error.go", "supply.go", "unsupported.go"}
	files := make(StaticFiles, len(names))
	for i, n := range names {
		files[i] = testSource(workspace, n)
	}
	by := map[string]SourceFile{}
	for _, f := range files {
		by[f.Path] = f
	}
	missing := sym("Missing", 12, 1, 0)
	noncall := sym("Field", 8, 2, 0)
	failed := sym("Failed", 6, 3, 0)
	client := &fakeDiscoveryClient{documentSupported: true, callSupported: true, symbols: map[string][]lsp.DocumentSymbol{by["missing.go"].URI: {missing}, by["noncallable.go"].URI: {noncall}, by["prepare-error.go"].URI: {failed}}, documentErrors: map[string]error{by["bad-symbols.go"].URI: errors.New("symbols failed")}, prepare: map[lsp.Position][]lsp.CallHierarchyItem{}, prepareErrors: map[lsp.Position]error{failed.SelectionRange.Start: errors.New("prepare failed")}}
	supplier := &fakeSupplier{errors: map[string]error{"supply.go": errors.New("unreadable")}}
	files[5].Disposition = census.FileUnsupported
	got, err := (DiscoveryAdapter{Workspace: workspace, Files: files, Supplier: supplier, Client: client}).Discover(context.Background(), SessionIdentity{"s", 1})
	if err != nil {
		t.Fatal(err)
	}
	if got.Complete {
		t.Fatal("ASSERT_FAILURES_KEEP_INCOMPLETE")
	}
	fileStatuses := map[census.FileDisposition]bool{}
	for _, e := range got.Accounting.Files {
		fileStatuses[e.Disposition] = true
	}
	for _, want := range []census.FileDisposition{census.FileDocumentSymbolFailed, census.FileUnreadable, census.FileUnsupported, census.FileIncomplete} {
		if !fileStatuses[want] {
			t.Fatalf("ASSERT_FILE_FAILURE_ACCOUNTED[%s]: %+v", want, got.Accounting.Files)
		}
	}
	symbolStatuses := map[census.SymbolDisposition]bool{}
	for _, e := range got.Accounting.Symbols {
		symbolStatuses[e.Disposition] = true
	}
	for _, want := range []census.SymbolDisposition{census.SymbolPrepareMissing, census.SymbolNonCallable, census.SymbolPreparationFailed} {
		if !symbolStatuses[want] {
			t.Fatalf("ASSERT_SYMBOL_FAILURE_ACCOUNTED[%s]: %+v", want, got.Accounting.Symbols)
		}
	}
	if err := got.Accounting.Validate(); err != nil {
		t.Fatal("ASSERT_FAILURE_ACCOUNTING_CLOSED", err)
	}
}

func TestDiscoveryAdapterUnsupportedPreparationMalformedAndBounds(t *testing.T) {
	workspace := t.TempDir()
	f := testSource(workspace, "f.go")
	symbols := []lsp.DocumentSymbol{sym("A", 12, 0, 0), sym("B", 12, 1, 0), sym("C", 12, 2, 0)}
	client := &fakeDiscoveryClient{documentSupported: true, callSupported: false, symbols: map[string][]lsp.DocumentSymbol{f.URI: symbols}, prepare: map[lsp.Position][]lsp.CallHierarchyItem{}}
	got, err := (DiscoveryAdapter{Workspace: workspace, Files: StaticFiles{f}, Supplier: &fakeSupplier{errors: map[string]error{}}, Client: client, Limits: DiscoveryLimits{MaxSymbols: 2}}).Discover(context.Background(), SessionIdentity{"s", 1})
	if err != nil {
		t.Fatal(err)
	}
	want := []census.SymbolDisposition{census.SymbolUnsupported, census.SymbolUnsupported, census.SymbolIncomplete}
	var have []census.SymbolDisposition
	for _, e := range got.Accounting.Symbols {
		have = append(have, e.Disposition)
	}
	if !reflect.DeepEqual(have, want) || len(client.prepareRequests) != 0 || got.Complete {
		t.Fatalf("ASSERT_BOUNDED_UNSUPPORTED_TERMINALS: %v", have)
	}

	client.callSupported = true
	client.prepare[symbols[0].SelectionRange.Start] = []lsp.CallHierarchyItem{item("one", f.URI, 12, 0, 0), item("two", f.URI, 12, 0, 0)}
	client.prepare[symbols[1].SelectionRange.Start] = []lsp.CallHierarchyItem{{Name: "outside", Kind: 12, URI: "file:///outside.go"}}
	got, err = (DiscoveryAdapter{Workspace: workspace, Files: StaticFiles{f}, Supplier: &fakeSupplier{errors: map[string]error{}}, Client: client}).Discover(context.Background(), SessionIdentity{"s", 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range got.Accounting.Symbols {
		if e.Disposition == census.SymbolSelected {
			t.Fatal("ASSERT_MALFORMED_PREPARE_NOT_SELECTED")
		}
	}
	if got.Complete {
		t.Fatal("ASSERT_MALFORMED_PREPARE_INCOMPLETE")
	}
}

func TestDiscoveryAdapterRejectsDuplicateCanonicalFilesAndPreservesWindowsNormalizationOrder(t *testing.T) {
	workspace := t.TempDir()
	f := testSource(workspace, "Dir/F.go")
	old := runtimeGOOS
	runtimeGOOS = func() string { return "windows" }
	defer func() { runtimeGOOS = old }()
	duplicate := f
	duplicate.Path = `dir/f.go`
	_, err := (DiscoveryAdapter{Workspace: workspace, Files: StaticFiles{f, duplicate}, Supplier: &fakeSupplier{}, Client: &fakeDiscoveryClient{}}).Discover(context.Background(), SessionIdentity{"s", 1})
	if err == nil {
		t.Fatal("ASSERT_WINDOWS_SLASH_NORMALIZE_BEFORE_CASE_FOLD")
	}
}

func TestStaticFilesReturnsIndependentSlice(t *testing.T) {
	in := StaticFiles{{Path: "a.go"}}
	got, _ := in.Enumerate(context.Background())
	got[0].Path = "changed"
	if in[0].Path != "a.go" {
		t.Fatal("ASSERT_ENUMERATOR_IMMUTABLE")
	}
	sort.Slice(got, func(i, j int) bool { return false })
}

func TestDiscoverToCoreRunAcquiresNestedSameFileTargets(t *testing.T) {
	workspace := t.TempDir()
	f := testSource(workspace, "same.go")
	child, root := sym("Child", 6, 2, 1), sym("Root", 12, 1, 1)
	root.Children = []lsp.DocumentSymbol{child}
	client := &fakeDiscoveryClient{documentSupported: true, callSupported: true, symbols: map[string][]lsp.DocumentSymbol{f.URI: {root}}, prepare: map[lsp.Position][]lsp.CallHierarchyItem{
		root.SelectionRange.Start:  {item("Root", f.URI, root.Kind, 1, 1)},
		child.SelectionRange.Start: {item("Child", f.URI, child.Kind, 2, 1)},
	}}
	adapter := DiscoveryAdapter{Workspace: workspace, Files: StaticFiles{f}, Supplier: &fakeSupplier{errors: map[string]error{}}, Client: client}
	acquired := 0
	projection, err := (Core{Discoverer: adapter, Acquirer: acquirerFunc(func(_ context.Context, b BatchRequest) (AcquiredV5, error) {
		acquired += len(b.Targets)
		return AcquiredV5{Session: b.Session, Raw: v5(t, b)}, nil
	})}).Run(context.Background(), SessionIdentity{"s", 7})
	if err != nil {
		t.Fatal(err)
	}
	if acquired != 2 || len(projection.Manifest.Targets) != 2 {
		t.Fatalf("ASSERT_DISCOVER_CORE_SAME_FILE_MULTI_TARGET_ACQUIRED: acquired=%d targets=%d", acquired, len(projection.Manifest.Targets))
	}
}

func TestDiscoveryAdapterTerminalResponseIncompleteBounds(t *testing.T) {
	workspace := t.TempDir()
	f := testSource(workspace, "bounded.go")
	wide := make([]lsp.DocumentSymbol, captureset.MaxTargets+1)
	for i := range wide {
		wide[i] = sym(fmt.Sprintf("S%d", i), 12, uint32(i), 0)
	}
	deep := sym("leaf", 12, 0, 0)
	for i := 0; i < defaultMaxSymbolDepth; i++ {
		deep = sym(fmt.Sprintf("D%d", i), 12, 0, 0, deep)
	}
	long := sym(strings.Repeat("x", defaultMaxStringBytes+1), 12, 0, 0)
	for name, symbols := range map[string][]lsp.DocumentSymbol{"wide": wide, "deep": {deep}, "long": {long}} {
		t.Run(name, func(t *testing.T) {
			client := &fakeDiscoveryClient{documentSupported: true, callSupported: true, symbols: map[string][]lsp.DocumentSymbol{f.URI: symbols}, prepare: map[lsp.Position][]lsp.CallHierarchyItem{}}
			got, err := (DiscoveryAdapter{Workspace: workspace, Files: StaticFiles{f}, Supplier: &fakeSupplier{errors: map[string]error{}}, Client: client}).Discover(context.Background(), SessionIdentity{"s", 1})
			if err != nil || got.Complete || len(client.prepareRequests) != 0 || got.Accounting.SymbolDenominator != 0 || got.Accounting.Files[0].Disposition != census.FileIncomplete {
				t.Fatalf("ASSERT_OVERSIZED_RESPONSE_TERMINAL_INCOMPLETE: %+v err=%v prepares=%d", got.Accounting, err, len(client.prepareRequests))
			}
		})
	}
}

func TestDiscoveryAdapterMaxUintAndPrepareSubstitutionIncomplete(t *testing.T) {
	workspace := t.TempDir()
	f := testSource(workspace, "mismatch.go")
	max := sym("Max", 12, math.MaxUint32, 0)
	selected := sym("Selected", 12, 1, 1)
	client := &fakeDiscoveryClient{documentSupported: true, callSupported: true, symbols: map[string][]lsp.DocumentSymbol{f.URI: {max, selected}}, prepare: map[lsp.Position][]lsp.CallHierarchyItem{
		max.SelectionRange.Start:      {item("Max", f.URI, max.Kind, math.MaxUint32, 0)},
		selected.SelectionRange.Start: {item("Substitute", f.URI, selected.Kind, 1, 1)},
	}}
	got, err := (DiscoveryAdapter{Workspace: workspace, Files: StaticFiles{f}, Supplier: &fakeSupplier{errors: map[string]error{}}, Client: client}).Discover(context.Background(), SessionIdentity{"s", 1})
	if err != nil || got.Complete || len(got.Targets) != 0 || got.Accounting.SymbolDenominator != 2 {
		t.Fatalf("ASSERT_MAX_UINT_AND_PREPARE_SUBSTITUTION_INCOMPLETE: %+v err=%v", got, err)
	}
}
