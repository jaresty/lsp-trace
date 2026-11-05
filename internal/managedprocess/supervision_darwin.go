//go:build darwin

package managedprocess

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func censusLocalDarwinGroupPrivateWindow(pid int, b *privateProviderWindow) GroupCensusObservation {
	ctx, cancel, ok := b.beginCensus()
	if !ok {
		return GroupCensusObservation{ProcessGroupID: pid, Limit: 64, Bounded: true, Truncated: true, Err: ErrPrivateProviderWindow, Evidence: LocalDarwinSupervisionOnly}
	}
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/ps", "-o", "pid=", "-g", strconv.Itoa(pid))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return GroupCensusObservation{Err: err}
	}
	if err = cmd.Start(); err != nil {
		return GroupCensusObservation{Err: err}
	}
	result := parsePrivateCensus(stdout, b)
	result.ProcessGroupID = pid
	if result.Truncated {
		_ = cmd.Process.Kill()
	}
	result.Err = errors.Join(result.Err, cmd.Wait(), ctx.Err())
	var exitErr *exec.ExitError
	if result.Members == 0 && !result.Truncated && ctx.Err() == nil && errors.As(result.Err, &exitErr) && exitErr.ExitCode() == 1 {
		result.Err = nil
	}
	return result
}

func localDarwinSupported() bool { return true }

func configureLocalDarwin(cmd *exec.Cmd) error {
	if cmd == nil {
		return errors.New("managedprocess: nil Darwin command")
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return nil
}

func signalLocalDarwinGroup(pid int, signal os.Signal) error {
	sig, ok := signal.(syscall.Signal)
	if !ok {
		return fmt.Errorf("managedprocess: unsupported Darwin signal %T", signal)
	}
	return syscall.Kill(-pid, sig)
}

func censusLocalDarwinGroup(pid, limit int) GroupCensusObservation {
	result := GroupCensusObservation{ProcessGroupID: pid, Limit: limit, Bounded: true, Evidence: LocalDarwinSupervisionOnly}
	if limit <= 0 {
		result.Err = errors.New("managedprocess: invalid census limit")
		return result
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/ps", "-o", "pid=", "-g", strconv.Itoa(pid))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		result.Err = err
		return result
	}
	if err = cmd.Start(); err != nil {
		result.Err = err
		return result
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		if result.Members == limit {
			result.Truncated = true
			break
		}
		result.Members++
	}
	if result.Truncated {
		_ = cmd.Process.Kill()
	}
	waitErr := cmd.Wait()
	result.Err = errors.Join(scanner.Err(), waitErr, ctx.Err())
	return result
}
