package censuslineagepilot

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/censussourceassembly"
	"lsp-trace/internal/publication"
)

type countingProductionObjects struct {
	ImmutableStore
	puts int
}

func (s *countingProductionObjects) Put(ctx context.Context, raw []byte) (string, error) {
	s.puts++
	return s.ImmutableStore.Put(ctx, raw)
}

func TestProductionStoreRejectsPresentEmptyHeadBeforeWrites(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir()
	if err := os.Chmod(path, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	objects, err := censussourceassembly.NewObjects(root)
	if err != nil {
		t.Fatal(err)
	}
	counting := &countingProductionObjects{ImmutableStore: objects}
	heads, err := NewHeads(root)
	if err != nil {
		t.Fatal(err)
	}
	const headKey = "present-empty-head"
	sum := sha256.Sum256([]byte(headKey))
	headPath := filepath.Join(path, filepath.FromSlash(headNamespace), hex.EncodeToString(sum[:])+".json")
	if err := os.MkdirAll(filepath.Dir(headPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(headPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseHeadRevision(nil); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty parser input err=%v", err)
	}
	if _, err := heads.Load(ctx, headKey); err == nil {
		t.Fatal("present empty head loaded as absence")
	}
	manifest, manifestRaw := fixture(t)
	request := Request{WorkflowKey: "present-empty-workflow", HeadKey: headKey, Manifest: manifest, ManifestBytes: manifestRaw, Objects: counting, Heads: heads}
	if _, err := Coordinate(ctx, request); err == nil {
		t.Fatal("coordinate accepted present empty head")
	}
	after, err := os.ReadFile(headPath)
	if err != nil {
		t.Fatal(err)
	}
	if counting.puts != 0 || len(after) != 0 {
		t.Fatalf("malformed state caused writes: puts=%d head=%q", counting.puts, after)
	}
}

func TestProductionStoresCloseReopenVerifyReadOnlyAndNamespaceIsolation(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir()
	if err := os.Chmod(path, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	objects, err := censussourceassembly.NewObjects(root)
	if err != nil {
		t.Fatal(err)
	}
	heads, err := NewHeads(root)
	if err != nil {
		t.Fatal(err)
	}
	manifest, manifestRaw := fixture(t)
	request := Request{WorkflowKey: "production-close-reopen", HeadKey: "lineage-head", Manifest: manifest, ManifestBytes: manifestRaw, Objects: objects, Heads: heads}
	committed, err := Coordinate(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	headBefore, err := heads.Load(ctx, request.HeadKey)
	if err != nil {
		t.Fatal(err)
	}
	objectBefore, err := objects.Get(ctx, committed.ManifestSelector)
	if err != nil {
		t.Fatal(err)
	}

	otherHeads, err := censussourceassembly.NewHeadsWithParser(root, "census-lineage-pilot-test/other-heads/private/v1", ParseHeadRevision)
	if err != nil {
		t.Fatal(err)
	}
	otherRaw, err := otherHeads.Load(ctx, request.HeadKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(otherRaw) != 0 {
		t.Fatalf("different namespace loaded pilot head: %s", otherRaw)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := publication.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	reopenedObjects, err := censussourceassembly.NewObjects(reopened)
	if err != nil {
		t.Fatal(err)
	}
	reopenedHeads, err := NewHeads(reopened)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyReopen(ctx, request.WorkflowKey, request.HeadKey, manifest.LineageID, manifestRaw, reopenedObjects, reopenedHeads); err != nil {
		t.Fatal(err)
	}
	headAfter, err := reopenedHeads.Load(ctx, request.HeadKey)
	if err != nil {
		t.Fatal(err)
	}
	objectAfter, err := reopenedObjects.Get(ctx, committed.ManifestSelector)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(headAfter, headBefore) || !bytes.Equal(objectAfter, objectBefore) {
		t.Fatalf("reopen verification mutated retained custody: head_equal=%v object_equal=%v", bytes.Equal(headAfter, headBefore), bytes.Equal(objectAfter, objectBefore))
	}
}
