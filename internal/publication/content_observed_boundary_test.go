//go:build linux || darwin

package publication

import (
	"context"
	"math"
	"os"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestReadContentObservedRejectsBeforeOps(t *testing.T) {
	r, id, _ := observedFixture(t)
	ops := contentReadOps{lstat: func(*Root, string) (os.FileInfo, error) {
		t.Fatal("invalid selector reached filesystem")
		return nil, nil
	}, open: func(*Root, string) (observedFile, error) { t.Fatal("invalid selector opened"); return nil, nil }}
	for _, s := range []string{"", id + "/x", "./" + id, "/" + id, "../" + id, id + "\n", strings.ToUpper(id), "sha256:" + strings.Repeat("a", 63), "sha256:" + strings.Repeat("g", 64), id + "\x00"} {
		if b, e := r.readContentObserved(context.Background(), s, 100, ops); e == nil || b != nil {
			t.Fatalf("selector admitted %q", s)
		}
	}
	for _, n := range []int64{-1, 0, math.MaxInt64} {
		if b, e := r.readContentObserved(context.Background(), id, n, ops); e == nil || b != nil {
			t.Fatal("limit admitted", n)
		}
	}
}
func TestReadContentObservedLateFIFO(t *testing.T) {
	r, id, _ := observedFixture(t)
	ops := contentReadOps{lstat: statObservedContent, open: func(r *Root, id string) (observedFile, error) {
		// Replace after a valid regular-file lstat. Nonblocking no-follow open must
		// return a descriptor that is rejected by fstat, rather than wait for a writer.
		if err := r.handle.Remove(id); err != nil {
			t.Fatal(err)
		}
		if err := unix.Mkfifo(r.Path()+"/"+id, 0600); err != nil {
			t.Fatal(err)
		}
		return openObservedContent(r, id)
	}}
	if b, e := r.readContentObserved(context.Background(), id, 100, ops); e == nil || b != nil {
		t.Fatal("FIFO admitted")
	}
}
func TestReadContentObservedCancellationBetweenStats(t *testing.T) {
	r, id, _ := observedFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	closed := false
	lookups := 0
	ops := contentReadOps{lstat: func(r *Root, id string) (os.FileInfo, error) {
		lookups++
		if lookups > 1 {
			t.Fatal("namespace read after observed cancellation")
		}
		return statObservedContent(r, id)
	}, open: func(r *Root, id string) (observedFile, error) {
		f, e := openObservedContent(r, id)
		if e != nil {
			return nil, e
		}
		calls := 0
		return &faultObservedFile{observedFile: f, closed: &closed, stat: func() (os.FileInfo, error) {
			calls++
			if calls == 2 {
				cancel()
			}
			return f.Stat()
		}}, nil
	}}
	b, e := r.readContentObserved(ctx, id, 100, ops)
	if b != nil || e == nil || !closed {
		t.Fatal("cancellation or close lost", e)
	}
}

func TestReadContentObservedRootPermissions(t *testing.T) {
	r, id, _ := observedFixture(t)
	if err := os.Chmod(r.Path(), 0755); err != nil {
		t.Fatal(err)
	}
	if b, e := r.ReadContentObserved(context.Background(), id, 100); e == nil || b != nil {
		t.Fatal("open root admitted")
	}
}
