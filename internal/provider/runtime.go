// Package provider owns transport-only execution of host-provisioned external providers.
package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Registration struct {
	ID   string
	Path string
	Args []string
	Dir  string
	Env  []string
}

type Limits struct {
	RequestBytes     int
	ResponseBytes    int
	ProtocolMessages int
	StderrBytes      int
	WallTime         time.Duration
	TerminationGrace time.Duration
}

type FailureKind string

const (
	ProviderUnknown     FailureKind = "PROVIDER_UNKNOWN"
	ProviderUnavailable FailureKind = "PROVIDER_UNAVAILABLE"
	StartFailed         FailureKind = "START_FAILED"
	ProtocolFailed      FailureKind = "PROTOCOL_FAILED"
	LimitExceeded       FailureKind = "LIMIT_EXCEEDED"
	Canceled            FailureKind = "CANCELED"
	TimedOut            FailureKind = "TIMED_OUT"
	AbnormalExit        FailureKind = "ABNORMAL_EXIT"
)

type Failure struct {
	Kind   FailureKind
	Reason string
	Err    error
}

type StderrReceipt struct {
	Bytes      []byte
	TotalBytes int64
	Truncated  bool
}
type Receipt struct {
	ProviderID string
	Response   json.RawMessage
	Messages   int
	Stderr     StderrReceipt
	ExitCode   int
	Terminated bool
	Reaped     bool
	Failure    *Failure
}

type Registry struct {
	mu        sync.RWMutex
	providers map[string]Registration
}

func NewRegistry() *Registry { return &Registry{providers: make(map[string]Registration)} }
func (r *Registry) Register(reg Registration) error {
	if r == nil {
		return errors.New("provider: nil registry")
	}
	if reg.ID == "" {
		return errors.New("provider: empty provider id")
	}
	if reg.Path == "" {
		return errors.New("provider: empty provider path")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.providers[reg.ID]; exists {
		return fmt.Errorf("provider: duplicate provider %q", reg.ID)
	}
	r.providers[reg.ID] = cloneRegistration(reg)
	return nil
}
func (r *Registry) Resolve(id string) (Registration, bool) {
	if r == nil {
		return Registration{}, false
	}
	r.mu.RLock()
	reg, ok := r.providers[id]
	r.mu.RUnlock()
	return cloneRegistration(reg), ok
}
func cloneRegistration(reg Registration) Registration {
	reg.Args = append([]string(nil), reg.Args...)
	reg.Env = append([]string(nil), reg.Env...)
	return reg
}

type Runtime struct{ registry *Registry }

func NewRuntime(registry *Registry) *Runtime { return &Runtime{registry: registry} }

func (r *Runtime) Execute(parent context.Context, providerID string, request json.RawMessage, limits Limits) Receipt {
	receipt := Receipt{ProviderID: providerID, ExitCode: -1}
	reg, ok := r.registry.Resolve(providerID)
	if !ok {
		receipt.Failure = failure(ProviderUnknown, "provider is not host-registered", nil)
		return receipt
	}
	if limits.RequestBytes < 0 || len(request) > limits.RequestBytes {
		receipt.Failure = failure(LimitExceeded, "request byte limit exceeded", nil)
		return receipt
	}
	if limits.ResponseBytes < 0 || limits.ProtocolMessages < 1 || limits.StderrBytes < 0 {
		receipt.Failure = failure(LimitExceeded, "invalid or exhausted transport limit", nil)
		return receipt
	}
	if limits.WallTime <= 0 {
		receipt.Failure = failure(LimitExceeded, "wall time limit must be positive", nil)
		return receipt
	}
	if err := parent.Err(); err != nil {
		receipt.Failure = failure(Canceled, "caller canceled", err)
		return receipt
	}

	ctx, cancel := context.WithTimeout(parent, limits.WallTime)
	defer cancel()
	cmd := exec.Command(reg.Path, reg.Args...)
	cmd.Dir = reg.Dir
	if reg.Env != nil {
		cmd.Env = append([]string(nil), reg.Env...)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		receipt.Failure = failure(StartFailed, "stdin pipe", err)
		return receipt
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		receipt.Failure = failure(StartFailed, "stdout pipe", err)
		return receipt
	}
	stderr := &stderrCounter{limit: limits.StderrBytes}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		kind := StartFailed
		if errors.Is(err, os.ErrNotExist) {
			kind = ProviderUnavailable
		}
		receipt.Failure = failure(kind, "provider start", err)
		return receipt
	}

	frame := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(request), request)
	writeErr := make(chan error, 1)
	go func() {
		_, err := io.WriteString(stdin, frame)
		closeErr := stdin.Close()
		writeErr <- errors.Join(err, closeErr)
	}()
	stdoutResult := make(chan readResult, 1)
	go func() {
		b, err := io.ReadAll(io.LimitReader(stdout, int64(limits.ResponseBytes)+8193))
		stdoutResult <- readResult{bytes: b, err: err}
	}()
	var raw []byte
	var waitErr error
	var stdinErr error
	var waitResult chan error
	var killTimer *time.Timer
	waitStarted := false
	ctxDone := ctx.Done()
	// os/exec requires all StdoutPipe reads to complete before Wait. Start Wait
	// only after stdout and stdin ownership have settled. Cancellation signals
	// the process directly and schedules kill escalation so those operations
	// unblock without racing Wait against ReadAll.
	for stdoutResult != nil || writeErr != nil || waitResult != nil || !waitStarted {
		if stdoutResult == nil && writeErr == nil && !waitStarted {
			waitResult = make(chan error, 1)
			waitStarted = true
			go func(ch chan<- error) { ch <- cmd.Wait() }(waitResult)
		}
		select {
		case result := <-stdoutResult:
			raw, err = result.bytes, result.err
			stdoutResult = nil
		case stdinErr = <-writeErr:
			writeErr = nil
		case waitErr = <-waitResult:
			waitResult = nil
			if killTimer != nil {
				killTimer.Stop()
			}
		case <-ctxDone:
			if waitResult != nil {
				receipt.Terminated = terminate(cmd, waitResult, limits.TerminationGrace, &waitErr)
				waitResult = nil
			} else if signalErr := cmd.Process.Signal(os.Interrupt); !errors.Is(signalErr, os.ErrProcessDone) {
				receipt.Terminated = true
				grace := limits.TerminationGrace
				if grace <= 0 {
					grace = 20 * time.Millisecond
				}
				killTimer = time.AfterFunc(grace, func() { _ = cmd.Process.Kill() })
			}
			if receipt.Terminated {
				if errors.Is(parent.Err(), context.Canceled) {
					receipt.Failure = failure(Canceled, "caller canceled", parent.Err())
				} else {
					receipt.Failure = failure(TimedOut, "wall time exceeded", ctx.Err())
				}
			}
			ctxDone = nil
		}
	}
	if err == nil {
		err = stdinErr
	}
	receipt.Reaped = true
	receipt.Stderr = stderr.receipt()
	setExit(&receipt, cmd)
	if receipt.Failure != nil {
		return receipt
	}
	if err != nil {
		receipt.Failure = failure(ProtocolFailed, "transport I/O", err)
		return receipt
	}
	body, messages, parseFailure := parseSingleFrame(raw, limits.ResponseBytes, limits.ProtocolMessages)
	receipt.Messages = messages
	if waitErr != nil {
		receipt.Failure = failure(AbnormalExit, "provider exited abnormally", waitErr)
		return receipt
	}
	if parseFailure != nil {
		receipt.Failure = parseFailure
		return receipt
	}
	receipt.Response = body
	return receipt
}

type readResult struct {
	bytes []byte
	err   error
}

func failure(kind FailureKind, reason string, err error) *Failure {
	return &Failure{Kind: kind, Reason: reason, Err: err}
}
func setExit(r *Receipt, cmd *exec.Cmd) {
	if cmd.ProcessState != nil {
		r.ExitCode = cmd.ProcessState.ExitCode()
	}
}
func terminate(cmd *exec.Cmd, waited <-chan error, grace time.Duration, waitErr *error) bool {
	if grace <= 0 {
		grace = 20 * time.Millisecond
	}
	if errors.Is(cmd.Process.Signal(os.Interrupt), os.ErrProcessDone) {
		// Wait may have completed before its result was published.
		*waitErr = <-waited
		return false
	}
	timer := time.NewTimer(grace)
	defer timer.Stop()
	select {
	case *waitErr = <-waited:
		return true
	case <-timer.C:
		_ = cmd.Process.Kill()
	}
	*waitErr = <-waited
	return true
}

func parseSingleFrame(raw []byte, responseLimit, messageLimit int) (json.RawMessage, int, *Failure) {
	if len(raw) > responseLimit+8192 {
		return nil, 0, failure(LimitExceeded, "response byte limit exceeded", nil)
	}
	r := bufio.NewReader(bytes.NewReader(raw))
	line, err := r.ReadString('\n')
	if err != nil || !strings.HasSuffix(line, "\r\n") {
		return nil, 0, failure(ProtocolFailed, "malformed Content-Length header", err)
	}
	parts := strings.Split(strings.TrimSuffix(line, "\r\n"), ": ")
	if len(parts) != 2 || parts[0] != "Content-Length" {
		return nil, 0, failure(ProtocolFailed, "malformed Content-Length header", nil)
	}
	n, err := strconv.Atoi(parts[1])
	if err != nil || n < 0 {
		return nil, 0, failure(ProtocolFailed, "invalid Content-Length", err)
	}
	if n > responseLimit {
		return nil, 0, failure(LimitExceeded, "response byte limit exceeded", nil)
	}
	blank, err := r.ReadString('\n')
	if err != nil || blank != "\r\n" {
		return nil, 0, failure(ProtocolFailed, "malformed frame separator", err)
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, 0, failure(ProtocolFailed, "truncated response", err)
	}
	if r.Buffered() != 0 {
		return nil, 2, failure(ProtocolFailed, "multiple messages or trailing bytes", nil)
	}
	if messageLimit < 1 {
		return nil, 1, failure(LimitExceeded, "protocol message limit exceeded", nil)
	}
	if !json.Valid(body) {
		return nil, 1, failure(ProtocolFailed, "response is not valid JSON", nil)
	}
	return json.RawMessage(body), 1, nil
}

type stderrCounter struct {
	mu    sync.Mutex
	limit int
	bytes []byte
	total int64
}

func (s *stderrCounter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.total += int64(len(p))
	remaining := s.limit - len(s.bytes)
	if remaining > 0 {
		n := len(p)
		if n > remaining {
			n = remaining
		}
		s.bytes = append(s.bytes, p[:n]...)
	}
	return len(p), nil
}
func (s *stderrCounter) receipt() StderrReceipt {
	s.mu.Lock()
	defer s.mu.Unlock()
	return StderrReceipt{Bytes: append([]byte(nil), s.bytes...), TotalBytes: s.total, Truncated: s.total > int64(len(s.bytes))}
}
