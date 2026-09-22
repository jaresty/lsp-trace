package censuscontinuation

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type observingStore struct {
	base Store

	mu        sync.Mutex
	gets      map[string]int
	overrides map[string][]byte
}

func newObservingStore(base Store) *observingStore {
	return &observingStore{base: base, gets: make(map[string]int), overrides: make(map[string][]byte)}
}

func (s *observingStore) Put(ctx context.Context, raw []byte) (string, error) {
	return s.base.Put(ctx, raw)
}

func (s *observingStore) Get(ctx context.Context, selector string) ([]byte, error) {
	s.mu.Lock()
	s.gets[selector]++
	override, changed := s.overrides[selector]
	s.mu.Unlock()
	if changed {
		return append([]byte(nil), override...), nil
	}
	return s.base.Get(ctx, selector)
}

func (s *observingStore) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gets = make(map[string]int)
}

func (s *observingStore) replace(selector string, raw []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.overrides[selector] = append([]byte(nil), raw...)
}

func (s *observingStore) counts() map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]int, len(s.gets))
	for selector, count := range s.gets {
		out[selector] = count
	}
	return out
}

func completeVerificationFixture(t *testing.T) (*observingStore, string) {
	t.Helper()
	fixture := newFixture(t)
	handoff, err := BuildHandoff(fixture.input)
	if err != nil {
		t.Fatal(err)
	}
	store := newObservingStore(NewMemoryStore())
	request := testManagedRequest(t, fixture, Request{
		Handoff: handoff, Workspace: fixture.workspace, Worker: &countingWorker{},
		Store: store, Contract: testContract(t),
	})
	result := Run(context.Background(), request)
	if result.Status != StatusComplete || result.Err != nil {
		t.Fatalf("ASSERT_VERIFICATION_COUNT_FIXTURE: %+v", result)
	}
	store.reset()
	return store, result.CheckpointID
}

func TestVerifyChainReadsAndParsesEachStoreObjectOncePerOperation(t *testing.T) {
	store, selector := completeVerificationFixture(t)
	var stats verificationStats
	if _, err := verifyChain(context.Background(), store, selector, &stats); err != nil {
		t.Fatal(err)
	}
	for object, count := range store.counts() {
		if count != 1 {
			t.Fatalf("ASSERT_VERIFY_CHAIN_ONE_READ_PER_OBJECT: selector=%s reads=%d", object, count)
		}
	}
	for object, count := range stats.ParseCounts {
		if count > 1 {
			t.Fatalf("ASSERT_VERIFY_CHAIN_ONE_PARSE_PER_OBJECT: selector=%s kind=%s parses=%d", object.selector, object.kind, count)
		}
	}
}

func TestVerifyChainDoesNotReuseObjectsAcrossOperations(t *testing.T) {
	store, selector := completeVerificationFixture(t)
	if _, err := VerifyChain(context.Background(), store, selector); err != nil {
		t.Fatal(err)
	}
	store.reset()
	store.replace(selector, []byte(`{"changed":true}`))
	if _, err := VerifyChain(context.Background(), store, selector); err == nil {
		t.Fatal("ASSERT_VERIFY_CHAIN_REREADS_CHANGED_STORE_OBJECT")
	}
	if got := store.counts()[selector]; got != 1 {
		t.Fatal(errors.New("ASSERT_VERIFY_CHAIN_CHANGED_SELECTOR_READ_COUNT"))
	}
}
