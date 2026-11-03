//go:build darwin

package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	product "lsp-trace/internal/privatehostproductv3"
)

func selectionRepairCompare(t *testing.T, includes, excludes, prunes []string, insensitive, hardlinks bool) {
	t.Helper()
	r, p, _ := hostApplicationCLIFixture(t)
	for _, name := range []string{"a/cache/x.go", "cache/x.go", "a/foo/deep/x.go", "foo/deep/x.go", "b/Case/F.go", "unselected/cache/x.go", "a/links/one.go"} {
		f := filepath.Join(r.Configuration.SourceRoot, name)
		if e := os.MkdirAll(filepath.Dir(f), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(f, []byte("package owned\n"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	if insensitive {
		a, e := os.Stat(filepath.Join(r.Configuration.SourceRoot, "b/Case/F.go"))
		if e != nil {
			t.Fatal(e)
		}
		b, e := os.Stat(filepath.Join(r.Configuration.SourceRoot, "b/Case/f.go"))
		if e != nil || !os.SameFile(a, b) {
			t.Skip("owned filesystem is not case-insensitive; platform case not executed")
		}
		t.Log("CASE_INSENSITIVE_OWNED_FIXTURE_CONFIRMED")
	}
	if hardlinks {
		if e := os.Link(filepath.Join(r.Configuration.SourceRoot, "a/links/one.go"), filepath.Join(r.Configuration.SourceRoot, "a/links/two.go")); e != nil {
			t.Fatal(e)
		}
	}
	args := []string{}
	for n := 0; n < len(r.Template.CensusArguments); n++ {
		if r.Template.CensusArguments[n] == "--include" {
			n++
			continue
		}
		args = append(args, r.Template.CensusArguments[n])
	}
	for _, v := range includes {
		args = append(args, "--include", v)
	}
	for _, v := range excludes {
		args = append(args, "--exclude", v)
	}
	for _, v := range prunes {
		args = append(args, "--prune-dir", v)
	}
	r.Template.CensusArguments = args
	o, e := parseCensusCLIOptions(args)
	if e != nil {
		t.Fatal(e)
	}
	native, e := enumerateCensusPreflightContextPrivate(context.Background(), o)
	if e != nil {
		t.Fatal(e)
	}
	expected := []string{}
	for _, f := range native {
		if matchesAnyDiscoveryPattern(o.Includes, f.Path) && !matchesAnyDiscoveryPattern(o.Excludes, f.Path) {
			expected = append(expected, f.Path)
		}
	}
	r, report := readinessDiscover(t, r, p)
	a, e := product.OpenApplication(r.Configuration, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	var s applicationState
	if e = a.GetRecord(a.Head().CursorSHA256, &s); e != nil {
		t.Fatal(e)
	}
	actual := []string{}
	for id := s.Selected; id != ""; {
		var entry applicationCatalogEntry
		if e = a.GetRecord(id, &entry); e != nil {
			t.Fatal(e)
		}
		actual = append(actual, entry.Path)
		id = entry.Next
	}
	sort.Strings(actual)
	sort.Strings(expected)
	if !reflect.DeepEqual(actual, expected) || report.Selected != uint64(len(expected)) {
		t.Fatalf("ASSERT_SELECTION_NATIVE_PATHS A-fail native=%v actual=%v selected=%d", expected, actual, report.Selected)
	}
	if hardlinks && len(actual) != 2 {
		t.Fatal("hardlink fixture must exercise two real names")
	}
	t.Logf("ASSERT_SELECTION_NATIVE_PATHS A-pass native=%v actual=%v literal=%+v", expected, actual, report.LiteralDiscovery)
}

func TestHostApplicationSelectionRepairPrune(t *testing.T) {
	for _, tc := range []struct {
		name             string
		includes, prunes []string
	}{
		{"literal-nested-basename", []string{"a/cache/x.go"}, []string{"cache"}},
		{"fallback-nested-basename", []string{"**/x.go"}, []string{"cache", "foo", "unselected"}},
		{"literal-deep-prefix", []string{"a/cache/x.go", "cache/x.go", "a/foo/deep/x.go", "foo/deep/x.go"}, []string{"cache", "foo", "unselected"}},
		{"fallback-deep-prefix", []string{"**/x.go"}, []string{"a/foo", "cache", "unselected"}},
	} {
		t.Run(tc.name, func(t *testing.T) { selectionRepairCompare(t, tc.includes, nil, tc.prunes, false, false) })
	}
}
func TestHostApplicationSelectionRepairCase(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		includes, excludes, prunes []string
	}{
		{"file-alias", []string{"b/Case/f.go"}, nil, nil},
		{"both-file-spellings", []string{"b/Case/F.go", "b/Case/f.go"}, nil, nil},
		{"directory-alias", []string{"b/case/F.go"}, nil, nil},
		{"directory-prune", []string{"b/case/F.go", "b/Case/F.go"}, nil, []string{"b/Case"}},
		{"directory-exclude", []string{"b/case/F.go", "b/Case/F.go"}, []string{"b/Case/**"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { selectionRepairCompare(t, tc.includes, tc.excludes, tc.prunes, true, false) })
	}
}

// Same source is grouped and frozen by the real controlled orchestration, not
// merely by a matcher surrogate. Native Requested must remain twenty, not zero.
func TestHostApplicationSelectionRepairCombinedNativePrune(t *testing.T) {
	for _, mode := range []string{"literal", "fallback"} {
		t.Run(mode, func(t *testing.T) {
			r, p, base := compatibilityOneFile(t)
			selected := "a/cache/twenty.go"
			if e := os.MkdirAll(filepath.Join(r.Configuration.SourceRoot, "a/cache"), 0700); e != nil {
				t.Fatal(e)
			}
			if e := os.Rename(filepath.Join(r.Configuration.SourceRoot, "twenty.go"), filepath.Join(r.Configuration.SourceRoot, selected)); e != nil {
				t.Fatal(e)
			}
			for n, arg := range r.Template.CensusArguments {
				if arg == "--include" {
					r.Template.CensusArguments[n+1] = selected
					if mode == "fallback" {
						r.Template.CensusArguments[n+1] = "**/twenty.go"
					}
				}
			}
			r.Template.CensusArguments = append(r.Template.CensusArguments, "--prune-dir", "cache")
			r.MaxUnitsPerRun = 1
			readinessCombinedTwenty(t, r, p, base, selected)
		})
	}
}

func TestHostApplicationSelectionRepairFD(t *testing.T) {
	root, e := os.OpenRoot(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer root.Close()
	f, e := root.OpenFile(".", applicationLiteralOpenFlags, 0)
	if e != nil {
		t.Fatal("open", e)
	}
	defer f.Close()
	p, e := applicationLiteralFDPath(f)
	if e != nil {
		t.Fatal("FD path", e)
	}
	t.Log("observed", p)
}

func TestHostApplicationSelectionRepairHardlinks(t *testing.T) {
	selectionRepairCompare(t, []string{"a/links/one.go", "a/links/two.go"}, nil, nil, false, true)
}
