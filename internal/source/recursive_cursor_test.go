package source

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBoundedRecursiveCursor(t *testing.T) {
	base := t.TempDir()
	for _, d := range []string{"nested", "pruned"} {
		if err := os.Mkdir(filepath.Join(base, d), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"root.go", "nested/one.go", "nested/two.go", "pruned/leak.go"} {
		if err := os.WriteFile(filepath.Join(base, f), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(base, "alias")); err != nil {
		t.Fatal(err)
	}
	c, err := OpenRecursiveCursor(base, []string{"."}, []string{"pruned"}, Limits{MaxEntries: 32, MaxWork: 64, MaxPathBytes: 128, MaxDepth: 8})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx := context.Background()
	seen := make(map[string]int)
	for n := 1; n <= 10; n++ {
		batch, done, err := c.Next(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		files := 0
		for _, r := range batch {
			if _, ok := seen[r.Name]; ok {
				t.Fatalf("duplicate record %q", r.Name)
			}
			seen[r.Name] = n
			if r.Kind != SourceDirectory {
				files++
			}
		}
		if files > 1 {
			t.Fatalf("ASSERT_CURSOR_POSITIVE_BOUND: batch returned %d non-directory records", files)
		}
		if !done {
			continue
		}
		if seen["root.go"] == 0 || seen["nested/one.go"] == 0 || seen["nested/two.go"] == 0 || seen["nested/one.go"] == seen["nested/two.go"] || seen["pruned/leak.go"] != 0 || seen["alias"] == 0 {
			t.Fatalf("ASSERT_CURSOR_NESTED_READ: unexpected batch membership %v", seen)
		}
		if _, _, err := c.Next(ctx, 1); err == nil {
			t.Fatal("ASSERT_CURSOR_CLOSED: terminal cursor accepted a further read")
		}
		t.Log("ASSERT_CURSOR_POSITIVE_BOUND: PASS; ASSERT_CURSOR_NESTED_READ: PASS; ASSERT_CURSOR_CLOSED: PASS")
		return
	}
	t.Fatal("cursor did not terminate")
}
