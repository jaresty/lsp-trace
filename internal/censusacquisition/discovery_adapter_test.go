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
	"time"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/census"
	"lsp-trace/internal/censusenumerationprojection"
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
	prepareHook                      func()
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
	if f.prepareHook != nil {
		f.prepareHook()
	}
	return append([]lsp.CallHierarchyItem(nil), f.prepare[p.Position]...), f.prepareErrors[p.Position]
}

type fakeSupplier struct {
	calls  []string
	errors map[string]error
	hook   func()
}

func (f *fakeSupplier) Supply(_ context.Context, file SourceFile) error {
	f.calls = append(f.calls, file.Path)
	if f.hook != nil {
		f.hook()
	}
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

func TestDiscoveryAcceptsPreparedIdentifierRangeWithinDeclaration(t *testing.T) {
	workspace := t.TempDir()
	file := testSource(workspace, "a.go")
	selection := lsp.Range{Start: lsp.Position{Line: 3, Character: 5}, End: lsp.Position{Line: 3, Character: 8}}
	declaration := lsp.Range{Start: lsp.Position{Line: 3}, End: lsp.Position{Line: 7, Character: 1}}
	symbol := lsp.DocumentSymbol{Name: "Run", Kind: 12, Range: declaration, SelectionRange: selection}
	prepared := lsp.CallHierarchyItem{Name: "Run", Kind: 12, URI: file.URI, Range: selection, SelectionRange: selection}
	client := &fakeDiscoveryClient{documentSupported: true, callSupported: true, symbols: map[string][]lsp.DocumentSymbol{file.URI: {symbol}}, prepare: map[lsp.Position][]lsp.CallHierarchyItem{selection.Start: {prepared}}}
	got, err := (DiscoveryAdapter{Workspace: workspace, Files: StaticFiles{file}, Supplier: &fakeSupplier{errors: map[string]error{}}, Client: client}).Discover(context.Background(), SessionIdentity{"ready", 2})
	if err != nil || !got.Complete || len(got.Targets) != 1 || got.Accounting.Symbols[0].Disposition != census.SymbolSelected {
		t.Fatalf("ASSERT_PREPARED_IDENTIFIER_RANGE_WITHIN_DECLARATION: got=%+v err=%v", got, err)
	}
}

func TestDiscoveryAcceptsCallableMethodFunctionKindVariation(t *testing.T) {
	workspace := t.TempDir()
	file := testSource(workspace, "a.go")
	symbol := sym("Run", 6, 3, 5)
	prepared := item("Run", file.URI, 12, 3, 5)
	client := &fakeDiscoveryClient{documentSupported: true, callSupported: true, symbols: map[string][]lsp.DocumentSymbol{file.URI: {symbol}}, prepare: map[lsp.Position][]lsp.CallHierarchyItem{symbol.SelectionRange.Start: {prepared}}}
	got, err := (DiscoveryAdapter{Workspace: workspace, Files: StaticFiles{file}, Supplier: &fakeSupplier{errors: map[string]error{}}, Client: client}).Discover(context.Background(), SessionIdentity{"ready", 2})
	if err != nil || !got.Complete || len(got.Targets) != 1 {
		t.Fatalf("ASSERT_CALLABLE_KIND_VARIATION_CORRESPONDS: got=%+v err=%v", got, err)
	}
}

func TestDiscoveryNonCallableIsAccountedExclusionNotIncompleteness(t *testing.T) {
	workspace := t.TempDir()
	file := testSource(workspace, "a.go")
	callable := sym("Run", 12, 3, 5)
	nonCallable := sym("Runtime", 5, 1, 5)
	client := &fakeDiscoveryClient{documentSupported: true, callSupported: true, symbols: map[string][]lsp.DocumentSymbol{file.URI: {nonCallable, callable}}, prepare: map[lsp.Position][]lsp.CallHierarchyItem{callable.SelectionRange.Start: {item("Run", file.URI, 12, 3, 5)}}}
	got, err := (DiscoveryAdapter{Workspace: workspace, Files: StaticFiles{file}, Supplier: &fakeSupplier{errors: map[string]error{}}, Client: client}).Discover(context.Background(), SessionIdentity{"ready", 2})
	if err != nil || !got.Complete || len(got.Targets) != 1 || got.Accounting.Files[0].Disposition != census.FileSelected {
		t.Fatalf("ASSERT_NON_CALLABLE_ACCOUNTED_EXCLUSION_COMPLETE: got=%+v err=%v", got, err)
	}
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

func TestPreparedItemNameMatchesExactAndGoplsMethodNormalization(t *testing.T) {
	for _, tc := range []struct {
		item, symbol string
		want         bool
	}{
		{"Function", "Function", true},
		{"Discover", "(fixedCensusRuntimeDiscovery).Discover", true},
		{"Other", "(fixedCensusRuntimeDiscovery).Discover", false},
		{"Discover", "().Discover", false},
		{"Discover", "fixedCensusRuntimeDiscovery.Discover", false},
		{"Discover", "(receiver).Discover.Extra", false},
		{"Discover", "(receiver).Other).Discover", false},
	} {
		if got := preparedItemNameMatches(tc.item, tc.symbol); got != tc.want {
			t.Fatalf("ASSERT_PREPARED_ITEM_NAME_CORRESPONDENCE[%q,%q]: got=%v want=%v", tc.item, tc.symbol, got, tc.want)
		}
	}
}

func TestDiscoveryAdapterAccountedPreparationOmissionsRemainComplete(t *testing.T) {
	workspace := t.TempDir()
	f := testSource(workspace, "mixed.go")
	prepared := sym("Prepared", 12, 0, 0)
	missing := sym("Missing", 12, 1, 0)
	failed := sym("Failed", 6, 2, 0)
	client := &fakeDiscoveryClient{
		documentSupported: true,
		callSupported:     true,
		symbols:           map[string][]lsp.DocumentSymbol{f.URI: {prepared, missing, failed}},
		prepare:           map[lsp.Position][]lsp.CallHierarchyItem{prepared.SelectionRange.Start: {item("Prepared", f.URI, 12, 0, 0)}},
		prepareErrors:     map[lsp.Position]error{failed.SelectionRange.Start: errors.New("provider cannot prepare")},
	}
	got, err := (DiscoveryAdapter{Workspace: workspace, Files: StaticFiles{f}, Supplier: &fakeSupplier{errors: map[string]error{}}, Client: client}).Discover(context.Background(), SessionIdentity{"s", 1})
	if err != nil || !got.Complete {
		t.Fatalf("ASSERT_ACCOUNTED_PREPARATION_OMISSIONS_COMPLETE: complete=%v err=%v", got.Complete, err)
	}
	if len(got.Targets) != 1 || got.Targets[0].Name != "Prepared" || got.Accounting.SymbolDenominator != 3 || got.SymbolLedger.Denominator != 3 {
		t.Fatalf("ASSERT_PREPARATION_OMISSIONS_NO_TARGET_FABRICATION: targets=%+v accounting=%+v ledger=%+v", got.Targets, got.Accounting, got.SymbolLedger)
	}
	want := []census.SymbolDisposition{census.SymbolSelected, census.SymbolPrepareMissing, census.SymbolPreparationFailed}
	var have []census.SymbolDisposition
	for _, entry := range got.Accounting.Symbols {
		have = append(have, entry.Disposition)
	}
	if !reflect.DeepEqual(have, want) || got.Accounting.Files[0].Disposition != census.FileSelected {
		t.Fatalf("ASSERT_PREPARATION_OMISSIONS_CLOSED_ACCOUNTING: symbols=%v files=%+v", have, got.Accounting.Files)
	}
	if err := got.Accounting.Validate(); err != nil {
		t.Fatalf("ASSERT_PREPARATION_OMISSIONS_ACCOUNTING_VALID: %v", err)
	}
}

func TestDiscoveryClassifiesSessionUnavailableSupplyAsIncomplete(t *testing.T) {
	workspace := t.TempDir()
	first, second := testSource(workspace, "a.go"), testSource(workspace, "b.go")
	client := &fakeDiscoveryClient{
		documentSupported: true,
		callSupported:     true,
		symbols:           map[string][]lsp.DocumentSymbol{first.URI: {}, second.URI: {}},
		prepare:           map[lsp.Position][]lsp.CallHierarchyItem{},
	}
	supplier := &fakeSupplier{errors: map[string]error{"b.go": ErrSessionUnavailable}}
	got, err := (DiscoveryAdapter{Workspace: workspace, Files: StaticFiles{first, second}, Supplier: supplier, Client: client}).Discover(context.Background(), SessionIdentity{"s", 1})
	if err != nil {
		t.Fatal(err)
	}
	want := []census.FileDisposition{census.FileSelected, census.FileIncomplete}
	if len(got.Accounting.Files) != len(want) {
		t.Fatalf("ASSERT_SESSION_UNAVAILABLE_NOT_SOURCE_UNREADABLE: files=%+v", got.Accounting.Files)
	}
	for i, entry := range got.Accounting.Files {
		if entry.Disposition != want[i] {
			t.Fatalf("ASSERT_SESSION_UNAVAILABLE_NOT_SOURCE_UNREADABLE: files=%+v", got.Accounting.Files)
		}
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
	for _, want := range []census.FileDisposition{census.FileDocumentSymbolFailed, census.FileUnreadable, census.FileUnsupported} {
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
	if !reflect.DeepEqual(incompleteCauseCodes(got), []string{"SYMBOL_LIMIT", "CHILD_INCOMPLETE"}) {
		t.Fatalf("ASSERT_SYMBOL_LIMIT_CAUSES: %+v", got.IncompleteCauses)
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
	if !reflect.DeepEqual(incompleteCauseCodes(got), []string{"RESULT_MISMATCH", "RESULT_MISMATCH", "CHILD_INCOMPLETE"}) {
		t.Fatalf("ASSERT_RESULT_MISMATCH_CAUSES: %+v", got.IncompleteCauses)
	}
}

func TestDiscoveryAdapterExclusionPrecedesCancellationAccounting(t *testing.T) {
	workspace := t.TempDir()
	excluded := testSource(workspace, "excluded.go")
	selected := testSource(workspace, "selected.go")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := (DiscoveryAdapter{
		Workspace: workspace,
		Filters:   Filters{Excludes: []string{"excluded.go"}},
		Files:     StaticFiles{selected, excluded},
		Supplier:  &fakeSupplier{errors: map[string]error{}},
		Client:    &fakeDiscoveryClient{documentSupported: true},
	}).Discover(ctx, SessionIdentity{"s", 1})
	if err != nil {
		t.Fatal(err)
	}
	if got.Accounting.Files[0].Disposition != census.FileExcluded || got.Accounting.Files[1].Disposition != census.FileIncomplete {
		t.Fatalf("ASSERT_EXCLUDED_FILE_STAYS_EXCLUDED_AFTER_CANCELLATION: %+v", got.Accounting.Files)
	}
	if len(got.IncompleteCauses) != 1 || got.IncompleteCauses[0].FileOrdinal == nil || *got.IncompleteCauses[0].FileOrdinal != 1 || got.IncompleteCauses[0].Code != "CANCELED" {
		t.Fatalf("ASSERT_SELECTED_FILE_CANCELLATION_REMAINS_INCOMPLETE: %+v", got.IncompleteCauses)
	}
}

func TestDiscoveryAdapterDeadlineAndGenericCancellationTimeoutProvenance(t *testing.T) {
	for _, tc := range []struct {
		name      string
		context   func() (context.Context, context.CancelFunc)
		wantKnown bool
	}{
		{name: "deadline", context: func() (context.Context, context.CancelFunc) {
			return context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		}, wantKnown: true},
		{name: "generic-cancellation", context: func() (context.Context, context.CancelFunc) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx, cancel
		}, wantKnown: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workspace := t.TempDir()
			file := testSource(workspace, "selected.go")
			ctx, cancel := tc.context()
			defer cancel()

			got, err := (DiscoveryAdapter{Workspace: workspace, Files: StaticFiles{file}, Supplier: &fakeSupplier{errors: map[string]error{}}, Client: &fakeDiscoveryClient{documentSupported: true}}).Discover(ctx, SessionIdentity{"s", 1})
			if err != nil {
				t.Fatal(err)
			}
			for _, observation := range got.Observations {
				if observation.Dimension == DiscoveryTimeoutRequestBudget && observation.Known != tc.wantKnown {
					t.Fatalf("ASSERT_TIMEOUT_REQUEST_BUDGET_DEADLINE_ONLY: want_known=%v observation=%+v", tc.wantKnown, observation)
				}
			}
		})
	}
}

func TestDiscoveryAdapterFileCancellationPrecedence(t *testing.T) {
	workspace := t.TempDir()
	f := testSource(workspace, "cancel.go")
	ctx, cancel := context.WithCancel(context.Background())
	supplier := &fakeSupplier{errors: map[string]error{f.Path: errors.New("provider")}, hook: cancel}
	got, err := (DiscoveryAdapter{Workspace: workspace, Files: StaticFiles{f}, Supplier: supplier, Client: &fakeDiscoveryClient{documentSupported: true}}).Discover(ctx, SessionIdentity{"s", 1})
	if err != nil || !reflect.DeepEqual(incompleteCauseCodes(got), []string{"CANCELED"}) || got.Accounting.Files[0].Disposition != census.FileIncomplete {
		t.Fatalf("discovery=%+v err=%v", got, err)
	}
}

func TestDiscoveryAdapterSuccessfulSupplyCancellationPrecedesUnsupportedCapability(t *testing.T) {
	workspace := t.TempDir()
	f := testSource(workspace, "cancel.go")
	ctx, cancel := context.WithCancel(context.Background())
	supplier := &fakeSupplier{errors: map[string]error{}, hook: cancel}
	client := &fakeDiscoveryClient{documentSupported: false}

	got, err := (DiscoveryAdapter{Workspace: workspace, Files: StaticFiles{f}, Supplier: supplier, Client: client}).Discover(ctx, SessionIdentity{"s", 1})
	if err != nil {
		t.Fatal(err)
	}
	if got.Complete || len(got.Accounting.Files) != 1 || got.Accounting.Files[0].Disposition != census.FileIncomplete {
		t.Fatalf("ASSERT_SUCCESSFUL_SUPPLY_CANCELLATION_INCOMPLETE: complete=%v files=%+v", got.Complete, got.Accounting.Files)
	}
	if len(got.IncompleteCauses) != 1 {
		t.Fatalf("ASSERT_SUCCESSFUL_SUPPLY_CANCELLATION_SINGLE_CAUSE: %+v", got.IncompleteCauses)
	}
	cause := got.IncompleteCauses[0]
	if cause.Scope != "FILE" || cause.Stage != "ENUMERATION" || cause.Code != "CANCELED" || cause.FileOrdinal == nil || *cause.FileOrdinal != 0 {
		t.Fatalf("ASSERT_SUCCESSFUL_SUPPLY_CANCELLATION_CAUSE: %+v", cause)
	}
	if len(client.documentRequests) != 0 || len(got.ProviderFailures) != 0 {
		t.Fatalf("ASSERT_SUCCESSFUL_SUPPLY_CANCELLATION_PRECEDENCE: requests=%v failures=%+v", client.documentRequests, got.ProviderFailures)
	}
	if err := validateDiscovery(got); err != nil {
		t.Fatalf("ASSERT_SUCCESSFUL_SUPPLY_CANCELLATION_DISCOVERY_VALID: %v", err)
	}
	if validDiscoveryCause(IncompleteCause{Scope: "FILE", Stage: "ENUMERATION", Code: "FILE_LIMIT"}) {
		t.Fatal("ASSERT_ENUMERATION_FILE_LIMIT_CAUSE_INCOMPATIBLE")
	}
	if got.Accounting.FileDenominator != 1 || got.FileLedger.Denominator != 1 || len(got.FileLedger.Entries) != 1 || got.FileLedger.Entries[0].Ordinal != 0 || got.FileLedger.Entries[0].Identity != f.Path || got.FileLedger.Entries[0].Disposition != string(census.FileIncomplete) {
		t.Fatalf("ASSERT_SUCCESSFUL_SUPPLY_CANCELLATION_FILE_LEDGERS_RECONCILED: accounting=%+v ledger=%+v", got.Accounting, got.FileLedger)
	}
	if got.Accounting.SymbolDenominator != 0 || got.SymbolLedger.Denominator != 0 || len(got.Accounting.Symbols) != 0 || len(got.SymbolLedger.Entries) != 0 {
		t.Fatalf("ASSERT_SUCCESSFUL_SUPPLY_CANCELLATION_SYMBOL_LEDGERS_RECONCILED: accounting=%+v ledger=%+v", got.Accounting, got.SymbolLedger)
	}
}

func TestDiscoveryAdapterIncompleteCauseCancellationPrecedence(t *testing.T) {
	workspace := t.TempDir()
	f := testSource(workspace, "cancel.go")
	s := sym("Mismatch", 12, 1, 1)
	ctx, cancel := context.WithCancel(context.Background())
	client := &fakeDiscoveryClient{documentSupported: true, callSupported: true, symbols: map[string][]lsp.DocumentSymbol{f.URI: {s}}, prepare: map[lsp.Position][]lsp.CallHierarchyItem{s.SelectionRange.Start: {item("wrong", f.URI, 12, 1, 1)}}, prepareHook: cancel}
	got, err := (DiscoveryAdapter{Workspace: workspace, Files: StaticFiles{f}, Supplier: &fakeSupplier{errors: map[string]error{}}, Client: client}).Discover(ctx, SessionIdentity{"s", 1})
	if err != nil || len(got.IncompleteCauses) != 2 {
		t.Fatalf("causes=%+v err=%v", got.IncompleteCauses, err)
	}
	for _, cause := range got.IncompleteCauses {
		if cause.Code != "CANCELED" {
			t.Fatalf("cancellation did not win: %+v", got.IncompleteCauses)
		}
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

func TestDiscoveryAdapterNonCallablesDoNotConsumeTargetCapacity(t *testing.T) {
	workspace := t.TempDir()
	file := testSource(workspace, "mixed.go")
	symbols := make([]lsp.DocumentSymbol, captureset.MaxTargets+1)
	for i := 0; i < captureset.MaxTargets; i++ {
		symbols[i] = sym(fmt.Sprintf("Field%d", i), 5, uint32(i), 0)
	}
	callable := sym("Run", 12, uint32(captureset.MaxTargets), 0)
	symbols[captureset.MaxTargets] = callable
	client := &fakeDiscoveryClient{
		documentSupported: true,
		callSupported:     true,
		symbols:           map[string][]lsp.DocumentSymbol{file.URI: symbols},
		prepare:           map[lsp.Position][]lsp.CallHierarchyItem{callable.SelectionRange.Start: {item("Run", file.URI, 12, uint32(captureset.MaxTargets), 0)}},
	}
	got, err := (DiscoveryAdapter{Workspace: workspace, Files: StaticFiles{file}, Supplier: &fakeSupplier{errors: map[string]error{}}, Client: client}).Discover(context.Background(), SessionIdentity{"s", 1})
	if err != nil || !got.Complete || len(got.Targets) != 1 || got.Targets[0].Name != "Run" {
		t.Fatalf("ASSERT_NON_CALLABLES_DO_NOT_CONSUM_TARGET_CAPACITY: complete=%v targets=%d err=%v causes=%+v", got.Complete, len(got.Targets), err, got.IncompleteCauses)
	}
}

func TestDiscoveryAdapterTargetAdmissionCapIsNotTraversalNodeCeiling(t *testing.T) {
	workspace := t.TempDir()
	files := StaticFiles{testSource(workspace, "a.go"), testSource(workspace, "b.go")}
	symbols := make(map[string][]lsp.DocumentSymbol, len(files))
	for fileIndex, file := range files {
		count := captureset.MaxTargets / 2
		if fileIndex == len(files)-1 {
			count = captureset.MaxTargets + 3 - count
		}
		items := make([]lsp.DocumentSymbol, count)
		for i := range items {
			line := fileIndex*captureset.MaxTargets + i
			items[i] = sym(fmt.Sprintf("S%d", line), 12, uint32(line), 0)
		}
		symbols[file.URI] = items
	}
	unseedable := &symbols[files[1].URI][captureset.MaxTargets/2]
	unseedablePosition := lsp.Position{Line: unseedable.SelectionRange.Start.Line, Character: math.MaxUint32}
	unseedable.Range = lsp.Range{Start: unseedablePosition, End: unseedablePosition}
	unseedable.SelectionRange = unseedable.Range
	prepare := make(map[lsp.Position][]lsp.CallHierarchyItem, captureset.MaxTargets+3)
	for _, file := range files {
		for _, symbol := range symbols[file.URI] {
			prepare[symbol.SelectionRange.Start] = []lsp.CallHierarchyItem{item(symbol.Name, file.URI, symbol.Kind, symbol.SelectionRange.Start.Line, symbol.SelectionRange.Start.Character)}
		}
	}
	client := &fakeDiscoveryClient{documentSupported: true, callSupported: true, symbols: symbols, prepare: prepare}
	got, err := (DiscoveryAdapter{Workspace: workspace, Files: files, Supplier: &fakeSupplier{errors: map[string]error{}}, Client: client}).Discover(context.Background(), SessionIdentity{"s", 1})
	if err != nil {
		t.Fatal(err)
	}
	wantSymbols := captureset.MaxTargets + 3
	if got.Complete || got.Accounting.FileDenominator != len(files) || got.Accounting.SymbolDenominator != wantSymbols {
		t.Fatalf("ASSERT_TARGET_CAP_ACCOUNTING: complete=%v files=%d symbols=%d", got.Complete, got.Accounting.FileDenominator, got.Accounting.SymbolDenominator)
	}
	if len(got.Accounting.Files) != len(files) || len(got.Accounting.Symbols) != wantSymbols || len(got.SymbolLedger.Entries) != wantSymbols {
		t.Fatalf("ASSERT_TARGET_CAP_OBSERVED_FILES_SYMBOLS: files=%d symbols=%d ledger=%d", len(got.Accounting.Files), len(got.Accounting.Symbols), len(got.SymbolLedger.Entries))
	}
	if len(got.Targets) != captureset.MaxTargets {
		t.Fatalf("ASSERT_TARGET_CAP_COUNTS_PREPARED_TARGETS: targets=%d", len(got.Targets))
	}
	wantCodes := []string{"COORDINATE_OVERFLOW", "TARGET_LIMIT", "TARGET_LIMIT", "CHILD_INCOMPLETE"}
	if !reflect.DeepEqual(incompleteCauseCodes(got), wantCodes) {
		t.Fatalf("ASSERT_ADMISSION_ORDER_EXACT_CAUSES: got=%+v want=%v", got.IncompleteCauses, wantCodes)
	}
	for i, wantOrdinal := range []int{captureset.MaxTargets, captureset.MaxTargets + 1, captureset.MaxTargets + 2} {
		cause := got.IncompleteCauses[i]
		if cause.Scope != "SYMBOL" || cause.SymbolOrdinal == nil || *cause.SymbolOrdinal != wantOrdinal || got.Accounting.Symbols[wantOrdinal].Disposition != census.SymbolIncomplete || got.SymbolLedger.Entries[wantOrdinal].Disposition != string(census.SymbolIncomplete) {
			t.Fatalf("ASSERT_INCOMPLETE_CAUSE_LEDGER_BY_ORDINAL[%d]: cause=%+v accounting=%+v ledger=%+v", wantOrdinal, cause, got.Accounting.Symbols[wantOrdinal], got.SymbolLedger.Entries[wantOrdinal])
		}
	}
	observations := make(map[DiscoveryDimension]DiscoveryObservation, len(got.Observations))
	for _, observation := range got.Observations {
		observations[observation.Dimension] = observation
	}
	if observations[DiscoveryNodeCeiling].Known {
		t.Fatalf("ASSERT_TARGET_CAP_NOT_NODE_CEILING: %+v", observations[DiscoveryNodeCeiling])
	}
	omissions := observations[DiscoveryTargetSourceOmissions]
	if !omissions.Known || omissions.Omitted == nil || *omissions.Omitted != 3 {
		t.Fatalf("ASSERT_TARGET_CAP_OMITTED_DENOMINATOR: %+v", omissions)
	}
}

func TestDiscoveryAdapterTerminalResponseIncompleteBounds(t *testing.T) {
	workspace := t.TempDir()
	f := testSource(workspace, "bounded.go")
	wide := make([]lsp.DocumentSymbol, censusenumerationprojection.MaxSymbols+1)
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
			wantCode := map[string]string{"wide": "NODE_LIMIT", "deep": "DEPTH_LIMIT", "long": "STRING_BYTE_LIMIT"}[name]
			if !reflect.DeepEqual(incompleteCauseCodes(got), []string{wantCode}) {
				t.Fatalf("ASSERT_FLATTEN_CAUSE[%s]: %+v", name, got.IncompleteCauses)
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
	if !reflect.DeepEqual(incompleteCauseCodes(got), []string{"RESULT_MISMATCH", "COORDINATE_OVERFLOW", "CHILD_INCOMPLETE"}) {
		t.Fatalf("ASSERT_EXACT_SYMBOL_CAUSES: %+v", got.IncompleteCauses)
	}
}

func TestDiscoveryAdapterFileLimitCause(t *testing.T) {
	workspace := t.TempDir()
	a, b := testSource(workspace, "a.go"), testSource(workspace, "b.go")
	noncall := sym("Field", 5, 0, 0)
	client := &fakeDiscoveryClient{documentSupported: true, callSupported: true, symbols: map[string][]lsp.DocumentSymbol{a.URI: {noncall}}, prepare: map[lsp.Position][]lsp.CallHierarchyItem{}}
	got, err := (DiscoveryAdapter{Workspace: workspace, Files: StaticFiles{a, b}, Supplier: &fakeSupplier{errors: map[string]error{}}, Client: client, Limits: DiscoveryLimits{MaxFiles: 1}}).Discover(context.Background(), SessionIdentity{"s", 1})
	if err != nil || !reflect.DeepEqual(incompleteCauseCodes(got), []string{"FILE_LIMIT"}) {
		t.Fatalf("causes=%+v err=%v", got.IncompleteCauses, err)
	}
}

func incompleteCauseCodes(d Discovery) []string {
	out := make([]string, len(d.IncompleteCauses))
	for i, cause := range d.IncompleteCauses {
		out[i] = cause.Code
	}
	return out
}
