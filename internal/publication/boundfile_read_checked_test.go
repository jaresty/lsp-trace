package publication

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func checkedReadRoot(t *testing.T) (string, *Root) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	return dir, root
}
func checkedReadPublish(t *testing.T, root *Root, selector string, raw []byte) *BoundFileReceipt {
	t.Helper()
	r, err := PublishBoundFile(root, selector, raw, func(got []byte) error {
		if !bytes.Equal(raw, got) {
			return errors.New("fixture byte mismatch")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func checkedReadRootLive(t *testing.T, root *Root) {
	t.Helper()
	got, err := ReadVerifiedBoundFileChecked(root, "keep", 64)
	if err != nil || string(got) != "borrowed-root-live" {
		t.Fatal("ASSERT_CHECKED_READ_BORROWED_ROOT", err)
	}
}
func TestReadVerifiedBoundFileCheckedExact(t *testing.T) {
	_, root := checkedReadRoot(t)
	checkedReadPublish(t, root, "keep", []byte("borrowed-root-live"))
	for _, selector := range []string{"flat", "a/b/c/nested"} {
		t.Run(selector, func(t *testing.T) {
			raw := []byte("exact private bytes\x00\xff")
			receipt := checkedReadPublish(t, root, selector, raw)
			got, err := ReadVerifiedBoundFileChecked(root, selector, int64(len(raw)))
			if err != nil || !bytes.Equal(got, raw) || fmt.Sprintf("sha256:%x", sha256.Sum256(got)) != receipt.Digest {
				t.Fatal("ASSERT_CHECKED_READ_EXACT", err)
			}
			checkedReadRootLive(t, root)
			t.Log("ASSERT_CHECKED_READ_EXACT: PASS; ASSERT_CHECKED_READ_BORROWED_ROOT: PASS")
		})
	}
}
func TestReadVerifiedBoundFileCheckedRefusals(t *testing.T) {
	for _, fault := range []string{"zero-limit", "negative-limit", "overflow-limit", "over-limit", "missing", "missing-parent", "final-symlink", "parent-symlink", "nested-parent-symlink", "nonprivate-file", "hard-link", "directory", "escape", "absolute", "unnormalized", "nul"} {
		t.Run(fault, func(t *testing.T) {
			dir, root := checkedReadRoot(t)
			checkedReadPublish(t, root, "keep", []byte("borrowed-root-live"))
			checkedReadPublish(t, root, "a/b/file", []byte("bounded bytes"))
			selector, limit := "a/b/file", int64(64)
			switch fault {
			case "zero-limit":
				limit = 0
			case "negative-limit":
				limit = -1
			case "overflow-limit":
				limit = 1<<63 - 1
			case "over-limit":
				limit = 1
			case "missing":
				selector = "a/b/missing"
			case "missing-parent":
				selector = "a/b/missing/file"
			case "final-symlink":
				selector = "a/b/link"
				if err := os.Symlink("file", filepath.Join(dir, selector)); err != nil {
					t.Fatal(err)
				}
			case "parent-symlink":
				selector = "link/b/file"
				if err := os.Symlink("a", filepath.Join(dir, "link")); err != nil {
					t.Fatal(err)
				}
			case "nested-parent-symlink":
				selector = "a/link/file"
				if err := os.Symlink("b", filepath.Join(dir, "a/link")); err != nil {
					t.Fatal(err)
				}
			case "nonprivate-file":
				if err := os.Chmod(filepath.Join(dir, selector), 0644); err != nil {
					t.Fatal(err)
				}
			case "hard-link":
				if err := os.Link(filepath.Join(dir, selector), filepath.Join(dir, "other-link")); err != nil {
					t.Fatal(err)
				}
			case "directory":
				selector = "a/b"
			case "escape":
				selector = "../keep"
			case "absolute":
				selector = filepath.Join(dir, "keep")
			case "unnormalized":
				selector = "a/../keep"
			case "nul":
				selector = "a\x00/b"
			}
			got, err := ReadVerifiedBoundFileChecked(root, selector, limit)
			if err == nil || got != nil {
				t.Fatal("ASSERT_CHECKED_READ_REFUSAL", fault, err)
			}
			if fault == "over-limit" && !errors.Is(err, ErrBoundFileLimit) {
				t.Fatal("missing limit cause", err)
			}
			if fault == "missing" && !errors.Is(err, os.ErrNotExist) {
				t.Fatal("missing absence cause", err)
			}
			checkedReadRootLive(t, root)
			t.Log("ASSERT_CHECKED_READ_REFUSAL: PASS; ASSERT_CHECKED_READ_BORROWED_ROOT: PASS")
		})
	}
	if got, err := ReadVerifiedBoundFileChecked(nil, "x", 1); err == nil || got != nil {
		t.Fatal("nil root")
	}
}
func TestReadVerifiedBoundFileCheckedPinnedRoot(t *testing.T) {
	base := t.TempDir()
	dir, retained := filepath.Join(base, "root"), filepath.Join(base, "retained")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	checkedReadPublish(t, root, "a/b/exact", []byte("pinned bytes"))
	if err := os.Rename(dir, retained); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "a/b"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a/b/exact"), []byte("forged bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadVerifiedBoundFileChecked(root, "a/b/exact", 64)
	if err != nil || string(got) != "pinned bytes" {
		t.Fatal("ASSERT_CHECKED_READ_NO_PATH_REOPEN", err, string(got))
	}
	t.Log("ASSERT_CHECKED_READ_NO_PATH_REOPEN: PASS")
}

// Real closes are invoked, then deterministic errors are reported by private
// instance-local wrappers. This models a reported close failure, not an injected
// operating-system close failure. No wrapper is available to public callers.
type checkedReadFaultFile struct {
	*os.File
	readErr, closeErr, statErr error
	statFailAt, stats, closes  int
	order                      *[]string
}

func (f *checkedReadFaultFile) Read(p []byte) (int, error) {
	if f.readErr != nil {
		n, err := f.File.Read(p[:1])
		return n, errors.Join(err, f.readErr)
	}
	return f.File.Read(p)
}
func (f *checkedReadFaultFile) Stat() (os.FileInfo, error) {
	f.stats++
	if f.statFailAt == f.stats {
		return nil, f.statErr
	}
	return f.File.Stat()
}
func (f *checkedReadFaultFile) Close() error {
	f.closes++
	*f.order = append(*f.order, "data")
	return errors.Join(f.File.Close(), f.closeErr)
}

type checkedReadFaultParent struct {
	io.Closer
	failure error
	closes  int
	name    string
	order   *[]string
}

func (p *checkedReadFaultParent) Close() error {
	p.closes++
	*p.order = append(*p.order, p.name)
	return errors.Join(p.Closer.Close(), p.failure)
}
func TestReadVerifiedBoundFileCheckedParentWalkCleanup(t *testing.T) {
	_, root := checkedReadRoot(t)
	checkedReadPublish(t, root, "keep", []byte("borrowed-root-live"))
	checkedReadPublish(t, root, "a/b/c/file", []byte("contents"))
	order := []string{}
	h := &boundFileReadHandles{}
	current := root.file
	prior := errors.New("earlier parent-walk error")
	causes := []error{prior}
	var parents []*checkedReadFaultParent
	for i, part := range []string{"a", "b", "c"} {
		child, err := openObservedDirectory(current, part)
		if err != nil {
			t.Fatal(err)
		}
		p := &checkedReadFaultParent{Closer: child, failure: errors.New(part + " parent close"), name: fmt.Sprintf("parent%d", i), order: &order}
		t.Cleanup(func() {
			if p.closes == 0 {
				if err := child.Close(); err != nil {
					t.Error(err)
				}
			}
		})
		h.parents = append(h.parents, p)
		parents = append(parents, p)
		causes = append(causes, p.failure)
		current = child
	}
	var raw []byte
	err := prior
	h.finish(&raw, &err) // no data descriptor was acquired
	for _, cause := range causes {
		if !errors.Is(err, cause) {
			t.Fatal("ASSERT_CHECKED_READ_WALK_CLOSE_JOIN", cause, err)
		}
	}
	for _, p := range parents {
		if p.closes != 1 {
			t.Fatal("ASSERT_CHECKED_READ_RELEASE_ALL", p.name, p.closes)
		}
	}
	if raw != nil || !reflect.DeepEqual(order, []string{"parent2", "parent1", "parent0"}) {
		t.Fatal("ASSERT_CHECKED_READ_WALK_CLOSE_JOIN", order)
	}
	checkedReadRootLive(t, root)
	t.Log("ASSERT_CHECKED_READ_WALK_CLOSE_JOIN: PASS; ASSERT_CHECKED_READ_RELEASE_ALL: PASS; ASSERT_CHECKED_READ_BORROWED_ROOT: PASS")
}

func TestReadVerifiedBoundFileCheckedOwnedCloseFailures(t *testing.T) {
	for _, fault := range []string{"clean", "data", "parent0", "parent1", "parent2", "all", "read+all", "stat+all", "after-stat+all", "limit+all"} {
		t.Run(fault, func(t *testing.T) {
			_, root := checkedReadRoot(t)
			checkedReadPublish(t, root, "keep", []byte("borrowed-root-live"))
			checkedReadPublish(t, root, "a/b/c/file", []byte("contents"))
			order := []string{}
			current := root.file
			h := &boundFileReadHandles{}
			var parents []*checkedReadFaultParent
			var causes []error
			for i, part := range []string{"a", "b", "c"} {
				child, err := openObservedDirectory(current, part)
				if err != nil {
					t.Fatal(err)
				}
				p := &checkedReadFaultParent{Closer: child, name: fmt.Sprintf("parent%d", i), order: &order}
				t.Cleanup(func() {
					if p.closes == 0 {
						if err := child.Close(); err != nil {
							t.Error(err)
						}
					}
				})
				if fault == p.name || strings.Contains(fault, "all") {
					p.failure = errors.New(p.name + " close")
					causes = append(causes, p.failure)
				}
				parents = append(parents, p)
				h.parents = append(h.parents, p)
				current = child
			}
			opened, err := openObservedContent(&Root{file: current}, "file")
			if err != nil {
				t.Fatal(err)
			}
			file := opened.(*os.File) // supported platform primitive returns the actual FD
			f := &checkedReadFaultFile{File: file, order: &order}
			t.Cleanup(func() {
				if f.closes == 0 {
					if err := file.Close(); err != nil {
						t.Error(err)
					}
				}
			})
			before, err := file.Stat()
			if err != nil {
				t.Fatal(err)
			}
			if fault == "data" || strings.Contains(fault, "all") {
				f.closeErr = errors.New("data close")
				causes = append(causes, f.closeErr)
			}
			if fault == "read+all" {
				f.readErr = errors.New("earlier partial read")
				causes = append(causes, f.readErr)
			}
			if fault == "stat+all" || fault == "after-stat+all" {
				f.statFailAt = 1
				if fault == "after-stat+all" {
					f.statFailAt = 2
				}
				f.statErr = errors.New("earlier stat")
				causes = append(causes, f.statErr)
			}
			limit := int64(64)
			if fault == "limit+all" {
				limit = 1
				causes = append(causes, ErrBoundFileLimit)
			}
			h.data = f
			got, err := h.read(root.info, before, limit)
			if fault == "clean" {
				if err != nil || string(got) != "contents" {
					t.Fatal("clean reader", err)
				}
			} else {
				if err == nil || got != nil {
					t.Fatal("ASSERT_CHECKED_READ_CLOSE_JOIN", fault, string(got), err)
				}
				for _, cause := range causes {
					if !errors.Is(err, cause) {
						t.Fatal("ASSERT_CHECKED_READ_CLOSE_JOIN", fault, "lost cause", cause, err)
					}
				}
			}
			if f.closes != 1 {
				t.Fatal("ASSERT_CHECKED_READ_RELEASE_ALL", fault, f.closes)
			}
			for _, p := range parents {
				if p.closes != 1 {
					t.Fatal("ASSERT_CHECKED_READ_RELEASE_ALL", fault, p.name, p.closes)
				}
			}
			if !reflect.DeepEqual(order, []string{"data", "parent2", "parent1", "parent0"}) {
				t.Fatal("release order", order)
			}
			checkedReadRootLive(t, root)
			t.Log("ASSERT_CHECKED_READ_CLOSE_JOIN: PASS; ASSERT_CHECKED_READ_RELEASE_ALL: PASS; ASSERT_CHECKED_READ_BORROWED_ROOT: PASS", fault)
		})
	}
}
