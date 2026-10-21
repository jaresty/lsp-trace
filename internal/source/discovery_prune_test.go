package source

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDiscoverContextPrunesGloballyAcrossOverlappingRoots(t *testing.T) {
	base := t.TempDir()
	for _, name := range []string{"keep/a.go", "vendor/b.go", "vendor/nested/c.go"} {
		p := filepath.Join(base, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	limits := Limits{MaxEntries: 100, MaxAccepted: 100, MaxWork: 200, MaxPathBytes: 4096, MaxDepth: 64}
	for _, inputs := range [][]string{{".", "vendor"}, {"vendor", "."}} {
		got, err := DiscoverContext(context.Background(), Request{Base: base, Inputs: inputs, AllowDot: true, PrunedDirectories: []string{"vendor"}}, limits)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, r := range got {
			names = append(names, r.Name)
		}
		if reflect.DeepEqual(names, []string{}) || containsPrefix(names, "vendor") {
			t.Fatalf("ASSERT_DISCOVER_PRUNE_OVERLAP: inputs=%v names=%v", inputs, names)
		}
	}
}
func containsPrefix(values []string, prefix string) bool {
	for _, v := range values {
		if v == prefix || len(v) > len(prefix) && v[:len(prefix)+1] == prefix+"/" {
			return true
		}
	}
	return false
}

type syntheticFileEntry struct{ fs.FileInfo }

func (syntheticFileEntry) Name() string               { return "a.go" }
func (syntheticFileEntry) IsDir() bool                { return false }
func (syntheticFileEntry) Type() fs.FileMode          { return 0 }
func (syntheticFileEntry) Info() (fs.FileInfo, error) { return nil, nil }

func TestDiscoverContextSyntheticHighCallbackLimit(t *testing.T) {
	base := t.TempDir()
	child := filepath.Join(base, "a.go")
	if err := os.WriteFile(child, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	walk := func(_ string, fn fs.WalkDirFunc) error {
		for i := 0; i < 224000; i++ {
			if err := fn(child, syntheticFileEntry{}, nil); err != nil {
				return err
			}
		}
		return nil
	}
	_, err := discoverContext(context.Background(), Request{Base: base, Inputs: []string{"."}, AllowDot: true}, Limits{MaxEntries: 100000, MaxAccepted: 100000, MaxWork: 200000, MaxPathBytes: 4096, MaxDepth: 64}, walk)
	var limit *LimitError
	if !errors.As(err, &limit) || limit.Category != "MAX_WORK" || limit.Observed != 200001 || limit.Limit != 200000 {
		t.Fatalf("ASSERT_DISCOVER_SYNTHETIC_224K_LIMIT: %#v", err)
	}
}

func TestDiscoverContextTypedFirstRejectedLimits(t *testing.T) {
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "a.go"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := DiscoverContext(context.Background(), Request{Base: base, Inputs: []string{"."}, AllowDot: true}, Limits{MaxEntries: 1, MaxAccepted: 1, MaxWork: 100, MaxPathBytes: 4096, MaxDepth: 64})
	var limit *LimitError
	if !errors.As(err, &limit) || limit.Category != "MAX_ENTRIES" || limit.Observed != 2 || limit.Limit != 1 {
		t.Fatalf("ASSERT_DISCOVER_TYPED_LIMIT: %#v", err)
	}
}
