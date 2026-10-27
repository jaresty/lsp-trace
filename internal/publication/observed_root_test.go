//go:build linux || darwin

package publication

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func observedRootFixture(t *testing.T) (string, string, []byte) {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	home, err = filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatal(err)
	}
	base, err := os.MkdirTemp(home, ".lsp-trace-observed-root-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(base); err != nil {
			t.Error(err)
		}
	})
	path := filepath.Join(base, "ancestor", "root")
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	raw := []byte("descriptor-admitted retained bytes")
	id := fmt.Sprintf("sha256:%x", sha256.Sum256(raw))
	if err := os.WriteFile(filepath.Join(path, id), raw, 0600); err != nil {
		t.Fatal(err)
	}
	return path, id, raw
}

type observedAcquisitionLog struct {
	files        []*os.File
	handles      []*os.Root
	fileCloses   map[*os.File]int
	handleCloses map[*os.Root]int
}

func trackingObservedRootOps(t *testing.T) (observedRootOps, *observedAcquisitionLog) {
	t.Helper()
	ops := defaultObservedRootOps()
	log := &observedAcquisitionLog{fileCloses: map[*os.File]int{}, handleCloses: map[*os.Root]int{}}
	openDir := ops.openDir
	openHandle := ops.openHandle
	ops.openDir = func(p *os.File, n string) (*os.File, error) {
		f, e := openDir(p, n)
		if f != nil {
			log.files = append(log.files, f)
		}
		return f, e
	}
	ops.openHandle = func(p string) (*os.Root, error) {
		h, e := openHandle(p)
		if h != nil {
			log.handles = append(log.handles, h)
		}
		return h, e
	}
	ops.closeFile = func(f *os.File) error { log.fileCloses[f]++; return f.Close() }
	ops.closeHandle = func(h *os.Root) error { log.handleCloses[h]++; return h.Close() }
	// Fallback is test-only leak containment if a regression fails an assertion;
	// assertions inspect counts and closed handles before this cleanup executes.
	t.Cleanup(func() {
		for _, h := range log.handles {
			if log.handleCloses[h] == 0 {
				h.Close()
			}
		}
		for _, f := range log.files {
			if log.fileCloses[f] == 0 {
				f.Close()
			}
		}
	})
	return ops, log
}
func (l *observedAcquisitionLog) assertClosed(t *testing.T) {
	t.Helper()
	for _, f := range l.files {
		if l.fileCloses[f] != 1 {
			t.Errorf("directory close count %q: %d", f.Name(), l.fileCloses[f])
		}
		if _, e := f.Stat(); !errors.Is(e, os.ErrClosed) {
			t.Errorf("directory still open %q: %v", f.Name(), e)
		}
	}
	for _, h := range l.handles {
		if l.handleCloses[h] != 1 {
			t.Errorf("secondary-root close count: %d", l.handleCloses[h])
		}
		if _, e := h.Stat("."); e == nil {
			t.Error("secondary-root still open")
		}
	}
}
func TestObservedRootValidAndPinnedRead(t *testing.T) {
	path, id, want := observedRootFixture(t)
	ops, log := trackingObservedRootOps(t)
	root, err := openObservedRoot(context.Background(), path, ops)
	if err != nil {
		t.Fatal(err)
	}
	if err := root.ValidatePrivate(); err != nil {
		t.Fatal(err)
	}
	// Rename only AFTER successful descriptor admission. A replacement pathname
	// must not become the source of content reads.
	moved := path + "-moved"
	if err := os.Rename(path, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, id), []byte("unauthorized replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := root.ReadContentObserved(context.Background(), id, 100)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("pinned read: %q %v", got, err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	if err := root.Close(); err != nil {
		t.Fatal("second close", err)
	}
	if err := root.ValidatePrivate(); err == nil {
		t.Fatal("closed root remained admitted")
	}
	log.assertClosed(t)
}
func TestObservedRootEarlyPolicy(t *testing.T) {
	path, _, _ := observedRootFixture(t)
	for _, name := range []string{"nil-context", "cancel", "unclean", "relative", "empty", "private-tmp", "private-tmp-child"} {
		t.Run(name, func(t *testing.T) {
			ops, log := trackingObservedRootOps(t)
			ctx := context.Background()
			p := path
			switch name {
			case "nil-context":
				ctx = nil
			case "cancel":
				c, cancel := context.WithCancel(ctx)
				cancel()
				ctx = c
			case "unclean":
				p += "/../root"
			case "relative":
				p = "relative"
			case "empty":
				p = ""
			case "private-tmp":
				p = "/private/tmp"
			case "private-tmp-child":
				p = "/private/tmp/child"
			}
			r, e := openObservedRoot(ctx, p, ops)
			if e == nil || r != nil {
				t.Fatal("unsafe root admitted", e)
			}
			if len(log.files) != 0 || len(log.handles) != 0 {
				t.Fatal("policy rejection opened descriptors")
			}
		})
	}
}
func TestObservedRootRejectsFilesystemPolicy(t *testing.T) {
	for _, name := range []string{"ancestor-symlink", "root-symlink", "nonprivate", "foreign-owner", "regular-file"} {
		t.Run(name, func(t *testing.T) {
			path, id, _ := observedRootFixture(t)
			ops, log := trackingObservedRootOps(t)
			switch name {
			case "ancestor-symlink":
				parent := filepath.Dir(path)
				if e := os.Rename(parent, parent+"-saved"); e != nil {
					t.Fatal(e)
				}
				if e := os.Symlink(parent+"-saved", parent); e != nil {
					t.Fatal(e)
				}
			case "root-symlink":
				if e := os.Rename(path, path+"-saved"); e != nil {
					t.Fatal(e)
				}
				if e := os.Symlink(path+"-saved", path); e != nil {
					t.Fatal(e)
				}
			case "nonprivate":
				if e := os.Chmod(path, 0755); e != nil {
					t.Fatal(e)
				}
			case "foreign-owner":
				stat := ops.statFile
				ops.statFile = func(f *os.File) (os.FileInfo, error) { i, e := stat(f); return foreignOwnerInfo{i}, e }
			case "regular-file":
				path = filepath.Join(path, id)
			}
			r, e := openObservedRoot(context.Background(), path, ops)
			if e == nil || r != nil {
				t.Fatal("unsafe filesystem root admitted", e)
			}
			log.assertClosed(t)
		})
	}
}
func TestObservedRootAcquisitionFailuresCloseAll(t *testing.T) {
	fail := errors.New("acquisition fault")
	fileCloseFail := errors.New("directory close fault")
	handleCloseFail := errors.New("secondary close fault")
	for _, name := range []string{"initial-open", "component-open", "intermediate-close", "stat", "handle-open", "handle-stat", "cancel-traversal", "cancel-secondary", "cleanup-errors"} {
		t.Run(name, func(t *testing.T) {
			path, _, _ := observedRootFixture(t)
			ops, log := trackingObservedRootOps(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch name {
			case "initial-open", "component-open", "cancel-traversal":
				open := ops.openDir
				ops.openDir = func(p *os.File, n string) (*os.File, error) {
					f, e := open(p, n)
					if e != nil {
						return f, e
					}
					if name == "initial-open" && p == nil || name == "component-open" && n == "ancestor" {
						return f, fail
					}
					if name == "cancel-traversal" && n == "ancestor" {
						cancel()
					}
					return f, nil
				}
			case "intermediate-close":
				close := ops.closeFile
				ops.closeFile = func(f *os.File) error {
					e := close(f)
					if f.Name() == "ancestor" {
						e = errors.Join(e, fileCloseFail)
					}
					return e
				}
			case "stat":
				ops.statFile = func(*os.File) (os.FileInfo, error) { return nil, fail }
			case "handle-open", "cancel-secondary":
				open := ops.openHandle
				ops.openHandle = func(p string) (*os.Root, error) {
					h, e := open(p)
					if name == "handle-open" {
						return h, errors.Join(e, fail)
					}
					cancel()
					return h, e
				}
			case "handle-stat":
				ops.statHandle = func(*os.Root) (os.FileInfo, error) { return nil, fail }
			case "cleanup-errors":
				ops.statHandle = func(*os.Root) (os.FileInfo, error) { return nil, fail }
				closeFile := ops.closeFile
				ops.closeFile = func(f *os.File) error {
					e := closeFile(f)
					if f.Name() == "root" {
						return errors.Join(e, fileCloseFail)
					}
					return e
				}
				closeHandle := ops.closeHandle
				ops.closeHandle = func(h *os.Root) error { return errors.Join(closeHandle(h), handleCloseFail) }
			}
			r, e := openObservedRoot(ctx, path, ops)
			if r != nil || e == nil {
				t.Fatal("acquisition failure admitted root", e)
			}
			if name == "cleanup-errors" && (!errors.Is(e, fail) || !errors.Is(e, fileCloseFail) || !errors.Is(e, handleCloseFail)) {
				t.Fatal("cleanup causes lost", e)
			}
			if name == "intermediate-close" && !errors.Is(e, fileCloseFail) {
				t.Fatal("intermediate close cause lost", e)
			}
			if (name == "cancel-traversal" || name == "cancel-secondary") && !errors.Is(e, context.Canceled) {
				t.Fatal("cancel cause lost", e)
			}
			log.assertClosed(t)
		})
	}
}
func TestObservedRootCloseJoinsBothErrors(t *testing.T) {
	path, _, _ := observedRootFixture(t)
	ops, log := trackingObservedRootOps(t)
	fileFail := errors.New("file close")
	handleFail := errors.New("handle close")
	closeFile := ops.closeFile
	ops.closeFile = func(f *os.File) error {
		e := closeFile(f)
		if f.Name() == "root" {
			return errors.Join(e, fileFail)
		}
		return e
	}
	closeHandle := ops.closeHandle
	ops.closeHandle = func(h *os.Root) error { return errors.Join(closeHandle(h), handleFail) }
	r, e := openObservedRoot(context.Background(), path, ops)
	if e != nil {
		t.Fatal(e)
	}
	e = r.Close()
	if !errors.Is(e, fileFail) || !errors.Is(e, handleFail) {
		t.Fatal("root close causes lost", e)
	}
	log.assertClosed(t)
}
func TestObservedRootSecondaryCapabilityMustMatch(t *testing.T) {
	path, _, _ := observedRootFixture(t)
	ops, log := trackingObservedRootOps(t)
	open := ops.openHandle
	ops.openHandle = func(p string) (*os.Root, error) {
		if e := os.Rename(path, path+"-admitted"); e != nil {
			t.Fatal(e)
		}
		if e := os.Mkdir(path, 0700); e != nil {
			t.Fatal(e)
		}
		return open(p)
	}
	r, e := openObservedRoot(context.Background(), path, ops)
	if len(log.handles) != 1 {
		t.Fatal("did not acquire distinct secondary capability")
	}
	if e == nil || r != nil {
		if r != nil {
			r.Close()
		}
		t.Fatal("different secondary identity admitted")
	}
	log.assertClosed(t)
}
