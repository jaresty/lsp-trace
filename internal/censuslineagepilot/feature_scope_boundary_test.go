//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFeatureScopeExclusionAndOwnedObservation(t *testing.T) {
	root := scopeFixture(t)
	// An unlisted source with an unsafe binding must not become a candidate or read.
	if e := os.Symlink("/nonexistent", filepath.Join(root, "cmd/lsp-trace/excluded.go")); e != nil {
		t.Fatal(e)
	}
	b, e := freezeFeatureScope(context.Background(), root, fixtureRevision, nil)
	if e != nil {
		t.Fatal(e)
	}
	original := b.Files()[0].Bytes
	if e := os.WriteFile(filepath.Join(root, fixtureScopePaths[0]), []byte("changed after return"), 0644); e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(original, b.Files()[0].Bytes) || len(b.Selection().Files) != 4 || len(b.Files()) != 6 {
		t.Fatal("P1/P2 frozen observation or exact exclusion")
	}
	bad, e := freezeFeatureScope(context.Background(), root, fixtureRevision, b.Files())
	if e == nil || bad.state != nil {
		t.Fatal("P3 changed checkout pin accepted")
	}
}
func TestFeatureScopeOrdinaryHardlinkAllowed(t *testing.T) {
	root := scopeFixture(t)
	target := filepath.Join(root, fixtureScopePaths[0])
	if e := os.Link(target, target+"-link"); e != nil {
		t.Fatal(e)
	}
	if _, e := freezeFeatureScope(context.Background(), root, fixtureRevision, nil); e != nil {
		t.Fatalf("source is not a published single-link object: %v", e)
	}
}
