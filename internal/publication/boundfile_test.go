package publication

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func resetBoundFileHooks(t *testing.T) {
	t.Helper()
	oldAfterVerify, oldBeforePublish := testHookBoundFileAfterVerify, testHookBoundFileBeforePublish
	oldAfterPublish, oldUnsupported := testHookBoundFileAfterPublish, testForceUnsupportedPrimitive
	oldDirectorySync := testHookBoundFileDirectorySync
	oldSourceClose, oldFinalClose, oldRootClose := testHookBoundFileSourceClose, testHookBoundFileFinalClose, testHookBoundFileRootClose
	t.Cleanup(func() {
		testHookBoundFileAfterVerify, testHookBoundFileBeforePublish = oldAfterVerify, oldBeforePublish
		testHookBoundFileAfterPublish, testForceUnsupportedPrimitive = oldAfterPublish, oldUnsupported
		testHookBoundFileDirectorySync = oldDirectorySync
		testHookBoundFileSourceClose, testHookBoundFileFinalClose, testHookBoundFileRootClose = oldSourceClose, oldFinalClose, oldRootClose
	})
}

func boundRoot(t *testing.T) (string, *Root) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	return dir, root
}

func TestBoundFileExactFDPublicationIgnoresSourcePathForgery(t *testing.T) {
	resetBoundFileHooks(t)
	dir, root := boundRoot(t)
	testHookBoundFileAfterVerify = func() {
		// There is no source pathname to replace. A forged legacy-style sibling
		// cannot influence the retained source descriptor.
		if err := os.WriteFile(filepath.Join(dir, ".lsp-trace-bundle-forgery"), []byte("forged"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	receipt, err := PublishBoundFile(root, "capture.bundle", []byte("verified"), func(got []byte) error {
		if string(got) != "verified" {
			return errors.New("wrong verified bytes")
		}
		return nil
	})
	if err != nil || receipt == nil || receipt.VerificationStatus != "VERIFIED" {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	got, err := ReadBoundFile(root, "capture.bundle", 1024)
	if err != nil || string(got) != "verified" {
		t.Fatalf("published=%q err=%v", got, err)
	}
}

func TestBoundFileNoPrecommitSelector(t *testing.T) {
	resetBoundFileHooks(t)
	dir, root := boundRoot(t)
	testHookBoundFileAfterVerify = func() {
		if _, err := os.Lstat(filepath.Join(dir, "capture.bundle")); !os.IsNotExist(err) {
			t.Fatalf("precommit selector visible: %v", err)
		}
	}
	if _, err := PublishBoundFile(root, "capture.bundle", []byte("verified"), func([]byte) error { return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestBoundFileFinalCompetitorPreserved(t *testing.T) {
	resetBoundFileHooks(t)
	dir, root := boundRoot(t)
	testHookBoundFileBeforePublish = func() {
		if err := os.WriteFile(filepath.Join(dir, "capture.bundle"), []byte("competitor"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	receipt, err := PublishBoundFile(root, "capture.bundle", []byte("verified"), func([]byte) error { return nil })
	if !errors.Is(err, os.ErrExist) || receipt != nil {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	got, readErr := os.ReadFile(filepath.Join(dir, "capture.bundle"))
	if readErr != nil || string(got) != "competitor" {
		t.Fatalf("competitor=%q err=%v", got, readErr)
	}
}

func TestBoundFileUnsupportedPrimitiveLeavesNoSelector(t *testing.T) {
	resetBoundFileHooks(t)
	dir, root := boundRoot(t)
	testForceUnsupportedPrimitive = true
	receipt, err := PublishBoundFile(root, "capture.bundle", []byte("verified"), func([]byte) error { return nil })
	if !errors.Is(err, errExactFDUnsupported) || receipt != nil {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	if _, statErr := os.Lstat(filepath.Join(dir, "capture.bundle")); !os.IsNotExist(statErr) {
		t.Fatalf("selector visible: %v", statErr)
	}
}

func TestBoundFilePostcommitVerificationReturnsCommittedReceipt(t *testing.T) {
	resetBoundFileHooks(t)
	dir, root := boundRoot(t)
	testHookBoundFileAfterPublish = func() {
		if err := os.WriteFile(filepath.Join(dir, "capture.bundle"), []byte("corrupt"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	receipt, err := PublishBoundFile(root, "capture.bundle", []byte("verified"), func([]byte) error { return nil })
	if err != nil || receipt == nil || receipt.VerificationStatus != "COMMITTED_VERIFICATION_FAILED" {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
}

func TestBoundFilePostcommitStatusMatrix(t *testing.T) {
	tests := []struct {
		name, directory, close, verification string
		configure                            func()
	}{
		{name: "complete", directory: "FINAL_DIRECTORY_SYNCED_NO_CRASH_GUARANTEE", close: "COMMITTED_CLOSE_COMPLETE", verification: "VERIFIED", configure: func() {}},
		{name: "directory", directory: "COMMITTED_DIRECTORY_SYNC_FAILED", close: "COMMITTED_CLOSE_COMPLETE", verification: "VERIFIED", configure: func() {
			testHookBoundFileDirectorySync = func() error { return errors.New("synthetic directory sync failure") }
		}},
		{name: "source-close", directory: "FINAL_DIRECTORY_SYNCED_NO_CRASH_GUARANTEE", close: "COMMITTED_CLOSE_FAILED", verification: "VERIFIED", configure: func() {
			testHookBoundFileSourceClose = func() error { return errors.New("synthetic source close failure") }
		}},
		{name: "final-close-and-verification", directory: "FINAL_DIRECTORY_SYNCED_NO_CRASH_GUARANTEE", close: "COMMITTED_CLOSE_FAILED", verification: "COMMITTED_VERIFICATION_FAILED", configure: func() {
			testHookBoundFileFinalClose = func() error { return errors.New("synthetic final close failure") }
		}},
		{name: "root-close", directory: "FINAL_DIRECTORY_SYNCED_NO_CRASH_GUARANTEE", close: "COMMITTED_CLOSE_FAILED", verification: "VERIFIED", configure: func() {
			testHookBoundFileRootClose = func() error { return errors.New("synthetic root close failure") }
		}},
		{name: "all-degraded", directory: "COMMITTED_DIRECTORY_SYNC_FAILED", close: "COMMITTED_CLOSE_FAILED", verification: "COMMITTED_VERIFICATION_FAILED", configure: func() {
			testHookBoundFileDirectorySync = func() error { return errors.New("synthetic directory sync failure") }
			testHookBoundFileSourceClose = func() error { return errors.New("synthetic source close failure") }
			testHookBoundFileFinalClose = func() error { return errors.New("synthetic final close failure") }
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetBoundFileHooks(t)
			_, root := boundRoot(t)
			tc.configure()
			before := openFDCount()
			for i := 0; i < 20; i++ {
				verifyCalls := 0
				verify := func([]byte) error {
					verifyCalls++
					if tc.verification == "COMMITTED_VERIFICATION_FAILED" && verifyCalls > 1 {
						return errors.New("synthetic postcommit verification failure")
					}
					return nil
				}
				selector := fmt.Sprintf("capture-%02d.bundle", i)
				receipt, err := PublishBoundFile(root, selector, []byte("verified"), verify)
				if err != nil || receipt == nil || receipt.FinalSelector != selector {
					t.Fatalf("iteration %d committed result: receipt=%+v err=%v", i, receipt, err)
				}
				if receipt.DirectorySyncStatus != tc.directory || receipt.CloseStatus != tc.close || receipt.VerificationStatus != tc.verification {
					t.Fatalf("iteration %d directory=%q want=%q close=%q want=%q verification=%q want=%q", i, receipt.DirectorySyncStatus, tc.directory, receipt.CloseStatus, tc.close, receipt.VerificationStatus, tc.verification)
				}
			}
			if after := openFDCount(); before >= 0 && after != before {
				t.Fatalf("descriptor count changed after repetitions: before=%d after=%d", before, after)
			}
		})
	}
}

func TestPostcommitStatusClassifiersAreClosed(t *testing.T) {
	synthetic := errors.New("synthetic")
	if got := postcommitDirectorySyncStatus(false, nil); got != DirectorySyncNotAttemptedPostCommit {
		t.Fatalf("unattempted directory sync=%q", got)
	}
	if got := postcommitDirectorySyncStatus(true, nil); got != DirectorySyncComplete {
		t.Fatalf("complete directory sync=%q", got)
	}
	if got := postcommitDirectorySyncStatus(true, synthetic); got != DirectorySyncFailed {
		t.Fatalf("failed directory sync=%q", got)
	}
	if got := postcommitCloseStatus(false, nil); got != CloseNotAttempted {
		t.Fatalf("unattempted close=%q", got)
	}
	if got := postcommitCloseStatus(true, nil); got != CloseComplete {
		t.Fatalf("complete close=%q", got)
	}
	if got := postcommitCloseStatus(true, synthetic); got != CloseFailed {
		t.Fatalf("failed close=%q", got)
	}
}

func TestBoundFilePrecommitCloseFailureRemainsError(t *testing.T) {
	resetBoundFileHooks(t)
	_, root := boundRoot(t)
	testHookBoundFileSourceClose = func() error { return errors.New("synthetic source close failure") }
	receipt, err := PublishBoundFile(root, "capture.bundle", []byte("bad"), func([]byte) error { return errors.New("reject before commit") })
	if err == nil || receipt != nil {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
}

func TestBoundFilePublishesNoSidecar(t *testing.T) {
	dir, root := boundRoot(t)
	if _, err := PublishBoundFile(root, "capture.bundle", []byte("verified"), func([]byte) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "capture.bundle.active")); !os.IsNotExist(err) {
		t.Fatalf("sidecar exists: %v", err)
	}
}

func TestBoundFileConcurrentPublicationHasOneWinner(t *testing.T) {
	_, root := boundRoot(t)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := PublishBoundFile(root, "capture.bundle", []byte("exact"), func([]byte) error { return nil })
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	var success, exists int
	for err := range errs {
		if err == nil {
			success++
		} else if errors.Is(err, os.ErrExist) {
			exists++
		}
	}
	if success != 1 || exists != 1 {
		t.Fatalf("success=%d exists=%d", success, exists)
	}
}

func TestBoundFileFailureFDStable(t *testing.T) {
	_, root := boundRoot(t)
	before := openFDCount()
	for i := 0; i < 20; i++ {
		name := "bad-" + string(rune('a'+i)) + ".bundle"
		if receipt, err := PublishBoundFile(root, name, []byte("bad"), func([]byte) error { return errors.New("reject") }); err == nil || receipt != nil {
			t.Fatalf("iteration %d committed", i)
		}
	}
	if after := openFDCount(); before >= 0 && after != before {
		t.Fatalf("descriptor count changed: before=%d after=%d", before, after)
	}
}
