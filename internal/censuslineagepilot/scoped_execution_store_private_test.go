//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"lsp-trace/internal/publication"
)

func realExecutionStorePrivate(t *testing.T, f executionClaimFixturePrivate, o *scopedExecutionClaimOwnerPrivate) *scopedExecutionStorePrivate {
	t.Helper()
	s, err := newScopedExecutionStorePrivate(context.Background(), o, f.roots)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return s
}
func TestScopedExecutionStorePrivateClaimAndRestart(t *testing.T) {
	f := executionClaimFixture(t)
	before := restartSnapshot(t, f.inputPaths)
	o := f.owner(t)
	s := realExecutionStorePrivate(t, f, o)
	out, err := o.claim(context.Background(), s)
	if err != nil || out.pending == nil || !o.ownsPending(out.pending) || !scopedExecutionGoodCASPrivate(out.cas, o.selector, o.raw) {
		t.Fatalf("%+v %v", out, err)
	}
	got, err := s.Load(context.Background(), o.selector)
	if err != nil || !bytes.Equal(got, o.raw) {
		t.Fatalf("%q %v", got, err)
	}
	next := f.owner(t)
	reopened := realExecutionStorePrivate(t, f, next)
	out, err = next.claim(context.Background(), reopened)
	if !errors.Is(err, ErrIndeterminateExecution) || out.pending != nil || out.cas != nil {
		t.Fatalf("restart regained pending %+v %v", out, err)
	}
	if !reflect.DeepEqual(before, restartSnapshot(t, f.inputPaths)) {
		t.Fatal("input changed")
	}
}
func TestScopedExecutionStorePrivateTwoOwners(t *testing.T) {
	f := executionClaimFixture(t)
	a, b := f.owner(t), f.owner(t)
	stores := []*scopedExecutionStorePrivate{realExecutionStorePrivate(t, f, a), realExecutionStorePrivate(t, f, b)}
	var wg sync.WaitGroup
	results := make(chan scopedExecutionClaimResultPrivate, 2)
	for i, o := range []*scopedExecutionClaimOwnerPrivate{a, b} {
		wg.Add(1)
		go func(i int, o *scopedExecutionClaimOwnerPrivate) {
			defer wg.Done()
			r, _ := o.claim(context.Background(), stores[i])
			results <- r
		}(i, o)
	}
	wg.Wait()
	close(results)
	pending := 0
	for r := range results {
		if r.pending != nil {
			pending++
		}
	}
	if pending != 1 {
		t.Fatalf("pending=%d", pending)
	}
}

// The decorator injects an error only after the actual primitive returned its
// actual commit receipt. It tests claim qualification, not simulated durability.
type executionStorePostFaultPrivate struct {
	*scopedExecutionStorePrivate
	fault  error
	calls  int
	cancel context.CancelFunc
}

func (s *executionStorePostFaultPrivate) CompareAbsent(c context.Context, k string, b []byte) (*publication.CompareAndReplaceReceipt, error) {
	s.calls++
	r, e := s.scopedExecutionStorePrivate.CompareAbsent(c, k, b)
	if s.cancel != nil {
		s.cancel()
	}
	return r, errors.Join(e, s.fault)
}
func TestScopedExecutionStorePrivatePostCommitNoPendingOrRetry(t *testing.T) {
	for _, kind := range []string{"error", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			f := executionClaimFixture(t)
			o := f.owner(t)
			s := &executionStorePostFaultPrivate{scopedExecutionStorePrivate: realExecutionStorePrivate(t, f, o)}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cause := errors.New("ambiguous after actual commit")
			if kind == "error" {
				s.fault = cause
			} else {
				s.cancel = cancel
				cause = context.Canceled
			}
			out, err := o.claim(ctx, s)
			if !errors.Is(err, cause) || !out.commitReported || out.pending != nil || out.cas == nil || !out.cas.Committed {
				t.Fatalf("%+v %v", out, err)
			}
			_, _ = o.claim(context.Background(), s)
			if s.calls != 1 {
				t.Fatal("retried")
			}
			next := f.owner(t)
			out, err = next.claim(context.Background(), realExecutionStorePrivate(t, f, next))
			if err == nil || out.pending != nil || out.cas != nil {
				t.Fatal("files regained pending")
			}
		})
	}
}
func TestScopedExecutionStorePrivateEmptyAndLifetime(t *testing.T) {
	f := executionClaimFixture(t)
	o := f.owner(t)
	s := realExecutionStorePrivate(t, f, o)
	// Fixtures own these roots; obtain the path only from the existing test
	// fixture's registered root helper, not from a production wrapper escape hatch.
	// Use a separate explicitly owned root for a zero-byte preexisting head.
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	home, err = filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatal(err)
	}
	path, err := os.MkdirTemp(home, ".lsp-trace-execution-empty-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(path); err != nil {
			t.Error(err)
		}
	})
	root, err := publication.OpenContentRetentionRoot(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	f.roots[2] = root
	o = f.owner(t)
	s = realExecutionStorePrivate(t, f, o)
	if err := os.WriteFile(filepath.Join(path, o.selector), nil, 0600); err != nil {
		t.Fatal(err)
	}
	out, err := o.claim(context.Background(), s)
	if err == nil || out.pending != nil || out.cas != nil {
		t.Fatal("empty treated as absent")
	}
	raw, err := s.Load(context.Background(), o.selector)
	if err != nil || raw == nil || len(raw) != 0 {
		t.Fatalf("%q %v", raw, err)
	}
	if _, err := s.Load(context.Background(), "../"+o.selector); err == nil {
		t.Fatal("selector escape")
	}
	if err := f.roots[0].Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(context.Background(), o.selector); err == nil {
		t.Fatal("closed primary admitted")
	}
}
