package adr0011methodresult

import (
	"encoding/hex"
	"path/filepath"
	"strings"
	"time"
)

const HostGitProbeVersion = "HOST_GIT_PROBE_V1"
const maxHostGitOutput = 64 << 10

type HostGitCommand struct {
	Args      []string
	Exit      int
	Stdout    string
	Stderr    string
	Timestamp string
}
type HostGitObservation struct {
	Version                           string
	Custody                           string
	WorkspaceRoot                     string
	HostExecutable                    string
	HostExecutableSHA256              string
	TopLevel, Head, Status, Worktrees HostGitCommand
}
type HostGitEvidence struct{ Before, After HostGitObservation }

func validHostGitCommand(c HostGitCommand, args ...string) bool {
	if len(c.Args) != len(args) || c.Exit != 0 || c.Stderr != "" || len(c.Stdout) > maxHostGitOutput || len(c.Stderr) > maxHostGitOutput || c.Timestamp == "" {
		return false
	}
	if _, err := time.Parse(time.RFC3339Nano, c.Timestamp); err != nil {
		return false
	}
	for i, a := range args {
		if c.Args[i] != a {
			return false
		}
	}
	return true
}
func validGitCommit(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	b, err := hex.DecodeString(s)
	return err == nil && len(b)*2 == len(s) && hex.EncodeToString(b) == s
}
func validHostGitObservation(o HostGitObservation) bool {
	if o.Version != HostGitProbeVersion || o.Custody != "HOST_OBSERVED_GIT" || !filepath.IsAbs(o.WorkspaceRoot) || o.HostExecutable == "" || !validDigest(o.HostExecutableSHA256) ||
		!validHostGitCommand(o.TopLevel, "rev-parse", "--show-toplevel") || !validHostGitCommand(o.Head, "rev-parse", "HEAD") || !validHostGitCommand(o.Status, "status", "--porcelain=v1", "--untracked-files=all") || !validHostGitCommand(o.Worktrees, "worktree", "list", "--porcelain") || o.Status.Stdout != "" || strings.TrimSpace(o.TopLevel.Stdout) != o.WorkspaceRoot || !validGitCommit(strings.TrimSpace(o.Head.Stdout)) {
		return false
	}
	// Worktree porcelain includes an exact worktree path and HEAD for this root.
	needle := "worktree " + o.WorkspaceRoot + "\nHEAD " + strings.TrimSpace(o.Head.Stdout) + "\n"
	return strings.Contains(o.Worktrees.Stdout, needle)
}
func ValidHostGitEvidence(e HostGitEvidence, root, commit string) bool {
	return validHostGitObservation(e.Before) && validHostGitObservation(e.After) && e.Before.WorkspaceRoot == root && e.After.WorkspaceRoot == root && strings.TrimSpace(e.Before.Head.Stdout) == commit && strings.TrimSpace(e.After.Head.Stdout) == commit && e.Before.HostExecutable == e.After.HostExecutable && e.Before.HostExecutableSHA256 == e.After.HostExecutableSHA256
}
