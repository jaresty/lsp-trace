package publication

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func withBoundFileHooks(t *testing.T) {
	oldVerified, oldActivate := testHookBoundFileAfterVerify, testHookBoundFileBeforeActivate
	oldWrite, oldSync, oldClose, oldRemove := boundFileWriteAll, boundFileSync, boundFileClose, boundFileRemove
	t.Cleanup(func() {
		testHookBoundFileAfterVerify, testHookBoundFileBeforeActivate = oldVerified, oldActivate
		boundFileWriteAll, boundFileSync, boundFileClose, boundFileRemove = oldWrite, oldSync, oldClose, oldRemove
	})
}

func TestBoundFilePathReplacementAfterVerificationCannotCommitDifferentBytes(t *testing.T) {
	withBoundFileHooks(t)
	dir := t.TempDir()
	_ = os.Chmod(dir, 0o700)
	root, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	testHookBoundFileAfterVerify = func(_ *os.Root, name string) {
		if err := os.Rename(filepath.Join(dir, name), filepath.Join(dir, name+".moved")); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte("competitor"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	receipt, err := PublishBoundFile(root, "capture.bundle", []byte("verified"), func(got []byte) error {
		if string(got) != "verified" {
			return errors.New("wrong verified bytes")
		}
		return nil
	})
	if err == nil || receipt != nil {
		t.Fatalf("replacement committed: receipt=%+v err=%v", receipt, err)
	}
	if got, readErr := os.ReadFile(filepath.Join(dir, "capture.bundle")); readErr != nil || string(got) != "competitor" {
		t.Fatalf("competitor changed: %q %v", got, readErr)
	}
	if _, readErr := ReadBoundFile(root, "capture.bundle", 1024); readErr == nil {
		t.Fatal("inactive replacement resolved")
	}
}

func TestBoundFileFsyncAndCleanupFailuresNeverActivate(t *testing.T) {
	for _, tc := range []struct {
		name      string
		configure func()
	}{
		{name: "zero-write", configure: func() { boundFileWriteAll = func(io.Writer, []byte) error { return io.ErrShortWrite } }},
		{name: "fsync", configure: func() { boundFileSync = func(*os.File) error { return errors.New("fsync failure") } }},
		{name: "cleanup", configure: func() { boundFileRemove = func(*os.Root, string) error { return errors.New("cleanup failure") } }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withBoundFileHooks(t)
			dir := t.TempDir()
			_ = os.Chmod(dir, 0o700)
			root, err := OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			tc.configure()
			verify := func([]byte) error { return nil }
			if tc.name == "cleanup" {
				verify = func([]byte) error { return errors.New("verification failure") }
			}
			if receipt, err := PublishBoundFile(root, "capture.bundle", []byte("exact"), verify); err == nil || receipt != nil {
				t.Fatalf("fault committed: %+v %v", receipt, err)
			}
			if _, err := ReadBoundFile(root, "capture.bundle", 1024); err == nil {
				t.Fatal("precommit fault resolved")
			}
			if _, err := os.Lstat(filepath.Join(dir, "capture.bundle.active")); !os.IsNotExist(err) {
				t.Fatalf("activation visible: %v", err)
			}
		})
	}
}

func TestBoundFilePostcommitCloseFailureCannotReverseSuccess(t *testing.T) {
	withBoundFileHooks(t)
	dir := t.TempDir()
	_ = os.Chmod(dir, 0o700)
	root, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	boundFileClose = func(f *os.File) error { _ = f.Close(); return errors.New("close failure") }
	receipt, err := PublishBoundFile(root, "capture.bundle", []byte("exact"), func([]byte) error { return nil })
	if err != nil || receipt == nil {
		t.Fatalf("postcommit close reversed success: %+v %v", receipt, err)
	}
	if got, err := ReadBoundFile(root, "capture.bundle", 1024); err != nil || string(got) != "exact" {
		t.Fatalf("committed bytes unavailable: %q %v", got, err)
	}
}

func TestBoundFileLateActivationCompetitorPreserved(t *testing.T) {
	withBoundFileHooks(t)
	dir := t.TempDir()
	_ = os.Chmod(dir, 0o700)
	root, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	testHookBoundFileBeforeActivate = func(parent *os.Root, name string) {
		if err := parent.Symlink("competitor", name+".active"); err != nil {
			t.Fatal(err)
		}
	}
	receipt, err := PublishBoundFile(root, "capture.bundle", []byte("exact"), func([]byte) error { return nil })
	if !errors.Is(err, os.ErrExist) || receipt != nil {
		t.Fatalf("late competitor: %+v %v", receipt, err)
	}
	if target, err := os.Readlink(filepath.Join(dir, "capture.bundle.active")); err != nil || target != "competitor" {
		t.Fatalf("competitor changed: %q %v", target, err)
	}
}

func TestBoundFileVerificationFailureCleansReservedNameAndFD(t *testing.T) {
	dir := t.TempDir()
	_ = os.Chmod(dir, 0o700)
	root, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	before := openFDCount()
	for i := 0; i < 20; i++ {
		name := filepath.Base(filepath.Join(".", "bad-"+string(rune('a'+i))+".bundle"))
		if receipt, err := PublishBoundFile(root, name, []byte("bad"), func([]byte) error { return errors.New("reject") }); err == nil || receipt != nil {
			t.Fatalf("iteration %d committed", i)
		}
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("iteration %d leaked reserved name: %v", i, err)
		}
	}
	if after := openFDCount(); before >= 0 && after != before {
		t.Fatalf("descriptor count changed: before=%d after=%d", before, after)
	}
}

func TestBoundFileUsesPinnedRootAfterPathReplacement(t *testing.T) {
	base := t.TempDir()
	original := filepath.Join(base, "root")
	moved := filepath.Join(base, "moved")
	if err := os.Mkdir(original, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := OpenRoot(original)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	withBoundFileHooks(t)
	testHookBoundFileBeforeActivate = func(_ *os.Root, _ string) {
		if err := os.Rename(original, moved); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(original, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := PublishBoundFile(root, "capture.bundle", []byte("exact"), func([]byte) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(moved, "capture.bundle.active")); err != nil {
		t.Fatalf("pinned root not activated: %v", err)
	}
	if _, err := os.Stat(filepath.Join(original, "capture.bundle")); !os.IsNotExist(err) {
		t.Fatalf("replacement root received publication: %v", err)
	}
}

func TestBoundFileConcurrentPublicationHasOneWinner(t *testing.T) {
	dir := t.TempDir()
	_ = os.Chmod(dir, 0o700)
	root, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
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
	if got, err := ReadBoundFile(root, "capture.bundle", 1024); err != nil || string(got) != "exact" {
		t.Fatalf("read=%q err=%v", got, err)
	}
}
