package vcssymbolsidecar

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type GitWorktreeProvider struct{ Timeout time.Duration }

func (g GitWorktreeProvider) Open(parent context.Context, repository, revision string) (HistoricalWorkspace, error) {
	timeout := g.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	real, err := filepath.EvalSymlinks(repository)
	if err != nil || !filepath.IsAbs(real) || filepath.Clean(real) != real {
		return HistoricalWorkspace{}, errors.New("invalid Git repository root")
	}
	top, err := symbolGitOutput(ctx, real, 64*1024, "rev-parse", "--show-toplevel")
	if err != nil {
		return HistoricalWorkspace{}, errors.New("resolve Git repository root")
	}
	realTop, err := filepath.EvalSymlinks(strings.TrimSpace(string(top)))
	if err != nil || realTop != real {
		return HistoricalWorkspace{}, errors.New("repository must equal Git top-level")
	}
	exact, err := symbolResolveCommit(ctx, real, revision)
	if err != nil {
		return HistoricalWorkspace{}, fmt.Errorf("resolve worktree revision: %w", err)
	}
	root, err := os.MkdirTemp("", "lsp-trace-symbol-worktree-")
	if err != nil {
		return HistoricalWorkspace{}, err
	}
	if err = os.Remove(root); err != nil {
		return HistoricalWorkspace{}, err
	}
	if _, err = symbolGitOutput(ctx, real, 64*1024, "worktree", "add", "--detach", root, exact); err != nil {
		_ = os.RemoveAll(root)
		return HistoricalWorkspace{}, fmt.Errorf("create detached worktree: %w", err)
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		_, _ = symbolGitOutput(ctx, real, 64*1024, "worktree", "remove", "--force", root)
		_ = os.RemoveAll(root)
		return HistoricalWorkspace{}, errors.New("resolve detached worktree root")
	}
	var once sync.Once
	var closeErr error
	closeFn := func() error {
		once.Do(func() {
			cleanupCtx, done := context.WithTimeout(context.Background(), timeout)
			defer done()
			_, removeErr := symbolGitOutput(cleanupCtx, real, 64*1024, "worktree", "remove", "--force", root)
			diskErr := os.RemoveAll(root)
			if removeErr != nil {
				closeErr = fmt.Errorf("remove Git worktree: %w", removeErr)
			} else if diskErr != nil {
				closeErr = diskErr
			}
		})
		return closeErr
	}
	return HistoricalWorkspace{Root: canonicalRoot, Revision: exact, Close: closeFn}, nil
}
