package publication

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type failingBoundFileTraceSink struct{ err error }

func (s failingBoundFileTraceSink) WriteBoundFileTrace(BoundFileTraceEvent) error { return s.err }

func TestBoundFileTraceSinkFailureIsTypedAndDoesNotChangePrimaryResult(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	sentinel := errors.New("private sink failure")
	result := PublishBoundFileWithTraceSink(root, "artifact.json", []byte("exact"), func([]byte) error { return nil }, failingBoundFileTraceSink{err: sentinel})
	if result.Receipt == nil || result.Failure != nil {
		t.Fatalf("ASSERT_TRACE_FAILURE_PRIMARY_RESULT_UNCHANGED: %#v", result)
	}
	if result.TraceFailure == nil || result.TraceFailure.Stage != "TRACE_WRITE" || result.TraceFailure.Reason != "SINK_FAILED" || !errors.Is(result.TraceFailure, sentinel) {
		t.Fatalf("ASSERT_TRACE_FAILURE_TYPED_PRIVATE: %#v", result.TraceFailure)
	}
}

func TestPreCommitPrimaryFailureIsNotOverwrittenByDeferredCleanup(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	testForceUnsupportedPrimitive = true
	t.Cleanup(func() { testForceUnsupportedPrimitive = false })

	result := PublishBoundFileWithTraceSink(root, "artifact.json", []byte("exact"), func([]byte) error { return nil }, nil)
	if result.Failure == nil {
		t.Fatal("ASSERT_PRIMARY_FAILURE_RETAINED: missing failure")
	}
	if result.Failure.Stage == "CLEANUP" || result.Failure.Code == "COMPLETE" {
		t.Fatalf("ASSERT_DEFERRED_CLEANUP_DID_NOT_OVERWRITE_PRIMARY: %#v", result.Failure)
	}
	if !errors.Is(result.Failure, errExactFDUnsupported) {
		t.Fatalf("ASSERT_PRIMARY_CAUSE_RETAINED: %#v", result.Failure)
	}
}

func TestPreCommitPrimaryFailureRemainsPrimaryWhenCleanupAlsoFails(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	cleanupErr := errors.New("injected cleanup failure")
	testForceUnsupportedPrimitive = true
	testHookBoundFileRootClose = func() error { return cleanupErr }
	t.Cleanup(func() {
		testForceUnsupportedPrimitive = false
		testHookBoundFileRootClose = nil
	})

	result := PublishBoundFileWithTraceSink(root, "nested/artifact.json", []byte("exact"), func([]byte) error { return nil }, nil)
	if result.Failure == nil || result.Failure.Stage != "TEMP" || result.Failure.Code != "UNSUPPORTED" {
		t.Fatalf("ASSERT_PRIMARY_PRECEDES_CLEANUP: %#v", result.Failure)
	}
	if !errors.Is(result.Failure, errExactFDUnsupported) || !errors.Is(result.Failure, cleanupErr) {
		t.Fatalf("ASSERT_PRIMARY_AND_CLEANUP_CAUSES_RETAINED: %#v", result.Failure)
	}
	if result.Receipt != nil || !result.Failure.Cleanup || result.Failure.AtomicRename {
		t.Fatalf("ASSERT_PRECOMMIT_FLAGS_EXACT: %#v", result)
	}
}

func TestExactReportedCandidateSizePublishesWithoutResidue(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	raw := make([]byte, 10_090_839)
	for i := range raw {
		raw[i] = byte(i % 251)
	}

	receipt, err := PublishBoundFile(root, "artifact.bundle", raw, func(got []byte) error {
		if len(got) != len(raw) {
			return errors.New("exact-size verification mismatch")
		}
		return nil
	})
	if err != nil || receipt == nil || !receipt.NamespaceAtomic || receipt.ByteLength != uint64(len(raw)) || receipt.VerificationStatus != "VERIFIED" {
		t.Fatalf("ASSERT_EXACT_SIZE_PUBLICATION: receipt=%#v err=%v", receipt, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "artifact.bundle" {
		t.Fatalf("ASSERT_NORMAL_PATH_NO_TEMP_RESIDUE: %v", entries)
	}
}

func TestFailAfterTempBeforeInstallCleansTempAndLeavesFinalAbsent(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	sentinel := errors.New("injected before install")
	testHookBoundFileAfterTempBeforeInstall = func() error { return sentinel }
	t.Cleanup(func() { testHookBoundFileAfterTempBeforeInstall = nil })
	result := PublishBoundFileWithTraceSink(root, "nested/artifact.json", []byte("exact"), func([]byte) error { return nil }, nil)
	if result.Failure == nil || result.Failure.Stage != "NO_REPLACE" || result.Failure.Code != "INJECTED_BEFORE_INSTALL" || !result.Failure.Cleanup || !errors.Is(result.Failure, sentinel) {
		t.Fatalf("ASSERT_FAIL_AFTER_TEMP_TYPED: %#v", result)
	}
	if _, err := os.Lstat(filepath.Join(dir, "nested", "artifact.json")); !os.IsNotExist(err) {
		t.Fatalf("ASSERT_FAIL_AFTER_TEMP_FINAL_ABSENT: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(dir, "nested"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("ASSERT_FAIL_AFTER_TEMP_NO_RESIDUE: %v", entries)
	}
}
