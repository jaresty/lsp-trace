package source

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestDiscoverDeterministicBoundedRecords(t *testing.T) {
	base := t.TempDir()
	mustWrite := func(name string) {
		t.Helper()
		path := filepath.Join(base, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("z.go")
	mustWrite("src/b.go")
	mustWrite("src/a.go")

	forward, err := Discover(Request{Base: base, Inputs: []string{"z.go", "src"}})
	if err != nil {
		t.Fatal(err)
	}
	reverse, err := Discover(Request{Base: base, Inputs: []string{"src", "z.go"}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(forward, reverse) {
		t.Fatalf("ASSERT_EQUIVALENT_INPUT_SETS: forward=%+v reverse=%+v", forward, reverse)
	}
	if !sort.SliceIsSorted(forward, func(i, j int) bool { return recordKey(forward[i]) < recordKey(forward[j]) }) {
		t.Fatalf("ASSERT_DETERMINISTIC_ORDER: records=%+v", forward)
	}
	wantNames := []string{"src", "src/a.go", "src/b.go", "z.go"}
	gotNames := make([]string, len(forward))
	for i, record := range forward {
		gotNames[i] = record.Name
		if filepath.IsAbs(record.Name) || strings.Contains(record.Name, "\\") {
			t.Fatalf("ASSERT_STABLE_RELATIVE_NAMES: record=%+v", record)
		}
	}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("ASSERT_DIRECTORY_ENUMERATION: got=%v want=%v", gotNames, wantNames)
	}
	if forward[0].Kind != SourceDirectory || forward[0].Inclusion != Included || forward[1].Kind != SourceRegularFile {
		t.Fatalf("ASSERT_DOMAIN_NEUTRAL_CLASSIFICATION: records=%+v", forward)
	}
}

func TestDiscoverClassifiesBoundaryCases(t *testing.T) {
	base := t.TempDir()
	file := filepath.Join(base, "same.go")
	if err := os.WriteFile(file, []byte("package same\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(file, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	outside := filepath.Join(filepath.Dir(base), "outside.go")

	records, err := Discover(Request{Base: base, Inputs: []string{"same.go", "./same.go", "missing.go", "link", outside}})
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, record := range records {
		counts[record.Name]++
	}
	if counts["same.go"] != 1 {
		t.Fatalf("ASSERT_DUPLICATE_NORMALIZATION: records=%+v", records)
	}
	assertRecord(t, records, "missing.go", SourceOther, Unavailable, "ASSERT_UNAVAILABLE_EXPLICIT")
	assertRecord(t, records, "link", SourceOther, Unsupported, "ASSERT_UNSUPPORTED_EXPLICIT")
	outsideFound := false
	for _, record := range records {
		if record.Inclusion == OutsideBase {
			outsideFound = true
			if filepath.IsAbs(record.Name) || strings.Contains(record.Name, "..") {
				t.Fatalf("ASSERT_ESCAPE_NAME_CONTAINED: record=%+v", record)
			}
		}
	}
	if !outsideFound {
		t.Fatalf("ASSERT_ESCAPE_EXPLICIT: records=%+v", records)
	}
}

func TestDiscoverRejectsAmbiguousInputsAndNeedsNoGit(t *testing.T) {
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "plain.go"), []byte("package plain\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"", "."} {
		if _, err := Discover(Request{Base: base, Inputs: []string{input}}); !errors.Is(err, ErrAmbiguousRequest) {
			t.Fatalf("ASSERT_AMBIGUITY_EXPLICIT: input=%q err=%v", input, err)
		}
	}
	records, err := Discover(Request{Base: base, Inputs: []string{"plain.go"}})
	if err != nil || len(records) != 1 || records[0].Inclusion != Included {
		t.Fatalf("ASSERT_NO_GIT_REQUIRED: records=%+v err=%v", records, err)
	}
}

func assertRecord(t *testing.T, records []Record, name string, kind SourceKind, inclusion Inclusion, assertion string) {
	t.Helper()
	for _, record := range records {
		if record.Name == name && record.Kind == kind && record.Inclusion == inclusion {
			return
		}
	}
	t.Fatalf("%s: missing name=%q kind=%q inclusion=%q records=%+v", assertion, name, kind, inclusion, records)
}

func recordKey(record Record) string {
	return record.Name + "\x00" + string(record.Kind) + "\x00" + string(record.Inclusion) + "\x00" + record.Reason
}
