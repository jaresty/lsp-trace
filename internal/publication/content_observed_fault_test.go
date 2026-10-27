//go:build linux || darwin

package publication

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

type faultObservedFile struct {
	observedFile
	read     func([]byte) (int, error)
	stat     func() (os.FileInfo, error)
	closeErr error
	closed   *bool
}

func (f *faultObservedFile) Read(p []byte) (int, error) {
	if f.read != nil {
		return f.read(p)
	}
	return f.observedFile.Read(p)
}
func (f *faultObservedFile) Stat() (os.FileInfo, error) {
	if f.stat != nil {
		return f.stat()
	}
	return f.observedFile.Stat()
}
func (f *faultObservedFile) Close() error {
	*f.closed = true
	return errors.Join(f.observedFile.Close(), f.closeErr)
}

type foreignOwnerInfo struct{ os.FileInfo }

func (i foreignOwnerInfo) Sys() any { s := *i.FileInfo.Sys().(*syscall.Stat_t); s.Uid++; return &s }

func TestReadContentObservedFaults(t *testing.T) {
	sentinelRead := errors.New("injected read")
	sentinelClose := errors.New("injected close")
	for _, name := range []string{"read", "close", "read-close", "stat", "post-stat", "owner", "identity", "replacement", "cancel-read", "short-read", "growing-read", "open-with-handle-error"} {
		t.Run(name, func(t *testing.T) {
			r, id, _ := observedFixture(t)
			closed := false
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ops := contentReadOps{lstat: statObservedContent, open: func(r *Root, id string) (observedFile, error) {
				f, err := openObservedContent(r, id)
				if err != nil {
					return nil, err
				}
				w := &faultObservedFile{observedFile: f, closed: &closed}
				switch name {
				case "read", "read-close":
					w.read = func([]byte) (int, error) { return 0, sentinelRead }
				case "short-read":
					w.read = bytes.NewReader([]byte("short")).Read
				case "growing-read":
					w.read = bytes.NewReader(bytes.Repeat([]byte("x"), 101)).Read
				case "stat":
					w.stat = func() (os.FileInfo, error) { return nil, io.ErrUnexpectedEOF }
				case "post-stat":
					calls := 0
					w.stat = func() (os.FileInfo, error) {
						calls++
						if calls == 2 {
							return nil, io.ErrUnexpectedEOF
						}
						return f.Stat()
					}
				case "owner":
					w.stat = func() (os.FileInfo, error) { i, e := f.Stat(); return foreignOwnerInfo{i}, e }
				case "identity":
					other := filepath.Join(r.Path(), "other")
					if err := os.WriteFile(other, []byte("foreign"), 0600); err != nil {
						t.Fatal(err)
					}
					w.stat = func() (os.FileInfo, error) { return os.Stat(other) }
				case "replacement":
					p := filepath.Join(r.Path(), id)
					if err := os.Rename(p, p+".old"); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(p, []byte("new inode"), 0600); err != nil {
						t.Fatal(err)
					}
				case "cancel-read":
					w.read = func(p []byte) (int, error) { cancel(); return f.Read(p) }
				}
				if name == "close" || name == "read-close" {
					w.closeErr = sentinelClose
				}
				if name == "open-with-handle-error" {
					return w, sentinelRead
				}
				return w, nil
			}}
			raw, err := r.readContentObserved(ctx, id, 100, ops)
			if raw != nil || err == nil || !closed {
				t.Fatalf("fault leaked bytes/error/handle: %q %v closed=%v", raw, err, closed)
			}
			if (name == "read" || name == "read-close") && !errors.Is(err, sentinelRead) {
				t.Fatal("read cause lost", err)
			}
			if (name == "close" || name == "read-close") && !errors.Is(err, sentinelClose) {
				t.Fatal("close cause lost", err)
			}
			if name == "cancel-read" && !errors.Is(err, context.Canceled) {
				t.Fatal("cancel cause lost", err)
			}
		})
	}
}
func TestReadContentObservedLateSymlink(t *testing.T) {
	r, id, raw := observedFixture(t)
	ops := contentReadOps{lstat: statObservedContent, open: func(r *Root, id string) (observedFile, error) {
		p := filepath.Join(r.Path(), id)
		if err := os.Rename(p, p+".real"); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(id+".real", p); err != nil {
			t.Fatal(err)
		}
		return openObservedContent(r, id)
	}}
	got, err := r.readContentObserved(context.Background(), id, int64(len(raw)), ops)
	if err == nil || got != nil {
		t.Fatal("followed late symlink", err)
	}
}
func TestReadContentObservedPinnedRoot(t *testing.T) {
	r, id, want := observedFixture(t)
	old := r.Path()
	moved := old + "-moved"
	if err := os.Rename(old, moved); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(moved); err != nil {
			t.Error(err)
		}
	})
	if err := os.Mkdir(old, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, id), []byte("substitution"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := r.ReadContentObserved(context.Background(), id, 100)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("not pinned: %q %v", got, err)
	}
}
func TestReadContentObservedPrecancelNoOps(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ops := contentReadOps{lstat: func(*Root, string) (os.FileInfo, error) { t.Fatal("read after precancel"); return nil, nil }}
	var r *Root
	got, err := r.readContentObserved(ctx, "sha256:"+fmtZeros(), 1, ops)
	if got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal(got, err)
	}
}
func fmtZeros() string { return "0000000000000000000000000000000000000000000000000000000000000000" }
