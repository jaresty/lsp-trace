package inventorysuccessor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/publication"
)

func TestMultiSeedV2ExactPredecessorSnapshot(t *testing.T) {
	t.Log("procedure: load populated v1 from real head adapter; retain/readback exact immutable bytes; close; remove mutable slot; fresh-root resolve prior bytes; corruption/missing/splice refuse")
	ctx := context.Background()
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	memory := &memoryHeads{}
	request := validRequest(memory)
	if _, err = coordinateTest(ctx, request); err != nil {
		t.Fatal(err)
	}
	heads, err := NewHeads(root)
	if err != nil {
		t.Fatal(err)
	}
	cas := heads.CompareAndSwap(ctx, request.HeadKey, censussourcecontinuation.HeadRevision{}, memory.raw)
	if cas.Outcome != censussourcecontinuation.CASCommitted {
		t.Fatal(cas)
	}
	loaded, err := heads.Load(ctx, request.HeadKey)
	if err != nil {
		t.Fatal(err)
	}
	objects, err := NewMultiSeedObjectsV2(root)
	if err != nil {
		t.Fatal(err)
	}
	prior, err := objects.RetainSuccessorPredecessorV2(ctx, loaded, request.WorkflowKey, request.HeadKey, request.Identity, "sha256:"+strings.Repeat("a", 64))
	if err != nil || prior.SHA256 != digest(loaded) || prior.ByteLength != uint64(len(loaded)) {
		t.Fatalf("EXACT_POPULATED_V1_PREDECESSOR_CUSTODY: %v", err)
	}
	revision, _ := ParseHeadRevision(loaded)
	if prior.Identity != revision.Selector || bytes.Contains(loaded, []byte(`"head_key"`)) {
		t.Fatal("V1_SELECTOR_VS_POPULATED_DIGEST_EXTERNAL_HEAD_KEY")
	}
	if err = root.Close(); err != nil {
		t.Fatal(err)
	}
	slot := sha256.Sum256([]byte(request.HeadKey))
	if err = os.Remove(filepath.Join(directory, headNamespace, hex.EncodeToString(slot[:])+".json")); err != nil {
		t.Fatal(err)
	}
	root, err = publication.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	objects, err = NewMultiSeedObjectsV2(root)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := objects.ResolveSuccessorPredecessorV2(ctx, prior, request.WorkflowKey, request.HeadKey, request.Identity, "sha256:"+strings.Repeat("a", 64))
	if err != nil || !bytes.Equal(recovered, loaded) {
		t.Fatalf("FRESH_ROOT_PRIOR_RESOLVES_IMMUTABLE_ONLY: %v", err)
	}
	if _, err = objects.ResolveSuccessorPredecessorV2(ctx, prior, "foreign-workflow", request.HeadKey, request.Identity, "sha256:"+strings.Repeat("a", 64)); err == nil {
		t.Fatal("PRIOR_WORKFLOW_SPLICE_REFUSAL")
	}
	selector := "program-c-representative/successor-predecessors/objects/private/v2/" + strings.TrimPrefix(prior.SHA256, "sha256:") + ".json"
	path := filepath.Join(directory, filepath.FromSlash(selector))
	if err = os.WriteFile(path, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = objects.ResolveSuccessorPredecessorV2(ctx, prior, request.WorkflowKey, request.HeadKey, request.Identity, "sha256:"+strings.Repeat("a", 64)); err == nil {
		t.Fatal("CORRUPT_PRIOR_SNAPSHOT_REFUSAL")
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err = objects.ResolveSuccessorPredecessorV2(ctx, prior, request.WorkflowKey, request.HeadKey, request.Identity, "sha256:"+strings.Repeat("a", 64)); err == nil {
		t.Fatal("MISSING_PRIOR_SNAPSHOT_REFUSAL")
	}
}
