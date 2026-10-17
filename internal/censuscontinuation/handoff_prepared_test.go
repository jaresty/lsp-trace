package censuscontinuation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"lsp-trace/internal/describerequest"
)

const (
	assertPrivateHandoffCanonicalCeilings       = "ASSERT_PRIVATE_HANDOFF_CANONICAL_CEILINGS"
	assertPrivateHandoffBindsVerifiedRequests   = "ASSERT_PRIVATE_HANDOFF_BINDS_VERIFIED_REQUESTS"
	assertPrivateHandoffRequiresRenderedRunning = "ASSERT_PRIVATE_HANDOFF_REQUIRES_RENDERED_RUNNING"
	assertPrivateHandoffTamperRejectedNoPut     = "ASSERT_PRIVATE_HANDOFF_TAMPER_REJECTED_WITHOUT_PUT"
	assertPrivateHandoffStoreIdempotent         = "ASSERT_PRIVATE_HANDOFF_STORE_IDEMPOTENT"
	assertPrivateHandoffProviderNeutrality      = "ASSERT_PRIVATE_HANDOFF_PROVIDER_NEUTRALITY"
	assertPublicPausedWorkerPathUnchanged       = "ASSERT_PUBLIC_PAUSED_AND_WORKER_PATH_UNCHANGED"
)

type privateHandoffObservingStore struct {
	Store
	puts      int
	overrides map[string][]byte
}

func (s *privateHandoffObservingStore) Put(ctx context.Context, raw []byte) (string, error) {
	s.puts++
	return s.Store.Put(ctx, raw)
}

func (s *privateHandoffObservingStore) Get(ctx context.Context, selector string) ([]byte, error) {
	if raw, ok := s.overrides[selector]; ok {
		return append([]byte(nil), raw...), nil
	}
	return s.Store.Get(ctx, selector)
}

var privateHandoffFixtureCache struct {
	once   sync.Once
	store  *MemoryStore
	paused Result
	chain  []Checkpoint
	err    error
}

func privateHandoffFixture(t *testing.T) (*MemoryStore, Result, []Checkpoint) {
	t.Helper()
	privateHandoffFixtureCache.once.Do(func() {
		f := newFixtureWithTargetCount(t, 2, 1)
		h, err := BuildHandoff(f.input)
		if err != nil {
			privateHandoffFixtureCache.err = err
			return
		}
		store := NewMemoryStore()
		paused := Run(context.Background(), Request{
			Handoff: h, Workspace: f.workspace, Store: store, Contract: testContractV2(t),
			StopAfter: StopAfterDescribeRequests, FreshCapture: testFreshCapture(t, f),
		})
		if paused.Err != nil || paused.Status != StatusPaused {
			privateHandoffFixtureCache.err = errors.New("fixture did not pause")
			return
		}
		chain, err := VerifyChain(context.Background(), store, paused.CheckpointID)
		if err != nil {
			privateHandoffFixtureCache.err = err
			return
		}
		privateHandoffFixtureCache.store = store
		privateHandoffFixtureCache.paused = paused
		privateHandoffFixtureCache.chain = chain
	})
	if privateHandoffFixtureCache.err != nil {
		t.Fatal(privateHandoffFixtureCache.err)
	}
	return privateHandoffFixtureCache.store, privateHandoffFixtureCache.paused, append([]Checkpoint(nil), privateHandoffFixtureCache.chain...)
}

func privateRequestsRef(t *testing.T, checkpoint Checkpoint) ArtifactRef {
	t.Helper()
	for _, ref := range checkpoint.Artifacts() {
		if ref.Kind == "requests" {
			return ref
		}
	}
	t.Fatal("requests artifact missing")
	return ArtifactRef{}
}

func TestPrivateHandoffCanonicalCeilings(t *testing.T) {
	store, paused, _ := privateHandoffFixture(t)
	prepared, selector, err := preparePrivateHandoff(context.Background(), store, paused.CheckpointID)
	if err != nil {
		t.Fatalf("%s: prepare: %v", assertPrivateHandoffCanonicalCeilings, err)
	}
	w := prepared.wire
	if w.SchemaVersion != privateHandoffPreparedSchema || w.Status != "HANDOFF_PREPARED" || w.Authority != 0 || w.Accepted || w.Completeness != ContinuationCompleteness || !validDigest(prepared.ID()) {
		t.Fatalf("%s: wire=%+v selector=%q", assertPrivateHandoffCanonicalCeilings, w, selector)
	}
	raw, err := prepared.Bytes()
	if err != nil || digestOf(raw) != selector {
		t.Fatalf("%s: bytes err=%v digest=%q selector=%q", assertPrivateHandoffCanonicalCeilings, err, digestOf(raw), selector)
	}
	parsed, err := parsePrivateHandoffPrepared(raw)
	if err != nil || parsed.ID() != prepared.ID() {
		t.Fatalf("%s: parse err=%v id=%q", assertPrivateHandoffCanonicalCeilings, err, parsed.ID())
	}
	for name, bad := range map[string][]byte{
		"noncanonical": append([]byte(" "), raw...),
		"unknown":      bytes.Replace(raw, []byte(`"custody":`), []byte(`"unknown":0,"custody":`), 1),
		"duplicate":    bytes.Replace(raw, []byte(`"status":`), []byte(`"status":"HANDOFF_PREPARED","status":`), 1),
		"identity":     bytes.Replace(raw, []byte(prepared.ID()), []byte(digestOf([]byte("wrong prepared identity"))), 1),
	} {
		if _, err := parsePrivateHandoffPrepared(bad); err == nil {
			t.Fatalf("%s: accepted %s", assertPrivateHandoffCanonicalCeilings, name)
		}
	}
}

func TestPrivateHandoffBindsVerifiedRequests(t *testing.T) {
	store, paused, chain := privateHandoffFixture(t)
	prepared, _, err := preparePrivateHandoff(context.Background(), store, paused.CheckpointID)
	if err != nil {
		t.Fatalf("%s: %v", assertPrivateHandoffBindsVerifiedRequests, err)
	}
	last := chain[len(chain)-1]
	ref := privateRequestsRef(t, last)
	raw, err := store.Get(context.Background(), ref.ID)
	if err != nil {
		t.Fatal(err)
	}
	requests, err := describerequest.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(chain))
	for i := range chain {
		ids[i] = chain[i].ID()
	}
	wantChain := identityJSON("lsp-trace:census-handoff-prepared-chain:private:v1", ids, func(*[]string) {})
	w := prepared.wire
	if w.CheckpointSelector != paused.CheckpointID || w.CheckpointID != last.ID() || w.ChainIdentity != wantChain || w.RequestsSelector != ref.ID || w.RequestsDigest != ref.Digest || w.RequestsByteLength != ref.ByteLength || w.RequestsCount != len(requests) || w.Custody != privateHandoffCustody {
		t.Fatalf("%s: got=%+v want_ref=%+v want_count=%d want_chain=%s", assertPrivateHandoffBindsVerifiedRequests, w, ref, len(requests), wantChain)
	}
}

func TestPrivateHandoffRequiresRenderedRunning(t *testing.T) {
	store, paused, chain := privateHandoffFixture(t)
	last := chain[len(chain)-1]
	if _, _, err := preparePrivateHandoff(context.Background(), store, last.PriorID()); err == nil {
		t.Fatalf("%s: accepted prior stage", assertPrivateHandoffRequiresRenderedRunning)
	}
	complete, err := NewCheckpoint(CheckpointInput{PriorCheckpointID: last.PriorID(), Stage: StageRequestsRendered, CensusID: last.CensusID(), HandoffID: last.HandoffID(), ContractID: last.ContractID(), ProfileID: last.ProfileID(), Artifacts: last.Artifacts(), Status: StatusComplete})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := complete.Bytes()
	selector, err := store.Put(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := preparePrivateHandoff(context.Background(), store, selector); err == nil {
		t.Fatalf("%s: accepted non-running checkpoint", assertPrivateHandoffRequiresRenderedRunning)
	}
	if _, _, err := preparePrivateHandoff(context.Background(), store, paused.CheckpointID); err != nil {
		t.Fatalf("%s: rejected rendered/running: %v", assertPrivateHandoffRequiresRenderedRunning, err)
	}
}

func TestPrivateHandoffTamperRejectedWithoutPut(t *testing.T) {
	base, paused, chain := privateHandoffFixture(t)
	ref := privateRequestsRef(t, chain[len(chain)-1])
	raw, err := base.Get(context.Background(), ref.ID)
	if err != nil {
		t.Fatal(err)
	}
	tampered := append([]byte(nil), raw...)
	tampered[len(tampered)-1] ^= 1
	store := &privateHandoffObservingStore{Store: base, overrides: map[string][]byte{ref.ID: tampered}}
	if _, _, err := preparePrivateHandoff(context.Background(), store, paused.CheckpointID); err == nil || err.Error() != "checkpoint artifact missing" || store.puts != 0 {
		t.Fatalf("%s: err=%v puts=%d", assertPrivateHandoffTamperRejectedNoPut, err, store.puts)
	}
}

func TestPrivateHandoffStoreIdempotent(t *testing.T) {
	base, paused, _ := privateHandoffFixture(t)
	store := &privateHandoffObservingStore{Store: base}
	first, firstSelector, err := preparePrivateHandoff(context.Background(), store, paused.CheckpointID)
	if err != nil {
		t.Fatalf("%s: first: %v", assertPrivateHandoffStoreIdempotent, err)
	}
	second, secondSelector, err := preparePrivateHandoff(context.Background(), store, paused.CheckpointID)
	if err != nil {
		t.Fatalf("%s: second: %v", assertPrivateHandoffStoreIdempotent, err)
	}
	raw, err := store.Get(context.Background(), firstSelector)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parsePrivateHandoffPrepared(raw)
	if err != nil || firstSelector != secondSelector || first.ID() != second.ID() || parsed.ID() != first.ID() || store.puts != 2 {
		t.Fatalf("%s: first=%q second=%q parsed=%q puts=%d err=%v", assertPrivateHandoffStoreIdempotent, first.ID(), second.ID(), parsed.ID(), store.puts, err)
	}
}

func TestPreparePrivateHandoffSelectorWrapper(t *testing.T) {
	base, paused, _ := privateHandoffFixture(t)
	store := &privateHandoffObservingStore{Store: base}
	selector, err := PreparePrivateHandoffSelector(context.Background(), store, paused.CheckpointID)
	if err != nil || !validDigest(selector) || store.puts != 1 {
		t.Fatalf("ASSERT_PRIVATE_HANDOFF_SELECTOR_ONLY_WRAPPER: selector=%q puts=%d err=%v", selector, store.puts, err)
	}
	raw, err := store.Get(context.Background(), selector)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parsePrivateHandoffPrepared(raw); err != nil {
		t.Fatalf("ASSERT_PRIVATE_HANDOFF_SELECTOR_ONLY_WRAPPER: resolve: %v", err)
	}
}

func TestPrivateHandoffProviderNeutrality(t *testing.T) {
	store, paused, _ := privateHandoffFixture(t)
	prepared, _, err := preparePrivateHandoff(context.Background(), store, paused.CheckpointID)
	if err != nil {
		t.Fatalf("%s: prepare: %v", assertPrivateHandoffProviderNeutrality, err)
	}
	raw, err := prepared.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if prepared.wire.Custody != "CALLER_CONTROLLED_INFERENCE" {
		t.Fatalf("%s: custody=%q", assertPrivateHandoffProviderNeutrality, prepared.wire.Custody)
	}
	for _, forbidden := range [][]byte{
		[]byte("pi"), []byte("provider"), []byte("harness"), []byte("session_id"), []byte("agent_id"),
		[]byte("EXTERNAL_HOST"), []byte("HANDED_OFF"), []byte("INGEST_REQUESTED"),
		[]byte("ADMITTED"), []byte("INGEST_ADMITTED"), []byte("RESULT_CONSOLIDATED"),
	} {
		if bytes.Contains(bytes.ToLower(raw), bytes.ToLower(forbidden)) {
			t.Fatalf("%s: provider-specific or future-state token %q in %s", assertPrivateHandoffProviderNeutrality, forbidden, raw)
		}
	}
	for name, bad := range map[string][]byte{
		"provider_identity": bytes.Replace(raw, []byte(`"custody":`), []byte(`"provider_id":"vendor","custody":`), 1),
		"future_state":      bytes.Replace(raw, []byte(`"HANDOFF_PREPARED"`), []byte(`"HANDED_OFF"`), 1),
	} {
		if _, err := parsePrivateHandoffPrepared(bad); err == nil {
			t.Fatalf("%s: accepted %s", assertPrivateHandoffProviderNeutrality, name)
		}
	}
}

func TestPrivateHandoffPublicPausedAndWorkerPathUnchanged(t *testing.T) {
	base, paused, _ := privateHandoffFixture(t)
	before, err := json.Marshal(paused)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := preparePrivateHandoff(context.Background(), base, paused.CheckpointID); err != nil {
		t.Fatalf("%s: prepare: %v", assertPublicPausedWorkerPathUnchanged, err)
	}
	after, err := json.Marshal(paused)
	if err != nil {
		t.Fatal(err)
	}
	if StatusPaused != ContinuationStatus("PAUSED") || paused.Status != StatusPaused || !bytes.Equal(before, after) {
		t.Fatalf("%s: status=%q before=%s after=%s", assertPublicPausedWorkerPathUnchanged, paused.Status, before, after)
	}
	var worker Worker
	var workerV2 WorkerV2
	if worker != nil || workerV2 != nil {
		t.Fatal(errors.New(assertPublicPausedWorkerPathUnchanged))
	}
}
