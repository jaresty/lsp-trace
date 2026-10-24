package censussourceassembly

import (
	"bytes"
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/censussourceplan"
	"lsp-trace/internal/lsp"
)

func terminalPlan(t *testing.T, partitionSize, count int) censussourceplan.Plan {
	t.Helper()
	workspace := t.TempDir()
	files := make([]censusacquisition.SourceFile, count)
	for i := range files {
		name := string(rune('a'+i)) + ".go"
		files[i] = censusacquisition.SourceFile{Path: name, URI: (&url.URL{Scheme: "file", Path: filepath.Join(workspace, name)}).String(), LanguageID: "go"}
	}
	plan, err := censussourceplan.Build(files, workspace, partitionSize)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func successfulEmptyDiscovery() censusacquisition.Discovery {
	return censusacquisition.Discovery{Closure: censusacquisition.DiscoveryClosureSuccessful}
}

type terminalSupplier struct{ failures map[string]error }

func (s terminalSupplier) Supply(_ context.Context, file censusacquisition.SourceFile) error {
	return s.failures[file.Path]
}

type terminalClient struct {
	symbols       map[string][]lsp.DocumentSymbol
	prepare       map[uint32][]lsp.CallHierarchyItem
	prepareErrors map[uint32]error
}

func (terminalClient) SupportsDocumentSymbols() bool { return true }
func (terminalClient) SupportsCallHierarchy() bool   { return true }
func (c terminalClient) DocumentSymbols(_ context.Context, params lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error) {
	return append([]lsp.DocumentSymbol(nil), c.symbols[params.TextDocument.URI]...), nil
}
func (c terminalClient) PrepareCallHierarchy(_ context.Context, params lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	return append([]lsp.CallHierarchyItem(nil), c.prepare[params.Position.Line]...), c.prepareErrors[params.Position.Line]
}

func terminalSymbol(name string, line uint32) lsp.DocumentSymbol {
	position := lsp.Position{Line: line}
	return lsp.DocumentSymbol{Name: name, Kind: 12, Range: lsp.Range{Start: position, End: position}, SelectionRange: lsp.Range{Start: position, End: position}}
}

func terminalItem(name, uri string, line uint32) lsp.CallHierarchyItem {
	position := lsp.Position{Line: line}
	return lsp.CallHierarchyItem{Name: name, Kind: 12, URI: uri, Range: lsp.Range{Start: position, End: position}, SelectionRange: lsp.Range{Start: position, End: position}}
}

func terminalDiscover(t *testing.T, files []censusacquisition.SourceFile, supplier terminalSupplier, client terminalClient) censusacquisition.Discovery {
	t.Helper()
	uri, err := url.Parse(files[0].URI)
	if err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Dir(filepath.FromSlash(uri.Path))
	discovery, err := (censusacquisition.DiscoveryAdapter{Workspace: workspace, Files: censusacquisition.StaticFiles(files), Supplier: supplier, Client: client}).Discover(context.Background(), censusacquisition.SessionIdentity{SessionID: "ready", Generation: 1})
	if err != nil {
		t.Fatal(err)
	}
	return discovery
}

func TestTerminalRecordSixOutcomesAndPrecedence(t *testing.T) {
	plan := terminalPlan(t, 2, 2)
	files := plan.Partitions[0].Members
	prepared := terminalSymbol("Prepared", 0)
	mixedSymbols := []lsp.DocumentSymbol{prepared, terminalSymbol("Missing", 1), terminalSymbol("Failed", 2)}
	baseClient := terminalClient{symbols: map[string][]lsp.DocumentSymbol{}, prepare: map[uint32][]lsp.CallHierarchyItem{}, prepareErrors: map[uint32]error{}}
	cases := []struct {
		name      string
		discovery func() censusacquisition.Discovery
		want      Outcome
	}{
		{"complete-nonempty", func() censusacquisition.Discovery {
			client := baseClient
			client.symbols = map[string][]lsp.DocumentSymbol{files[0].URI: {prepared}}
			client.prepare = map[uint32][]lsp.CallHierarchyItem{0: {terminalItem("Prepared", files[0].URI, 0)}}
			return terminalDiscover(t, files, terminalSupplier{failures: map[string]error{}}, client)
		}, OutcomeCompleteNonempty},
		{"successful-empty", func() censusacquisition.Discovery {
			return terminalDiscover(t, files, terminalSupplier{failures: map[string]error{}}, baseClient)
		}, OutcomeSuccessfulEmpty},
		{"partial", func() censusacquisition.Discovery {
			client := baseClient
			client.symbols = map[string][]lsp.DocumentSymbol{files[0].URI: mixedSymbols}
			client.prepare = map[uint32][]lsp.CallHierarchyItem{0: {terminalItem("Prepared", files[0].URI, 0)}}
			client.prepareErrors = map[uint32]error{2: errors.New("provider cannot prepare")}
			return terminalDiscover(t, files, terminalSupplier{failures: map[string]error{}}, client)
		}, OutcomePartial},
		{"failed", func() censusacquisition.Discovery {
			return terminalDiscover(t, files, terminalSupplier{failures: map[string]error{files[0].Path: censusacquisition.ErrSessionPoisoned}}, baseClient)
		}, OutcomeFailed},
		{"refused", func() censusacquisition.Discovery {
			return terminalDiscover(t, files, terminalSupplier{failures: map[string]error{files[0].Path: censusacquisition.ErrSessionPoisoned, files[1].Path: censusacquisition.ErrLifecycleConflict}}, baseClient)
		}, OutcomeRefused},
		{"unknown", func() censusacquisition.Discovery {
			unreadable := append([]censusacquisition.SourceFile(nil), files...)
			unreadable[0].Disposition = census.FileUnreadable
			return terminalDiscover(t, unreadable, terminalSupplier{failures: map[string]error{}}, baseClient)
		}, OutcomeUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record, err := NewTerminalRecord(plan, 2, 0, tc.discovery())
			if err != nil || record.Outcome != tc.want {
				t.Fatalf("ASSERT_ASSEMBLY_TERMINAL_SIX_OUTCOMES got=%+v want=%q err=%v", record, tc.want, err)
			}
		})
	}
}

func TestTerminalRecordRejectsMalformedAccountingCompletePreparationFailures(t *testing.T) {
	plan := terminalPlan(t, 1, 1)
	symbolOrdinal := 1
	discovery := censusacquisition.Discovery{
		Complete:         true,
		Targets:          []censusacquisition.PreparedTarget{{}},
		ProviderFailures: []censusacquisition.ProviderFailure{{Stage: "PREPARE_CALL_HIERARCHY", Code: "REQUEST_FAILED", SymbolOrdinal: &symbolOrdinal}},
	}

	if record, err := NewTerminalRecord(plan, 1, 0, discovery); err == nil || record != nil {
		t.Fatalf("ASSERT_ASSEMBLY_MALFORMED_COMPLETE_FAILURES_REJECTED: record=%+v err=%v", record, err)
	}
}

func TestTerminalRecordRejectsMalformedAuthenticPreparationFailureCustody(t *testing.T) {
	plan := terminalPlan(t, 1, 1)
	files := plan.Partitions[0].Members
	client := terminalClient{
		symbols:       map[string][]lsp.DocumentSymbol{files[0].URI: {terminalSymbol("Prepared", 0), terminalSymbol("Missing", 1), terminalSymbol("Failed", 2)}},
		prepare:       map[uint32][]lsp.CallHierarchyItem{0: {terminalItem("Prepared", files[0].URI, 0)}},
		prepareErrors: map[uint32]error{2: errors.New("provider cannot prepare")},
	}
	authentic := terminalDiscover(t, files, terminalSupplier{failures: map[string]error{}}, client)
	for name, mutate := range map[string]func(*censusacquisition.Discovery){
		"stage": func(d *censusacquisition.Discovery) { d.ProviderFailures[0].Stage = "DOCUMENT_SYMBOL" },
		"code":  func(d *censusacquisition.Discovery) { d.ProviderFailures[0].Code = "UNSUPPORTED" },
		"file-scope": func(d *censusacquisition.Discovery) {
			d.ProviderFailures[0].FileOrdinal = d.ProviderFailures[0].SymbolOrdinal
			d.ProviderFailures[0].SymbolOrdinal = nil
		},
		"missing-scope": func(d *censusacquisition.Discovery) { d.ProviderFailures[0].SymbolOrdinal = nil },
		"both-scopes": func(d *censusacquisition.Discovery) {
			value := 0
			d.ProviderFailures[0].FileOrdinal = &value
		},
		"out-of-range": func(d *censusacquisition.Discovery) {
			value := d.Accounting.SymbolDenominator
			d.ProviderFailures[0].SymbolOrdinal = &value
		},
		"disposition-mismatch": func(d *censusacquisition.Discovery) {
			ordinal := *d.ProviderFailures[0].SymbolOrdinal
			for i := range d.Accounting.Symbols {
				if d.Accounting.Symbols[i].Ordinal == ordinal {
					d.Accounting.Symbols[i].Disposition = census.SymbolPreparationFailed
				}
			}
			for i := range d.SymbolLedger.Entries {
				if d.SymbolLedger.Entries[i].Ordinal == ordinal {
					d.SymbolLedger.Entries[i].Disposition = string(census.SymbolPreparationFailed)
				}
			}
		},
		"duplicate": func(d *censusacquisition.Discovery) {
			d.ProviderFailures = append(d.ProviderFailures, d.ProviderFailures[0])
		},
		"invalid-closure": func(d *censusacquisition.Discovery) { d.Closure = censusacquisition.DiscoveryClosure(99) },
	} {
		t.Run(name, func(t *testing.T) {
			invalid := CloneDiscovery(authentic)
			mutate(&invalid)
			if record, err := NewTerminalRecord(plan, 1, 0, invalid); err == nil || record != nil {
				t.Fatalf("ASSERT_ASSEMBLY_AUTHENTIC_MALFORMED_FAILURE_REJECTED case=%s record=%+v err=%v", name, record, err)
			}
		})
	}
}

func TestTerminalRecordRejectsInvalidPlanOrdinalAuthorityAccountingAndClosure(t *testing.T) {
	plan := terminalPlan(t, 1, 2)
	invalidPlan := plan
	invalidPlan.Identity = "foreign"
	invalidAuthority := plan
	invalidAuthority.Authority = 1
	badAccounting := censusacquisition.Discovery{Accounting: census.Accounting{FileDenominator: 1}}
	badClosure := censusacquisition.Discovery{Closure: censusacquisition.DiscoveryClosure(99)}
	contradictory := successfulEmptyDiscovery()
	contradictory.ProviderFailures = []censusacquisition.ProviderFailure{{Code: "UNAVAILABLE"}}
	for _, tc := range []struct {
		name    string
		plan    censussourceplan.Plan
		ordinal int
		d       censusacquisition.Discovery
	}{
		{"plan", invalidPlan, 0, censusacquisition.Discovery{}},
		{"ordinal-negative", plan, -1, censusacquisition.Discovery{}},
		{"ordinal-high", plan, 2, censusacquisition.Discovery{}},
		{"authority", invalidAuthority, 0, censusacquisition.Discovery{}},
		{"accounting", plan, 0, badAccounting},
		{"closure", plan, 0, badClosure},
		{"closure-custody", plan, 0, contradictory},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := NewTerminalRecord(tc.plan, 1, tc.ordinal, tc.d); err == nil || got != nil {
				t.Fatalf("ASSERT_ASSEMBLY_TERMINAL_INVALID_REJECTED got=%+v err=%v", got, err)
			}
		})
	}
}

func TestTerminalRecordExactAdaptationAndIndependentClones(t *testing.T) {
	plan := terminalPlan(t, 1, 1)
	files := plan.Partitions[0].Members
	client := terminalClient{
		symbols:       map[string][]lsp.DocumentSymbol{files[0].URI: {terminalSymbol("Prepared", 0), terminalSymbol("Missing", 1), terminalSymbol("Failed", 2)}},
		prepare:       map[uint32][]lsp.CallHierarchyItem{0: {terminalItem("Prepared", files[0].URI, 0)}},
		prepareErrors: map[uint32]error{2: errors.New("provider cannot prepare")},
	}
	d := terminalDiscover(t, files, terminalSupplier{failures: map[string]error{}}, client)
	record, err := NewTerminalRecord(plan, 1, 0, d)
	if err != nil {
		t.Fatal(err)
	}
	first, err := AdaptTerminal(plan, 1, record)
	if err != nil {
		t.Fatal(err)
	}
	second, err := AdaptTerminal(plan, 1, record)
	if err != nil {
		t.Fatal(err)
	}
	want := censussourcecontinuation.Terminal{PlanIdentity: plan.Identity, Authority: 0, Ordinal: 0, SourceFiles: plan.Partitions[0].Members, Discovery: d, Outcome: censussourcecontinuation.OutcomePartial}
	if !reflect.DeepEqual(first, want) || !reflect.DeepEqual(second, want) {
		t.Fatalf("ASSERT_ASSEMBLY_TERMINAL_EXACT_ADAPTATION first=%+v second=%+v want=%+v", first, second, want)
	}
	first.SourceFiles[0].Path = "mutated.go"
	first.Discovery.Targets[0].CanonicalSeedV2[0] ^= 0xff
	*first.Discovery.ProviderFailures[0].SymbolOrdinal = 9
	if record.Members[0].Path != "a.go" || second.SourceFiles[0].Path != "a.go" || reflect.DeepEqual(first.Discovery.Targets[0].CanonicalSeedV2, record.Discovery.Targets[0].CanonicalSeedV2) || !reflect.DeepEqual(second.Discovery.Targets[0].CanonicalSeedV2, record.Discovery.Targets[0].CanonicalSeedV2) || *record.Discovery.ProviderFailures[0].SymbolOrdinal == 9 || *second.Discovery.ProviderFailures[0].SymbolOrdinal == 9 {
		t.Fatal("ASSERT_ASSEMBLY_TERMINAL_NESTED_ANTI_ALIAS")
	}
}

func TestTerminalRecordNilEmptyMatrix(t *testing.T) {
	plan := terminalPlan(t, 1, 1)
	type sliceCase struct {
		name  string
		set   func(*censusacquisition.Discovery)
		isNil func(censusacquisition.Discovery) bool
	}
	cases := []sliceCase{
		{"Accounting.Files", func(d *censusacquisition.Discovery) { d.Accounting.Files = make([]census.FileEntry, 0) }, func(d censusacquisition.Discovery) bool { return d.Accounting.Files == nil }},
		{"Accounting.Symbols", func(d *censusacquisition.Discovery) { d.Accounting.Symbols = make([]census.SymbolEntry, 0) }, func(d censusacquisition.Discovery) bool { return d.Accounting.Symbols == nil }},
		{"FileLedger.Entries", func(d *censusacquisition.Discovery) { d.FileLedger.Entries = make([]captureset.LedgerEntry, 0) }, func(d censusacquisition.Discovery) bool { return d.FileLedger.Entries == nil }},
		{"SymbolLedger.Entries", func(d *censusacquisition.Discovery) { d.SymbolLedger.Entries = make([]captureset.LedgerEntry, 0) }, func(d censusacquisition.Discovery) bool { return d.SymbolLedger.Entries == nil }},
		{"Targets", func(d *censusacquisition.Discovery) { d.Targets = make([]censusacquisition.PreparedTarget, 0) }, func(d censusacquisition.Discovery) bool { return d.Targets == nil }},
		{"EnumerationSymbols", func(d *censusacquisition.Discovery) {
			d.EnumerationSymbols = make([]censusacquisition.EnumerationSymbol, 0)
		}, func(d censusacquisition.Discovery) bool { return d.EnumerationSymbols == nil }},
		{"ProviderFailures", func(d *censusacquisition.Discovery) {
			d.ProviderFailures = make([]censusacquisition.ProviderFailure, 0)
		}, func(d censusacquisition.Discovery) bool { return d.ProviderFailures == nil }},
		{"IncompleteCauses", func(d *censusacquisition.Discovery) {
			d.IncompleteCauses = make([]censusacquisition.IncompleteCause, 0)
		}, func(d censusacquisition.Discovery) bool { return d.IncompleteCauses == nil }},
		{"Observations", func(d *censusacquisition.Discovery) {
			d.Observations = make([]censusacquisition.DiscoveryObservation, 0)
		}, func(d censusacquisition.Discovery) bool { return d.Observations == nil }},
	}
	for _, tc := range cases {
		for _, empty := range []bool{false, true} {
			t.Run(tc.name+map[bool]string{false: "/nil", true: "/empty"}[empty], func(t *testing.T) {
				var d censusacquisition.Discovery
				if empty {
					tc.set(&d)
				}
				record, err := NewTerminalRecord(plan, 1, 0, d)
				if err != nil {
					t.Fatal(err)
				}
				adapted, err := AdaptTerminal(plan, 1, record)
				if err != nil {
					t.Fatal(err)
				}
				if tc.isNil(record.Discovery) != !empty || tc.isNil(adapted.Discovery) != !empty {
					t.Fatalf("ASSERT_ASSEMBLY_TERMINAL_NIL_EMPTY field=%s empty=%t", tc.name, empty)
				}
			})
		}
	}
}

func TestTerminalProductionBoundaryRemainsNeutral(t *testing.T) {
	raw, err := os.ReadFile("terminal.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"cmd/lsp-trace-mcp", "sessionruntime", "censussourcedriver", "internal/operation", "internal/schema", "func Run("} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("ASSERT_ASSEMBLY_TERMINAL_FORBIDDEN_DEPENDENCY %q", forbidden)
		}
	}
}

func TestAdaptTerminalRejectsInvalidRecordCustodyAndOutcome(t *testing.T) {
	plan := terminalPlan(t, 1, 1)
	record, err := NewTerminalRecord(plan, 1, 0, censusacquisition.Discovery{})
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*TerminalRecord){
		func(r *TerminalRecord) { r.PlanIdentity = "foreign" },
		func(r *TerminalRecord) { r.Authority = 1 },
		func(r *TerminalRecord) { r.Ordinal = -1 },
		func(r *TerminalRecord) { r.Members = nil },
		func(r *TerminalRecord) { r.Outcome = Outcome("INVALID") },
		func(r *TerminalRecord) { r.Discovery.Closure = censusacquisition.DiscoveryClosure(99) },
		func(r *TerminalRecord) {
			symbolOrdinal := 0
			r.Discovery = censusacquisition.Discovery{
				Complete:         true,
				Targets:          []censusacquisition.PreparedTarget{{}},
				ProviderFailures: []censusacquisition.ProviderFailure{{Stage: "DOCUMENT_SYMBOL", Code: "UNSUPPORTED", SymbolOrdinal: &symbolOrdinal}},
			}
			r.Outcome = OutcomePartial
		},
	} {
		copy := *record
		mutate(&copy)
		if got, err := AdaptTerminal(plan, 1, &copy); err == nil || !reflect.DeepEqual(got, censussourcecontinuation.Terminal{}) {
			t.Fatalf("ASSERT_ASSEMBLY_TERMINAL_INVALID_ADAPT_REJECTED got=%+v err=%v", got, err)
		}
	}
}
