package publication

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateChildCleanupCannotUnlinkFile(t *testing.T) {
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	root, e := OpenRoot(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer root.Close()
	for _, name := range []string{"file", "link"} {
		if name == "file" {
			e = os.WriteFile(filepath.Join(dir, name), []byte("keep"), 0600)
		} else {
			e = os.Symlink("file", filepath.Join(dir, name))
		}
		if e != nil {
			t.Fatal(e)
		}
		if e = root.RemoveEmptyPrivateChild(name); e == nil {
			t.Fatalf("non-directory cleanup removed %s", name)
		}
		if _, e = os.Lstat(filepath.Join(dir, name)); e != nil {
			t.Fatal("cleanup deleted non-directory", e)
		}
	}
}

func TestPrivateChildCapability(t *testing.T) {
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	root, e := OpenRoot(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer root.Close()
	for _, name := range []string{"", ".", "..", "a/b", "a\\b", "bad\x00name"} {
		if child, e := root.OpenPrivateChild(name, true); e == nil {
			child.Close()
			t.Fatalf("unsafe child %q", name)
		}
	}
	child, e := root.OpenPrivateChild("job", true)
	if e != nil {
		t.Fatal(e)
	}
	if e = child.Close(); e != nil {
		t.Fatal(e)
	}
	if _, e = root.OpenPrivateChild("job", true); e == nil {
		t.Fatal("existing child adopted as fresh")
	}
	child, e = root.OpenPrivateChild("job", false)
	if e != nil {
		t.Fatal(e)
	}
	child.Close()
	if e = os.Symlink("job", filepath.Join(dir, "link")); e != nil {
		t.Fatal(e)
	}
	if _, e = root.OpenPrivateChild("link", false); e == nil {
		t.Fatal("symlink followed")
	}
	if e = os.Mkdir(filepath.Join(dir, "public"), 0755); e != nil {
		t.Fatal(e)
	}
	if _, e = root.OpenPrivateChild("public", false); e == nil {
		t.Fatal("public child accepted")
	}
	if e = root.RemoveEmptyPrivateChild("job"); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(dir, "job")); !os.IsNotExist(e) {
		t.Fatal("empty cleanup failed")
	}
	child, e = root.OpenPrivateChild("occupied", true)
	if e != nil {
		t.Fatal(e)
	}
	child.Close()
	if e = os.WriteFile(filepath.Join(dir, "occupied", "head"), nil, 0600); e != nil {
		t.Fatal(e)
	}
	if e = root.RemoveEmptyPrivateChild("occupied"); e == nil {
		t.Fatal("cleanup removed retained contents")
	}
}
