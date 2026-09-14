package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/publication"
)

func validCensusProjection() censusacquisition.Projection {
	return censusacquisition.Projection{Session: censusacquisition.SessionIdentity{SessionID: "session", Generation: 2}, CensusID: "census", Manifest: captureset.Manifest{LogicalDigest: "capture-set", FileLedger: captureset.Ledger{Denominator: 1, Entries: []captureset.LedgerEntry{{Ordinal: 0, Identity: "private/file.go", Disposition: censusacquisition.FileProcessed}}}, SymbolLedger: captureset.Ledger{Denominator: 1, Entries: []captureset.LedgerEntry{{Ordinal: 0, Identity: "private.symbol", Disposition: censusacquisition.SymbolPrepared}}}, Targets: []captureset.Target{{CensusOrdinal: 0}}, Batches: []captureset.Batch{{Ordinal: 0, TargetCount: 1}}}}
}
func validCensusReceipt() publication.BoundFileReceipt {
	return publication.BoundFileReceipt{FinalSelector: "private/census.json", Digest: "sha256:" + strings.Repeat("a", 64), ByteLength: 12, VerificationStatus: "VERIFIED", DirectorySyncStatus: publication.DirectorySyncComplete, CloseStatus: publication.CloseComplete}
}

type censusFixtureSupplier struct{}

func (censusFixtureSupplier) Supply(context.Context, censusacquisition.SourceFile) error { return nil }

type censusFixtureClient struct {
	uri      string
	symbol   lsp.DocumentSymbol
	prepared lsp.CallHierarchyItem
}

func (c censusFixtureClient) SupportsDocumentSymbols() bool { return true }
func (c censusFixtureClient) SupportsCallHierarchy() bool   { return true }
func (c censusFixtureClient) DocumentSymbols(_ context.Context, p lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error) {
	if p.TextDocument.URI != c.uri {
		return nil, nil
	}
	return []lsp.DocumentSymbol{c.symbol}, nil
}
func (c censusFixtureClient) PrepareCallHierarchy(_ context.Context, p lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	if p.TextDocument.URI != c.uri || p.Position != c.symbol.SelectionRange.Start {
		return nil, nil
	}
	return []lsp.CallHierarchyItem{c.prepared}, nil
}

func integratedCensusProjection(t *testing.T) censusacquisition.Projection {
	t.Helper()
	workspace := t.TempDir()
	fileURI := (&url.URL{Scheme: "file", Path: filepath.Join(workspace, "a.go")}).String()
	position := lsp.Position{Line: 1, Character: 2}
	symbol := lsp.DocumentSymbol{Name: "Run", Kind: 12, Range: lsp.Range{Start: position, End: position}, SelectionRange: lsp.Range{Start: position, End: position}}
	client := censusFixtureClient{uri: fileURI, symbol: symbol, prepared: lsp.CallHierarchyItem{Name: symbol.Name, Kind: symbol.Kind, URI: fileURI, Range: symbol.Range, SelectionRange: symbol.SelectionRange}}
	discovery, err := (censusacquisition.DiscoveryAdapter{
		Workspace: workspace,
		Files: censusacquisition.StaticFiles{
			{Path: "a.go", URI: fileURI, LanguageID: "go"},
			{Path: "z.go", URI: (&url.URL{Scheme: "file", Path: filepath.Join(workspace, "z.go")}).String(), LanguageID: "go"},
		},
		Supplier: censusFixtureSupplier{}, Client: client,
		Limits: censusacquisition.DiscoveryLimits{MaxFiles: 1},
	}).Discover(context.Background(), censusacquisition.SessionIdentity{SessionID: "session", Generation: 2})
	if err != nil {
		t.Fatal(err)
	}
	discovery.FileLedger.Denominator++
	discovery.FileLedger.Entries = append(discovery.FileLedger.Entries, captureset.LedgerEntry{Ordinal: 2, Identity: "private/omitted.go", Disposition: string(census.FileOmitted)})
	discovery.SymbolLedger.Denominator++
	discovery.SymbolLedger.Entries = append(discovery.SymbolLedger.Entries, captureset.LedgerEntry{Ordinal: 1, Identity: "private.omitted", Disposition: string(census.SymbolOmitted)})
	return censusacquisition.Projection{Session: discovery.Session, CensusID: "census", Manifest: captureset.Manifest{LogicalDigest: "capture-set", FileLedger: discovery.FileLedger, SymbolLedger: discovery.SymbolLedger, Targets: []captureset.Target{{CensusOrdinal: 0}}, Batches: []captureset.Batch{{Ordinal: 0, TargetCount: 1}}}}
}

func TestCensusCLIResultClosedDeterministicPrivateProjection(t *testing.T) {
	p := validCensusProjection()
	r, err := buildCensusCLIResult(p, validCensusReceipt())
	if err != nil {
		t.Fatal(err)
	}
	a, err := marshalCensusCLIResult(r)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := marshalCensusCLIResult(r)
	if string(a) != string(b) {
		t.Fatal("ASSERT_CENSUS_DETERMINISTIC_BYTES")
	}
	var raw map[string]json.RawMessage
	if err = json.Unmarshal(a, &raw); err != nil {
		t.Fatal(err)
	}
	want := []string{"authority", "batch_count", "capture_set_id", "census_id", "cross_capture_calls", "file_accounting", "generation", "leiden_admissible", "native_aggregate_custody", "publication", "schema_version", "session_id", "source_graph_complete", "status", "symbol_accounting", "target_count"}
	got := make([]string, 0, len(raw))
	for k := range raw {
		got = append(got, k)
	}
	sortStrings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("keys=%v", got)
	}
	if strings.Contains(string(a), "private/file.go") || strings.Contains(string(a), "private.symbol") || strings.Contains(string(a), "Workspace") {
		t.Fatal("ASSERT_CENSUS_PRIVATE_VALUE_LEAK")
	}
	if r.Authority != 0 || r.SourceGraphComplete != "UNKNOWN" || r.NativeAggregateCustody || r.CrossCaptureCalls == nil || len(r.CrossCaptureCalls) != 0 || r.LeidenAdmissible {
		t.Fatal("ASSERT_CENSUS_FIXED_CEILING")
	}
	if r.Publication.DirectorySyncStatus != "COMPLETE" || r.Publication.CloseStatus != "COMPLETE" || strings.Contains(string(a), publication.DirectorySyncComplete) || strings.Contains(string(a), publication.CloseComplete) {
		t.Fatalf("ASSERT_CENSUS_PUBLICATION_STATUS_NORMALIZED: publication=%+v raw=%s", r.Publication, a)
	}
}
func sortStrings(v []string) {
	for i := range v {
		for j := i + 1; j < len(v); j++ {
			if v[j] < v[i] {
				v[i], v[j] = v[j], v[i]
			}
		}
	}
}

func TestCensusCLIResultDiscoveryLedgerVocabularyAndMixedAccounting(t *testing.T) {
	p := integratedCensusProjection(t)
	if got := []string{p.Manifest.FileLedger.Entries[0].Disposition, p.Manifest.SymbolLedger.Entries[0].Disposition, p.Manifest.FileLedger.Entries[1].Disposition}; !reflect.DeepEqual(got, []string{"processed", "prepared", string(census.FileIncomplete)}) {
		t.Fatalf("ASSERT_CENSUS_DISCOVERY_LEDGER_SUCCESS_VOCABULARY: %v", got)
	}
	r, err := buildCensusCLIResult(p, validCensusReceipt())
	if err != nil {
		t.Fatal("ASSERT_CENSUS_DISCOVERY_LEDGER_ACCOUNTING", err)
	}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var projected struct {
		File   struct{ Denominator, Processed, Incomplete, Omitted int } `json:"file_accounting"`
		Symbol struct{ Denominator, Prepared, Omitted int }              `json:"symbol_accounting"`
	}
	if err := json.Unmarshal(raw, &projected); err != nil || projected.File.Denominator != 3 || projected.File.Processed != 1 || projected.File.Incomplete != 1 || projected.File.Omitted != 1 || projected.Symbol.Denominator != 2 || projected.Symbol.Prepared != 1 || projected.Symbol.Omitted != 1 {
		t.Fatalf("ASSERT_CENSUS_DISCOVERY_LEDGER_ACCOUNTING: file=%+v symbol=%+v err=%v", projected.File, projected.Symbol, err)
	}
}

func TestCensusCLIResultRejectsInvalidBoundsPrivacyAndAccounting(t *testing.T) {
	cases := []func(*censusacquisition.Projection, *publication.BoundFileReceipt){
		func(p *censusacquisition.Projection, r *publication.BoundFileReceipt) { p.Session.Generation = 0 },
		func(p *censusacquisition.Projection, r *publication.BoundFileReceipt) {
			p.Manifest.FileLedger.Entries[0].Ordinal = 1
		},
		func(p *censusacquisition.Projection, r *publication.BoundFileReceipt) { r.FinalSelector = "../escape" },
		func(p *censusacquisition.Projection, r *publication.BoundFileReceipt) { r.Digest = "private" },
		func(p *censusacquisition.Projection, r *publication.BoundFileReceipt) {
			r.VerificationStatus = "FAILED"
		},
		func(p *censusacquisition.Projection, r *publication.BoundFileReceipt) {
			r.CloseStatus = publication.CloseNotAttempted
		},
	}
	for i, mut := range cases {
		p, r := validCensusProjection(), validCensusReceipt()
		mut(&p, &r)
		if _, err := buildCensusCLIResult(p, r); err == nil {
			t.Fatalf("case %d accepted", i)
		}
	}
}

func TestCensusCLIDiagnosticClosedMatrix(t *testing.T) {
	stages := []censusFailureStage{censusStageSyntax, censusStageConfig, censusStageDiscovery, censusStageAcquisition, censusStageAssembly, censusStagePublication, censusStageCommitted}
	for _, s := range stages {
		d, err := buildCensusCLIDiagnostic(s, nil)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := marshalCensusCLIDiagnostic(d)
		if err != nil {
			t.Fatal(err)
		}
		repeated, err := marshalCensusCLIDiagnostic(d)
		if err != nil || !bytes.Equal(raw, repeated) {
			t.Fatalf("ASSERT_DETERMINISTIC_DIAGNOSTIC_BYTES first=%q repeated=%q err=%v", raw, repeated, err)
		}
		if !strings.HasSuffix(string(raw), "\n") {
			t.Fatal("not jsonl")
		}
		if strings.Contains(string(raw), "error") || s == censusStageCommitted && (d.Status != "SUCCEEDED_DEGRADED" || d.Retry) {
			t.Fatalf("diagnostic=%s", raw)
		}
	}
	for _, n := range []int{1, 2, 3} {
		d, err := buildCensusCLIDiagnostic(censusStageAcquisition, &n)
		if err != nil {
			t.Fatal(err)
		}
		first, err := marshalCensusCLIDiagnostic(d)
		if err != nil {
			t.Fatal(err)
		}
		second, err := marshalCensusCLIDiagnostic(d)
		if err != nil || !bytes.Equal(first, second) {
			t.Fatalf("ASSERT_DETERMINISTIC_BATCH_DIAGNOSTIC ordinal=%d", n)
		}
	}
	n := 3
	if _, err := buildCensusCLIDiagnostic(censusStageDiscovery, &n); err == nil {
		t.Fatal("ordinal accepted outside acquisition")
	}
	if _, err := buildCensusCLIDiagnostic("unknown", nil); err == nil {
		t.Fatal("unknown stage accepted")
	}
}

func TestParseCensusCLIOptionsPureDefaultsMachineAndPreflight(t *testing.T) {
	beforeEnv := os.Environ()
	beforeWD, _ := os.Getwd()
	base := []string{"census", "--workspace", "work", "--source", ".", "--publication-root", "/private"}
	cases := []struct {
		name  string
		args  []string
		check func(censusCLIOptions) bool
	}{
		{"direct-server", []string{"--server", "gopls", "--server-arg", "--machine"}, func(o censusCLIOptions) bool {
			return o.Server == "gopls" && reflect.DeepEqual(o.ServerArgs, []string{"--machine"})
		}},
		{"profile-server", []string{"--profile", "p", "--server", "gopls"}, func(o censusCLIOptions) bool { return o.Profile == "p" && o.Server == "gopls" }},
		{"profile-server-arg", []string{"--profile", "p", "--server-arg", "one", "--server-arg", "two"}, func(o censusCLIOptions) bool {
			return o.Profile == "p" && reflect.DeepEqual(o.ServerArgs, []string{"one", "two"})
		}},
		{"profile-config-overrides", []string{"--profile", "p", "--config", "profiles.toml", "--server", "gopls", "--server-arg", "one"}, func(o censusCLIOptions) bool {
			return o.ConfigPath == "profiles.toml" && o.Server == "gopls" && reflect.DeepEqual(o.ServerArgs, []string{"one"})
		}},
		{"repeated-sources-filters", []string{"--profile", "p", "--source", "src", "--include", "**/*.go", "--include", "**/*.mod", "--exclude", "vendor/**", "--exclude", "tmp/**"}, func(o censusCLIOptions) bool {
			return reflect.DeepEqual(o.Sources, []string{".", "src"}) && len(o.Includes) == 2 && len(o.Excludes) == 2
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o, err := parseCensusCLIOptions(append(append([]string{}, base...), tc.args...))
			if err != nil || !tc.check(o) {
				t.Fatalf("ASSERT_CENSUS_PROFILE_OVERRIDE_PRECEDENCE: options=%+v err=%v", o, err)
			}
		})
	}
	machineArgs := append([]string{"census", "--machine"}, base[1:]...)
	o, err := parseCensusCLIOptions(append(machineArgs, "--server", "gopls", "--server-arg", "--machine"))
	if err != nil || o.DownDepth != census.DefaultDownDepth || o.UpDepth != census.DefaultUpDepth || !o.Machine {
		t.Fatalf("options=%+v err=%v", o, err)
	}
	for _, args := range [][]string{
		append(append(append([]string{}, base...), "--server", "gopls"), "--machine"),
		append(append(append([]string{}, base[:3]...), append([]string{"--machine"}, base[3:]...)...), "--server", "gopls"),
	} {
		got, err := parseCensusCLIOptions(args)
		if err != nil || !got.Machine {
			t.Fatalf("ASSERT_CENSUS_MACHINE_FLAG_CONVENTIONAL_PLACEMENT: args=%v options=%+v err=%v", args, got, err)
		}
	}
	afterWD, _ := os.Getwd()
	if beforeWD != afterWD || !reflect.DeepEqual(beforeEnv, os.Environ()) {
		t.Fatal("ASSERT_CENSUS_PARSE_SIDE_EFFECT")
	}
	bad := [][]string{
		{"census", "--machine", "--machine=true"},
		{"census", "positional"},
		append([]string{}, base...),
		append(append([]string{}, base...), "--config", "profiles.toml", "--server", "gopls"),
		append(append([]string{}, base...), "--server-arg", "orphan"),
		append(append([]string{}, base...), "--profile", "p", "--down-depth", "-1"),
		append(append([]string{}, base...), "--profile", "p", "--request-timeout", "2m", "--timeout", "1m"),
	}
	for i, a := range bad {
		if _, err := parseCensusCLIOptions(a); err == nil {
			t.Fatalf("ASSERT_CENSUS_REPEATABLE_FLAGS_AND_STRICT_INVALID_MATRIX: bad %d accepted", i)
		}
	}
	h, err := parseCensusCLIOptions([]string{"census", "--machine=false", "--help"})
	if err != nil || !h.Help || h.Machine {
		t.Fatalf("help=%+v err=%v", h, err)
	}
}
