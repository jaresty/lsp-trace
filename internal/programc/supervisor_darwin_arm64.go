//go:build darwin && arm64

package programc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	supervisorPSPath      = "/bin/ps"
	supervisorWall        = SupervisorWall
	supervisorRSSBytes    = SupervisorRSSBytes
	supervisorPoll        = SupervisorPoll
	supervisorOutputBytes = MaxWorkerOutputBytes
)

type cappedBuffer struct {
	mu       sync.Mutex
	b        bytes.Buffer
	limit    int64
	exceeded bool
}

func (w *cappedBuffer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	left := w.limit - int64(w.b.Len())
	if left <= 0 {
		w.exceeded = true
		return len(p), nil
	}
	keep := int64(len(p))
	if keep > left {
		keep = left
		w.exceeded = true
	}
	_, _ = w.b.Write(p[:keep])
	return len(p), nil
}
func (w *cappedBuffer) Bytes() []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]byte(nil), w.b.Bytes()...)
}
func (w *cappedBuffer) String() string { return string(w.Bytes()) }
func (w *cappedBuffer) Exceeded() bool { w.mu.Lock(); defer w.mu.Unlock(); return w.exceeded }

func computeSupervised(ctx context.Context, input []byte, seed uint64) (Outcome, SupervisionObservation, *SupervisionFailure) {
	var zero Outcome
	observation := SupervisionObservation{Ceiling: ObservationCeiling}
	request, err := json.Marshal(workerRequest{Version: workerVersion, Input: input, Seed: seed})
	if err != nil {
		return zero, observation, failure(CodeInternalAdmission, err)
	}
	if int64(len(request)) > MaxWorkerInputBytes {
		return zero, observation, failure(CodeInternalAdmission, errors.New("worker input limit exceeded"))
	}
	path, failed := executablePath()
	if failed != nil {
		return zero, observation, failed
	}
	if st, e := os.Stat(supervisorPSPath); e != nil || st.Mode()&0111 == 0 {
		return zero, observation, failure(CodeMemoryUnsupported, errors.New("/bin/ps unavailable"))
	}
	cmd := exec.Command(path, workerArgument)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return zero, observation, failure(CodeNonzeroExit, err)
	}
	stdout := &cappedBuffer{limit: supervisorOutputBytes}
	stderr := &cappedBuffer{limit: MaxWorkerStderrBytes}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	started := time.Now()
	if err = cmd.Start(); err != nil {
		return zero, observation, failure(CodeNonzeroExit, err)
	}
	pid := cmd.Process.Pid
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	kill := func() {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		select {
		case <-done:
			return
		case <-time.After(time.Second):
			return
		}
	}
	peak, err := initialTreeRSS(pid)
	if err != nil {
		kill()
		return zero, observation, failure(CodeMemoryUnsupported, err)
	}
	observation.PeakTreeRSSBytes = peak
	observation.Samples = 1
	if peak > supervisorRSSBytes {
		kill()
		return zero, observation, failure(CodeMemoryLimit, fmt.Errorf("sampled tree RSS %d exceeds %d", peak, supervisorRSSBytes))
	}
	if _, err = stdin.Write(request); err != nil {
		kill()
		return zero, observation, failure(CodeNonzeroExit, err)
	}
	if err = stdin.Close(); err != nil {
		kill()
		return zero, observation, failure(CodeNonzeroExit, err)
	}
	ticker := time.NewTicker(supervisorPoll)
	defer ticker.Stop()
	timer := time.NewTimer(supervisorWall)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			kill()
			observation.ElapsedNanos = time.Since(started).Nanoseconds()
			return zero, observation, failure(CodeCancelled, ctx.Err())
		case <-timer.C:
			kill()
			observation.ElapsedNanos = time.Since(started).Nanoseconds()
			return zero, observation, failure(CodeTimeout, errors.New("60s wall limit exceeded"))
		case <-ticker.C:
			if stdout.Exceeded() {
				kill()
				observation.ElapsedNanos = time.Since(started).Nanoseconds()
				return zero, observation, failure(CodeOutputLimit, errors.New("worker stdout limit exceeded"))
			}
			rss, e := treeRSS(pid)
			if e != nil {
				kill()
				observation.ElapsedNanos = time.Since(started).Nanoseconds()
				return zero, observation, failure(CodeMemoryUnsupported, e)
			}
			observation.Samples++
			if rss > observation.PeakTreeRSSBytes {
				observation.PeakTreeRSSBytes = rss
			}
			if rss > supervisorRSSBytes {
				kill()
				observation.ElapsedNanos = time.Since(started).Nanoseconds()
				return zero, observation, failure(CodeMemoryLimit, fmt.Errorf("sampled tree RSS %d exceeds %d", rss, supervisorRSSBytes))
			}
		case waitErr := <-done:
			observation.ElapsedNanos = time.Since(started).Nanoseconds()
			if stdout.Exceeded() {
				_ = syscall.Kill(-pid, syscall.SIGKILL)
				return zero, observation, failure(CodeOutputLimit, errors.New("worker stdout limit exceeded"))
			}
			if waitErr != nil {
				_ = syscall.Kill(-pid, syscall.SIGKILL)
				detail := strings.TrimSpace(stderr.String())
				if strings.Contains(detail, "panic:") || strings.Contains(detail, "SIGABRT: abort") {
					return zero, observation, failure(CodePanic, errors.New(detail))
				}
				return zero, observation, failure(CodeNonzeroExit, errors.New(detail))
			}
			var response workerResponse
			if err = strictDecode(bytes.NewReader(stdout.Bytes()), supervisorOutputBytes, &response); err != nil || response.Version != workerVersion || (response.Outcome == nil) == (response.Failure == nil) {
				_ = syscall.Kill(-pid, syscall.SIGKILL)
				return zero, observation, failure(CodeMalformedOutput, errors.New("invalid worker response"))
			}
			if response.Failure != nil {
				return zero, observation, response.Failure
			}
			out, reconstructFailure := reconstruct(input, seed, response.Outcome)
			return out, observation, reconstructFailure
		}
	}
}

func initialTreeRSS(root int) (int64, error) {
	deadline := time.Now().Add(2 * time.Second)
	var err error
	for {
		var rss int64
		rss, err = treeRSS(root)
		if err == nil {
			return rss, nil
		}
		if time.Now().After(deadline) {
			return 0, err
		}
		time.Sleep(time.Millisecond)
	}
}

func treeRSS(root int) (int64, error) {
	out, err := exec.Command(supervisorPSPath, "-axo", "pid=,ppid=,rss=").Output()
	if err != nil {
		return 0, err
	}
	children := map[int][]int{}
	rss := map[int]int64{}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return 0, errors.New("malformed /bin/ps output")
		}
		p, e1 := strconv.Atoi(fields[0])
		pp, e2 := strconv.Atoi(fields[1])
		r, e3 := strconv.ParseInt(fields[2], 10, 64)
		if e1 != nil || e2 != nil || e3 != nil || r < 0 {
			return 0, errors.New("malformed /bin/ps output")
		}
		children[pp] = append(children[pp], p)
		rss[p] = r * 1024
	}
	if _, ok := rss[root]; !ok {
		return 0, errors.New("worker process absent from /bin/ps; valid measurement unavailable")
	}
	seen := map[int]bool{}
	queue := []int{root}
	var total int64
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		if seen[p] {
			continue
		}
		seen[p] = true
		value, ok := rss[p]
		if !ok {
			return 0, errors.New("descendant absent from /bin/ps; valid measurement unavailable")
		}
		if value > int64(^uint64(0)>>1)-total {
			return 0, errors.New("aggregate RSS overflow")
		}
		total += value
		queue = append(queue, children[p]...)
	}
	return total, nil
}
