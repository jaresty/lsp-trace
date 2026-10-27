//go:build linux || darwin

package publication

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"golang.org/x/sys/unix"
)

func executionJournalFixturePrivate(t *testing.T) (*ContentRetentionRoot, *ScopedExecutionJournalPrivate, string) {
	t.Helper()
	path, _, _ := observedRootFixture(t)
	r, err := OpenContentRetentionRoot(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := r.Close(); err != nil {
			t.Error(err)
		}
	})
	selector := scopedExecutionHeadPrefixPrivate + strings.Repeat("a", 64) + ".json"
	j, err := r.OpenScopedExecutionJournalPrivate(selector)
	if err != nil {
		t.Fatal(err)
	}
	return r, j, path
}
func TestScopedExecutionJournalPrivateAtomicAndRestart(t *testing.T) {
	r, j, path := executionJournalFixturePrivate(t)
	ctx := context.Background()
	if raw, err := j.Load(ctx); raw != nil || err != nil {
		t.Fatalf("absence %q %v", raw, err)
	}
	want := []byte("exact claimed bytes")
	receipt, err := j.CompareAbsent(ctx, want)
	if err != nil || receipt == nil || !receipt.Committed || receipt.Outcome != CASOutcomeCommitted || receipt.CloseStatus != CloseComplete || receipt.DirectorySyncStatus != DirectorySyncComplete || receipt.VerificationStatus != "VERIFIED" || receipt.Digest != DigestForTest(want) {
		t.Fatalf("%+v %v", receipt, err)
	}
	want[0] = 'X'
	restarted, err := r.OpenScopedExecutionJournalPrivate(j.selector)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := restarted.Load(ctx)
	if err != nil || string(raw) != "exact claimed bytes" {
		t.Fatalf("%q %v", raw, err)
	}
	receipt, err = restarted.CompareAbsent(ctx, []byte("overwrite"))
	if receipt == nil || receipt.Committed || err != ErrStalePredecessor {
		t.Fatalf("overwrite %+v %v", receipt, err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.Load(ctx); err == nil {
		t.Fatal("closed borrowed root")
	}
	reopened, err := OpenContentRetentionRoot(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := reopened.Close(); err != nil {
			t.Error(err)
		}
	}()
	disk, err := reopened.OpenScopedExecutionJournalPrivate(j.selector)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = disk.Load(ctx)
	if err != nil || string(raw) != "exact claimed bytes" {
		t.Fatalf("reopened %q %v", raw, err)
	}
}
func TestScopedExecutionJournalPrivateConcurrent(t *testing.T) {
	_, j, _ := executionJournalFixturePrivate(t)
	var wg sync.WaitGroup
	results := make(chan *CompareAndReplaceReceipt, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := j.CompareAbsent(context.Background(), []byte("claim"))
			results <- r
			errs <- e
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	commits, conflicts := 0, 0
	for r := range results {
		if r != nil && r.Committed {
			commits++
		}
	}
	for e := range errs {
		if e == ErrStalePredecessor {
			conflicts++
		} else if e != nil {
			t.Fatal(e)
		}
	}
	if commits != 1 || conflicts != 1 {
		t.Fatalf("commits=%d conflicts=%d", commits, conflicts)
	}
}
func TestScopedExecutionJournalPrivateLoadAndBounds(t *testing.T) {
	r, j, path := executionJournalFixturePrivate(t)
	ctx := context.Background()
	for _, raw := range [][]byte{{}, []byte("present"), bytes.Repeat([]byte("x"), scopedExecutionHeadLimitPrivate+1)} {
		if err := os.WriteFile(filepath.Join(path, j.selector), raw, 0600); err != nil {
			t.Fatal(err)
		}
		got, err := j.Load(ctx)
		if len(raw) > scopedExecutionHeadLimitPrivate {
			if err == nil {
				t.Fatal("over cap")
			}
		} else if err != nil || got == nil || !bytes.Equal(raw, got) {
			t.Fatalf("present %q %v", got, err)
		}
	}
	for _, bad := range []string{"", "../" + j.selector, strings.ToUpper(j.selector), j.selector + "x", strings.Replace(j.selector, "aaaa", "gggg", 1)} {
		if _, err := (*ContentRetentionRoot)(nil).OpenScopedExecutionJournalPrivate(bad); err != ErrContentObservation {
			t.Fatalf("not rejected before root IO: %v", err)
		}
	}
	for _, raw := range [][]byte{nil, {}, make([]byte, scopedExecutionHeadLimitPrivate+1)} {
		if _, err := j.CompareAbsent(ctx, raw); err == nil {
			t.Fatal("invalid bytes")
		}
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := j.CompareAbsent(ctx, []byte("claim")); err == nil {
		t.Fatal("closed root")
	}
}

type executionFaultFilePrivate struct {
	observedFile
	readErr, closeErr error
}

func (f executionFaultFilePrivate) Read(p []byte) (int, error) {
	if f.readErr != nil {
		return 0, f.readErr
	}
	return f.observedFile.Read(p)
}
func (f executionFaultFilePrivate) Close() error {
	return errors.Join(f.observedFile.Close(), f.closeErr)
}

func TestScopedExecutionJournalPrivateLoadUncertainty(t *testing.T) {
	_, j, path := executionJournalFixturePrivate(t)
	if err := os.WriteFile(filepath.Join(path, j.selector), []byte("present"), 0600); err != nil {
		t.Fatal(err)
	}
	a, b := errors.New("read original"), errors.New("close original")
	ops := defaultScopedExecutionOpsPrivate()
	open := ops.read.open
	ops.read.open = func(r *Root, s string) (observedFile, error) {
		f, e := open(r, s)
		if e != nil {
			return f, e
		}
		return executionFaultFilePrivate{f, a, b}, nil
	}
	if raw, err := loadScopedExecutionPrivate(context.Background(), j, ops); raw != nil || !errors.Is(err, a) || !errors.Is(err, b) {
		t.Fatalf("lost causes: %q %v", raw, err)
	}
	// ENOENT after an affirmative lstat is observation uncertainty, not absence.
	ops = defaultScopedExecutionOpsPrivate()
	ops.read.open = func(*Root, string) (observedFile, error) { return nil, os.ErrNotExist }
	if _, err := loadScopedExecutionPrivate(context.Background(), j, ops); err == nil {
		t.Fatal("open ENOENT became absence")
	}
	ops = defaultScopedExecutionOpsPrivate()
	ops.read.lstat = func(*Root, string) (os.FileInfo, error) { return nil, errors.Join(os.ErrNotExist, b) }
	if _, err := loadScopedExecutionPrivate(context.Background(), j, ops); !errors.Is(err, b) {
		t.Fatalf("mixed absence lost %v", err)
	}
}
func TestScopedExecutionJournalPrivateFaults(t *testing.T) {
	for _, kind := range []string{"source-sync", "source-read", "source-hash", "install", "mixed-exist", "directory-sync", "final-read", "final-close", "source-close", "descriptor-close", "cancel-before", "cancel-after"} {
		t.Run(kind, func(t *testing.T) {
			_, j, _ := executionJournalFixturePrivate(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			a, b := errors.New("original operation"), errors.New("original cleanup")
			ops := defaultScopedExecutionOpsPrivate()
			fdclose := ops.closeFD
			ops.closeFD = func(fd int) error { return errors.Join(fdclose(fd), b) }
			committed := false
			switch kind {
			case "source-sync":
				ops.syncFile = func(*os.File) error { return a }
			case "source-read":
				ops.reread = func(*os.File, int64) ([]byte, error) { return nil, a }
			case "source-hash":
				ops.reread = func(*os.File, int64) ([]byte, error) { return []byte("wrong"), nil }
			case "install":
				ops.install = func(int, int, string) error { return a }
			case "mixed-exist":
				ops.install = func(int, int, string) error { return errors.Join(unix.EEXIST, a) }
			case "directory-sync":
				committed = true
				ops.syncDir = func(int) error { return a }
			case "final-read", "final-close":
				committed = true
				open := ops.read.open
				ops.read.open = func(r *Root, s string) (observedFile, error) {
					f, e := open(r, s)
					if e != nil {
						return f, e
					}
					if kind == "final-read" {
						return executionFaultFilePrivate{f, a, nil}, nil
					}
					return executionFaultFilePrivate{f, nil, a}, nil
				}
			case "source-close":
				committed = true
				close := ops.closeFile
				ops.closeFile = func(f *os.File) error { return errors.Join(close(f), a) }
			case "descriptor-close":
				committed = true
			case "cancel-before":
				cancel()
			case "cancel-after":
				committed = true
				install := ops.install
				ops.install = func(s, p int, n string) error { e := install(s, p, n); cancel(); return e }
			}
			receipt, err := compareScopedExecutionPrivate(ctx, j, []byte("claim"), ops)
			if err == nil || errors.Is(err, ErrStalePredecessor) {
				t.Fatalf("uncertainty became success/conflict: %v", err)
			}
			if kind != "cancel-before" && !errors.Is(err, b) {
				t.Fatalf("lost descriptor close: %v", err)
			}
			if kind != "source-hash" && kind != "descriptor-close" && kind != "cancel-before" && kind != "cancel-after" && !errors.Is(err, a) {
				t.Fatalf("lost original: %v", err)
			}
			if committed {
				if receipt == nil || !receipt.Committed || receipt.Digest != DigestForTest([]byte("claim")) || receipt.CloseStatus != CloseFailed {
					t.Fatalf("lost commit %+v %v", receipt, err)
				}
			} else if receipt != nil && receipt.Committed {
				t.Fatal("false commit")
			}
			if kind == "cancel-before" || kind == "cancel-after" {
				if !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			}
		})
	}
}
