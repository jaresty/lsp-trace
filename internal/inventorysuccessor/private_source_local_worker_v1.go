//go:build darwin || linux

package inventorysuccessor

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"lsp-trace/internal/describeworker"
)

// localOwnedPipeV1 is caller-goroutine ownership, not exec's child-side FD copy.
// Drainers only Read the *os.File. Each underlying parent FD Close is attempted
// once; every first-close error remains in the named result, including early exits.
// Four close attempts are already in the existing process/FD quote; replacing
// duplicate raw Close calls adds no call credit or funding capacity.
type localOwnedPipeV1 struct {
	f         *os.File
	role      string
	io        privateLocalIOV1
	attempted bool
	err       error
}

func (p *localOwnedPipeV1) close() error {
	if p.attempted {
		return nil
	}
	p.attempted = true
	p.err = p.f.Close()
	if observer, ok := p.io.(interface{ observeEffect(string) error }); ok {
		p.err = errors.Join(p.err, observer.observeEffect("pipe-close:"+p.role))
	}
	return p.err
}
func (p *localOwnedPipeV1) finish(result *error) {
	e := p.close()
	*result = errors.Join(*result, e)
	if p.err != nil && !errors.Is(*result, p.err) {
		*result = errors.Join(*result, p.err)
	}
}

// ExecuteWorker consumes the configured process operation exactly once while P
// remains pending. Only the operator supplies its opaque-admitted original prompt.
// Returned bytes confer no admission; CompleteSourceHost must still bind them.
func (p PendingLocalSourceRV1) ExecuteWorker(ctx context.Context, prompt string) (out []byte, err error) {
	if p.state == nil {
		return nil, ErrPrivateSourceLocalV1
	}
	s := p.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.localLive() || !s.verified || s.localWorkerStarted || s.local.Config.AdapterIdentity != PrivateSourceLocalWorkerAdapterV1 || ctx == nil || ctx.Err() != nil {
		return nil, ErrPrivateSourceLocalV1
	}
	s.localWorkerStarted = true
	phase := "prepare"
	defer func() {
		if err != nil {
			if s.localManagedUse != nil {
				err = fmt.Errorf("managed worker %s: %w", phase, err)
			}
			s.localFailure = errors.Join(s.localFailure, err)
			out = nil
		}
	}()
	c := s.localStagedConfig
	l := c.Limits
	stageBytes, stageErr := localStageBytesV1(*s.local)
	if stageErr != nil || s.localStageDir == "" || len(prompt) == 0 || len(prompt) > l.TempBytes || uint64(len(prompt))+stageBytes > uint64(l.TempBytes) || len(prompt) > l.WorkBytes {
		return nil, ErrPrivateSourceLocalV1
	}
	end := time.Now().Add(time.Duration(l.TimeoutMS) * time.Millisecond)
	if s.request.Deadline.Before(end) {
		end = s.request.Deadline
	}
	if d, ok := ctx.Deadline(); ok && d.Before(end) {
		end = d
	}
	// Cleanup gets only the tail of the original operation window, never renewal.
	if time.Until(end) <= 50*time.Millisecond {
		return nil, ErrPrivateSourceLocalV1
	}
	if s.localManagedUse != nil {
		ctx = s.execution
	}
	whole, stop := context.WithDeadline(ctx, end)
	defer stop()
	run, stopRun := context.WithDeadline(whole, end.Add(-50*time.Millisecond))
	defer stopRun()
	// All artifact paths refer to read-only copies produced from verified FDs.
	dir := s.localStageDir
	var e error
	defer func() {
		// Never recursively enumerate or delete worker-controlled files. Unexpected
		// scratch contents make this fixed cleanup fail and retain PENDING.
		promptErr := s.removeLocalPromptV1()
		err = errors.Join(err, promptErr)
		if whole.Err() != nil || !s.localLive() {
			err = errors.Join(err, ErrPrivateSourceLocalV1)
		}
	}()
	path := filepath.Join(dir, "prompt.txt")
	if e = s.validateLocalStageV1(); e != nil {
		return nil, e
	}
	f, e := openLocalStageFileV1(s.localStage.dir, "prompt.txt")
	if e != nil {
		return nil, e
	}
	n, we := f.WriteString(prompt)
	ce := f.Close()
	if we != nil || ce != nil || n != len(prompt) {
		return nil, errors.Join(we, ce, io.ErrShortWrite)
	}
	stdout, stdoutW, e := os.Pipe()
	if e != nil {
		return nil, e
	}
	stdoutOwner := &localOwnedPipeV1{f: stdout, role: "stdout-read", io: s.local.io}
	defer stdoutOwner.finish(&err)
	stdoutWOwner := &localOwnedPipeV1{f: stdoutW, role: "stdout-write", io: s.local.io}
	defer stdoutWOwner.finish(&err)
	stderr, stderrW, e := os.Pipe()
	if e != nil {
		return nil, e
	}
	stderrOwner := &localOwnedPipeV1{f: stderr, role: "stderr-read", io: s.local.io}
	defer stderrOwner.finish(&err)
	stderrWOwner := &localOwnedPipeV1{f: stderrW, role: "stderr-write", io: s.local.io}
	defer stderrWOwner.finish(&err)
	// Package-private observation only; the sole implementation is test code.
	if observer, ok := s.local.io.(interface{ observeLocalPipeFD(string, *os.File) error }); ok {
		for _, item := range []struct {
			name string
			f    *os.File
		}{{"stdout-read", stdout}, {"stdout-write", stdoutW}, {"stderr-read", stderr}, {"stderr-write", stderrW}} {
			if e = observer.observeLocalPipeFD(item.name, item.f); e != nil {
				return nil, e
			}
		}
	}
	if observer, ok := s.local.io.(interface{ observeEffect(string) error }); ok {
		if e = observer.observeEffect("managed-pipes-created"); e != nil {
			return nil, e
		}
	}
	args := []string{"-f", c.SandboxProfile.Path, "-D", "ARTIFACT_ROOT=" + dir, c.Worker.Path, "-model", c.Model.Path, "-lib", filepath.Dir(c.Library.Path), "-prompt-file", path, "-grammar-file", c.Grammar.Path, "-timeout", time.Until(end).String(), "-max-tokens", strconv.Itoa(l.MaxTokens), "-context-tokens", strconv.Itoa(l.ContextTokens)}
	if s.localManagedUse != nil {
		if e := s.localManagedUse.check(); e != nil {
			return nil, e
		}
		args = append([]string{"-f", c.SandboxProfile.Path, "-D", "ARTIFACT_ROOT=" + s.localManagedUse.g.stage.localStageDir, "-D", "ATTEMPT_ROOT=" + dir}, args[4:]...)
	}
	cmd := exec.Command(c.SandboxExecutable.Path, args...)
	cmd.Dir = dir
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + dir, "TMPDIR=" + dir, "LANG=C", "LC_ALL=C"}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = stdoutW
	cmd.Stderr = stderrW
	// Path-based exec is admitted only while its entire no-follow ancestry and
	// retained stage identities still match. Other UIDs cannot mutate this chain.
	if e = s.validateLocalStageV1(); e != nil {
		return nil, e
	}
	phase = "start"
	if s.localManagedUse != nil {
		if observer, ok := s.local.io.(interface{ observeEffect(string) error }); ok {
			if e = observer.observeEffect("managed-before-start"); e != nil {
				return nil, e
			}
		}
	}
	if s.localManagedUse != nil {
		e = s.localManagedUse.start(run, cmd)
	} else {
		e = cmd.Start()
	}
	if e != nil {
		return nil, e
	}
	phase = "drain"
	closeW := errors.Join(stdoutWOwner.close(), stderrWOwner.close())
	if observer, ok := s.local.io.(interface{ observeEffect(string) error }); ok {
		closeW = errors.Join(closeW, observer.observeEffect("worker-start"))
	}
	kill := func() error {
		e := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(e, syscall.ESRCH) {
			return nil
		}
		return e
	}
	type drained struct {
		b   []byte
		err error
	}
	drain := func(f *os.File, limit int, ch chan<- drained) {
		// Exact configured buffer, plus a rejecting byte. Every read, including EOF,
		// is charged before syscall; positive fragmentation can refuse early.
		b := make([]byte, limit+1)
		used := 0
		calls, e := localReadCallBoundV1(uint64(limit))
		if e != nil {
			ch <- drained{nil, e}
			return
		}
		for {
			if run.Err() != nil || calls == 0 {
				ch <- drained{nil, fmt.Errorf("%w: prepaid worker descriptor read-call/deadline ceiling", ErrPrivateSourceLocalV1)}
				return
			}
			calls--
			n, e := f.Read(b[used:])
			if n < 0 || n > len(b)-used {
				ch <- drained{nil, ErrPrivateSourceLocalV1}
				return
			}
			used += n
			if used > limit {
				ch <- drained{nil, ErrPrivateSourceLocalV1}
				return
			}
			if e == io.EOF {
				ch <- drained{b[:used], nil}
				return
			}
			if e != nil || n == 0 {
				ch <- drained{nil, errors.Join(e, ErrPrivateSourceLocalV1)}
				return
			}
		}
	}
	oc, ec := make(chan drained, 1), make(chan drained, 1)
	go drain(stdout, l.StdoutBytes, oc)
	go drain(stderr, l.StderrBytes, ec)
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	var ob, eb drained
	var waitErr error
	ow, ew, ww := false, false, false
	failed := closeW
	for !ow || !ew || !ww {
		select {
		case ob = <-oc:
			ow = true
			oc = nil
			failed = errors.Join(failed, ob.err)
		case eb = <-ec:
			ew = true
			ec = nil
			failed = errors.Join(failed, eb.err)
		case waitErr = <-waited:
			ww = true
			waited = nil
			failed = errors.Join(failed, waitErr)
		case <-run.Done():
			failed = errors.Join(failed, run.Err())
		}
		if failed != nil {
			failed = errors.Join(failed, kill(), stdoutOwner.close(), stderrOwner.close())
			// Channels are buffered; even uncertain teardown cannot block its producers.
			for !ow || !ew || !ww {
				select {
				case <-oc:
					ow = true
					oc = nil
				case <-ec:
					ew = true
					ec = nil
				case <-waited:
					ww = true
					waited = nil
				case <-whole.Done():
					return nil, errors.Join(failed, whole.Err())
				}
			}
			return nil, failed
		}
	}
	// A child retaining a pipe prevents the loop completing; the original run
	// cutoff above kills the whole process group and refuses, not detached success.
	// After reaping the direct child and observing EOF on both owned pipes, a
	// still-existing process group means descendants remain unjoined. Signal
	// them, but withhold success even if signalling succeeds; a signal is not a
	// death/reap receipt. No fresh teardown deadline is created.
	if groupErr := syscall.Kill(-cmd.Process.Pid, 0); !errors.Is(groupErr, syscall.ESRCH) {
		return nil, errors.Join(ErrPrivateSourceLocalV1, groupErr, kill())
	}
	if e = errors.Join(stdoutOwner.close(), stderrOwner.close()); e != nil {
		return nil, e
	}
	if whole.Err() != nil || !s.localLive() {
		return nil, ErrPrivateSourceLocalV1
	}
	if observer, ok := s.local.io.(interface{ observeEffect(string) error }); ok {
		if e = observer.observeEffect("worker-joined"); e != nil {
			return nil, e
		}
	}
	phase = "stage-check"
	if e = s.checkLocalStageV1(); e != nil {
		return nil, e
	}
	if s.localManagedUse != nil {
		if e := s.localManagedUse.check(); e != nil {
			return nil, e
		}
	}
	framed, e := frameLocalWorkerStdoutV1(ob.b, localWorkerFrameCapV1(l.StdoutBytes))
	if e != nil {
		return nil, e
	}
	if _, e = describeworker.ParseSuppliedTerminalResponse(framed, describeworker.SuppliedResponseLimits{MaxAdapterBytes: localWorkerFrameCapV1(l.StdoutBytes), MaxSemanticBytes: l.WorkBytes}); e != nil {
		return nil, e
	}
	s.localWorkerObservation = PrivateSourceLocalWorkerObservationV1{RawStdoutSHA256: fmt.Sprintf("sha256:%x", sha256.Sum256(ob.b)), RawStdoutBytes: uint64(len(ob.b)), AdapterSHA256: fmt.Sprintf("sha256:%x", sha256.Sum256(framed)), ExecutorContract: s.localExecutorContract}
	return framed, nil
}
