package censussourceassembly

import (
	"bytes"
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

func terminalDiscovery(targets int) censusacquisition.Discovery {
	d := censusacquisition.Discovery{Closure: censusacquisition.DiscoveryClosureSuccessful}
	if targets > 0 {
		d.Complete = true
		d.Targets = make([]censusacquisition.PreparedTarget, targets)
	}
	return d
}

func TestTerminalRecordSixOutcomesAndPrecedence(t *testing.T) {
	plan := terminalPlan(t, 1, 1)
	cases := []struct {
		name string
		d    censusacquisition.Discovery
		want Outcome
	}{
		{"complete-nonempty", terminalDiscovery(1), OutcomeCompleteNonempty},
		{"successful-empty", terminalDiscovery(0), OutcomeSuccessfulEmpty},
		{"partial", censusacquisition.Discovery{Targets: []censusacquisition.PreparedTarget{{}}}, OutcomePartial},
		{"failed", censusacquisition.Discovery{IncompleteCauses: []censusacquisition.IncompleteCause{{Code: "SESSION_POISONED"}}}, OutcomeFailed},
		{"refused", censusacquisition.Discovery{IncompleteCauses: []censusacquisition.IncompleteCause{{Code: "SESSION_POISONED"}, {Code: "LIFECYCLE_CONFLICT"}}}, OutcomeRefused},
		{"unknown", censusacquisition.Discovery{ProviderFailures: []censusacquisition.ProviderFailure{{Code: "UNAVAILABLE"}}}, OutcomeUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record, err := NewTerminalRecord(plan, 1, 0, tc.d)
			if err != nil || record.Outcome != tc.want {
				t.Fatalf("ASSERT_ASSEMBLY_TERMINAL_SIX_OUTCOMES got=%+v want=%q err=%v", record, tc.want, err)
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
	contradictory := terminalDiscovery(0)
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
	fileOrdinal, symbolOrdinal := 0, 1
	observed, limit, omitted := uint64(2), uint64(3), uint64(4)
	d := censusacquisition.Discovery{
		Accounting:         census.Accounting{Files: make([]census.FileEntry, 0), Symbols: make([]census.SymbolEntry, 0)},
		FileLedger:         captureset.Ledger{Entries: make([]captureset.LedgerEntry, 0)},
		Targets:            []censusacquisition.PreparedTarget{{CanonicalSeedV2: []byte{0, 1, 0, 2}}},
		EnumerationSymbols: make([]censusacquisition.EnumerationSymbol, 0),
		ProviderFailures:   []censusacquisition.ProviderFailure{{Stage: "stage", Code: "code", FileOrdinal: &fileOrdinal}},
		IncompleteCauses:   []censusacquisition.IncompleteCause{{Scope: "SYMBOL", Stage: "DISCOVERY", Code: "CANCELED", SymbolOrdinal: &symbolOrdinal}},
		Observations:       []censusacquisition.DiscoveryObservation{{Known: true, Observed: &observed, Limit: &limit, Omitted: &omitted}},
	}
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
	first.Discovery.Targets[0].CanonicalSeedV2[0] = 9
	*first.Discovery.ProviderFailures[0].FileOrdinal = 9
	*first.Discovery.IncompleteCauses[0].SymbolOrdinal = 9
	*first.Discovery.Observations[0].Observed = 9
	if record.Members[0].Path != "a.go" || second.SourceFiles[0].Path != "a.go" || record.Discovery.Targets[0].CanonicalSeedV2[0] != 0 || second.Discovery.Targets[0].CanonicalSeedV2[0] != 0 || *record.Discovery.ProviderFailures[0].FileOrdinal != 0 || *second.Discovery.ProviderFailures[0].FileOrdinal != 0 || *record.Discovery.IncompleteCauses[0].SymbolOrdinal != 1 || *second.Discovery.IncompleteCauses[0].SymbolOrdinal != 1 || *record.Discovery.Observations[0].Observed != 2 || *second.Discovery.Observations[0].Observed != 2 {
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
	} {
		copy := *record
		mutate(&copy)
		if got, err := AdaptTerminal(plan, 1, &copy); err == nil || !reflect.DeepEqual(got, censussourcecontinuation.Terminal{}) {
			t.Fatalf("ASSERT_ASSEMBLY_TERMINAL_INVALID_ADAPT_REJECTED got=%+v err=%v", got, err)
		}
	}
}
