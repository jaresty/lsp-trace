package censussourceassembly

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/censussourceplan"
	"lsp-trace/internal/publication"
)

var _ censussourcecontinuation.ImmutableStore = (*Objects)(nil)
var _ censussourcecontinuation.HeadStore = (*Heads)(nil)

func privateRoot(t *testing.T) *publication.Root {
	t.Helper()
	path := t.TempDir()
	if err := os.Chmod(path, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return root
}

func publicRoot(t *testing.T) *publication.Root {
	t.Helper()
	path := t.TempDir()
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return root
}

func objectSelector(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func expectedHeadSelector(key string) string {
	sum := sha256.Sum256([]byte(key))
	return "census-source-continuation/heads/v1/" + hex.EncodeToString(sum[:]) + ".json"
}

func validHeadBytes(t *testing.T, objects censussourcecontinuation.ImmutableStore, heads censussourcecontinuation.HeadStore, key string) ([]byte, censussourcecontinuation.HeadRevision) {
	t.Helper()
	workspace := t.TempDir()
	plan, err := censussourceplan.Build([]censusacquisition.SourceFile{{Path: "a.go", URI: "file://" + filepath.ToSlash(filepath.Join(workspace, "a.go")), LanguageID: "go"}}, workspace, 1)
	if err != nil {
		t.Fatal(err)
	}
	terminal := censussourcecontinuation.Terminal{PlanIdentity: plan.Identity, Ordinal: 0, SourceFiles: append([]censusacquisition.SourceFile(nil), plan.Partitions[0].Members...), Discovery: censusacquisition.Discovery{Closure: censusacquisition.DiscoveryClosureSuccessful}, Outcome: censussourcecontinuation.OutcomeSuccessfulEmpty}
	result, err := censussourcecontinuation.CoordinateNext(context.Background(), censussourcecontinuation.Request{Plan: plan, PartitionSize: 1, WorkflowKey: key, HeadKey: key, ExistingOrdinalZero: &terminal, Objects: objects, Heads: heads})
	if err != nil || result.Status != censussourcecontinuation.StatusCommitted {
		t.Fatalf("ASSERT_ASSEMBLY_HEAD_FIXTURE_COMMITTED result=%+v err=%v", result, err)
	}
	raw, err := heads.Load(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	return raw, result.Head
}

func TestConstructorsRejectNilAndPublicRoots(t *testing.T) {
	if got, err := NewObjects(nil); err == nil || got != nil {
		t.Fatalf("ASSERT_ASSEMBLY_OBJECTS_REJECT_NIL got=%v err=%v", got, err)
	}
	if got, err := NewHeads(nil); err == nil || got != nil {
		t.Fatalf("ASSERT_ASSEMBLY_HEADS_REJECT_NIL got=%v err=%v", got, err)
	}
	root := publicRoot(t)
	if got, err := NewObjects(root); err == nil || got != nil {
		t.Fatalf("ASSERT_ASSEMBLY_OBJECTS_REJECT_PUBLIC got=%v err=%v", got, err)
	}
	if got, err := NewHeads(root); err == nil || got != nil {
		t.Fatalf("ASSERT_ASSEMBLY_HEADS_REJECT_PUBLIC got=%v err=%v", got, err)
	}
}

func TestObjectsExactSelectorBytesPutGetExistingAndClone(t *testing.T) {
	root := privateRoot(t)
	objects, err := NewObjects(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte(`{"object":"exact"}`)
	selector, err := objects.Put(context.Background(), want)
	if err != nil || selector != objectSelector(want) {
		t.Fatalf("ASSERT_ASSEMBLY_OBJECT_EXACT_SELECTOR got=%q want=%q err=%v", selector, objectSelector(want), err)
	}
	want[0] = 'X'
	got, err := objects.Get(context.Background(), selector)
	if err != nil || string(got) != `{"object":"exact"}` {
		t.Fatalf("ASSERT_ASSEMBLY_OBJECT_EXACT_BYTES got=%q err=%v", got, err)
	}
	got[0] = 'Y'
	again, err := objects.Get(context.Background(), selector)
	if err != nil || string(again) != `{"object":"exact"}` {
		t.Fatalf("ASSERT_ASSEMBLY_OBJECT_GET_CLONE got=%q err=%v", again, err)
	}
	if duplicate, err := objects.Put(context.Background(), again); err != nil || duplicate != selector {
		t.Fatalf("ASSERT_ASSEMBLY_OBJECT_EXISTING_TARGET selector=%q err=%v", duplicate, err)
	}
}

func TestObjectsCollisionFailsClosed(t *testing.T) {
	root := privateRoot(t)
	objects, err := NewObjects(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte("wanted")
	selector := objectSelector(want)
	published := publication.NewPublisher().Publish(publication.Request{Root: root, Selector: selector, Bytes: []byte("competitor"), ArtifactSchemaID: "test.collision"})
	if published.Failure != nil {
		t.Fatal(published.Failure)
	}
	if got, err := objects.Put(context.Background(), want); err == nil || got != "" || !strings.Contains(err.Error(), "collision") {
		t.Fatalf("ASSERT_ASSEMBLY_OBJECT_COLLISION got=%q err=%v", got, err)
	}
}

func TestObjectsFourMiBBoundary(t *testing.T) {
	objects, err := NewObjects(privateRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	at := bytes.Repeat([]byte{'a'}, MaxBytes)
	if _, err := objects.Put(context.Background(), at); err != nil {
		t.Fatalf("ASSERT_ASSEMBLY_OBJECT_4MIB_ACCEPTED err=%v", err)
	}
	if selector, err := objects.Put(context.Background(), append(at, 'b')); err == nil || selector != "" {
		t.Fatalf("ASSERT_ASSEMBLY_OBJECT_4MIB_PLUS_ONE_REJECTED selector=%q err=%v", selector, err)
	}
}

func TestHeadsMissingExactSelectorBytesMalformedStaleAndCommitted(t *testing.T) {
	root := privateRoot(t)
	objects, err := NewObjects(root)
	if err != nil {
		t.Fatal(err)
	}
	heads, err := NewHeads(root)
	if err != nil {
		t.Fatal(err)
	}
	if raw, err := heads.Load(context.Background(), "missing"); err != nil || raw != nil {
		t.Fatalf("ASSERT_ASSEMBLY_HEAD_MISSING_NIL raw=%q err=%v", raw, err)
	}
	raw, revision := validHeadBytes(t, objects, heads, "exact")
	stored, err := root.ReadSelector(expectedHeadSelector("exact"), MaxBytes)
	if err != nil || !bytes.Equal(stored, raw) {
		t.Fatalf("ASSERT_ASSEMBLY_HEAD_EXACT_SELECTOR_BYTES stored=%q raw=%q err=%v", stored, raw, err)
	}
	parsed, err := censussourcecontinuation.ParseHeadRevision(raw)
	if err != nil || parsed != revision {
		t.Fatalf("ASSERT_ASSEMBLY_HEAD_PARSE_REVISION parsed=%+v want=%+v err=%v", parsed, revision, err)
	}
	if result := heads.CompareAndSwap(context.Background(), "exact", censussourcecontinuation.HeadRevision{}, raw); result.Outcome != censussourcecontinuation.CASNotCommitted || result.Err != nil {
		t.Fatalf("ASSERT_ASSEMBLY_HEAD_STALE_CAS result=%+v", result)
	}
	otherRoot := privateRoot(t)
	otherHeads, _ := NewHeads(otherRoot)
	if result := otherHeads.CompareAndSwap(context.Background(), "committed", censussourcecontinuation.HeadRevision{}, raw); result.Outcome != censussourcecontinuation.CASCommitted || result.Err != nil {
		t.Fatalf("ASSERT_ASSEMBLY_HEAD_COMMITTED_CAS result=%+v", result)
	}
	malformedSelector := expectedHeadSelector("malformed")
	published := publication.NewPublisher().Publish(publication.Request{Root: root, Selector: malformedSelector, Bytes: []byte("{}"), ArtifactSchemaID: "test.invalid"})
	if published.Failure != nil {
		t.Fatal(published.Failure)
	}
	if got, err := heads.Load(context.Background(), "malformed"); err == nil || got != nil {
		t.Fatalf("ASSERT_ASSEMBLY_HEAD_MALFORMED_REJECTED got=%q err=%v", got, err)
	}
}

func TestHeadsCommittedErrorMapsToUncertain(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("bound-file CAS unsupported")
	}
	root := privateRoot(t)
	objects, _ := NewObjects(root)
	heads, _ := NewHeads(root)
	raw, _ := validHeadBytes(t, objects, heads, "source")
	heads.compareAndReplace = func(context.Context, *publication.Root, string, publication.BoundFilePredecessor, []byte, func([]byte) error) (*publication.CompareAndReplaceReceipt, error) {
		return &publication.CompareAndReplaceReceipt{Committed: true}, errors.New("synthetic post-commit uncertainty")
	}
	result := heads.CompareAndSwap(context.Background(), "uncertain", censussourcecontinuation.HeadRevision{}, raw)
	if result.Outcome != censussourcecontinuation.CASCommitUncertain || result.Err == nil {
		t.Fatalf("ASSERT_ASSEMBLY_HEAD_UNCERTAIN_CAS result=%+v", result)
	}
}

func TestProductionImportsRemainNeutral(t *testing.T) {
	raw, err := os.ReadFile("stores.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"sessionruntime", "censussourceplan", "censussourcedriver", "cmd/lsp-trace-mcp", "internal/operation", "internal/schema"} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("ASSERT_ASSEMBLY_FORBIDDEN_IMPORT %q", forbidden)
		}
	}
	if !bytes.Contains(raw, []byte(`lsp-trace.census-source-continuation-object.private.v1`)) {
		t.Fatal("ASSERT_ASSEMBLY_EXACT_OBJECT_SCHEMA_ID")
	}
}
