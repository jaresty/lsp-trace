package gonumlouvain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const PSPath = "/bin/ps"

type Limits struct {
	Wall     time.Duration
	RSSBytes int64
	Poll     time.Duration
}
type Observation struct {
	ElapsedNanos     int64  `json:"elapsed_nanos"`
	PeakTreeRSSBytes int64  `json:"peak_tree_rss_bytes"`
	ExitKind         string `json:"exit_kind"`
}
type SupervisedResult struct {
	Output      Output      `json:"output"`
	Observation Observation `json:"observation"`
}
type Supervisor struct {
	GOOS        string
	PS          string
	Limits      Limits
	BeforeStart func() error
	Args        []string
	Env         []string
}

func DarwinSupervisor() Supervisor {
	return Supervisor{GOOS: runtime.GOOS, PS: PSPath, Limits: Limits{Wall: 60 * time.Second, RSSBytes: 512 << 20, Poll: 20 * time.Millisecond}}
}

func (s Supervisor) Run(executable string, req Request) (SupervisedResult, error) {
	var z SupervisedResult
	if s.GOOS != "darwin" || s.PS != PSPath {
		return z, Fail("MEMORY_UNSUPPORTED", "Darwin /bin/ps process-tree supervisor unavailable")
	}
	if s.Limits.Wall <= 0 || s.Limits.Wall > 60*time.Second || s.Limits.RSSBytes <= 0 || s.Limits.RSSBytes > 512<<20 || s.Limits.Poll <= 0 {
		return z, Fail("SUPERVISOR_UNSUPPORTED", "invalid limits")
	}
	if st, err := os.Stat(s.PS); err != nil || st.Mode()&0111 == 0 {
		return z, Fail("MEMORY_UNSUPPORTED", "/bin/ps unavailable")
	}
	if s.BeforeStart != nil {
		if err := s.BeforeStart(); err != nil {
			return z, Fail("SUPERVISOR_UNSUPPORTED", err.Error())
		}
	}
	in, err := json.Marshal(req)
	if err != nil {
		return z, Fail("DERIVATION_REJECTED", err.Error())
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, s.Args...)
	if len(s.Env) != 0 {
		cmd.Env = append(os.Environ(), s.Env...)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return z, Fail("SUPERVISOR_UNSUPPORTED", err.Error())
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	started := time.Now()
	if err = cmd.Start(); err != nil {
		return z, Fail("NONZERO_EXIT", err.Error())
	}
	pid := cmd.Process.Pid
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	kill := func() {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		select {
		case <-done:
		case <-time.After(time.Second):
		}
	}
	// The child blocks in JSON decoding until this external supervisor has
	// established and sampled the process tree. Candidate code cannot run first.
	var peak int64
	var sampleErr error
	visibilityDeadline := time.Now().Add(2 * time.Second)
	for {
		peak, sampleErr = treeRSS(s.PS, pid)
		if sampleErr == nil || time.Now().After(visibilityDeadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if sampleErr != nil {
		kill()
		return z, Fail("MEMORY_UNSUPPORTED", sampleErr.Error())
	}
	if peak > s.Limits.RSSBytes {
		kill()
		return z, Fail("MEMORY_EXCEEDED", fmt.Sprint(peak))
	}
	if _, err = stdin.Write(in); err != nil {
		kill()
		return z, Fail("NONZERO_EXIT", err.Error())
	}
	if err = stdin.Close(); err != nil {
		kill()
		return z, Fail("NONZERO_EXIT", err.Error())
	}
	tick := time.NewTicker(s.Limits.Poll)
	defer tick.Stop()
	for {
		select {
		case err = <-done:
			z.Observation = Observation{ElapsedNanos: time.Since(started).Nanoseconds(), PeakTreeRSSBytes: peak, ExitKind: "OK"}
			if err != nil {
				detail := strings.TrimSpace(stderr.String())
				if ee, ok := err.(*exec.ExitError); ok {
					if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() && ws.Signal() == syscall.SIGABRT {
						return z, Fail("PANIC", detail)
					}
				}
				if strings.HasPrefix(detail, "SIGABRT: abort") {
					return z, Fail("PANIC", detail)
				}
				return z, Fail("NONZERO_EXIT", detail)
			}
			if err = json.Unmarshal(stdout.Bytes(), &z.Output); err != nil {
				return z, Fail("NONCANONICAL_OUTPUT", "invalid JSON")
			}
			if err = ValidateCanonical(z.Output); err != nil {
				return z, err
			}
			return z, nil
		case <-tick.C:
			if time.Since(started) > s.Limits.Wall {
				kill()
				return z, Fail("TIMEOUT", s.Limits.Wall.String())
			}
			rss, e := treeRSS(s.PS, pid)
			if e != nil {
				kill()
				return z, Fail("MEMORY_UNSUPPORTED", e.Error())
			}
			if rss > peak {
				peak = rss
			}
			if rss > s.Limits.RSSBytes {
				kill()
				return z, Fail("MEMORY_EXCEEDED", fmt.Sprint(rss))
			}
		}
	}
}
func treeRSS(ps string, root int) (int64, error) {
	out, err := exec.Command(ps, "-axo", "pid=,ppid=,rss=").Output()
	if err != nil {
		return 0, err
	}
	children := map[int][]int{}
	rss := map[int]int64{}
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) != 3 {
			continue
		}
		p, e1 := strconv.Atoi(f[0])
		pp, e2 := strconv.Atoi(f[1])
		r, e3 := strconv.ParseInt(f[2], 10, 64)
		if e1 != nil || e2 != nil || e3 != nil {
			return 0, fmt.Errorf("malformed /bin/ps output")
		}
		children[pp] = append(children[pp], p)
		rss[p] = r * 1024
	}
	if _, ok := rss[root]; !ok {
		return 0, fmt.Errorf("candidate process absent from /bin/ps")
	}
	seen := map[int]bool{}
	q := []int{root}
	var total int64
	for len(q) > 0 {
		p := q[0]
		q = q[1:]
		if seen[p] {
			continue
		}
		seen[p] = true
		total += rss[p]
		q = append(q, children[p]...)
	}
	return total, nil
}
