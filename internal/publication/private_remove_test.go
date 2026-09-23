package publication

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func privateRootForRemove(t *testing.T) (*Root, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "owner")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return root, path
}

func TestPrivateRemovalSharesStablePublicationLockKey(t *testing.T) {
	root, path := privateRootForRemove(t)
	selector := "retained.json"
	if err := os.WriteFile(filepath.Join(path, selector), []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	pubTarget, err := capabilityTarget(root, selector)
	if err != nil {
		t.Fatal(err)
	}
	defer pubTarget.close()
	before, err := root.handle.Stat(".")
	if err != nil {
		t.Fatal(err)
	}
	if pubTarget.key != targetLockKey(before, selector) {
		t.Fatal("publication and removal do not share a selector lock")
	}
	if err := os.WriteFile(filepath.Join(path, "changed-directory-metadata"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	after, err := root.handle.Stat(".")
	if err != nil {
		t.Fatal(err)
	}
	if pubTarget.key != targetLockKey(after, selector) {
		t.Fatal("directory mutation changed publication/removal lock identity")
	}
	if other := targetLockKey(after, "other.json"); other == pubTarget.key {
		t.Fatal("different final selectors share a lock unexpectedly")
	}
}

func TestRemovePrivateFilePinnedRootAndExactSelector(t *testing.T) {
	root, path := privateRootForRemove(t)
	if err := os.Mkdir(filepath.Join(path, "objects"), 0o700); err != nil {
		t.Fatal(err)
	}
	selector := filepath.Join("objects", "retained.json")
	if err := os.WriteFile(filepath.Join(path, selector), []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := root.RemovePrivateFile(selector)
	if err != nil || !result.Removed || !result.DirectorySynced {
		t.Fatalf("private removal incomplete: %+v %v", result, err)
	}
	if _, err := root.ReadSelector(selector, 100); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("removed selector readable: %v", err)
	}
	if result, err := root.RemovePrivateFile(selector); err == nil || result.Removed {
		t.Fatalf("missing selector accepted: %+v %v", result, err)
	}
}

func TestRemovePrivateFileUsesPinnedDirectoryAfterPathReplacement(t *testing.T) {
	root, path := privateRootForRemove(t)
	if err := os.WriteFile(filepath.Join(path, "old"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	moved := path + "-moved"
	if err := os.Rename(path, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "old"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := root.RemovePrivateFile("old")
	if err != nil || !result.Removed || !result.DirectorySynced {
		t.Fatalf("pinned removal failed: %+v %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(moved, "old")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pinned file remains: %v", err)
	}
	if bytes, err := os.ReadFile(filepath.Join(path, "old")); err != nil || string(bytes) != "new" {
		t.Fatalf("replacement root changed: %q %v", bytes, err)
	}
}

func TestRemovePrivateFileRejectsUnsafeAndNonPrivateTargets(t *testing.T) {
	root, path := privateRootForRemove(t)
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(path, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(path, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(outside), filepath.Join(path, "sub-link")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "public"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	wide := filepath.Join(path, "wide")
	if err := os.Mkdir(wide, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(wide, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wide, "private"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, selector := range []string{"link", "sub-link/outside", "public", "sub", "wide/private", "../outside", outside, ""} {
		result, err := root.RemovePrivateFile(selector)
		if err == nil || result.Removed {
			t.Fatalf("unsafe selector %q accepted: %+v %v", selector, result, err)
		}
	}
	if bytes, err := os.ReadFile(outside); err != nil || string(bytes) != "keep" {
		t.Fatalf("outside file changed: %q %v", bytes, err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	if result, err := root.RemovePrivateFile("public"); err == nil || result.Removed {
		t.Fatalf("closed root accepted: %+v %v", result, err)
	}
}
