package vcssymbolsidecar

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const maxGitDiffBytes = 16 * 1024 * 1024

type DiffCollection struct {
	FromRevision string
	ToRevision   string
	Lines        []ChangedLine
}

type GitDiff struct{ Timeout time.Duration }

type limitedWriter struct {
	bytes.Buffer
	limit    int
	exceeded bool
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	n := len(p)
	remain := w.limit - w.Len()
	if remain > len(p) {
		remain = len(p)
	}
	if remain > 0 {
		_, _ = w.Buffer.Write(p[:remain])
	}
	if n > remain {
		w.exceeded = true
	}
	return n, nil
}

func (g GitDiff) Collect(workspace string, paths []string, from, to string) (DiffCollection, error) {
	timeout := g.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	real, err := filepath.EvalSymlinks(workspace)
	if err != nil || !filepath.IsAbs(real) || filepath.Clean(real) != real {
		return DiffCollection{}, errors.New("invalid Git workspace root")
	}
	top, err := symbolGitOutput(ctx, real, 64*1024, "rev-parse", "--show-toplevel")
	if err != nil {
		return DiffCollection{}, errors.New("resolve Git workspace root")
	}
	realTop, err := filepath.EvalSymlinks(strings.TrimSpace(string(top)))
	if err != nil || realTop != real {
		return DiffCollection{}, errors.New("workspace must equal Git top-level")
	}
	requested := make(map[string]struct{}, len(paths))
	clean := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" || filepath.IsAbs(path) || filepath.Clean(path) != path || path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
			return DiffCollection{}, errors.New("invalid requested path")
		}
		slash := filepath.ToSlash(path)
		if _, ok := requested[slash]; ok {
			continue
		}
		requested[slash] = struct{}{}
		clean = append(clean, slash)
	}
	if len(clean) == 0 || len(clean) > 128 {
		return DiffCollection{}, errors.New("requested path count outside limit")
	}
	sort.Strings(clean)
	fromHash, err := symbolResolveCommit(ctx, real, from)
	if err != nil {
		return DiffCollection{}, fmt.Errorf("resolve from revision: %w", err)
	}
	toHash, err := symbolResolveCommit(ctx, real, to)
	if err != nil {
		return DiffCollection{}, fmt.Errorf("resolve to revision: %w", err)
	}
	args := []string{"diff", "--no-renames", "--unified=0", fromHash, toHash, "--"}
	args = append(args, clean...)
	raw, err := symbolGitOutput(ctx, real, maxGitDiffBytes, args...)
	if ctx.Err() != nil {
		return DiffCollection{}, errors.New("Git diff timeout")
	}
	if err != nil {
		return DiffCollection{}, fmt.Errorf("Git diff failed: %w", err)
	}
	lines, err := parseUnifiedZeroDiff(raw, requested)
	if err != nil {
		return DiffCollection{}, err
	}
	return DiffCollection{FromRevision: fromHash, ToRevision: toHash, Lines: lines}, nil
}
func symbolResolveCommit(ctx context.Context, workspace, revision string) (string, error) {
	if strings.TrimSpace(revision) == "" {
		return "", errors.New("empty revision")
	}
	out, err := symbolGitOutput(ctx, workspace, 64*1024, "rev-parse", "--verify", "--end-of-options", revision+"^{commit}")
	if err != nil {
		return "", err
	}
	hash := strings.TrimSpace(string(out))
	if !validSymbolCommit(hash) {
		return "", errors.New("non-canonical commit identity")
	}
	return hash, nil
}
func validSymbolCommit(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, c := range s {
		if c < '0' || (c > '9' && c < 'a') || c > 'f' {
			return false
		}
	}
	return true
}
func symbolGitOutput(ctx context.Context, workspace string, limit int, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", workspace}, args...)...)
	var stdout, stderr limitedWriter
	stdout.limit = limit
	stderr.limit = 64 * 1024
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if stdout.exceeded || stderr.exceeded {
		return nil, errors.New("Git command output exceeds limit")
	}
	if err != nil {
		return nil, err
	}
	return stdout.Bytes(), nil
}

var zeroHunkPattern = regexp.MustCompile(`^@@ -([0-9]+)(?:,([0-9]+))? \+([0-9]+)(?:,([0-9]+))? @@`)

func parseUnifiedZeroDiff(raw []byte, requested map[string]struct{}) ([]ChangedLine, error) {
	if len(raw) > 16*1024*1024 {
		return nil, errors.New("Git diff output exceeds limit")
	}
	var out []ChangedLine
	oldPath, newPath := "", ""
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "--- ") {
			value := strings.TrimPrefix(line, "--- ")
			oldPath = ""
			if value != "/dev/null" {
				if !strings.HasPrefix(value, "a/") {
					return nil, errors.New("non-canonical Git old path")
				}
				oldPath = strings.TrimPrefix(value, "a/")
				if _, ok := requested[oldPath]; !ok {
					return nil, fmt.Errorf("Git diff returned unrequested path %q", oldPath)
				}
			}
			continue
		}
		if strings.HasPrefix(line, "+++ ") {
			value := strings.TrimPrefix(line, "+++ ")
			newPath = ""
			if value != "/dev/null" {
				if !strings.HasPrefix(value, "b/") {
					return nil, errors.New("non-canonical Git new path")
				}
				newPath = strings.TrimPrefix(value, "b/")
				if _, ok := requested[newPath]; !ok {
					return nil, fmt.Errorf("Git diff returned unrequested path %q", newPath)
				}
			}
			continue
		}
		if !strings.HasPrefix(line, "@@ ") {
			continue
		}
		if oldPath == "" && newPath == "" {
			return nil, errors.New("Git hunk lacks admitted path")
		}
		m := zeroHunkPattern.FindStringSubmatch(line)
		if m == nil {
			return nil, errors.New("malformed zero-context Git hunk")
		}
		oldStart, _ := strconv.Atoi(m[1])
		newStart, _ := strconv.Atoi(m[3])
		oldCount, newCount := 1, 1
		if m[2] != "" {
			oldCount, _ = strconv.Atoi(m[2])
		}
		if m[4] != "" {
			newCount, _ = strconv.Atoi(m[4])
		}
		if oldCount > 0 && oldPath == "" {
			return nil, errors.New("Git old hunk lacks admitted path")
		}
		if newCount > 0 && newPath == "" {
			return nil, errors.New("Git new hunk lacks admitted path")
		}
		for i := 0; i < oldCount; i++ {
			out = append(out, ChangedLine{Side: "OLD", Path: oldPath, Line: oldStart - 1 + i})
		}
		for i := 0; i < newCount; i++ {
			out = append(out, ChangedLine{Side: "NEW", Path: newPath, Line: newStart - 1 + i})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		if out[i].Side != out[j].Side {
			return out[i].Side == "OLD"
		}
		return out[i].Line < out[j].Line
	})
	return out, nil
}
