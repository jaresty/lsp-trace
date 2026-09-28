package adr0011acquisition

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"lsp-trace/internal/adr0011methodresult"
)

const maxGitOutput = 64 << 10
const maxHostExecutable = 128 << 20

type boundedOutput struct {
	bytes    []byte
	exceeded bool
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	if len(b.bytes)+len(p) > maxGitOutput {
		b.exceeded = true
		return 0, ErrAcquisition
	}
	b.bytes = append(b.bytes, p...)
	return len(p), nil
}

func hostExecutableIdentity() (string, string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", "", ErrAcquisition
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", "", ErrAcquisition
	}
	f, err := os.Open(path)
	if err != nil {
		return "", "", ErrAcquisition
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maxHostExecutable {
		return "", "", ErrAcquisition
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, maxHostExecutable+1))
	if err != nil || n != info.Size() {
		return "", "", ErrAcquisition
	}
	return path, fmt.Sprintf("sha256:%x", h.Sum(nil)), nil
}

func hostCommand(parent context.Context, root string, args ...string) (adr0011methodresult.HostGitCommand, error) {
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	result := adr0011methodresult.HostGitCommand{Args: append([]string(nil), args...), Exit: -1, Timestamp: time.Now().UTC().Format(time.RFC3339Nano)}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	out, errs := &boundedOutput{}, &boundedOutput{}
	cmd.Stdout = out
	cmd.Stderr = errs
	err := cmd.Run()
	result.Stdout = string(out.bytes)
	result.Stderr = string(errs.bytes)
	if cmd.ProcessState != nil {
		result.Exit = cmd.ProcessState.ExitCode()
	}
	if err != nil || ctx.Err() != nil || out.exceeded || errs.exceeded || result.Exit != 0 || len(result.Stderr) > 0 {
		return result, ErrAcquisition
	}
	return result, nil
}

// observeHostGit takes the workspace only from the exact READY Manager record.
// Git worktree membership and exact clean HEAD are independent host observations;
// neither proves what the peer analyzed or upgrades caller revision custody.
func observeHostGit(ctx context.Context, root, revision string) (adr0011methodresult.HostGitObservation, error) {
	zero := adr0011methodresult.HostGitObservation{}
	if !filepath.IsAbs(root) || revision == "" || len(root) > 4096 {
		return zero, ErrAcquisition
	}
	executable, digest, err := hostExecutableIdentity()
	if err != nil {
		return zero, err
	}
	o := adr0011methodresult.HostGitObservation{Version: adr0011methodresult.HostGitProbeVersion, Custody: "HOST_OBSERVED_GIT", WorkspaceRoot: root, HostExecutable: executable, HostExecutableSHA256: digest}
	if o.TopLevel, err = hostCommand(ctx, root, "rev-parse", "--show-toplevel"); err != nil {
		return zero, err
	}
	if o.Head, err = hostCommand(ctx, root, "rev-parse", "HEAD"); err != nil {
		return zero, err
	}
	if o.Status, err = hostCommand(ctx, root, "status", "--porcelain=v1", "--untracked-files=all"); err != nil {
		return zero, err
	}
	if o.Worktrees, err = hostCommand(ctx, root, "worktree", "list", "--porcelain"); err != nil {
		return zero, err
	}
	if strings.TrimSpace(o.TopLevel.Stdout) != root || strings.TrimSpace(o.Head.Stdout) != revision || o.Status.Stdout != "" || !adr0011methodresult.ValidHostGitEvidence(adr0011methodresult.HostGitEvidence{Before: o, After: o}, root, revision) {
		return zero, ErrAcquisition
	}
	return o, nil
}
