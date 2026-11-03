//go:build darwin || linux

package privatedescribeprep

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// Umask is process-global. Only this owned child changes it, never the parent
// test process. Run the original public Exchange unsafe-mode assertion.
func TestAutomaticHandoffModeBarrier(t *testing.T) {
	for _, mask := range []string{"022", "077"} {
		t.Run(mask, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.v", "-test.run=^TestAutomaticHandoffModeBarrierChild$/^mode")
			cmd.Env = append(os.Environ(), "OWNED_HANDOFF_MODE_CHILD="+mask)
			raw, err := cmd.CombinedOutput()
			t.Logf("owned child umask=%s\n%s", mask, raw)
			if err != nil {
				t.Fatalf("ASSERT_MODE_BARRIER A-fail umask=%s child=%v", mask, err)
			}
			t.Logf("ASSERT_MODE_BARRIER A-pass umask=%s", mask)
		})
	}
}

func TestAutomaticHandoffModeBarrierChild(t *testing.T) {
	mask := os.Getenv("OWNED_HANDOFF_MODE_CHILD")
	if mask == "" {
		return
	}
	if mask == "022" {
		syscall.Umask(0022)
	} else if mask == "077" {
		syscall.Umask(0077)
	} else {
		t.Fatal("invalid owned mask")
	}
	TestAutomaticHandoffPendingUnsafe(t)
	t.Run("mode-positive", func(t *testing.T) {
		h, i, _ := handoffTest(t, 1)
		r := AutomaticRequestPrivateV2{Protocol: AutomaticProtocolPrivateV2, Stage: "NAVIGATE", PreparationSHA256: h.state.sha}
		raw := handoffFixtureResponse(i, r, 4096, []byte("[]"))
		done := make(chan error, 1)
		var held *os.File
		name := filepath.Join(i.shared.cfg.Directory, "response-000001.json")
		go func() {
			deadline := time.Now().Add(time.Second)
			for {
				if _, err := os.Stat(filepath.Join(i.shared.cfg.Directory, "request-000001.json")); err == nil {
					break
				}
				if time.Now().After(deadline) {
					done <- fmt.Errorf("request absent")
					return
				}
				time.Sleep(time.Millisecond)
			}
			var err error
			held, err = publishHandoffModeFixture(name, raw, 0600)
			done <- err
		}()
		payload, err := i.Exchange(context.Background(), r, 4096)
		if he := <-done; he != nil {
			t.Fatal(he)
		}
		defer held.Close()
		logHandoffModeFixture(t, held, raw)
		if he := verifyHandoffModeFixture(held, name, 0600, len(raw)); he != nil {
			t.Fatal(he)
		}
		if err != nil || string(payload) != "[]" || i.shared.failed || i.shared.sequence != 1 {
			t.Fatalf("ASSERT_MODE_POSITIVE A-fail %v", err)
		}
		t.Log("ASSERT_MODE_POSITIVE A-pass")
	})
}

// Prepare whole bytes and explicit mode OFF the response name. The only
// publication is a no-replace rename, never a visible chmod or two-link file.
func publishHandoffModeFixture(name string, raw []byte, mode os.FileMode) (_ *os.File, err error) {
	f, err := os.OpenFile(name+".owned-mode", os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			f.Close()
		}
	}()
	if n, e := f.Write(raw); e != nil || n != len(raw) {
		return nil, fmt.Errorf("fixture write n=%d: %v", n, e)
	}
	if err = f.Chmod(mode); err != nil {
		return nil, err
	}
	if err = f.Sync(); err != nil {
		return nil, err
	}
	if err = verifyHandoffModeFixture(f, name+".owned-mode", mode, len(raw)); err != nil {
		return nil, err
	}
	if err = renameHandoffModeNoReplace(name+".owned-mode", name); err != nil {
		return nil, err
	}
	if err = verifyHandoffModeFixture(f, name, mode, len(raw)); err != nil {
		return nil, err
	}
	return f, nil
}

func logHandoffModeFixture(t *testing.T, f *os.File, raw []byte) {
	t.Helper()
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	s := info.Sys().(*syscall.Stat_t)
	t.Logf("MODE_FIXTURE mode=%04o inode=%d uid=%d nlink=%d size=%d payload_sha256=%x", info.Mode().Perm(), s.Ino, s.Uid, s.Nlink, info.Size(), sha256.Sum256(raw))
}

func verifyHandoffModeFixture(f *os.File, name string, mode os.FileMode, size int) error {
	fd, err := f.Stat()
	if err != nil {
		return err
	}
	visible, err := os.Lstat(name)
	if err != nil {
		return err
	}
	s, ok := fd.Sys().(*syscall.Stat_t)
	if !ok || !fd.Mode().IsRegular() || fd.Mode().Perm() != mode || s.Uid != uint32(os.Getuid()) || s.Nlink != 1 || fd.Size() != int64(size) || !os.SameFile(fd, visible) || fd.Mode() != visible.Mode() || fd.Size() != visible.Size() {
		return errors.New("mode fixture identity/metadata mismatch")
	}
	return nil
}
