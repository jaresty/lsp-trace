package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censuschunking"
	"lsp-trace/internal/publication"
)

func privateAdapterRoot(t *testing.T) *publication.Root {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return root
}

func privateHeadBytes(t *testing.T, generation uint64, suffix string) []byte {
	t.Helper()
	withoutIdentity := fmt.Sprintf(`{"schema_version":"%s","logical_digest":"","immutable_selector":"","workflow_key":"workflow%s","root_selector":"root","plan_digest":"plan","index_digest":"index","session_id":"session","session_generation":7,"workspace_digest":"workspace","generation":%d,"state":"COMMITTED","committed_prefix":0,"checkpoint_selector":"root","claim_ordinal":-1,"idempotency_key":"","receipt_selector":"","manifest_selector":"","terminal_selector":""}`, censuschunking.CoordinatorHeadVersion, suffix, generation)
	sum := sha256.Sum256(append(append([]byte("lsp-trace:census-chunk-coordinator-head:private:v1"), 0), withoutIdentity...))
	digest := "sha256:" + hex.EncodeToString(sum[:])
	selector := "census-chunk-heads/v1/" + hex.EncodeToString(sum[:])
	raw := bytes.Replace([]byte(withoutIdentity), []byte(`"logical_digest":""`), []byte(fmt.Sprintf(`"logical_digest":"%s"`, digest)), 1)
	raw = bytes.Replace(raw, []byte(`"immutable_selector":""`), []byte(fmt.Sprintf(`"immutable_selector":"%s"`, selector)), 1)
	if _, err := censuschunking.ParseCoordinatorHeadRevision(raw); err != nil {
		t.Fatalf("fixture: %v\n%s", err, raw)
	}
	return raw
}

func TestPrivateCensusCoordinatorStorePutGetIdempotencyTamperAndLimit(t *testing.T) {
	root := privateAdapterRoot(t)
	store, err := newPrivateCensusCoordinatorStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if store.MaxObjectBytes() != censuschunking.MaxCanonicalBytes {
		t.Fatalf("ASSERT_STORE_EXACT_LIMIT got=%d", store.MaxObjectBytes())
	}
	raw := []byte(`{"canonical":true}`)
	selector, err := store.Put(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := store.Put(context.Background(), raw); err != nil || again != selector {
		t.Fatalf("ASSERT_STORE_IDEMPOTENT selector=%q err=%v", again, err)
	}
	got, err := store.Get(context.Background(), selector)
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("ASSERT_STORE_GET got=%q err=%v", got, err)
	}
	got[0] = 'X'
	if reread, err := store.Get(context.Background(), selector); err != nil || !bytes.Equal(reread, raw) {
		t.Fatalf("ASSERT_STORE_CLONES got=%q err=%v", reread, err)
	}
	if _, err := store.Put(context.Background(), make([]byte, censuschunking.MaxCanonicalBytes+1)); err == nil {
		t.Fatal("ASSERT_STORE_REJECTS_OVERSIZE")
	}
	if err := os.WriteFile(filepath.Join(root.Path(), selector), []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(context.Background(), selector); err == nil {
		t.Fatal("ASSERT_STORE_REJECTS_TAMPER")
	}
	if _, err := store.Put(context.Background(), raw); err == nil {
		t.Fatal("ASSERT_STORE_IDEMPOTENT_REQUIRES_EXACT_READBACK")
	}
}

func TestPrivateCensusHeadStoreCASLoadRestartAndValidation(t *testing.T) {
	root := privateAdapterRoot(t)
	store, err := newPrivateCensusHeadStore(root)
	if err != nil {
		t.Fatal(err)
	}
	key := "workflow"
	first, second := privateHeadBytes(t, 1, "-first"), privateHeadBytes(t, 2, "-second")
	if err := store.CompareAndSwap(context.Background(), key, censuschunking.HeadRevision{}, first); err != nil {
		t.Fatalf("ASSERT_HEAD_ABSENT_CREATE: %v", err)
	}
	loaded, err := store.Load(context.Background(), key)
	if err != nil || !bytes.Equal(loaded, first) {
		t.Fatalf("ASSERT_HEAD_LOAD got=%q err=%v", loaded, err)
	}
	revision, _ := censuschunking.ParseCoordinatorHeadRevision(first)
	if err := store.CompareAndSwap(context.Background(), key, revision, second); err != nil {
		t.Fatalf("ASSERT_HEAD_EXACT_CAS: %v", err)
	}
	if err := store.CompareAndSwap(context.Background(), key, revision, first); !errors.Is(err, publication.ErrStalePredecessor) {
		t.Fatalf("ASSERT_HEAD_STALE_GENERATION: %v", err)
	}
	current, _ := censuschunking.ParseCoordinatorHeadRevision(second)
	staleSelector := current
	staleSelector.Selector += "-stale"
	if err := store.CompareAndSwap(context.Background(), key, staleSelector, first); !errors.Is(err, publication.ErrStalePredecessor) {
		t.Fatalf("ASSERT_HEAD_STALE_SELECTOR: %v", err)
	}
	if err := store.CompareAndSwap(context.Background(), key, current, append(bytes.Clone(first), '\n')); err == nil {
		t.Fatal("ASSERT_HEAD_REJECTS_NONCANONICAL_REPLACEMENT")
	}
	if err := store.CompareAndSwap(context.Background(), key, current, []byte(`{"bad":true}`)); err == nil {
		t.Fatal("ASSERT_HEAD_REJECTS_MALFORMED_REPLACEMENT")
	}
	restarted, err := newPrivateCensusHeadStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := restarted.Load(context.Background(), key); err != nil || !bytes.Equal(got, second) {
		t.Fatalf("ASSERT_HEAD_RESTART_LOAD got=%q err=%v", got, err)
	}
	selector, _ := privateCensusHeadSelector(key)
	if err := os.WriteFile(filepath.Join(root.Path(), selector), append(bytes.Clone(second), '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.Load(context.Background(), key); err == nil {
		t.Fatal("ASSERT_HEAD_REJECTS_NONCANONICAL_LOAD")
	}
	if err := os.WriteFile(filepath.Join(root.Path(), selector), []byte(`{"bad":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.Load(context.Background(), key); err == nil {
		t.Fatal("ASSERT_HEAD_REJECTS_MALFORMED_LOAD")
	}
}

func TestPrivateCensusHeadStoreConcurrentCASOneWinner(t *testing.T) {
	root := privateAdapterRoot(t)
	store, _ := newPrivateCensusHeadStore(root)
	key := "race"
	base := privateHeadBytes(t, 1, "-base")
	if err := store.CompareAndSwap(context.Background(), key, censuschunking.HeadRevision{}, base); err != nil {
		t.Fatal(err)
	}
	revision, _ := censuschunking.ParseCoordinatorHeadRevision(base)
	candidates := [][]byte{privateHeadBytes(t, 2, "-a"), privateHeadBytes(t, 2, "-b")}
	var winners atomic.Int32
	var wg sync.WaitGroup
	for _, candidate := range candidates {
		candidate := candidate
		wg.Add(1)
		go func() {
			defer wg.Done()
			if store.CompareAndSwap(context.Background(), key, revision, candidate) == nil {
				winners.Add(1)
			}
		}()
	}
	wg.Wait()
	if winners.Load() != 1 {
		t.Fatalf("ASSERT_HEAD_CONCURRENT_ONE_WINNER got=%d", winners.Load())
	}
}

func TestPrivateHeldCensusSessionExactMismatchCancellationAndCore(t *testing.T) {
	expected := censuschunking.ExpectedSession{SessionID: "s", Generation: 7, WorkspaceURI: "file:///workspace"}
	marker := func(censusacquisition.Phase) {}
	core := censusacquisition.Core{OnErrorPhase: marker}
	held := newPrivateHeldCensusSession(expected, core)
	calls := 0
	if err := held.WithReadySession(context.Background(), expected, func(got censusacquisition.Core) error {
		calls++
		if fmt.Sprintf("%p", got.OnErrorPhase) != fmt.Sprintf("%p", marker) {
			t.Fatal("ASSERT_HELD_CORE_PRESERVED")
		}
		return nil
	}); err != nil || calls != 1 {
		t.Fatalf("ASSERT_HELD_EXACT_ONCE calls=%d err=%v", calls, err)
	}
	mismatches := []censuschunking.ExpectedSession{{SessionID: "x", Generation: 7, WorkspaceURI: "file:///workspace"}, {SessionID: "s", Generation: 8, WorkspaceURI: "file:///workspace"}, {SessionID: "s", Generation: 7, WorkspaceURI: "file:///other"}}
	for _, mismatch := range mismatches {
		if err := held.WithReadySession(context.Background(), mismatch, func(censusacquisition.Core) error { calls++; return nil }); err == nil {
			t.Fatal("ASSERT_HELD_REJECTS_MISMATCH")
		}
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := held.WithReadySession(cancelled, expected, func(censusacquisition.Core) error { calls++; return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("ASSERT_HELD_CANCELLED: %v", err)
	}
	if calls != 1 {
		t.Fatalf("ASSERT_HELD_ZERO_CALLS_ON_REFUSAL got=%d", calls)
	}
}

type privateOuterLeaseCounter struct{ active, total atomic.Int32 }

func (f *privateOuterLeaseCounter) with(run func() error) error {
	f.total.Add(1)
	f.active.Add(1)
	defer f.active.Add(-1)
	return run()
}

func TestPrivateHeldCensusSessionOneOuterLeaseMultipleCalls(t *testing.T) {
	expected := censuschunking.ExpectedSession{SessionID: "s", Generation: 1, WorkspaceURI: "file:///w"}
	held := newPrivateHeldCensusSession(expected, censusacquisition.Core{})
	outer := &privateOuterLeaseCounter{}
	calls := 0
	if err := outer.with(func() error {
		for i := 0; i < 3; i++ {
			if err := held.WithReadySession(context.Background(), expected, func(censusacquisition.Core) error { calls++; return nil }); err != nil {
				return err
			}
		}
		if outer.active.Load() != 1 {
			t.Fatalf("ASSERT_OUTER_LEASE_ONE_ACTIVE got=%d", outer.active.Load())
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if outer.total.Load() != 1 || outer.active.Load() != 0 || calls != 3 {
		t.Fatalf("ASSERT_OUTER_LEASE_CARDINALITY total=%d active=%d calls=%d", outer.total.Load(), outer.active.Load(), calls)
	}
}
