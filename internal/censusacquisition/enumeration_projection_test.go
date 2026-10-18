package censusacquisition

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/census"
	"lsp-trace/internal/lsp"
)

type projectionFailSupplier struct{ err error }

func (s projectionFailSupplier) Supply(context.Context, SourceFile) error { return s.err }

type projectionClient struct {
	symbol     lsp.DocumentSymbol
	prepareErr error
}

func (projectionClient) SupportsDocumentSymbols() bool { return true }
func (projectionClient) SupportsCallHierarchy() bool   { return true }
func (c projectionClient) DocumentSymbols(context.Context, lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error) {
	return []lsp.DocumentSymbol{c.symbol}, nil
}
func (c projectionClient) PrepareCallHierarchy(context.Context, lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	return nil, c.prepareErr
}

func TestStaticFilesPredispositionsProduceClosedProviderFailures(t *testing.T) {
	workspace := t.TempDir()
	file := func(name string, disposition census.FileDisposition) SourceFile {
		return SourceFile{Path: name, URI: (&url.URL{Scheme: "file", Path: filepath.Join(workspace, name)}).String(), LanguageID: "go", Disposition: disposition}
	}
	discovery, err := (DiscoveryAdapter{Workspace: workspace, Files: StaticFiles{file("unreadable.go", census.FileUnreadable), file("unsupported.go", census.FileUnsupported)}, Supplier: projectionFailSupplier{}, Client: projectionClient{}}).Discover(context.Background(), SessionIdentity{SessionID: "s", Generation: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(discovery.ProviderFailures) != 2 || discovery.ProviderFailures[0].Stage != "DOCUMENT_SUPPLY" || discovery.ProviderFailures[0].Code != "UNAVAILABLE" || discovery.ProviderFailures[1].Stage != "DOCUMENT_SYMBOL" || discovery.ProviderFailures[1].Code != "UNSUPPORTED" {
		t.Fatalf("failures=%+v", discovery.ProviderFailures)
	}
}

func TestSourceFilePathRejectsNonLocalAndNoncanonicalForms(t *testing.T) {
	workspace := t.TempDir()
	for _, candidate := range []string{"", "../secret.go", "a/../secret.go", "file:/x.go", "file://x.go", "C:", "C:/x.go", "//host/x.go", `a\\b.go`, "./a.go", "a/./b.go", "a//b.go", "a\x00b.go", string([]byte{0xff})} {
		t.Run(strings.ReplaceAll(candidate, "/", "_"), func(t *testing.T) {
			f := SourceFile{Path: candidate, URI: (&url.URL{Scheme: "file", Path: filepath.Join(workspace, "x.go")}).String()}
			if err := validateSourceFile(f, workspace); err == nil {
				t.Fatalf("accepted %q", candidate)
			}
		})
	}
}

func TestEnumerationProviderFailuresClosedNoRawTextAndNoFabricatedSymbols(t *testing.T) {
	workspace := t.TempDir()
	uri := (&url.URL{Scheme: "file", Path: filepath.Join(workspace, "a.go")}).String()
	session := SessionIdentity{SessionID: "s", Generation: 1}
	raw := "SECRET absolute /tmp/provider prose"
	failed, err := (DiscoveryAdapter{Workspace: workspace, Files: StaticFiles{{Path: "a.go", URI: uri, LanguageID: "go"}}, Supplier: projectionFailSupplier{errors.New(raw)}, Client: projectionClient{}}).Discover(context.Background(), session)
	if err != nil || len(failed.EnumerationSymbols) != 0 || failed.SymbolLedger.Denominator != 0 || len(failed.ProviderFailures) != 1 || failed.ProviderFailures[0].Stage != "DOCUMENT_SUPPLY" || failed.ProviderFailures[0].Code != "REQUEST_FAILED" || strings.Contains(failed.ProviderFailures[0].Stage+failed.ProviderFailures[0].Code, raw) {
		t.Fatalf("discovery=%+v err=%v", failed, err)
	}
	pos := lsp.Position{Line: 1, Character: 2}
	symbol := lsp.DocumentSymbol{Name: "privateName", Kind: 12, Range: lsp.Range{Start: pos, End: pos}, SelectionRange: lsp.Range{Start: pos, End: pos}}
	prepared, err := (DiscoveryAdapter{Workspace: workspace, Files: StaticFiles{{Path: "a.go", URI: uri, LanguageID: "go"}}, Supplier: projectionFailSupplier{}, Client: projectionClient{symbol: symbol, prepareErr: errors.New(raw)}}).Discover(context.Background(), session)
	if err != nil || len(prepared.EnumerationSymbols) != 1 || len(prepared.ProviderFailures) != 1 || prepared.ProviderFailures[0].Stage != "PREPARE_CALL_HIERARCHY" || prepared.ProviderFailures[0].Code != "REQUEST_FAILED" {
		t.Fatalf("discovery=%+v err=%v", prepared, err)
	}
}
