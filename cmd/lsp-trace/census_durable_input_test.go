//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"errors"
	"golang.org/x/sys/unix"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestCensusDurableHeldOpenInput(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "-test.run=^TestCensusDurableInputChild$", "-test.v")
	cmd.Env = append(os.Environ(), "CENSUS_INPUT_CHILD=1")
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if _, err = in.Write([]byte("{\"partial\":")); err != nil {
		t.Fatal(err)
	}
	// Deliberately keep the writer open until the child has exited.
	err = cmd.Wait()
	if err != nil || !strings.Contains(out.String(), "ASSERT_DURABLE_INPUT_CANCELED_JOINED: PASS") {
		t.Fatal("ASSERT_DURABLE_HELD_PIPE: FAIL", err, out.String())
	}
	t.Log("ASSERT_DURABLE_HELD_PIPE: PASS")
}
func TestCensusDurableInputChild(t *testing.T) {
	if os.Getenv("CENSUS_INPUT_CHILD") == "" {
		t.Skip("subprocess only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := readCensusDurableInput(ctx, os.Stdin)
	if !errors.Is(err, context.DeadlineExceeded) || ctx.Err() != context.DeadlineExceeded {
		t.Fatal("ASSERT_DURABLE_INPUT_CANCELED_JOINED: FAIL", err)
	}
	t.Log("ASSERT_DURABLE_INPUT_CANCELED_JOINED: PASS")
}
func TestCensusDurableInputComplete(t *testing.T) {
	for _, raw := range []string{"bad\n", strings.Repeat("x", (64<<10)+3)} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		got, err := readCensusDurableInput(ctx, strings.NewReader(raw))
		cancel()
		if err != nil || len(got) != min(len(raw), (64<<10)+1) {
			t.Fatal("ASSERT_DURABLE_COMPLETE_INPUT: FAIL", err, len(got))
		}
	}
	t.Log("ASSERT_DURABLE_COMPLETE_INPUT: PASS")
}

// Model the real scheduling race deterministically: the descriptor's absolute
// deadline becomes observable before the context's notification is delivered.
// This is a context scheduling control, not an input-reader production hook.
type delayedInputContext struct {
	context.Context
	deadline time.Time
}

func (c delayedInputContext) Deadline() (time.Time, bool) { return c.deadline, true }
func TestCensusDurableInputDeadlineNotification(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	fd := int(r.Fd())
	before, err := unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0)
	if err != nil {
		t.Fatal(err)
	}
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := delayedInputContext{parent, time.Now().Add(10 * time.Millisecond)}
	timer := time.AfterFunc(60*time.Millisecond, cancel)
	defer timer.Stop()
	_, err = readCensusDurableInput(ctx, r)
	if !errors.Is(err, context.Canceled) || ctx.Err() != context.Canceled {
		t.Fatal("ASSERT_DURABLE_INPUT_NOTIFICATION_JOIN: FAIL", err, ctx.Err())
	}
	after, e := unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0)
	if e != nil || before != after {
		t.Fatal("ASSERT_DURABLE_INPUT_FLAGS: FAIL", before, after, e)
	}
	// Original descriptor remains usable; the owned duplicate/callback no longer
	// changes its flags after return. No reader goroutine was started by the helper.
	if _, e = w.Write([]byte("x")); e != nil {
		t.Fatal(e)
	}
	var b [1]byte
	if _, e = r.Read(b[:]); e != nil || b[0] != 'x' {
		t.Fatal("original descriptor unavailable", e)
	}
	t.Log("ASSERT_DURABLE_INPUT_NOTIFICATION_JOIN: PASS")
	t.Log("ASSERT_DURABLE_INPUT_FLAGS: PASS")
}
