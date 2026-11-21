package censusacquisition

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"lsp-trace/internal/census"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/source"
)

func TestSelectedSourceFilesOverlappingRoots(t *testing.T) {
	workspace := t.TempDir()
	if err := os.Mkdir(filepath.Join(workspace, "src"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.ts", "a_test.ts"} {
		if err := os.WriteFile(filepath.Join(workspace, "src", name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(workspace, "src", "alias")); err != nil {
		t.Fatal(err)
	}
	cursor, err := source.OpenRecursiveCursor(workspace, []string{"src", "src/a.ts", "src"}, nil, source.Limits{MaxEntries: 16, MaxWork: 64, MaxPathBytes: 4096, MaxDepth: 8})
	if err != nil {
		t.Fatal(err)
	}
	defer cursor.Close()
	var records []source.Record
	for {
		batch, done, err := cursor.Next(context.Background(), 2)
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, batch...)
		if done {
			break
		}
	}
	if len(records) != 4 {
		t.Fatalf("directory/file/overlap accounting: %+v", records)
	}
	files := PreparedWorkspaceFiles(workspace, records)
	selected, err := SelectedSourceFiles(workspace, files, Filters{Includes: []string{"src/*.ts"}, Excludes: []string{"**/*_test.ts"}})
	if err != nil || len(selected) != 1 || selected[0].Path != "src/a.ts" {
		t.Fatalf("root-relative selection: %+v %v", selected, err)
	}
	if len(files) != 3 {
		t.Fatalf("full file denominator: %+v", files)
	}
	for _, f := range files {
		if f.Path == "src/alias" && f.Disposition != census.FileUnsupported {
			t.Fatal("symlink gained acquisition eligibility")
		}
	}
}

func TestSelectedSourceFilesDiscoveryParity(t *testing.T) {
	workspace := t.TempDir()
	names := []string{"cmd/lsp-trace/census.go", "cmd/lsp-trace/census_test.go", "cmd/lsp-trace/nested/census.go", "internal/applicationinventory/a.go", "src/App.ts", "src/nested/tool.py", "src/Thing.cs"}
	files := make([]SourceFile, 0, len(names))
	for _, name := range names {
		f := testSource(workspace, name)
		f.LanguageID = "controlled-language"
		files = append(files, f)
	}
	original := append([]SourceFile(nil), files...)
	for _, tc := range []struct {
		name    string
		filters Filters
		want    []string
	}{
		{"requested-root-relative", Filters{Includes: []string{"cmd/lsp-trace/census*.go", "internal/applicationinventory/*.go"}, Excludes: []string{"**/*_test.go"}}, []string{names[0], names[3]}},
		{"windows-pattern-slashes", Filters{Includes: []string{`src\**`}, Excludes: []string{`**\*.cs`}}, []string{names[4], names[5]}},
		{"basename", Filters{Includes: []string{"*.py"}}, []string{names[5]}},
		{"anchored", Filters{Includes: []string{"/src/*.ts"}}, []string{names[4]}},
		{"trailing-directory", Filters{Includes: []string{"src/"}, Excludes: []string{"**/*.py", "**/*.cs"}}, []string{names[4]}},
		{"exclude-wins", Filters{Includes: []string{"**"}, Excludes: []string{"**"}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			selected, err := SelectedSourceFiles(workspace, files, tc.filters)
			if err != nil {
				t.Fatal(err)
			}
			var paths []string
			for _, f := range selected {
				paths = append(paths, f.Path)
			}
			if !reflect.DeepEqual(paths, tc.want) {
				t.Fatalf("selected=%v want=%v", paths, tc.want)
			}
			supplier := &fakeSupplier{}
			client := &fakeDiscoveryClient{documentSupported: true, callSupported: true}
			got, err := (DiscoveryAdapter{Workspace: workspace, Files: StaticFiles(files), Filters: tc.filters, Supplier: supplier, Client: client, Limits: DiscoveryLimits{MaxFiles: len(selected)}}).Discover(context.Background(), SessionIdentity{"controlled", 1})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(supplier.calls, paths) {
				t.Fatalf("selection/supply mismatch: %v / %v", paths, supplier.calls)
			}
			if got.Accounting.FileDenominator != len(files) || got.FileLedger.Denominator != len(files) || len(got.Accounting.Files) != len(files) {
				t.Fatalf("denominator lost: %+v", got.Accounting)
			}
			excluded := 0
			for _, f := range got.Accounting.Files {
				if f.Disposition == census.FileExcluded {
					excluded++
				}
			}
			if excluded != len(files)-len(selected) {
				t.Fatalf("excluded=%d", excluded)
			}
			if got.Complete {
				t.Fatal("empty symbol results claimed complete inventory")
			}
			if !reflect.DeepEqual(files, original) {
				t.Fatal("input mutated")
			}
		})
	}
}

func TestSelectedSourceFilesPreservesUnavailablePrecedenceAndPositions(t *testing.T) {
	workspace := t.TempDir()
	files := []SourceFile{testSource(workspace, "a.ts"), testSource(workspace, "b.py"), testSource(workspace, "c.cs"), testSource(workspace, "d.ts")}
	files[0].Disposition = census.FileUnreadable
	files[1].Disposition = census.FileUnsupported
	filters := Filters{Includes: []string{"d.ts"}, Excludes: []string{"a.ts", "b.py"}}
	selected, err := SelectedSourceFiles(workspace, files, filters)
	if err != nil || len(selected) != 1 || selected[0] != files[3] {
		t.Fatalf("%+v %v", selected, err)
	}
	p := lsp.Position{Line: 7, Character: 13}
	symbol := sym("Run", 12, p.Line, p.Character)
	client := &fakeDiscoveryClient{documentSupported: true, callSupported: true, symbols: map[string][]lsp.DocumentSymbol{files[3].URI: {symbol}}, prepare: map[lsp.Position][]lsp.CallHierarchyItem{p: {item("Run", files[3].URI, 12, p.Line, p.Character)}}}
	supplier := &fakeSupplier{}
	got, err := (DiscoveryAdapter{Workspace: workspace, Files: StaticFiles(files), Filters: filters, Supplier: supplier, Client: client, Limits: DiscoveryLimits{MaxFiles: 1}}).Discover(context.Background(), SessionIdentity{"controlled", 1})
	if err != nil {
		t.Fatal(err)
	}
	want := []census.FileDisposition{census.FileUnreadable, census.FileUnsupported, census.FileExcluded, census.FileSelected}
	for i, f := range got.Accounting.Files {
		if f.Disposition != want[i] || f.Ordinal != i {
			t.Fatalf("file %d: %+v", i, f)
		}
	}
	if got.Complete || got.Accounting.FileDenominator != 4 || len(got.ProviderFailures) != 2 {
		t.Fatalf("unavailable denominator laundered: %+v", got)
	}
	if !reflect.DeepEqual(client.prepareRequests, []lsp.Position{p}) || len(got.Targets) != 1 || got.Targets[0].Position() != p || got.EnumerationSymbols[0].FileOrdinal != 3 {
		t.Fatal("raw position or original file ordinal changed")
	}
	if !reflect.DeepEqual(supplier.calls, []string{"d.ts"}) {
		t.Fatal(supplier.calls)
	}
}

func TestSelectedSourceFilesRejectsInvalidExcludedIdentities(t *testing.T) {
	workspace := t.TempDir()
	valid := testSource(workspace, "a.go")
	for _, tc := range []struct {
		name    string
		files   []SourceFile
		filters Filters
	}{
		{"pattern", []SourceFile{valid}, Filters{Includes: []string{"["}}},
		{"duplicate", []SourceFile{valid, valid}, Filters{Excludes: []string{"**"}}},
		{"traversal", []SourceFile{testSource(workspace, "../escape.go")}, Filters{Excludes: []string{"**"}}},
		{"backslash-path", []SourceFile{testSource(workspace, `dir\a.go`)}, Filters{Excludes: []string{"**"}}},
		{"uri-mismatch", []SourceFile{{Path: "b.go", URI: valid.URI}}, Filters{Excludes: []string{"**"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := SelectedSourceFiles(workspace, tc.files, tc.filters); err == nil {
				t.Fatal("invalid metadata admitted")
			}
			supplier := &fakeSupplier{}
			client := &fakeDiscoveryClient{}
			_, err := (DiscoveryAdapter{Workspace: workspace, Files: StaticFiles(tc.files), Filters: tc.filters, Supplier: supplier, Client: client}).Discover(context.Background(), SessionIdentity{"controlled", 1})
			if err == nil || len(supplier.calls) != 0 || len(client.documentRequests) != 0 {
				t.Fatal("invalid metadata had effects")
			}
		})
	}
}
