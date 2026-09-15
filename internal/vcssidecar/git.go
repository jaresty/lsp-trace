package vcssidecar

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const maxGitOutputBytes = 16 * 1024 * 1024

type GitHistory struct {
	Timeout time.Duration
}

type cappedBuffer struct {
	bytes.Buffer
	limit    int
	exceeded bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := b.limit - b.Len()
	if remaining > 0 {
		if remaining > len(p) {
			remaining = len(p)
		}
		_, _ = b.Buffer.Write(p[:remaining])
	}
	if n > remaining {
		b.exceeded = true
	}
	return n, nil
}

func (g GitHistory) Collect(workspace string, paths []string, from, to string) (HistoryCollection, error) {
	timeout := g.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	realWorkspace, err := filepath.EvalSymlinks(workspace)
	if err != nil || !filepath.IsAbs(realWorkspace) || filepath.Clean(realWorkspace) != realWorkspace {
		return HistoryCollection{}, errors.New("invalid Git workspace root")
	}
	top, err := gitOutput(ctx, realWorkspace, "rev-parse", "--show-toplevel")
	if err != nil {
		return HistoryCollection{}, errors.New("resolve Git workspace root")
	}
	realTop, err := filepath.EvalSymlinks(strings.TrimSpace(string(top)))
	if err != nil || realTop != realWorkspace {
		return HistoryCollection{}, errors.New("workspace must equal Git top-level")
	}
	fromHash, err := resolveCommit(ctx, realWorkspace, from)
	if err != nil {
		return HistoryCollection{}, fmt.Errorf("resolve from revision: %w", err)
	}
	toHash, err := resolveCommit(ctx, realWorkspace, to)
	if err != nil {
		return HistoryCollection{}, fmt.Errorf("resolve to revision: %w", err)
	}
	args := []string{"-C", realWorkspace, "log", "--no-renames", "--format=C%x00%H%x00", "--numstat", "-z", fromHash + ".." + toHash, "--"}
	args = append(args, paths...)
	cmd := exec.CommandContext(ctx, "git", args...)
	var stdout, stderr cappedBuffer
	stdout.limit, stderr.limit = maxGitOutputBytes, 64*1024
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	out := stdout.Bytes()
	if ctx.Err() != nil {
		return HistoryCollection{}, errors.New("git history timeout")
	}
	if err != nil {
		return HistoryCollection{}, fmt.Errorf("git history failed: %w", err)
	}
	if stdout.exceeded || stderr.exceeded || len(out) > maxGitOutputBytes {
		return HistoryCollection{}, errors.New("git history output exceeds limit")
	}
	requested := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		requested[path] = struct{}{}
	}
	metrics, err := parseGitNumstat(out, requested)
	if err != nil {
		return HistoryCollection{}, err
	}
	return HistoryCollection{FromRevision: fromHash, ToRevision: toHash, Metrics: metrics}, nil
}

func gitOutput(ctx context.Context, workspace string, args ...string) ([]byte, error) {
	all := append([]string{"-C", workspace}, args...)
	cmd := exec.CommandContext(ctx, "git", all...)
	var stdout, stderr cappedBuffer
	stdout.limit, stderr.limit = 64*1024, 64*1024
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	if stdout.exceeded || stderr.exceeded {
		return nil, errors.New("Git command output exceeds limit")
	}
	return stdout.Bytes(), nil
}

func resolveCommit(ctx context.Context, workspace, revision string) (string, error) {
	out, err := gitOutput(ctx, workspace, "rev-parse", "--verify", "--end-of-options", revision+"^{commit}")
	if err != nil {
		return "", err
	}
	hash := strings.TrimSpace(string(out))
	if len(hash) != 40 && len(hash) != 64 {
		return "", errors.New("git returned non-canonical commit identity")
	}
	for _, c := range hash {
		if c < '0' || (c > '9' && c < 'a') || c > 'f' {
			return "", errors.New("git returned non-canonical commit identity")
		}
	}
	return hash, nil
}

func parseGitNumstat(raw []byte, requested map[string]struct{}) (map[string]FileMetric, error) {
	metrics := make(map[string]FileMetric)
	var commit string
	seen := make(map[string]bool)
	parts := bytes.Split(raw, []byte{0})
	for i := 0; i < len(parts); i++ {
		part := strings.TrimPrefix(string(parts[i]), "\n")
		if part == "" {
			continue
		}
		if part == "C" {
			i++
			if i >= len(parts) || len(parts[i]) == 0 {
				return nil, errors.New("git history missing commit identity")
			}
			commit = string(parts[i])
			seen = make(map[string]bool)
			continue
		}
		if commit == "" {
			return nil, errors.New("git numstat precedes commit identity")
		}
		fields := strings.SplitN(part, "\t", 3)
		if len(fields) != 3 || fields[2] == "" {
			return nil, errors.New("malformed git numstat record")
		}
		path := fields[2]
		if _, ok := requested[path]; !ok {
			return nil, fmt.Errorf("git returned unrequested path %q", path)
		}
		metric := metrics[path]
		if !seen[path] {
			metric.CommitCount++
			seen[path] = true
		}
		if metric.LastRevision == "" {
			metric.LastRevision = commit
		}
		if fields[0] == "-" && fields[1] == "-" {
			metric.BinaryChanges++
		} else {
			added, aerr := strconv.Atoi(fields[0])
			deleted, derr := strconv.Atoi(fields[1])
			if aerr != nil || derr != nil || added < 0 || deleted < 0 {
				return nil, errors.New("invalid git numstat counts")
			}
			metric.LinesAdded += added
			metric.LinesDeleted += deleted
		}
		metrics[path] = metric
	}
	return metrics, nil
}
