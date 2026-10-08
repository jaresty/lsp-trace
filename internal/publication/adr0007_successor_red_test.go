//go:build linux || darwin

package publication

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestADR0007RenameBoundFileSiblingDirectorySyncFailureIsPostRenameCommitted(t *testing.T) {
	resetBoundFileHooks(t)
	_, root := boundRoot(t)
	raw := []byte(`{"generation":"g-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}` + "\n")
	if _, err := PublishBoundFile(root, "current.json.tmp", raw, func([]byte) error { return nil }); err != nil {
		t.Fatal(err)
	}
	testHookBoundFileDirectorySync = func() error { return errors.New("synthetic directory sync failure") }
	err := renameBoundFileSibling(root, "current.json.tmp", "current.json")
	got, readErr := root.ReadSelector("current.json", int64(len(raw))+32)
	if readErr != nil || !bytes.Equal(got, raw) {
		t.Fatalf("ASSERT_ADR0007_RENAME_SYNC_FIXTURE_FINAL_VISIBLE_BEFORE_ERROR: got=%q readErr=%v err=%v", got, readErr, err)
	}
	if err != nil {
		t.Fatalf("ASSERT_ADR0007_RENAME_SYNC_POSTRENAME_NEEDS_CLOSED_COMMITTED_OUTCOME: err=%v", err)
	}
}

func TestADR0007CompareAndReplaceBoundFilePostRenameVerifierFailureFailsClosed(t *testing.T) {
	resetBoundFileHooks(t)
	_, root := boundRoot(t)
	raw := []byte(`{"generation":"g-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}` + "\n")
	calls := 0
	verify := func(got []byte) error {
		calls++
		if calls < 3 {
			return nil
		}
		final, err := root.ReadSelector("current.json", int64(len(raw))+32)
		if err != nil || !bytes.Equal(final, raw) || !bytes.Equal(got, raw) {
			t.Fatalf("ASSERT_ADR0007_CAS_VERIFIER_FIXTURE_FINAL_BYTES_EXIST_BEFORE_ERROR: final=%q got=%q err=%v", final, got, err)
		}
		return errors.New("synthetic final verifier failure")
	}
	receipt, err := CompareAndReplaceBoundFile(context.Background(), root, "current.json", BoundFilePredecessor{Absent: true}, raw, verify)
	if err == nil || receipt == nil || !receipt.Committed || receipt.VerificationStatus != "COMMITTED_VERIFICATION_FAILED" {
		t.Fatalf("ASSERT_ADR0007_CAS_POST_RENAME_VERIFIER_FAILS_CLOSED: receipt=%+v err=%v", receipt, err)
	}
}

func TestADR0007CompareAndReplaceBoundFilePostRenameCancellationFailsClosed(t *testing.T) {
	resetBoundFileHooks(t)
	_, root := boundRoot(t)
	raw := []byte(`{"generation":"g-cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}` + "\n")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	verify := func(got []byte) error {
		calls++
		if calls < 3 {
			return nil
		}
		final, err := root.ReadSelector("current.json", int64(len(raw))+32)
		if err != nil || !bytes.Equal(final, raw) || !bytes.Equal(got, raw) {
			t.Fatalf("ASSERT_ADR0007_CAS_CANCEL_FIXTURE_FINAL_BYTES_EXIST_BEFORE_CANCEL: final=%q got=%q err=%v", final, got, err)
		}
		cancel()
		return nil
	}
	receipt, err := CompareAndReplaceBoundFile(ctx, root, "current.json", BoundFilePredecessor{Absent: true}, raw, verify)
	if err == nil || receipt == nil || !receipt.Committed {
		t.Fatalf("ASSERT_ADR0007_CAS_POST_RENAME_CANCEL_FAILS_CLOSED_NO_SAFE_RETRY: receipt=%+v err=%v", receipt, err)
	}
}

func TestADR0007StableCASLockRetainsInodeAndRejectsPathSubstitutionWhileHeld(t *testing.T) {
	if os.Getenv("LSP_TRACE_ADR0007_LOCK_HELPER") == "hold" {
		adr0007LockHelper()
		return
	}
	_, root := boundRoot(t)
	first := adr0007AcquireLockIdentity(t, root)
	if first.Uid != uint32(os.Geteuid()) {
		t.Fatalf("ASSERT_ADR0007_CAS_LOCK_OWNER_POLICY_ACCEPTS_CURRENT_EUID: uid=%d euid=%d", first.Uid, os.Geteuid())
	}
	second := adr0007AcquireLockIdentity(t, root)
	if first.Dev == 0 || first.Ino == 0 || first.Dev != second.Dev || first.Ino != second.Ino {
		t.Fatalf("ASSERT_ADR0007_CAS_LOCK_RETAINS_INODE_SEQUENTIAL: first=%+v second=%+v", first, second)
	}
	reopened, err := OpenRoot(root.Path())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	third := adr0007AcquireLockIdentity(t, reopened)
	if first.Dev != third.Dev || first.Ino != third.Ino {
		t.Fatalf("ASSERT_ADR0007_CAS_LOCK_RETAINS_INODE_RESTART: first=%+v third=%+v", first, third)
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestADR0007StableCASLockRetainsInodeAndRejectsPathSubstitutionWhileHeld")
	cmd.Env = append(os.Environ(), "LSP_TRACE_ADR0007_LOCK_HELPER=hold", "LSP_TRACE_ADR0007_LOCK_ROOT="+root.Path())
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	lockPath := filepath.Join(root.Path(), ".lsp-trace-candidate-publication.lock")
	adr0007WaitForHelperHeld(t, lockPath+".held")
	if err := os.Remove(lockPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath, []byte("replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	unlock, err := root.casLock(ctx, "current.json")
	if err == nil {
		unlock()
		t.Fatalf("ASSERT_ADR0007_CAS_LOCK_PATH_SUBSTITUTION_WHILE_HELD_FAILS_CLOSED")
	}
}

func adr0007LockHelper() {
	rootDir := os.Getenv("LSP_TRACE_ADR0007_LOCK_ROOT")
	root, err := OpenRoot(rootDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer root.Close()
	unlock, err := root.casLock(context.Background(), "current.json")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer unlock()
	_ = os.WriteFile(filepath.Join(rootDir, ".lsp-trace-candidate-publication.lock.held"), []byte("held\n"), 0o600)
	select {}
}

func adr0007WaitForHelperHeld(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(path); err == nil && strings.TrimSpace(string(data)) == "held" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("ASSERT_ADR0007_CAS_LOCK_HELPER_HELD_FLOCK")
}

func adr0007AcquireLockIdentity(t *testing.T, root *Root) syscall.Stat_t {
	t.Helper()
	unlock, err := root.casLock(context.Background(), "current.json")
	if err != nil {
		t.Fatalf("acquire cas lock: %v", err)
	}
	defer unlock()
	return adr0007StatLock(t, root)
}

func adr0007StatLock(t *testing.T, root *Root) syscall.Stat_t {
	t.Helper()
	var st unix.Stat_t
	if err := unix.Stat(filepath.Join(root.Path(), ".lsp-trace-candidate-publication.lock"), &st); err != nil {
		t.Fatalf("stat lock: %v", err)
	}
	return syscall.Stat_t{Dev: st.Dev, Ino: st.Ino, Nlink: st.Nlink, Uid: st.Uid, Mode: st.Mode}
}

func TestADR0007StableCASLockPreFlockMetadataPolicy(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*testing.T, string)
	}{
		{"symlink", func(t *testing.T, p string) {
			_ = os.Remove(p)
			if err := os.Symlink("target", p); err != nil {
				t.Fatal(err)
			}
		}},
		{"directory", func(t *testing.T, p string) {
			_ = os.Remove(p)
			if err := os.MkdirAll(p, 0o700); err != nil {
				t.Fatal(err)
			}
		}},
		{"hardlink", func(t *testing.T, p string) {
			if err := os.Link(p, p+".link"); err != nil {
				t.Skipf("hardlink not supported: %v", err)
			}
		}},
		{"group-writable", func(t *testing.T, p string) {
			if err := os.Chmod(p, 0o620); err != nil {
				t.Fatal(err)
			}
		}},
		{"world-writable", func(t *testing.T, p string) {
			if err := os.Chmod(p, 0o602); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, root := boundRoot(t)
			_ = adr0007AcquireLockIdentity(t, root)
			p := filepath.Join(root.Path(), ".lsp-trace-candidate-publication.lock")
			tc.setup(t, p)
			unlock, err := root.casLock(context.Background(), "current.json")
			if err == nil {
				unlock()
				t.Fatalf("ASSERT_ADR0007_CAS_LOCK_PREFLOCK_POLICY_REJECTS_%s", strings.ToUpper(strings.ReplaceAll(tc.name, "-", "_")))
			}
		})
	}
}
