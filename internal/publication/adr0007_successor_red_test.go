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
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestADR0007CompareAndReplaceBoundFilePostRenameDegradationFailsClosed(t *testing.T) {
	cases := []struct {
		name      string
		configure func(*testing.T, *Root, string, []byte, context.CancelFunc) func([]byte) error
		assert    func(*testing.T, *CompareAndReplaceReceipt, error)
	}{
		{
			name: "directory-sync",
			configure: func(t *testing.T, root *Root, selector string, raw []byte, cancel context.CancelFunc) func([]byte) error {
				testHookBoundFileDirectorySync = func() error { return errors.New("synthetic directory sync failure") }
				return func([]byte) error { return nil }
			},
			assert: func(t *testing.T, receipt *CompareAndReplaceReceipt, err error) {
				if err == nil || receipt == nil || !receipt.Committed || receipt.DirectorySyncStatus == DirectorySyncComplete {
					t.Fatalf("ASSERT_ADR0007_CAS_POST_RENAME_DIRECTORY_SYNC_FAILS_CLOSED: receipt=%+v err=%v", receipt, err)
				}
			},
		},
		{
			name: "close",
			configure: func(t *testing.T, root *Root, selector string, raw []byte, cancel context.CancelFunc) func([]byte) error {
				testHookBoundFileFinalClose = func() error { return errors.New("synthetic final close failure") }
				return func([]byte) error { return nil }
			},
			assert: func(t *testing.T, receipt *CompareAndReplaceReceipt, err error) {
				if err == nil || receipt == nil || !receipt.Committed || receipt.CloseStatus == CloseComplete {
					t.Fatalf("ASSERT_ADR0007_CAS_POST_RENAME_CLOSE_FAILS_CLOSED: receipt=%+v err=%v", receipt, err)
				}
			},
		},
		{
			name: "reread",
			configure: func(t *testing.T, root *Root, selector string, raw []byte, cancel context.CancelFunc) func([]byte) error {
				testHookBoundFileAfterPublish = func() {
					if err := os.WriteFile(filepath.Join(root.Path(), filepath.FromSlash(selector)), []byte("corrupt"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				return func([]byte) error { return nil }
			},
			assert: func(t *testing.T, receipt *CompareAndReplaceReceipt, err error) {
				if err == nil || receipt == nil || !receipt.Committed || receipt.VerificationStatus != "COMMITTED_VERIFICATION_FAILED" {
					t.Fatalf("ASSERT_ADR0007_CAS_POST_RENAME_REREAD_FAILS_CLOSED: receipt=%+v err=%v", receipt, err)
				}
			},
		},
		{
			name: "verifier",
			configure: func(t *testing.T, root *Root, selector string, raw []byte, cancel context.CancelFunc) func([]byte) error {
				calls := 0
				return func([]byte) error {
					calls++
					if calls > 2 {
						return errors.New("synthetic verifier failure")
					}
					return nil
				}
			},
			assert: func(t *testing.T, receipt *CompareAndReplaceReceipt, err error) {
				if err == nil || receipt == nil || !receipt.Committed || receipt.VerificationStatus != "COMMITTED_VERIFICATION_FAILED" {
					t.Fatalf("ASSERT_ADR0007_CAS_POST_RENAME_VERIFIER_FAILS_CLOSED: receipt=%+v err=%v", receipt, err)
				}
			},
		},
		{
			name: "cancel",
			configure: func(t *testing.T, root *Root, selector string, raw []byte, cancel context.CancelFunc) func([]byte) error {
				calls := 0
				return func([]byte) error {
					calls++
					if calls > 2 {
						cancel()
					}
					return nil
				}
			},
			assert: func(t *testing.T, receipt *CompareAndReplaceReceipt, err error) {
				if err == nil || receipt == nil || !receipt.Committed {
					t.Fatalf("ASSERT_ADR0007_CAS_POST_RENAME_CANCEL_FAILS_CLOSED: receipt=%+v err=%v", receipt, err)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetBoundFileHooks(t)
			_, root := boundRoot(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			raw := []byte(`{"generation":"g-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}` + "\n")
			verify := tc.configure(t, root, "current.json", raw, cancel)
			receipt, err := CompareAndReplaceBoundFile(ctx, root, "current.json", BoundFilePredecessor{Absent: true}, raw, verify)
			if got, readErr := root.ReadSelector("current.json", int64(len(raw))+32); readErr != nil || !bytes.Equal(got, raw) {
				t.Fatalf("ASSERT_ADR0007_CAS_POST_RENAME_FIXTURE_COMMITTED_FINAL_BYTES: got=%q readErr=%v receipt=%+v err=%v", got, readErr, receipt, err)
			}
			tc.assert(t, receipt, err)
		})
	}
}

func TestADR0007StableCASLockRetainsInodeAndRejectsSubstitution(t *testing.T) {
	if os.Getenv("LSP_TRACE_ADR0007_LOCK_HELPER") == "hold" {
		adr0007LockHelper()
		return
	}
	_, root := boundRoot(t)
	first := adr0007AcquireLockIdentity(t, root)
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

	held := make(chan struct{})
	cmd := exec.Command(os.Args[0], "-test.run=TestADR0007StableCASLockRetainsInodeAndRejectsSubstitution")
	cmd.Env = append(os.Environ(), "LSP_TRACE_ADR0007_LOCK_HELPER=hold", "LSP_TRACE_ADR0007_LOCK_ROOT="+root.Path())
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		lockPath := filepath.Join(root.Path(), ".lsp-trace-candidate-publication.lock")
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if data, err := os.ReadFile(lockPath + ".held"); err == nil && strings.TrimSpace(string(data)) == "held" {
				close(held)
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	select {
	case <-held:
	case <-time.After(6 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("ASSERT_ADR0007_CAS_LOCK_HELPER_HELD_FLOCK")
	}
	if err := os.Remove(filepath.Join(root.Path(), ".lsp-trace-candidate-publication.lock")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root.Path(), ".lsp-trace-candidate-publication.lock"), []byte("replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	st := adr0007AcquireLockIdentity(t, root)
	if st.Dev != first.Dev || st.Ino != first.Ino {
		t.Fatalf("ASSERT_ADR0007_CAS_LOCK_SUBSTITUTION_FAILS_CLOSED: original=%+v replacement=%+v", first, st)
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	after := adr0007AcquireLockIdentity(t, root)
	if after.Dev != first.Dev || after.Ino != first.Ino {
		t.Fatalf("ASSERT_ADR0007_CAS_LOCK_KILLED_HELPER_RELEASE_RETAINS_INODE: original=%+v after=%+v", first, after)
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

func adr0007AcquireLockIdentity(t *testing.T, root *Root) syscall.Stat_t {
	t.Helper()
	unlock, err := root.casLock(context.Background(), "current.json")
	if err != nil {
		t.Fatalf("acquire cas lock: %v", err)
	}
	defer unlock()
	var st unix.Stat_t
	if err := unix.Stat(filepath.Join(root.Path(), ".lsp-trace-candidate-publication.lock"), &st); err != nil {
		t.Fatalf("stat lock: %v", err)
	}
	return syscall.Stat_t{Dev: st.Dev, Ino: st.Ino, Nlink: st.Nlink, Uid: st.Uid, Mode: st.Mode}
}

func TestADR0007StableCASLockPortableMetadataPolicy(t *testing.T) {
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
				t.Fatalf("ASSERT_ADR0007_CAS_LOCK_POLICY_REJECTS_%s", strings.ToUpper(strings.ReplaceAll(tc.name, "-", "_")))
			}
		})
	}
}

var _ = runtime.GOOS
