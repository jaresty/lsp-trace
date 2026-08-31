package server

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"time"
)

type Process struct {
	Cmd    *exec.Cmd
	Stdin  io.WriteCloser
	Stdout io.ReadCloser
	stderr bytes.Buffer
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
	p := &Process{Cmd: cmd, Stdin: stdin, Stdout: stdout}
	cmd.Stderr = &p.stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return p, nil
}
func (p *Process) Stderr() string { return p.stderr.String() }
func (p *Process) Stop(grace time.Duration) error {
	_ = p.Stdin.Close()
	done := make(chan error, 1)
	go func() { done <- p.Cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(grace):
		_ = p.Cmd.Process.Kill()
		return <-done
	}
}
