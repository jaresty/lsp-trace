//go:build darwin || linux

package sourcecontinuation

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
	"lsp-trace/internal/publication"
)

// This isolates the persistence seam, not native preparation or semantic acceptance.
func TestDurableMutationDeadline(t *testing.T) {
	for _, stage := range []string{"reserve", "claim", "final", "confirmation", "reopen", "killed-holder"} {
		t.Run(stage, func(t *testing.T) {
			dir, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if err = os.Chmod(dir, 0700); err != nil {
				t.Fatal(err)
			}
			root, err := publication.OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			s := DurableDescribeStoreV1{Root: root}
			h := durableDescribeHeadV1{RunID: strings.Repeat("a", 64), Status: "PENDING", Deadline: time.Now().Add(250 * time.Millisecond).UnixNano(), RunDeadline: time.Now().Add(time.Second).UnixNano()}
			if stage == "final" {
				h.Status = "CLAIMED"
			}
			if stage == "reopen" {
				h.Status = "COMPLETE"
			}
			old, _ := durableSeal(h)
			if err = s.put(durableHeadName(h.RunID), old); err != nil {
				t.Fatal(err)
			}
			exe, _ := os.Executable()
			cmd := exec.Command(exe, "-test.run=^TestDurableDeadlineLockChild$")
			cmd.Env = append(os.Environ(), "DURABLE_DEADLINE_LOCK="+dir)
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			stdin, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err = cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = stdin.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() }()
			line, err := bufio.NewReader(stdout).ReadString('\n')
			if err != nil || line != "LOCKED\n" {
				t.Fatal(line, err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			// Release after the persisted task deadline but before the continuation deadline.
			released := make(chan struct{})
			go func() {
				time.Sleep(time.Until(time.Unix(0, h.Deadline)) + 150*time.Millisecond)
				if stage == "killed-holder" {
					_ = cmd.Process.Kill()
				}
				_ = stdin.Close()
				close(released)
			}()
			var got []byte
			if stage == "reserve" || stage == "claim" {
				_, got, err = s.reserve(ctx, h, old, 1, stage == "claim")
			} else if stage == "confirmation" {
				err = s.confirm(ctx, h)
				if s.confirmed(h) == nil {
					t.Fatal("ASSERT_DURABLE_CONFIRMATION_DEADLINE: FAIL acknowledged late confirmation")
				}
			} else {
				h.Status = "COMPLETE"
				got, err = s.change(ctx, old, h)
			}
			<-released
			if stage == "killed-holder" {
				// The expired mutation remains refused; a new independent mutation
				// can obtain the kernel-released lock, without renewing this task.
				fresh := h
				fresh.RunID = strings.Repeat("b", 64)
				fresh.Status = "COMPLETE"
				before, _ := durableSeal(fresh)
				if e := s.put(durableHeadName(fresh.RunID), before); e != nil {
					t.Fatal(e)
				}
				if _, e := s.change(ctx, before, fresh); e != nil {
					t.Fatal("ASSERT_DURABLE_KILLED_LOCK_RELEASED: FAIL", e)
				}
				t.Log("ASSERT_DURABLE_KILLED_LOCK_RELEASED: PASS")
			}
			marker := "ASSERT_DURABLE_" + strings.ToUpper(stage) + "_DEADLINE"
			if stage == "reopen" {
				if err != nil || len(got) == 0 {
					t.Fatal(marker+": FAIL", err)
				}
			} else if err == nil {
				t.Fatal(marker + ": FAIL accepted late mutation")
			}
			t.Log(marker + ": PASS")
		})
	}
}

func TestDurableDeadlineLockChild(t *testing.T) {
	dir := os.Getenv("DURABLE_DEADLINE_LOCK")
	if dir == "" {
		t.Skip("subprocess only")
	}
	fd, err := unix.Open(filepath.Join(dir, ".lsp-trace-candidate-publication.lock"), unix.O_CREAT|unix.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(fd)
	if err = unix.Flock(fd, unix.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	_, _ = os.Stdout.WriteString("LOCKED\n")
	var b [1]byte
	_, _ = os.Stdin.Read(b[:])
}
