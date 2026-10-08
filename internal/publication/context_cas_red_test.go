package publication

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
)

func TestPublishVerifiedGenerationContextCancelledBeforeStart(t *testing.T) {
	_, root := boundRoot(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := NewPublisher().PublishVerifiedGenerationContext(ctx, root, []byte("artifact"), "fixture")
	if result.Failure == nil || result.Receipt != nil || result.Partial == nil || result.Partial.ArtifactCommitted || result.Partial.ReceiptCommitted || result.Partial.SelectorCommitted {
		t.Fatalf("ASSERT_VERIFIED_GENERATION_CONTEXT_PREFLIGHT_CANCEL_NO_COMMIT: %+v", result)
	}
}

func TestCompareAndReplaceBoundFilePredecessorCAS(t *testing.T) {
	_, root := boundRoot(t)
	first := []byte(`{"generation":"g-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}` + "\n")
	created, err := CompareAndReplaceBoundFile(context.Background(), root, "current.json", BoundFilePredecessor{Absent: true}, first, func([]byte) error { return nil })
	if err != nil || created == nil || !created.Committed || created.FinalSelector != "current.json" {
		t.Fatalf("ASSERT_BOUND_CAS_CREATE_FROM_ABSENT: receipt=%+v err=%v", created, err)
	}
	stale, err := CompareAndReplaceBoundFile(context.Background(), root, "current.json", BoundFilePredecessor{Absent: true}, []byte("stale\n"), func([]byte) error { return nil })
	if !errors.Is(err, ErrStalePredecessor) || stale == nil || stale.Committed {
		t.Fatalf("ASSERT_BOUND_CAS_STALE_ABSENT_LOSES: receipt=%+v err=%v", stale, err)
	}
	second := []byte(`{"generation":"g-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}` + "\n")
	replaced, err := CompareAndReplaceBoundFile(context.Background(), root, "current.json", BoundFilePredecessor{Selector: "current.json", Digest: DigestForTest(first), ByteLength: uint64(len(first))}, second, func([]byte) error { return nil })
	if err != nil || replaced == nil || !replaced.Committed {
		t.Fatalf("ASSERT_BOUND_CAS_REPLACE_EXACT_PREDECESSOR: receipt=%+v err=%v", replaced, err)
	}
	got, err := root.ReadSelector("current.json", 4096)
	if err != nil || !bytes.Equal(got, second) {
		t.Fatalf("ASSERT_BOUND_CAS_FINAL_BYTES_REREAD: got=%q err=%v", got, err)
	}
}

func TestCompareAndReplaceBoundFileRejectsTraversalAndRootSubstitution(t *testing.T) {
	dir, root := boundRoot(t)
	if receipt, err := CompareAndReplaceBoundFile(context.Background(), root, "../escape", BoundFilePredecessor{Absent: true}, []byte("x"), func([]byte) error { return nil }); err == nil || receipt != nil {
		t.Fatalf("ASSERT_BOUND_CAS_REJECTS_TRAVERSAL: receipt=%+v err=%v", receipt, err)
	}
	if err := os.Rename(dir, dir+".old"); err != nil {
		t.Skipf("platform kept pinned root path in place: %v", err)
	}
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	receipt, err := CompareAndReplaceBoundFile(context.Background(), root, "current.json", BoundFilePredecessor{Absent: true}, []byte("x"), func([]byte) error { return nil })
	if err != nil || receipt == nil || !receipt.Committed {
		t.Fatalf("ASSERT_BOUND_CAS_USES_PINNED_ROOT_AFTER_PATH_SUBSTITUTION: receipt=%+v err=%v", receipt, err)
	}
	if _, err := os.Stat(dir + "/current.json"); !os.IsNotExist(err) {
		t.Fatalf("ASSERT_BOUND_CAS_DOES_NOT_WRITE_SUBSTITUTED_PATH: err=%v", err)
	}
	got, err := root.ReadSelector("current.json", 10)
	if err != nil || string(got) != "x" {
		t.Fatalf("ASSERT_BOUND_CAS_WRITES_PINNED_ROOT: got=%q err=%v", got, err)
	}
}
