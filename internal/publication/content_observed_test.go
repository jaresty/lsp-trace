//go:build linux || darwin

package publication

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func observedFixture(t *testing.T) (*Root, string, []byte) {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	raw := []byte("actual retained bytes")
	id := fmt.Sprintf("sha256:%x", sha256.Sum256(raw))
	if err := os.WriteFile(filepath.Join(dir, id), raw, 0600); err != nil {
		t.Fatal(err)
	}
	root, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := root.ValidatePrivate(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	return root, id, raw
}
func TestReadContentObservedDisk(t *testing.T) {
	r, id, want := observedFixture(t)
	before, err := os.ReadDir(r.Path())
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.ReadContentObserved(context.Background(), id, int64(len(want)))
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("retained bytes: %q %v", got, err)
	}
	after, err := os.ReadDir(r.Path())
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != len(after) {
		t.Fatal("read wrote directory entries")
	}
	got[0] = 'X'
	again, err := r.ReadContentObserved(context.Background(), id, 100)
	if err != nil || !bytes.Equal(again, want) {
		t.Fatal("alias or disk mutation", err)
	}
}
func TestReadContentObservedReject(t *testing.T) {
	for _, name := range []string{"selector", "foreign", "symlink", "permissions", "hardlink", "directory", "oversize", "digest", "missing", "nil-context", "cancel", "deadline", "nil-root", "zero-limit"} {
		t.Run(name, func(t *testing.T) {
			r, id, raw := observedFixture(t)
			ctx := context.Background()
			limit := int64(100)
			p := filepath.Join(r.Path(), id)
			switch name {
			case "selector":
				id = "../" + id
			case "foreign":
				id = strings.ToUpper(id)
			case "symlink":
				if err := os.Remove(p); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("target", p); err != nil {
					t.Fatal(err)
				}
			case "permissions":
				if err := os.Chmod(p, 0644); err != nil {
					t.Fatal(err)
				}
			case "hardlink":
				if err := os.Link(p, filepath.Join(r.Path(), "alias")); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Remove(p); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(p, 0700); err != nil {
					t.Fatal(err)
				}
			case "oversize":
				limit = int64(len(raw) - 1)
			case "digest":
				if err := os.WriteFile(p, bytes.Repeat([]byte("x"), len(raw)), 0600); err != nil {
					t.Fatal(err)
				}
			case "missing":
				id = "sha256:" + strings.Repeat("0", 64)
			case "nil-context":
				ctx = nil
			case "cancel":
				c, cancel := context.WithCancel(ctx)
				cancel()
				ctx = c
			case "deadline":
				c, cancel := context.WithDeadline(ctx, time.Unix(0, 0))
				defer cancel()
				ctx = c
			case "nil-root":
				r = nil
			case "zero-limit":
				limit = 0
			}
			got, err := r.ReadContentObserved(ctx, id, limit)
			if err == nil || got != nil {
				t.Fatalf("unsafe input admitted: %q %v", got, err)
			}
		})
	}
}
