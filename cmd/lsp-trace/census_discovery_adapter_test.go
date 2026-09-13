package main

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/lsp"
	"lsp-trace/sessionruntime"
)

type fakeCensusInitializedSession struct {
	workspace string
	supplied  map[string][]byte
}

func (f *fakeCensusInitializedSession) SessionID() string  { return "initialized" }
func (f *fakeCensusInitializedSession) Generation() uint64 { return 7 }
func (f *fakeCensusInitializedSession) PrepareDocument(ctx context.Context, r sessionruntime.DocumentRequest) sessionruntime.DocumentResult {
	if ctx.Err() != nil {
		return sessionruntime.DocumentResult{Failure: sessionruntime.DocumentSupplyUnavailable}
	}
	u, _ := url.Parse(r.URI)
	b, err := os.ReadFile(filepath.FromSlash(u.Path))
	if err != nil {
		return sessionruntime.DocumentResult{Failure: sessionruntime.DocumentSupplyUnavailable}
	}
	owned := append([]byte(nil), b...)
	if f.supplied == nil {
		f.supplied = map[string][]byte{}
	}
	f.supplied[r.URI] = owned
	return sessionruntime.DocumentResult{URI: r.URI, LanguageID: r.LanguageID, Version: 1, Supply: &sessionruntime.DocumentSupply{URI: r.URI, SessionID: f.SessionID(), Generation: f.Generation(), Content: append([]byte(nil), owned...)}}
}

type fakeCensusDiscoveryClient struct{ uris []string }

func (f *fakeCensusDiscoveryClient) SupportsDocumentSymbols() bool { return true }
func (f *fakeCensusDiscoveryClient) SupportsCallHierarchy() bool   { return true }
func (f *fakeCensusDiscoveryClient) DocumentSymbols(_ context.Context, p lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error) {
	f.uris = append(f.uris, p.TextDocument.URI)
	return nil, nil
}
func (*fakeCensusDiscoveryClient) PrepareCallHierarchy(context.Context, lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	return nil, nil
}

func writeCensusSource(t *testing.T, root, name, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func discoverCensusFixture(t *testing.T, workspace string, mutate func(*censusCLIOptions)) (censusacquisition.Discovery, *fakeCensusInitializedSession, *fakeCensusDiscoveryClient) {
	t.Helper()
	o := censusCLIOptions{Workspace: workspace, Sources: []string{"."}, MaxNodes: 100}
	if mutate != nil {
		mutate(&o)
	}
	s := &fakeCensusInitializedSession{workspace: workspace}
	c := &fakeCensusDiscoveryClient{}
	d, err := newCensusDiscoveryAdapter(o, s, c)
	if err != nil {
		t.Fatal(err)
	}
	got, err := d.Discover(context.Background(), censusacquisition.SessionIdentity{SessionID: s.SessionID(), Generation: s.Generation()})
	if err != nil {
		t.Fatal(err)
	}
	return got, s, c
}

func TestCensusDiscoveryAdapterDefaultRootDeterministicUnicodeAndExactBytes(t *testing.T) {
	workspace := t.TempDir()
	writeCensusSource(t, workspace, "z.go", "package z\n")
	writeCensusSource(t, workspace, "α/a.go", "package α\n// 😀\n")
	first, session, client := discoverCensusFixture(t, workspace, nil)
	second, _, secondClient := discoverCensusFixture(t, workspace, nil)
	if !reflect.DeepEqual(first.FileLedger, second.FileLedger) || !reflect.DeepEqual(client.uris, secondClient.uris) {
		t.Fatalf("ASSERT_CENSUS_DETERMINISTIC: first=%+v second=%+v", first.FileLedger, second.FileLedger)
	}
	if len(first.FileLedger.Entries) != 2 || first.FileLedger.Entries[0].Identity != "z.go" && first.FileLedger.Entries[0].Identity != "α/a.go" {
		t.Fatalf("ASSERT_CENSUS_DEFAULT_DOT: %+v", first.FileLedger)
	}
	for uri, supplied := range session.supplied {
		u, _ := url.Parse(uri)
		want, _ := os.ReadFile(filepath.FromSlash(u.Path))
		if !reflect.DeepEqual(supplied, want) {
			t.Fatalf("ASSERT_CENSUS_EXACT_BYTES: %q", uri)
		}
	}
}

func TestCensusDiscoveryAdapterExcludeBeforeIncludeAndOverlapDeduplicates(t *testing.T) {
	workspace := t.TempDir()
	writeCensusSource(t, workspace, "src/a.go", "package a\n")
	writeCensusSource(t, workspace, "src/generated/b.go", "package b\n")
	got, _, client := discoverCensusFixture(t, workspace, func(o *censusCLIOptions) {
		o.Sources = []string{"src", "src/a.go"}
		o.Includes = []string{"**/*.go"}
		o.Excludes = []string{"src/generated/**"}
	})
	if got.Accounting.FileDenominator != 2 || len(client.uris) != 1 {
		t.Fatalf("ASSERT_CENSUS_OVERLAP_FILTER_ACCOUNTING: accounting=%+v uris=%v", got.Accounting, client.uris)
	}
	if got.Accounting.Files[1].Disposition != census.FileExcluded && got.Accounting.Files[0].Disposition != census.FileExcluded {
		t.Fatalf("ASSERT_CENSUS_EXCLUDE_PRECEDENCE: %+v", got.Accounting.Files)
	}
}

func TestCensusDiscoveryAdapterUnsafeSourcesAreTerminalAndNeverSupplied(t *testing.T) {
	workspace := t.TempDir()
	writeCensusSource(t, workspace, "ok.go", "package ok\n")
	if err := os.Symlink(filepath.Join(workspace, "ok.go"), filepath.Join(workspace, "link.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(workspace, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, _, client := discoverCensusFixture(t, workspace, nil)
	if got.Accounting.FileDenominator != len(got.Accounting.Files) || got.FileLedger.Denominator != len(got.FileLedger.Entries) {
		t.Fatalf("ASSERT_CENSUS_TERMINAL_ACCOUNTING: %+v %+v", got.Accounting, got.FileLedger)
	}
	if len(client.uris) != 1 || !strings.HasSuffix(client.uris[0], "/ok.go") {
		t.Fatalf("ASSERT_CENSUS_UNSAFE_NOT_SUPPLIED: %v", client.uris)
	}
	for _, entry := range got.Accounting.Files {
		if entry.Disposition == "" {
			t.Fatal("ASSERT_CENSUS_TERMINAL_DISPOSITION")
		}
	}
}

func TestCensusDiscoveryAdapterCancellationZeroFilesAndClosedErrors(t *testing.T) {
	workspace := t.TempDir()
	s := &fakeCensusInitializedSession{}
	c := &fakeCensusDiscoveryClient{}
	d, err := newCensusDiscoveryAdapter(censusCLIOptions{Workspace: workspace, Sources: []string{"."}, MaxNodes: 1}, s, c)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = d.Discover(ctx, censusacquisition.SessionIdentity{SessionID: s.SessionID(), Generation: s.Generation()})
	if err == nil || len(c.uris) != 0 {
		t.Fatalf("ASSERT_CENSUS_CANCELLATION: err=%v calls=%v", err, c.uris)
	}
	got, err := d.Discover(context.Background(), censusacquisition.SessionIdentity{SessionID: s.SessionID(), Generation: s.Generation()})
	if err != nil || got.Accounting.FileDenominator != 0 || len(c.uris) != 0 {
		t.Fatalf("ASSERT_CENSUS_ZERO_FILES: got=%+v err=%v calls=%v", got, err, c.uris)
	}
	_, err = newCensusDiscoveryAdapter(censusCLIOptions{Workspace: filepath.Join(workspace, "missing")}, s, c)
	failure, ok := err.(censusDiscoveryFailure)
	if !ok || failure.Stage != censusStageDiscovery || failure.Code != censusCodeDiscoveryFailed || strings.Contains(err.Error(), workspace) {
		t.Fatalf("ASSERT_CENSUS_CLOSED_ERROR: %#v", err)
	}
}

func TestCensusRuntimeSupplierSnapshotsMutation(t *testing.T) {
	workspace := t.TempDir()
	writeCensusSource(t, workspace, "race.go", "package before\n")
	uri := (&url.URL{Scheme: "file", Path: filepath.Join(workspace, "race.go")}).String()
	s := &fakeCensusInitializedSession{}
	supplier := censusRuntimeDocumentSupplier{runtime: s}
	if err := supplier.Supply(context.Background(), censusacquisition.SourceFile{Path: "race.go", URI: uri, LanguageID: "go"}); err != nil {
		t.Fatal(err)
	}
	before := append([]byte(nil), s.supplied[uri]...)
	writeCensusSource(t, workspace, "race.go", "package after\n")
	if string(before) != "package before\n" || string(s.supplied[uri]) != "package before\n" {
		t.Fatalf("ASSERT_CENSUS_IMMUTABLE_SNAPSHOT: before=%q supplied=%q", before, s.supplied[uri])
	}
}
