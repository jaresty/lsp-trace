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
	supervisorPSPath       = "/bin/ps"
	supervisorWall         = SupervisorWall
	supervisorRSSBytes     = SupervisorRSSBytes
	supervisorPoll         = SupervisorPoll
	supervisorOutputBytes  = MaxWorkerOutputBytes
	supervisorGroupKill    = syscall.Kill
	supervisorGroupState   = processGroupState
	supervisorCleanupNow   = time.Now
	supervisorCleanupWait  = time.Sleep
	supervisorProcessStart = processStart
)

type processGroupIdentity struct {
	PID   int
	PGID  int
	Start string
}

type processGroupMember struct {
	PID   int
	PGID  int
	State string
	Start string
}

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
	decodeWorker := func(waitErr error) (Outcome, *SupervisionFailure) {
		if stdout.Exceeded() {
			return zero, failure(CodeOutputLimit, errors.New("worker stdout limit exceeded"))
		}
		if waitErr != nil {
			return zero, classifyTerminatedWorker(waitErr, stderr.String())
		}
		var response workerResponse
		if decodeErr := strictDecode(bytes.NewReader(stdout.Bytes()), supervisorOutputBytes, &response); decodeErr != nil || response.Version != workerVersion || (response.Outcome == nil) == (response.Failure == nil) {
			return zero, failure(CodeMalformedOutput, errors.New("invalid worker response"))
		}
		if response.Failure != nil {
			return zero, response.Failure
		}
		return reconstruct(input, seed, response.Outcome)
	}
	group, groupErr := establishProcessGroupIdentity(pid)
	if groupErr != nil {
		select {
		case waitErr := <-done:
			out, terminalFailure := decodeWorker(waitErr)
			return out, observation, terminalFailure
		case <-time.After(time.Second):
			return zero, observation, cleanupFailure(failure(CodeMemoryUnsupported, groupErr), groupErr)
		}
	}
	finishWorker := func(waitErr error) (Outcome, *SupervisionFailure) {
		out, terminalFailure := decodeWorker(waitErr)
		if cleanupErr := terminateWorkerGroup(group, nil); cleanupErr != nil {
			return zero, cleanupFailure(terminalFailure, cleanupErr)
		}
		return out, terminalFailure
	}
	stopWorker := func(nominal *SupervisionFailure) *SupervisionFailure {
		if cleanupErr := terminateWorkerGroup(group, done); cleanupErr != nil {
			return cleanupFailure(nominal, cleanupErr)
		}
		return nominal
	}
	peak, err := initialTreeRSS(pid)
	if err != nil {
		if waitErr, completed := awaitTerminatedWorker(done); completed {
			observation.ElapsedNanos = time.Since(started).Nanoseconds()
			if waitErr != nil {
				out, terminalFailure := finishWorker(waitErr)
				return out, observation, terminalFailure
			}
		} else {
			return zero, observation, stopWorker(failure(CodeMemoryUnsupported, err))
		}
		return zero, observation, failure(CodeMemoryUnsupported, err)
	}
	observation.PeakTreeRSSBytes = peak
	observation.Samples = 1
	if peak > supervisorRSSBytes {
		return zero, observation, stopWorker(failure(CodeMemoryLimit, fmt.Errorf("sampled tree RSS %d exceeds %d", peak, supervisorRSSBytes)))
	}
	if _, err = stdin.Write(request); err != nil {
		return zero, observation, stopWorker(failure(CodeNonzeroExit, err))
	}
	if err = stdin.Close(); err != nil {
		return zero, observation, stopWorker(failure(CodeNonzeroExit, err))
	}
	ticker := time.NewTicker(supervisorPoll)
	defer ticker.Stop()
	timer := time.NewTimer(supervisorWall)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			observation.ElapsedNanos = time.Since(started).Nanoseconds()
			return zero, observation, stopWorker(failure(CodeCancelled, ctx.Err()))
		case <-timer.C:
			observation.ElapsedNanos = time.Since(started).Nanoseconds()
			return zero, observation, stopWorker(failure(CodeTimeout, errors.New("60s wall limit exceeded")))
		case <-ticker.C:
			if stdout.Exceeded() {
				observation.ElapsedNanos = time.Since(started).Nanoseconds()
				return zero, observation, stopWorker(failure(CodeOutputLimit, errors.New("worker stdout limit exceeded")))
			}
			rss, e := treeRSS(pid)
			if e != nil {
				if waitErr, completed := awaitTerminatedWorker(done); completed {
					observation.ElapsedNanos = time.Since(started).Nanoseconds()
					out, terminalFailure := finishWorker(waitErr)
					return out, observation, terminalFailure
				}
				observation.ElapsedNanos = time.Since(started).Nanoseconds()
				return zero, observation, stopWorker(failure(CodeMemoryUnsupported, e))
			}
			observation.Samples++
			if rss > observation.PeakTreeRSSBytes {
				observation.PeakTreeRSSBytes = rss
			}
			if rss > supervisorRSSBytes {
				observation.ElapsedNanos = time.Since(started).Nanoseconds()
				return zero, observation, stopWorker(failure(CodeMemoryLimit, fmt.Errorf("sampled tree RSS %d exceeds %d", rss, supervisorRSSBytes)))
			}
		case waitErr := <-done:
			observation.ElapsedNanos = time.Since(started).Nanoseconds()
			out, terminalFailure := finishWorker(waitErr)
			return out, observation, terminalFailure
		}
	}
}

func processGroupState(pgid int) ([]processGroupMember, error) {
	out, err := exec.Command(supervisorPSPath, "-axo", "pid=,pgid=,state=").Output()
	if err != nil {
		return nil, err
	}
	var members []processGroupMember
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 3 {
			return nil, errors.New("malformed /bin/ps process-group output")
		}
		pid, pidErr := strconv.Atoi(fields[0])
		group, groupErr := strconv.Atoi(fields[1])
		if pidErr != nil || groupErr != nil || pid <= 0 || group <= 0 || fields[2] == "" {
			return nil, errors.New("malformed /bin/ps process-group output")
		}
		if group == pgid {
			members = append(members, processGroupMember{PID: pid, PGID: group, State: fields[2]})
		}
	}
	return members, nil
}

func processStart(pid int) (string, error) {
	out, err := exec.Command(supervisorPSPath, "-o", "lstart=", "-p", strconv.Itoa(pid)).Output()
	start := strings.TrimSpace(string(out))
	if err != nil || start == "" {
		return "", errors.New("worker process-group start identity unavailable")
	}
	return start, nil
}

func establishProcessGroupIdentity(pid int) (processGroupIdentity, error) {
	members, err := supervisorGroupState(pid)
	if err != nil {
		return processGroupIdentity{}, err
	}
	leader := false
	for _, member := range members {
		leader = leader || member.PID == pid && member.PGID == pid
	}
	if !leader {
		return processGroupIdentity{}, errors.New("worker process-group identity unavailable")
	}
	start, err := supervisorProcessStart(pid)
	if err != nil {
		return processGroupIdentity{}, err
	}
	return processGroupIdentity{PID: pid, PGID: pid, Start: start}, nil
}

func cleanupFailure(nominal *SupervisionFailure, _ error) *SupervisionFailure {
	cause := "unknown"
	if nominal != nil {
		cause = string(nominal.Code)
	}
	return failure(CodeCleanupFailure, fmt.Errorf("worker group cleanup could not be verified; superseded=%s", cause))
}

func terminateWorkerGroup(group processGroupIdentity, done <-chan error) error {
	deadline := supervisorCleanupNow().Add(time.Second)
	for {
		members, err := supervisorGroupState(group.PGID)
		if err != nil {
			return err
		}
		live, leader := false, false
		for _, member := range members {
			if member.PID == group.PID && member.PGID == group.PGID {
				start, startErr := supervisorProcessStart(group.PID)
				if startErr != nil || start != group.Start {
					return errors.New("worker process-group identity changed")
				}
				leader = true
			}
			if !strings.HasPrefix(member.State, "Z") {
				live = true
			}
		}
		if !live {
			return awaitWorkerWithin(done, deadline)
		}
		if !leader {
			return errors.New("worker process-group leader identity unavailable")
		}
		if supervisorCleanupNow().After(deadline) {
			return errors.New("process group remained live after bounded cleanup")
		}
		// Re-observe the exact leader identity immediately before the group signal.
		// Darwin exposes no atomic identity-and-signal primitive, so failure to
		// retain custody here fails closed without signaling.
		confirm, confirmErr := supervisorGroupState(group.PGID)
		if confirmErr != nil {
			return confirmErr
		}
		confirmed := false
		for _, member := range confirm {
			confirmed = confirmed || (member.PID == group.PID && member.PGID == group.PGID && !strings.HasPrefix(member.State, "Z"))
		}
		start, startErr := supervisorProcessStart(group.PID)
		if !confirmed || startErr != nil || start != group.Start {
			return errors.New("worker process-group signal custody unavailable")
		}
		if err = supervisorGroupKill(-group.PGID, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
			return err
		}
		supervisorCleanupWait(supervisorPoll)
	}
}

func awaitWorkerWithin(done <-chan error, deadline time.Time) error {
	if done == nil {
		return nil
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return errors.New("worker wait exceeded bounded cleanup")
	}
	timer := time.NewTimer(remaining)
	defer timer.Stop()
	select {
	case <-done:
		return nil
	case <-timer.C:
		return errors.New("worker wait exceeded bounded cleanup")
	}
}

func awaitTerminatedWorker(done <-chan error) (error, bool) {
	timer := time.NewTimer(supervisorPoll)
	defer timer.Stop()
	select {
	case waitErr := <-done:
		return waitErr, true
	case <-timer.C:
		return nil, false
	}
}

func classifyTerminatedWorker(waitErr error, stderr string) *SupervisionFailure {
	if waitErr == nil {
		return nil
	}
	detail := strings.TrimSpace(stderr)
	if strings.Contains(detail, "panic:") || strings.Contains(detail, "SIGABRT: abort") {
		return failure(CodePanic, errors.New(detail))
	}
	return failure(CodeNonzeroExit, errors.New(detail))
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
