package main

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"reflect"
	"testing"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censussourceplan"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/operation"
	"lsp-trace/sessionruntime"
)

func terminalPlanFixture(t *testing.T, count int) censussourceplan.Plan {
	t.Helper()
	workspace := t.TempDir()
	files := make([]censusacquisition.SourceFile, count)
	for i := range files {
		name := string(rune('a'+i)) + ".go"
		files[i] = censusacquisition.SourceFile{Path: name, URI: (&url.URL{Scheme: "file", Path: filepath.Join(workspace, name)}).String(), LanguageID: "go"}
	}
	plan, err := censussourceplan.Build(files, workspace, censusRuntimeSourcePartitionSize)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

type privateTerminalSupplier struct{ failures map[string]error }

func (s privateTerminalSupplier) Supply(_ context.Context, file censusacquisition.SourceFile) error {
	return s.failures[file.Path]
}

type privateTerminalClient struct {
	uri   string
	mixed bool
}

func (privateTerminalClient) SupportsDocumentSymbols() bool { return true }
func (privateTerminalClient) SupportsCallHierarchy() bool   { return true }
func (c privateTerminalClient) DocumentSymbols(context.Context, lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error) {
	count := 1
	if c.mixed {
		count = 3
	}
	symbols := make([]lsp.DocumentSymbol, count)
	for line := range symbols {
		position := lsp.Position{Line: uint32(line)}
		symbols[line] = lsp.DocumentSymbol{Name: []string{"Prepared", "Missing", "Failed"}[line], Kind: 12, Range: lsp.Range{Start: position, End: position}, SelectionRange: lsp.Range{Start: position, End: position}}
	}
	return symbols, nil
}
func (c privateTerminalClient) PrepareCallHierarchy(_ context.Context, params lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	position := params.Position
	switch {
	case !c.mixed || position.Line == 0:
		return []lsp.CallHierarchyItem{{Name: "Prepared", Kind: 12, URI: c.uri, Range: lsp.Range{Start: position, End: position}, SelectionRange: lsp.Range{Start: position, End: position}}}, nil
	case position.Line == 1:
		return nil, nil
	default:
		return nil, errors.New("provider cannot prepare")
	}
}

func validPrivateTerminalDiscovery(t *testing.T, plan censussourceplan.Plan, mixed bool, supplyErr error) censusacquisition.Discovery {
	t.Helper()
	files := append([]censusacquisition.SourceFile(nil), plan.Partitions[0].Members...)
	uri, err := url.Parse(files[0].URI)
	if err != nil {
		t.Fatal(err)
	}
	failures := map[string]error{}
	if supplyErr != nil {
		for _, file := range files {
			failures[file.Path] = supplyErr
		}
	}
	discovery, err := (censusacquisition.DiscoveryAdapter{
		Workspace: filepath.Dir(filepath.FromSlash(uri.Path)),
		Files:     censusacquisition.StaticFiles(files),
		Supplier:  privateTerminalSupplier{failures: failures},
		Client:    privateTerminalClient{uri: files[0].URI, mixed: mixed},
	}).Discover(context.Background(), censusacquisition.SessionIdentity{SessionID: "ready", Generation: 1})
	if err != nil {
		t.Fatal(err)
	}
	return discovery
}

func validClosedTerminalDiscovery(t *testing.T, plan censussourceplan.Plan) censusacquisition.Discovery {
	t.Helper()
	file := plan.Partitions[0].Members[0]
	uri, err := url.Parse(file.URI)
	if err != nil {
		t.Fatal(err)
	}
	discovery, err := (censusacquisition.DiscoveryAdapter{
		Workspace: filepath.Dir(filepath.FromSlash(uri.Path)),
		Files:     censusacquisition.StaticFiles{file},
		Supplier:  privateTerminalSupplier{},
		Client:    privateTerminalClient{uri: file.URI},
	}).Discover(context.Background(), censusacquisition.SessionIdentity{SessionID: "ready", Generation: 1})
	if err != nil {
		t.Fatal(err)
	}
	return discovery
}

func validMixedTerminalDiscovery(t *testing.T, plan censussourceplan.Plan) censusacquisition.Discovery {
	return validPrivateTerminalDiscovery(t, plan, true, nil)
}

func validLifecycleTerminalDiscovery(t *testing.T, plan censussourceplan.Plan, err error) censusacquisition.Discovery {
	return validPrivateTerminalDiscovery(t, plan, false, err)
}

func closedTerminalDiscovery(targets int) censusacquisition.Discovery {
	discovery := censusacquisition.Discovery{Closure: censusacquisition.DiscoveryClosureSuccessful}
	if targets > 0 {
		discovery.Complete = true
		discovery.Targets = make([]censusacquisition.PreparedTarget, targets)
	}
	return discovery
}

func TestPrivateDiscoveryTerminalCustodyShape(t *testing.T) {
	if _, ok := reflect.TypeOf(censusRuntimeResult{}).FieldByName("discoveryTerminal"); !ok {
		t.Fatal("ASSERT_PRIVATE_DISCOVERY_TERMINAL_RUNTIME_CUSTODY")
	}
	if _, ok := reflect.TypeOf(censusCompletion{}).FieldByName("discoveryTerminal"); !ok {
		t.Fatal("ASSERT_PRIVATE_DISCOVERY_TERMINAL_COMPLETION_CUSTODY")
	}
}

func TestPrivateDiscoveryTerminalIdentityAndValidation(t *testing.T) {
	plan := terminalPlanFixture(t, 2)
	discovery := validClosedTerminalDiscovery(t, plan)
	terminal, err := newPrivateConstituentDiscoveryTerminal(plan, discovery)
	if err != nil {
		t.Fatal(err)
	}
	if terminal.planIdentity != plan.Identity || terminal.authority != 0 || terminal.ordinal != 0 || !reflect.DeepEqual(terminal.members, plan.Partitions[0].Members) {
		t.Fatalf("ASSERT_PRIVATE_DISCOVERY_TERMINAL_PLAN_CUSTODY terminal=%+v plan=%+v", terminal, plan)
	}
	plan.Identity = "sha256:" + string(make([]byte, 64))
	if _, err := newPrivateConstituentDiscoveryTerminal(plan, discovery); err == nil {
		t.Fatal("ASSERT_PRIVATE_DISCOVERY_TERMINAL_MALFORMED_PLAN_REJECTED")
	}
}

func TestPrivateDiscoveryTerminalCloneSliceNilnessMatrix(t *testing.T) {
	type sliceCase struct {
		name  string
		set   func(*censusacquisition.Discovery, bool)
		isNil func(censusacquisition.Discovery) bool
	}
	cases := []sliceCase{
		{"Accounting.Files", func(d *censusacquisition.Discovery, empty bool) {
			if empty {
				d.Accounting.Files = make([]census.FileEntry, 0)
			}
		}, func(d censusacquisition.Discovery) bool { return d.Accounting.Files == nil }},
		{"Accounting.Symbols", func(d *censusacquisition.Discovery, empty bool) {
			if empty {
				d.Accounting.Symbols = make([]census.SymbolEntry, 0)
			}
		}, func(d censusacquisition.Discovery) bool { return d.Accounting.Symbols == nil }},
		{"FileLedger.Entries", func(d *censusacquisition.Discovery, empty bool) {
			if empty {
				d.FileLedger.Entries = make([]captureset.LedgerEntry, 0)
			}
		}, func(d censusacquisition.Discovery) bool { return d.FileLedger.Entries == nil }},
		{"SymbolLedger.Entries", func(d *censusacquisition.Discovery, empty bool) {
			if empty {
				d.SymbolLedger.Entries = make([]captureset.LedgerEntry, 0)
			}
		}, func(d censusacquisition.Discovery) bool { return d.SymbolLedger.Entries == nil }},
		{"Targets", func(d *censusacquisition.Discovery, empty bool) {
			if empty {
				d.Targets = make([]censusacquisition.PreparedTarget, 0)
			}
		}, func(d censusacquisition.Discovery) bool { return d.Targets == nil }},
		{"EnumerationSymbols", func(d *censusacquisition.Discovery, empty bool) {
			if empty {
				d.EnumerationSymbols = make([]censusacquisition.EnumerationSymbol, 0)
			}
		}, func(d censusacquisition.Discovery) bool { return d.EnumerationSymbols == nil }},
		{"ProviderFailures", func(d *censusacquisition.Discovery, empty bool) {
			if empty {
				d.ProviderFailures = make([]censusacquisition.ProviderFailure, 0)
			}
		}, func(d censusacquisition.Discovery) bool { return d.ProviderFailures == nil }},
		{"IncompleteCauses", func(d *censusacquisition.Discovery, empty bool) {
			if empty {
				d.IncompleteCauses = make([]censusacquisition.IncompleteCause, 0)
			}
		}, func(d censusacquisition.Discovery) bool { return d.IncompleteCauses == nil }},
		{"Observations", func(d *censusacquisition.Discovery, empty bool) {
			if empty {
				d.Observations = make([]censusacquisition.DiscoveryObservation, 0)
			}
		}, func(d censusacquisition.Discovery) bool { return d.Observations == nil }},
	}
	for _, tc := range cases {
		for _, empty := range []bool{false, true} {
			name := "nil"
			if empty {
				name = "non-nil-empty"
			}
			t.Run(tc.name+"/"+name, func(t *testing.T) {
				var original censusacquisition.Discovery
				tc.set(&original, empty)
				clone := clonePrivateTerminalDiscovery(original)
				if gotNil := tc.isNil(clone); gotNil == empty {
					t.Fatalf("ASSERT_PRIVATE_DISCOVERY_TERMINAL_SLICE_NILNESS field=%s empty=%t got_nil=%t", tc.name, empty, gotNil)
				}
			})
		}
	}
	for _, tc := range []struct {
		name   string
		values []censusacquisition.SourceFile
	}{{"nil", nil}, {"non-nil-empty", make([]censusacquisition.SourceFile, 0)}} {
		t.Run("members/"+tc.name, func(t *testing.T) {
			clone := cloneTerminalSlice(tc.values)
			if (clone == nil) != (tc.values == nil) {
				t.Fatalf("ASSERT_PRIVATE_DISCOVERY_TERMINAL_MEMBERS_NILNESS input_nil=%t output_nil=%t", tc.values == nil, clone == nil)
			}
		})
	}
}

func TestPrivateDiscoveryTerminalDeepCloneAndNilPreservation(t *testing.T) {
	for _, seed := range []struct {
		name  string
		value []byte
	}{
		{name: "nil"},
		{name: "non-nil-empty", value: make([]byte, 0)},
	} {
		t.Run("CanonicalSeedV2/"+seed.name, func(t *testing.T) {
			discovery := censusacquisition.Discovery{Targets: []censusacquisition.PreparedTarget{{CanonicalSeedV2: seed.value}}}
			clone := clonePrivateTerminalDiscovery(discovery)
			if gotNil := clone.Targets[0].CanonicalSeedV2 == nil; gotNil != (seed.value == nil) {
				t.Fatalf("ASSERT_PRIVATE_DISCOVERY_TERMINAL_CANONICAL_SEED_NILNESS input_nil=%t output_nil=%t", seed.value == nil, gotNil)
			}
		})
	}
	fileOrdinal, symbolOrdinal := 0, 1
	observed, omitted := uint64(2), uint64(3)
	discovery := closedTerminalDiscovery(1)
	discovery.Accounting = census.Accounting{FileDenominator: 1, Files: []census.FileEntry{{Ordinal: 0, Disposition: census.FileSelected}}}
	discovery.FileLedger = captureset.Ledger{Denominator: 1, Entries: []captureset.LedgerEntry{{Ordinal: 0, Identity: "a.go", Disposition: censusacquisition.FileProcessed}}}
	discovery.Targets[0].CanonicalSeedV2 = []byte("seed")
	discovery.ProviderFailures = []censusacquisition.ProviderFailure{{Stage: "x", Code: "y", FileOrdinal: &fileOrdinal}}
	discovery.IncompleteCauses = []censusacquisition.IncompleteCause{{Scope: "SYMBOL", Stage: "DISCOVERY", Code: "CANCELED", SymbolOrdinal: &symbolOrdinal}}
	discovery.Observations = []censusacquisition.DiscoveryObservation{{Observed: &observed, Limit: nil, Omitted: &omitted}}
	discovery.Closure = censusacquisition.DiscoveryClosureUnknown
	discovery.Complete = false
	clone := clonePrivateTerminalDiscovery(discovery)
	if clone.Observations[0].Limit != nil {
		t.Fatal("ASSERT_PRIVATE_DISCOVERY_TERMINAL_NIL_PRESERVED")
	}
	discovery.Targets[0].CanonicalSeedV2[0] = 'X'
	*discovery.ProviderFailures[0].FileOrdinal = 9
	*discovery.IncompleteCauses[0].SymbolOrdinal = 9
	*discovery.Observations[0].Observed = 9
	if string(clone.Targets[0].CanonicalSeedV2) != "seed" || *clone.ProviderFailures[0].FileOrdinal != 0 || *clone.IncompleteCauses[0].SymbolOrdinal != 1 || *clone.Observations[0].Observed != 2 {
		t.Fatal("ASSERT_PRIVATE_DISCOVERY_TERMINAL_DEEP_ANTI_ALIAS")
	}
}

func TestPrivateDiscoveryTerminalOutcomeClasses(t *testing.T) {
	cases := []struct {
		name string
		d    censusacquisition.Discovery
		want privateConstituentDiscoveryOutcome
	}{
		{"complete-nonempty", closedTerminalDiscovery(1), privateDiscoveryCompleteNonempty},
		{"successful-empty", closedTerminalDiscovery(0), privateDiscoverySuccessfulEmpty},
		{"partial", censusacquisition.Discovery{Targets: []censusacquisition.PreparedTarget{{}}}, privateDiscoveryPartial},
		{"failed-poison", censusacquisition.Discovery{IncompleteCauses: []censusacquisition.IncompleteCause{{Code: "SESSION_POISONED"}}}, privateDiscoveryFailed},
		{"refused-conflict", censusacquisition.Discovery{IncompleteCauses: []censusacquisition.IncompleteCause{{Code: "LIFECYCLE_CONFLICT"}}}, privateDiscoveryRefused},
		{"unknown-unavailable", censusacquisition.Discovery{ProviderFailures: []censusacquisition.ProviderFailure{{Code: "UNAVAILABLE"}}}, privateDiscoveryUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := privateDiscoveryOutcome(tc.d); got != tc.want {
				t.Fatalf("ASSERT_PRIVATE_DISCOVERY_TERMINAL_OUTCOME got=%q want=%q", got, tc.want)
			}
		})
	}
}

func TestPrivateDiscoveryTerminalRejectsContradictoryClosure(t *testing.T) {
	plan := terminalPlanFixture(t, 1)
	discovery := closedTerminalDiscovery(0)
	discovery.ProviderFailures = []censusacquisition.ProviderFailure{{Code: "UNAVAILABLE"}}
	if _, err := newPrivateConstituentDiscoveryTerminal(plan, discovery); err == nil {
		t.Fatal("ASSERT_PRIVATE_DISCOVERY_TERMINAL_CONTRADICTORY_CLOSURE_REJECTED")
	}
}

func TestPrivateDiscoveryTerminalRejectsCompleteWithFailureCustodyRegardlessOfClosure(t *testing.T) {
	plan := terminalPlanFixture(t, 1)
	cases := []struct {
		name string
		d    censusacquisition.Discovery
	}{
		{"provider-failure", censusacquisition.Discovery{Complete: true, ProviderFailures: []censusacquisition.ProviderFailure{{Code: "UNAVAILABLE"}}}},
		{"incomplete-cause", censusacquisition.Discovery{Complete: true, IncompleteCauses: []censusacquisition.IncompleteCause{{Code: "CANCELED"}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := newPrivateConstituentDiscoveryTerminal(plan, tc.d); err == nil {
				t.Fatalf("ASSERT_PRIVATE_DISCOVERY_TERMINAL_COMPLETE_FAILURE_CUSTODY_REJECTED case=%s", tc.name)
			}
		})
	}
}

func TestPrivateDiscoveryTerminalRunCustodyAndErrorBoundary(t *testing.T) {
	workspace := t.TempDir()
	writeRuntimeSource(t, workspace, "a.go")
	writeRuntimeSource(t, workspace, "b.go")
	runtime, _, started := censusRuntimeFixture(t, censusRuntimeOptions{workspace: workspace, ready: true, documentSymbolSupport: true, callHierarchySupport: true})
	shell := newCensusRuntime(runtime)
	shell.enumerate = func(context.Context, censusAdmittedSession, censusRuntimeConfig) ([]censusacquisition.SourceFile, error) {
		files := []censusacquisition.SourceFile{}
		for _, name := range []string{"a.go", "b.go"} {
			files = append(files, censusacquisition.SourceFile{Path: name, URI: (&url.URL{Scheme: "file", Path: filepath.Join(workspace, name)}).String(), LanguageID: "go"})
		}
		return files, nil
	}
	discoveryCalls := 0
	shell.discoverEnumerated = func(context.Context, *sessionruntime.Manager, censusAdmittedSession, censusRuntimeConfig, bool, censusacquisition.FileEnumerator) (censusacquisition.Discovery, error) {
		discoveryCalls++
		return censusacquisition.Discovery{Closure: censusacquisition.DiscoveryClosureSuccessful}, nil
	}
	request := censusRuntimeRequest(t, openCensusRuntimeRoot(t), runtimeRaw(t, "project", started.Generation, nil))
	request.PrivateCensusChunking = true
	completion := shell.run(context.Background(), operation.Request(request))
	if discoveryCalls != 1 || completion.discoveryTerminal == nil || completion.discoveryTerminal.outcome != privateDiscoverySuccessfulEmpty || len(completion.discoveryTerminal.members) != 2 {
		t.Fatalf("ASSERT_PRIVATE_DISCOVERY_TERMINAL_SINGLE_DISCOVERY_CUSTODY calls=%d completion=%+v", discoveryCalls, completion)
	}

	shell.discoverEnumerated = func(context.Context, *sessionruntime.Manager, censusAdmittedSession, censusRuntimeConfig, bool, censusacquisition.FileEnumerator) (censusacquisition.Discovery, error) {
		return censusacquisition.Discovery{}, errors.New("sentinel")
	}
	completion = shell.run(context.Background(), operation.Request(request))
	if completion.discoveryTerminal != nil || completion.Diagnostic == nil {
		t.Fatalf("ASSERT_PRIVATE_DISCOVERY_TERMINAL_ERROR_HAS_NO_CUSTODY completion=%+v", completion)
	}
}
