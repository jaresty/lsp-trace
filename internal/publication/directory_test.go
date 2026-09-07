package publication

import (
	"os"
	"runtime"
	"testing"
)

func TestDirectorySyncResultNotFileSync(t *testing.T) {
	root, err := OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	// Publish succeeds after syncing the temporary FILE. Deliberately close only
	// the pinned DIRECTORY descriptor to force a distinct directory-sync failure.
	if got := NewPublisher().Publish(Request{Root: root, Selector: "artifact.json", Bytes: []byte("bytes"), ArtifactSchemaID: "fixture"}); got.Err() != nil {
		t.Fatal(got.Err())
	}
	checked, err := root.SyncDirectory()
	if err != nil || checked != (runtime.GOOS != "windows") {
		t.Fatalf("actual directory result: %v %v", checked, err)
	}
	if err := root.file.Close(); err != nil {
		t.Fatal(err)
	}
	if checked, err := syncDirectory(root.file, "linux"); checked || err == nil {
		t.Fatalf("file sync masked directory failure: %v %v", checked, err)
	}
	if checked, err := syncDirectory(root.file, "windows"); checked || err != nil {
		t.Fatalf("unsupported policy: %v %v", checked, err)
	}
	root.file = nil
}

func TestDirectorySyncUsesPinnedRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows directory sync unavailable")
	}
	path := t.TempDir() + "/parent"
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := os.Rename(path, path+"-moved"); err != nil {
		t.Fatal(err)
	}
	if checked, err := root.SyncDirectory(); !checked || err != nil {
		t.Fatalf("pinned directory: %v %v", checked, err)
	}
}
