package adr0007sourcetextsearchv1

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDestructiveSymlinkRejectedByFreeze(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "target"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target", filepath.Join(dir, "link")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := Census(dir); err == nil {
		t.Fatal("expected symlink rejection")
	}
}

func TestDestructivePathTraversalRejected(t *testing.T) {
	in := Input{SchemaVersion: SchemaInputV1, Query: "a", Source: SourceBinding{Path: "../escape", Revision: "r", FileSHA256: "sha256:ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb", ObjectID: "o", AdmissionID: "a", Seal: "s"}, Limits: Limits{MaxMatches: 1, MaxOutputBytes: 1000, MaxWork: 1000, MaxPathBytes: 100, MaxSourceBytes: 1}}
	_, fail := Search(in, []byte("a"))
	if fail == nil || fail.Code != "SOURCE_BINDING_MISMATCH" {
		t.Fatalf("expected traversal failure: %#v", fail)
	}
}
