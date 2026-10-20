package censuschunking

import (
	"context"
	"errors"
	"sync"
	"testing"

	"lsp-trace/internal/censusacquisition"
)

type coordinatorStore struct {
	mu        sync.Mutex
	objects   map[string][]byte
	limit     int64
	puts      int
	failAt    int
	truncate  bool
	collision bool
	panicPut  bool
}

func newCoordinatorStore(limit int64) *coordinatorStore {
	return &coordinatorStore{objects: map[string][]byte{}, limit: limit}
}
func (s *coordinatorStore) MaxObjectBytes() int64 { return s.limit }
func (s *coordinatorStore) Put(_ context.Context, b []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.panicPut {
		panic("put panic")
	}
	s.puts++
	if s.failAt > 0 && s.puts == s.failAt {
		return "", errors.New("publication sentinel")
	}
	if int64(len(b)) > s.limit {
		return "", errors.New("store limit sentinel")
	}
	id := digestBytes(b)
	if s.collision {
		id = digestBytes([]byte("collision"))
	}
	if old, ok := s.objects[id]; ok && string(old) != string(b) {
		return "", errors.New("collision")
	}
	s.objects[id] = append([]byte(nil), b...)
	return id, nil
}
func (s *coordinatorStore) Get(_ context.Context, id string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.objects[id]
	if !ok {
		return nil, errors.New("missing sentinel")
	}
	b = append([]byte(nil), b...)
	if s.truncate && len(b) > 0 {
		b = b[:len(b)-1]
	}
	return b, nil
}

type memoryHeadStore struct {
	mu       sync.Mutex
	raw      map[string][]byte
	failCAS  int
	cas      int
	panicCAS bool
}

func newHeadStore() *memoryHeadStore { return &memoryHeadStore{raw: map[string][]byte{}} }
func (h *memoryHeadStore) Load(_ context.Context, k string) ([]byte, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	b, ok := h.raw[k]
	if !ok {
		return nil, errors.New("head missing")
	}
	return append([]byte(nil), b...), nil
}
func (h *memoryHeadStore) CompareAndSwap(_ context.Context, k string, want HeadRevision, repl []byte) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.panicCAS {
		panic("CAS panic")
	}
	h.cas++
	if h.failCAS > 0 && h.cas == h.failCAS {
		return errors.New("CAS collision")
	}
	old, ok := h.raw[k]
	if !ok {
		if want != (HeadRevision{}) {
			return errors.New("CAS conflict")
		}
	} else {
		var x coordinatorHead
		if decodeCoordinator(old, &x) != nil || x.Generation != want.Generation || x.ImmutableSelector != want.Selector {
			return errors.New("CAS conflict")
		}
	}
	var next coordinatorHead
	if decodeCoordinator(repl, &next) != nil {
		return errors.New("bad replacement")
	}
	h.raw[k] = append([]byte(nil), repl...)
	return nil
}
func (h *memoryHeadStore) current() (string, coordinatorHead) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for k, b := range h.raw {
		var v coordinatorHead
		_ = decodeCoordinator(b, &v)
		return k, v
	}
	return "", coordinatorHead{}
}
func (h *memoryHeadStore) set(k string, v coordinatorHead) {
	setHeadIdentity(&v)
	b, _ := encodeCoordinator(v)
	h.mu.Lock()
	h.raw[k] = b
	h.mu.Unlock()
}

type leaseFunc func(context.Context, ExpectedSession, func(censusacquisition.Core) error) error

func (f leaseFunc) WithReadySession(ctx context.Context, e ExpectedSession, run func(censusacquisition.Core) error) error {
	return f(ctx, e, run)
}

type coreCounter struct {
	mu        sync.Mutex
	n         int
	t         *testing.T
	panicCore bool
}

func (c *coreCounter) core() censusacquisition.Core {
	return censusacquisition.Core{Acquirer: chunkAcquirerFunc(func(_ context.Context, r censusacquisition.BatchRequest) (censusacquisition.AcquiredV5, error) {
		c.mu.Lock()
		c.n++
		c.mu.Unlock()
		if c.panicCore {
			panic("core panic")
		}
		return censusacquisition.AcquiredV5{Session: r.Session, Raw: exactChunkV5(c.t, r)}, nil
	})}
}
func (c *coreCounter) count() int { c.mu.Lock(); defer c.mu.Unlock(); return c.n }

func coordinatorFixture(t *testing.T, n int) (CoordinatorRequest, *coreCounter) {
	t.Helper()
	in := fixture(t, n)
	built, e := Build(in, nil)
	if e != nil {
		t.Fatal(e)
	}
	p := make([]Projection, len(built.Chunks))
	s := make([]Sidecar, len(built.Chunks))
	for i := range built.Chunks {
		p[i] = built.Chunks[i].Projection
		s[i] = built.Chunks[i].Sidecar
	}
	counter := &coreCounter{t: t}
	r := CoordinatorRequest{Input: in, Plan: built.Plan, Index: built.Index, Projections: p, Sidecars: s, Store: newCoordinatorStore(MaxCanonicalBytes), Heads: newHeadStore()}
	r.Sessions = leaseFunc(func(_ context.Context, x ExpectedSession, run func(censusacquisition.Core) error) error {
		if x.SessionID != r.Plan.SessionID || x.Generation != r.Plan.Generation || x.WorkspaceURI != r.Input.WorkspaceURI {
			return errors.New("binding")
		}
		return run(counter.core())
	})
	return r, counter
}

func TestCoordinatorAuthoritativeClaimResumeAndTerminal(t *testing.T) {
	r, c := coordinatorFixture(t, ChunkSize+1)
	out, e := Coordinate(context.Background(), r)
	if e != nil {
		t.Fatal(e)
	}
	if c.count() != 160 || len(out.CommitSelectors) != 2 || out.TerminalSelector == "" {
		t.Fatalf("ASSERT_COMMITTED_CHAIN %+v calls=%d", out, c.count())
	}
	again, e := Coordinate(context.Background(), r)
	if e != nil || c.count() != 160 || again.TerminalSelector != out.TerminalSelector {
		t.Fatalf("ASSERT_COMMITTED_HEAD_NEVER_RERUN err=%v calls=%d", e, c.count())
	}
	_, h := r.Heads.(*memoryHeadStore).current()
	if h.State != headTerminal || h.CommittedPrefix != 2 {
		t.Fatalf("ASSERT_TERMINAL_HEAD %+v", h)
	}
}

func TestCoordinatorClaimWithoutReceiptIsIndeterminate(t *testing.T) {
	r, c := coordinatorFixture(t, ChunkSize+1)
	hs := r.Heads.(*memoryHeadStore)
	hs.failCAS = 2 // initial head succeeds; claim CAS fails first
	if _, e := Coordinate(context.Background(), r); e == nil || !IsCoordinatorStage(e, CoordinatorStageConflict) || c.count() != 0 {
		t.Fatalf("ASSERT_CAS_BEFORE_CORE err=%v calls=%d", e, c.count())
	}
	hs.failCAS = 0
	hs.cas = 0
	// Cancel inside the lease after a durable claim but before Core.
	r.Sessions = leaseFunc(func(context.Context, ExpectedSession, func(censusacquisition.Core) error) error {
		return context.Canceled
	})
	if _, e := Coordinate(context.Background(), r); e == nil || !IsCoordinatorStage(e, CoordinatorStageSession) {
		t.Fatalf("ASSERT_CLAIMED_CANCEL err=%v", e)
	}
	if _, e := Coordinate(context.Background(), r); e == nil || !IsCoordinatorStage(e, CoordinatorStageIndeterminate) || c.count() != 0 {
		t.Fatalf("ASSERT_NO_ARBITRARY_RETRY err=%v calls=%d", e, c.count())
	}
}

func TestCoordinatorReceiptRecoveryNoDuplicateCore(t *testing.T) {
	r, c := coordinatorFixture(t, ChunkSize+1)
	hs := r.Heads.(*memoryHeadStore)
	hs.failCAS = 4 // create, claim, receipt-link succeed; commit loses CAS
	if _, e := Coordinate(context.Background(), r); e == nil || !IsCoordinatorStage(e, CoordinatorStageConflict) {
		t.Fatalf("ASSERT_CRASH_AFTER_RECEIPT err=%v", e)
	}
	before := c.count()
	hs.failCAS = 0
	if _, e := Coordinate(context.Background(), r); e != nil || c.count() != before+1 {
		t.Fatalf("ASSERT_RECEIPT_FINALIZE_NO_DUPLICATE_CORE err=%v before=%d after=%d", e, before, c.count())
	}
}

func TestHeadTransitionRejectsRollbackForkAndDuplicate(t *testing.T) {
	base := coordinatorHead{WorkflowKey: "k", RootSelector: "r", PlanDigest: "p", IndexDigest: "i", SessionID: "s", SessionGeneration: 1, WorkspaceDigest: "w", Generation: 2, State: headCommitted, CommittedPrefix: 1}
	cases := map[string]coordinatorHead{}
	x := base
	x.Generation++
	x.State = headCommitted
	x.CommittedPrefix = 0
	cases["rollback"] = x
	x = base
	x.Generation++
	x.State = headCommitted
	x.CommittedPrefix = 2
	cases["duplicate_extension"] = x
	x = base
	x.Generation += 2
	x.State = headClaimed
	x.ClaimOrdinal = 1
	cases["stale_generation"] = x
	for name, next := range cases {
		t.Run(name, func(t *testing.T) {
			if validateTransition(base, next, 3) == nil {
				t.Fatal("ASSERT_INVALID_TRANSITION_REJECTED")
			}
		})
	}
	hs := newHeadStore()
	setHeadIdentity(&base)
	raw, _ := encodeCoordinator(base)
	hs.raw["k"] = raw
	next := base
	next.Generation++
	next.State = headClaimed
	next.ClaimOrdinal = 1
	setHeadIdentity(&next)
	nr, _ := encodeCoordinator(next)
	if hs.CompareAndSwap(context.Background(), "k", HeadRevision{1, "stale"}, nr) == nil {
		t.Fatal("ASSERT_FORKED_CAS_REJECTED")
	}
}

func TestCoordinatorLeaseTOCTOUAndPanicFailClosed(t *testing.T) {
	r, c := coordinatorFixture(t, ChunkSize+1)
	active := false
	r.Sessions = leaseFunc(func(_ context.Context, _ ExpectedSession, run func(censusacquisition.Core) error) error {
		active = true
		defer func() { active = false }()
		core := c.core()
		orig := core.Acquirer
		core.Acquirer = chunkAcquirerFunc(func(ctx context.Context, b censusacquisition.BatchRequest) (censusacquisition.AcquiredV5, error) {
			if !active {
				return censusacquisition.AcquiredV5{}, errors.New("lease released")
			}
			return orig.AcquireV5(ctx, b)
		})
		return run(core)
	})
	if _, e := Coordinate(context.Background(), r); e != nil {
		t.Fatalf("ASSERT_CUSTODY_HELD %v", e)
	}
	r, c = coordinatorFixture(t, ChunkSize+1)
	c.panicCore = true
	if _, e := Coordinate(context.Background(), r); e == nil || !IsCoordinatorStage(e, CoordinatorStagePanic) {
		t.Fatalf("ASSERT_CORE_PANIC_TYPED %v", e)
	}
	_, h := r.Heads.(*memoryHeadStore).current()
	if h.State != headClaimed {
		t.Fatalf("ASSERT_PANIC_RETAINS_CLAIM %+v", h)
	}
}

func TestCoordinatorStoreCollisionTruncationSizeCancellationAndPanic(t *testing.T) {
	cases := map[string]func(*CoordinatorRequest){
		"collision":  func(r *CoordinatorRequest) { r.Store.(*coordinatorStore).collision = true },
		"truncation": func(r *CoordinatorRequest) { r.Store.(*coordinatorStore).truncate = true },
		"size":       func(r *CoordinatorRequest) { r.Store = newCoordinatorStore(1) },
		"cancel": func(r *CoordinatorRequest) {
			r.Sessions = leaseFunc(func(context.Context, ExpectedSession, func(censusacquisition.Core) error) error {
				return context.Canceled
			})
		},
		"panic_store": func(r *CoordinatorRequest) { r.Store.(*coordinatorStore).panicPut = true },
		"panic_head":  func(r *CoordinatorRequest) { r.Heads.(*memoryHeadStore).panicCAS = true },
	}
	for n, m := range cases {
		t.Run(n, func(t *testing.T) {
			r, _ := coordinatorFixture(t, ChunkSize+1)
			m(&r)
			if _, e := Coordinate(context.Background(), r); e == nil {
				t.Fatal("ASSERT_FAIL_CLOSED_BOUNDARY")
			}
		})
	}
}

func TestCoordinatorTerminalDispositionSubstitution(t *testing.T) {
	r, _ := coordinatorFixture(t, ChunkSize+1)
	out, e := Coordinate(context.Background(), r)
	if e != nil {
		t.Fatal(e)
	}
	st := r.Store.(*coordinatorStore)
	raw, _ := st.Get(context.Background(), out.TerminalSelector)
	term, _ := ParseTerminalIndex(raw)
	st.mu.Lock()
	st.objects[term.GlobalDispositionSelector] = []byte(`{}`)
	st.mu.Unlock()
	if _, e = Coordinate(context.Background(), r); e == nil || !IsCoordinatorStage(e, CoordinatorStageTerminal) {
		t.Fatalf("ASSERT_DISPOSITION_SUBSTITUTION %v", e)
	}
}

func TestHeadTransitionAndReceiptRecoveryRaceSmall(t *testing.T) {
	for i := 0; i < 32; i++ {
		hs := newHeadStore()
		base := coordinatorHead{SchemaVersion: CoordinatorHeadVersion, WorkflowKey: "k", RootSelector: "r", PlanDigest: "p", IndexDigest: "i", SessionID: "s", SessionGeneration: 1, WorkspaceDigest: "w", Generation: 1, State: headCommitted, ClaimOrdinal: -1}
		setHeadIdentity(&base)
		baseRaw, _ := encodeCoordinator(base)
		hs.raw["k"] = baseRaw
		var wg sync.WaitGroup
		wg.Add(2)
		results := make(chan error, 2)
		for fork := 0; fork < 2; fork++ {
			go func() {
				defer wg.Done()
				next := base
				next.Generation++
				next.State = headClaimed
				next.ClaimOrdinal = 0
				next.IdempotencyKey = claimKey("k", 0)
				if err := validateTransition(base, next, 2); err != nil {
					results <- err
					return
				}
				setHeadIdentity(&next)
				raw, _ := encodeCoordinator(next)
				results <- hs.CompareAndSwap(context.Background(), "k", HeadRevision{Generation: base.Generation, Selector: base.ImmutableSelector}, raw)
			}()
		}
		wg.Wait()
		close(results)
		wins := 0
		for err := range results {
			if err == nil {
				wins++
			}
		}
		if wins != 1 {
			t.Fatalf("ASSERT_ONE_CAS_WINNER wins=%d", wins)
		}
	}
}

func TestCoordinatorClaimReceiptRejectsForeignReplay(t *testing.T) {
	cases := map[string]func(*claimReceipt){
		"workflow":     func(v *claimReceipt) { v.WorkflowKey += "-foreign" },
		"generation":   func(v *claimReceipt) { v.ClaimedHeadGeneration++ },
		"idempotency":  func(v *claimReceipt) { v.IdempotencyKey += "-foreign" },
		"predecessor":  func(v *claimReceipt) { v.PredecessorSelector += "-foreign" },
		"execution_id": func(v *claimReceipt) { v.ChunkExecutionDigest = domainDigest("foreign", []byte("execution")) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r, _ := coordinatorFixture(t, ChunkSize+1)
			hs := r.Heads.(*memoryHeadStore)
			hs.failCAS = 4
			if _, err := Coordinate(context.Background(), r); err == nil || !IsCoordinatorStage(err, CoordinatorStageConflict) {
				t.Fatalf("ASSERT_RECEIPT_SETUP err=%v", err)
			}
			key, h := hs.current()
			raw, err := r.Store.Get(context.Background(), h.ReceiptSelector)
			if err != nil {
				t.Fatal(err)
			}
			var envelope claimReceipt
			if err = decodeCoordinator(raw, &envelope); err != nil {
				t.Fatal(err)
			}
			mutate(&envelope)
			raw, _ = encodeCoordinator(envelope)
			selector, err := r.Store.Put(context.Background(), raw)
			if err != nil {
				t.Fatal(err)
			}
			h.ReceiptSelector = selector
			hs.set(key, h)
			hs.failCAS = 0
			if _, err = Coordinate(context.Background(), r); err == nil || !IsCoordinatorStage(err, CoordinatorStageReadback) {
				t.Fatalf("ASSERT_FOREIGN_RECEIPT_REJECTED err=%v", err)
			}
		})
	}
}

func TestCoordinatorTerminalReplayRejectsIndependentSubstitution(t *testing.T) {
	validDigest := func(label string) string { return domainDigest("terminal-substitution", []byte(label)) }
	cases := map[string]func(*TerminalIndex){
		"plan_digest":  func(v *TerminalIndex) { v.PlanDigest = validDigest("plan") },
		"index_digest": func(v *TerminalIndex) { v.IndexDigest = validDigest("index") },
		"ordered_commit_selectors": func(v *TerminalIndex) {
			v.CommitSelectors[0], v.CommitSelectors[1] = v.CommitSelectors[1], v.CommitSelectors[0]
		},
		"global_target_count":   func(v *TerminalIndex) { v.GlobalTargetCount++ },
		"final_checkpoint":      func(v *TerminalIndex) { v.FinalCheckpoint += "-foreign" },
		"disposition_selector":  func(v *TerminalIndex) { v.GlobalDispositionSelector += "-foreign" },
		"disposition_digest":    func(v *TerminalIndex) { v.GlobalDispositionDigest = validDigest("disposition") },
		"source_graph_complete": func(v *TerminalIndex) { v.SourceGraphComplete = "COMPLETE" },
		"cross_chunk_calls":     func(v *TerminalIndex) { v.CrossChunkCalls = "COMPUTED" },
		"aggregate_capture":     func(v *TerminalIndex) { v.AggregateCapture = "MATERIALIZED" },
		"workflow_key":          func(v *TerminalIndex) { v.WorkflowKey += "-foreign" },
		"head_generation":       func(v *TerminalIndex) { v.HeadGeneration++ },
		"head_selector":         func(v *TerminalIndex) { v.HeadSelector += "-foreign" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r, _ := coordinatorFixture(t, ChunkSize+1)
			out, err := Coordinate(context.Background(), r)
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := r.Store.Get(context.Background(), out.TerminalSelector)
			term, err := ParseTerminalIndex(raw)
			if err != nil {
				t.Fatal(err)
			}
			mutate(&term)
			setCoordinatorIdentity("lsp-trace:census-chunk-terminal-index:private:v2", "census-chunk-terminal-indexes/v2/", &term, func(v *TerminalIndex, d, s string) { v.LogicalDigest = d; v.ImmutableSelector = s })
			raw, _ = encodeCoordinator(term)
			selector, err := r.Store.Put(context.Background(), raw)
			if err != nil {
				t.Fatal(err)
			}
			key, h := r.Heads.(*memoryHeadStore).current()
			h.TerminalSelector = selector
			r.Heads.(*memoryHeadStore).set(key, h)
			if _, err = Coordinate(context.Background(), r); err == nil || !IsCoordinatorStage(err, CoordinatorStageTerminal) {
				t.Fatalf("ASSERT_TERMINAL_SUBSTITUTION_REJECTED err=%v", err)
			}
		})
	}
}

func TestCoordinatorSessionLeaseCallbackCardinality(t *testing.T) {
	t.Run("zero", func(t *testing.T) {
		r, c := coordinatorFixture(t, ChunkSize+1)
		r.Sessions = leaseFunc(func(context.Context, ExpectedSession, func(censusacquisition.Core) error) error { return nil })
		if _, err := Coordinate(context.Background(), r); err == nil || !IsCoordinatorStage(err, CoordinatorStageSession) || c.count() != 0 {
			t.Fatalf("ASSERT_ZERO_CALLBACK_REFUSED err=%v calls=%d", err, c.count())
		}
	})
	t.Run("double", func(t *testing.T) {
		r, c := coordinatorFixture(t, ChunkSize+1)
		r.Sessions = leaseFunc(func(_ context.Context, _ ExpectedSession, run func(censusacquisition.Core) error) error {
			if err := run(c.core()); err != nil {
				return err
			}
			_ = run(c.core()) // malicious lease ignores the callback's conflict
			return nil
		})
		if _, err := Coordinate(context.Background(), r); err == nil || !IsCoordinatorStage(err, CoordinatorStageConflict) || c.count() != 159 {
			t.Fatalf("ASSERT_DOUBLE_CALLBACK_CONFLICT err=%v calls=%d", err, c.count())
		}
	})
	t.Run("concurrent", func(t *testing.T) {
		r, c := coordinatorFixture(t, ChunkSize+1)
		r.Sessions = leaseFunc(func(_ context.Context, _ ExpectedSession, run func(censusacquisition.Core) error) error {
			var wg sync.WaitGroup
			wg.Add(2)
			for i := 0; i < 2; i++ {
				go func() { defer wg.Done(); _ = run(c.core()) }()
			}
			wg.Wait()
			return nil
		})
		if _, err := Coordinate(context.Background(), r); err == nil || !IsCoordinatorStage(err, CoordinatorStageConflict) || c.count() != 159 {
			t.Fatalf("ASSERT_CONCURRENT_CALLBACK_CONFLICT err=%v calls=%d", err, c.count())
		}
	})
	t.Run("success_then_error", func(t *testing.T) {
		r, c := coordinatorFixture(t, ChunkSize+1)
		r.Sessions = leaseFunc(func(_ context.Context, _ ExpectedSession, run func(censusacquisition.Core) error) error {
			if err := run(c.core()); err != nil {
				return err
			}
			return errors.New("lease post-success sentinel")
		})
		if _, err := Coordinate(context.Background(), r); err == nil || !IsCoordinatorStage(err, CoordinatorStageIndeterminate) || IsCoordinatorStage(err, CoordinatorStageCore) || c.count() != 159 {
			t.Fatalf("ASSERT_SUCCESS_THEN_ERROR_INDETERMINATE err=%v calls=%d", err, c.count())
		}
	})
	t.Run("failure_then_extra", func(t *testing.T) {
		r, c := coordinatorFixture(t, ChunkSize+1)
		r.Sessions = leaseFunc(func(_ context.Context, _ ExpectedSession, run func(censusacquisition.Core) error) error {
			_ = run(censusacquisition.Core{})
			return run(c.core())
		})
		if _, err := Coordinate(context.Background(), r); err == nil || !IsCoordinatorStage(err, CoordinatorStageConflict) || c.count() != 0 {
			t.Fatalf("ASSERT_FAILURE_THEN_EXTRA_CONFLICT err=%v calls=%d", err, c.count())
		}
	})
}

func TestCoordinatorNormal10001And20001SemanticBoundary(t *testing.T) {
	for _, n := range []int{10001, 20001} {
		t.Run(string(rune(n)), func(t *testing.T) {
			r, _ := coordinatorFixture(t, n)
			if got := len(r.Plan.Chunks); got != (n+ChunkSize-1)/ChunkSize {
				t.Fatalf("ASSERT_NORMAL_BOUNDARY chunks=%d", got)
			}
		})
	}
}
