package censusacquisition

import (
	"fmt"
	"reflect"
	"testing"

	"lsp-trace/internal/census"
	"lsp-trace/internal/discoveryfilter"
)

// The pre-preparation implementation, retained as a controlled differential oracle.
func legacySelectionForTest(workspace string, files []SourceFile, filters Filters) ([]SourceFile, error) {
	if err := discoveryfilter.ValidatePatterns(filters.Includes, filters.Excludes); err != nil {
		return nil, err
	}
	canonical, err := canonicalDiscoveryFiles(workspace, files)
	if err != nil {
		return nil, err
	}
	selected := make([]SourceFile, 0, len(canonical))
	for _, file := range canonical {
		if file.Disposition == "" && selectPath(filters.Includes, filters.Excludes, file.Path) {
			selected = append(selected, file)
		}
	}
	return selected, nil
}

func TestPreparedSelectionDifferential(t *testing.T) {
	workspace := t.TempDir()
	filters := []Filters{{}, {Includes: []string{"**"}, Excludes: []string{"**"}}, {Includes: []string{"src/", "*.go"}, Excludes: []string{"**/*_test.go"}}, {Includes: []string{"["}, Excludes: []string{"**"}}, {Includes: []string{`src\**`}}, {Includes: []string{"/src/*.go"}}}
	names := []string{"a.go", "a_test.go", "src/a.go", "src/ü.go", "src/a:b.go", "a:b.go", "../a.go", "./a.go", "src//a.go", `src\a.go`, "src/a%20.go", "src/a?.go", "src/a#.go", "src/a\n.go", "src/\xff.go", "src/\x00.go"}
	for _, filter := range filters {
		for _, name := range names {
			for _, disposition := range []census.FileDisposition{"", census.FileUnreadable, census.FileUnsupported, "invalid"} {
				f := testSource(workspace, name)
				f.Disposition = disposition
				for _, mismatch := range []bool{false, true} {
					if mismatch {
						f.URI = testSource(workspace, "other.go").URI
					}
					old, oldErr := legacySelectionForTest(workspace, []SourceFile{f}, filter)
					current, err := SelectedSourceFiles(workspace, []SourceFile{f}, filter)
					if !reflect.DeepEqual(old, current) || fmt.Sprint(oldErr) != fmt.Sprint(err) {
						t.Fatalf("collection differential %q: %v / %v", name, oldErr, err)
					}
					prepared, pe := PrepareSourceSelection(workspace, filter)
					selected := false
					if pe == nil {
						selected, pe = prepared.Select(f)
					}
					if selected != (len(old) == 1) || fmt.Sprint(pe) != fmt.Sprint(oldErr) {
						t.Fatalf("prepared differential %q: %v %v / %v %v", name, selected, pe, old, oldErr)
					}
				}
			}
		}
	}
}

func TestPreparedSelectionSnapshotAndCollection(t *testing.T) {
	workspace := t.TempDir()
	filters := Filters{Includes: []string{"*.go"}, Excludes: []string{"*_test.go"}}
	p, err := PrepareSourceSelection(workspace, filters)
	if err != nil {
		t.Fatal(err)
	}
	filters.Includes[0], filters.Excludes[0] = "[", "**"
	if ok, err := p.Select(testSource(workspace, "a.go")); !ok || err != nil {
		t.Fatalf("caller mutation affected prepared state: %v %v", ok, err)
	}
	files := []SourceFile{testSource(workspace, "b.go"), testSource(workspace, "a.go")}
	original := append([]SourceFile(nil), files...)
	got, err := SelectedSourceFiles(workspace, files, Filters{})
	if err != nil || got[0].Path != "a.go" || !reflect.DeepEqual(files, original) {
		t.Fatal("collection ordering or isolation", got, err)
	}
	got[0].Path = "mutated"
	if !reflect.DeepEqual(files, original) {
		t.Fatal("result aliases caller")
	}
	if _, err := SelectedSourceFiles(workspace, []SourceFile{files[0], files[0]}, Filters{Excludes: []string{"**"}}); err == nil {
		t.Fatal("excluded duplicate accepted")
	}
	if _, err := (&PreparedSourceSelection{}).Select(files[0]); err == nil {
		t.Fatal("zero value trusted")
	}
}
