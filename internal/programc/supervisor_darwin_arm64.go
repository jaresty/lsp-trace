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
	supervisorCommandWait  = func(cmd *exec.Cmd) error { return cmd.Wait() }
	supervisorProcessKill  = func(process *os.Process) error { return process.Kill() }
	supervisorInitialRSS   = initialTreeRSS
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

func supervisePrivateWorker(ctx context.Context, requestData workerRequest) (privateSupervisorResult, SupervisionObservation, *SupervisionFailure) {
	var zero privateSupervisorResult
	observation := SupervisionObservation{Ceiling: ObservationCeiling}
	started := time.Now()
	deadline := started.Add(supervisorWall)
	if parent, ok := ctx.Deadline(); ok && parent.Before(deadline) {
		deadline = parent
	}
	windowFailure := func() *SupervisionFailure {
		if err := ctx.Err(); err != nil {
			return failure(CodeCancelled, err)
		}
		if !time.Now().Before(deadline) {
			return failure(CodeTimeout, errors.New("shared wall limit exceeded"))
		}
		return nil
	}
	if failed := windowFailure(); failed != nil {
		return zero, observation, failed
	}
	windowCtx, cancelWindow := context.WithDeadline(ctx, deadline)
	defer cancelWindow()
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	request, err := json.Marshal(requestData)
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
	defer stdin.Close()
	if failed := windowFailure(); failed != nil {
		return zero, observation, failed
	}
	if err = cmd.Start(); err != nil {
		return zero, observation, failure(CodeNonzeroExit, err)
	}
	pid := cmd.Process.Pid
	decodeWorker := func(waitErr error) (privateSupervisorResult, *SupervisionFailure) {
		if stdout.Exceeded() {
			return zero, failure(CodeOutputLimit, errors.New("worker stdout limit exceeded"))
		}
		if waitErr != nil {
			return zero, classifyTerminatedWorker(waitErr, stderr.String())
		}
		var response workerResponse
		var decodeErr error
		if requestData.Version == workerVersion {
			var legacy legacyWorkerResponse
			decodeErr = strictDecode(bytes.NewReader(stdout.Bytes()), supervisorOutputBytes, &legacy)
			response = workerResponse{Version: legacy.Version, Outcome: legacy.Outcome, Failure: legacy.Failure}
			if (legacy.Outcome == nil) == (legacy.Failure == nil) {
				decodeErr = errors.New("invalid legacy response family")
			}
		} else {
			decodeErr = strictDecode(bytes.NewReader(stdout.Bytes()), supervisorOutputBytes, &response)
		}
		if decodeErr != nil || response.Version != requestData.Version {
			return zero, failure(CodeMalformedOutput, errors.New("invalid worker response"))
		}
		if response.Failure != nil {
			if response.Outcome != nil || response.Bundle != nil || len(response.Report) != 0 {
				return zero, failure(CodeMalformedOutput, errors.New("mixed worker response families"))
			}
			return zero, response.Failure
		}
		if requestData.Version == bundleWorkerVersionPrivate {
			var canonical bytes.Buffer
			if stderr.Exceeded() || writeWorkerResponse(&canonical, response) != 0 || !bytes.Equal(canonical.Bytes(), stdout.Bytes()) {
				return zero, failure(CodeMalformedOutput, errors.New("noncanonical checked-bundle response or stderr overflow"))
			}
			report, failed := acceptBundleResponsePrivate(requestData, response)
			return privateSupervisorResult{report: report}, failed
		}
		if response.Outcome == nil || response.Bundle != nil || len(response.Report) != 0 {
			return zero, failure(CodeMalformedOutput, errors.New("invalid legacy response family"))
		}
		out, failed := reconstruct(requestData.Input, requestData.Seed, response.Outcome)
		return privateSupervisorResult{legacy: out}, failed
	}
	group, groupErr := establishProcessGroupIdentity(windowCtx, pid)
	if groupErr != nil {
		killErr := supervisorProcessKill(cmd.Process)
		if killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
			return zero, observation, cleanupFailure(failure(CodeMemoryUnsupported, groupErr), killErr)
		}
		_ = supervisorCommandWait(cmd) // the sole Wait owner reaps only after Kill succeeded or the child was already done
		return zero, observation, cleanupFailure(failure(CodeMemoryUnsupported, groupErr), groupErr)
	}
	var delivery <-chan error
	finishWorker := func(cleanupErr error, nominal *SupervisionFailure) (privateSupervisorResult, *SupervisionFailure) {
		// Closing the parent's pipe releases any blocked writer before Wait.
		_ = stdin.Close()
		if delivery != nil {
			<-delivery
			delivery = nil
		}
		if cleanupErr != nil {
			return zero, cleanupFailure(nominal, cleanupErr)
		}
		waitErr := supervisorCommandWait(cmd)
		if cmd.ProcessState != nil {
			notifySupervisorPrivate(supervisorReapedPrivate)
		}
		if nominal == nil {
			nominal = windowFailure()
		}
		var out privateSupervisorResult
		var terminalFailure *SupervisionFailure
		if nominal == nil {
			notifySupervisorPrivate(supervisorBeforeVerifyPrivate)
			nominal = windowFailure()
			if nominal == nil {
				out, terminalFailure = decodeWorker(waitErr)
				notifySupervisorPrivate(supervisorAfterVerifyPrivate)
			}
		}
		if nominal == nil {
			nominal = windowFailure()
		}
		observation.ElapsedNanos = time.Since(started).Nanoseconds()
		if nominal != nil {
			terminalFailure = nominal
			out = zero
		}
		return out, terminalFailure
	}
	finishExitedWorker := func(nominal *SupervisionFailure) (privateSupervisorResult, *SupervisionFailure) {
		return finishWorker(terminateWorkerGroup(group), nominal)
	}
	stopWorker := func(nominal *SupervisionFailure) *SupervisionFailure {
		_, terminalFailure := finishExitedWorker(nominal)
		return terminalFailure
	}
	peak, err := supervisorInitialRSS(windowCtx, pid)
	if err != nil {
		if nominal := windowFailure(); nominal != nil {
			return zero, observation, stopWorker(nominal)
		}
		if processGroupLeaderZombie(windowCtx, group) {
			observation.ElapsedNanos = time.Since(started).Nanoseconds()
			out, terminalFailure := finishExitedWorker(failure(CodeMemoryUnsupported, err))
			return out, observation, terminalFailure
		}
		return zero, observation, stopWorker(failure(CodeMemoryUnsupported, err))
	}
	observation.PeakTreeRSSBytes = peak
	observation.Samples = 1
	if peak > supervisorRSSBytes {
		return zero, observation, stopWorker(failure(CodeMemoryLimit, fmt.Errorf("sampled tree RSS %d exceeds %d", peak, supervisorRSSBytes)))
	}
	if failed := windowFailure(); failed != nil {
		return zero, observation, stopWorker(failed)
	}
	written := make(chan error, 1)
	delivery = written
	go func() {
		_, writeErr := stdin.Write(request)
		closeErr := stdin.Close()
		if writeErr == nil {
			writeErr = closeErr
		}
		notifySupervisorPrivate(supervisorDeliveryFinishedPrivate)
		written <- writeErr
	}()
	ticker := time.NewTicker(supervisorPoll)
	defer ticker.Stop()
	for {
		select {
		case err := <-delivery:
			delivery = nil
			if err != nil {
				nominal := windowFailure()
				if nominal == nil {
					nominal = failure(CodeNonzeroExit, err)
				}
				return zero, observation, stopWorker(nominal)
			}
		case <-ctx.Done():
			observation.ElapsedNanos = time.Since(started).Nanoseconds()
			return zero, observation, stopWorker(failure(CodeCancelled, ctx.Err()))
		case <-timer.C:
			observation.ElapsedNanos = time.Since(started).Nanoseconds()
			return zero, observation, stopWorker(failure(CodeTimeout, errors.New("60s wall limit exceeded")))
		case <-ticker.C:
			if err := ctx.Err(); err != nil {
				observation.ElapsedNanos = time.Since(started).Nanoseconds()
				return zero, observation, stopWorker(failure(CodeCancelled, err))
			}
			if processGroupLeaderZombie(windowCtx, group) {
				observation.ElapsedNanos = time.Since(started).Nanoseconds()
				out, terminalFailure := finishExitedWorker(nil)
				return out, observation, terminalFailure
			}
			if stdout.Exceeded() {
				observation.ElapsedNanos = time.Since(started).Nanoseconds()
				return zero, observation, stopWorker(failure(CodeOutputLimit, errors.New("worker stdout limit exceeded")))
			}
			rss, e := treeRSS(windowCtx, pid)
			if e != nil {
				observation.ElapsedNanos = time.Since(started).Nanoseconds()
				nominal := windowFailure()
				if nominal == nil {
					nominal = failure(CodeMemoryUnsupported, e)
				}
				return zero, observation, stopWorker(nominal)
			}
			observation.Samples++
			if rss > observation.PeakTreeRSSBytes {
				observation.PeakTreeRSSBytes = rss
			}
			if rss > supervisorRSSBytes {
				observation.ElapsedNanos = time.Since(started).Nanoseconds()
				return zero, observation, stopWorker(failure(CodeMemoryLimit, fmt.Errorf("sampled tree RSS %d exceeds %d", rss, supervisorRSSBytes)))
			}
		}
	}
}

func supervisorPSOutput(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, supervisorPSPath, args...)
	cmd.WaitDelay = 100 * time.Millisecond
	return cmd.Output()
}

func processGroupState(ctx context.Context, pgid int) ([]processGroupMember, error) {
	out, err := supervisorPSOutput(ctx, "-axo", "pid=,pgid=,state=")
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

func processStart(ctx context.Context, pid int) (string, error) {
	out, err := supervisorPSOutput(ctx, "-o", "lstart=", "-p", strconv.Itoa(pid))
	start := strings.TrimSpace(string(out))
	if err != nil || start == "" {
		return "", errors.New("worker process-group start identity unavailable")
	}
	return start, nil
}

func establishProcessGroupIdentity(ctx context.Context, pid int) (processGroupIdentity, error) {
	deadline := supervisorCleanupNow().Add(time.Second)
	var last error
	for {
		members, err := supervisorGroupState(ctx, pid)
		if err == nil {
			for _, member := range members {
				if member.PID == pid && member.PGID == pid {
					start, startErr := supervisorProcessStart(ctx, pid)
					if startErr == nil {
						return processGroupIdentity{PID: pid, PGID: pid, Start: start}, nil
					}
					last = startErr
				}
			}
			if last == nil {
				last = errors.New("worker process-group identity unavailable")
			}
		} else {
			last = err
		}
		if ctx.Err() != nil || !supervisorCleanupNow().Before(deadline) {
			return processGroupIdentity{}, last
		}
		supervisorCleanupWait(supervisorPoll)
	}
}

func processGroupLeaderZombie(ctx context.Context, group processGroupIdentity) bool {
	members, err := supervisorGroupState(ctx, group.PGID)
	if err != nil {
		return false
	}
	for _, member := range members {
		if member.PID == group.PID && member.PGID == group.PGID && strings.HasPrefix(member.State, "Z") {
			start, startErr := supervisorProcessStart(ctx, group.PID)
			return startErr == nil && start == group.Start
		}
	}
	return false
}

func cleanupFailure(nominal *SupervisionFailure, _ error) *SupervisionFailure {
	cause := "unknown"
	if nominal != nil {
		cause = string(nominal.Code)
	}
	return failure(CodeCleanupFailure, fmt.Errorf("worker group cleanup could not be verified; superseded=%s", cause))
}

func terminateWorkerGroup(group processGroupIdentity) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	deadline := supervisorCleanupNow().Add(time.Second)
	for {
		members, err := supervisorGroupState(ctx, group.PGID)
		if err != nil {
			return err
		}
		live, leader := false, false
		for _, member := range members {
			if member.PID == group.PID && member.PGID == group.PGID {
				start, startErr := supervisorProcessStart(ctx, group.PID)
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
			return nil
		}
		if !leader {
			return errors.New("worker process-group leader identity unavailable")
		}
		if ctx.Err() != nil || supervisorCleanupNow().After(deadline) {
			return errors.New("process group remained live after bounded cleanup")
		}
		// Re-observe the exact leader identity immediately before the group signal.
		// Darwin exposes no atomic identity-and-signal primitive, so failure to
		// retain custody here fails closed without signaling.
		confirm, confirmErr := supervisorGroupState(ctx, group.PGID)
		if confirmErr != nil {
			return confirmErr
		}
		confirmed, confirmedLive := false, false
		for _, member := range confirm {
			confirmed = confirmed || (member.PID == group.PID && member.PGID == group.PGID)
			confirmedLive = confirmedLive || !strings.HasPrefix(member.State, "Z")
		}
		// A dying Darwin process may transition ?E -> Z between snapshots.
		// Reuse the existing terminal-group rule at this confirmation point:
		// no live members means no further signal; the sole Wait owner reaps.
		// Any still-live group retains the exact identity/signal checks below.
		if !confirmedLive {
			return nil
		}
		start, startErr := supervisorProcessStart(ctx, group.PID)
		if !confirmed || startErr != nil || start != group.Start {
			return errors.New("worker process-group signal custody unavailable")
		}
		if err = supervisorGroupKill(-group.PGID, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
			return err
		}
		supervisorCleanupWait(supervisorPoll)
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

func initialTreeRSS(ctx context.Context, root int) (int64, error) {
	deadline := time.Now().Add(2 * time.Second)
	var err error
	for {
		var rss int64
		rss, err = treeRSS(ctx, root)
		if err == nil {
			return rss, nil
		}
		if ctx.Err() != nil || time.Now().After(deadline) {
			return 0, err
		}
		time.Sleep(time.Millisecond)
	}
}

func treeRSS(ctx context.Context, root int) (int64, error) {
	out, err := supervisorPSOutput(ctx, "-axo", "pid=,ppid=,rss=")
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
