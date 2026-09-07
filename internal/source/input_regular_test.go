package source

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInputRecorderRegularControls(t *testing.T) {
	r, root := recorderFixture(t)
	if err := os.Mkdir(filepath.Join(root, "directory"), 0700); err != nil {
		t.Fatal(err)
	}
	if b, id, err := r.ReadInput("directory", InputSource); err == nil || b != nil || id == "" {
		t.Fatalf("directory: %q %q %v", b, id, err)
	}
	if err := os.WriteFile(filepath.Join(root, "empty"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if b, id, err := r.ReadInput("empty", InputSource); err != nil || len(b) != 0 || id == "" {
		t.Fatalf("empty regular: %q %q %v", b, id, err)
	}
	if err := os.Symlink("input", filepath.Join(root, "inside")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if b, id, err := r.ReadInput("inside", InputSource); err != nil || string(b) != "original\r\n" || id == "" {
		t.Fatalf("contained symlink: %q %q %v", b, id, err)
	}
}
