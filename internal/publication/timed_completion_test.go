//go:build darwin || linux

package publication

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"
)

func TestTimedCompletionKilledWriter(t *testing.T) {
	for _, mode := range []string{"candidate-only", "late-candidate", "timely"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			if e := os.Chmod(dir, 0700); e != nil {
				t.Fatal(e)
			}
			exe, e := os.Executable()
			if e != nil {
				t.Fatal(e)
			}
			deadline := time.Now().Add(2 * time.Second)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, exe, "-test.run=^TestTimedCompletionChild$", "-test.count=1")
			cmd.Env = append(os.Environ(), "TIMED_COMPLETION_CHILD="+mode, "TIMED_COMPLETION_ROOT="+dir, "TIMED_COMPLETION_DEADLINE="+strconv.FormatInt(deadline.UnixNano(), 10))
			output, e := cmd.StdoutPipe()
			if e != nil {
				t.Fatal(e)
			}
			input, e := cmd.StdinPipe()
			if e != nil {
				t.Fatal(e)
			}
			defer input.Close()
			var diagnostic bytes.Buffer
			cmd.Stderr = &diagnostic
			if e = cmd.Start(); e != nil {
				t.Fatal(e)
			}
			line, e := bufio.NewReader(output).ReadString('\n')
			if e != nil || line != "COMMITTED\n" {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
				t.Fatalf("child handoff %q %v %s", line, e, diagnostic.String())
			}
			// A real OS kill prevents all post-candidate cleanup or sealing in this writer.
			if e = cmd.Process.Kill(); e != nil {
				t.Fatal(e)
			}
			if e = cmd.Wait(); e == nil {
				t.Fatal("writer was not killed")
			}
			if delay := time.Until(deadline); delay > 0 {
				timer := time.NewTimer(delay)
				defer timer.Stop()
				select {
				case <-timer.C:
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
			}
			root, e := OpenRoot(dir)
			if e != nil {
				t.Fatal(e)
			}
			defer root.Close()
			candidate, e := ReadVerifiedBoundFileChecked(root, "completion", 4096)
			if e != nil || string(candidate) != "completed semantic binding" {
				t.Fatal("candidate not retained", e)
			}
			e = VerifyTimedCompletion(root, "completion", candidate, deadline)
			if mode == "timely" {
				if e != nil {
					t.Fatal("ASSERT_TIMELY_REOPEN_AFTER_EXPIRY: FAIL", e)
				}
				t.Log("ASSERT_TIMELY_REOPEN_AFTER_EXPIRY: PASS")
			} else {
				if e == nil {
					t.Fatal("ASSERT_KILLED_UNCONFIRMED_REFUSED: FAIL")
				}
				t.Log("ASSERT_KILLED_UNCONFIRMED_REFUSED: PASS", mode)
			}
		})
	}
}
func TestTimedCompletionChild(t *testing.T) {
	mode := os.Getenv("TIMED_COMPLETION_CHILD")
	if mode == "" {
		t.Skip("subprocess only")
	}
	nanos, e := strconv.ParseInt(os.Getenv("TIMED_COMPLETION_DEADLINE"), 10, 64)
	if e != nil {
		t.Fatal(e)
	}
	deadline := time.Unix(0, nanos)
	root, e := OpenRoot(os.Getenv("TIMED_COMPLETION_ROOT"))
	if e != nil {
		t.Fatal(e)
	}
	defer root.Close()
	binding := []byte("completed semantic binding")
	if mode == "late-candidate" {
		// Materialize the exact failure cutpoint using real production CAS, not a
		// production fixture hook. The candidate commits after its semantic deadline.
		timer := time.NewTimer(max(time.Until(deadline)+time.Millisecond, 0))
		<-timer.C
		e = prepareTimedCompletion(context.Background(), root, "completion", binding)
	} else if mode == "timely" {
		e = PublishTimedCompletion(context.Background(), root, "completion", binding, deadline)
	} else {
		e = prepareTimedCompletion(context.Background(), root, "completion", binding)
	}
	if e != nil {
		t.Fatal(e)
	}
	fmt.Println("COMMITTED")
	// Parent kills here. No receipt or cleanup is executed for candidate-only cuts.
	_, _ = io.Copy(io.Discard, os.Stdin)
	t.Fatal("parent did not kill writer")
}
func TestTimedCompletionRejectsLateProof(t *testing.T) {
	dir := t.TempDir()
	if e := os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	root, e := OpenRoot(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer root.Close()
	binding := []byte("completed semantic binding")
	deadline := time.Now().Add(time.Second)
	if e = prepareTimedCompletion(context.Background(), root, "completion", binding); e != nil {
		t.Fatal(e)
	}
	proof := timedCompletion{"lsp-trace.timed-completion.v1", "completion", DigestForTest(binding), len(binding), deadline.UnixNano(), deadline.UnixNano()}
	raw, _ := json.Marshal(proof)
	if _, e = PublishBoundFile(root, "completion.receipt", raw, func([]byte) error { return nil }); e != nil {
		t.Fatal(e)
	}
	if VerifyTimedCompletion(root, "completion", binding, deadline) == nil {
		t.Fatal("late verification accepted")
	}
	t.Log("ASSERT_LATE_VERIFICATION_PROOF_REFUSED: PASS")
}
