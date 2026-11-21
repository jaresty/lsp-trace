package scopejob

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/source"
)

func TestPreparedSelectionOperationCount(t *testing.T) {
	c, _, _ := fixture(t)
	c.Includes = []string{"*.go"}
	c.Excludes = []string{"*_test.go"}
	for _, name := range []string{"a.go", "b.go", "c.go", "d_test.go"} {
		file(t, c.Workspace, name)
	}
	c, err := Canonical(c)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	original := prepareSourceSelection
	prepareSourceSelection = func(w string, f censusacquisition.Filters) (*censusacquisition.PreparedSourceSelection, error) {
		calls++
		return original(w, f)
	}
	defer func() { prepareSourceSelection = original }()
	cur, err := OpenEnumeration(c)
	if err != nil {
		t.Fatal(err)
	}
	defer cur.Close()
	// Mutation before lazy initialization must not affect the live operation.
	c.Includes[0], c.Excludes[0] = "[", "**"
	b := cur.Batch(context.Background(), 128)
	if !b.Done || b.Reason != "" {
		t.Fatalf("frozen operation changed: %+v", b)
	}
	if calls != 1 {
		t.Fatalf("live preparation count = %d, want 1", calls)
	}
	c.Includes[0], c.Excludes[0] = "*.go", "*_test.go"
	calls = 0
	var replay EnumerationReplay
	if err := replay.Apply(context.Background(), c, b, 128); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("replay preparation count = %d, want 1", calls)
	}
	if replay.Counts().Selected != 3 || replay.Counts().Excluded != 1 {
		t.Fatal(replay.Counts())
	}
	t.Log("live and replay each prepared exactly once for four eligible records")
}

func TestPreparedSelectionEligibilityAndIdentity(t *testing.T) {
	c, _, _ := fixture(t)
	for _, filters := range []struct{ include, exclude []string }{{nil, nil}, {nil, []string{"**"}}, {[]string{"["}, []string{"**"}}} {
		c.Includes, c.Excludes = filters.include, filters.exclude
		var prepared regularSelector
		for _, name := range []string{"notes.txt", "other.ts", "a.go", "a:b.go", "dir/a:b.go", "../escape.go", "dir/ü.go", "dir/a?.go", "dir/a#.go", "dir/a%20.go", "dir/\xff.go", "dir/\x00.go"} {
			role, language, err := prepared.selectRegular(c, name)
			wantRole, wantErr := legacyRegularForTest(c, name)
			if role != wantRole || language != "" || fmt.Sprint(err) != fmt.Sprint(wantErr) {
				t.Fatalf("%q: %q %v / %q %v", name, role, err, wantRole, wantErr)
			}
		}
	}
}

func legacyRegularForTest(c Config, path string) (string, error) {
	language := source.LanguageID(path)
	if language == "" {
		return "NOT_SOURCE", nil
	}
	if language != c.Language {
		return "UNSUPPORTED_LANGUAGE", nil
	}
	uri, err := source.FileURI(filepath.Join(c.Workspace, path))
	if err != nil {
		return "", err
	}
	selected, err := censusacquisition.SelectedSourceFiles(c.Workspace, []censusacquisition.SourceFile{{Path: path, URI: uri, LanguageID: language}}, censusacquisition.Filters{Includes: c.Includes, Excludes: c.Excludes})
	if err != nil {
		return "", err
	}
	if len(selected) == 1 {
		return "SUPPORTED_UNACQUIRED", nil
	}
	return "EXCLUDED", nil
}
