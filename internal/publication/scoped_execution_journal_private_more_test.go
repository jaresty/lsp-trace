//go:build darwin || linux

package publication

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestScopedExecutionJournalPrivateMetadataAndCap(t *testing.T) {
	_, j, path := executionJournalFixturePrivate(t)
	ctx := context.Background()
	raw := bytes.Repeat([]byte("x"), scopedExecutionHeadLimitPrivate)
	if r, e := j.CompareAbsent(ctx, raw); e != nil || r == nil || !r.Committed {
		t.Fatalf("cap %+v %v", r, e)
	}
	if got, e := j.Load(ctx); e != nil || !bytes.Equal(got, raw) {
		t.Fatalf("cap load %v", e)
	}
	target := filepath.Join(path, j.selector)
	if err := os.Chmod(target, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := j.Load(ctx); err == nil {
		t.Fatal("public mode admitted")
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("missing", target); err != nil {
		t.Fatal(err)
	}
	if _, err := j.Load(ctx); err == nil {
		t.Fatal("symlink missing target became absent")
	}
	r, e := j.CompareAbsent(ctx, []byte("refuse"))
	if e == nil || r != nil && r.Committed {
		t.Fatalf("symlink %+v %v", r, e)
	}
	if _, err := os.Lstat(filepath.Join(path, "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("symlink target created: %v", err)
	}
	if info, err := os.Lstat(target); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink replaced: %v", err)
	}
}
func TestScopedExecutionJournalPrivateExistCleanupAndCancel(t *testing.T) {
	for _, kind := range []string{"close", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			_, j, path := executionJournalFixturePrivate(t)
			if err := os.WriteFile(filepath.Join(path, j.selector), nil, 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cause := errors.New("close original")
			ops := defaultScopedExecutionOpsPrivate()
			if kind == "close" {
				close := ops.closeFile
				ops.closeFile = func(f *os.File) error { return errors.Join(close(f), cause) }
			} else {
				cause = context.Canceled
				install := ops.install
				ops.install = func(s, p int, n string) error { err := install(s, p, n); cancel(); return err }
			}
			r, err := compareScopedExecutionPrivate(ctx, j, []byte("claim"), ops)
			if r != nil || !errors.Is(err, cause) || !errors.Is(err, unix.EEXIST) || errors.Is(err, ErrStalePredecessor) {
				t.Fatalf("%+v %v", r, err)
			}
			got, e := j.Load(context.Background())
			if e != nil || got == nil || len(got) != 0 {
				t.Fatalf("overwrote empty %q %v", got, e)
			}
		})
	}
}
func TestScopedExecutionJournalPrivateAllPostCommitCauses(t *testing.T) {
	_, j, _ := executionJournalFixturePrivate(t)
	syncCause, readCause, closeCause, fdCause := errors.New("directory sync"), errors.New("read"), errors.New("read close"), errors.New("descriptor close")
	ops := defaultScopedExecutionOpsPrivate()
	ops.syncDir = func(int) error { return syncCause }
	open := ops.read.open
	ops.read.open = func(r *Root, n string) (observedFile, error) {
		f, e := open(r, n)
		if e != nil {
			return f, e
		}
		return executionFaultFilePrivate{f, readCause, closeCause}, nil
	}
	close := ops.closeFD
	ops.closeFD = func(fd int) error { return errors.Join(close(fd), fdCause) }
	r, err := compareScopedExecutionPrivate(context.Background(), j, []byte("claim"), ops)
	if r == nil || !r.Committed || r.CloseStatus != CloseFailed || r.DirectorySyncStatus != DirectorySyncFailed || r.Outcome != CASOutcomeCommittedVerificationFailed {
		t.Fatalf("receipt %+v", r)
	}
	for _, cause := range []error{syncCause, readCause, closeCause, fdCause} {
		if !errors.Is(err, cause) {
			t.Fatalf("lost %v in %v", cause, err)
		}
	}
}
func TestScopedExecutionJournalPrivateAbsentCancellation(t *testing.T) {
	_, j, _ := executionJournalFixturePrivate(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ops := defaultScopedExecutionOpsPrivate()
	ops.read.lstat = func(*Root, string) (os.FileInfo, error) { cancel(); return nil, os.ErrNotExist }
	if _, err := loadScopedExecutionPrivate(ctx, j, ops); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel became absence: %v", err)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := j.Load(context.Background()); err == nil {
		t.Fatal("closed journal admitted")
	}
}
