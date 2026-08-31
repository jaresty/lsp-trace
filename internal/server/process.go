package server

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"sync"
	"time"
)

type Process struct {
	Cmd    *exec.Cmd
	Stdin  io.WriteCloser
	Stdout io.ReadCloser
	stderr bytes.Buffer

	closeOnce sync.Once
	killOnce  sync.Once
	waitOnce  sync.Once
	waitDone  chan struct{}
	waitErr   error
}

func Start(ctx context.Context, command string, args []string, env []string) (*Process, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	if len(env) > 0 {
		cmd.Env = append(cmd.Environ(), env...)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	p := &Process{Cmd: cmd, Stdin: stdin, Stdout: stdout, waitDone: make(chan struct{})}
	cmd.Stderr = &p.stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return p, nil
}
func (p *Process) Stderr() string { return p.stderr.String() }
func (p *Process) Stop(grace time.Duration) error {
	p.closeOnce.Do(func() { _ = p.Stdin.Close() })
	p.waitOnce.Do(func() {
		go func() {
			p.waitErr = p.Cmd.Wait()
			close(p.waitDone)
		}()
	})
	timer := time.NewTimer(grace)
	defer timer.Stop()
	select {
	case <-p.waitDone:
		return p.waitErr
	case <-timer.C:
		p.killOnce.Do(func() { _ = p.Cmd.Process.Kill() })
		<-p.waitDone
		return p.waitErr
	}
}
